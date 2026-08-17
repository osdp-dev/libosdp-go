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

// roundTripCommand marshals cmd to its C struct and decodes it back, freeing the
// C memory. It fails the test if marshalling errors.
func roundTripCommand(t *testing.T, cmd *Command) *Command {
	t.Helper()
	c, err := commandToC(cmd)
	if err != nil {
		t.Fatalf("commandToC(%T): %v", cmd.Command, err)
	}
	defer freeCmd(c)
	return decodeCommand(c)
}

// roundTripEvent marshals ev to its C struct and decodes it back, freeing the C
// memory. It fails the test if marshalling errors.
func roundTripEvent(t *testing.T, ev *Event) *Event {
	t.Helper()
	c, err := eventToC(ev)
	if err != nil {
		t.Fatalf("eventToC(%T): %v", ev.Event, err)
	}
	defer freeEvent(c)
	return decodeEvent(c)
}

func TestEncodeRejectsOverLengthPayload(t *testing.T) {
	tooBig := bytes.Repeat([]byte{0xAB}, 1000) // exceeds every command/event max
	if _, err := commandToC(&Command{Command: CommandMfg{VendorCode: [3]uint8{1, 2, 3}, Data: tooBig}}); err == nil {
		t.Error("commandToC(CommandMfg oversize): want error, got nil")
	}
	if _, err := eventToC(&Event{Event: EventKeyPress{ReaderNo: 0, Data: tooBig}}); err == nil {
		t.Error("eventToC(EventKeyPress oversize): want error, got nil")
	}
}

func TestMfgCommandRoundTrip(t *testing.T) {
	in := CommandMfg{VendorCode: [3]uint8{0x11, 0x22, 0x33}, Data: []byte{1, 2, 3, 4}}
	got, ok := roundTripCommand(t, &Command{Command: in}).Command.(CommandMfg)
	if !ok {
		t.Fatalf("decoded type mismatch")
	}
	if got.VendorCode != in.VendorCode || !bytes.Equal(got.Data, in.Data) {
		t.Errorf("round-trip = %+v, want %+v", got, in)
	}
}

func TestEncodeAcceptsEmptyPayload(t *testing.T) {
	got := roundTripCommand(t, &Command{Command: CommandMfg{VendorCode: [3]uint8{1, 2, 3}, Data: nil}})
	mfg, ok := got.Command.(CommandMfg)
	if !ok {
		t.Fatalf("decoded type = %T, want CommandMfg", got.Command)
	}
	if len(mfg.Data) != 0 {
		t.Errorf("empty mfg Data round-tripped to len %d", len(mfg.Data))
	}
}
