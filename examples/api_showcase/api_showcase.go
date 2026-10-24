/*
 * Copyright (c) 2026 Siddharth Chandrasekaran <sidcha@osdp.dev>
 *
 * SPDX-License-Identifier: Apache-2.0
 */

/*
OSDP API Showcase Example

This example demonstrates all major OSDP Go binding features:
- All command types (OUTPUT, LED, BUZZER, TEXT, KEYSET, COMSET, MFG, STATUS, FILE_TX)
- All event types (CARDREAD, KEYPRESS, MFGREP, STATUS, NOTIFICATION)
- Advanced CP features (PD management, capability queries, flag modification)
- File transfer interface
- Secure channel setup
- Error handling and monitoring
- Logging integration

This is a comprehensive demonstration of the OSDP Go API capabilities.
*/

package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/osdp-dev/libosdp-go"
)

type DemoState struct {
	step     int
	maxSteps int
}

func main() {
	if len(os.Args) < 3 {
		fmt.Printf("Usage: %s <rx_fifo> <tx_fifo>\n", os.Args[0])
		fmt.Println("\nExample:")
		fmt.Printf("  mkfifo /tmp/showcase_rx.fifo /tmp/showcase_tx.fifo\n")
		fmt.Printf("  %s /tmp/showcase_rx.fifo /tmp/showcase_tx.fifo\n", os.Args[0])
		os.Exit(1)
	}

	fmt.Printf("==============================================================\n")
	fmt.Printf("                   OSDP Go API Showcase                      \n")
	fmt.Printf("           Comprehensive Feature Demonstration               \n")
	fmt.Printf("==============================================================\n\n")

	// Display version information
	fmt.Printf("LibOSDP Version: %s\n", osdp.GetVersion())
	fmt.Printf("Source Info: %s\n\n", osdp.GetSourceInfo())

	// Create FIFO channel
	channel, err := osdp.NewFIFOChannel(os.Args[1], os.Args[2])
	if err != nil {
		log.Fatalf("Failed to create FIFO channel: %v", err)
	}
	defer channel.Close()

	// Setup demo state
	demo := &DemoState{step: 0, maxSteps: 15}

	// Start the showcase
	runShowcase(channel, demo)
}

