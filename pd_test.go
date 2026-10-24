/*
 * Copyright (c) 2026 Siddharth Chandrasekaran <sidcha@osdp.dev>
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package osdp_test

import (
	"testing"
	"time"

	"github.com/osdp-dev/libosdp-go"
)

func TestPDNotificationsDoNotSurfaceAsStatusCommands(t *testing.T) {
	link := newFIFOLink(t)
	pd := startTestPD(t, link, testPdInfo(osdp.FlagInstallMode|osdp.FlagEnableNotification), nil)

	cmds := make(chan *osdp.Command, 32)
	pd.SetCommandHandler(func(cmd *osdp.Command) (int, *osdp.Event) {
		cmds <- cmd
		return 0, nil
	})

	cp := startTestCP(t, link, testPdInfo(osdp.FlagInstallMode))
	if !cp.OnlineWait(0, 10*time.Second) {
		t.Fatal("PD did not come online")
	}

	deadline := time.After(2 * time.Second)
	for {
		select {
		case cmd := <-cmds:
			if cmd.Command.GetType() == osdp.CmdStatus {
				t.Fatalf("PD saw %#v; the CP never sent a status command", cmd.Command)
			}
		case <-deadline:
			return
		}
	}
}

func TestPDReceivesLinkUpNotification(t *testing.T) {
	link := newFIFOLink(t)
	pd := startTestPD(t, link, testPdInfo(osdp.FlagInstallMode|osdp.FlagEnableNotification), nil)

	linkUp := make(chan struct{}, 1)
	pd.SetCommandHandler(func(cmd *osdp.Command) (int, *osdp.Event) {
		n, ok := cmd.Command.(osdp.CommandNotification)
		if !ok || n.Type != osdp.NotificationTypePDStatus {
			return 0, nil
		}
		if p, ok := n.Payload.(osdp.NotificationPDStatus); ok && p.Online {
			select {
			case linkUp <- struct{}{}:
			default:
			}
		}
		return 0, nil
	})

	startTestCP(t, link, testPdInfo(osdp.FlagInstallMode))
	select {
	case <-linkUp:
	case <-time.After(10 * time.Second):
		t.Fatal("PD never reported the CP link coming up")
	}
}

func TestSubmitCommandRejectsNotification(t *testing.T) {
	link := newFIFOLink(t)
	cp := startTestCP(t, link, testPdInfo(osdp.FlagInstallMode))
	err := cp.SubmitCommand(0, &osdp.Command{Command: osdp.CommandNotification{
		Type: osdp.NotificationTypePDStatus, Payload: osdp.NotificationPDStatus{Online: true},
	}})
	if err == nil {
		t.Fatal("SubmitCommand accepted a synthesized notification")
	}
}

func TestPDReceivesComsetDoneWithAppliedSettings(t *testing.T) {
	link := newFIFOLink(t)
	pd := startTestPD(t, link, testPdInfo(osdp.FlagInstallMode), nil)

	cmds := make(chan *osdp.Command, 8)
	pd.SetCommandHandler(func(cmd *osdp.Command) (int, *osdp.Event) {
		cmds <- cmd
		return 0, nil
	})

	cp := startTestCP(t, link, testPdInfo(osdp.FlagInstallMode))
	if !cp.OnlineWait(0, 10*time.Second) {
		t.Fatal("PD did not come online")
	}
	// Re-apply the current settings so the link survives the change.
	want := osdp.CommandComset{Address: 1, BaudRate: 9600}
	if err := cp.SubmitCommand(0, &osdp.Command{Command: want}); err != nil {
		t.Fatalf("SubmitCommand: %v", err)
	}

	deadline := time.After(5 * time.Second)
	for {
		select {
		case cmd := <-cmds:
			if cmd.Command.GetType() != osdp.CmdComsetDone {
				continue
			}
			got, ok := cmd.Command.(osdp.CommandComsetDone)
			if !ok || got.Address != want.Address || got.BaudRate != want.BaudRate {
				t.Fatalf("COMSET_DONE = %#v, want settings %+v", cmd.Command, want)
			}
			return
		case <-deadline:
			t.Fatal("PD never reported COMSET_DONE")
		}
	}
}

// A reply event returned from the command handler must go out as the reply to
// that command. The CP dispatches a reply's event before it completes the
// command, whereas an ACK completes it first and the event trails on a poll.
func TestPDHandlerReplyIsSentAsDirectReply(t *testing.T) {
	link := newFIFOLink(t)
	pd := startTestPD(t, link, testPdInfo(osdp.FlagInstallMode), nil)
	vendor := [3]uint8{0x0A, 0x0B, 0x0C}
	pd.SetCommandHandler(func(cmd *osdp.Command) (int, *osdp.Event) {
		if cmd.Command.GetType() != osdp.CmdMfg {
			return 0, nil
		}
		return 0, &osdp.Event{Event: osdp.EventMfgReply{VendorCode: vendor, Data: []byte{0x42}}}
	})

	cp := startTestCP(t, link, testPdInfo(osdp.FlagInstallMode))
	order := make(chan string, 4)
	cp.SetEventHandler(func(_ int, ev *osdp.Event) int {
		if ev.Event.GetType() == osdp.EvtMfgReply {
			order <- "reply"
		}
		return 0
	})
	cp.SetCommandCompletionHandler(func(_ int, cmd *osdp.Command, _ osdp.CompletionStatus) {
		if cmd.Command.GetType() == osdp.CmdMfg {
			order <- "completed"
		}
	})
	if !cp.OnlineWait(0, 10*time.Second) {
		t.Fatal("PD did not come online")
	}
	if err := cp.SubmitCommand(0, &osdp.Command{Command: osdp.CommandMfg{VendorCode: vendor, Data: []byte{1}}}); err != nil {
		t.Fatalf("SubmitCommand: %v", err)
	}

	for _, want := range []string{"reply", "completed"} {
		select {
		case got := <-order:
			if got != want {
				t.Fatalf("CP saw %q before the reply; PD ACKed instead of replying", got)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out waiting for %q", want)
		}
	}
}

func TestFlushEventsDropsQueuedEvents(t *testing.T) {
	pd := startTestPD(t, newFIFOLink(t), testPdInfo(osdp.FlagInstallMode), nil) // no CP: events stay queued
	flushed := make(chan struct{}, 4)
	pd.SetEventCompletionHandler(func(_ *osdp.Event, st osdp.CompletionStatus) {
		if st == osdp.CompletionFlushed {
			flushed <- struct{}{}
		}
	})
	for i := 0; i < 3; i++ {
		if err := pd.SubmitEvent(&osdp.Event{Event: osdp.EventKeyPress{Data: []byte{'1'}}}); err != nil {
			t.Fatalf("SubmitEvent %d: %v", i, err)
		}
	}
	if n, err := pd.FlushEvents(); err != nil || n != 3 {
		t.Fatalf("FlushEvents = %d, %v; want 3", n, err)
	}
	for i := 0; i < 3; i++ {
		select {
		case <-flushed:
		case <-time.After(time.Second):
			t.Fatalf("only %d of 3 events reported as flushed", i)
		}
	}
}

// TestPDMetricsWithoutLink checks the PD getters that do not require an
// established link.
func TestPDMetricsWithoutLink(t *testing.T) {
	_, _, pdRx, pdTx, cleanup := createFIFOPair(t)
	defer cleanup()

	ch, err := osdp.NewFIFOChannel(pdRx, pdTx)
	if err != nil {
		t.Fatalf("PD channel: %v", err)
	}
	defer ch.Close()

	pd, err := osdp.NewPeripheralDevice(ch,
		&osdp.PdInfo{Address: 1, BaudRate: 9600, Flags: osdp.FlagInstallMode,
			ID: osdp.PdId{Version: 1, Model: 1, VendorCode: 0x153, SerialNumber: 0x12345, FirmwareMajor: 1}},
		osdp.LogError)
	if err != nil {
		t.Fatalf("NewPeripheralDevice: %v", err)
	}
	defer pd.Stop()

	if _, err := pd.GetMetrics(); err != nil {
		t.Errorf("PD GetMetrics: %v", err)
	}
	if online, err := pd.IsOnline(); err != nil || online {
		t.Errorf("IsOnline = %v (err %v), want false (no CP)", online, err)
	}
}

// TestPdInfoCapabilitiesAdvertised verifies that capabilities set on
// PdInfo.Capabilities are advertised by the PD and discoverable by the CP via
// GetPDCapability.
func TestPdInfoCapabilitiesAdvertised(t *testing.T) {
	cpRx, cpTx, pdRx, pdTx, cleanup := createFIFOPair(t)
	defer cleanup()

	id := osdp.PdId{Version: 1, Model: 1, VendorCode: 0x153, SerialNumber: 0x12345, FirmwareMajor: 1}

	pdChannel, err := osdp.NewFIFOChannel(pdRx, pdTx)
	if err != nil {
		t.Fatalf("PD channel: %v", err)
	}
	defer pdChannel.Close()

	pd, err := osdp.NewPeripheralDevice(pdChannel,
		&osdp.PdInfo{
			Address: 1, BaudRate: 9600, Flags: osdp.FlagInstallMode, ID: id,
			Capabilities: []osdp.PdCapability{{Function: osdp.CapOutputControl, Compliance: 1, NumItems: 4}},
		},
		osdp.LogError)
	if err != nil {
		t.Fatalf("NewPeripheralDevice: %v", err)
	}
	defer pd.Stop()

	cpChannel, err := osdp.NewFIFOChannel(cpRx, cpTx)
	if err != nil {
		t.Fatalf("CP channel: %v", err)
	}
	defer cpChannel.Close()

	cp, err := osdp.NewControlPanelBuilder(cpChannel).
		AddPD(&osdp.PdInfo{Address: 1, BaudRate: 9600, Flags: osdp.FlagInstallMode, ID: id}).
		LogLevel(osdp.LogError).
		Build()
	if err != nil {
		t.Fatalf("Build CP: %v", err)
	}
	defer cp.Stop()

	if !cp.OnlineWait(0, 10*time.Second) {
		t.Fatal("PD did not come online within timeout")
	}

	cap, err := cp.GetPDCapability(0, osdp.CapOutputControl)
	if err != nil {
		t.Fatalf("GetPDCapability: %v", err)
	}
	if cap.NumItems != 4 {
		t.Errorf("advertised OutputControl NumItems = %d, want 4", cap.NumItems)
	}
}
