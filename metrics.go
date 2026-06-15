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
	"unsafe"
)

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
