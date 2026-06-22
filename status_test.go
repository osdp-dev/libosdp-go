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
							Reply:      mfg.Command,
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

		cp, err := osdp.NewControlPanel(cpChannel, []*osdp.PdInfo{pdInfo}, osdp.LogInfo)
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
				Command:    0x01,
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
