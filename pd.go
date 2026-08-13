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
	"runtime"
	"sync"
	"time"
	"unsafe"
)

var (
	pdMu       sync.RWMutex
	pdRegistry = make(map[uintptr]*PeripheralDevice)
	pdCounter  uintptr
)

func pdLookup(handle uintptr) (*PeripheralDevice, bool) {
	pdMu.RLock()
	defer pdMu.RUnlock()
	pd, ok := pdRegistry[handle]
	return pd, ok
}

// PeripheralDevice represents an OSDP Peripheral Device
type PeripheralDevice struct {
	ctx                    *C.osdp_t
	handle                 uintptr
	pdInfo                 *PdInfo
	capabilities           []PdCapability
	commandHandler         CommandHandler
	eventCompletionHandler EventCompletionHandler
	refreshContext         *RefreshContext
	mu                     sync.Mutex
	commandQueue           chan *Command
	channel                *C.struct_osdp_channel

	// pendingEvents tracks C-allocated osdp_event structs submitted to libosdp.
	// libosdp's event queue is intrusive (zero-copy): it stores a pointer to the
	// app-owned struct and dereferences it later during refresh, signalling it is
	// done only via the event completion callback. The memory must therefore stay
	// alive until pdEventCompletionWrapper fires; freeing on submit return is a
	// use-after-free. See pdEventCompletionWrapper.
	pendingMu     sync.Mutex
	pendingEvents map[*C.struct_osdp_event]struct{}
	shutdownOnce  sync.Once
}

// NewPeripheralDevice creates a new Peripheral Device instance.
// channel is the bus this PD is attached to.
func NewPeripheralDevice(channel Channel, pdInfo *PdInfo, capabilities []PdCapability, logLevel LogLevel) (*PeripheralDevice, error) {
	if channel == nil {
		return nil, fmt.Errorf("channel is required")
	}
	if pdInfo == nil {
		return nil, fmt.Errorf("PD info is required")
	}

	pd := &PeripheralDevice{
		pdInfo:        pdInfo,
		capabilities:  capabilities,
		commandQueue:  make(chan *Command, 100),
		pendingEvents: make(map[*C.struct_osdp_event]struct{}),
		refreshContext: &RefreshContext{
			stop:     make(chan bool),
			stopped:  make(chan bool),
			interval: 20 * time.Millisecond,
		},
	}

	InitializeLogging("osdp-pd", logLevel)

	pd.channel = CreateOSDPChannel(channel)

	cInfo, scbkCleanup := pdInfo.asCStruct()
	defer scbkCleanup()

	var capArrayPtr *C.struct_osdp_pd_cap
	if len(capabilities) > 0 {
		capCount := len(capabilities) + 1
		capArrayPtr = (*C.struct_osdp_pd_cap)(C.malloc(C.size_t(capCount) * C.size_t(unsafe.Sizeof(C.struct_osdp_pd_cap{}))))
		defer C.free(unsafe.Pointer(capArrayPtr))
		capArray := (*[1 << 30]C.struct_osdp_pd_cap)(unsafe.Pointer(capArrayPtr))

		for i, cap := range capabilities {
			capArray[i].function_code = C.enum_osdp_pd_cap_function_code_e(cap.Function)
			capArray[i].compliance_level = C.uchar(cap.Compliance)
			capArray[i].num_items = C.uchar(cap.NumItems)
		}
		capArray[len(capabilities)].function_code = C.OSDP_PD_CAP_SENTINEL
		capArray[len(capabilities)].compliance_level = 0
		capArray[len(capabilities)].num_items = 0

		cInfo.cap = capArrayPtr
	}

	pd.ctx = (*C.osdp_t)(C.osdp_pd_setup(pd.channel, &cInfo))
	if pd.ctx == nil {
		FreeOSDPChannel(pd.channel)
		pd.channel = nil
		return nil, fmt.Errorf("failed to create OSDP PD context")
	}

	// Register this PD against an opaque handle and wire the C callbacks up
	// front. The command callback always feeds the command queue (so GetCommand
	// works without an explicit handler) and the event completion callback owns
	// freeing of submitted event memory (see pendingEvents).
	pdMu.Lock()
	pdCounter++
	pd.handle = pdCounter
	pdRegistry[pd.handle] = pd
	pdMu.Unlock()
	C.set_command_callback_wrapper(unsafe.Pointer(pd.ctx), handlePtr(pd.handle))
	C.set_event_completion_wrapper(unsafe.Pointer(pd.ctx), handlePtr(pd.handle))

	runtime.SetFinalizer(pd, (*PeripheralDevice).cleanup)

	go pd.refreshWorker(pd.refreshContext)

	return pd, nil
}

// Start starts the peripheral device operations (explicit start for better control)
func (pd *PeripheralDevice) Start() error {
	// Peripheral device is started automatically in NewPeripheralDevice
	// This method is provided for API consistency with Python bindings
	return nil
}

