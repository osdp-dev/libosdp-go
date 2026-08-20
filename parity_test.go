/*
 * Copyright (c) 2026 Siddharth Chandrasekaran <sidcha@osdp.dev>
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package osdp_test

import (
	"bytes"
	"path/filepath"
	"testing"
	"time"

	"github.com/osdp-dev/libosdp-go"
)

// TestKeyStore exercises the pure-Go SCBK key store.
func TestKeyStore(t *testing.T) {
	ks, err := osdp.NewKeyStore("")
	if err != nil {
		t.Fatalf("NewKeyStore: %v", err)
	}
	defer ks.Close()

	key, err := ks.NewKey("scbk", 16, true)
	if err != nil {
		t.Fatalf("NewKey: %v", err)
	}
	if len(key) != 16 {
		t.Fatalf("key length = %d, want 16", len(key))
	}

	got, err := ks.GetKey("scbk")
	if err != nil || !bytes.Equal(got, key) {
		t.Fatalf("GetKey = %x (err %v), want %x", got, err, key)
	}

	// force=false must refuse to overwrite an existing key.
	if _, err := ks.NewKey("scbk", 16, false); err == nil {
		t.Error("NewKey(force=false) on existing key should error")
	}

	if err := ks.CommitKey("scbk"); err != nil {
		t.Fatalf("CommitKey: %v", err)
	}

	// A second store pointed at the same directory should load the same bytes.
	ks2, err := osdp.NewKeyStore(filepath.Dir(ks.KeyFile("scbk")))
	if err != nil {
		t.Fatalf("NewKeyStore(dir): %v", err)
	}
	loaded, err := ks2.LoadKey("scbk", 16)
	if err != nil || !bytes.Equal(loaded, key) {
		t.Fatalf("LoadKey = %x (err %v), want %x", loaded, err, key)
	}
	if _, err := ks2.LoadKey("scbk", 32); err == nil {
		t.Error("LoadKey with wrong length should error")
	}
}

// TestPDSetCapabilitiesAndMetrics checks the PD-side parity additions that do
// not require an established link.
func TestPDSetCapabilitiesAndMetrics(t *testing.T) {
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
		nil, osdp.LogError)
	if err != nil {
		t.Fatalf("NewPeripheralDevice: %v", err)
	}
	defer pd.Stop()

	if err := pd.SetCapabilities([]osdp.PdCapability{
		{Function: osdp.CapReaderLEDControl, Compliance: 1, NumItems: 2},
		{Function: osdp.CapReaderAudibleOutput, Compliance: 1, NumItems: 1},
	}); err != nil {
		t.Errorf("SetCapabilities: %v", err)
	}

	if _, err := pd.GetMetrics(); err != nil {
		t.Errorf("PD GetMetrics: %v", err)
	}
	if online, err := pd.IsOnline(); err != nil || online {
		t.Errorf("IsOnline = %v (err %v), want false (no CP)", online, err)
	}
}

// TestCompletionCallbacksAndMetrics brings up a CP/PD pair over FIFOs and
// verifies the completion callbacks, status/wait helpers, and metrics added for
// Python parity.
func TestCompletionCallbacksAndMetrics(t *testing.T) {
	cpRx, cpTx, pdRx, pdTx, cleanup := createFIFOPair(t)
	defer cleanup()

	id := osdp.PdId{Version: 1, Model: 1, VendorCode: 0x153, SerialNumber: 0x12345, FirmwareMajor: 1}

	// --- PD ---
	pdChannel, err := osdp.NewFIFOChannel(pdRx, pdTx)
	if err != nil {
		t.Fatalf("PD channel: %v", err)
	}
	defer pdChannel.Close()

	pd, err := osdp.NewPeripheralDevice(pdChannel,
		&osdp.PdInfo{Address: 1, BaudRate: 9600, Flags: osdp.FlagInstallMode, ID: id},
		[]osdp.PdCapability{{Function: osdp.CapReaderLEDControl, Compliance: 1, NumItems: 2}},
		osdp.LogError)
	if err != nil {
		t.Fatalf("NewPeripheralDevice: %v", err)
	}
	defer pd.Stop()

	eventDone := make(chan osdp.CompletionStatus, 4)
	pd.SetEventCompletionHandler(func(ev *osdp.Event, status osdp.CompletionStatus) {
		eventDone <- status
	})

	// --- CP ---
	cpChannel, err := osdp.NewFIFOChannel(cpRx, cpTx)
	if err != nil {
		t.Fatalf("CP channel: %v", err)
	}
	defer cpChannel.Close()

	cp, err := osdp.NewControlPanel(cpChannel,
		[]*osdp.PdInfo{{Address: 1, BaudRate: 9600, Flags: osdp.FlagInstallMode, ID: id}},
		osdp.LogError)
	if err != nil {
		t.Fatalf("NewControlPanel: %v", err)
	}
	defer cp.Stop()

	cmdDone := make(chan osdp.CompletionStatus, 4)
	cp.SetCommandCompletionHandler(func(pdNo int, cmd *osdp.Command, status osdp.CompletionStatus) {
		cmdDone <- status
	})

	if !cp.OnlineWait(0, 10*time.Second) {
		t.Fatal("PD did not come online within timeout")
	}

	// Status / count helpers.
	if n, err := cp.GetNumOnline(); err != nil || n != 1 {
		t.Errorf("GetNumOnline = %d (err %v), want 1", n, err)
	}
	if mask, err := cp.Status(); err != nil || mask&0x1 == 0 {
		t.Errorf("Status = %#x (err %v), want bit 0 set", mask, err)
	}
	if online, err := cp.IsOnline(0); err != nil || !online {
		t.Errorf("IsOnline = %v (err %v), want true", online, err)
	}

	// Command completion: a valid LED command should be ACKed (CompletionOK).
	led := &osdp.Command{Command: osdp.CommandLed{
		Reader: 0, LedNumber: 0,
		Permanent: osdp.LedParams{ControlCode: 1, OnCount: 10, OffCount: 0, OnColor: osdp.LEDColorRed},
	}}
	if err := cp.SubmitCommand(0, led); err != nil {
		t.Fatalf("SubmitCommand: %v", err)
	}
	select {
	case st := <-cmdDone:
		if st != osdp.CompletionOK {
			t.Errorf("command completion = %v, want CompletionOK", st)
		}
	case <-time.After(5 * time.Second):
		t.Error("command completion callback did not fire")
	}

	// Event completion: a card-read event submitted by the PD should complete.
	if err := pd.SubmitEvent(&osdp.Event{Event: osdp.EventCardRead{
		ReaderNo: 0, Format: osdp.CardFormatWiegand, Direction: true, NrBits: 32, Data: []byte{0x12, 0x34, 0x56, 0x78},
	}}); err != nil {
		t.Fatalf("SubmitEvent: %v", err)
	}
	select {
	case st := <-eventDone:
		if st != osdp.CompletionOK {
			t.Errorf("event completion = %v, want CompletionOK", st)
		}
	case <-time.After(5 * time.Second):
		t.Error("event completion callback did not fire")
	}

	// Metrics should reflect the traffic exchanged above.
	m, err := cp.GetMetrics(0)
	if err != nil {
		t.Fatalf("GetMetrics: %v", err)
	}
	if m.PacketsSent == 0 || m.PacketsReceived == 0 {
		t.Errorf("metrics look empty: %+v", m)
	}
}

// TestPdInfoCapabilitiesAdvertised verifies that capabilities set on
// PdInfo.Capabilities (with no explicit capabilities argument) are advertised by
// the PD and discoverable by the CP via GetPDCapability.
func TestPdInfoCapabilitiesAdvertised(t *testing.T) {
	cpRx, cpTx, pdRx, pdTx, cleanup := createFIFOPair(t)
	defer cleanup()

	id := osdp.PdId{Version: 1, Model: 1, VendorCode: 0x153, SerialNumber: 0x12345, FirmwareMajor: 1}

	pdChannel, err := osdp.NewFIFOChannel(pdRx, pdTx)
	if err != nil {
		t.Fatalf("PD channel: %v", err)
	}
	defer pdChannel.Close()

	// Capabilities live ONLY on PdInfo.Capabilities; the explicit argument is nil.
	pd, err := osdp.NewPeripheralDevice(pdChannel,
		&osdp.PdInfo{
			Address: 1, BaudRate: 9600, Flags: osdp.FlagInstallMode, ID: id,
			Capabilities: []osdp.PdCapability{{Function: osdp.CapOutputControl, Compliance: 1, NumItems: 4}},
		},
		nil,
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

	cp, err := osdp.NewControlPanel(cpChannel,
		[]*osdp.PdInfo{{Address: 1, BaudRate: 9600, Flags: osdp.FlagInstallMode, ID: id}},
		osdp.LogError)
	if err != nil {
		t.Fatalf("NewControlPanel: %v", err)
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
