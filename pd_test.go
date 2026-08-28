/*
 * Copyright (c) 2026 Siddharth Chandrasekaran <sidcha@osdp.dev>
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package osdp_test

import (
	"testing"
	"time"

	"github.com/osdp-dev/libosdp-go"
)

func TestPDNotificationsDoNotSurfaceAsStatusCommands(t *testing.T) {
	link := newFIFOLink(t)
	pd := startTestPD(t, link, testPdInfo(osdp.FlagInstallMode|osdp.FlagEnableNotification), nil)

	cmds := make(chan *osdp.Command, 32)
	pd.SetCommandHandler(func(cmd *osdp.Command) (int, *osdp.Event) {
		cmds <- cmd
		return 0, nil
	})

	cp := startTestCP(t, link, testPdInfo(osdp.FlagInstallMode))
	if !cp.OnlineWait(0, 10*time.Second) {
		t.Fatal("PD did not come online")
	}

	deadline := time.After(2 * time.Second)
	for {
		select {
		case cmd := <-cmds:
			if cmd.Command.GetType() == osdp.CmdStatus {
				t.Fatalf("PD saw %#v; the CP never sent a status command", cmd.Command)
			}
		case <-deadline:
			return
		}
	}
}

func TestPDReceivesLinkUpNotification(t *testing.T) {
	link := newFIFOLink(t)
	pd := startTestPD(t, link, testPdInfo(osdp.FlagInstallMode|osdp.FlagEnableNotification), nil)

	linkUp := make(chan struct{}, 1)
	pd.SetCommandHandler(func(cmd *osdp.Command) (int, *osdp.Event) {
		n, ok := cmd.Command.(osdp.CommandNotification)
		if !ok || n.Type != osdp.NotificationTypePDStatus {
			return 0, nil
		}
		if p, ok := n.Payload.(osdp.NotificationPDStatus); ok && p.Online {
			select {
			case linkUp <- struct{}{}:
			default:
			}
		}
		return 0, nil
	})

	startTestCP(t, link, testPdInfo(osdp.FlagInstallMode))
	select {
	case <-linkUp:
	case <-time.After(10 * time.Second):
		t.Fatal("PD never reported the CP link coming up")
	}
}

func TestSubmitCommandRejectsNotification(t *testing.T) {
	link := newFIFOLink(t)
	cp := startTestCP(t, link, testPdInfo(osdp.FlagInstallMode))
	err := cp.SubmitCommand(0, &osdp.Command{Command: osdp.CommandNotification{
		Type: osdp.NotificationTypePDStatus, Payload: osdp.NotificationPDStatus{Online: true},
	}})
	if err == nil {
		t.Fatal("SubmitCommand accepted a synthesized notification")
	}
}
