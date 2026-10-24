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

// TestCommandTextEmbeddedNUL verifies that a text command whose payload contains
// an embedded NUL byte round-trips through the CP->PD command path without being
// truncated at the first NUL (as C.GoString would do).
func TestCommandTextEmbeddedNUL(t *testing.T) {
	cpRx, cpTx, pdRx, pdTx, cleanup := createFIFOPair(t)
	defer cleanup()

	in := []byte{'A', 0x00, 'B', 'C'}
	id := osdp.PdId{Version: 1, Model: 1, VendorCode: 0x153, SerialNumber: 0x12345, FirmwareMajor: 1}

	got := make(chan []byte, 1)

	pdChannel, err := osdp.NewFIFOChannel(pdRx, pdTx)
	if err != nil {
		t.Fatalf("PD channel: %v", err)
	}
	defer pdChannel.Close()

	pd, err := osdp.NewPeripheralDevice(pdChannel,
		&osdp.PdInfo{Address: 1, BaudRate: 9600, Flags: osdp.FlagInstallMode, ID: id,
			Capabilities: []osdp.PdCapability{{Function: osdp.CapReaderTextOutput, Compliance: 1, NumItems: 1}}},
		osdp.LogError)
	if err != nil {
		t.Fatalf("NewPeripheralDevice: %v", err)
	}
	defer pd.Stop()

	if err := pd.SetCommandHandler(func(command *osdp.Command) (int, *osdp.Event) {
		if text, ok := command.Command.(osdp.CommandText); ok {
			cp := make([]byte, len(text.Data))
			copy(cp, text.Data)
			select {
			case got <- cp:
			default:
			}
		}
		return 0, nil
	}); err != nil {
		t.Fatalf("SetCommandHandler: %v", err)
	}

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

	textCmd := &osdp.Command{Command: osdp.CommandText{
		Reader: 0, ControlCode: 1, TempTime: 0, OffsetRow: 1, OffsetCol: 1, Data: in,
	}}
	if err := cp.SubmitCommand(0, textCmd); err != nil {
		t.Fatalf("SubmitCommand: %v", err)
	}

	select {
	case data := <-got:
		if !bytes.Equal(data, in) {
			t.Errorf("decoded text Data = %q (len %d), want %q (len %d)", data, len(data), in, len(in))
		}
	case <-time.After(5 * time.Second):
		t.Fatal("PD did not receive the text command")
	}
}

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

		pdInfo.Capabilities = capabilities
		pd, err := osdp.NewPeripheralDevice(pdChannel, pdInfo, osdp.LogInfo)
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

		pdInfo.Capabilities = capabilities
		pd, err := osdp.NewPeripheralDevice(pdChannel, pdInfo, osdp.LogInfo)
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

		cp, err := osdp.NewControlPanelBuilder(cpChannel).AddPD(pdInfo).Build()
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

func TestEncodeRejectsOverLengthPayload(t *testing.T) {
	tooBig := bytes.Repeat([]byte{0xAB}, 1000) // exceeds every command/event max
	if _, err := osdp.RoundTripCommand(&osdp.Command{Command: osdp.CommandMfg{VendorCode: [3]uint8{1, 2, 3}, Data: tooBig}}); err == nil {
		t.Error("osdp.RoundTripCommand(CommandMfg oversize): want error, got nil")
	}
	if _, err := osdp.RoundTripEvent(&osdp.Event{Event: osdp.EventKeyPress{ReaderNo: 0, Data: tooBig}}); err == nil {
		t.Error("osdp.RoundTripEvent(EventKeyPress oversize): want error, got nil")
	}
}

func TestMfgCommandRoundTrip(t *testing.T) {
	in := osdp.CommandMfg{VendorCode: [3]uint8{0x11, 0x22, 0x33}, Data: []byte{1, 2, 3, 4}}
	got, ok := roundTripCommand(t, &osdp.Command{Command: in}).Command.(osdp.CommandMfg)
	if !ok {
		t.Fatalf("decoded type mismatch")
	}
	if got.VendorCode != in.VendorCode || !bytes.Equal(got.Data, in.Data) {
		t.Errorf("round-trip = %+v, want %+v", got, in)
	}
}

func TestTDSetRoundTrip(t *testing.T) {
	in := osdp.CommandTDSet{Year: 2026, Month: 7, Day: 23, Hour: 14, Minute: 5, Second: 9}
	got, ok := roundTripCommand(t, &osdp.Command{Command: in}).Command.(osdp.CommandTDSet)
	if !ok || got != in {
		t.Fatalf("round-trip = %+v, want %+v", got, in)
	}
}

func TestEncodeAcceptsEmptyPayload(t *testing.T) {
	got := roundTripCommand(t, &osdp.Command{Command: osdp.CommandMfg{VendorCode: [3]uint8{1, 2, 3}, Data: nil}})
	mfg, ok := got.Command.(osdp.CommandMfg)
	if !ok {
		t.Fatalf("decoded type = %T, want CommandMfg", got.Command)
	}
	if len(mfg.Data) != 0 {
		t.Errorf("empty mfg Data round-tripped to len %d", len(mfg.Data))
	}
}