// Stop stops the peripheral device and releases all associated resources.
// It is idempotent and is also invoked automatically via a finalizer.
func (pd *PeripheralDevice) Stop() {
	pd.shutdown()
}

// cleanup is the finalizer entry point.
func (pd *PeripheralDevice) cleanup() {
	pd.shutdown()
}

// shutdown tears down the PD exactly once.
func (pd *PeripheralDevice) shutdown() {
	pd.shutdownOnce.Do(func() {
		// Stop the refresh worker first so no further C calls run concurrently
		// with teardown.
		pd.mu.Lock()
		refreshCtx := pd.refreshContext
		pd.refreshContext = nil
		pd.mu.Unlock()

		if refreshCtx != nil {
			close(refreshCtx.stop)
			select {
			case <-refreshCtx.stopped:
				// Worker stopped cleanly
			case <-time.After(2 * time.Second):
				// Timeout - worker might be blocked in C code; continue anyway
			}
		}

		// Teardown aborts any queued events, invoking pdEventCompletionWrapper
		// (with ABORTED status) which frees their memory and clears pendingEvents.
		pd.mu.Lock()
		if pd.ctx != nil {
			// SAFETY: pd.ctx is a valid C osdp_t pointer from osdp_pd_setup
			C.osdp_pd_teardown(unsafe.Pointer(pd.ctx))
			pd.ctx = nil
		}
		pd.mu.Unlock()

		// Free any event memory not reclaimed via the completion callback.
		pd.pendingMu.Lock()
		for ev := range pd.pendingEvents {
			C.free(unsafe.Pointer(ev))
		}
		pd.pendingEvents = nil
		pd.pendingMu.Unlock()

		// Unregister the handle only after teardown so completion callbacks
		// fired during teardown can still resolve this PD.
		pdMu.Lock()
		delete(pdRegistry, pd.handle)
		pdMu.Unlock()

		if pd.channel != nil {
			FreeOSDPChannel(pd.channel)
			pd.channel = nil
		}

		close(pd.commandQueue)
	})
}

// refreshWorker runs the refresh loop in a separate goroutine. It captures its
// RefreshContext locally so it never races with shutdown clearing the field.
func (pd *PeripheralDevice) refreshWorker(rc *RefreshContext) {
	defer close(rc.stopped)

	ticker := time.NewTicker(rc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-rc.stop:
			return
		case <-ticker.C:
			pd.mu.Lock()
			if pd.ctx != nil {
				C.osdp_pd_refresh(unsafe.Pointer(pd.ctx))
			}
			pd.mu.Unlock()
		}
	}
}

// SubmitEvent queues an event for delivery to the CP. A successful return only
// means libosdp accepted the submission; terminal status arrives via the event
// completion handler (see SetEventCompletionHandler).
func (pd *PeripheralDevice) SubmitEvent(event *Event) error {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	if pd.ctx == nil {
		return fmt.Errorf("peripheral device context is not initialized")
	}

	cEvent := eventToC(event)
	if cEvent == nil {
		return fmt.Errorf("failed to convert event to C structure")
	}

	// libosdp takes ownership of cEvent via its intrusive zero-copy queue: the
	// struct must outlive this call and is freed in pdEventCompletionWrapper.
	// Track it as pending before submitting so the completion callback (or
	// teardown) can reclaim it.
	pd.pendingMu.Lock()
	pd.pendingEvents[cEvent] = struct{}{}
	pd.pendingMu.Unlock()

	ret := C.osdp_pd_submit_event(unsafe.Pointer(pd.ctx), cEvent)
	if ret != 0 {
		pd.pendingMu.Lock()
		delete(pd.pendingEvents, cEvent)
		pd.pendingMu.Unlock()
		C.free(unsafe.Pointer(cEvent))
		return fmt.Errorf("failed to submit event, error code: %d", ret)
	}

	return nil
}

// GetCommand retrieves a command from the command queue
func (pd *PeripheralDevice) GetCommand(timeout time.Duration) (*Command, error) {
	select {
	case cmd := <-pd.commandQueue:
		return cmd, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("timeout waiting for command")
	}
}

// SetCommandHandler sets or updates the command handler
func (pd *PeripheralDevice) SetCommandHandler(handler CommandHandler) error {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	if pd.ctx == nil {
		return fmt.Errorf("peripheral device context is not initialized")
	}

	// The command callback is registered once at construction and always feeds
	// the command queue; here we only record the user handler it should invoke.
	pd.commandHandler = handler
	return nil
}

// SetEventCompletionHandler registers a callback invoked when an event
// submitted via SubmitEvent reaches a terminal state (delivered, failed,
// flushed or aborted). The handler runs on its own goroutine.
func (pd *PeripheralDevice) SetEventCompletionHandler(handler EventCompletionHandler) error {
	pd.mu.Lock()
	defer pd.mu.Unlock()
	pd.eventCompletionHandler = handler
	return nil
}

