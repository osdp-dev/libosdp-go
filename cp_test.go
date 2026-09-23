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

func startOnlinePair(t *testing.T, cpFlags osdp.OsdpFlag) (*osdp.ControlPanel, *osdp.PeripheralDevice, *osdp.PdInfo) {
	t.Helper()
	link := newFIFOLink(t)
	pdInfo := testPdInfo(osdp.FlagInstallMode)
	pdInfo.ID = osdp.PdId{Version: 2, Model: 5, VendorCode: 0xA1B2C3, SerialNumber: 0x0BADF00D,
		FirmwareMajor: 3, FirmwareMinor: 4, FirmwareBuild: 5}
	pd := startTestPD(t, link, pdInfo, nil)
	cp := startTestCP(t, link, testPdInfo(osdp.FlagInstallMode|cpFlags))
	if !cp.OnlineWait(0, 10*time.Second) {
		t.Fatal("PD did not come online")
	}
	return cp, pd, pdInfo
}

func TestGetPDIDReturnsIdentityReportedByPD(t *testing.T) {
	cp, _, pdInfo := startOnlinePair(t, 0)
	got, err := cp.GetPDID(0)
	if err != nil {
		t.Fatalf("GetPDID: %v", err)
	}
	if *got != pdInfo.ID {
		t.Fatalf("GetPDID = %+v, want %+v", *got, pdInfo.ID)
	}
	if _, err := cp.GetPDID(1); err == nil {
		t.Error("GetPDID accepted an out-of-range PD number")
	}
}

func TestSetFlagEnablesNotificationsAtRuntime(t *testing.T) {
	cp, pd, _ := startOnlinePair(t, 0)
	pd.RegisterFileOperations(newMemFile(1, nil))
	cp.RegisterFileOperations(0, newMemFile(1, make([]byte, 64)))
	started := make(chan osdp.NotificationMP, 4)
	cp.SetEventHandler(func(_ int, ev *osdp.Event) int {
		if n, ok := ev.Event.(osdp.EventNotification); ok && n.Type == osdp.NotificationTypeMPStart {
			started <- n.Payload.(osdp.NotificationMP)
		}
		return 0
	})
	done := make(chan struct{}, 4)
	cp.SetCommandCompletionHandler(func(int, *osdp.Command, osdp.CompletionStatus) { done <- struct{}{} })
	transfer := func() {
		t.Helper()
		if err := cp.SubmitCommand(0, &osdp.Command{Command: osdp.CommandFileTx{ID: 1}}); err != nil {
			t.Fatalf("SubmitCommand(FileTx): %v", err)
		}
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatal("file transfer did not complete")
		}
	}

	if err := cp.SetFlag(0, osdp.FlagEnableNotification); err != nil {
		t.Fatalf("SetFlag: %v", err)
	}
	transfer()
	select {
	case n := <-started:
		if n.MPType != osdp.MPMsgFileTransfer {
			t.Errorf("MP start for %v, want file transfer", n.MPType)
		}
	default:
		t.Fatal("no MP start notification after enabling the flag")
	}

	if err := cp.ClearFlag(0, osdp.FlagEnableNotification); err != nil {
		t.Fatalf("ClearFlag: %v", err)
	}
	transfer()
	select {
	case n := <-started:
		t.Fatalf("got %+v after clearing the flag", n)
	default:
	}
}

func TestSetFlagRejectsSetupOnlyFlag(t *testing.T) {
	cp, _, _ := startOnlinePair(t, 0)
	if err := cp.SetFlag(0, osdp.FlagCapturePackets); err == nil {
		t.Fatal("SetFlag accepted FlagCapturePackets, which is setup-only")
	}
}

func TestAddPDAppendsAddressablePD(t *testing.T) {
	cp, _, _ := startOnlinePair(t, 0)
	extra := testPdInfo(osdp.FlagInstallMode)
	extra.Address = 2
	if err := cp.AddPD([]*osdp.PdInfo{extra}); err != nil {
		t.Fatalf("AddPD: %v", err)
	}
	if enabled, err := cp.IsPDEnabled(1); err != nil || !enabled {
		t.Fatalf("IsPDEnabled(1) = %v, %v; want the added PD enabled", enabled, err)
	}
	if online, err := cp.IsOnline(1); err != nil || online {
		t.Errorf("IsOnline(1) = %v, %v; nothing answers at address 2", online, err)
	}
	if !cp.OnlineWait(0, 2*time.Second) {
		t.Error("existing PD went offline after AddPD")
	}
	// libosdp refuses commands for a PD that is not online.
	if err := cp.SubmitCommand(1, &osdp.Command{Command: osdp.CommandBuzzer{}}); err == nil {
		t.Error("SubmitCommand to the offline added PD succeeded")
	}
	if _, err := cp.IsPDEnabled(2); err == nil {
		t.Error("IsPDEnabled(2) accepted an index past the added PD")
	}
}
