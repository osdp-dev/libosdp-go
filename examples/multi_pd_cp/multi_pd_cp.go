/*
 * Copyright (c) 2026 Siddharth Chandrasekaran <sidcha@osdp.dev>
 *
 * SPDX-License-Identifier: Apache-2.0
 */

/*
Multi-PD Control Panel Example

This example demonstrates how to create a Control Panel that manages multiple
Peripheral Devices simultaneously. It shows:
- Managing 3 different PDs with different capabilities
- Sending commands to specific PDs
- Handling events from multiple PDs
- Monitoring individual PD status
- Using different communication channels
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

func main() {
	if len(os.Args) < 4 {
		fmt.Printf("Usage: %s <pd1_rx_fifo> <pd1_tx_fifo> <pd2_rx_fifo> <pd2_tx_fifo> <pd3_rx_fifo> <pd3_tx_fifo>\n", os.Args[0])
		fmt.Println("\nExample:")
		fmt.Printf("  mkfifo /tmp/pd1_rx.fifo /tmp/pd1_tx.fifo /tmp/pd2_rx.fifo /tmp/pd2_tx.fifo /tmp/pd3_rx.fifo /tmp/pd3_tx.fifo\n")
		fmt.Printf("  %s /tmp/pd1_rx.fifo /tmp/pd1_tx.fifo /tmp/pd2_rx.fifo /tmp/pd2_tx.fifo /tmp/pd3_rx.fifo /tmp/pd3_tx.fifo\n", os.Args[0])
		os.Exit(1)
	}

	// Create channels for 3 PDs
	channels := make([]*osdp.FIFOChannel, 3)
	var err error

	for i := 0; i < 3; i++ {
		channels[i], err = osdp.NewFIFOChannel(os.Args[1+i*2], os.Args[2+i*2])
		if err != nil {
			log.Fatalf("Failed to create FIFO channel for PD %d: %v", i+1, err)
		}
		defer channels[i].Close()
	}

	// Configure multiple PDs with different capabilities
	pdInfoList := []*osdp.PdInfo{
		// PD 1: Door Reader with LED, Buzzer, and Input monitoring
		{
			Address:  1,
			BaudRate: 9600,
			Flags:    osdp.FlagInstallMode,
			ID: osdp.PdId{
				Version: 1, Model: 1, VendorCode: 0x153,
				SerialNumber: 0x11111, FirmwareMajor: 1, FirmwareMinor: 0, FirmwareBuild: 1,
			},
		},
		// PD 2: Display Terminal with Text Output and Advanced Features
		{
			Address:  2,
			BaudRate: 9600,
			Flags:    osdp.FlagInstallMode,
			ID: osdp.PdId{
				Version: 1, Model: 2, VendorCode: 0x153,
				SerialNumber: 0x22222, FirmwareMajor: 1, FirmwareMinor: 1, FirmwareBuild: 0,
			},
		},
		// PD 3: Output Controller with Multiple Outputs
		{
			Address:  3,
			BaudRate: 9600,
			Flags:    osdp.FlagInstallMode,
			ID: osdp.PdId{
				Version: 1, Model: 3, VendorCode: 0x153,
				SerialNumber: 0x33333, FirmwareMajor: 2, FirmwareMinor: 0, FirmwareBuild: 0,
			},
		},
	}

	// Event handler that processes events from all PDs
	eventHandler := func(pdNumber int, event *osdp.Event) int {
		timestamp := time.Now().Format("15:04:05")
		fmt.Printf("[%s] PD-%d EVENT: ", timestamp, pdNumber+1)

		switch eventData := event.Event.(type) {
		case osdp.EventCardRead:
			fmt.Printf("CARD READ - Reader: %d, Format: %d, Data: %X\n",
				eventData.ReaderNo, eventData.Format, eventData.Data)
		case osdp.EventKeyPress:
			fmt.Printf("KEYPAD - Reader: %d, Data: %s\n",
				eventData.ReaderNo, string(eventData.Data))
		case osdp.EventMfgReply:
			fmt.Printf("MFG REPLY - Vendor: 0x%02X%02X%02X, Data: %s\n",
				eventData.VendorCode[2], eventData.VendorCode[1], eventData.VendorCode[0], string(eventData.Data))
		case osdp.EventStatus:
			fmt.Printf("STATUS - Type: %d, Report: %v\n",
				eventData.Type, eventData.Report)
		default:
			fmt.Printf("Type: %d (Unknown)\n", event.Event.GetType())
		}

		return 0 // ACK
	}

	// Create Control Panel
	fmt.Println("Creating Multi-PD Control Panel...")
	cp, err := osdp.NewControlPanel(channels[0], pdInfoList, osdp.LogInfo)
	if err != nil {
		log.Fatalf("Failed to create Control Panel: %v", err)
	}
	defer cp.Stop()

	// Set event handler after creation
	err = cp.SetEventHandler(eventHandler)
	if err != nil {
		log.Fatalf("Failed to set event handler: %v", err)
	}

	fmt.Printf("Multi-PD Control Panel started with %d PDs\n", len(pdInfoList))
	fmt.Println("=================================================")

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Status monitoring ticker
	statusTicker := time.NewTicker(5 * time.Second)
	defer statusTicker.Stop()

	// Command sending ticker
	commandTicker := time.NewTicker(3 * time.Second)
	defer commandTicker.Stop()

	commandIndex := 0
	colors := []osdp.OsdpLedColor{osdp.LEDColorRed, osdp.LEDColorGreen, osdp.LEDColorBlue, osdp.LEDColorAmber}

	for {
		select {
		case <-sigChan:
			fmt.Println("\nReceived shutdown signal, stopping...")
			return

		case <-statusTicker.C:
			// Check status of all PDs
			fmt.Println("\n--- PD Status Report ---")
			for i := 0; i < len(pdInfoList); i++ {
				online, _ := cp.IsOnline(i)
				scActive, _ := cp.IsSCActive(i)
				enabled, _ := cp.IsPDEnabled(i)

				status := "OFFLINE"
				if online {
					if scActive {
						status = "ONLINE (SC)"
					} else {
						status = "ONLINE"
					}
				}

				fmt.Printf("  PD-%d: %s (Enabled: %t)\n", i+1, status, enabled)

				// Try to get PD ID information
				if online {
					if pdID, err := cp.GetPDID(i); err == nil {
						fmt.Printf("    ID: Model=%d, Vendor=0x%X, Serial=0x%X, FW=%d.%d.%d\n",
							pdID.Model, pdID.VendorCode, pdID.SerialNumber,
							pdID.FirmwareMajor, pdID.FirmwareMinor, pdID.FirmwareBuild)
					}
				}
			}

		case <-commandTicker.C:
			// Send different commands to different PDs in rotation
			pdIndex := commandIndex % len(pdInfoList)
			color := colors[commandIndex%len(colors)]

			switch commandIndex % 4 {
			case 0:
				// Send LED command
				ledCmd := &osdp.Command{
					Command: osdp.CommandLed{
						Reader:    0,
						LedNumber: 0,
						Permanent: osdp.LedParams{
							ControlCode: 1,
							OnCount:     10,
							OffCount:    0,
							OnColor:     color,
						},
					},
				}
				err := cp.SubmitCommand(pdIndex, ledCmd)
				if err != nil {
					fmt.Printf("Failed to send LED command to PD-%d: %v\n", pdIndex+1, err)
				} else {
					fmt.Printf("→ PD-%d: LED set to %s\n", pdIndex+1, getColorName(color))
				}

			case 1:
				// Send Buzzer command
				buzzerCmd := &osdp.Command{
					Command: osdp.CommandBuzzer{
						Reader:      0,
						ControlCode: 2, // Default tone
						OnCount:     2, // 200ms
						OffCount:    1, // 100ms
						RepCount:    3, // 3 beeps
					},
				}
				err := cp.SubmitCommand(pdIndex, buzzerCmd)
				if err != nil {
					fmt.Printf("Failed to send Buzzer command to PD-%d: %v\n", pdIndex+1, err)
				} else {
					fmt.Printf("→ PD-%d: Buzzer activated (3 beeps)\n", pdIndex+1)
				}

			case 2:
				// Send Text command (if PD supports it)
				if pdIndex == 1 { // PD-2 is our display terminal
					textCmd := &osdp.Command{
						Command: osdp.CommandText{
							Reader:      0,
							ControlCode: 1, // Permanent text
							TempTime:    0,
							OffsetRow:   0,
							OffsetCol:   0,
							Data:        []byte(fmt.Sprintf("Time: %s", time.Now().Format("15:04:05"))),
						},
					}
					err := cp.SubmitCommand(pdIndex, textCmd)
					if err != nil {
						fmt.Printf("Failed to send Text command to PD-%d: %v\n", pdIndex+1, err)
					} else {
						fmt.Printf("→ PD-%d: Display updated with current time\n", pdIndex+1)
					}
				}

			case 3:
				// Send Output command (if PD supports it)
				if pdIndex == 2 { // PD-3 is our output controller
					outputCmd := &osdp.Command{
						Command: osdp.CommandOutput{
							OutputNo:    uint8(commandIndex % 2), // Alternate between outputs 0 and 1
							ControlCode: 5,                       // Temporary ON
							TimerCount:  50,                      // 5 seconds (units of 100ms)
						},
					}
					err := cp.SubmitCommand(pdIndex, outputCmd)
					if err != nil {
						fmt.Printf("Failed to send Output command to PD-%d: %v\n", pdIndex+1, err)
					} else {
						fmt.Printf("→ PD-%d: Output %d activated for 5 seconds\n", pdIndex+1, commandIndex%2)
					}
				}
			}

			commandIndex++

			// Demonstrate advanced CP features occasionally
			if commandIndex%10 == 0 {
				fmt.Println("\n--- Advanced Features Demo ---")

				// Flush commands for a PD
				if count, err := cp.FlushCommands(0); err == nil {
					fmt.Printf("Flushed %d pending commands for PD-1\n", count)
				}

				// Modify flags (example)
				if err := cp.ModifyFlag(1, osdp.FlagEnableNotification, true); err == nil {
					fmt.Println("Enabled notifications for PD-2")
				}

				// Query capabilities
				if cap, err := cp.GetPDCapability(0, osdp.CapReaderLEDControl); err == nil {
					fmt.Printf("PD-1 LED capability: Compliance=%d, NumItems=%d\n", cap.Compliance, cap.NumItems)
				}
			}

			// Demonstrate PD management occasionally
			if commandIndex%15 == 0 {
				fmt.Println("\n--- PD Management Demo ---")

				// Temporarily disable and re-enable PD-3
				fmt.Println("Temporarily disabling PD-3...")
				if err := cp.DisablePD(2); err == nil {
					time.Sleep(2 * time.Second)
					fmt.Println("Re-enabling PD-3...")
					cp.EnablePD(2)
				}
			}
		}
	}
}

func getColorName(color osdp.OsdpLedColor) string {
	switch color {
	case osdp.LEDColorRed:
		return "Red"
	case osdp.LEDColorGreen:
		return "Green"
	case osdp.LEDColorBlue:
		return "Blue"
	case osdp.LEDColorAmber:
		return "Amber"
	case osdp.LEDColorMagenta:
		return "Magenta"
	case osdp.LEDColorCyan:
		return "Cyan"
	case osdp.LEDColorWhite:
		return "White"
	default:
		return "Unknown"
	}
}
