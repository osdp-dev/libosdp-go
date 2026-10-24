/*
 * Copyright (c) 2026 Siddharth Chandrasekaran <sidcha@osdp.dev>
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package osdp_test

import (
	"sync"
	"testing"
	"time"

	"github.com/osdp-dev/libosdp-go"
)

// TestStatusAndMfgCommands tests STATUS and MFG commands
func TestStatusAndMfgCommands(t *testing.T) {
	cpRx, cpTx, pdRx, pdTx, cleanup := createFIFOPair(t)
	defer cleanup()

	var wg sync.WaitGroup
	wg.Add(2)

	// Track commands received
	commandsReceived := make(map[osdp.OsdpCommandType]bool)
	var commandsMutex sync.Mutex

	// Start PD
	go func() {
		defer wg.Done()

		commandHandler := func(command *osdp.Command) (int, *osdp.Event) {
			commandsMutex.Lock()
			commandsReceived[command.Command.GetType()] = true
			commandsMutex.Unlock()

			t.Logf("PD received command: %d", command.Command.GetType())

			// Handle specific commands and send appropriate response events
			switch command.Command.GetType() {
			case osdp.CmdStatus:
				// Respond with a status report event
				statusEvent := &osdp.Event{
					Event: osdp.EventStatus{
						Type:   osdp.StatusReportInput,
						Report: []byte{1, 0}, // Input 0 active, input 1 inactive (2 entries == cap num_items)
					},
				}
				return 0, statusEvent
			case osdp.CmdMfg:
				// Extract MFG command data
				if mfg, ok := command.Command.(osdp.CommandMfg); ok {
					t.Logf("PD received MFG command: VendorCode=0x%02X%02X%02X, Data=%v",
						mfg.VendorCode[2], mfg.VendorCode[1], mfg.VendorCode[0], mfg.Data)
					// Respond with MFG reply event
					mfgReply := &osdp.Event{
						Event: osdp.EventMfgReply{
							VendorCode: mfg.VendorCode,
							Data:       append(mfg.Data, []byte(" OK")...),
						},
					}
					return 0, mfgReply
				}
			}
			return 0, nil // Success
		}

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
			{Function: osdp.CapContactStatusMonitoring, Compliance: 1, NumItems: 2},
		}

		pd, err := osdp.NewPeripheralDevice(pdChannel, pdInfo, capabilities, osdp.LogInfo)
		if err != nil {
			t.Errorf("Failed to create PD: %v", err)
			return
		}
		defer pd.Stop()

		if err := pd.SetCommandHandler(commandHandler); err != nil {
			t.Errorf("Failed to set command handler: %v", err)
		}

		time.Sleep(8 * time.Second)
	}()

	// Start CP and send STATUS/MFG commands
	go func() {
		defer wg.Done()

		eventsReceived := make(map[osdp.OsdpEventType]bool)
		var eventsMutex sync.Mutex

		eventHandler := func(pdNumber int, event *osdp.Event) int {
			eventsMutex.Lock()
			eventsReceived[event.Event.GetType()] = true
			eventsMutex.Unlock()

			t.Logf("CP received event: %d from PD %d", event.Event.GetType(), pdNumber)

			// Process specific events
			switch eventData := event.Event.(type) {
			case osdp.EventStatus:
				t.Logf("Status report type: %d, report: %v", eventData.Type, eventData.Report)
			case osdp.EventMfgReply:
				t.Logf("MFG reply: VendorCode=0x%02X%02X%02X, Data=%s", eventData.VendorCode[2], eventData.VendorCode[1], eventData.VendorCode[0], string(eventData.Data))
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

		// Wait for connection
		time.Sleep(2 * time.Second)

		// Send STATUS command
		statusCmd := &osdp.Command{
			Command: osdp.CommandStatus{
				Type: osdp.StatusReportInput,
			},
		}
		err = cp.SubmitCommand(0, statusCmd)
		if err != nil {
			t.Errorf("Failed to send STATUS command: %v", err)
		}
		time.Sleep(1 * time.Second)

		// Send MFG command
		mfgCmd := &osdp.Command{
			Command: osdp.CommandMfg{
				VendorCode: [3]uint8{0x53, 0x01, 0x00}, // 0x153 in little endian
				Data:       []byte("Hello MFG"),
			},
		}
		err = cp.SubmitCommand(0, mfgCmd)
		if err != nil {
			t.Errorf("Failed to send MFG command: %v", err)
		}

		time.Sleep(3 * time.Second)

		// Check events received
		expectedEvents := []osdp.OsdpEventType{osdp.EvtStatus, osdp.EvtMfgReply}
		eventsMutex.Lock()
		for _, eventType := range expectedEvents {
			if !eventsReceived[eventType] {
				t.Errorf("Event type %d was not received", eventType)
			}
		}
		eventsMutex.Unlock()
	}()

	wg.Wait()

	// Verify commands were received
	expectedCommands := []osdp.OsdpCommandType{osdp.CmdStatus, osdp.CmdMfg}
	for _, cmdType := range expectedCommands {
		if !commandsReceived[cmdType] {
			t.Errorf("Command type %d was not received", cmdType)
		}
	}

	t.Logf("STATUS and MFG commands test completed successfully")
}

// TestSecureChannel tests secure channel establishment
func TestSecureChannel(t *testing.T) {
	cpRx, cpTx, pdRx, pdTx, cleanup := createFIFOPair(t)
	defer cleanup()

	var wg sync.WaitGroup
	wg.Add(2)

	secureEstablished := make(chan bool, 2)

	// Start PD with secure channel support
	go func() {
		defer wg.Done()

		commandHandler := func(command *osdp.Command) (int, *osdp.Event) {
			return 0, nil // Success (0 = ACK, non-zero = NAK)
		}

		pdChannel, err := osdp.NewFIFOChannel(pdRx, pdTx)
		if err != nil {
			t.Errorf("Failed to create PD FIFO channel: %v", err)
			return
		}
		defer pdChannel.Close()

		pdInfo := &osdp.PdInfo{
			Address:  1,
			BaudRate: 9600,
			Flags:    0,                                                                                                      // Default mode to allow secure channel setup
			SCBK:     []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F}, // Matching SCBK
			ID: osdp.PdId{
				Version:       1,
				Model:         1,
				VendorCode:    0x153,
				SerialNumber:  0x12345,
				FirmwareMajor: 1,
				FirmwareMinor: 0,
				FirmwareBuild: 0,
			},
		}

		capabilities := []osdp.PdCapability{
			{Function: osdp.CapCommunicationSecurity, Compliance: 1, NumItems: 1},
		}

		pd, err := osdp.NewPeripheralDevice(pdChannel, pdInfo, capabilities, osdp.LogInfo)
		if err != nil {
			t.Errorf("Failed to create PD: %v", err)
			return
		}
		defer pd.Stop()

		if err := pd.SetCommandHandler(commandHandler); err != nil {
			t.Errorf("Failed to set command handler: %v", err)
		}

		// Monitor secure channel status
		for i := 0; i < 40; i++ { // Increased attempts
			time.Sleep(250 * time.Millisecond)

			secure, err := pd.IsSCActive()
			if err == nil && secure {
				t.Log("PD: Secure channel established")
				secureEstablished <- true
				break
			}
		}
		t.Log("PD: Monitoring complete")
	}()

	// Start CP with secure channel enforcement
	go func() {
		defer wg.Done()

		eventHandler := func(pdNumber int, event *osdp.Event) int {
			return 0
		}

		cpChannel, err := osdp.NewFIFOChannel(cpRx, cpTx)
		if err != nil {
			t.Errorf("Failed to create CP FIFO channel: %v", err)
			return
		}
		defer cpChannel.Close()

		pdInfo := &osdp.PdInfo{
			Address:  1,
			BaudRate: 9600,
			Flags:    osdp.FlagEnforceSecure,
			SCBK:     []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F},
			ID: osdp.PdId{
				Version:       1,
				Model:         1,
				VendorCode:    0x153,
				SerialNumber:  0x12345,
				FirmwareMajor: 1,
				FirmwareMinor: 0,
				FirmwareBuild: 0,
			},
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

		// Monitor secure channel status
		for i := 0; i < 40; i++ { // Increased attempts
			time.Sleep(250 * time.Millisecond)

			secure, err := cp.IsSCActive(0)
			if err == nil && secure {
				t.Log("CP: Secure channel established")
				secureEstablished <- true
				break
			}
		}
		t.Log("CP: Monitoring complete")
	}()

	// Wait for secure channel establishment (at least one side)
	timeout := time.After(20 * time.Second)
	secureCount := 0

	for secureCount < 1 { // Require at least one side to confirm
		select {
		case <-secureEstablished:
			secureCount++
			t.Log("Test: Secure channel confirmed")
		case <-timeout:
			t.Error("Timeout waiting for secure channel establishment")
			return
		}
	}

	// Give time for both sides to potentially signal
	additionalTimeout := time.After(3 * time.Second)
	for secureCount < 2 {
		select {
		case <-secureEstablished:
			secureCount++
			t.Log("Test: Both sides confirmed secure channel")
		case <-additionalTimeout:
			t.Log("Test: Secure channel established (one side confirmed)")
			goto success
		}
	}

success:
	t.Log("Test: Secure channel establishment successful")
	// Don't wait for goroutines as they may be blocked on PD operations
}

func TestSecureChannelWaitHelpers(t *testing.T) {
	scbk, err := osdp.GenKey(16)
	if err != nil {
		t.Fatal(err)
	}
	link := newFIFOLink(t)
	pdInfo := testPdInfo(0)
	pdInfo.SCBK = scbk
	pd := startTestPD(t, link, pdInfo, []osdp.PdCapability{
		{Function: osdp.CapCommunicationSecurity, Compliance: 1, NumItems: 1},
	})
	cpInfo := testPdInfo(osdp.FlagEnforceSecure)
	cpInfo.SCBK = scbk
	cp := startTestCP(t, link, cpInfo)

	if !cp.OnlineWaitAll(10 * time.Second) {
		t.Fatal("OnlineWaitAll timed out")
	}
	if !cp.SCWait(0, 10*time.Second) || !cp.SCWaitAll(time.Second) {
		t.Fatal("CP never reported an active secure channel")
	}
	if !pd.SCWait(5 * time.Second) {
		t.Fatal("PD never reported an active secure channel")
	}
	if mask, err := cp.SCStatus(); err != nil || mask != 1 {
		t.Errorf("SCStatus = %#x, %v; want 0x1", mask, err)
	}
	if n, err := cp.GetNumSCActive(); err != nil || n != 1 {
		t.Errorf("GetNumSCActive = %d, %v; want 1", n, err)
	}

	pd.Stop()
	if !cp.OfflineWait(0, 15*time.Second) {
		t.Fatal("CP did not notice the PD going away")
	}
	if cp.SCWait(0, 100*time.Millisecond) {
		t.Error("SCWait reported an active channel for an offline PD")
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

	cp, err := osdp.NewControlPanelBuilder(cpChannel).
		AddPD(&osdp.PdInfo{Address: 1, BaudRate: 9600, Flags: osdp.FlagInstallMode, ID: id}).
		LogLevel(osdp.LogError).
		Build()
	if err != nil {
		t.Fatalf("Build CP: %v", err)
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
