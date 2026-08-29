/*
 * Copyright (c) 2026 Siddharth Chandrasekaran <sidcha@osdp.dev>
 *
 * SPDX-License-Identifier: Apache-2.0
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

// eventHandler handles events from peripheral devices
func eventHandler(pdNumber int, event *osdp.Event) int {
	fmt.Printf("CP: Received event from PD %d, Type: %d\n", pdNumber, event.Event.GetType())

	switch eventData := event.Event.(type) {
	case osdp.EventCardRead:
		fmt.Printf("CP: Card read on reader %d, format: %d, data: %x\n",
			eventData.ReaderNo, eventData.Format, eventData.Data)
	case osdp.EventKeyPress:
		fmt.Printf("CP: Keypad input on reader %d, data: %s\n",
			eventData.ReaderNo, string(eventData.Data))
	}

	return 0 // Success
}

func main() {
	fmt.Println("OSDP Go CP Sample Application")
	fmt.Printf("LibOSDP Version: %s\n", osdp.GetVersion())

	// Configure logging - use structured logging for this example
	logger := log.New(os.Stdout, "[OSDP-CP] ", log.LstdFlags|log.Lshortfile)
	osdp.SetStandardLogger(logger, "CP")

	// Check for FIFO file arguments
	if len(os.Args) < 3 {
		fmt.Println("Usage: cp_sample <rx_fifo> <tx_fifo>")
		fmt.Println("Example: cp_sample /tmp/pd_rx.fifo /tmp/pd_tx.fifo")
		os.Exit(1)
	}

	rxFIFO := os.Args[1]
	txFIFO := os.Args[2]

	// Create FIFO channel for communication with PD
	channel, err := osdp.NewFIFOChannel(rxFIFO, txFIFO)
	if err != nil {
		log.Fatalf("Failed to create FIFO channel: %v", err)
	}
	defer channel.Close()

	// Create PD info
	pdInfo := &osdp.PdInfo{
		Address:  1,
		BaudRate: 9600,
		Flags:    osdp.FlagEnforceSecure,
		SCBK:     []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F}, // Example key
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

	// Create Control Panel
	cp, err := osdp.NewControlPanel(channel, []*osdp.PdInfo{pdInfo}, osdp.LogInfo)
	if err != nil {
		log.Fatalf("Failed to create Control Panel: %v", err)
	}
	defer cp.Stop()

	// Set event handler after creation
	err = cp.SetEventHandler(eventHandler)
	if err != nil {
		log.Fatalf("Failed to set event handler: %v", err)
	}

	fmt.Println("Control Panel created successfully")

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Command sending loop
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	ledState := false

	for {
		select {
		case <-sigChan:
			fmt.Println("\nReceived termination signal, shutting down...")
			return

		case <-ticker.C:
			// Check PD status
			online, err := cp.IsOnline(0)
			if err != nil {
				fmt.Printf("Error checking PD status: %v\n", err)
				continue
			}

			secure, err := cp.IsSCActive(0)
			if err != nil {
				fmt.Printf("Error checking secure channel status: %v\n", err)
				continue
			}

			fmt.Printf("PD Status - Online: %t, Secure: %t\n", online, secure)

			if online {
				// Toggle LED
				ledState = !ledState
				var color osdp.OsdpLedColor
				if ledState {
					color = osdp.LEDColorGreen
				} else {
					color = osdp.LEDColorRed
				}

				ledCmd := &osdp.Command{
					Command: osdp.CommandLed{
						Reader:    0,
						LedNumber: 0,
						Permanent: osdp.LedParams{
							ControlCode: osdp.LEDPermanentCCSet,
							OnCount:     10,
							OffCount:    0,
							OnColor:     color,
						},
					},
				}

				err := cp.SubmitCommand(0, ledCmd)
				if err != nil {
					fmt.Printf("Error sending LED command: %v\n", err)
				} else {
					fmt.Printf("Sent LED command: %s\n", map[osdp.OsdpLedColor]string{
						osdp.LEDColorGreen: "GREEN",
						osdp.LEDColorRed:   "RED",
					}[color])
				}

				// Send buzzer command occasionally
				if ledState {
					buzzerCmd := &osdp.Command{
						Command: osdp.CommandBuzzer{
							Reader:      0,
							ControlCode: osdp.BuzzerCCDefaultTone,
							OnCount:     5, // 50ms
							OffCount:    5, // 50ms
							RepCount:    3, // 3 beeps
						},
					}

					err := cp.SubmitCommand(0, buzzerCmd)
					if err != nil {
						fmt.Printf("Error sending buzzer command: %v\n", err)
					} else {
						fmt.Println("Sent buzzer command")
					}
				}
			}

		default:
			// Check for events (non-blocking)
			event, err := cp.GetEvent(0, 10*time.Millisecond)
			if err == nil && event != nil {
				fmt.Printf("Retrieved event from queue: Type %d\n", event.Event.GetType())
			}
		}
	}
}
