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

	"github.com/Osprio/libosdp-go"
)

// TestMultipleCommands tests sending multiple different commands
func TestMultipleCommands(t *testing.T) {
	cpRx, cpTx, pdRx, pdTx, cleanup := createFIFOPair(t)
	defer cleanup()

	commandsReceived := make(map[osdp.OsdpCommandType]bool)
	var mu sync.Mutex

	var wg sync.WaitGroup
	wg.Add(2)

	// Start PD
	go func() {
		defer wg.Done()

		commandHandler := func(command *osdp.Command) (int, *osdp.Event) {
			mu.Lock()
			commandsReceived[command.Command.GetType()] = true
			mu.Unlock()

			t.Logf("PD: Received command type: %d", command.Command.GetType())
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
			Flags:    osdp.FlagInstallMode,
			ID: osdp.PdId{
				Version: 1, Model: 1, VendorCode: 0x153,
				SerialNumber: 0x12345, FirmwareMajor: 1,
			},
		}

		capabilities := []osdp.PdCapability{
			{Function: osdp.CapReaderLEDControl, Compliance: 1, NumItems: 2},
			{Function: osdp.CapReaderAudibleOutput, Compliance: 1, NumItems: 1},
			{Function: osdp.CapReaderTextOutput, Compliance: 1, NumItems: 1},
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

		// Keep PD running
		time.Sleep(10 * time.Second)
	}()

	// Start CP and send multiple commands
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

		// Send LED command
		ledCmd := &osdp.Command{
			Command: osdp.CommandLed{
				Reader:    0,
				LedNumber: 0,
				Permanent: osdp.LedParams{
					ControlCode: 1,
					OnCount:     10,
					OffCount:    0,
					OnColor:     osdp.LEDColorGreen,
				},
			},
		}
		cp.SubmitCommand(0, ledCmd)
		time.Sleep(1 * time.Second)

		// Send buzzer command
		buzzerCmd := &osdp.Command{
			Command: osdp.CommandBuzzer{
				Reader:      0,
				ControlCode: 2,
				OnCount:     5,
				OffCount:    5,
				RepCount:    3,
			},
		}
		cp.SubmitCommand(0, buzzerCmd)
		time.Sleep(1 * time.Second)

		// Send text command
		textCmd := &osdp.Command{
			Command: osdp.CommandText{
				Reader:      0,
				ControlCode: 1, // Permanent text display
				TempTime:    0, // Not used for permanent display
				OffsetRow:   0, // Start at row 0
				OffsetCol:   0, // Start at column 0
				Data:        []byte("Hello OSDP"),
			},
		}
		cp.SubmitCommand(0, textCmd)

		// Wait for commands to be processed
		time.Sleep(3 * time.Second)
	}()

	wg.Wait()

	// Verify all commands were received
	expectedCommands := []osdp.OsdpCommandType{osdp.CmdLED, osdp.CmdBuzzer, osdp.CmdText}
	for _, cmdType := range expectedCommands {
		if !commandsReceived[cmdType] {
			t.Errorf("Command type %d was not received", cmdType)
		}
	}

	t.Logf("Test: All %d commands received successfully", len(expectedCommands))
}

// TestComprehensiveLEDColors tests all LED colors and variations
func TestComprehensiveLEDColors(t *testing.T) {
	cpRx, cpTx, pdRx, pdTx, cleanup := createFIFOPair(t)
	defer cleanup()

	var wg sync.WaitGroup
	wg.Add(2)

	commandsReceived := 0
	var commandsMutex sync.Mutex

	// Start PD with LED capabilities
	go func() {
		defer wg.Done()

		commandHandler := func(command *osdp.Command) (int, *osdp.Event) {
			if command.Command.GetType() == osdp.CmdLED {
				commandsMutex.Lock()
				commandsReceived++
				commandsMutex.Unlock()

				if ledCmd, ok := command.Command.(osdp.CommandLed); ok {
					t.Logf("PD received LED command: LED %d, Perm Color %d", ledCmd.LedNumber, ledCmd.Permanent.OnColor)
				}
			}
			return 0, nil
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
			{Function: osdp.CapReaderLEDControl, Compliance: 1, NumItems: 4},
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

		time.Sleep(6 * time.Second)
	}()

	// Start CP and test all LED colors
	go func() {
		defer wg.Done()

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

		// Wait for connection
		time.Sleep(1 * time.Second)

		// Test all LED colors
		ledColors := []osdp.OsdpLedColor{
			osdp.LEDColorRed,
			osdp.LEDColorGreen,
			osdp.LEDColorAmber,
			osdp.LEDColorBlue,
			osdp.LEDColorMagenta,
			osdp.LEDColorCyan,
			osdp.LEDColorWhite,
		}

		for i, color := range ledColors {
			ledCmd := &osdp.Command{
				Command: osdp.CommandLed{
					Reader:    0,
					LedNumber: uint8(i % 4), // Use LEDs 0-3
					Permanent: osdp.LedParams{
						ControlCode: 1,
						OnCount:     10,
						OffCount:    0,
						OnColor:     color,
					},
				},
			}
			err = cp.SubmitCommand(0, ledCmd)
			if err != nil {
				t.Errorf("Failed to send LED command for color %d: %v", color, err)
			}
			time.Sleep(200 * time.Millisecond)
		}

		time.Sleep(2 * time.Second)
	}()

	wg.Wait()

	expectedCommands := len([]osdp.OsdpLedColor{osdp.LEDColorRed, osdp.LEDColorGreen, osdp.LEDColorAmber, osdp.LEDColorBlue, osdp.LEDColorMagenta, osdp.LEDColorCyan, osdp.LEDColorWhite})
	if commandsReceived != expectedCommands {
		t.Errorf("Expected %d LED commands, received %d", expectedCommands, commandsReceived)
	}

	t.Logf("All LED colors tested successfully: %d commands processed", commandsReceived)
}