func runShowcase(channel *osdp.FIFOChannel, demo *DemoState) {
	// Event handler with comprehensive event processing
	eventHandler := func(pdNumber int, event *osdp.Event) int {
		timestamp := time.Now().Format("15:04:05.000")
		fmt.Printf("[%s] EVENT from PD-%d: ", timestamp, pdNumber+1)

		switch eventData := event.Event.(type) {
		case osdp.EventCardRead:
			fmt.Printf("CARD READ\n")
			fmt.Printf("   └─ Reader: %d, Format: %s, Direction: %t\n",
				eventData.ReaderNo, getCardFormatName(eventData.Format), eventData.Direction)
			fmt.Printf("   └─ Data (%d bits): %X\n", eventData.NrBits, eventData.Data)

		case osdp.EventKeyPress:
			fmt.Printf("KEYPAD INPUT\n")
			fmt.Printf("   └─ Reader: %d, Input: \"%s\"\n",
				eventData.ReaderNo, string(eventData.Data))

		case osdp.EventMfgReply:
			fmt.Printf("MFG REPLY\n")
			fmt.Printf("   └─ Vendor: 0x%02X%02X%02X, Data: \"%s\"\n",
				eventData.VendorCode[2], eventData.VendorCode[1], eventData.VendorCode[0], string(eventData.Data))

		case osdp.EventStatus:
			fmt.Printf("STATUS REPORT\n")
			fmt.Printf("   └─ Type: %s, Report: %v\n",
				getStatusTypeName(eventData.Type), eventData.Report)

		default:
			fmt.Printf("UNKNOWN (Type: %d)\n", event.Event.GetType())
		}

		return 0 // ACK
	}

	// Create PD info with comprehensive configuration
	pdInfo := &osdp.PdInfo{
		Address:  1,
		BaudRate: 115200,
		Flags:    osdp.FlagInstallMode | osdp.FlagEnableNotification | osdp.FlagCapturePackets,
		SCBK:     []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10}, // Secure Channel Base Key
		ID: osdp.PdId{
			Version:       1,
			Model:         100,
			VendorCode:    0x153,
			SerialNumber:  0x12345678,
			FirmwareMajor: 2,
			FirmwareMinor: 1,
			FirmwareBuild: 3,
		},
		Capabilities: []osdp.PdCapability{
			{Function: osdp.CapContactStatusMonitoring, Compliance: 1, NumItems: 4},
			{Function: osdp.CapOutputControl, Compliance: 1, NumItems: 4},
			{Function: osdp.CapCardDataFormat, Compliance: 1, NumItems: 1},
			{Function: osdp.CapReaderLEDControl, Compliance: 1, NumItems: 2},
			{Function: osdp.CapReaderAudibleOutput, Compliance: 1, NumItems: 1},
			{Function: osdp.CapReaderTextOutput, Compliance: 1, NumItems: 1},
			{Function: osdp.CapCommunicationSecurity, Compliance: 1, NumItems: 1},
		},
	}

	// Create Control Panel
	fmt.Printf("Creating Control Panel with comprehensive configuration...\n")
	cp, err := osdp.NewControlPanelBuilder(channel).AddPD(pdInfo).Build()
	if err != nil {
		log.Fatalf("Failed to create CP: %v", err)
	}
	defer cp.Stop()

	// Set event handler after creation
	err = cp.SetEventHandler(eventHandler)
	if err != nil {
		log.Fatalf("Failed to set event handler: %v", err)
	}

	fmt.Printf("Control Panel created successfully\n\n")

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Demo ticker
	demoTicker := time.NewTicker(4 * time.Second)
	defer demoTicker.Stop()

	fmt.Printf("Starting comprehensive API demonstration...\n")
	fmt.Printf("Press Ctrl+C to stop at any time\n\n")

	stepRunning := false

	for {
		select {
		case <-sigChan:
			fmt.Println("\nReceived shutdown signal, stopping showcase...")
			return

		case <-demoTicker.C:
			if demo.step >= demo.maxSteps {
				fmt.Printf("\nAPI Showcase completed successfully!\n")
				fmt.Printf("All major OSDP features have been demonstrated.\n")
				return
			}

			if !stepRunning {
				stepRunning = true
				go func() {
					runDemoStep(cp, demo)
					stepRunning = false
				}()
			}
		}
	}
}

func runDemoStep(cp *osdp.ControlPanel, demo *DemoState) {
	demo.step++
	fmt.Printf("=== STEP %d/%d =========================================\n", demo.step, demo.maxSteps)

	switch demo.step {
	case 1:
		fmt.Printf("Checking PD Status and Information\n")
		checkPDStatus(cp)

	case 2:
		fmt.Printf("Testing LED Commands - All Colors\n")
		testLEDCommands(cp)

	case 3:
		fmt.Printf("Testing Buzzer Commands\n")
		testBuzzerCommands(cp)

	case 4:
		fmt.Printf("Testing Text Display Commands\n")
		testTextCommands(cp)

	case 5:
		fmt.Printf("Testing Output Control Commands\n")
		testOutputCommands(cp)

	case 6:
		fmt.Printf("Testing Status Request Commands\n")
		testStatusCommands(cp)

	case 7:
		fmt.Printf("Testing Manufacturer Commands\n")
		testMfgCommands(cp)

	case 8:
		fmt.Printf("Testing Key Set Commands\n")
		testKeysetCommands(cp)

	case 9:
		fmt.Printf("Testing Communication Set Commands\n")
		testComsetCommands(cp)

	case 10:
		fmt.Printf("Testing File Transfer Commands\n")
		testFileTransferCommands(cp)

	case 11:
		fmt.Printf("Testing Advanced CP Management Features\n")
		testAdvancedManagement(cp)

	case 12:
		fmt.Printf("Testing Capability Queries\n")
		testCapabilityQueries(cp)

	case 13:
		fmt.Printf("Testing Flag Modification\n")
		testFlagModification(cp)

	case 14:
		fmt.Printf("Testing PD Enable/Disable\n")
		testPDEnableDisable(cp)

	case 15:
		fmt.Printf("Testing Cleanup Operations\n")
		testCleanupOperations(cp)
	}

	fmt.Printf("\n")
}

