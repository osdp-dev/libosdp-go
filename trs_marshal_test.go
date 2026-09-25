/*
 * Copyright (c) 2026 Siddharth Chandrasekaran <sidcha@osdp.dev>
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package osdp

import (
	"reflect"
	"testing"
)

func TestTRSRepliesRoundTrip(t *testing.T) {
	for _, payload := range []TRSReplyData{
		TRSCardInfo{Reader: 0, Protocol: TRSCardProtocolContactless,
			CSN: []byte{0x04, 0xA2, 0x3B, 0x91}, ProtocolData: []byte{0x3B, 0x8F, 0x80, 0x01}},
		TRSCardPresent{Reader: 1, Status: TRSCardStatusContact},
		TRSCardData{Reader: 0, Status: 2, APDU: []byte{0x6A, 0x82}},
		TRSPINResult{Reader: 0, Status: 1, Tries: 3},
		TRSError{Code: 0x7F},
	} {
		want := EventTRS{Reply: payload.trsReply(), Payload: payload}
		got := roundTripEvent(t, &Event{Event: EventTRS{Payload: payload}}).Event
		if !reflect.DeepEqual(got, want) {
			t.Errorf("round trip of %T = %+v, want %+v", payload, got, want)
		}
	}
}

func TestTRSRejectsOversizeAPDU(t *testing.T) {
	big := make([]byte, TRSAPDUMaxLen+1)
	if _, err := commandToC(&Command{Command: CommandTRS{Command: TRSCmdSendAPDU, APDU: big}}); err == nil {
		t.Error("oversize C-APDU accepted")
	}
	if _, err := eventToC(&Event{Event: EventTRS{Payload: TRSCardData{APDU: big}}}); err == nil {
		t.Error("oversize R-APDU accepted")
	}
	if _, err := eventToC(&Event{Event: EventTRS{Payload: TRSCardInfo{CSN: make([]byte, TRSCSNMaxLen+1)}}}); err == nil {
		t.Error("oversize CSN accepted")
	}
}
