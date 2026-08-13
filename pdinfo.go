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
import "unsafe"

// PdInfo represents peripheral device information.
// Channel is no longer carried per-PD; the C library now requires a single
// shared channel per CP (multi-drop bus). Pass the channel to NewControlPanel
// or NewPeripheralDevice instead.
type PdInfo struct {
	Address      int
	BaudRate     int
	Flags        OsdpFlag
	SCBK         []byte // Secure Channel Base Key (16 bytes)
	ID           PdId
	Capabilities []PdCapability
}

// PdId represents PD identification information
type PdId struct {
	Version       int
	Model         int
	VendorCode    int
	SerialNumber  int
	FirmwareMajor int
	FirmwareMinor int
	FirmwareBuild int
}

// PdCapability represents a PD capability with compliance level and number of items
type PdCapability struct {
	Function   PdCapabilityFunction
	Compliance int
	NumItems   int
}

// asCStruct converts PdInfo to a C osdp_pd_info_t.
// Returns the populated struct and a cleanup func that releases any C-side
// allocations referenced by the struct (currently just the SCBK buffer).
// The cleanup must be called after osdp_*_setup returns; libosdp copies
// these inputs into its own context.
func (p *PdInfo) asCStruct() (C.osdp_pd_info_t, func()) {
	var cInfo C.osdp_pd_info_t
	cleanup := func() {}

	cInfo.name = nil
	cInfo.baud_rate = C.uint32_t(p.BaudRate)
	cInfo.address = C.int(p.Address)
	cInfo.flags = C.uint32_t(p.Flags)

	cInfo.id.version = C.int(p.ID.Version)
	cInfo.id.model = C.int(p.ID.Model)
	cInfo.id.vendor_code = C.uint(p.ID.VendorCode)
	cInfo.id.serial_number = C.uint(p.ID.SerialNumber)
	cInfo.id.firmware_version = C.uint((p.ID.FirmwareMajor << 16) | (p.ID.FirmwareMinor << 8) | p.ID.FirmwareBuild)

	cInfo.cap = nil

	if len(p.SCBK) == 16 {
		scbkPtr := (*C.uint8_t)(C.malloc(16))
		for i := 0; i < 16; i++ {
			*(*C.uint8_t)(unsafe.Pointer(uintptr(unsafe.Pointer(scbkPtr)) + uintptr(i))) = C.uint8_t(p.SCBK[i])
		}
		cInfo.scbk = scbkPtr
		cleanup = func() { C.free(unsafe.Pointer(scbkPtr)) }
	} else {
		cInfo.scbk = nil
	}

	return cInfo, cleanup
}
