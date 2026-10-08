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

func pattern(n int, seed byte) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = seed + byte(i*13)
	}
	return b
}

// Each biometric/PIV/auth command crosses the wire to the PD and the PD's
// reply event comes back intact. The 256-byte payloads force multipart
// transfers.
func TestSmartcardAndBiometricRequestsGetReplies(t *testing.T) {
	cases := []struct {
		name  string
		cmd   osdp.CommandData
		reply osdp.EventData
	}{
		{"bioread",
			osdp.CommandBioRead{Reader: 0, Type: osdp.BioTypeRightIndexFingerPrint, Format: osdp.BioFmtANSIINCITS378, Quality: 60},
			osdp.EventBioReadR{Reader: 0, Status: osdp.BioStatusSuccess, Type: osdp.BioTypeRightIndexFingerPrint, Quality: 80, Data: pattern(200, 1)}},
		{"biomatch",
			osdp.CommandBioMatch{Reader: 0, Type: osdp.BioTypeLeftThumbPrint, Format: osdp.BioFmtRawPGM, Quality: 50, Data: pattern(128, 2)},
			osdp.EventBioMatchR{Reader: 0, Status: osdp.BioStatusSuccess, Score: 97}},
		{"pivdata",
			osdp.CommandPIVData{OID: [3]byte{0x5F, 0xC1, 0x02}, Element: 1, Offset: 0},
			osdp.EventPIVData{Data: pattern(256, 3)}},
		{"genauth",
			osdp.CommandGenAuth{Algorithm: 0x07, Key: 0x9A, Data: pattern(256, 4)},
			osdp.EventGenAuthReply{Data: pattern(256, 5)}},
		{"crauth",
			osdp.CommandCRAuth{Algorithm: 0x07, Key: 0x9E, Data: pattern(64, 6)},
			osdp.EventCRAuthReply{Data: pattern(128, 7)}},
	}

	link := newFIFOLink(t)
	pd := startTestPD(t, link, testPdInfo(osdp.FlagInstallMode|osdp.FlagBioReadRMultipart), []osdp.PdCapability{
		{Function: osdp.CapBiometrics, Compliance: 1, NumItems: 1},
		{Function: osdp.CapSmartCardSupport, Compliance: 1, NumItems: 1},
		{Function: osdp.CapReaders, Compliance: 1, NumItems: 1},
	})
	received := make(chan osdp.CommandData, 1)
	replyFor := map[osdp.OsdpCommandType]osdp.EventData{}
	for _, c := range cases {
		replyFor[c.cmd.GetType()] = c.reply
	}
	pd.SetCommandHandler(func(cmd *osdp.Command) (int, *osdp.Event) {
		reply, ok := replyFor[cmd.Command.GetType()]
		if !ok {
			return 0, nil
		}
		received <- cmd.Command
		return 0, &osdp.Event{Event: reply}
	})

	cp := startTestCP(t, link, testPdInfo(osdp.FlagInstallMode))
	events := make(chan osdp.EventData, 4)
	cp.SetEventHandler(func(_ int, ev *osdp.Event) int {
		if ev.Event.GetType() != osdp.EvtNotification {
			events <- ev.Event
		}
		return 0
	})
	if !cp.OnlineWait(0, 10*time.Second) {
		t.Fatal("PD did not come online")
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := cp.SubmitCommand(0, &osdp.Command{Command: c.cmd}); err != nil {
				t.Fatalf("SubmitCommand: %v", err)
			}
			select {
			case got := <-received:
				if !reflect.DeepEqual(got, c.cmd) {
					t.Errorf("PD got %#v, want %#v", got, c.cmd)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("PD never received the command")
			}
			select {
			case got := <-events:
				if !reflect.DeepEqual(got, c.reply) {
					t.Errorf("CP got %#v, want %#v", got, c.reply)
				}
			case <-time.After(10 * time.Second):
				t.Fatal("CP never received the reply")
			}
		})
	}
}
