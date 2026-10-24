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
	cpMu       sync.RWMutex
	cpRegistry = make(map[uintptr]*ControlPanel)
	cpCounter  uintptr
)

func cpLookup(handle uintptr) (*ControlPanel, bool) {
	cpMu.RLock()
	defer cpMu.RUnlock()
	cp, ok := cpRegistry[handle]
	return cp, ok
}

// EventHandler is the callback invoked when a CP receives an event from a PD.
type EventHandler func(pdNumber int, event *Event) int

// CommandCompletionHandler is invoked when a CP command submitted via
// SubmitCommand reaches a terminal state (delivered, failed, flushed, aborted).
type CommandCompletionHandler func(pdNumber int, command *Command, status CompletionStatus)

// ControlPanel represents an OSDP Control Panel that manages one or more Peripheral Devices
type ControlPanel struct {
	ctx                  *C.osdp_t
	handle               uintptr
	pdInfoList           []*PdInfo
	eventHandler         EventHandler
	cmdCompletionHandler CommandCompletionHandler
	refreshContext       *refreshLoop
	mu                   sync.Mutex
	eventQueues          []chan *Event
	channel              *C.struct_osdp_channel

	// pendingCmds tracks C-allocated osdp_cmd structs submitted to libosdp.
	// As with PD events, the CP command queue is intrusive (zero-copy): libosdp
	// keeps the app-owned struct alive across retries and frees nothing itself,
	// signalling completion only via the command completion callback. The memory
	// must outlive the submit and is freed in cpCommandCompletionWrapper.
	pendingMu    sync.Mutex
	pendingCmds  map[*C.struct_osdp_cmd]struct{}
	shutdownOnce sync.Once
}

// ControlPanelBuilder collects the PDs on one OSDP bus and builds the
// ControlPanel that drives them. The PD list is fixed once the CP is built.
type ControlPanelBuilder struct {
	channel  Channel
	pdInfos  []*PdInfo
	logLevel LogLevel
}

// NewControlPanelBuilder starts building a CP on channel, the shared
// multi-drop bus that all of its PDs sit on. Logging defaults to LogInfo.
func NewControlPanelBuilder(channel Channel) *ControlPanelBuilder {
	return &ControlPanelBuilder{channel: channel, logLevel: LogInfo}
}

// AddPD puts a PD on the bus. The ControlPanel addresses PDs by the order they
// were added in, starting at 0.
func (b *ControlPanelBuilder) AddPD(info *PdInfo) *ControlPanelBuilder {
	b.pdInfos = append(b.pdInfos, info)
	return b
}

// LogLevel sets the most verbose libosdp log level that is reported.
func (b *ControlPanelBuilder) LogLevel(level LogLevel) *ControlPanelBuilder {
	b.logLevel = level
	return b
}

