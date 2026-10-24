/*
 * Copyright (c) 2026 Siddharth Chandrasekaran <sidcha@osdp.dev>
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package osdp_test

import (
	"log"
	"strings"
	"sync"
	"testing"

	"github.com/osdp-dev/libosdp-go"
)

type logRecord struct {
	pd    int
	level osdp.LogLevel
	msg   string
}

func captureLogs(t *testing.T) func() []logRecord {
	t.Helper()
	var mu sync.Mutex
	var recs []logRecord
	osdp.SetLogHandler(func(pd int, level osdp.LogLevel, _ string, _ int, msg string) {
		mu.Lock()
		recs = append(recs, logRecord{pd, level, msg})
		mu.Unlock()
	})
	t.Cleanup(func() { osdp.SetLogHandler(nil) })
	return func() []logRecord {
		mu.Lock()
		defer mu.Unlock()
		return append([]logRecord(nil), recs...)
	}
}

func newPDLoggingAt(t *testing.T, level osdp.LogLevel) {
	t.Helper()
	link := newFIFOLink(t)
	ch, err := osdp.NewFIFOChannel(link.pdRx, link.pdTx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ch.Close() })
	info := testPdInfo(0) // no SCBK and no install mode: libosdp warns
	info.Address = 7
	pd, err := osdp.NewPeripheralDevice(ch, info, level)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pd.Stop)
}

const noSCBKWarning = "secure channel is disabled"

func TestLogHandlerSetBeforeSetupReceivesPDLogs(t *testing.T) {
	logs := captureLogs(t)
	newPDLoggingAt(t, osdp.LogInfo)

	for _, r := range logs() {
		if strings.Contains(r.msg, noSCBKWarning) {
			if r.pd != 7 || r.level != osdp.LogWarning {
				t.Errorf("got pd=%d level=%v, want pd=7 level=WARN", r.pd, r.level)
			}
			return
		}
	}
	t.Fatalf("handler never saw the PD's missing-SCBK warning; got %+v", logs())
}

func TestSetupLogLevelFiltersHandler(t *testing.T) {
	logs := captureLogs(t)
	newPDLoggingAt(t, osdp.LogError)

	for _, r := range logs() {
		if r.level > osdp.LogError {
			t.Fatalf("got %v log %q despite LogError setup level", r.level, r.msg)
		}
	}
}

func TestDefaultLoggerPrefixDoesNotNameOneContext(t *testing.T) {
	var buf strings.Builder
	var mu sync.Mutex
	prev := log.Writer()
	log.SetOutput(writerFunc(func(p []byte) (int, error) {
		mu.Lock()
		defer mu.Unlock()
		return buf.Write(p)
	}))
	t.Cleanup(func() { log.SetOutput(prev) })
	osdp.SetLogHandler(nil)
	t.Cleanup(func() { osdp.SetLogHandler(nil) })

	newPDLoggingAt(t, osdp.LogInfo) // installs the default logger

	mu.Lock()
	out := buf.String()
	mu.Unlock()
	if !strings.Contains(out, "[osdp] ") {
		t.Fatalf("default logger output lacks the [osdp] prefix:\n%s", out)
	}
}

type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }
