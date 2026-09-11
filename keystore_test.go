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

// A PD stores a key it was handed (from a KEYSET command) under a name it
// has never generated, and can load it back after a restart.
func TestKeyStoreSetKeyPersistsReceivedKey(t *testing.T) {
	dir := t.TempDir()
	ks, _ := osdp.NewKeyStore(dir)
	received := []byte{0: 0xAA, 15: 0x55}
	if err := ks.SetKey("scbk", received); err != nil {
		t.Fatalf("SetKey: %v", err)
	}
	if err := ks.CommitKey("scbk"); err != nil {
		t.Fatalf("CommitKey: %v", err)
	}

	reloaded, _ := osdp.NewKeyStore(dir)
	got, err := reloaded.LoadKey("scbk", 16)
	if err != nil || !bytes.Equal(got, received) {
		t.Fatalf("LoadKey = %x, %v; want %x", got, err, received)
	}
}

func TestKeyStoreSetKeyCopiesInput(t *testing.T) {
	ks, _ := osdp.NewKeyStore("")
	defer ks.Close()
	key := []byte{1, 2, 3, 4}
	ks.SetKey("k", key)
	key[0] = 9
	if got, _ := ks.GetKey("k"); got[0] != 1 {
		t.Fatal("SetKey kept a reference to the caller's slice")
	}
}