// Build sets up the CP and starts it talking on the bus before returning; call
// Stop to shut it down.
func (b *ControlPanelBuilder) Build() (*ControlPanel, error) {
	channel := b.channel
	pdInfoList := append([]*PdInfo(nil), b.pdInfos...)
	if channel == nil {
		return nil, fmt.Errorf("channel is required")
	}
	if len(pdInfoList) == 0 {
		return nil, fmt.Errorf("at least one PD info is required")
	}
	for i, info := range pdInfoList {
		if info == nil {
			return nil, fmt.Errorf("PD %d: PD info is required", i)
		}
	}

	cp := &ControlPanel{
		pdInfoList:  pdInfoList,
		eventQueues: make([]chan *Event, len(pdInfoList)),
		pendingCmds: make(map[*C.struct_osdp_cmd]struct{}),
		refreshContext: &refreshLoop{
			stop:     make(chan bool),
			stopped:  make(chan bool),
			interval: 20 * time.Millisecond,
		},
	}

	for i := range cp.eventQueues {
		cp.eventQueues[i] = make(chan *Event, 100)
	}

	setupLogging(b.logLevel)

	cp.channel = newCChannel(channel)

	numPDs := C.int(len(pdInfoList))
	pdInfoArray := C.malloc(C.size_t(numPDs) * C.size_t(unsafe.Sizeof(C.osdp_pd_info_t{})))
	defer C.free(pdInfoArray)
	pdInfoPtr := (*[1 << 30]C.osdp_pd_info_t)(pdInfoArray)

	cleanups := make([]func(), 0, len(pdInfoList))
	defer func() {
		for _, c := range cleanups {
			c()
		}
	}()
	for i, pdInfo := range pdInfoList {
		cInfo, cleanup := pdInfo.asCStruct()
		cleanups = append(cleanups, cleanup)
		pdInfoPtr[i] = cInfo
	}

	cp.ctx = (*C.osdp_t)(C.osdp_cp_setup(cp.channel, numPDs, (*C.osdp_pd_info_t)(pdInfoArray)))
	if cp.ctx == nil {
		freeCChannel(cp.channel)
		cp.channel = nil
		return nil, fmt.Errorf("failed to create OSDP CP context")
	}

	// Register this CP against an opaque handle and wire the C callbacks up
	// front. The event callback always feeds the per-PD event queues (so
	// GetEvent works without an explicit handler), and the command completion
	// callback owns freeing of submitted command memory (see pendingCmds).
	cpMu.Lock()
	cpCounter++
	cp.handle = cpCounter
	cpRegistry[cp.handle] = cp
	cpMu.Unlock()
	C.set_event_callback_wrapper(unsafe.Pointer(cp.ctx), handlePtr(cp.handle))
	C.set_command_completion_wrapper(unsafe.Pointer(cp.ctx), handlePtr(cp.handle))

	runtime.SetFinalizer(cp, (*ControlPanel).cleanup)

	go cp.refreshWorker(cp.refreshContext)

	return cp, nil
}

// Stop stops the control panel and releases all associated resources.
// It is idempotent and is also invoked automatically via a finalizer.
func (cp *ControlPanel) Stop() {
	cp.shutdown()
}

// cleanup is the finalizer entry point.
func (cp *ControlPanel) cleanup() {
	cp.shutdown()
}

// shutdown tears down the CP exactly once.
func (cp *ControlPanel) shutdown() {
	cp.shutdownOnce.Do(func() {
		// Stop the refresh worker first so no further C calls run concurrently
		// with teardown.
		cp.mu.Lock()
		refreshCtx := cp.refreshContext
		cp.refreshContext = nil
		cp.mu.Unlock()

		if refreshCtx != nil {
			close(refreshCtx.stop)
			select {
			case <-refreshCtx.stopped:
				// Worker stopped cleanly
			case <-time.After(2 * time.Second):
				// Timeout - worker might be blocked in C code; continue anyway
			}
		}

		// Teardown aborts any queued commands, invoking cpCommandCompletionWrapper
		// (with ABORTED status) which frees their memory and clears pendingCmds.
		cp.mu.Lock()
		if cp.ctx != nil {
			// SAFETY: cp.ctx is a valid C osdp_t pointer from osdp_cp_setup
			C.osdp_cp_teardown(unsafe.Pointer(cp.ctx))
			cp.ctx = nil
		}
		cp.mu.Unlock()

		// Free any command memory not reclaimed via the completion callback.
		cp.pendingMu.Lock()
		for cmd := range cp.pendingCmds {
			C.free(unsafe.Pointer(cmd))
		}
		cp.pendingCmds = nil
		cp.pendingMu.Unlock()

		// Unregister the handle only after teardown so completion callbacks
		// fired during teardown can still resolve this CP.
		cpMu.Lock()
		delete(cpRegistry, cp.handle)
		cpMu.Unlock()

		if cp.channel != nil {
			freeCChannel(cp.channel)
			cp.channel = nil
		}

		// Close event queues
		for _, queue := range cp.eventQueues {
			close(queue)
		}
	})
}

// refreshWorker runs the refresh loop in a separate goroutine. It captures its
// refreshLoop locally so it never races with shutdown clearing the field.
func (cp *ControlPanel) refreshWorker(rc *refreshLoop) {
	defer close(rc.stopped)

	ticker := time.NewTicker(rc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-rc.stop:
			return
		case <-ticker.C:
			cp.mu.Lock()
			if cp.ctx != nil {
				C.osdp_cp_refresh(unsafe.Pointer(cp.ctx))
			}
			cp.mu.Unlock()
		}
	}
}