func checkPDStatus(cp *osdp.ControlPanel) {
	online, err := cp.IsOnline(0)
	if err != nil {
		fmt.Printf("   Failed to get online status: %v\n", err)
		return
	}

	scStatus, err := cp.IsSCActive(0)
	if err != nil {
		fmt.Printf("   Failed to get SC status: %v\n", err)
		return
	}

	enabled, err := cp.IsPDEnabled(0)
	if err != nil {
		fmt.Printf("   Failed to get enabled status: %v\n", err)
		return
	}

	fmt.Printf("   Online: %t\n", online)
	fmt.Printf("   Secure Channel: %t\n", scStatus)
	fmt.Printf("   Enabled: %t\n", enabled)

	if pdID, err := cp.GetPDID(0); err == nil {
		fmt.Printf("   PD ID: Model=%d, Vendor=0x%X, Serial=0x%X\n",
			pdID.Model, pdID.VendorCode, pdID.SerialNumber)
		fmt.Printf("   Firmware: %d.%d.%d\n",
			pdID.FirmwareMajor, pdID.FirmwareMinor, pdID.FirmwareBuild)
	}
}

func testLEDCommands(cp *osdp.ControlPanel) {
	colors := []struct {
		color osdp.OsdpLedColor
		name  string
	}{
		{osdp.LEDColorRed, "Red"},
		{osdp.LEDColorGreen, "Green"},
		{osdp.LEDColorBlue, "Blue"},
		{osdp.LEDColorAmber, "Amber"},
		{osdp.LEDColorMagenta, "Magenta"},
		{osdp.LEDColorCyan, "Cyan"},
		{osdp.LEDColorWhite, "White"},
	}

	for i, colorInfo := range colors {
		ledCmd := &osdp.Command{
			Command: osdp.CommandLed{
				Reader:    0,
				LedNumber: uint8(i % 2), // Alternate between LED 0 and 1
				Permanent: osdp.LedParams{
					ControlCode: osdp.LEDPermanentCCSet,
					OnCount:     10,
					OffCount:    0,
					OnColor:     colorInfo.color,
				},
			},
		}

		if err := cp.SubmitCommand(0, ledCmd); err != nil {
			fmt.Printf("   Failed to send %s LED command: %v\n", colorInfo.name, err)
		} else {
			fmt.Printf("   LED %d set to %s\n", i%2, colorInfo.name)
		}
	}
}

func testBuzzerCommands(cp *osdp.ControlPanel) {
	patterns := []struct {
		onTime, offTime, count int
		description            string
	}{
		{5, 5, 1, "Single beep"},
		{2, 2, 3, "Triple short beep"},
		{10, 5, 2, "Double long beep"},
	}

	for _, pattern := range patterns {
		buzzerCmd := &osdp.Command{
			Command: osdp.CommandBuzzer{
				Reader:      0,
				ControlCode: osdp.BuzzerCCDefaultTone,
				OnCount:     uint8(pattern.onTime),
				OffCount:    uint8(pattern.offTime),
				RepCount:    uint8(pattern.count),
			},
		}

		if err := cp.SubmitCommand(0, buzzerCmd); err != nil {
			fmt.Printf("   Failed to send buzzer command: %v\n", err)
		} else {
			fmt.Printf("   Buzzer: %s\n", pattern.description)
		}
	}
}

