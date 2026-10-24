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

// commandHandler handles commands from the control panel
func commandHandler(command *osdp.Command) (int, *osdp.Event) {
	fmt.Printf("PD: Received command, Type: %d\n", command.Command.GetType())

	switch cmdData := command.Command.(type) {
	case osdp.CommandLed:
		fmt.Printf("PD: LED command for LED %d, permanent color: %d\n",
			cmdData.LedNumber, cmdData.Permanent.OnColor)
		return 1, nil // ACK

	case osdp.CommandBuzzer:
		fmt.Printf("PD: Buzzer command on reader %d, control: %d, count: %d\n",
			cmdData.Reader, cmdData.ControlCode, cmdData.RepCount)
		return 1, nil // ACK

	case osdp.CommandOutput:
		fmt.Printf("PD: Output command for output %d, control: %d\n",
			cmdData.OutputNo, cmdData.ControlCode)
		return 1, nil // ACK

	case osdp.CommandText:
		fmt.Printf("PD: Text command for reader %d, text: '%s'\n",
			cmdData.Reader, string(cmdData.Data))
		return 1, nil // ACK

	default:
		fmt.Printf("PD: Unknown command type: %d\n", command.Command.GetType())
		return -1, nil // NACK
	}
}

func main() {
	fmt.Println("OSDP Go PD Sample Application")
	fmt.Printf("LibOSDP Version: %s\n", osdp.GetVersion())

	// Configure logging - use custom handler for this example
	osdp.SetLogHandler(func(pd int, level osdp.LogLevel, file string, line int, message string) {
		fmt.Printf("[PD %d] %s: %s\n", pd, level.String(), message)
	})

	// Check for FIFO file arguments
	if len(os.Args) < 3 {
		fmt.Println("Usage: pd_sample <rx_fifo> <tx_fifo>")
		fmt.Println("Example: pd_sample /tmp/pd_tx.fifo /tmp/pd_rx.fifo")
		os.Exit(1)
	}

	rxFIFO := os.Args[1]
	txFIFO := os.Args[2]

	// Create FIFO channel for communication with CP
	channel, err := osdp.NewFIFOChannel(rxFIFO, txFIFO)
	if err != nil {
		log.Fatalf("Failed to create FIFO channel: %v", err)
	}
	defer channel.Close()

	// Create PD info
	pdInfo := &osdp.PdInfo{
		Address:  1,
		BaudRate: 9600,
		Flags:    osdp.FlagInstallMode, // Allow secure channel setup with SCBK-D
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

	// Create PD capabilities
	capabilities := []osdp.PdCapability{
		{
			Function:   osdp.CapContactStatusMonitoring,
			Compliance: 1,
			NumItems:   4, // 4 inputs
		},
		{
			Function:   osdp.CapOutputControl,
			Compliance: 1,
			NumItems:   2, // 2 outputs
		},
		{
			Function:   osdp.CapCardDataFormat,
			Compliance: 1,
			NumItems:   1, // Support for card data format
		},
		{
			Function:   osdp.CapReaderLEDControl,
			Compliance: 1,
			NumItems:   2, // 2 LEDs
		},
		{
			Function:   osdp.CapReaderAudibleOutput,
			Compliance: 1,
			NumItems:   1, // 1 buzzer
		},
		{
			Function:   osdp.CapReaderTextOutput,
			Compliance: 1,
			NumItems:   1, // Text display support
		},
		{
			Function:   osdp.CapCommunicationSecurity,
			Compliance: 1,
			NumItems:   1, // Secure channel support
		},
	}

	// Create Peripheral Device
	pdInfo.Capabilities = capabilities
	pd, err := osdp.NewPeripheralDevice(channel, pdInfo, osdp.LogInfo)
	if err != nil {
		log.Fatalf("Failed to create Peripheral Device: %v", err)
	}
	defer pd.Stop()

	// Set command handler after creation
	err = pd.SetCommandHandler(commandHandler)
	if err != nil {
		log.Fatalf("Failed to set command handler: %v", err)
	}

	fmt.Println("Peripheral Device created successfully")

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Event generation loop
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	cardCounter := 1

	for {
		select {
		case <-sigChan:
			fmt.Println("\nReceived termination signal, shutting down...")
			return

		case <-ticker.C:
			// Check PD status
			online, err := pd.IsOnline()
			if err != nil {
				fmt.Printf("Error checking PD status: %v\n", err)
				continue
			}

			secure, err := pd.IsSCActive()
			if err != nil {
				fmt.Printf("Error checking secure channel status: %v\n", err)
				continue
			}

			fmt.Printf("PD Status - Online: %t, Secure: %t\n", online, secure)

			if online {
				// Simulate card read event
				cardData := []byte{0x12, 0x34, 0x56, 0x78, byte(cardCounter)}

				cardEvent := &osdp.Event{
					Event: osdp.EventCardRead{
						ReaderNo:  0,
						Format:    osdp.CardFormatWiegand,
						Direction: true, // Entry
						NrBits:    uint32(len(cardData) * 8),
						Data:      cardData,
					},
				}

				err := pd.SubmitEvent(cardEvent)
				if err != nil {
					fmt.Printf("Error sending card read event: %v\n", err)
				} else {
					fmt.Printf("Sent card read event: card data %x\n", cardData)
				}

				cardCounter++

				// Simulate status change (every other cycle)
				if cardCounter%2 == 0 {
					statusEvent := &osdp.Event{
						Event: osdp.EventStatus{
							Type: osdp.StatusReportInput,
							Report: []byte{func() byte {
								if cardCounter%4 == 0 {
									return 0x01
								}
								return 0x00
							}()}, // Toggle every 2 cycles
						},
					}

					err := pd.SubmitEvent(statusEvent)
					if err != nil {
						fmt.Printf("Error sending input status event: %v\n", err)
					} else {
						fmt.Printf("Sent input status event: input 0 = %t\n", cardCounter%4 == 0)
					}
				}
			}

		default:
			// Check for commands (non-blocking)
			cmd, err := pd.GetCommand(10 * time.Millisecond)
			if err == nil && cmd != nil {
				fmt.Printf("Retrieved command from queue: Type %d\n", cmd.Command.GetType())
			}
		}
	}
}
