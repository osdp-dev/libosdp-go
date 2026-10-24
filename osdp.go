/*
 * Copyright (c) 2026 Siddharth Chandrasekaran <sidcha@osdp.dev>
 *
 * SPDX-License-Identifier: Apache-2.0
 */

/*
Package osdp provides Go bindings for the LibOSDP C library.

LibOSDP is a library implementation of IEC 60839-11-5 Open Supervised Device
Protocol (OSDP), a communication protocol for secure exchange between Control
Panels (CP) and Peripheral Devices (PD) in access control systems. See
https://osdp.dev for an overview of the protocol and the LibOSDP project.

The package wraps the C API with idiomatic Go: interfaces for channels,
goroutine-managed refresh loops (20ms per the OSDP standard), and Go errors
in place of integer status codes.

# Getting Started

Two main types: ControlPanel (one bus, one or more PDs) and PeripheralDevice
(one PD on a bus). Both take a Channel as their first constructor argument.

	channel, err := osdp.NewSerialChannel("/dev/ttyUSB0", 9600)
	if err != nil {
		return err
	}
	defer channel.Close()

	pdInfo := &osdp.PdInfo{
		Address:  1,
		BaudRate: 9600,
		Flags:    osdp.FlagEnforceSecure,
		SCBK:     scbk16Bytes,
	}

	cp, err := osdp.NewControlPanelBuilder(channel).AddPD(pdInfo).Build()
	if err != nil {
		return err
	}
	defer cp.Stop()

For a runnable PD example, a complete CP↔PD walkthrough, and patterns for
multi-drop buses, see the examples/ tree in the repository.

# Commands and Events

The core command and event types live in commands.go and events.go. The
biometric, PIV, and smartcard-authentication surface — CommandBioRead,
CommandBioMatch, CommandPIVData, CommandGenAuth, CommandCRAuth and their reply
events (EventBioReadR, EventBioMatchR, EventPIVData, EventGenAuthReply,
EventCRAuthReply) — lives in smartcard.go. libosdp reassembles the multipart
replies transparently; set FlagBioReadRMultipart on both roles for oversized
biometric templates. Notifications carry a typed Payload (see EventNotification
and NotificationData).

# Channels

Implement the Channel interface to wrap any transport. NewSerialChannel and
NewFIFOChannel are provided out of the box (FIFO for tests).

# Logging

LibOSDP routes log messages through a single global callback:

	osdp.SetLogHandler(func(pd int, level osdp.LogLevel, file string, line int, msg string) {
		log.Printf("[osdp pd=%d %s] %s:%d %s", pd, level, file, line, msg)
	})

Convenience helpers are provided for the standard log package
(SetStandardLogger), slog (SetSlogLogger), level filtering
(SetLevelFilteredLogger), and silencing (SetDiscardLogger).

# Thread Safety

Both ControlPanel and PeripheralDevice serialize their own state with an
internal mutex. The bindings spawn one goroutine per instance to drive the
refresh loop. User-facing methods are safe to call concurrently.

# Memory Management

C resources are released by Stop() (explicit) and by a runtime.SetFinalizer
backstop. Always call Stop() in production code; finalizers are best-effort.

# Compatibility

Requires Go 1.21+ and a C compiler. The LibOSDP C sources are vendored into the
internal/clib package and compiled by cgo, so no prebuilt library or external
link dependency is needed. Maintainers refresh the vendored copy from the
libosdp/ submodule with "go generate .".

# See Also

  - OSDP protocol and the LibOSDP project: https://osdp.dev
  - Documentation: https://doc.osdp.dev
  - LibOSDP C library: https://github.com/osdp-dev/libosdp
*/
package osdp

/*
#include "cgo_helpers.h"
*/
import "C"
import (
	"time"
	"unsafe"
)

// GetVersion returns the LibOSDP version string
func GetVersion() string {
	return C.GoString(C.osdp_get_version())
}

// GetSourceInfo returns source information about LibOSDP
func GetSourceInfo() string {
	return C.GoString(C.osdp_get_source_info())
}

// PdInfo represents peripheral device information.
// Channel is no longer carried per-PD; the C library now requires a single
// shared channel per CP (multi-drop bus). Pass the channel to
// NewControlPanelBuilder or NewPeripheralDevice instead.
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
// allocations referenced by the struct (the SCBK buffer and, if set, the
// capability array). The cleanup must be called after osdp_*_setup returns;
// libosdp copies these inputs into its own context.
//
// Capabilities are populated from p.Capabilities. libosdp uses cInfo.cap only in
// PD mode; on the CP side it is ignored. NewPeripheralDevice may override
// cInfo.cap with an explicitly-supplied capability argument.
func (p *PdInfo) asCStruct() (C.osdp_pd_info_t, func()) {
	var cInfo C.osdp_pd_info_t
	frees := []unsafe.Pointer{}

	cInfo.name = nil
	cInfo.baud_rate = C.uint32_t(p.BaudRate)
	cInfo.address = C.int(p.Address)
	cInfo.flags = C.uint32_t(p.Flags)

	cInfo.id.version = C.int(p.ID.Version)
	cInfo.id.model = C.int(p.ID.Model)
	cInfo.id.vendor_code = C.uint(p.ID.VendorCode)
	cInfo.id.serial_number = C.uint(p.ID.SerialNumber)
	cInfo.id.firmware_version = C.uint((p.ID.FirmwareMajor << 16) | (p.ID.FirmwareMinor << 8) | p.ID.FirmwareBuild)

	if len(p.Capabilities) > 0 {
		n := len(p.Capabilities)
		capPtr := (*C.struct_osdp_pd_cap)(C.malloc(C.size_t(n+1) * C.size_t(unsafe.Sizeof(C.struct_osdp_pd_cap{}))))
		capArr := (*[1 << 20]C.struct_osdp_pd_cap)(unsafe.Pointer(capPtr))
		for i, c := range p.Capabilities {
			capArr[i].function_code = C.enum_osdp_pd_cap_function_code_e(c.Function)
			capArr[i].compliance_level = C.uchar(c.Compliance)
			capArr[i].num_items = C.uchar(c.NumItems)
		}
		capArr[n].function_code = C.OSDP_PD_CAP_SENTINEL
		capArr[n].compliance_level = 0
		capArr[n].num_items = 0
		cInfo.cap = capPtr
		frees = append(frees, unsafe.Pointer(capPtr))
	} else {
		cInfo.cap = nil
	}

	if len(p.SCBK) == 16 {
		scbkPtr := (*C.uint8_t)(C.malloc(16))
		for i := 0; i < 16; i++ {
			*(*C.uint8_t)(unsafe.Pointer(uintptr(unsafe.Pointer(scbkPtr)) + uintptr(i))) = C.uint8_t(p.SCBK[i])
		}
		cInfo.scbk = scbkPtr
		frees = append(frees, unsafe.Pointer(scbkPtr))
	} else {
		cInfo.scbk = nil
	}

	cleanup := func() {
		for _, ptr := range frees {
			C.free(ptr)
		}
	}
	return cInfo, cleanup
}

// refreshLoop holds the signalling channels for a CP/PD refresh goroutine.
type refreshLoop struct {
	stop     chan bool
	stopped  chan bool
	interval time.Duration
}
