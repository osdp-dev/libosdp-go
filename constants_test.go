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
