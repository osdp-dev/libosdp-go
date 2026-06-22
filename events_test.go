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

// TestEventGeneration tests PD event generation capabilities
func TestEventGeneration(t *testing.T) {
	cpRx, cpTx, pdRx, pdTx, cleanup := createFIFOPair(t)
	defer cleanup()

	var wg sync.WaitGroup
	wg.Add(2)

	eventsReceived := make(map[osdp.OsdpEventType]int)
	var eventsMutex sync.Mutex

	// Start CP to receive events
	go func() {
		defer wg.Done()

		eventHandler := func(pdNumber int, event *osdp.Event) int {
			eventsMutex.Lock()
			eventsReceived[event.Event.GetType()]++
			eventsMutex.Unlock()

			t.Logf("CP received event: Type %d from PD %d", event.Event.GetType(), pdNumber)

			switch eventData := event.Event.(type) {
			case osdp.EventCardRead:
				t.Logf("Card read: Reader %d, Format %d, Data %v", eventData.ReaderNo, eventData.Format, eventData.Data)
			case osdp.EventKeyPress:
				t.Logf("Keypad: Reader %d, Data %s", eventData.ReaderNo, string(eventData.Data))
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

		time.Sleep(8 * time.Second)
	}()

	// Start PD and generate events
	go func() {
		defer wg.Done()

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
			{Function: osdp.CapCardDataFormat, Compliance: 1, NumItems: 1},
			// OUTPUT_CONTROL with 1 item so a 1-entry OSTATR report can be built.
			{Function: osdp.CapOutputControl, Compliance: 1, NumItems: 1},
		}

		pd, err := osdp.NewPeripheralDevice(pdChannel, pdInfo, capabilities, osdp.LogInfo)
		if err != nil {
			t.Errorf("Failed to create PD: %v", err)
			return
		}
		defer pd.Stop()

		// Wait for connection
		time.Sleep(2 * time.Second)

		// Generate card read event
		cardEvent := &osdp.Event{
			Event: osdp.EventCardRead{
				ReaderNo:  0,
				Format:    osdp.CardFormatWiegand,
				Direction: true,
				NrBits:    32,
				Data:      []byte{0x12, 0x34, 0x56, 0x78},
			},
		}
		err = pd.SubmitEvent(cardEvent)
		if err != nil {
			t.Errorf("Failed to send card read event: %v", err)
		}

		time.Sleep(1 * time.Second)

		// Generate keypad event
		keypadEvent := &osdp.Event{
			Event: osdp.EventKeyPress{
				ReaderNo: 0,
				Data:     []byte("1234"),
			},
		}
		err = pd.SubmitEvent(keypadEvent)
		if err != nil {
			t.Errorf("Failed to send keypad event: %v", err)
		}

		time.Sleep(1 * time.Second)

		// Generate status event
		statusEvent := &osdp.Event{
			Event: osdp.EventStatus{
				Type:   osdp.StatusReportOutput,
				Report: []byte{1}, // Output 0 active (1 entry == OUTPUT_CONTROL num_items)
			},
		}
		err = pd.SubmitEvent(statusEvent)
		if err != nil {
			t.Errorf("Failed to send status event: %v", err)
		}

		time.Sleep(3 * time.Second)
	}()

	wg.Wait()

	// Verify events were received
	expectedEvents := []osdp.OsdpEventType{osdp.EvtCardRead, osdp.EvtKeypress, osdp.EvtStatus}
	for _, eventType := range expectedEvents {
		if count, ok := eventsReceived[eventType]; !ok || count == 0 {
			t.Errorf("Event type %d was not received", eventType)
		}
	}

	t.Logf("Event generation test completed successfully. Events received: %v", eventsReceived)
}
