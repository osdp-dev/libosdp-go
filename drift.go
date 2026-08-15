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

// Live C enum member counts / terminal ordinals read from the vendored LibOSDP
// header, consumed only by TestEnumDrift to detect upstream enum growth. They
// live in this non-test file because the Go toolchain rejects `import "C"` in
// any *_test.go file, so the test cannot read these values itself.
const (
	driftCmdSentinel      = int(C.OSDP_CMD_SENTINEL)
	driftEventSentinel    = int(C.OSDP_EVENT_SENTINEL)
	driftPdCapSentinel    = int(C.OSDP_PD_CAP_SENTINEL)
	driftCardFmtSentinel  = int(C.OSDP_CARD_FMT_SENTINEL)
	driftNotificationLast = int(C.OSDP_NOTIFICATION_MP_DONE)
	driftStatusReportLast = int(C.OSDP_STATUS_REPORT_READER)
)
