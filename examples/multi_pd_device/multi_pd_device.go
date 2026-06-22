/*
 * Copyright (c) 2026 Siddharth Chandrasekaran <sidcha@osdp.dev>
 *
 * SPDX-License-Identifier: Apache-2.0
 */

/*
Multi-PD Device Example

This example demonstrates how to simulate multiple Peripheral Devices,
each with different capabilities and behaviors. It shows:
- PD 1: Door Reader (LED, Buzzer, Input monitoring, Card events)
- PD 2: Display Terminal (Text display, Keypad input)
- PD 3: Output Controller (Multiple outputs, Status monitoring)

This works with the multi_pd_cp.go example.
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

// PD configurations and behaviors
type PDConfig struct {
	Name         string
	Address      int
	SerialNumber uint32
	Model        int
	Capabilities []osdp.PdCapability
	EventGen     func(*osdp.PeripheralDevice, int) // Event generator function
}

func main() {
	if len(os.Args) < 7 {
		fmt.Printf("Usage: %s <pd1_rx_fifo> <pd1_tx_fifo> <pd2_rx_fifo> <pd2_tx_fifo> <pd3_rx_fifo> <pd3_tx_fifo>\n", os.Args[0])
		fmt.Println("\nExample:")
		fmt.Printf("  mkfifo /tmp/pd1_tx.fifo /tmp/pd1_rx.fifo /tmp/pd2_tx.fifo /tmp/pd2_rx.fifo /tmp/pd3_tx.fifo /tmp/pd3_rx.fifo\n")
		fmt.Printf("  %s /tmp/pd1_tx.fifo /tmp/pd1_rx.fifo /tmp/pd2_tx.fifo /tmp/pd2_rx.fifo /tmp/pd3_tx.fifo /tmp/pd3_rx.fifo\n", os.Args[0])
		os.Exit(1)
	}

	// Configure 3 different PD types
	configs := []PDConfig{
		{
			Name:         "Door Reader",
			Address:      1,
			SerialNumber: 0x11111,
			Model:        1,
			Capabilities: []osdp.PdCapability{
				{Function: osdp.CapContactStatusMonitoring, Compliance: 1, NumItems: 2},
				{Function: osdp.CapReaderLEDControl, Compliance: 1, NumItems: 2},
				{Function: osdp.CapReaderAudibleOutput, Compliance: 1, NumItems: 1},
				{Function: osdp.CapCardDataFormat, Compliance: 1, NumItems: 1},
			},
			EventGen: generateDoorReaderEvents,
		},
		{
			Name:         "Display Terminal",
			Address:      2,
			SerialNumber: 0x22222,
			Model:        2,
			Capabilities: []osdp.PdCapability{
				{Function: osdp.CapReaderTextOutput, Compliance: 1, NumItems: 1},
				{Function: osdp.CapReaderLEDControl, Compliance: 1, NumItems: 1},
			},
			EventGen: generateDisplayTerminalEvents,
		},
		{
			Name:         "Output Controller",
			Address:      3,
			SerialNumber: 0x33333,
			Model:        3,
			Capabilities: []osdp.PdCapability{
				{Function: osdp.CapOutputControl, Compliance: 1, NumItems: 4},
				{Function: osdp.CapContactStatusMonitoring, Compliance: 1, NumItems: 4},
			},
			EventGen: generateOutputControllerEvents,
		},
	}

	// Create channels and PDs
	pds := make([]*osdp.PeripheralDevice, 3)
	channels := make([]*osdp.FIFOChannel, 3)

	for i, config := range configs {
		// Create FIFO channel
		var err error
		channels[i], err = osdp.NewFIFOChannel(os.Args[1+i*2], os.Args[2+i*2])
		if err != nil {
			log.Fatalf("Failed to create FIFO channel for %s: %v", config.Name, err)
		}
		defer channels[i].Close()

		// Create command handler for this PD
		commandHandler := createCommandHandler(config.Name, i+1)

		// Create PD info
		pdInfo := &osdp.PdInfo{
			Address:  config.Address,
			BaudRate: 9600,
			Flags:    osdp.FlagInstallMode,
			ID: osdp.PdId{
				Version:       1,
				Model:         config.Model,
				VendorCode:    0x153,
				SerialNumber:  int(config.SerialNumber),
				FirmwareMajor: 1,
				FirmwareMinor: 0,
				FirmwareBuild: 1,
			},
		}

		// Create PD
		pds[i], err = osdp.NewPeripheralDevice(channels[i], pdInfo, config.Capabilities, osdp.LogInfo)
		if err != nil {
			log.Fatalf("Failed to create %s: %v", config.Name, err)
		}
		defer pds[i].Stop()

		// Set command handler after creation
		err = pds[i].SetCommandHandler(commandHandler)
		if err != nil {
			log.Fatalf("Failed to set command handler for %s: %v", config.Name, err)
		}

		fmt.Printf("%s (PD-%d) started\n", config.Name, i+1)
	}

	fmt.Println("=================================================")
	fmt.Println("All Peripheral Devices started and ready")
	fmt.Println("Waiting for commands from Control Panel...")

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Event generation ticker
	eventTicker := time.NewTicker(7 * time.Second)
	defer eventTicker.Stop()

	eventIndex := 0

	for {
		select {
		case <-sigChan:
			fmt.Println("\nReceived shutdown signal, stopping all PDs...")
			return

		case <-eventTicker.C:
			// Generate events from different PDs
			pdIndex := eventIndex % len(pds)
			configs[pdIndex].EventGen(pds[pdIndex], pdIndex+1)
			eventIndex++
		}
	}
}

// Creates a command handler for a specific PD
func createCommandHandler(pdName string, pdNumber int) osdp.CommandHandler {
	return func(command *osdp.Command) (int, *osdp.Event) {
		timestamp := time.Now().Format("15:04:05")
		fmt.Printf("[%s] %s COMMAND: ", timestamp, pdName)

		switch cmdData := command.Command.(type) {
		case osdp.CommandLed:
			fmt.Printf("LED %d set to color %d\n", cmdData.LedNumber, cmdData.Permanent.OnColor)

		case osdp.CommandBuzzer:
			fmt.Printf("Buzzer activated: %d beeps, %dms on/%dms off\n",
				cmdData.RepCount, cmdData.OnCount*100, cmdData.OffCount*100)

		case osdp.CommandText:
			fmt.Printf("Display text: \"%s\" at (%d,%d)\n",
				string(cmdData.Data), cmdData.OffsetRow, cmdData.OffsetCol)

		case osdp.CommandOutput:
			fmt.Printf("Output %d control=%d, timer=%d\n",
				cmdData.OutputNo, cmdData.ControlCode, cmdData.TimerCount)

		case osdp.CommandMfg:
			fmt.Printf("Manufacturer command: Vendor=0x%02X%02X%02X, Data=%v\n",
				cmdData.VendorCode[2], cmdData.VendorCode[1], cmdData.VendorCode[0], cmdData.Data)
			// Echo back with confirmation
			return 0, &osdp.Event{
				Event: osdp.EventMfgReply{
					VendorCode: cmdData.VendorCode,
					Reply:      cmdData.Command,
					Data:       append(cmdData.Data, []byte(" ACK")...),
				},
			}

		case osdp.CommandKeyset:
			fmt.Printf("Key set command\n")

		case osdp.CommandComset:
			fmt.Printf("Communication set: Address=%d, BaudRate=%d\n",
				cmdData.Address, cmdData.BaudRate)

		case osdp.CommandFileTx:
			fmt.Printf("File transfer: ID=%d, Flags=0x%X\n",
				cmdData.ID, cmdData.Flags)

		case osdp.CommandStatus:
			fmt.Printf("Status request\n")
			// Respond with a status report
			return 0, &osdp.Event{
				Event: osdp.EventStatus{
					Type:   osdp.StatusReportInput,
					Report: []byte{1, 0}, // Input 0 active, Input 1 inactive
				},
			}

		default:
			fmt.Printf("Type %d (Unknown)\n", command.Command.GetType())
		}

		return 0, nil // ACK
	}
}

// Event generators for different PD types

func generateDoorReaderEvents(pd *osdp.PeripheralDevice, pdNumber int) {
	events := []func() *osdp.Event{
		// Card read event
		func() *osdp.Event {
			cardData := []byte{0x12, 0x34, 0x56, byte(time.Now().Unix() & 0xFF)}
			return &osdp.Event{
				Event: osdp.EventCardRead{
					ReaderNo:  0,
					Format:    osdp.CardFormatWiegand,
					Direction: true,
					NrBits:    uint32(len(cardData) * 8),
					Data:      cardData,
				},
			}
		},
		// Input status change
		func() *osdp.Event {
			return &osdp.Event{
				Event: osdp.EventStatus{
					Type:   osdp.StatusReportInput,
					Report: []byte{byte(time.Now().Unix() % 2)}, // Alternating state
				},
			}
		},
	}

	event := events[time.Now().Unix()%int64(len(events))]()
	if err := pd.SubmitEvent(event); err != nil {
		fmt.Printf("Failed to send event from Door Reader: %v\n", err)
	} else {
		fmt.Printf("← Door Reader: Sent %s event\n", getEventTypeName(event.Event.GetType()))
	}
}

func generateDisplayTerminalEvents(pd *osdp.PeripheralDevice, pdNumber int) {
	// Simulate keypad input
	keypads := []string{"1234", "5678", "9999", "0000", "*#12"}
	keypadData := keypads[time.Now().Unix()%int64(len(keypads))]

	event := &osdp.Event{
		Event: osdp.EventKeyPress{
			ReaderNo: 0,
			Data:     []byte(keypadData),
		},
	}

	if err := pd.SubmitEvent(event); err != nil {
		fmt.Printf("Failed to send event from Display Terminal: %v\n", err)
	} else {
		fmt.Printf("← Display Terminal: Sent keypad event \"%s\"\n", keypadData)
	}
}

func generateOutputControllerEvents(pd *osdp.PeripheralDevice, pdNumber int) {
	// Simulate output status changes
	outputNum := time.Now().Unix() % 4
	state := (time.Now().Unix() / 4) % 2

	event := &osdp.Event{
		Event: osdp.EventStatus{
			Type:   osdp.StatusReportOutput,
			Report: []byte{byte(state)},
		},
	}

	if err := pd.SubmitEvent(event); err != nil {
		fmt.Printf("Failed to send event from Output Controller: %v\n", err)
	} else {
		status := "OFF"
		if state == 1 {
			status = "ON"
		}
		fmt.Printf("← Output Controller: Output %d is %s\n", outputNum, status)
	}
}

func getEventTypeName(eventType osdp.OsdpEventType) string {
	switch eventType {
	case osdp.EvtCardRead:
		return "CARD_READ"
	case osdp.EvtKeypress:
		return "KEYPRESS"
	case osdp.EvtStatus:
		return "STATUS"
	case osdp.EvtMfgReply:
		return "MFG_REPLY"
	default:
		return "UNKNOWN"
	}
}
