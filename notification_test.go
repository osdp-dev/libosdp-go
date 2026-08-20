/*
 * Copyright (c) 2026 Siddharth Chandrasekaran <sidcha@osdp.dev>
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package osdp

import "testing"

func TestNotificationCommandDecode(t *testing.T) {
	e := buildNotificationCommand(CmdOutput, true)
	defer freeEvent(e)
	n := decodeEvent(e).Event.(EventNotification)
	if n.Type != NotificationTypeCommand {
		t.Fatalf("Type = %v, want NotificationTypeCommand", n.Type)
	}
	p, ok := n.Payload.(NotificationCommand)
	if !ok {
		t.Fatalf("payload = %T, want NotificationCommand", n.Payload)
	}
	if p.Command != CmdOutput || !p.Success {
		t.Errorf("payload = %+v", p)
	}
}

func TestNotificationSCStatusDecode(t *testing.T) {
	e := buildNotificationSCStatus(true, false)
	defer freeEvent(e)
	n := decodeEvent(e).Event.(EventNotification)
	p, ok := n.Payload.(NotificationSCStatus)
	if n.Type != NotificationTypeSCStatus || !ok || !p.Active || p.SCBKD {
		t.Errorf("Type=%v payload=%+v ok=%v", n.Type, n.Payload, ok)
	}
}

func TestNotificationPDStatusDecode(t *testing.T) {
	e := buildNotificationPDStatus(true)
	defer freeEvent(e)
	n := decodeEvent(e).Event.(EventNotification)
	p, ok := n.Payload.(NotificationPDStatus)
	if n.Type != NotificationTypePDStatus || !ok || !p.Online {
		t.Errorf("Type=%v payload=%+v ok=%v", n.Type, n.Payload, ok)
	}
}

func TestNotificationMPDecode(t *testing.T) {
	e := buildNotificationMP(NotificationTypeMPDone, MPMsgBioRead, 7, 256, 128, FileTxOutcomeOK)
	defer freeEvent(e)
	n := decodeEvent(e).Event.(EventNotification)
	if n.Type != NotificationTypeMPDone {
		t.Fatalf("Type = %v, want NotificationTypeMPDone", n.Type)
	}
	mp, ok := n.Payload.(NotificationMP)
	if !ok {
		t.Fatalf("payload = %T, want NotificationMP", n.Payload)
	}
	if mp.MPType != MPMsgBioRead || mp.Total != 256 || mp.Offset != 128 || mp.ObjectID != 7 || mp.Outcome != FileTxOutcomeOK {
		t.Errorf("mp = %+v", mp)
	}
}
