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
