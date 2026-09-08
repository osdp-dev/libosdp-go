/*
 * Copyright (c) 2026 Siddharth Chandrasekaran <sidcha@osdp.dev>
 *
 * SPDX-License-Identifier: Apache-2.0
 */

// secure_channel provisions a factory-fresh PD with a Secure Channel Base Key
// (SCBK) and then reconnects with the secure channel enforced. CP and PD run in
// one process over a pair of named pipes, so it needs no hardware:
//
//	go run ./examples/secure_channel
//
// Phase 1: the PD is in install mode with no key. The CP, which holds the SCBK,
// opens a secure channel with the well-known install key (SCBK-D) and sends its
// SCBK in a KEYSET command; the PD persists it.
//
// Phase 2: the PD restarts with the persisted key and without install mode, and
// the CP sets FlagEnforceSecure so it refuses to talk in the clear.
package main

import (
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/osdp-dev/libosdp-go"
)

const pdAddress = 101

type link struct{ cpRx, cpTx string }

func newLink(dir, name string) link {
	l := link{filepath.Join(dir, name+"-cp-rx"), filepath.Join(dir, name+"-cp-tx")}
	for _, p := range []string{l.cpRx, l.cpTx} {
		if err := syscall.Mkfifo(p, 0o600); err != nil {
			log.Fatalf("mkfifo %s: %v", p, err)
		}
	}
	return l
}

func pdInfo(flags osdp.OsdpFlag, scbk []byte) *osdp.PdInfo {
	return &osdp.PdInfo{
		Address:  pdAddress,
		BaudRate: 115200,
		Flags:    flags,
		SCBK:     scbk,
		ID:       osdp.PdId{Version: 1, Model: 1, VendorCode: 0x153, SerialNumber: 0x1001},
	}
}

var pdCaps = []osdp.PdCapability{
	{Function: osdp.CapCommunicationSecurity, Compliance: 1, NumItems: 1},
}

func startPD(l link, info *osdp.PdInfo, handler osdp.CommandHandler) *osdp.PeripheralDevice {
	ch, err := osdp.NewFIFOChannel(l.cpTx, l.cpRx) // the PD reads what the CP writes
	if err != nil {
		log.Fatal(err)
	}
	pd, err := osdp.NewPeripheralDevice(ch, info, pdCaps, osdp.LogWarning)
	if err != nil {
		log.Fatal(err)
	}
	if handler != nil {
		pd.SetCommandHandler(handler)
	}
	return pd
}

func startCP(l link, info *osdp.PdInfo) *osdp.ControlPanel {
	ch, err := osdp.NewFIFOChannel(l.cpRx, l.cpTx)
	if err != nil {
		log.Fatal(err)
	}
	cp, err := osdp.NewControlPanel(ch, []*osdp.PdInfo{info}, osdp.LogWarning)
	if err != nil {
		log.Fatal(err)
	}
	return cp
}

func main() {
	dir, err := os.MkdirTemp("", "osdp-secure-channel-")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// The CP owns the key: load it, or create and persist one on first run.
	cpKeys, err := osdp.NewKeyStore(dir)
	if err != nil {
		log.Fatal(err)
	}
	scbk, err := cpKeys.LoadKey("pd-101", 16)
	if err != nil {
		if scbk, err = cpKeys.NewKey("pd-101", 16, false); err != nil {
			log.Fatal(err)
		}
		if err := cpKeys.CommitKey("pd-101"); err != nil {
			log.Fatal(err)
		}
	}
	fmt.Printf("CP: SCBK for PD %d stored at %s\n", pdAddress, cpKeys.KeyFile("pd-101"))

	// Phase 1: provision.
	pdKeyFile := filepath.Join(dir, "pd-flash.key")
	l := newLink(dir, "install")
	pd := startPD(l, pdInfo(osdp.FlagInstallMode, nil), func(cmd *osdp.Command) (int, *osdp.Event) {
		ks, ok := cmd.Command.(osdp.CommandKeyset)
		if !ok {
			return 0, nil
		}
		// Returning 0 accepts the key; libosdp switches to it once the
		// CP acknowledges. A real PD writes it to non-volatile storage.
		if err := os.WriteFile(pdKeyFile, []byte(hex.EncodeToString(ks.Data)), 0o600); err != nil {
			return -1, nil
		}
		fmt.Println("PD: received and stored a new SCBK")
		return 0, nil
	})
	cp := startCP(l, pdInfo(osdp.FlagInstallMode, scbk))
	if !cp.SCWait(0, 15*time.Second) {
		log.Fatal("provisioning failed: no secure channel")
	}
	fmt.Println("CP: secure channel up with the provisioned key")
	cp.Stop()
	pd.Stop()

	// Phase 2: restart the PD from its stored key and enforce the channel.
	stored, err := os.ReadFile(pdKeyFile)
	if err != nil {
		log.Fatal(err)
	}
	pdKey, err := hex.DecodeString(string(stored))
	if err != nil {
		log.Fatal(err)
	}
	l = newLink(dir, "secure")
	pd = startPD(l, pdInfo(0, pdKey), nil)
	defer pd.Stop()
	cp = startCP(l, pdInfo(osdp.FlagEnforceSecure, scbk))
	defer cp.Stop()
	if !cp.SCWait(0, 15*time.Second) {
		log.Fatal("enforced secure channel did not come up")
	}
	fmt.Println("CP: PD online over an enforced secure channel")
}