// IsOnline checks if the PD is online
func (pd *PeripheralDevice) IsOnline() (bool, error) {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	if pd.ctx == nil {
		return false, fmt.Errorf("peripheral device context is not initialized")
	}

	var mask C.uint8_t
	C.osdp_get_status_mask(unsafe.Pointer(pd.ctx), &mask)
	return mask != 0, nil
}

// IsSCActive checks if secure channel is established
func (pd *PeripheralDevice) IsSCActive() (bool, error) {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	if pd.ctx == nil {
		return false, fmt.Errorf("peripheral device context is not initialized")
	}

	var mask C.uint8_t
	C.osdp_get_sc_status_mask(unsafe.Pointer(pd.ctx), &mask)
	return mask != 0, nil
}

// FlushEvents deletes all events from the PD's event queue
func (pd *PeripheralDevice) FlushEvents() (int, error) {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	if pd.ctx == nil {
		return 0, fmt.Errorf("peripheral device context is not initialized")
	}

	count := C.osdp_pd_flush_events(unsafe.Pointer(pd.ctx))
	return int(count), nil
}

// commandCallbackWrapper is the C callback wrapper for commands
//
//export commandCallbackWrapper
func commandCallbackWrapper(data unsafe.Pointer, cCmd *C.struct_osdp_cmd) C.int {
	pd, ok := pdLookup(uintptr(data))
	if !ok {
		return C.int(-1)
	}

	cmd := decodeCommand(cCmd)

	// Add to command queue
	select {
	case pd.commandQueue <- cmd:
	default:
		// Queue is full, drop oldest command
		<-pd.commandQueue
		pd.commandQueue <- cmd
	}

	// Call user command handler if set
	if pd.commandHandler != nil {
		ret, event := pd.commandHandler(cmd)
		if event != nil {
			// For status commands, populate the response in-place
			if cmd.Command.GetType() == CmdStatus {
				if statusEvent, ok := event.Event.(EventStatus); ok {
					// Populate the C command structure with the status response
					// in-place (one status byte per entry).
					reportBytes := make([]C.uint8_t, len(statusEvent.Report))
					for i, b := range statusEvent.Report {
						reportBytes[i] = C.uint8_t(b)
					}
					var reportPtr *C.uint8_t
					if len(reportBytes) > 0 {
						reportPtr = &reportBytes[0]
					}
					C.set_cmd_status_response(cCmd, C.uint8_t(statusEvent.Type),
						C.int(len(statusEvent.Report)), reportPtr)
					// Don't send as async notification - it's handled inline
					return C.int(ret)
				}
			}
			// For other events, send as async notification
			go func() {
				pd.SubmitEvent(event)
			}()
		}
		return C.int(ret)
	}

	return 0 // Success
}

// pdEventCompletionWrapper is invoked by libosdp when a submitted event reaches
// a terminal state. It frees the app-owned event memory that was kept alive
// across the submit and optionally notifies the user's completion handler.
//
//export pdEventCompletionWrapper
func pdEventCompletionWrapper(arg unsafe.Pointer, cEvent *C.struct_osdp_event, status C.int) {
	pd, ok := pdLookup(uintptr(arg))
	if !ok {
		return
	}

	// Decode before freeing if a handler will consume it.
	var goEvent *Event
	if pd.eventCompletionHandler != nil {
		goEvent = decodeEvent(cEvent)
	}

	pd.pendingMu.Lock()
	if _, tracked := pd.pendingEvents[cEvent]; tracked {
		delete(pd.pendingEvents, cEvent)
		C.free(unsafe.Pointer(cEvent))
	}
	pd.pendingMu.Unlock()

	if pd.eventCompletionHandler != nil {
		handler := pd.eventCompletionHandler
		// Dispatch off the refresh goroutine so the handler may call PD APIs.
		go handler(goEvent, CompletionStatus(status))
	}
}

// SetCapabilities replaces this PD's advertised capabilities. The capabilities
// are copied into libosdp; a 0xFF sentinel terminator is appended automatically.
func (pd *PeripheralDevice) SetCapabilities(capabilities []PdCapability) error {
	pd.mu.Lock()
	defer pd.mu.Unlock()
	if pd.ctx == nil {
		return fmt.Errorf("peripheral device context is not initialized")
	}

	arr := make([]C.struct_osdp_pd_cap, len(capabilities)+1)
	for i, c := range capabilities {
		arr[i].function_code = C.enum_osdp_pd_cap_function_code_e(c.Function)
		arr[i].compliance_level = C.uchar(c.Compliance)
		arr[i].num_items = C.uchar(c.NumItems)
	}
	arr[len(capabilities)].function_code = C.OSDP_PD_CAP_SENTINEL

	C.osdp_pd_set_capabilities(unsafe.Pointer(pd.ctx), &arr[0])
	pd.capabilities = capabilities
	return nil
}

// SCWait blocks until this PD has an active secure channel or timeout elapses.
func (pd *PeripheralDevice) SCWait(timeout time.Duration) bool {
	return waitFor(timeout, func() bool { ok, _ := pd.IsSCActive(); return ok })
}
