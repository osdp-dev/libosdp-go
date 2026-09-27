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
	"time"
	"unsafe"
)

// readMask fills a bitmask for all PDs. libosdp writes ceil(NumPD/8) bytes, so
// the buffer must be sized accordingly — passing a single byte overflows the
// stack for buses with more than 8 PDs. getMask must be one of the C
// osdp_get_*_status_mask functions. Caller must hold cp.mu and have a live ctx.
func (cp *ControlPanel) readMask(getMask func(unsafe.Pointer, *C.uint8_t)) []byte {
	nbytes := (len(cp.pdInfoList) + 7) / 8
	if nbytes == 0 {
		nbytes = 1
	}
	buf := make([]byte, nbytes)
	getMask(unsafe.Pointer(cp.ctx), (*C.uint8_t)(unsafe.Pointer(&buf[0])))
	return buf
}

func maskBit(buf []byte, pd int) bool {
	idx := pd / 8
	return idx < len(buf) && buf[idx]&(1<<uint(pd%8)) != 0
}

func popcount(buf []byte) int {
	n := 0
	for _, b := range buf {
		for ; b != 0; b &= b - 1 {
			n++
		}
	}
	return n
}

func maskToUint64(buf []byte) uint64 {
	var m uint64
	for i := 0; i < len(buf) && i < 8; i++ {
		m |= uint64(buf[i]) << (uint(i) * 8)
	}
	return m
}

func cStatusMask(c unsafe.Pointer, m *C.uint8_t)   { C.osdp_get_status_mask(c, m) }
func cSCStatusMask(c unsafe.Pointer, m *C.uint8_t) { C.osdp_get_sc_status_mask(c, m) }

// Status returns the online bitmask (bit i set => PD i is online). Values beyond
// 64 PDs are truncated; use IsOnline for buses that large.
func (cp *ControlPanel) Status() (uint64, error) {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	if cp.ctx == nil {
		return 0, fmt.Errorf("control panel context is not initialized")
	}
	return maskToUint64(cp.readMask(cStatusMask)), nil
}

// SCStatus returns the secure-channel bitmask (bit i set => PD i has an active
// secure channel).
func (cp *ControlPanel) SCStatus() (uint64, error) {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	if cp.ctx == nil {
		return 0, fmt.Errorf("control panel context is not initialized")
	}
	return maskToUint64(cp.readMask(cSCStatusMask)), nil
}

// IsOnline reports whether a specific PD is online.
func (cp *ControlPanel) IsOnline(pdNumber int) (bool, error) {
	if pdNumber < 0 || pdNumber >= len(cp.pdInfoList) {
		return false, fmt.Errorf("invalid PD number: %d", pdNumber)
	}
	cp.mu.Lock()
	defer cp.mu.Unlock()
	if cp.ctx == nil {
		return false, fmt.Errorf("control panel context is not initialized")
	}
	return maskBit(cp.readMask(cStatusMask), pdNumber), nil
}

// IsSCActive reports whether a specific PD has an active secure channel.
func (cp *ControlPanel) IsSCActive(pdNumber int) (bool, error) {
	if pdNumber < 0 || pdNumber >= len(cp.pdInfoList) {
		return false, fmt.Errorf("invalid PD number: %d", pdNumber)
	}
	cp.mu.Lock()
	defer cp.mu.Unlock()
	if cp.ctx == nil {
		return false, fmt.Errorf("control panel context is not initialized")
	}
	return maskBit(cp.readMask(cSCStatusMask), pdNumber), nil
}

// GetNumOnline returns the number of PDs currently online.
func (cp *ControlPanel) GetNumOnline() (int, error) {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	if cp.ctx == nil {
		return 0, fmt.Errorf("control panel context is not initialized")
	}
	return popcount(cp.readMask(cStatusMask)), nil
}

// GetNumSCActive returns the number of PDs with an active secure channel.
func (cp *ControlPanel) GetNumSCActive() (int, error) {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	if cp.ctx == nil {
		return 0, fmt.Errorf("control panel context is not initialized")
	}
	return popcount(cp.readMask(cSCStatusMask)), nil
}

