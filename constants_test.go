/*
 * Copyright (c) 2026 Siddharth Chandrasekaran <sidcha@osdp.dev>
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package osdp_test

import (
	"testing"
	"time"

	"github.com/osdp-dev/libosdp-go"
)

// Control codes are sent on the wire as-is, so pin them to the values in the
// OSDP specification and check they drop straight into the uint8 fields.
func TestControlCodesMatchOSDPSpec(t *testing.T) {
	cases := []struct {
		name string
		got  uint8
		want uint8
	}{
		{"OutputCCNop", osdp.OutputCCNop, 0},
		{"OutputCCPermanentOff", osdp.OutputCCPermanentOff, 1},
		{"OutputCCPermanentOn", osdp.OutputCCPermanentOn, 2},
		{"OutputCCPermanentOffAllowTimed", osdp.OutputCCPermanentOffAllowTimed, 3},
		{"OutputCCPermanentOnAllowTimed", osdp.OutputCCPermanentOnAllowTimed, 4},
		{"OutputCCTemporaryOn", osdp.OutputCCTemporaryOn, 5},
		{"OutputCCTemporaryOff", osdp.OutputCCTemporaryOff, 6},
		{"LEDTemporaryCCNop", osdp.LEDTemporaryCCNop, 0},
		{"LEDTemporaryCCCancel", osdp.LEDTemporaryCCCancel, 1},
		{"LEDTemporaryCCSet", osdp.LEDTemporaryCCSet, 2},
		{"LEDPermanentCCNop", osdp.LEDPermanentCCNop, 0},
		{"LEDPermanentCCSet", osdp.LEDPermanentCCSet, 1},
		{"BuzzerCCNoTone", osdp.BuzzerCCNoTone, 0},
		{"BuzzerCCOff", osdp.BuzzerCCOff, 1},
		{"BuzzerCCDefaultTone", osdp.BuzzerCCDefaultTone, 2},
		{"TextCCPermanentNoWrap", osdp.TextCCPermanentNoWrap, 1},
		{"TextCCPermanentWrap", osdp.TextCCPermanentWrap, 2},
		{"TextCCTemporaryNoWrap", osdp.TextCCTemporaryNoWrap, 3},
		{"TextCCTemporaryWrap", osdp.TextCCTemporaryWrap, 4},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
		}
	}

	_ = osdp.CommandOutput{ControlCode: osdp.OutputCCTemporaryOn}
	_ = osdp.CommandBuzzer{ControlCode: osdp.BuzzerCCDefaultTone}
	_ = osdp.CommandText{ControlCode: osdp.TextCCPermanentWrap}
	_ = osdp.LedParams{ControlCode: osdp.LEDPermanentCCSet}
}

func TestCPSeesAdvertisedPINEntryAndVersionCapabilities(t *testing.T) {
	link := newFIFOLink(t)
	startTestPD(t, link, testPdInfo(osdp.FlagInstallMode), []osdp.PdCapability{
		{Function: osdp.CapSecurePINEntry, Compliance: 1, NumItems: 0},
		{Function: osdp.CapOSDPVersion, Compliance: 2, NumItems: 0},
	})
	cp := startTestCP(t, link, testPdInfo(osdp.FlagInstallMode))
	if !cp.OnlineWait(0, 10*time.Second) {
		t.Fatal("PD did not come online")
	}

	for fn, want := range map[osdp.PdCapabilityFunction]int{
		osdp.CapSecurePINEntry: 1,
		osdp.CapOSDPVersion:    2,
	} {
		got, err := cp.GetPDCapability(0, fn)
		if err != nil {
			t.Fatalf("GetPDCapability(%v): %v", fn, err)
		}
		if got.Compliance != want {
			t.Errorf("capability %v compliance = %d, want %d", fn, got.Compliance, want)
		}
	}
}
