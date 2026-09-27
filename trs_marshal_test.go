/*
 * Copyright (c) 2026 Siddharth Chandrasekaran <sidcha@osdp.dev>
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package osdp_test

import (
	"reflect"
	"testing"

	"github.com/osdp-dev/libosdp-go"
)

func TestTRSRepliesRoundTrip(t *testing.T) {
	for _, want := range []osdp.EventTRS{
		{Reply: osdp.TRSReplyCardInfo, Payload: osdp.TRSCardInfo{Reader: 0, Protocol: osdp.TRSCardProtocolContactless,
			CSN: []byte{0x04, 0xA2, 0x3B, 0x91}, ProtocolData: []byte{0x3B, 0x8F, 0x80, 0x01}}},
		{Reply: osdp.TRSReplyCardPresent, Payload: osdp.TRSCardPresent{Reader: 1, Status: osdp.TRSCardStatusContact}},
		{Reply: osdp.TRSReplyCardData, Payload: osdp.TRSCardData{Reader: 0, Status: 2, APDU: []byte{0x6A, 0x82}}},
		{Reply: osdp.TRSReplyPINComplete, Payload: osdp.TRSPINResult{Reader: 0, Status: 1, Tries: 3}},
		{Reply: osdp.TRSReplyError, Payload: osdp.TRSError{Code: 0x7F}},
	} {
		payload := want.Payload
		got := roundTripEvent(t, &osdp.Event{Event: osdp.EventTRS{Payload: payload}}).Event
		if !reflect.DeepEqual(got, want) {
			t.Errorf("round trip of %T = %+v, want %+v", payload, got, want)
		}
	}
}

func TestTRSRejectsOversizeAPDU(t *testing.T) {
	big := make([]byte, osdp.TRSAPDUMaxLen+1)
	if _, err := osdp.RoundTripCommand(&osdp.Command{Command: osdp.CommandTRS{Command: osdp.TRSCmdSendAPDU, APDU: big}}); err == nil {
		t.Error("oversize C-APDU accepted")
	}
	if _, err := osdp.RoundTripEvent(&osdp.Event{Event: osdp.EventTRS{Payload: osdp.TRSCardData{APDU: big}}}); err == nil {
		t.Error("oversize R-APDU accepted")
	}
	if _, err := osdp.RoundTripEvent(&osdp.Event{Event: osdp.EventTRS{Payload: osdp.TRSCardInfo{CSN: make([]byte, osdp.TRSCSNMaxLen+1)}}}); err == nil {
		t.Error("oversize CSN accepted")
	}
}