// SubmitCommand queues a command for a specific PD. The command is delivered to
// the PD at the next opportunity; a successful return only means libosdp
// accepted the submission. Terminal status arrives via the command completion
// handler (see SetCommandCompletionHandler).
func (cp *ControlPanel) SubmitCommand(pdNumber int, cmd *Command) error {
	if pdNumber < 0 || pdNumber >= len(cp.pdInfoList) {
		return fmt.Errorf("invalid PD number: %d", pdNumber)
	}

	cp.mu.Lock()
	defer cp.mu.Unlock()

	if cp.ctx == nil {
		return fmt.Errorf("control panel context is not initialized")
	}

	cCmd, err := commandToC(cmd)
	if err != nil {
		return err
	}

	// libosdp takes ownership of cCmd via its intrusive zero-copy queue; it must
	// outlive this call and is freed in cpCommandCompletionWrapper. Track it as
	// pending before submitting so the completion callback (or teardown) can
	// reclaim it.
	cp.pendingMu.Lock()
	cp.pendingCmds[cCmd] = struct{}{}
	cp.pendingMu.Unlock()

	ret := C.osdp_cp_submit_command(unsafe.Pointer(cp.ctx), C.int(pdNumber), cCmd)
	if ret != 0 {
		cp.pendingMu.Lock()
		delete(cp.pendingCmds, cCmd)
		cp.pendingMu.Unlock()
		C.free(unsafe.Pointer(cCmd))
		return fmt.Errorf("failed to submit command, error code: %d", ret)
	}

	return nil
}

// GetEvent retrieves an event from the event queue for a specific PD
func (cp *ControlPanel) GetEvent(pdNumber int, timeout time.Duration) (*Event, error) {
	if pdNumber < 0 || pdNumber >= len(cp.pdInfoList) {
		return nil, fmt.Errorf("invalid PD number: %d", pdNumber)
	}

	select {
	case event := <-cp.eventQueues[pdNumber]:
		return event, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("timeout waiting for event")
	}
}

// SetEventHandler sets or updates the event handler
func (cp *ControlPanel) SetEventHandler(handler EventHandler) error {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if cp.ctx == nil {
		return fmt.Errorf("control panel context is not initialized")
	}

	// The event callback is registered once at construction and always feeds the
	// per-PD event queues; here we only record the user handler it should invoke.
	cp.eventHandler = handler
	return nil
}

// SetCommandCompletionHandler registers a callback invoked when a command
// submitted via SubmitCommand reaches a terminal state (delivered, failed,
// flushed or aborted). The handler runs on its own goroutine.
func (cp *ControlPanel) SetCommandCompletionHandler(handler CommandCompletionHandler) error {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	cp.cmdCompletionHandler = handler
	return nil
}

// FlushCommands deletes all commands queued for a given PD
func (cp *ControlPanel) FlushCommands(pdNumber int) (int, error) {
	if pdNumber < 0 || pdNumber >= len(cp.pdInfoList) {
		return 0, fmt.Errorf("invalid PD number: %d", pdNumber)
	}

	cp.mu.Lock()
	defer cp.mu.Unlock()

	if cp.ctx == nil {
		return 0, fmt.Errorf("control panel context is not initialized")
	}

	count := C.osdp_cp_flush_commands(unsafe.Pointer(cp.ctx), C.int(pdNumber))
	return int(count), nil
}

// Cancel asks the running multipart operation of kind what (file transfer,
// PIV, GENAUTH, CRAUTH or multipart BIOREAD) on a PD to stop. It is
// cooperative: the operation ends on a later refresh and its command then
// completes with CompletionFailed, with MPOutcomeAborted on the MP_DONE
// notification. It errors if no such operation is running.
func (cp *ControlPanel) Cancel(pdNumber int, what MPMsgType) error {
	if pdNumber < 0 || pdNumber >= len(cp.pdInfoList) {
		return fmt.Errorf("invalid PD number: %d", pdNumber)
	}

	cp.mu.Lock()
	defer cp.mu.Unlock()

	if cp.ctx == nil {
		return fmt.Errorf("control panel context is not initialized")
	}
	if C.osdp_cp_cancel(unsafe.Pointer(cp.ctx), C.int(pdNumber), C.enum_osdp_mp_msg_type(what)) != 0 {
		return fmt.Errorf("no %v operation in progress on PD %d", what, pdNumber)
	}
	return nil
}

