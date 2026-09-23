/*
 * Copyright (c) 2026 Siddharth Chandrasekaran <sidcha@osdp.dev>
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package osdp_test

import (
	"regexp"
	"testing"

	"github.com/osdp-dev/libosdp-go"
)

// The vendored osdp_config.h must have every template variable resolved;
// otherwise GetVersion reports a literal "@VAR@".
func TestGetVersionIsResolvedSemver(t *testing.T) {
	v := osdp.GetVersion()
	if !regexp.MustCompile(`^\d+\.\d+\.\d+([-+.][0-9A-Za-z.+-]*)?$`).MatchString(v) {
		t.Fatalf("GetVersion() = %q, want a resolved semantic version", v)
	}
}
