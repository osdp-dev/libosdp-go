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
	"syscall"
	"unsafe"
)

// Channel is the transport a CP/PD uses to talk to the OSDP bus. Implement it to
// integrate a custom transport; SerialChannel and FIFOChannel are provided.
type Channel interface {
	// Read reads available bytes into data and returns the count (0 if none).
	Read(data []byte) int
	// Write writes data to the transport and returns the count written.
	Write(data []byte) int
	// Flush discards any buffered transmit data.
	Flush()
}

// SerialChannel implements Channel over a serial port.
type SerialChannel struct {
	Device   string
	baudRate int
	fd       int
}

// FIFOChannel implements Channel over a pair of named pipes (useful for testing).
type FIFOChannel struct {
	RxFIFO string
	TxFIFO string
	rxFd   int
	txFd   int
}

// NewSerialChannel creates a new serial communication channel
func NewSerialChannel(device string, baudRate int) (*SerialChannel, error) {
	ch := &SerialChannel{
		Device:   device,
		baudRate: baudRate,
		fd:       -1,
	}

	fd, err := syscall.Open(device, syscall.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to open serial device %s: %v", device, err)
	}

	ch.fd = fd
	return ch, nil
}

// Read implements Channel.Read for serial communication
func (sc *SerialChannel) Read(data []byte) int {
	if sc.fd < 0 {
		return -1
	}

	n, err := syscall.Read(sc.fd, data)
	if err != nil {
		return -1
	}
	return n
}

// Write implements Channel.Write for serial communication
func (sc *SerialChannel) Write(data []byte) int {
	if sc.fd < 0 {
		return -1
	}

	n, err := syscall.Write(sc.fd, data)
	if err != nil {
		return -1
	}
	return n
}

// Flush implements Channel.Flush for serial communication
func (sc *SerialChannel) Flush() {
	if sc.fd >= 0 {
		syscall.Syscall(syscall.SYS_FSYNC, uintptr(sc.fd), 0, 0)
	}
}

// Close closes the serial channel
func (sc *SerialChannel) Close() error {
	if sc.fd >= 0 {
		err := syscall.Close(sc.fd)
		sc.fd = -1
		return err
	}
	return nil
}

// NewFIFOChannel creates a new FIFO-based communication channel (for testing)
func NewFIFOChannel(rxFIFO, txFIFO string) (*FIFOChannel, error) {
	ch := &FIFOChannel{
		RxFIFO: rxFIFO,
		TxFIFO: txFIFO,
		rxFd:   -1,
		txFd:   -1,
	}

	// Don't open FIFOs here to avoid blocking - they will be opened lazily on first use
	return ch, nil
}

// openFIFOs opens the FIFOs if they haven't been opened yet
func (fc *FIFOChannel) openFIFOs() error {
	if fc.rxFd < 0 {
		// Open RX FIFO (for reading) - use O_RDONLY | O_NONBLOCK to avoid blocking
		rxFd, err := syscall.Open(fc.RxFIFO, syscall.O_RDONLY|syscall.O_NONBLOCK, 0)
		if err != nil {
			return fmt.Errorf("failed to open RX FIFO %s: %v", fc.RxFIFO, err)
		}
		fc.rxFd = rxFd
	}

	if fc.txFd < 0 {
		// Open TX FIFO (for writing) - use O_WRONLY (blocking writes are OK)
		txFd, err := syscall.Open(fc.TxFIFO, syscall.O_WRONLY, 0)
		if err != nil {
			return fmt.Errorf("failed to open TX FIFO %s: %v", fc.TxFIFO, err)
		}
		fc.txFd = txFd
	}

	return nil
}

// Read implements Channel.Read for FIFO communication
func (fc *FIFOChannel) Read(data []byte) int {
	if fc.rxFd < 0 {
		if err := fc.openFIFOs(); err != nil {
			return -1
		}
	}

	n, err := syscall.Read(fc.rxFd, data)
	if err != nil {
		if err == syscall.EAGAIN || err == syscall.EWOULDBLOCK {
			return 0 // No data available
		}
		return -1
	}
	return n
}

// Write implements Channel.Write for FIFO communication
func (fc *FIFOChannel) Write(data []byte) int {
	if fc.txFd < 0 {
		if err := fc.openFIFOs(); err != nil {
			return -1
		}
	}

	n, err := syscall.Write(fc.txFd, data)
	if err != nil {
		return -1
	}
	return n
}

// Flush implements Channel.Flush for FIFO communication
func (fc *FIFOChannel) Flush() {
	if fc.txFd >= 0 {
		syscall.Syscall(syscall.SYS_FSYNC, uintptr(fc.txFd), 0, 0)
	}
}

// Close closes the FIFO channel
func (fc *FIFOChannel) Close() error {
	var err error
	if fc.rxFd >= 0 {
		syscall.Close(fc.rxFd)
		fc.rxFd = -1
	}
	if fc.txFd >= 0 {
		err = syscall.Close(fc.txFd)
		fc.txFd = -1
	}
	return err
}

var (
	channelMu       sync.RWMutex
	channelRegistry = make(map[uintptr]Channel)
	channelCounter  uintptr
)

func channelLookup(handle uintptr) (Channel, bool) {
	channelMu.RLock()
	defer channelMu.RUnlock()
	ch, ok := channelRegistry[handle]
	return ch, ok
}

// handlePtr fabricates the opaque void* libosdp stores and hands back to our
// callbacks from an integer registry handle. The integer->pointer cast happens
// in C (see uintptr_to_ptr) to stay clear of Go's checkptr rules.
func handlePtr(h uintptr) unsafe.Pointer {
	return C.uintptr_to_ptr(C.uintptr_t(h))
}

// CreateOSDPChannel creates an OSDP channel from a Go Channel interface
func CreateOSDPChannel(ch Channel) *C.struct_osdp_channel {
	channelMu.Lock()
	channelCounter++
	handle := channelCounter
	channelRegistry[handle] = ch
	channelMu.Unlock()

	return C.create_channel(handlePtr(handle))
}

// channelSendWrapper is the C callback wrapper for channel send
//
//export channelSendWrapper
func channelSendWrapper(chPtr unsafe.Pointer, buf *C.uint8_t, length C.int) C.int {
	ch, ok := channelLookup(uintptr(chPtr))
	if !ok {
		return C.int(-1)
	}
	data := C.GoBytes(unsafe.Pointer(buf), length)
	return C.int(ch.Write(data))
}

//export channelRecvWrapper
func channelRecvWrapper(chPtr unsafe.Pointer, buf *C.uint8_t, length C.int) C.int {
	ch, ok := channelLookup(uintptr(chPtr))
	if !ok {
		return C.int(-1)
	}
	data := make([]byte, int(length))
	n := ch.Read(data)
	if n > 0 {
		copy((*[1 << 30]byte)(unsafe.Pointer(buf))[:n], data[:n])
	}
	return C.int(n)
}

//export channelFlushWrapper
func channelFlushWrapper(chPtr unsafe.Pointer) {
	if ch, ok := channelLookup(uintptr(chPtr)); ok {
		ch.Flush()
	}
}

// FreeOSDPChannel frees an OSDP channel
func FreeOSDPChannel(ch *C.struct_osdp_channel) {
	if ch == nil {
		return
	}
	handle := uintptr(unsafe.Pointer(ch.data))
	channelMu.Lock()
	delete(channelRegistry, handle)
	channelMu.Unlock()
	C.free_channel(ch)
}
