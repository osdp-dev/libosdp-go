/*
 * Copyright (c) 2026 Siddharth Chandrasekaran <sidcha@osdp.dev>
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package osdp

import (
	"bytes"
	"testing"
)

func TestBiometricEnumValues(t *testing.T) {
	if BioTypeNotSpecified != 0x00 || BioTypeLeftHandGeometry != 0x11 {
		t.Errorf("bio type bounds: %d..%d", BioTypeNotSpecified, BioTypeLeftHandGeometry)
	}
	if BioFmtANSIINCITS378 != 0x02 {
		t.Errorf("BioFmtANSIINCITS378 = %d", BioFmtANSIINCITS378)
	}
	if BioStatusSuccess != 0x00 || BioStatusTimeout != 0x01 || BioStatusUnknownError != 0xFF {
		t.Errorf("bio status = %d/%d/%d", BioStatusSuccess, BioStatusTimeout, BioStatusUnknownError)
	}
}

func TestBioMatchRoundTrip(t *testing.T) {
	cmd := CommandBioMatch{Reader: 1, Type: BioTypeRightThumbPrint, Format: BioFmtANSIINCITS378, Quality: 200, Data: []byte{1, 2, 3, 4, 5}}
	got, ok := roundTripCommand(t, &Command{Command: cmd}).Command.(CommandBioMatch)
	if !ok || got.Reader != cmd.Reader || got.Type != cmd.Type || got.Format != cmd.Format || got.Quality != cmd.Quality || !bytes.Equal(got.Data, cmd.Data) {
		t.Fatalf("cmd round-trip = %+v, want %+v", got, cmd)
	}
	ev := EventBioMatchR{Reader: 1, Status: BioStatusSuccess, Score: 0xFF}
	gotEv, ok := roundTripEvent(t, &Event{Event: ev}).Event.(EventBioMatchR)
	if !ok || gotEv != ev {
		t.Fatalf("event round-trip = %+v, want %+v", gotEv, ev)
	}
}