// pollInterval is how often the *Wait helpers re-check status.
const pollInterval = 50 * time.Millisecond

func waitFor(timeout time.Duration, cond func() bool) bool {
	deadline := time.Now().Add(timeout)
	for {
		if cond() {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(pollInterval)
	}
}

// OnlineWait blocks until the given PD is online or timeout elapses.
func (cp *ControlPanel) OnlineWait(pdNumber int, timeout time.Duration) bool {
	return waitFor(timeout, func() bool { ok, _ := cp.IsOnline(pdNumber); return ok })
}

// OfflineWait blocks until the given PD is offline or timeout elapses.
func (cp *ControlPanel) OfflineWait(pdNumber int, timeout time.Duration) bool {
	return waitFor(timeout, func() bool { ok, err := cp.IsOnline(pdNumber); return err == nil && !ok })
}

// OnlineWaitAll blocks until every PD is online or timeout elapses.
func (cp *ControlPanel) OnlineWaitAll(timeout time.Duration) bool {
	return waitFor(timeout, func() bool { n, _ := cp.GetNumOnline(); return n == len(cp.pdInfoList) })
}

// SCWait blocks until the given PD has an active secure channel or timeout elapses.
func (cp *ControlPanel) SCWait(pdNumber int, timeout time.Duration) bool {
	return waitFor(timeout, func() bool { ok, _ := cp.IsSCActive(pdNumber); return ok })
}

// SCWaitAll blocks until every PD has an active secure channel or timeout elapses.
func (cp *ControlPanel) SCWaitAll(timeout time.Duration) bool {
	return waitFor(timeout, func() bool { n, _ := cp.GetNumSCActive(); return n == len(cp.pdInfoList) })
}

// Metrics holds per-context communication counters as returned by GetMetrics.
// Counters are cumulative since context setup (or since the previous GetMetrics
// call, depending on the libosdp build).
type Metrics struct {
	PacketsSent       uint32
	PacketsReceived   uint32
	PacketCheckErrors uint32
	NakCount          uint32
	SCHandshakeCount  uint32
	SCFailureCount    uint32
	CommandCount      uint32
	EventCount        uint32
}

func metricsFromC(m *C.struct_osdp_metrics) *Metrics {
	return &Metrics{
		PacketsSent:       uint32(m.packets_sent),
		PacketsReceived:   uint32(m.packets_received),
		PacketCheckErrors: uint32(m.packet_check_errors),
		NakCount:          uint32(m.nak_count),
		SCHandshakeCount:  uint32(m.sc_handshake_count),
		SCFailureCount:    uint32(m.sc_failure_count),
		CommandCount:      uint32(m.command_count),
		EventCount:        uint32(m.event_count),
	}
}

// GetMetrics returns the communication metrics for a specific PD.
func (cp *ControlPanel) GetMetrics(pdNumber int) (*Metrics, error) {
	if pdNumber < 0 || pdNumber >= len(cp.pdInfoList) {
		return nil, fmt.Errorf("invalid PD number: %d", pdNumber)
	}
	cp.mu.Lock()
	defer cp.mu.Unlock()
	if cp.ctx == nil {
		return nil, fmt.Errorf("control panel context is not initialized")
	}
	var m C.struct_osdp_metrics
	if ret := C.osdp_get_metrics(unsafe.Pointer(cp.ctx), C.int(pdNumber), &m); ret != 0 {
		return nil, fmt.Errorf("failed to get metrics, error code: %d", ret)
	}
	return metricsFromC(&m), nil
}

// GetMetrics returns the communication metrics for this PD.
func (pd *PeripheralDevice) GetMetrics() (*Metrics, error) {
	pd.mu.Lock()
	defer pd.mu.Unlock()
	if pd.ctx == nil {
		return nil, fmt.Errorf("peripheral device context is not initialized")
	}
	var m C.struct_osdp_metrics
	if ret := C.osdp_get_metrics(unsafe.Pointer(pd.ctx), 0, &m); ret != 0 {
		return nil, fmt.Errorf("failed to get metrics, error code: %d", ret)
	}
	return metricsFromC(&m), nil
}
