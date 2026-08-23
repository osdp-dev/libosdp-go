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

func TestBioReadRoundTrip(t *testing.T) {
	cmd := CommandBioRead{Reader: 2, Type: BioTypeFullFaceImage, Format: BioFmtRawPGM, Quality: 128}
	got, ok := roundTripCommand(t, &Command{Command: cmd}).Command.(CommandBioRead)
	if !ok || got != cmd {
		t.Fatalf("cmd = %+v, want %+v", got, cmd)
	}
	big := make([]byte, 256) // == OSDP_EVENT_BIOREADR_MAX_TEMPLATE_LEN
	for i := range big {
		big[i] = byte(i)
	}
	ev := EventBioReadR{Reader: 2, Status: BioStatusSuccess, Type: BioTypeFullFaceImage, Quality: 250, Data: big}
	gotEv, ok := roundTripEvent(t, &Event{Event: ev}).Event.(EventBioReadR)
	if !ok || gotEv.Reader != ev.Reader || gotEv.Status != ev.Status || gotEv.Type != ev.Type || gotEv.Quality != ev.Quality || !bytes.Equal(gotEv.Data, big) {
		t.Fatalf("event data mismatch len=%d", len(gotEv.Data))
	}
	if FlagBioReadRMultipart != 0x00400000 {
		t.Errorf("FlagBioReadRMultipart = %#x", uint32(FlagBioReadRMultipart))
	}
}
