/*
 * Copyright (c) 2026 Siddharth Chandrasekaran <sidcha@osdp.dev>
 *
 * SPDX-License-Identifier: Apache-2.0
 */

/*
Companion PD for API Showcase

This is a simple PD that runs alongside api_showcase to provide responses.
It automatically uses swapped FIFOs (TX/RX reversed) to communicate with the CP.

Usage:
  Terminal 1: ./companion_pd /tmp/showcase_rx.fifo /tmp/showcase_tx.fifo
  Terminal 2: ./api_showcase /tmp/showcase_rx.fifo /tmp/showcase_tx.fifo
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
	if len(os.Args) < 3 {
		fmt.Printf("Usage: %s <cp_rx_fifo> <cp_tx_fifo>\n", os.Args[0])
		fmt.Println("\nThis companion PD automatically swaps the FIFOs to connect to the CP.")
		fmt.Println("\nExample:")
		fmt.Printf("  mkfifo /tmp/showcase_rx.fifo /tmp/showcase_tx.fifo\n")
		fmt.Printf("  Terminal 1: %s /tmp/showcase_rx.fifo /tmp/showcase_tx.fifo\n", os.Args[0])
		fmt.Printf("  Terminal 2: ./api_showcase /tmp/showcase_rx.fifo /tmp/showcase_tx.fifo\n")
		os.Exit(1)
	}

	// Swap the FIFOs - PD's TX goes to CP's RX and vice versa
	pdRxFIFO := os.Args[2] // CP's TX becomes PD's RX
	pdTxFIFO := os.Args[1] // CP's RX becomes PD's TX

	fmt.Printf("==============================================================\n")
	fmt.Printf("              API Showcase - Companion PD                    \n")
	fmt.Printf("==============================================================\n\n")
	fmt.Printf("LibOSDP Version: %s\n", osdp.GetVersion())
	fmt.Printf("PD RX FIFO: %s\n", pdRxFIFO)
	fmt.Printf("PD TX FIFO: %s\n\n", pdTxFIFO)

	// Create FIFO channel
	channel, err := osdp.NewFIFOChannel(pdRxFIFO, pdTxFIFO)
	if err != nil {
		log.Fatalf("Failed to create FIFO channel: %v", err)
	}
	defer channel.Close()

	// Command handler
	commandHandler := func(command *osdp.Command) (int, *osdp.Event) {
		timestamp := time.Now().Format("15:04:05.000")
		fmt.Printf("[%s] Received command: ", timestamp)

		switch cmdData := command.Command.(type) {
		case osdp.CommandLed:
			fmt.Printf("LED %d -> Color %d\n", cmdData.LedNumber, cmdData.Permanent.OnColor)
		case osdp.CommandBuzzer:
			fmt.Printf("BUZZER -> %d beeps\n", cmdData.RepCount)
		case osdp.CommandText:
			fmt.Printf("TEXT -> \"%s\"\n", string(cmdData.Data))
		case osdp.CommandOutput:
			fmt.Printf("OUTPUT %d -> Control %d\n", cmdData.OutputNo, cmdData.ControlCode)
		case osdp.CommandKeyset:
			fmt.Printf("KEYSET\n")
		case osdp.CommandComset:
			fmt.Printf("COMSET -> Addr %d, Baud %d (Go values received)\n", cmdData.Address, cmdData.BaudRate)
			// Log what we're returning
			fmt.Printf("  Returning: ret=0 (ACK), event=nil\n")
		case osdp.CommandMfg:
			fmt.Printf("MFG -> Vendor 0x%02X%02X%02X\n",
				cmdData.VendorCode[2], cmdData.VendorCode[1], cmdData.VendorCode[0])
		case osdp.CommandStatus:
			fmt.Printf("STATUS REQUEST\n")
			// Respond with status - must match capability (4 inputs)
			return 0, &osdp.Event{
				Event: osdp.EventStatus{
					Type:   osdp.StatusReportInput,
					Report: []byte{1, 1, 1, 1}, // 4 inputs active
				},
			}
		case osdp.CommandFileTx:
			fmt.Printf("FILE_TX -> ID %d\n", cmdData.ID)
		default:
			fmt.Printf("UNKNOWN (Type: %d)\n", command.Command.GetType())
		}

		return 0, nil // ACK
	}

	// Create PD info with comprehensive capabilities
	pdInfo := &osdp.PdInfo{
		Address:  1,
		BaudRate: 115200,
		Flags:    osdp.FlagInstallMode | osdp.FlagEnableNotification,
		SCBK:     []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10},
		ID: osdp.PdId{
			Version:       1,
			Model:         100,
			VendorCode:    0x153,
			SerialNumber:  0x12345678,
			FirmwareMajor: 2,
			FirmwareMinor: 1,
			FirmwareBuild: 3,
		},
	}

	// Define all capabilities to match api_showcase expectations
	capabilities := []osdp.PdCapability{
		{Function: osdp.CapContactStatusMonitoring, Compliance: 1, NumItems: 4},
		{Function: osdp.CapOutputControl, Compliance: 1, NumItems: 4},
		{Function: osdp.CapCardDataFormat, Compliance: 1, NumItems: 1},
		{Function: osdp.CapReaderLEDControl, Compliance: 1, NumItems: 2},
		{Function: osdp.CapReaderAudibleOutput, Compliance: 1, NumItems: 1},
		{Function: osdp.CapReaderTextOutput, Compliance: 1, NumItems: 1},
		{Function: osdp.CapCommunicationSecurity, Compliance: 1, NumItems: 1},
	}

	// Create Peripheral Device
	fmt.Printf("Creating Peripheral Device...\n")
	pd, err := osdp.NewPeripheralDevice(channel, pdInfo, capabilities, osdp.LogInfo)
	if err != nil {
		log.Fatalf("Failed to create PD: %v", err)
	}
	defer pd.Stop()

	// Set command handler
	err = pd.SetCommandHandler(commandHandler)
	if err != nil {
		log.Fatalf("Failed to set command handler: %v", err)
	}

	fmt.Printf("Peripheral Device ready and listening...\n")
	fmt.Printf("Waiting for CP to connect...\n\n")

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Status ticker
	statusTicker := time.NewTicker(10 * time.Second)
	defer statusTicker.Stop()

	// Event generation ticker (simulate card reads occasionally)
	eventTicker := time.NewTicker(15 * time.Second)
	defer eventTicker.Stop()

	cardCounter := 1

	for {
		select {
		case <-sigChan:
			fmt.Println("\nReceived shutdown signal, stopping PD...")
			return

		case <-statusTicker.C:
			online, _ := pd.IsOnline()
			secure, _ := pd.IsSCActive()
			if online {
				scStatus := "No"
				if secure {
					scStatus = "Yes"
				}
				fmt.Printf("[STATUS] Online: Yes, Secure Channel: %s\n", scStatus)
			} else {
				fmt.Printf("[STATUS] Waiting for CP connection...\n")
			}

		case <-eventTicker.C:
			// Send a simulated card read event
			online, _ := pd.IsOnline()
			if online {
				cardData := []byte{0x12, 0x34, 0x56, 0x78, byte(cardCounter)}
				event := &osdp.Event{
					Event: osdp.EventCardRead{
						ReaderNo:  0,
						Format:    osdp.CardFormatWiegand,
						Direction: true,
						NrBits:    uint32(len(cardData) * 8),
						Data:      cardData,
					},
				}

				err := pd.SubmitEvent(event)
				if err != nil {
					fmt.Printf("Failed to send card event: %v\n", err)
				} else {
					fmt.Printf("[EVENT] Sent card read: %X\n", cardData)
				}
				cardCounter++
			}
		}
	}
}
