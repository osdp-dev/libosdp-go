/*
 * Copyright (c) 2026 Siddharth Chandrasekaran <sidcha@osdp.dev>
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package osdp_test

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/osdp-dev/libosdp-go"
)

// Test setup helper functions
func createFIFOPair(t *testing.T) (cpRx, cpTx, pdRx, pdTx string, cleanup func()) {
	tmpDir := t.TempDir()

	cpRx = filepath.Join(tmpDir, "cp_rx.fifo")
	cpTx = filepath.Join(tmpDir, "cp_tx.fifo")
	pdRx = cpTx // PD reads what CP writes
	pdTx = cpRx // PD writes what CP reads

	// Create named pipes (FIFOs)
	if err := syscall.Mkfifo(cpRx, 0o666); err != nil {
		t.Fatalf("Failed to create CP RX FIFO: %v", err)
	}
	if err := syscall.Mkfifo(cpTx, 0o666); err != nil {
		t.Fatalf("Failed to create CP TX FIFO: %v", err)
	}

	cleanup = func() {
		os.Remove(cpRx)
		os.Remove(cpTx)
	}

	return cpRx, cpTx, pdRx, pdTx, cleanup
}

type fifoLink struct {
	cpRx, cpTx, pdRx, pdTx string
}

func newFIFOLink(t *testing.T) fifoLink {
	t.Helper()
	cpRx, cpTx, pdRx, pdTx, cleanup := createFIFOPair(t)
	t.Cleanup(cleanup)
	return fifoLink{cpRx, cpTx, pdRx, pdTx}
}

func testPdInfo(flags osdp.OsdpFlag) *osdp.PdInfo {
	return &osdp.PdInfo{
		Address:  1,
		BaudRate: 9600,
		Flags:    flags,
		ID: osdp.PdId{
			Version: 1, Model: 1, VendorCode: 0x153, SerialNumber: 0x12345,
			FirmwareMajor: 1,
		},
	}
}

// startTestPD creates a PD on link and stops it when the test ends. Set
// handlers before calling startTestCP so nothing the CP triggers is missed.
func startTestPD(t *testing.T, link fifoLink, info *osdp.PdInfo, caps []osdp.PdCapability) *osdp.PeripheralDevice {
	t.Helper()
	ch, err := osdp.NewFIFOChannel(link.pdRx, link.pdTx)
	if err != nil {
		t.Fatalf("PD channel: %v", err)
	}
	t.Cleanup(func() { ch.Close() })
	pd, err := osdp.NewPeripheralDevice(ch, info, caps, osdp.LogWarning)
	if err != nil {
		t.Fatalf("NewPeripheralDevice: %v", err)
	}
	t.Cleanup(pd.Stop)
	return pd
}

// startTestCP creates a CP on link and stops it when the test ends.
func startTestCP(t *testing.T, link fifoLink, infos ...*osdp.PdInfo) *osdp.ControlPanel {
	t.Helper()
	ch, err := osdp.NewFIFOChannel(link.cpRx, link.cpTx)
	if err != nil {
		t.Fatalf("CP channel: %v", err)
	}
	t.Cleanup(func() { ch.Close() })
	cp, err := osdp.NewControlPanel(ch, infos, osdp.LogWarning)
	if err != nil {
		t.Fatalf("NewControlPanel: %v", err)
	}
	t.Cleanup(cp.Stop)
	return cp
}

// roundTripCommand marshals cmd through its C struct and back, failing the
// test if marshalling errors.
func roundTripCommand(t *testing.T, cmd *osdp.Command) *osdp.Command {
	t.Helper()
	got, err := osdp.RoundTripCommand(cmd)
	if err != nil {
		t.Fatalf("RoundTripCommand(%T): %v", cmd.Command, err)
	}
	return got
}

// roundTripEvent marshals ev through its C struct and back, failing the test if
// marshalling errors.
func roundTripEvent(t *testing.T, ev *osdp.Event) *osdp.Event {
	t.Helper()
	got, err := osdp.RoundTripEvent(ev)
	if err != nil {
		t.Fatalf("RoundTripEvent(%T): %v", ev.Event, err)
	}
	return got
}
