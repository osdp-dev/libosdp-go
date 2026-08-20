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

// These constructors build osdp_event notification fixtures for tests.
// Notifications are decode-only in production (libosdp synthesizes them), so
// there is no production encode path; the test helpers live here beside the
// other cgo code because *_test.go files cannot import "C".

func newCNotificationEvent() *C.struct_osdp_event {
	e := (*C.struct_osdp_event)(C.malloc(C.size_t(unsafe.Sizeof(C.struct_osdp_event{}))))
	C.memset(unsafe.Pointer(e), 0, C.size_t(unsafe.Sizeof(C.struct_osdp_event{})))
	e._type = C.OSDP_EVENT_NOTIFICATION
	return e
}

func buildNotificationCommand(command OsdpCommandType, success bool) *C.struct_osdp_event {
	e := newCNotificationEvent()
	s := 0
	if success {
		s = 1
	}
	C.set_notification_command(e, C.int(command), C.int(s))
	return e
}

func buildNotificationSCStatus(active, scbkD bool) *C.struct_osdp_event {
	e := newCNotificationEvent()
	a, d := 0, 0
	if active {
		a = 1
	}
	if scbkD {
		d = 1
	}
	C.set_notification_sc_status(e, C.int(a), C.int(d))
	return e
}

func buildNotificationPDStatus(online bool) *C.struct_osdp_event {
	e := newCNotificationEvent()
	o := 0
	if online {
		o = 1
	}
	C.set_notification_pd_status(e, C.int(o))
	return e
}

func buildNotificationMP(ntype NotificationType, mpType MPMsgType, objectID int, total, offset uint32, outcome FileTxOutcome) *C.struct_osdp_event {
	e := newCNotificationEvent()
	C.set_notification_mp(e, C.int(ntype), C.enum_osdp_mp_msg_type(mpType), C.int(objectID), C.uint32_t(total), C.uint32_t(offset), C.int(outcome))
	return e
}
