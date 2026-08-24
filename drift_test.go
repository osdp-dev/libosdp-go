/*
 * Copyright (c) 2026 Siddharth Chandrasekaran <sidcha@osdp.dev>
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package osdp

import "testing"

// These counts are pinned to the vendored LibOSDP header. For the four
// SENTINEL-terminated enums (cmd, event, pd_cap, card_fmt), when upstream adds
// a member anywhere — inserted or appended — the sentinel's integer value
// changes and the matching assertion fails: bind the new member in the Go
// layer (or, if intentionally deferring, bump the count here WITH a comment
// explaining why it stays unbound). The notification and status-report enums
// have no sentinel, so their last member's ordinal is pinned instead; this
// only catches a member INSERTED before that last member, NOT one APPENDED
// after it (the conventional way wire-format enums grow) — see the caveats on
// expectedNotificationLast/expectedStatusReportLast below.
//
// The live C values are read in drift.go (a non-test file) and exposed as the
// drift* constants below, because the Go toolchain rejects `import "C"` in any
// *_test.go file.
const (
	// osdp_cmd_e is 1-based (OSDP_CMD_OUTPUT = 1). All 17 members are now bound
	// (the 6 biometric/PIV/auth commands landed in Phase 2), so OSDP_CMD_SENTINEL
	// == 18 and there is no remaining gap.
	expectedCmdSentinel = 18

	// osdp_event_type is 1-based (OSDP_EVENT_CARDREAD = 1). All 12 members are now
	// bound (the 7 mfg-status/biometric/PIV/auth reply events landed in Phase 2),
	// so OSDP_EVENT_SENTINEL == 13 and there is no remaining gap.
	expectedEventSentinel = 13

	// osdp_pd_cap_function_code_e is 0-based (OSDP_PD_CAP_UNUSED = 0) with 17 real
	// codes, so OSDP_PD_CAP_SENTINEL == 17.
	expectedPdCapSentinel = 17

	// osdp_event_cardread_format_e sentinel value (RAW_UNSPECIFIED, RAW_WIEGAND,
	// ASCII, SENTINEL).
	expectedCardFmtSentinel = 3

	// osdp_notification_type last member ordinal (no sentinel): MP_DONE.
	// Catches an insert before MP_DONE, NOT a member appended after it (C has
	// no sentinel to bump); the robust fix is an upstream sentinel.
	expectedNotificationLast = 5

	// osdp_status_report_type last member ordinal (no sentinel): READER.
	// Catches an insert before READER, NOT a member appended after it (C has
	// no sentinel to bump); the robust fix is an upstream sentinel.
	expectedStatusReportLast = 3

	// osdp_biometric_type_e sentinel value: 18 members (0x00..0x11) then SENTINEL.
	expectedBioTypeSentinel = 18

	// osdp_biometric_format_e sentinel value (NOT_SPECIFIED, RAW_PGM,
	// ANSI_INCITS_378, SENTINEL).
	expectedBioFmtSentinel = 3

	// osdp_biometric_status_e is non-sequential (Success=0, Timeout=1,
	// UnknownError=0xFF) with no sentinel: the max value UNKNOWN_ERROR is pinned.
	// This only detects a change to that terminal value, not a new member
	// inserted between Timeout and UnknownError.
	expectedBioStatusMax = 0xFF

	// osdp_mp_msg_type last member ordinal (no sentinel, 1-based): BIOREAD == 5.
	// Catches an insert before BIOREAD, NOT a member appended after it.
	expectedMPMsgLast = 5
)

func TestEnumDrift(t *testing.T) {
	cases := []struct {
		name   string
		got    int
		want   int
		remedy string
	}{
		{"osdp_cmd_e (OSDP_CMD_SENTINEL)", driftCmdSentinel, expectedCmdSentinel,
			"a command was added upstream; bind it in commands.go/constants.go or bump expectedCmdSentinel with justification"},
		{"osdp_event_type (OSDP_EVENT_SENTINEL)", driftEventSentinel, expectedEventSentinel,
			"an event was added upstream; bind it in events.go/constants.go or bump expectedEventSentinel with justification"},
		{"osdp_pd_cap (OSDP_PD_CAP_SENTINEL)", driftPdCapSentinel, expectedPdCapSentinel,
			"a capability code was added upstream; bind it in constants.go or bump expectedPdCapSentinel"},
		{"osdp_card_format (OSDP_CARD_FMT_SENTINEL)", driftCardFmtSentinel, expectedCardFmtSentinel,
			"a card format was added upstream; bind it in constants.go or bump expectedCardFmtSentinel"},
		{"osdp_notification_type (last=MP_DONE)", driftNotificationLast, expectedNotificationLast,
			"a notification type was inserted upstream; update constants.go and the notification union decode, then bump expectedNotificationLast"},
		{"osdp_status_report_type (last=READER)", driftStatusReportLast, expectedStatusReportLast,
			"a status report type was inserted upstream; update constants.go then bump expectedStatusReportLast"},
		{"osdp_biometric_type (SENTINEL)", driftBioTypeSentinel, expectedBioTypeSentinel,
			"a biometric type was added upstream; bind it in smartcard.go or bump expectedBioTypeSentinel"},
		{"osdp_biometric_format (SENTINEL)", driftBioFmtSentinel, expectedBioFmtSentinel,
			"a biometric format was added upstream; bind it in smartcard.go or bump expectedBioFmtSentinel"},
		{"osdp_biometric_status (UNKNOWN_ERROR)", driftBioStatusMax, expectedBioStatusMax,
			"the biometric status enum changed; review smartcard.go and bump expectedBioStatusMax"},
		{"osdp_mp_msg_type (last=BIOREAD)", driftMPMsgLast, expectedMPMsgLast,
			"an MP message type was appended upstream (sentinel-less enum); verify and bump expectedMPMsgLast"},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s: C reports %d, Go pinned to %d — %s", tc.name, tc.got, tc.want, tc.remedy)
		}
	}
}