// GetPDID retrieves PD ID information as reported by the PD
func (cp *ControlPanel) GetPDID(pdNumber int) (*PdId, error) {
	if pdNumber < 0 || pdNumber >= len(cp.pdInfoList) {
		return nil, fmt.Errorf("invalid PD number: %d", pdNumber)
	}

	cp.mu.Lock()
	defer cp.mu.Unlock()

	if cp.ctx == nil {
		return nil, fmt.Errorf("control panel context is not initialized")
	}

	var cId C.struct_osdp_pd_id
	ret := C.osdp_cp_get_pd_id(unsafe.Pointer(cp.ctx), C.int(pdNumber), &cId)
	if ret != 0 {
		return nil, fmt.Errorf("failed to get PD ID, error code: %d", ret)
	}

	pdId := &PdId{
		Version:       int(cId.version),
		Model:         int(cId.model),
		VendorCode:    int(cId.vendor_code),
		SerialNumber:  int(cId.serial_number),
		FirmwareMajor: int((cId.firmware_version >> 16) & 0xFF),
		FirmwareMinor: int((cId.firmware_version >> 8) & 0xFF),
		FirmwareBuild: int(cId.firmware_version & 0xFF),
	}

	return pdId, nil
}

// GetPDCapability retrieves PD capability for a specific function code
func (cp *ControlPanel) GetPDCapability(pdNumber int, functionCode PdCapabilityFunction) (*PdCapability, error) {
	if pdNumber < 0 || pdNumber >= len(cp.pdInfoList) {
		return nil, fmt.Errorf("invalid PD number: %d", pdNumber)
	}

	cp.mu.Lock()
	defer cp.mu.Unlock()

	if cp.ctx == nil {
		return nil, fmt.Errorf("control panel context is not initialized")
	}

	cap := C.struct_osdp_pd_cap{
		function_code: C.enum_osdp_pd_cap_function_code_e(functionCode),
	}

	ret := C.osdp_cp_get_capability(unsafe.Pointer(cp.ctx), C.int(pdNumber), &cap)
	if ret != 0 {
		return nil, fmt.Errorf("failed to get PD capability, error code: %d", ret)
	}

	capability := &PdCapability{
		Function:   PdCapabilityFunction(cap.function_code),
		Compliance: int(cap.compliance_level),
		NumItems:   int(cap.num_items),
	}

	return capability, nil
}

// ModifyFlag sets or clears OSDP public flags for a PD
func (cp *ControlPanel) ModifyFlag(pdNumber int, flags OsdpFlag, doSet bool) error {
	if pdNumber < 0 || pdNumber >= len(cp.pdInfoList) {
		return fmt.Errorf("invalid PD number: %d", pdNumber)
	}

	cp.mu.Lock()
	defer cp.mu.Unlock()

	if cp.ctx == nil {
		return fmt.Errorf("control panel context is not initialized")
	}

	ret := C.osdp_cp_modify_flag(unsafe.Pointer(cp.ctx), C.int(pdNumber), C.uint32_t(flags), C.bool(doSet))
	if ret != 0 {
		return fmt.Errorf("failed to modify flag, error code: %d", ret)
	}

	return nil
}

// DisablePD asks the CP to stop polling a PD. The request takes effect on the
// next refresh cycle, so IsPDEnabled reports false shortly after, not at once.
// It errors if the PD is already disabled or a disable is already pending.
func (cp *ControlPanel) DisablePD(pdNumber int) error {
	if pdNumber < 0 || pdNumber >= len(cp.pdInfoList) {
		return fmt.Errorf("invalid PD number: %d", pdNumber)
	}

	cp.mu.Lock()
	defer cp.mu.Unlock()

	if cp.ctx == nil {
		return fmt.Errorf("control panel context is not initialized")
	}

	ret := C.osdp_cp_disable_pd(unsafe.Pointer(cp.ctx), C.int(pdNumber))
	if ret != 0 {
		return fmt.Errorf("failed to disable PD, error code: %d", ret)
	}

	return nil
}

