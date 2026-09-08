/*
 * Copyright (c) 2026 Siddharth Chandrasekaran <sidcha@osdp.dev>
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package osdp_test

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/osdp-dev/libosdp-go"
)

// A CP and a PD talking in one process over a pair of named pipes. The PD
// answers a manufacturer-specific command; the CP prints the reply.
func Example() {
	dir, _ := os.MkdirTemp("", "osdp-example-")
	defer os.RemoveAll(dir)
	toPD, toCP := filepath.Join(dir, "to-pd"), filepath.Join(dir, "to-cp")
	syscall.Mkfifo(toPD, 0o600)
	syscall.Mkfifo(toCP, 0o600)

	osdp.SetDiscardLogger()
	info := &osdp.PdInfo{
		Address: 1, BaudRate: 115200, Flags: osdp.FlagInstallMode,
		ID: osdp.PdId{VendorCode: 0x153, SerialNumber: 42},
	}

	pdChannel, _ := osdp.NewFIFOChannel(toPD, toCP)
	pd, err := osdp.NewPeripheralDevice(pdChannel, info, nil, osdp.LogInfo)
	if err != nil {
		log.Fatal(err)
	}
	defer pd.Stop()
	pd.SetCommandHandler(func(cmd *osdp.Command) (int, *osdp.Event) {
		if mfg, ok := cmd.Command.(osdp.CommandMfg); ok {
			reply := osdp.EventMfgReply{VendorCode: mfg.VendorCode, Data: []byte("pong")}
			return 0, &osdp.Event{Event: reply}
		}
		return 0, nil
	})

	cpChannel, _ := osdp.NewFIFOChannel(toCP, toPD)
	cp, err := osdp.NewControlPanel(cpChannel, []*osdp.PdInfo{info}, osdp.LogInfo)
	if err != nil {
		log.Fatal(err)
	}
	defer cp.Stop()
	replies := make(chan osdp.EventMfgReply, 1)
	cp.SetEventHandler(func(pdNumber int, ev *osdp.Event) int {
		if r, ok := ev.Event.(osdp.EventMfgReply); ok {
			replies <- r
		}
		return 0
	})

	if !cp.OnlineWait(0, 10*time.Second) {
		log.Fatal("PD did not come online")
	}
	ping := osdp.CommandMfg{VendorCode: [3]uint8{0x0A, 0x0B, 0x0C}, Data: []byte("ping")}
	if err := cp.SubmitCommand(0, &osdp.Command{Command: ping}); err != nil {
		log.Fatal(err)
	}
	r := <-replies
	fmt.Printf("PD replied %q\n", r.Data)
	// Output: PD replied "pong"
}

func ExampleNewControlPanel() {
	channel, err := osdp.NewSerialChannel("/dev/ttyUSB0", 115200)
	if err != nil {
		log.Fatal(err)
	}
	defer channel.Close()

	scbk, _ := os.ReadFile("/etc/osdp/pd-101.scbk") // provisioned earlier
	pds := []*osdp.PdInfo{{Address: 101, BaudRate: 115200, Flags: osdp.FlagEnforceSecure, SCBK: scbk}}
	cp, err := osdp.NewControlPanel(channel, pds, osdp.LogInfo)
	if err != nil {
		log.Fatal(err)
	}
	defer cp.Stop()

	cp.SetEventHandler(func(pdNumber int, ev *osdp.Event) int {
		switch e := ev.Event.(type) {
		case osdp.EventCardRead:
			fmt.Printf("PD %d: card %x (%d bits)\n", pdNumber, e.Data, e.NrBits)
		case osdp.EventKeyPress:
			fmt.Printf("PD %d: keys %q\n", pdNumber, e.Data)
		}
		return 0
	})

	if cp.SCWait(0, 10*time.Second) {
		cp.SubmitCommand(0, &osdp.Command{Command: osdp.CommandLed{
			Permanent: osdp.LedParams{ControlCode: osdp.LEDPermanentCCSet, OnCount: 1, OnColor: osdp.LEDColorGreen},
		}})
	}
}

func ExampleNewPeripheralDevice() {
	channel, err := osdp.NewSerialChannel("/dev/ttyS0", 115200)
	if err != nil {
		log.Fatal(err)
	}
	defer channel.Close()

	info := &osdp.PdInfo{
		Address: 101, BaudRate: 115200, Flags: osdp.FlagInstallMode,
		ID: osdp.PdId{Version: 1, Model: 1, VendorCode: 0x153, SerialNumber: 0x1001},
	}
	caps := []osdp.PdCapability{
		{Function: osdp.CapReaderLEDControl, Compliance: 1, NumItems: 1},
		{Function: osdp.CapCommunicationSecurity, Compliance: 1, NumItems: 1},
	}
	pd, err := osdp.NewPeripheralDevice(channel, info, caps, osdp.LogInfo)
	if err != nil {
		log.Fatal(err)
	}
	defer pd.Stop()

	pd.SetCommandHandler(func(cmd *osdp.Command) (int, *osdp.Event) {
		switch c := cmd.Command.(type) {
		case osdp.CommandLed:
			fmt.Printf("LED %d -> %v\n", c.LedNumber, c.Permanent.OnColor)
		case osdp.CommandKeyset:
			os.WriteFile("/var/lib/pd/scbk", c.Data, 0o600) // persist the provisioned key
		}
		return 0, nil // ACK
	})

	// Report a badge read to the CP.
	pd.SubmitEvent(&osdp.Event{Event: osdp.EventCardRead{
		Format: osdp.CardFormatWiegand, NrBits: 26, Data: []byte{0x01, 0x23, 0x45, 0x40},
	}})
}