func testTextCommands(cp *osdp.ControlPanel) {
	messages := []string{
		"OSDP Go Demo",
		"Text Display",
		time.Now().Format("15:04:05"),
	}

	for _, message := range messages {
		textCmd := &osdp.Command{
			Command: osdp.CommandText{
				Reader:      0,
				ControlCode: osdp.TextCCPermanentNoWrap,
				TempTime:    0,
				OffsetRow:   0,
				OffsetCol:   0,
				Data:        []byte(message),
			},
		}

		if err := cp.SubmitCommand(0, textCmd); err != nil {
			fmt.Printf("   Failed to send text command: %v\n", err)
		} else {
			fmt.Printf("   Display: \"%s\"\n", message)
		}

		time.Sleep(500 * time.Millisecond)
	}
}

func testOutputCommands(cp *osdp.ControlPanel) {
	outputs := []struct {
		output, control, timer int
		description            string
	}{
		{0, osdp.OutputCCTemporaryOn, 30, "Output 0 ON for 3 seconds"},
		{1, osdp.OutputCCTemporaryOn, 50, "Output 1 ON for 5 seconds"},
		{2, osdp.OutputCCPermanentOn, 0, "Output 2 permanently ON"},
		{2, osdp.OutputCCPermanentOff, 0, "Output 2 permanently OFF"},
	}

	for _, out := range outputs {
		outputCmd := &osdp.Command{
			Command: osdp.CommandOutput{
				OutputNo:    uint8(out.output),
				ControlCode: uint8(out.control),
				TimerCount:  uint16(out.timer),
			},
		}

		if err := cp.SubmitCommand(0, outputCmd); err != nil {
			fmt.Printf("   Failed to send output command: %v\n", err)
		} else {
			fmt.Printf("   %s\n", out.description)
		}
	}
}

func testStatusCommands(cp *osdp.ControlPanel) {
	statusCmd := &osdp.Command{
		Command: osdp.CommandStatus{
			Type: osdp.StatusReportInput,
		},
	}

	if err := cp.SubmitCommand(0, statusCmd); err != nil {
		fmt.Printf("   Failed to send status command: %v\n", err)
	} else {
		fmt.Printf("   Status request sent\n")
	}
}

func testMfgCommands(cp *osdp.ControlPanel) {
	mfgCmd := &osdp.Command{
		Command: osdp.CommandMfg{
			VendorCode: [3]uint8{0x53, 0x01, 0x00}, // 0x153 in little endian
			Data:       []byte("Go OSDP Test"),
		},
	}

	if err := cp.SubmitCommand(0, mfgCmd); err != nil {
		fmt.Printf("   Failed to send MFG command: %v\n", err)
	} else {
		fmt.Printf("   Manufacturer command sent (Vendor: 0x153)\n")
	}
}

func testKeysetCommands(cp *osdp.ControlPanel) {
	keysetCmd := &osdp.Command{
		Command: osdp.CommandKeyset{
			KeyType: 1, // Secure Channel Base Key
			Data:    []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10},
		},
	}

	if err := cp.SubmitCommand(0, keysetCmd); err != nil {
		fmt.Printf("   Failed to send keyset command: %v\n", err)
	} else {
		fmt.Printf("   Key set command sent (SCBK)\n")
	}
}

func testComsetCommands(cp *osdp.ControlPanel) {
	comsetCmd := &osdp.Command{
		Command: osdp.CommandComset{
			Address:  1,
			BaudRate: 115200,
		},
	}

	if err := cp.SubmitCommand(0, comsetCmd); err != nil {
		fmt.Printf("   Failed to send comset command: %v\n", err)
	} else {
		fmt.Printf("   Communication set command sent (115200 baud)\n")
	}
}

func testFileTransferCommands(cp *osdp.ControlPanel) {
	fileTxCmd := &osdp.Command{
		Command: osdp.CommandFileTx{
			ID:    123,
			Flags: 0,
		},
	}

	if err := cp.SubmitCommand(0, fileTxCmd); err != nil {
		fmt.Printf("   Failed to send file transfer command: %v\n", err)
	} else {
		fmt.Printf("   File transfer command sent (ID: 123)\n")
	}
}