// EnablePD asks the CP to resume polling a disabled PD; like DisablePD it takes
// effect on the next refresh cycle.
func (cp *ControlPanel) EnablePD(pdNumber int) error {
	if pdNumber < 0 || pdNumber >= len(cp.pdInfoList) {
		return fmt.Errorf("invalid PD number: %d", pdNumber)
	}

	cp.mu.Lock()
	defer cp.mu.Unlock()

	if cp.ctx == nil {
		return fmt.Errorf("control panel context is not initialized")
	}

	ret := C.osdp_cp_enable_pd(unsafe.Pointer(cp.ctx), C.int(pdNumber))
	if ret != 0 {
		return fmt.Errorf("failed to enable PD, error code: %d", ret)
	}

	return nil
}

// IsPDEnabled checks if a PD is currently enabled
func (cp *ControlPanel) IsPDEnabled(pdNumber int) (bool, error) {
	if pdNumber < 0 || pdNumber >= len(cp.pdInfoList) {
		return false, fmt.Errorf("invalid PD number: %d", pdNumber)
	}

	cp.mu.Lock()
	defer cp.mu.Unlock()

	if cp.ctx == nil {
		return false, fmt.Errorf("control panel context is not initialized")
	}

	enabled := C.osdp_cp_is_pd_enabled(unsafe.Pointer(cp.ctx), C.int(pdNumber))
	return bool(enabled), nil
}

// eventCallbackWrapper is the C callback wrapper for events
//
//export eventCallbackWrapper
func eventCallbackWrapper(data unsafe.Pointer, pd C.int, cEvent *C.struct_osdp_event) C.int {
	cp, ok := cpLookup(uintptr(data))
	if !ok {
		return C.int(-1)
	}

	event := decodeEvent(cEvent)
	event.PDNumber = int(pd)

	// Add to event queue
	if int(pd) < len(cp.eventQueues) {
		select {
		case cp.eventQueues[pd] <- event:
		default:
			// Queue is full, drop oldest event
			<-cp.eventQueues[pd]
			cp.eventQueues[pd] <- event
		}
	}

	// Call user event handler if set
	if cp.eventHandler != nil {
		return C.int(cp.eventHandler(int(pd), event))
	}

	return 0 // Success
}

// cpCommandCompletionWrapper is invoked by libosdp when a submitted command
// reaches a terminal state. It frees the app-owned command memory that was kept
// alive across the submit and optionally notifies the user's completion handler.
//
//export cpCommandCompletionWrapper
func cpCommandCompletionWrapper(arg unsafe.Pointer, pd C.int, cCmd *C.struct_osdp_cmd, status C.int) {
	cp, ok := cpLookup(uintptr(arg))
	if !ok {
		return
	}

	// Decode before freeing if a handler will consume it.
	var goCmd *Command
	if cp.cmdCompletionHandler != nil {
		goCmd = decodeCommand(cCmd)
	}

	cp.pendingMu.Lock()
	if _, tracked := cp.pendingCmds[cCmd]; tracked {
		delete(cp.pendingCmds, cCmd)
		C.free(unsafe.Pointer(cCmd))
	}
	cp.pendingMu.Unlock()

	if cp.cmdCompletionHandler != nil {
		handler := cp.cmdCompletionHandler
		// Dispatch off the refresh goroutine so the handler may call CP APIs.
		go handler(int(pd), goCmd, CompletionStatus(status))
	}
}

// SetFlag sets one or more OSDP flags on a PD.
func (cp *ControlPanel) SetFlag(pdNumber int, flag OsdpFlag) error {
	return cp.ModifyFlag(pdNumber, flag, true)
}

// ClearFlag clears one or more OSDP flags on a PD.
func (cp *ControlPanel) ClearFlag(pdNumber int, flag OsdpFlag) error {
	return cp.ModifyFlag(pdNumber, flag, false)
}
