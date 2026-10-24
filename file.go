/*
 * Copyright (c) 2026 Siddharth Chandrasekaran <sidcha@osdp.dev>
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package osdp

/*
#include "cgo_helpers.h"
*/
import "C"
import (
	"fmt"
	"sync"
	"time"
	"unsafe"
)

// FileOperations interface defines the file operations that need to be implemented
// for OSDP file transfer functionality
type FileOperations interface {
	// Open opens a file with the given file ID and returns the file size
	Open(fileID int) (size int, err error)
	// Read reads data from the file at the specified offset
	Read(buffer []byte, size int, offset int) (bytesRead int, err error)
	// Write writes data to the file at the specified offset
	Write(buffer []byte, size int, offset int) (bytesWritten int, err error)
	// Close closes the currently open file
	Close() error
}

// FileFinalizer is an optional extension of FileOperations for a PD. Once the
// last byte of a file has arrived, Finalize is called, and called again on each
// CP poll for as long as it reports done=false; meanwhile the transfer stays
// open, so the application can verify or install what it received. retryIn
// hints how long the CP should wait before asking again. When done, outcome is
// the transfer's result: anything but MPOutcomeOK fails it on the CP.
//
// Finalize runs inside the PD's refresh loop and must return promptly; do slow
// work on another goroutine and only report progress here. A CP that missed a
// status reply may resend its last chunk while finalizing, so Write must still
// tolerate calls in that window. Without this interface a received file is
// final as soon as its last byte is written.
type FileFinalizer interface {
	Finalize() (done bool, outcome MPOutcome, retryIn time.Duration)
}

// FileTransferStatus represents the current status of a file transfer
type FileTransferStatus struct {
	Size   int // Total file size
	Offset int // Current offset (bytes transferred)
}

var (
	fileOpsMu       sync.RWMutex
	fileOpsRegistry = make(map[uintptr]FileOperations)
	fileOpsCounter  uintptr
)

func fileOpsLookup(handle uintptr) (FileOperations, bool) {
	fileOpsMu.RLock()
	defer fileOpsMu.RUnlock()
	ops, ok := fileOpsRegistry[handle]
	return ops, ok
}

// registerFileOperations hands fileOps to libosdp: on a CP for the PD at
// pdNumber, on a PD for itself (pdNumber is ignored). libosdp only accepts them
// before the context's first refresh.
func registerFileOperations(ctx *C.osdp_t, pdNumber int, fileOps FileOperations) error {
	fileOpsMu.Lock()
	fileOpsCounter++
	handle := fileOpsCounter
	fileOpsRegistry[handle] = fileOps
	fileOpsMu.Unlock()

	// libosdp copies the ops struct, so it only needs to outlive the call.
	// calloc leaves optional ops this binding does not set (finalize) NULL.
	cFileOps := (*C.struct_osdp_file_ops)(C.calloc(1, C.size_t(unsafe.Sizeof(C.struct_osdp_file_ops{}))))
	defer C.free(unsafe.Pointer(cFileOps))
	cFileOps.arg = handlePtr(handle)
	cFileOps.open = (*[0]byte)(C.fileOpenWrapper)
	cFileOps.read = (*[0]byte)(C.fileReadWrapper)
	cFileOps.write = (*[0]byte)(C.fileWriteWrapper)
	cFileOps.close = (*[0]byte)(C.fileCloseWrapper)
	if _, ok := fileOps.(FileFinalizer); ok {
		cFileOps.finalize = (*[0]byte)(C.fileFinalizeWrapper)
	}

	ret := C.osdp_file_register_ops(unsafe.Pointer(ctx), C.int(pdNumber), cFileOps)
	if ret != 0 {
		fileOpsMu.Lock()
		delete(fileOpsRegistry, handle)
		fileOpsMu.Unlock()
		return fmt.Errorf("failed to register file operations, error code: %d", ret)
	}

	return nil
}

