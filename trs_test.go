/*
 * Copyright (c) 2026 Siddharth Chandrasekaran <sidcha@osdp.dev>
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package osdp_test

import (
	"bytes"
	"reflect"
	"testing"
	"time"

	"github.com/osdp-dev/libosdp-go"
)

var trsCaps = []osdp.PdCapability{
	{Function: osdp.CapSmartCardSupport, Compliance: 1, NumItems: 0},
	{Function: osdp.CapReaders, Compliance: 1, NumItems: 1},
}

type trsPair struct {
	cp      *osdp.ControlPanel
	pdCmds  chan osdp.CommandTRS
	replies chan osdp.EventTRS
	status  chan osdp.TRSSessionStatus
}

// startTRSPair connects a CP to a PD that acts as a transparent reader:
// answer builds the PD's reply to each card command it receives.
func startTRSPair(t *testing.T, answer func(osdp.CommandTRS) osdp.TRSReplyData) *trsPair {
	t.Helper()
	p := &trsPair{
		pdCmds:  make(chan osdp.CommandTRS, 8),
		replies: make(chan osdp.EventTRS, 8),
		status:  make(chan osdp.TRSSessionStatus, 8),
	}
	link := newFIFOLink(t)
	pd := startTestPD(t, link, testPdInfo(osdp.FlagInstallMode), trsCaps)
	pd.SetCommandHandler(func(cmd *osdp.Command) (int, *osdp.Event) {
		c, ok := cmd.Command.(osdp.CommandTRS)
		if !ok {
			return 0, nil
		}
		p.pdCmds <- c
		reply := answer(c)
		if reply == nil {
			return 0, nil
		}
		return 0, &osdp.Event{Event: osdp.EventTRS{Payload: reply}}
	})
	p.cp = startTestCP(t, link, testPdInfo(osdp.FlagInstallMode|osdp.FlagEnableNotification))
	p.cp.SetEventHandler(func(_ int, ev *osdp.Event) int {
		switch e := ev.Event.(type) {
		case osdp.EventTRS:
			p.replies <- e
		case osdp.EventNotification:
			if s, ok := e.Payload.(osdp.NotificationTRSStatus); ok {
				p.status <- s.Status
			}
		}
		return 0
	})
	if !p.cp.OnlineWait(0, 10*time.Second) {
		t.Fatal("PD did not come online")
	}
	return p
}

func (p *trsPair) submit(t *testing.T, c osdp.CommandTRS) {
	t.Helper()
	if err := p.cp.SubmitCommand(0, &osdp.Command{Command: c}); err != nil {
		t.Fatalf("SubmitCommand(%v): %v", c.Command, err)
	}
}

func (p *trsPair) wantStatus(t *testing.T, want osdp.TRSSessionStatus) {
	t.Helper()
	select {
	case got := <-p.status:
		if got != want {
			t.Fatalf("TRS status %v, want %v", got, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("no TRS status %v", want)
	}
}

func TestTRSSessionCarriesAPDUToCardAndBack(t *testing.T) {
	selectAID := []byte{0x00, 0xA4, 0x04, 0x00, 0x07, 0xA0, 0x00, 0x00, 0x03, 0x08, 0x00, 0x00, 0x00}
	rapdu := []byte{0x61, 0x10, 0x90, 0x00}
	p := startTRSPair(t, func(c osdp.CommandTRS) osdp.TRSReplyData {
		return osdp.TRSCardData{Reader: 0, Status: 0, APDU: rapdu}
	})

	if err := p.cp.SubmitCommand(0, &osdp.Command{Command: osdp.CommandTRS{Command: osdp.TRSCmdSendAPDU, APDU: selectAID}}); err == nil {
		t.Fatal("card command accepted outside a session")
	}

	p.submit(t, osdp.CommandTRS{Command: osdp.TRSCmdStart})
	p.submit(t, osdp.CommandTRS{Command: osdp.TRSCmdSendAPDU, APDU: selectAID})
	p.submit(t, osdp.CommandTRS{Command: osdp.TRSCmdStop})
	p.wantStatus(t, osdp.TRSSessionOpened)

	select {
	case c := <-p.pdCmds:
		if c.Command != osdp.TRSCmdSendAPDU || !bytes.Equal(c.APDU, selectAID) {
			t.Errorf("PD got %+v, want SEND_APDU %x", c, selectAID)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("PD never got the APDU")
	}
	select {
	case r := <-p.replies:
		d, ok := r.Payload.(osdp.TRSCardData)
		if r.Reply != osdp.TRSReplyCardData || !ok || !bytes.Equal(d.APDU, rapdu) {
			t.Errorf("CP got %+v, want card data %x", r, rapdu)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("CP never got the R-APDU")
	}
	p.wantStatus(t, osdp.TRSSessionClosed)
}

func TestTRSPINEntryReachesReaderIntact(t *testing.T) {
	pin := osdp.TRSPINEntry{
		TimeoutInitial: 30, TimeoutDigit: 5,
		Format: osdp.TRSPINFormatBCD, RightJustify: true, BlockOffsetBits: 8, BlockSizeBytes: 8,
		LengthFieldSizeBits: 4, LengthFieldOffsetBits: 0,
		MinDigits: 4, MaxDigits: 8,
		CompleteOn:  osdp.TRSPINCompleteOnKey | osdp.TRSPINCompleteOnTimeout,
		NumMessages: 1, LanguageID: 0x0409, MsgIndex: 0, TEOPrologue: [3]uint8{0, 0, 0},
		APDU: []byte{0x00, 0x20, 0x00, 0x80, 0x08, 0x24, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
	}
	p := startTRSPair(t, func(c osdp.CommandTRS) osdp.TRSReplyData {
		return osdp.TRSPINResult{Reader: 0, Status: 0, Tries: 1}
	})
	p.submit(t, osdp.CommandTRS{Command: osdp.TRSCmdStart})
	p.submit(t, osdp.CommandTRS{Command: osdp.TRSCmdEnterPIN, PINEntry: pin})
	p.submit(t, osdp.CommandTRS{Command: osdp.TRSCmdStop})

	select {
	case c := <-p.pdCmds:
		if c.Command != osdp.TRSCmdEnterPIN || !reflect.DeepEqual(c.PINEntry, pin) {
			t.Errorf("PD got %+v, want %+v", c.PINEntry, pin)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("PD never got the PIN entry")
	}
	select {
	case r := <-p.replies:
		if res, ok := r.Payload.(osdp.TRSPINResult); r.Reply != osdp.TRSReplyPINComplete || !ok || res.Tries != 1 {
			t.Errorf("CP got %+v, want PIN complete with 1 try", r)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("CP never got the PIN result")
	}
}

func TestTRSScanControlAndAPDULimit(t *testing.T) {
	p := startTRSPair(t, func(osdp.CommandTRS) osdp.TRSReplyData { return nil })
	n, err := p.cp.TRSMaxAPDULen(0)
	if err != nil || n <= 0 || n > osdp.TRSAPDUMaxLen {
		t.Fatalf("TRSMaxAPDULen = %d, %v; want 1..%d", n, err, osdp.TRSAPDUMaxLen)
	}
	if err := p.cp.TRSScanEnable(0, osdp.TRSScanParams{Mode0Dwell: 200 * time.Millisecond}); err != nil {
		t.Fatalf("TRSScanEnable: %v", err)
	}
	if err := p.cp.TRSScanDisable(0); err != nil {
		t.Fatalf("TRSScanDisable: %v", err)
	}
	if _, err := p.cp.TRSMaxAPDULen(5); err == nil {
		t.Error("TRSMaxAPDULen accepted a bad PD number")
	}
}

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
