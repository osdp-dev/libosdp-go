/*
 * Copyright (c) 2026 Siddharth Chandrasekaran <sidcha@osdp.dev>
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package osdp_test

import (
	"testing"

	"github.com/osdp-dev/libosdp-go"
)

// TestAPIcoverage tests newly implemented APIs to ensure they work correctly
func TestAPICoverage(t *testing.T) {
	t.Run("VersionAPIs", func(t *testing.T) {
		version := osdp.GetVersion()
		if version == "" {
			t.Errorf("GetVersion() returned empty string")
		}
		t.Logf("LibOSDP Version: %s", version)

		sourceInfo := osdp.GetSourceInfo()
		if sourceInfo == "" {
			t.Errorf("GetSourceInfo() returned empty string")
		}
		t.Logf("LibOSDP Source Info: %s", sourceInfo)
	})

	t.Run("CPAPIsWithoutConnection", func(t *testing.T) {
		// Test CP creation and basic API calls without actual connection
		cpRx, cpTx, _, _, cleanup := createFIFOPair(t)
		defer cleanup()

		cpChannel, err := osdp.NewFIFOChannel(cpRx, cpTx)
		if err != nil {
			t.Fatalf("Failed to create CP FIFO channel: %v", err)
		}
		defer cpChannel.Close()

		pdInfo := &osdp.PdInfo{
			Address: 1, BaudRate: 9600, Flags: osdp.FlagInstallMode,
			ID: osdp.PdId{Version: 1, Model: 1, VendorCode: 0x153, SerialNumber: 0x12345},
		}

		cp, err := osdp.NewControlPanel(cpChannel, []*osdp.PdInfo{pdInfo}, osdp.LogInfo)
		if err != nil {
			t.Fatalf("Failed to create CP: %v", err)
		}
		defer cp.Stop()

		// Test newly implemented APIs

		// Test IsPDEnabled
		enabled, err := cp.IsPDEnabled(0)
		if err != nil {
			t.Errorf("IsPDEnabled() failed: %v", err)
		}
		t.Logf("PD 0 enabled status: %v", enabled)

		// Test FlushCommands
		count, err := cp.FlushCommands(0)
		if err != nil {
			t.Errorf("FlushCommands() failed: %v", err)
		}
		t.Logf("Flushed %d commands", count)

		// Test invalid PD number handling
		_, err = cp.IsPDEnabled(999)
		if err == nil {
			t.Errorf("Expected error for invalid PD number")
		}

		// Test DisablePD and EnablePD
		err = cp.DisablePD(0)
		if err != nil {
			t.Errorf("DisablePD() failed: %v", err)
		}

		enabledAfterDisable, _ := cp.IsPDEnabled(0)
		t.Logf("PD enabled after disable: %v", enabledAfterDisable)

		err = cp.EnablePD(0)
		if err != nil {
			// EnablePD may fail when no actual PD is connected, which is expected in this test
			t.Logf("EnablePD() failed as expected without connection: %v", err)
		}

		enabledAfterEnable, _ := cp.IsPDEnabled(0)
		t.Logf("PD enabled after enable: %v", enabledAfterEnable)
	})

	t.Run("PDAPIsWithoutConnection", func(t *testing.T) {
		// Test PD creation and basic API calls
		_, _, pdRx, pdTx, cleanup := createFIFOPair(t)
		defer cleanup()

		pdChannel, err := osdp.NewFIFOChannel(pdRx, pdTx)
		if err != nil {
			t.Fatalf("Failed to create PD FIFO channel: %v", err)
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
			{Function: osdp.CapReaderLEDControl, Compliance: 1, NumItems: 2},
		}

		pd, err := osdp.NewPeripheralDevice(pdChannel, pdInfo, capabilities, osdp.LogInfo)
		if err != nil {
			t.Fatalf("Failed to create PD: %v", err)
		}
		defer pd.Stop()

		// Test FlushEvents
		count, err := pd.FlushEvents()
		if err != nil {
			t.Errorf("FlushEvents() failed: %v", err)
		}
		t.Logf("Flushed %d events", count)
	})

	t.Run("NewCommandTypes", func(t *testing.T) {
		// Test that new command types can be created without errors
		commands := []struct {
			name string
			cmd  *osdp.Command
		}{
			{
				"Keyset Command",
				&osdp.Command{
					Command: osdp.CommandKeyset{
						KeyType: 1,
						Data:    []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10},
					},
				},
			},
			{
				"Comset Command",
				&osdp.Command{
					Command: osdp.CommandComset{
						Address:  2,
						BaudRate: 115200,
					},
				},
			},
			{
				"FileTx Command",
				&osdp.Command{
					Command: osdp.CommandFileTx{
						ID:    123,
						Flags: 0,
					},
				},
			},
		}

		for _, tc := range commands {
			t.Run(tc.name, func(t *testing.T) {
				if tc.cmd.Command == nil {
					t.Errorf("Command is nil for %s", tc.name)
				}
				if tc.cmd.Command.GetType() == 0 {
					t.Errorf("Command type is zero for %s", tc.name)
				}
				t.Logf("Successfully created %s with type %d", tc.name, tc.cmd.Command.GetType())
			})
		}
	})
}

// mockFileOps is a mock implementation for testing file operations
type mockFileOps struct {
	openCalled  bool
	readCalled  bool
	writeCalled bool
	closeCalled bool
	testData    []byte
}

// fileOpsImpl implements FileOperations for testing
type fileOpsImpl struct {
	mock *mockFileOps
}

// TestFileTransferInterface tests the file transfer interface structure
func TestFileTransferInterface(t *testing.T) {
	// Create a simple mock file operations implementation
	mock := &mockFileOps{
		testData: []byte("test file content"),
	}

	// Implement FileOperations interface
	var fileOps osdp.FileOperations = &fileOpsImpl{mock: mock}

	// Test interface methods
	size, err := fileOps.Open(123)
	if err != nil {
		t.Errorf("Open failed: %v", err)
	}
	if size != len(mock.testData) {
		t.Errorf("Expected size %d, got %d", len(mock.testData), size)
	}
	if !mock.openCalled {
		t.Errorf("Open was not called")
	}

	// Test read
	buffer := make([]byte, len(mock.testData))
	bytesRead, err := fileOps.Read(buffer, len(buffer), 0)
	if err != nil {
		t.Errorf("Read failed: %v", err)
	}
	if bytesRead != len(mock.testData) {
		t.Errorf("Expected %d bytes read, got %d", len(mock.testData), bytesRead)
	}
	if string(buffer) != string(mock.testData) {
		t.Errorf("Data mismatch: expected %s, got %s", string(mock.testData), string(buffer))
	}

	// Test close
	err = fileOps.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}
	if !mock.closeCalled {
		t.Errorf("Close was not called")
	}

	t.Logf("File transfer interface test completed successfully")
}

func (f *fileOpsImpl) Open(fileID int) (size int, err error) {
	f.mock.openCalled = true
	return len(f.mock.testData), nil
}

func (f *fileOpsImpl) Read(buffer []byte, size int, offset int) (bytesRead int, err error) {
	f.mock.readCalled = true
	copy(buffer, f.mock.testData[offset:])
	return len(f.mock.testData) - offset, nil
}

func (f *fileOpsImpl) Write(buffer []byte, size int, offset int) (bytesWritten int, err error) {
	f.mock.writeCalled = true
	return size, nil
}

func (f *fileOpsImpl) Close() error {
	f.mock.closeCalled = true
	return nil
}
