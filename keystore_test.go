/*
 * Copyright (c) 2026 Siddharth Chandrasekaran <sidcha@osdp.dev>
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package osdp_test

import (
	"bytes"
	"path/filepath"
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

// TestKeyStore exercises the pure-Go SCBK key store.
func TestKeyStore(t *testing.T) {
	ks, err := osdp.NewKeyStore("")
	if err != nil {
		t.Fatalf("NewKeyStore: %v", err)
	}
	defer ks.Close()

	key, err := ks.NewKey("scbk", 16, true)
	if err != nil {
		t.Fatalf("NewKey: %v", err)
	}
	if len(key) != 16 {
		t.Fatalf("key length = %d, want 16", len(key))
	}

	got, err := ks.GetKey("scbk")
	if err != nil || !bytes.Equal(got, key) {
		t.Fatalf("GetKey = %x (err %v), want %x", got, err, key)
	}

	// force=false must refuse to overwrite an existing key.
	if _, err := ks.NewKey("scbk", 16, false); err == nil {
		t.Error("NewKey(force=false) on existing key should error")
	}

	if err := ks.CommitKey("scbk"); err != nil {
		t.Fatalf("CommitKey: %v", err)
	}

	// A second store pointed at the same directory should load the same bytes.
	ks2, err := osdp.NewKeyStore(filepath.Dir(ks.KeyFile("scbk")))
	if err != nil {
		t.Fatalf("NewKeyStore(dir): %v", err)
	}
	loaded, err := ks2.LoadKey("scbk", 16)
	if err != nil || !bytes.Equal(loaded, key) {
		t.Fatalf("LoadKey = %x (err %v), want %x", loaded, err, key)
	}
	if _, err := ks2.LoadKey("scbk", 32); err == nil {
		t.Error("LoadKey with wrong length should error")
	}
}
