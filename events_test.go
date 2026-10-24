/*
 * Copyright (c) 2026 Siddharth Chandrasekaran <sidcha@osdp.dev>
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package osdp_test

import (
	"bytes"
	"sync"
	"testing"
	"time"

	"github.com/osdp-dev/libosdp-go"
)

// TestEventGeneration tests PD event generation capabilities
func TestEventGeneration(t *testing.T) {
	cpRx, cpTx, pdRx, pdTx, cleanup := createFIFOPair(t)
	defer cleanup()

	var wg sync.WaitGroup
	wg.Add(2)

	eventsReceived := make(map[osdp.OsdpEventType]int)
	var eventsMutex sync.Mutex

	// Start CP to receive events
	go func() {
		defer wg.Done()

		eventHandler := func(pdNumber int, event *osdp.Event) int {
			eventsMutex.Lock()
			eventsReceived[event.Event.GetType()]++
			eventsMutex.Unlock()

			t.Logf("CP received event: Type %d from PD %d", event.Event.GetType(), pdNumber)

			switch eventData := event.Event.(type) {
			case osdp.EventCardRead:
				t.Logf("Card read: Reader %d, Format %d, Data %v", eventData.ReaderNo, eventData.Format, eventData.Data)
			case osdp.EventKeyPress:
				t.Logf("Keypad: Reader %d, Data %s", eventData.ReaderNo, string(eventData.Data))
			}
			return 0
		}

		cpChannel, err := osdp.NewFIFOChannel(cpRx, cpTx)
		if err != nil {
			t.Errorf("Failed to create CP FIFO channel: %v", err)
			return
		}
		defer cpChannel.Close()

		pdInfo := &osdp.PdInfo{
			Address: 1, BaudRate: 9600, Flags: osdp.FlagInstallMode,
			ID: osdp.PdId{Version: 1, Model: 1, VendorCode: 0x153, SerialNumber: 0x12345},
		}

		cp, err := osdp.NewControlPanelBuilder(cpChannel).AddPD(pdInfo).Build()
		if err != nil {
			t.Errorf("Failed to create CP: %v", err)
			return
		}
		defer cp.Stop()

		if err := cp.SetEventHandler(eventHandler); err != nil {
			t.Errorf("Failed to set event handler: %v", err)
		}

		time.Sleep(8 * time.Second)
	}()

	// Start PD and generate events
	go func() {
		defer wg.Done()

		pdChannel, err := osdp.NewFIFOChannel(pdRx, pdTx)
		if err != nil {
			t.Errorf("Failed to create PD FIFO channel: %v", err)
			return
		}
		defer pdChannel.Close()

		pdInfo := &osdp.PdInfo{
			Address:  1,
			BaudRate: 9600,
			Flags:    osdp.FlagInstallMode,
			ID: osdp.PdId{
				Version: 1, Model: 1, VendorCode: 0x153,
				SerialNumber: 0x12345, FirmwareMajor: 1,
			},
		}

		capabilities := []osdp.PdCapability{
			{Function: osdp.CapCardDataFormat, Compliance: 1, NumItems: 1},
			// OUTPUT_CONTROL with 1 item so a 1-entry OSTATR report can be built.
			{Function: osdp.CapOutputControl, Compliance: 1, NumItems: 1},
		}

		pd, err := osdp.NewPeripheralDevice(pdChannel, pdInfo, capabilities, osdp.LogInfo)
		if err != nil {
			t.Errorf("Failed to create PD: %v", err)
			return
		}
		defer pd.Stop()

		// Wait for connection
		time.Sleep(2 * time.Second)

		// Generate card read event
		cardEvent := &osdp.Event{
			Event: osdp.EventCardRead{
				ReaderNo:  0,
				Format:    osdp.CardFormatWiegand,
				Direction: true,
				NrBits:    32,
				Data:      []byte{0x12, 0x34, 0x56, 0x78},
			},
		}
		err = pd.SubmitEvent(cardEvent)
		if err != nil {
			t.Errorf("Failed to send card read event: %v", err)
		}

		time.Sleep(1 * time.Second)

		// Generate keypad event
		keypadEvent := &osdp.Event{
			Event: osdp.EventKeyPress{
				ReaderNo: 0,
				Data:     []byte("1234"),
			},
		}
		err = pd.SubmitEvent(keypadEvent)
		if err != nil {
			t.Errorf("Failed to send keypad event: %v", err)
		}

		time.Sleep(1 * time.Second)

		// Generate status event
		statusEvent := &osdp.Event{
			Event: osdp.EventStatus{
				Type:   osdp.StatusReportOutput,
				Report: []byte{1}, // Output 0 active (1 entry == OUTPUT_CONTROL num_items)
			},
		}
		err = pd.SubmitEvent(statusEvent)
		if err != nil {
			t.Errorf("Failed to send status event: %v", err)
		}

		time.Sleep(3 * time.Second)
	}()

	wg.Wait()

	// Verify events were received
	expectedEvents := []osdp.OsdpEventType{osdp.EvtCardRead, osdp.EvtKeypress, osdp.EvtStatus}
	for _, eventType := range expectedEvents {
		if count, ok := eventsReceived[eventType]; !ok || count == 0 {
			t.Errorf("Event type %d was not received", eventType)
		}
	}

	t.Logf("Event generation test completed successfully. Events received: %v", eventsReceived)
}

// TestMfgReplyLargePayload round-trips an MFGREP event whose payload exceeds the
// old 64-byte decode buffer (but stays within the 128-byte C max) through a real
// CP/PD FIFO pair. Before the decode-buffer fix, get_event_mfgrep memcpy'd 100
// bytes into a 64-byte Go slice, corrupting the heap. The byte-equality assertion
// here only guards against truncation/decode-length regressions (a shrunk buffer
// would fail this check); it does not itself detect the heap overflow. Run with
// AddressSanitizer (`go test -asan`) to catch the overflow directly.
func TestMfgReplyLargePayload(t *testing.T) {
	data := make([]byte, 100) // > old 64-byte buffer, < 128 C max
	for i := range data {
		data[i] = byte(i)
	}

	cpRx, cpTx, pdRx, pdTx, cleanup := createFIFOPair(t)
	defer cleanup()

	id := osdp.PdId{Version: 1, Model: 1, VendorCode: 0x153, SerialNumber: 0x12345, FirmwareMajor: 1}

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

	if err := pd.SubmitEvent(&osdp.Event{Event: osdp.EventMfgReply{
		VendorCode: [3]uint8{1, 2, 3}, Data: data,
	}}); err != nil {
		t.Fatalf("SubmitEvent: %v", err)
	}

	ev, err := cp.GetEvent(0, 5*time.Second)
	if err != nil {
		t.Fatalf("GetEvent: %v", err)
	}

	mfg, ok := ev.Event.(osdp.EventMfgReply)
	if !ok {
		t.Fatalf("decoded event type = %T, want EventMfgReply", ev.Event)
	}
	if len(mfg.Data) != len(data) {
		t.Fatalf("decoded Data length = %d, want %d", len(mfg.Data), len(data))
	}
	if !bytes.Equal(mfg.Data, data) {
		t.Fatalf("decoded Data = %v, want %v", mfg.Data, data)
	}
}

// Every CP-side notification, triggered for real: the link coming up, the PD's
// identity, a secure channel on the install key, and a file transfer.
func TestCPNotificationsDecodeOverTheWire(t *testing.T) {
	link := newFIFOLink(t)
	pdInfo := testPdInfo(osdp.FlagInstallMode)
	pdInfo.ID = osdp.PdId{Version: 3, Model: 7, VendorCode: 0x0A0B0C, SerialNumber: 0xCAFE, FirmwareMajor: 2, FirmwareMinor: 1, FirmwareBuild: 9}
	pd := startTestPD(t, link, pdInfo, []osdp.PdCapability{
		{Function: osdp.CapCommunicationSecurity, Compliance: 1, NumItems: 1},
	})
	pd.RegisterFileOperations(newMemFile(5, nil))

	notes := make(chan osdp.NotificationData, 32)
	// The CP holds an SCBK, so it first opens SC on the install key (SCBK-D)
	// to provision the PD.
	cpInfo := testPdInfo(osdp.FlagInstallMode | osdp.FlagEnableNotification)
	cpInfo.SCBK, _ = osdp.GenKey(16)
	cp := startTestCP(t, link, cpInfo)
	cp.SetEventHandler(func(_ int, ev *osdp.Event) int {
		if n, ok := ev.Event.(osdp.EventNotification); ok {
			notes <- n.Payload
		}
		return 0
	})
	cp.RegisterFileOperations(0, newMemFile(5, make([]byte, 300)))

	want := map[string]osdp.NotificationData{
		"link":     osdp.NotificationPDStatus{Online: true},
		"identity": osdp.NotificationPDID{ID: pdInfo.ID},
		"sc":       osdp.NotificationSCStatus{Active: true, SCBKD: true},
		"mp-start": osdp.NotificationMP{MPType: osdp.MPMsgFileTransfer, ObjectID: 5, Total: 300},
		"mp-done":  osdp.NotificationMP{MPType: osdp.MPMsgFileTransfer, ObjectID: 5, Total: 300, Offset: 300, Outcome: osdp.MPOutcomeOK},
	}
	key := func(n osdp.NotificationData) string {
		switch p := n.(type) {
		case osdp.NotificationPDStatus:
			return "link"
		case osdp.NotificationPDID:
			return "identity"
		case osdp.NotificationSCStatus:
			return "sc"
		case osdp.NotificationMP:
			if p.Outcome != osdp.MPOutcomeOK || p.Offset == p.Total {
				return "mp-done"
			}
			if p.Offset == 0 {
				return "mp-start"
			}
		}
		return ""
	}

	transferSent := false
	deadline := time.After(15 * time.Second)
	for len(want) > 0 {
		if !transferSent && cp.SCWait(0, 0) {
			cp.SubmitCommand(0, &osdp.Command{Command: osdp.CommandFileTx{ID: 5}})
			transferSent = true
		}
		select {
		case n := <-notes:
			k := key(n)
			w, ok := want[k]
			if !ok {
				continue
			}
			if n != w {
				t.Errorf("%s notification = %+v, want %+v", k, n, w)
			}
			delete(want, k)
		case <-time.After(50 * time.Millisecond):
		case <-deadline:
			t.Fatalf("never saw notifications: %v", want)
		}
	}
}

func TestMfgStatusEventsRoundTrip(t *testing.T) {
	data := []byte{9, 8, 7, 6, 5}
	statusEv := roundTripEvent(t, &osdp.Event{Event: osdp.EventMfgStatus{Data: data}})
	got, ok := statusEv.Event.(osdp.EventMfgStatus)
	if !ok {
		t.Fatalf("status decoded as %T, want EventMfgStatus", statusEv.Event)
	}
	if !bytes.Equal(got.Data, data) {
		t.Errorf("status data = %v, want %v", got.Data, data)
	}
	errEv := roundTripEvent(t, &osdp.Event{Event: osdp.EventMfgError{Data: data}})
	if _, ok := errEv.Event.(osdp.EventMfgError); !ok {
		t.Fatalf("error decoded as %T, want EventMfgError", errEv.Event)
	}
}