// GetFileTransferStatus queries the current file transfer status
func (cp *ControlPanel) GetFileTransferStatus(pdNumber int) (*FileTransferStatus, error) {
	if pdNumber < 0 || pdNumber >= len(cp.pdInfoList) {
		return nil, fmt.Errorf("invalid PD number: %d", pdNumber)
	}

	cp.mu.Lock()
	defer cp.mu.Unlock()

	if cp.ctx == nil {
		return nil, fmt.Errorf("control panel context is not initialized")
	}

	var size, offset C.uint32_t
	ret := C.osdp_get_file_tx_status(unsafe.Pointer(cp.ctx), C.int(pdNumber), &size, &offset)
	if ret != 0 {
		return nil, fmt.Errorf("failed to get file transfer status, error code: %d", ret)
	}

	status := &FileTransferStatus{
		Size:   int(size),
		Offset: int(offset),
	}

	return status, nil
}

// GetFileTransferStatusPD queries the current file transfer status for PD
func (pd *PeripheralDevice) GetFileTransferStatus() (*FileTransferStatus, error) {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	if pd.ctx == nil {
		return nil, fmt.Errorf("peripheral device context is not initialized")
	}

	var size, offset C.uint32_t
	ret := C.osdp_get_file_tx_status(unsafe.Pointer(pd.ctx), 0, &size, &offset) // PD mode ignores PD number
	if ret != 0 {
		return nil, fmt.Errorf("failed to get file transfer status, error code: %d", ret)
	}

	status := &FileTransferStatus{
		Size:   int(size),
		Offset: int(offset),
	}

	return status, nil
}

// C callback wrappers for file operations

//export fileOpenWrapper
func fileOpenWrapper(arg unsafe.Pointer, fileID C.int, size *C.uint32_t) C.int {
	fileOps, ok := fileOpsLookup(uintptr(arg))
	if !ok {
		return C.int(-1)
	}

	fileSize, err := fileOps.Open(int(fileID))
	if err != nil {
		return C.int(-1)
	}

	*size = C.uint32_t(fileSize)
	return C.int(0)
}

//export fileReadWrapper
func fileReadWrapper(arg unsafe.Pointer, buf unsafe.Pointer, size C.uint32_t, offset C.uint32_t) C.int {
	fileOps, ok := fileOpsLookup(uintptr(arg))
	if !ok {
		return C.int(-1)
	}

	buffer := (*[1 << 30]byte)(buf)[:int(size):int(size)]
	bytesRead, err := fileOps.Read(buffer, int(size), int(offset))
	if err != nil {
		return C.int(-1)
	}

	return C.int(bytesRead)
}

//export fileWriteWrapper
func fileWriteWrapper(arg unsafe.Pointer, buf unsafe.Pointer, size C.uint32_t, offset C.uint32_t) C.int {
	fileOps, ok := fileOpsLookup(uintptr(arg))
	if !ok {
		return C.int(-1)
	}

	buffer := (*[1 << 30]byte)(buf)[:int(size):int(size)]
	bytesWritten, err := fileOps.Write(buffer, int(size), int(offset))
	if err != nil {
		return C.int(-1)
	}

	return C.int(bytesWritten)
}

//export fileCloseWrapper
func fileCloseWrapper(arg unsafe.Pointer) C.int {
	fileOps, ok := fileOpsLookup(uintptr(arg))
	if !ok {
		return C.int(-1)
	}

	err := fileOps.Close()
	if err != nil {
		return C.int(-1)
	}

	return C.int(0)
}

//export fileFinalizeWrapper
func fileFinalizeWrapper(arg unsafe.Pointer, outcome *C.enum_osdp_mp_outcome, delayMs *C.uint16_t) C.bool {
	fileOps, ok := fileOpsLookup(uintptr(arg))
	if !ok {
		*outcome = C.OSDP_MP_OUTCOME_ABORTED
		return true
	}
	done, result, retryIn := fileOps.(FileFinalizer).Finalize()
	if !done {
		ms := retryIn.Milliseconds()
		if ms > 0xFFFF {
			ms = 0xFFFF
		}
		*delayMs = C.uint16_t(ms)
		return false
	}
	*outcome = C.enum_osdp_mp_outcome(result)
	return true
}
