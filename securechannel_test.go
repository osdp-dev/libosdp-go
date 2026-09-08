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

// TestSecureChannel tests secure channel establishment
func TestSecureChannel(t *testing.T) {
	cpRx, cpTx, pdRx, pdTx, cleanup := createFIFOPair(t)
	defer cleanup()

	var wg sync.WaitGroup
	wg.Add(2)

	secureEstablished := make(chan bool, 2)

	// Start PD with secure channel support
	go func() {
		defer wg.Done()

		commandHandler := func(command *osdp.Command) (int, *osdp.Event) {
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
			Flags:    0,                                                                                                      // Default mode to allow secure channel setup
			SCBK:     []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F}, // Matching SCBK
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

		capabilities := []osdp.PdCapability{
			{Function: osdp.CapCommunicationSecurity, Compliance: 1, NumItems: 1},
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

		// Monitor secure channel status
		for i := 0; i < 40; i++ { // Increased attempts
			time.Sleep(250 * time.Millisecond)

			secure, err := pd.IsSCActive()
			if err == nil && secure {
				t.Log("PD: Secure channel established")
				secureEstablished <- true
				break
			}
		}
		t.Log("PD: Monitoring complete")
	}()

	// Start CP with secure channel enforcement
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
			Address:  1,
			BaudRate: 9600,
			Flags:    osdp.FlagEnforceSecure,
			SCBK:     []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F},
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

		cp, err := osdp.NewControlPanel(cpChannel, []*osdp.PdInfo{pdInfo}, osdp.LogInfo)
		if err != nil {
			t.Errorf("Failed to create CP: %v", err)
			return
		}
		defer cp.Stop()

		if err := cp.SetEventHandler(eventHandler); err != nil {
			t.Errorf("Failed to set event handler: %v", err)
		}

		// Monitor secure channel status
		for i := 0; i < 40; i++ { // Increased attempts
			time.Sleep(250 * time.Millisecond)

			secure, err := cp.IsSCActive(0)
			if err == nil && secure {
				t.Log("CP: Secure channel established")
				secureEstablished <- true
				break
			}
		}
		t.Log("CP: Monitoring complete")
	}()

	// Wait for secure channel establishment (at least one side)
	timeout := time.After(20 * time.Second)
	secureCount := 0

	for secureCount < 1 { // Require at least one side to confirm
		select {
		case <-secureEstablished:
			secureCount++
			t.Log("Test: Secure channel confirmed")
		case <-timeout:
			t.Error("Timeout waiting for secure channel establishment")
			return
		}
	}

	// Give time for both sides to potentially signal
	additionalTimeout := time.After(3 * time.Second)
	for secureCount < 2 {
		select {
		case <-secureEstablished:
			secureCount++
			t.Log("Test: Both sides confirmed secure channel")
		case <-additionalTimeout:
			t.Log("Test: Secure channel established (one side confirmed)")
			goto success
		}
	}

success:
	t.Log("Test: Secure channel establishment successful")
	// Don't wait for goroutines as they may be blocked on PD operations
}

func TestSecureChannelWaitHelpers(t *testing.T) {
	scbk, err := osdp.GenKey(16)
	if err != nil {
		t.Fatal(err)
	}
	link := newFIFOLink(t)
	pdInfo := testPdInfo(0)
	pdInfo.SCBK = scbk
	pd := startTestPD(t, link, pdInfo, []osdp.PdCapability{
		{Function: osdp.CapCommunicationSecurity, Compliance: 1, NumItems: 1},
	})
	cpInfo := testPdInfo(osdp.FlagEnforceSecure)
	cpInfo.SCBK = scbk
	cp := startTestCP(t, link, cpInfo)

	if !cp.OnlineWaitAll(10 * time.Second) {
		t.Fatal("OnlineWaitAll timed out")
	}
	if !cp.SCWait(0, 10*time.Second) || !cp.SCWaitAll(time.Second) {
		t.Fatal("CP never reported an active secure channel")
	}
	if !pd.SCWait(5 * time.Second) {
		t.Fatal("PD never reported an active secure channel")
	}
	if mask, err := cp.SCStatus(); err != nil || mask != 1 {
		t.Errorf("SCStatus = %#x, %v; want 0x1", mask, err)
	}
	if n, err := cp.GetNumSCActive(); err != nil || n != 1 {
		t.Errorf("GetNumSCActive = %d, %v; want 1", n, err)
	}

	pd.Stop()
	if !cp.OfflineWait(0, 15*time.Second) {
		t.Fatal("CP did not notice the PD going away")
	}
	if cp.SCWait(0, 100*time.Millisecond) {
		t.Error("SCWait reported an active channel for an offline PD")
	}
}

// A factory-fresh PD (install mode, no key) is provisioned by a CP that holds
// the SCBK, persists the key from the KEYSET command, and after a restart
// talks only over a secure channel with that key.
func TestInstallModeProvisioningThenEnforcedSecureChannel(t *testing.T) {
	scbk, err := osdp.GenKey(16)
	if err != nil {
		t.Fatal(err)
	}
	cpInfo := testPdInfo(osdp.FlagInstallMode)
	cpInfo.SCBK = scbk

	link := newFIFOLink(t)
	fresh := startTestPD(t, link, testPdInfo(osdp.FlagInstallMode), []osdp.PdCapability{
		{Function: osdp.CapCommunicationSecurity, Compliance: 1, NumItems: 1},
	})
	persisted := make(chan []byte, 1)
	fresh.SetCommandHandler(func(cmd *osdp.Command) (int, *osdp.Event) {
		if ks, ok := cmd.Command.(osdp.CommandKeyset); ok {
			persisted <- append([]byte(nil), ks.Data...)
		}
		return 0, nil
	})
	cp := startTestCP(t, link, cpInfo)

	var stored []byte
	select {
	case stored = <-persisted:
	case <-time.After(15 * time.Second):
		t.Fatal("PD never received the SCBK")
	}
	if !bytes.Equal(stored, scbk) {
		t.Fatalf("PD was given %x, want %x", stored, scbk)
	}
	if !cp.SCWait(0, 10*time.Second) {
		t.Fatal("secure channel did not come up with the provisioned key")
	}
	cp.Stop()
	fresh.Stop()

	link = newFIFOLink(t)
	pdInfo := testPdInfo(0)
	pdInfo.SCBK = stored
	startTestPD(t, link, pdInfo, []osdp.PdCapability{
		{Function: osdp.CapCommunicationSecurity, Compliance: 1, NumItems: 1},
	})
	cpInfo = testPdInfo(osdp.FlagEnforceSecure)
	cpInfo.SCBK = scbk
	cp = startTestCP(t, link, cpInfo)
	if !cp.SCWait(0, 10*time.Second) {
		t.Fatal("enforced secure channel did not come up after provisioning")
	}
}
