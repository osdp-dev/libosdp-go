/*
 * Copyright (c) 2026 Siddharth Chandrasekaran <sidcha@osdp.dev>
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package osdp_test

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Header identifiers that are deliberately not bound, with the reason. Enum
// *_SENTINEL members are skipped wholesale.
var unboundByDesign = map[string]string{
	"OSDP_EXPORT":            "symbol visibility macro",
	"OSDP_DEPRECATED_EXPORT": "symbol visibility macro",
	"OSDP_LOG_MAX_LEVEL":     "enum bound marker, not a level",
	"OSDP_PD_CAP_UNUSED":     "placeholder function code 0",
	"OSDP_PD_ONLINE_TOUT_MS": "internal timing constant, documentation only",
	"osdp_logger_init":       "would replace the callback that routes logs to SetLogHandler",
}

// Transparent reader support (TRS) arrived with LibOSDP 4.0 and is bound by
// its own change; until then its identifiers are exempt.
func isTRS(name string) bool {
	switch name {
	case "OSDP_CMD_XWR", "OSDP_EVENT_TRS", "OSDP_NOTIFICATION_TRS_STATUS":
		return true
	}
	return strings.HasPrefix(name, "OSDP_TRS_") || strings.HasPrefix(name, "osdp_cp_trs_")
}

var cComment = regexp.MustCompile(`(?s)/\*.*?\*/|//[^\n]*`)

func readStripped(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return cComment.ReplaceAllString(string(b), "")
}

// TestHeaderIsFullyBound fails when the vendored osdp.h gains a constant,
// enum member or function that the binding does not reference, so an
// upstream bump cannot silently leave API unbound.
func TestHeaderIsFullyBound(t *testing.T) {
	header := readStripped(t, filepath.Join("internal", "clib", "include", "osdp.h"))
	declared := map[string]bool{}
	for _, m := range regexp.MustCompile(`\bOSDP_[A-Z0-9_]+\b`).FindAllString(header, -1) {
		declared[m] = true
	}
	for _, m := range regexp.MustCompile(`\b(osdp_[a-z0-9_]+)\s*\(`).FindAllStringSubmatch(header, -1) {
		declared[m[1]] = true
	}

	used := map[string]bool{}
	goRef := regexp.MustCompile(`\bC\.((?:OSDP|osdp)_[A-Za-z0-9_]+)`)
	files, _ := filepath.Glob("*.go")
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range goRef.FindAllStringSubmatch(string(b), -1) {
			used[m[1]] = true
		}
	}
	helpers := readStripped(t, "cgo_helpers.h")
	for _, m := range regexp.MustCompile(`\b(?:OSDP|osdp)_[A-Za-z0-9_]+\b`).FindAllString(helpers, -1) {
		used[m] = true
	}

	var missing []string
	for name := range declared {
		if used[name] || strings.HasSuffix(name, "_SENTINEL") || isTRS(name) {
			continue
		}
		if _, ok := unboundByDesign[name]; ok {
			continue
		}
		missing = append(missing, name)
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("osdp.h declares %d identifiers with no binding; bind them or "+
			"add them to unboundByDesign with a reason:\n  %s",
			len(missing), strings.Join(missing, "\n  "))
	}
}