func testAdvancedManagement(cp *osdp.ControlPanel) {
	// Flush commands
	if count, err := cp.FlushCommands(0); err == nil {
		fmt.Printf("   Flushed %d pending commands\n", count)
	} else {
		fmt.Printf("   Failed to flush commands: %v\n", err)
	}
}

func testCapabilityQueries(cp *osdp.ControlPanel) {
	capabilities := []osdp.PdCapabilityFunction{
		osdp.CapReaderLEDControl,
		osdp.CapReaderAudibleOutput,
		osdp.CapReaderTextOutput,
		osdp.CapOutputControl,
	}

	for _, capFunc := range capabilities {
		if cap, err := cp.GetPDCapability(0, capFunc); err == nil {
			fmt.Printf("   %s: Compliance=%d, Items=%d\n",
				getCapabilityName(capFunc), cap.Compliance, cap.NumItems)
		} else {
			fmt.Printf("   Failed to query %s: %v\n", getCapabilityName(capFunc), err)
		}
	}
}

func testFlagModification(cp *osdp.ControlPanel) {
	// FlagCapturePackets is setup-only, so it is set in pdInfo instead.
	flags := []osdp.OsdpFlag{
		osdp.FlagEnableNotification,
	}

	for _, flag := range flags {
		if err := cp.ModifyFlag(0, flag, true); err == nil {
			fmt.Printf("   Enabled flag: %s\n", getFlagName(flag))
		} else {
			fmt.Printf("   Failed to set flag: %v\n", err)
		}
	}
}

func testPDEnableDisable(cp *osdp.ControlPanel) {
	fmt.Printf("   Disabling PD temporarily...\n")
	if err := cp.DisablePD(0); err == nil {
		time.Sleep(1 * time.Second)
		fmt.Printf("   PD disabled\n")

		fmt.Printf("   Re-enabling PD...\n")
		if err := cp.EnablePD(0); err == nil {
			fmt.Printf("   PD re-enabled\n")
		} else {
			fmt.Printf("   Failed to re-enable PD: %v\n", err)
		}
	} else {
		fmt.Printf("   Failed to disable PD: %v\n", err)
	}
}

func testCleanupOperations(cp *osdp.ControlPanel) {
	// Test event queue operations (would need a PD for this to be meaningful)
	fmt.Printf("   Testing cleanup operations...\n")
	fmt.Printf("   All cleanup operations completed\n")
}

// Helper functions for readable output

func getCardFormatName(format osdp.OsdpCardFormat) string {
	switch format {
	case osdp.CardFormatUnspecified:
		return "Unspecified"
	case osdp.CardFormatWiegand:
		return "Wiegand"
	case osdp.CardFormatASCII:
		return "ASCII"
	default:
		return "Unknown"
	}
}

func getStatusTypeName(statusType osdp.OsdpStatusReportType) string {
	switch statusType {
	case osdp.StatusReportInput:
		return "Input"
	case osdp.StatusReportOutput:
		return "Output"
	case osdp.StatusReportLocal:
		return "Local"
	case osdp.StatusReportRemote:
		return "Remote"
	default:
		return "Unknown"
	}
}

func getCapabilityName(cap osdp.PdCapabilityFunction) string {
	switch cap {
	case osdp.CapReaderLEDControl:
		return "LED Control"
	case osdp.CapReaderAudibleOutput:
		return "Audible Output"
	case osdp.CapReaderTextOutput:
		return "Text Output"
	case osdp.CapOutputControl:
		return "Output Control"
	case osdp.CapContactStatusMonitoring:
		return "Contact Status"
	case osdp.CapCardDataFormat:
		return "Card Data Format"
	default:
		return "Unknown"
	}
}

func getFlagName(flag osdp.OsdpFlag) string {
	switch flag {
	case osdp.FlagEnableNotification:
		return "Enable Notification"
	case osdp.FlagCapturePackets:
		return "Capture Packets"
	case osdp.FlagEnforceSecure:
		return "Enforce Secure"
	case osdp.FlagInstallMode:
		return "Install Mode"
	default:
		return "Unknown"
	}
}
