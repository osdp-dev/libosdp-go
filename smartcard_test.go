/*
 * Copyright (c) 2026 Siddharth Chandrasekaran <sidcha@osdp.dev>
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package osdp_test

import (
	"bytes"
	"testing"

	"github.com/osdp-dev/libosdp-go"
)

func TestBiometricEnumValues(t *testing.T) {
	if osdp.BioTypeNotSpecified != 0x00 || osdp.BioTypeLeftHandGeometry != 0x11 {
		t.Errorf("bio type bounds: %d..%d", osdp.BioTypeNotSpecified, osdp.BioTypeLeftHandGeometry)
	}
	if osdp.BioFmtANSIINCITS378 != 0x02 {
		t.Errorf("BioFmtANSIINCITS378 = %d", osdp.BioFmtANSIINCITS378)
	}
	if osdp.BioStatusSuccess != 0x00 || osdp.BioStatusTimeout != 0x01 || osdp.BioStatusUnknownError != 0xFF {
		t.Errorf("bio status = %d/%d/%d", osdp.BioStatusSuccess, osdp.BioStatusTimeout, osdp.BioStatusUnknownError)
	}
}

func TestBioMatchRoundTrip(t *testing.T) {
	cmd := osdp.CommandBioMatch{Reader: 1, Type: osdp.BioTypeRightThumbPrint, Format: osdp.BioFmtANSIINCITS378, Quality: 200, Data: []byte{1, 2, 3, 4, 5}}
	got, ok := roundTripCommand(t, &osdp.Command{Command: cmd}).Command.(osdp.CommandBioMatch)
	if !ok || got.Reader != cmd.Reader || got.Type != cmd.Type || got.Format != cmd.Format || got.Quality != cmd.Quality || !bytes.Equal(got.Data, cmd.Data) {
		t.Fatalf("cmd round-trip = %+v, want %+v", got, cmd)
	}
	ev := osdp.EventBioMatchR{Reader: 1, Status: osdp.BioStatusSuccess, Score: 0xFF}
	gotEv, ok := roundTripEvent(t, &osdp.Event{Event: ev}).Event.(osdp.EventBioMatchR)
	if !ok || gotEv != ev {
		t.Fatalf("event round-trip = %+v, want %+v", gotEv, ev)
	}
}

func TestBioReadRoundTrip(t *testing.T) {
	cmd := osdp.CommandBioRead{Reader: 2, Type: osdp.BioTypeFullFaceImage, Format: osdp.BioFmtRawPGM, Quality: 128}
	got, ok := roundTripCommand(t, &osdp.Command{Command: cmd}).Command.(osdp.CommandBioRead)
	if !ok || got != cmd {
		t.Fatalf("cmd = %+v, want %+v", got, cmd)
	}
	big := make([]byte, 256) // == OSDP_EVENT_BIOREADR_MAX_TEMPLATE_LEN
	for i := range big {
		big[i] = byte(i)
	}
	ev := osdp.EventBioReadR{Reader: 2, Status: osdp.BioStatusSuccess, Type: osdp.BioTypeFullFaceImage, Quality: 250, Data: big}
	gotEv, ok := roundTripEvent(t, &osdp.Event{Event: ev}).Event.(osdp.EventBioReadR)
	if !ok || gotEv.Reader != ev.Reader || gotEv.Status != ev.Status || gotEv.Type != ev.Type || gotEv.Quality != ev.Quality || !bytes.Equal(gotEv.Data, big) {
		t.Fatalf("event data mismatch len=%d", len(gotEv.Data))
	}
	if osdp.FlagBioReadRMultipart != 0x00400000 {
		t.Errorf("FlagBioReadRMultipart = %#x", uint32(osdp.FlagBioReadRMultipart))
	}
}

func TestPIVDataRoundTrip(t *testing.T) {
	cmd := osdp.CommandPIVData{OID: [3]byte{0x5F, 0xC1, 0x02}, Element: 0x01, Offset: 0}
	got, ok := roundTripCommand(t, &osdp.Command{Command: cmd}).Command.(osdp.CommandPIVData)
	if !ok || got != cmd {
		t.Fatalf("cmd = %+v, want %+v", got, cmd)
	}
	big := make([]byte, 256)
	for i := range big {
		big[i] = byte(255 - i%256)
	}
	ev := osdp.EventPIVData{Data: big}
	gotEv, ok := roundTripEvent(t, &osdp.Event{Event: ev}).Event.(osdp.EventPIVData)
	if !ok || !bytes.Equal(gotEv.Data, big) {
		t.Fatalf("piv reply len=%d", len(gotEv.Data))
	}
}

func TestAuthRoundTrip(t *testing.T) {
	chal := bytes.Repeat([]byte{0x5A}, 200)
	gen, ok := roundTripCommand(t, &osdp.Command{Command: osdp.CommandGenAuth{Algorithm: 1, Key: 2, Data: chal}}).Command.(osdp.CommandGenAuth)
	if !ok || gen.Algorithm != 1 || gen.Key != 2 || !bytes.Equal(gen.Data, chal) {
		t.Fatalf("genauth cmd = %+v", gen)
	}
	if _, ok := roundTripCommand(t, &osdp.Command{Command: osdp.CommandCRAuth{Algorithm: 3, Key: 4, Data: chal}}).Command.(osdp.CommandCRAuth); !ok {
		t.Fatal("crauth cmd decoded to wrong type")
	}
	reply := bytes.Repeat([]byte{0xC3}, 256)
	if g, ok := roundTripEvent(t, &osdp.Event{Event: osdp.EventGenAuthReply{Data: reply}}).Event.(osdp.EventGenAuthReply); !ok || !bytes.Equal(g.Data, reply) {
		t.Fatal("genauthr reply wrong type/data")
	}
	if _, ok := roundTripEvent(t, &osdp.Event{Event: osdp.EventCRAuthReply{Data: reply}}).Event.(osdp.EventCRAuthReply); !ok {
		t.Fatal("crauthr reply decoded to wrong type")
	}
}
