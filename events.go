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

// freeEvent releases a C osdp_event allocated by eventToC.
func freeEvent(c *C.struct_osdp_event) { C.free(unsafe.Pointer(c)) }

// Event represents an OSDP event
type Event struct {
	PDNumber int
	Event    EventData
}

// EventData represents the specific event data
type EventData interface {
	GetType() OsdpEventType
}

// EventCardRead represents card read event
type EventCardRead struct {
	ReaderNo  int32
	Format    OsdpCardFormat
	Direction bool
	NrBits    uint32
	Data      []byte
}

func (e EventCardRead) GetType() OsdpEventType { return EvtCardRead }

// EventKeyPress represents keypad event
type EventKeyPress struct {
	ReaderNo int32
	Data     []byte
}

func (e EventKeyPress) GetType() OsdpEventType { return EvtKeypress }

// EventStatus represents a status report event (PD->CP). Report holds one status
// byte per entry; the number of entries is len(Report) and must match the
// corresponding PD capability's num_items.
type EventStatus struct {
	Type   OsdpStatusReportType
	Report []byte
}

func (e EventStatus) GetType() OsdpEventType { return EvtStatus }

// EventNotification represents a libosdp notification event (synthesized when
// FlagEnableNotification is set). Arg0/Arg1 meaning depends on Type; see the
// NotificationType / FileTxOutcome docs.
type EventNotification struct {
	Type NotificationType
	Arg0 int
	Arg1 int
}

func (e EventNotification) GetType() OsdpEventType { return EvtNotification }

// EventMfgReply represents manufacturer reply event
type EventMfgReply struct {
	VendorCode [3]uint8
	Data       []byte
}

func (e EventMfgReply) GetType() OsdpEventType { return EvtMfgReply }

// eventToC converts a Go Event to a newly malloc'd C osdp_event structure. The
// caller owns the returned memory.
func eventToC(event *Event) (*C.struct_osdp_event, error) {
	cEvent := (*C.struct_osdp_event)(C.malloc(C.size_t(unsafe.Sizeof(C.struct_osdp_event{}))))
	if cEvent == nil {
		return nil, fmt.Errorf("osdp: failed to allocate event")
	}
	C.memset(unsafe.Pointer(cEvent), 0, C.size_t(unsafe.Sizeof(C.struct_osdp_event{})))

	cEvent._type = C.enum_osdp_event_type(event.Event.GetType())

	switch eventData := event.Event.(type) {
	case EventCardRead:
		if err := checkLen("cardread", len(eventData.Data), C.OSDP_EVENT_CARDREAD_MAX_DATALEN); err != nil {
			C.free(unsafe.Pointer(cEvent))
			return nil, err
		}
		cardData := make([]C.uint8_t, len(eventData.Data))
		for i, b := range eventData.Data {
			cardData[i] = C.uint8_t(b)
		}
		direction := C.uint8_t(0)
		if eventData.Direction {
			direction = C.uint8_t(1)
		}
		var dataPtr *C.uint8_t
		if len(cardData) > 0 {
			dataPtr = &cardData[0]
		}
		C.set_event_cardread(cEvent, C.uint8_t(eventData.ReaderNo), C.uint8_t(eventData.Format), direction,
			C.int(eventData.NrBits), dataPtr, C.int(len(eventData.Data)))

	case EventKeyPress:
		if err := checkLen("keypress", len(eventData.Data), C.OSDP_EVENT_KEYPRESS_MAX_DATALEN); err != nil {
			C.free(unsafe.Pointer(cEvent))
			return nil, err
		}
		keypadData := make([]C.uint8_t, len(eventData.Data))
		for i, b := range eventData.Data {
			keypadData[i] = C.uint8_t(b)
		}
		var keyPtr *C.uint8_t
		if len(keypadData) > 0 {
			keyPtr = &keypadData[0]
		}
		C.set_event_keypress(cEvent, C.uint8_t(eventData.ReaderNo), C.uint8_t(len(eventData.Data)), keyPtr)

	case EventStatus:
		if err := checkLen("status report", len(eventData.Report), C.OSDP_STATUS_REPORT_MAX_LEN); err != nil {
			C.free(unsafe.Pointer(cEvent))
			return nil, err
		}
		reportData := make([]C.uint8_t, len(eventData.Report))
		for i, b := range eventData.Report {
			reportData[i] = C.uint8_t(b)
		}
		var reportPtr *C.uint8_t
		if len(reportData) > 0 {
			reportPtr = &reportData[0]
		}
		C.set_event_status_report(cEvent, C.uint8_t(eventData.Type), C.uint8_t(len(eventData.Report)), reportPtr)

	case EventMfgReply:
		if err := checkLen("mfgrep", len(eventData.Data), C.OSDP_EVENT_MFGREP_MAX_DATALEN); err != nil {
			C.free(unsafe.Pointer(cEvent))
			return nil, err
		}
		mfgData := make([]C.uint8_t, len(eventData.Data))
		for i, b := range eventData.Data {
			mfgData[i] = C.uint8_t(b)
		}
		var mfgPtr *C.uint8_t
		if len(mfgData) > 0 {
			mfgPtr = &mfgData[0]
		}
		vendorCode := uint32(eventData.VendorCode[0]) | uint32(eventData.VendorCode[1])<<8 | uint32(eventData.VendorCode[2])<<16
		C.set_event_mfgrep(cEvent, C.uint32_t(vendorCode), C.uint8_t(len(eventData.Data)), mfgPtr)
	}

	return cEvent, nil
}

