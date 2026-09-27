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

func TestEncodeRejectsOverLengthPayload(t *testing.T) {
	tooBig := bytes.Repeat([]byte{0xAB}, 1000) // exceeds every command/event max
	if _, err := osdp.RoundTripCommand(&osdp.Command{Command: osdp.CommandMfg{VendorCode: [3]uint8{1, 2, 3}, Data: tooBig}}); err == nil {
		t.Error("osdp.RoundTripCommand(CommandMfg oversize): want error, got nil")
	}
	if _, err := osdp.RoundTripEvent(&osdp.Event{Event: osdp.EventKeyPress{ReaderNo: 0, Data: tooBig}}); err == nil {
		t.Error("osdp.RoundTripEvent(EventKeyPress oversize): want error, got nil")
	}
}

func TestMfgCommandRoundTrip(t *testing.T) {
	in := osdp.CommandMfg{VendorCode: [3]uint8{0x11, 0x22, 0x33}, Data: []byte{1, 2, 3, 4}}
	got, ok := roundTripCommand(t, &osdp.Command{Command: in}).Command.(osdp.CommandMfg)
	if !ok {
		t.Fatalf("decoded type mismatch")
	}
	if got.VendorCode != in.VendorCode || !bytes.Equal(got.Data, in.Data) {
		t.Errorf("round-trip = %+v, want %+v", got, in)
	}
}

func TestMfgStatusEventsRoundTrip(t *testing.T) {
	data := []byte{9, 8, 7, 6, 5}
	statusEv := roundTripEvent(t, &osdp.Event{Event: osdp.EventMfgStatus{Data: data}})
	got, ok := statusEv.Event.(osdp.EventMfgStatus)
	if !ok {
		t.Fatalf("status decoded as %T, want EventMfgStatus", statusEv.Event)
	}
	if !bytes.Equal(got.Data, data) {
		t.Errorf("status data = %v, want %v", got.Data, data)
	}
	errEv := roundTripEvent(t, &osdp.Event{Event: osdp.EventMfgError{Data: data}})
	if _, ok := errEv.Event.(osdp.EventMfgError); !ok {
		t.Fatalf("error decoded as %T, want EventMfgError", errEv.Event)
	}
}

func TestTDSetRoundTrip(t *testing.T) {
	in := osdp.CommandTDSet{Year: 2026, Month: 7, Day: 23, Hour: 14, Minute: 5, Second: 9}
	got, ok := roundTripCommand(t, &osdp.Command{Command: in}).Command.(osdp.CommandTDSet)
	if !ok || got != in {
		t.Fatalf("round-trip = %+v, want %+v", got, in)
	}
}

func TestEncodeAcceptsEmptyPayload(t *testing.T) {
	got := roundTripCommand(t, &osdp.Command{Command: osdp.CommandMfg{VendorCode: [3]uint8{1, 2, 3}, Data: nil}})
	mfg, ok := got.Command.(osdp.CommandMfg)
	if !ok {
		t.Fatalf("decoded type = %T, want CommandMfg", got.Command)
	}
	if len(mfg.Data) != 0 {
		t.Errorf("empty mfg Data round-tripped to len %d", len(mfg.Data))
	}
}
