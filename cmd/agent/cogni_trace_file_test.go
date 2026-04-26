package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"yunque-agent/pkg/cogni"
)

func TestFileTraceStore_PersistsAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "traces.jsonl")

	// Boot the first store, record a couple of traces, close it.
	s1, err := NewFileTraceStore(path, 16, 1024*1024)
	if err != nil {
		t.Fatalf("open 1: %v", err)
	}
	s1.Record(cogni.Trace{
		Timestamp:   time.Now(),
		MessageHash: "hash-1",
		Activations: []cogni.TraceActivation{{ID: "x", Activated: true}},
	})
	s1.Record(cogni.Trace{
		Timestamp:   time.Now(),
		MessageHash: "hash-2",
		Activations: []cogni.TraceActivation{{ID: "y", Activated: true}},
	})
	if err := s1.Close(); err != nil {
		t.Fatalf("close 1: %v", err)
	}

	// Reopen; previously-recorded traces should be restored.
	s2, err := NewFileTraceStore(path, 16, 1024*1024)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s2.Close()
	got := s2.Recent(0)
	if len(got) != 2 {
		t.Fatalf("expected 2 restored traces, got %d", len(got))
	}
	if got[0].MessageHash != "hash-2" {
		t.Fatalf("most recent first: got %q", got[0].MessageHash)
	}
	if st := s2.Stats(); st.TotalTurns != 2 {
		t.Fatalf("stats should count restored turns, got %d", st.TotalTurns)
	}
	if ids := s2.ByCogni("x", 0); len(ids) != 1 {
		t.Fatalf("ByCogni index must survive restore, got %+v", ids)
	}
}

func TestFileTraceStore_KeepsOnlyTailWithinCapacity(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "traces.jsonl")

	s1, _ := NewFileTraceStore(path, 8, 1024*1024)
	for i := 0; i < 20; i++ {
		s1.Record(cogni.Trace{
			MessageHash: string(rune('a' + i)),
			Activations: []cogni.TraceActivation{{ID: "x", Activated: true}},
		})
	}
	_ = s1.Close()

	s2, _ := NewFileTraceStore(path, 5, 1024*1024)
	defer s2.Close()
	got := s2.Recent(0)
	if len(got) != 5 {
		t.Fatalf("restore must honour new capacity (5), got %d", len(got))
	}
}

func TestFileTraceStore_RotatesWhenExceedingMaxBytes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "traces.jsonl")

	// Tiny cap forces rotation almost immediately.
	s, err := NewFileTraceStore(path, 16, 128)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()
	for i := 0; i < 10; i++ {
		s.Record(cogni.Trace{
			MessageHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			Activations: []cogni.TraceActivation{{ID: "x", Activated: true}},
		})
	}

	if _, err := os.Stat(path + ".1"); err != nil {
		t.Fatalf("backup file must exist after rotation: %v", err)
	}
}
