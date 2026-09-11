/*
 * Copyright (c) 2026 Siddharth Chandrasekaran <sidcha@osdp.dev>
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package osdp

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

// KeyStore generates and persists Secure Channel Base Keys (SCBK), mirroring the
// Python binding's KeyStore. Keys are held in memory and optionally committed to
// hex-encoded files in a directory. It is a convenience helper for applications
// and tests; libosdp itself never touches the store.
type KeyStore struct {
	dir  string
	temp bool
	keys map[string][]byte
}

// NewKeyStore creates a key store backed by dir. If dir is empty, a temporary
// directory is created and removed by Close.
func NewKeyStore(dir string) (*KeyStore, error) {
	ks := &KeyStore{keys: make(map[string][]byte)}
	if dir == "" {
		tmp, err := os.MkdirTemp("", "osdp-keystore-")
		if err != nil {
			return nil, err
		}
		ks.dir = tmp
		ks.temp = true
	} else {
		ks.dir = dir
	}
	return ks, nil
}

// Close removes the backing directory if it was created by NewKeyStore.
func (ks *KeyStore) Close() error {
	if ks.temp {
		return os.RemoveAll(ks.dir)
	}
	return nil
}

// KeyFile returns the on-disk path a named key is stored at.
func (ks *KeyStore) KeyFile(name string) string {
	return filepath.Join(ks.dir, "key_"+name+".bin")
}

// GenKey returns keyLen cryptographically-random bytes.
func GenKey(keyLen int) ([]byte, error) {
	key := make([]byte, keyLen)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	return key, nil
}

// GetKey returns a previously generated/loaded key by name.
func (ks *KeyStore) GetKey(name string) ([]byte, error) {
	key, ok := ks.keys[name]
	if !ok {
		return nil, fmt.Errorf("key %q not found", name)
	}
	return key, nil
}

// NewKey generates a fresh key of keyLen bytes for name. With force=false it
// errors if a key already exists for name.
func (ks *KeyStore) NewKey(name string, keyLen int, force bool) ([]byte, error) {
	if !force {
		if _, ok := ks.keys[name]; ok {
			return nil, fmt.Errorf("key %q already exists", name)
		}
	}
	key, err := GenKey(keyLen)
	if err != nil {
		return nil, err
	}
	ks.keys[name] = key
	return key, nil
}

// SetKey stores a copy of key under name, adding or replacing it. A PD uses
// it to keep the SCBK it receives in a CommandKeyset; call CommitKey to
// persist it.
func (ks *KeyStore) SetKey(name string, key []byte) error {
	if len(key) == 0 {
		return fmt.Errorf("key %q is empty", name)
	}
	ks.keys[name] = append([]byte(nil), key...)
	return nil
}

// CommitKey writes the named key to disk as hex.
func (ks *KeyStore) CommitKey(name string) error {
	key, ok := ks.keys[name]
	if !ok {
		return fmt.Errorf("key %q not found", name)
	}
	return os.WriteFile(ks.KeyFile(name), []byte(hex.EncodeToString(key)), 0o600)
}

// LoadKey reads a previously committed key from disk and caches it in memory.
func (ks *KeyStore) LoadKey(name string, keyLen int) ([]byte, error) {
	data, err := os.ReadFile(ks.KeyFile(name))
	if err != nil {
		return nil, err
	}
	key, err := hex.DecodeString(string(data))
	if err != nil {
		return nil, err
	}
	if len(key) != keyLen {
		return nil, fmt.Errorf("key %q has length %d, want %d", name, len(key), keyLen)
	}
	ks.keys[name] = key
	return key, nil
}