// decodeEvent converts a C osdp_event to a Go Event. It reads only cEvent, so it
// is shared by both the CP event callback and the PD event completion path.
func decodeEvent(cEvent *C.struct_osdp_event) *Event {
	var eventData EventData

	switch OsdpEventType(cEvent._type) {
	case EvtCardRead:
		var readerNo, direction C.int
		var format C.uint8_t
		var lengthBits C.uint16_t
		cardData := make([]C.uint8_t, C.OSDP_EVENT_CARDREAD_MAX_DATALEN)
		C.get_event_cardread(cEvent, &readerNo, &format, &direction, &lengthBits, &cardData[0])

		nbytes := (int(lengthBits) + 7) / 8
		eventData = EventCardRead{
			ReaderNo:  int32(readerNo),
			Format:    OsdpCardFormat(format),
			Direction: direction != 0,
			NrBits:    uint32(lengthBits),
			// SAFETY: cardData is sized to OSDP_EVENT_CARDREAD_MAX_DATALEN; nbytes = ceil(bits/8).
			Data: C.GoBytes(unsafe.Pointer(&cardData[0]), C.int(nbytes)),
		}
	case EvtKeypress:
		var readerNo, length C.int
		keypadData := make([]C.uint8_t, C.OSDP_EVENT_KEYPRESS_MAX_DATALEN)
		C.get_event_keypress(cEvent, &readerNo, &length, &keypadData[0])

		eventData = EventKeyPress{
			ReaderNo: int32(readerNo),
			Data:     C.GoBytes(unsafe.Pointer(&keypadData[0]), C.int(length)),
		}
	case EvtMfgReply:
		var vendorCode C.uint32_t
		var length C.uint8_t
		mfgData := make([]C.uint8_t, C.OSDP_EVENT_MFGREP_MAX_DATALEN)
		C.get_event_mfgrep(cEvent, &vendorCode, &length, &mfgData[0])

		// Convert vendor code to 3-byte array
		var vendorArray [3]uint8
		vendorArray[0] = uint8(vendorCode)
		vendorArray[1] = uint8(vendorCode >> 8)
		vendorArray[2] = uint8(vendorCode >> 16)

		eventData = EventMfgReply{
			VendorCode: vendorArray,
			Data:       C.GoBytes(unsafe.Pointer(&mfgData[0]), C.int(length)),
		}
	case EvtStatus:
		var reportType, nrEntries C.uint8_t
		statusReport := make([]C.uint8_t, C.OSDP_STATUS_REPORT_MAX_LEN)
		C.get_event_status(cEvent, &reportType, &nrEntries, &statusReport[0])

		eventData = EventStatus{
			Type:   OsdpStatusReportType(reportType),
			Report: C.GoBytes(unsafe.Pointer(&statusReport[0]), C.int(nrEntries)),
		}
	case EvtNotification:
		var ntype, arg0, arg1 C.int
		C.get_event_notification(cEvent, &ntype, &arg0, &arg1)
		eventData = EventNotification{
			Type: NotificationType(ntype),
			Arg0: int(arg0),
			Arg1: int(arg1),
		}
	default:
		// For unknown event types, return a basic status event
		eventData = EventStatus{
			Type: StatusReportInput,
		}
	}

	return &Event{
		Event: eventData,
	}
}
