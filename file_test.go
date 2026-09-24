/*
 * Copyright (c) 2026 Siddharth Chandrasekaran <sidcha@osdp.dev>
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package osdp_test

import (
	"bytes"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/osdp-dev/libosdp-go"
)

// memFile serves file ID id from data (CP side) or collects writes into data
// (PD side).
type memFile struct {
	mu     sync.Mutex
	id     int
	data   []byte
	opened bool
	closed chan struct{}
}

func newMemFile(id int, data []byte) *memFile {
	return &memFile{id: id, data: data, closed: make(chan struct{}, 1)}
}

func (f *memFile) Open(fileID int) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if fileID != f.id {
		return 0, fmt.Errorf("unknown file %d", fileID)
	}
	f.opened = true
	return len(f.data), nil
}

func (f *memFile) Read(buf []byte, size, offset int) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return copy(buf[:size], f.data[offset:]), nil
}

func (f *memFile) Write(buf []byte, size, offset int) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if end := offset + size; end > len(f.data) {
		f.data = append(f.data, make([]byte, end-len(f.data))...)
	}
	return copy(f.data[offset:], buf[:size]), nil
}

func (f *memFile) Close() error {
	select {
	case f.closed <- struct{}{}:
	default:
	}
	return nil
}

func (f *memFile) bytes() []byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]byte(nil), f.data...)
}

func TestFileTransferDeliversFileToPD(t *testing.T) {
	payload := make([]byte, 3000) // several fragments
	for i := range payload {
		payload[i] = byte(i * 7)
	}

	link := newFIFOLink(t)
	pd := startTestPD(t, link, testPdInfo(osdp.FlagInstallMode), nil)
	rx := newMemFile(3, nil)
	if err := pd.RegisterFileOperations(rx); err != nil {
		t.Fatalf("PD RegisterFileOperations: %v", err)
	}

	cp := startTestCP(t, link, testPdInfo(osdp.FlagInstallMode))
	tx := newMemFile(3, payload)
	if err := cp.RegisterFileOperations(0, tx); err != nil {
		t.Fatalf("CP RegisterFileOperations: %v", err)
	}
	if !cp.OnlineWait(0, 10*time.Second) {
		t.Fatal("PD did not come online")
	}
	if err := cp.SubmitCommand(0, &osdp.Command{Command: osdp.CommandFileTx{ID: 3}}); err != nil {
		t.Fatalf("SubmitCommand(FileTx): %v", err)
	}

	select {
	case <-rx.closed:
	case <-time.After(30 * time.Second):
		st, _ := cp.GetFileTransferStatus(0)
		t.Fatalf("transfer did not finish; CP status %+v, PD has %d bytes", st, len(rx.bytes()))
	}
	if got := rx.bytes(); !bytes.Equal(got, payload) {
		t.Fatalf("PD received %d bytes that differ from the %d sent", len(got), len(payload))
	}
}

func TestFileTransferStatusReportsProgress(t *testing.T) {
	payload := make([]byte, 20000)
	link := newFIFOLink(t)
	pd := startTestPD(t, link, testPdInfo(osdp.FlagInstallMode), nil)
	rx := newMemFile(1, nil)
	pd.RegisterFileOperations(rx)
	cp := startTestCP(t, link, testPdInfo(osdp.FlagInstallMode))
	cp.RegisterFileOperations(0, newMemFile(1, payload))
	if !cp.OnlineWait(0, 10*time.Second) {
		t.Fatal("PD did not come online")
	}
	cp.SubmitCommand(0, &osdp.Command{Command: osdp.CommandFileTx{ID: 1}})

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		cpSt, cpErr := cp.GetFileTransferStatus(0)
		pdSt, pdErr := pd.GetFileTransferStatus()
		if cpErr == nil && pdErr == nil && cpSt.Offset > 0 && pdSt.Offset > 0 {
			if cpSt.Size != len(payload) || pdSt.Size != len(payload) {
				t.Fatalf("sizes CP=%d PD=%d, want %d", cpSt.Size, pdSt.Size, len(payload))
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("never observed an in-progress transfer on both ends")
}

func TestCancelAbortsFileTransfer(t *testing.T) {
	link := newFIFOLink(t)
	pd := startTestPD(t, link, testPdInfo(osdp.FlagInstallMode), nil)
	pd.RegisterFileOperations(newMemFile(1, nil))
	cp := startTestCP(t, link, testPdInfo(osdp.FlagInstallMode|osdp.FlagEnableNotification))
	cp.RegisterFileOperations(0, newMemFile(1, make([]byte, 1<<20)))

	outcome := make(chan osdp.MPOutcome, 1)
	cp.SetEventHandler(func(_ int, ev *osdp.Event) int {
		if n, ok := ev.Event.(osdp.EventNotification); ok && n.Type == osdp.NotificationTypeMPDone {
			outcome <- n.Payload.(osdp.NotificationMP).Outcome
		}
		return 0
	})
	completed := make(chan osdp.CompletionStatus, 1)
	cp.SetCommandCompletionHandler(func(_ int, cmd *osdp.Command, st osdp.CompletionStatus) {
		if cmd.Command.GetType() == osdp.CmdFileTx {
			completed <- st
		}
	})
	if !cp.OnlineWait(0, 10*time.Second) {
		t.Fatal("PD did not come online")
	}

	if err := cp.Cancel(0, osdp.MPMsgFileTransfer); err == nil {
		t.Fatal("Cancel succeeded with no transfer running")
	}
	cp.SubmitCommand(0, &osdp.Command{Command: osdp.CommandFileTx{ID: 1}})
	for {
		if st, err := cp.GetFileTransferStatus(0); err == nil && st.Offset > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := cp.Cancel(0, osdp.MPMsgFileTransfer); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	select {
	case st := <-completed:
		if st != osdp.CompletionFailed {
			t.Errorf("FileTx completed %v, want CompletionFailed", st)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("cancelled transfer never completed")
	}
	select {
	case o := <-outcome:
		if o != osdp.MPOutcomeAborted {
			t.Errorf("MP_DONE outcome %v, want MPOutcomeAborted", o)
		}
	case <-time.After(time.Second):
		t.Fatal("no MP_DONE notification")
	}
}

// finalizingFile is a PD-side memFile that takes pending polls to "install"
// what it received, then reports outcome.
type finalizingFile struct {
	*memFile
	pending int
	outcome osdp.MPOutcome
	calls   int
}

func (f *finalizingFile) Finalize() (bool, osdp.MPOutcome, time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.calls <= f.pending {
		return false, 0, 20 * time.Millisecond
	}
	return true, f.outcome, 0
}

func transferWithFinalizer(t *testing.T, rx *finalizingFile) osdp.CompletionStatus {
	t.Helper()
	link := newFIFOLink(t)
	pd := startTestPD(t, link, testPdInfo(osdp.FlagInstallMode), nil)
	if err := pd.RegisterFileOperations(rx); err != nil {
		t.Fatalf("PD RegisterFileOperations: %v", err)
	}
	cp := startTestCP(t, link, testPdInfo(osdp.FlagInstallMode))
	cp.RegisterFileOperations(0, newMemFile(1, make([]byte, 500)))
	completed := make(chan osdp.CompletionStatus, 1)
	cp.SetCommandCompletionHandler(func(_ int, cmd *osdp.Command, st osdp.CompletionStatus) {
		if cmd.Command.GetType() == osdp.CmdFileTx {
			completed <- st
		}
	})
	if !cp.OnlineWait(0, 10*time.Second) {
		t.Fatal("PD did not come online")
	}
	cp.SubmitCommand(0, &osdp.Command{Command: osdp.CommandFileTx{ID: 1}})
	select {
	case st := <-completed:
		return st
	case <-time.After(15 * time.Second):
		t.Fatal("transfer never completed")
	}
	return 0
}

func TestFileFinalizerHoldsTransferOpenUntilDone(t *testing.T) {
	rx := &finalizingFile{memFile: newMemFile(1, nil), pending: 3, outcome: osdp.MPOutcomeOK}
	if st := transferWithFinalizer(t, rx); st != osdp.CompletionOK {
		t.Fatalf("transfer completed %v, want CompletionOK", st)
	}
	rx.mu.Lock()
	defer rx.mu.Unlock()
	if rx.calls != rx.pending+1 {
		t.Errorf("Finalize called %d times, want %d", rx.calls, rx.pending+1)
	}
}

func TestFileFinalizerCanRejectTheFile(t *testing.T) {
	rx := &finalizingFile{memFile: newMemFile(1, nil), outcome: osdp.MPOutcomeInvalid}
	if st := transferWithFinalizer(t, rx); st != osdp.CompletionFailed {
		t.Fatalf("transfer completed %v, want CompletionFailed", st)
	}
}
