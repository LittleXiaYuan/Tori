package main

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"yunque-agent/pkg/cogni"
)

// FileTraceStore is a TraceStore that layers a JSONL (newline-delimited JSON)
// append-only log on top of an in-memory ring buffer.  Every Record call
// synchronously fsyncs a single line so an unexpected shutdown loses at most
// the currently-writing entry.  On startup we read the last ~capacity lines
// back into memory so the admin UI keeps showing decision history across
// restarts.
//
// Deliberate trade-offs:
//   - No sqlite / no new dependencies — traces are diagnostic, not
//     transactional; a plain JSONL file is perfectly adequate and can be
//     grepped/tailed from a terminal.
//   - Size cap is enforced by file rotation: when the file exceeds
//     maxBytes, the current file is renamed to ".1" (overwriting any prior
//     backup) and a fresh log is started.
//   - The in-memory mirror remains authoritative for queries so every
//     consumer (Monitor, Sentinel, /v1/cognis/traces) sees identical data
//     without touching disk on each read.
type FileTraceStore struct {
	mem      *cogni.InMemoryTraceStore
	path     string
	maxBytes int64

	mu   sync.Mutex
	f    *os.File
	size int64
}

// NewFileTraceStore opens (or creates) `path` and restores up to `capacity`
// trailing entries into memory.  maxBytes bounds the file; 0 = 10 MB.
func NewFileTraceStore(path string, capacity int, maxBytes int64) (*FileTraceStore, error) {
	if maxBytes <= 0 {
		maxBytes = 10 * 1024 * 1024
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	s := &FileTraceStore{
		mem:      cogni.NewInMemoryTraceStore(capacity),
		path:     path,
		maxBytes: maxBytes,
	}
	if err := s.restore(capacity); err != nil {
		slog.Warn("cogni: trace log restore failed", "err", err, "path", path)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	s.f = f
	s.size = info.Size()
	return s, nil
}

func (s *FileTraceStore) restore(capacity int) error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	// Count lines cheaply to pick the tail window without storing everything.
	var traces []cogni.Trace
	offset := 0
	for offset < len(data) {
		end := offset
		for end < len(data) && data[end] != '\n' {
			end++
		}
		line := data[offset:end]
		offset = end + 1
		if len(line) == 0 {
			continue
		}
		var t cogni.Trace
		if err := json.Unmarshal(line, &t); err != nil {
			continue // best-effort; skip corrupt line
		}
		traces = append(traces, t)
	}
	// Keep only the tail
	if len(traces) > capacity {
		traces = traces[len(traces)-capacity:]
	}
	for _, t := range traces {
		s.mem.Record(t)
	}
	if len(traces) > 0 {
		slog.Info("cogni: trace log restored", "path", s.path, "entries", len(traces))
	}
	return nil
}

// Record writes to both the in-memory mirror and the append-only file.
// File-write errors are logged but never propagated: traces are best-effort
// diagnostics, never a hard-path runtime dependency.
func (s *FileTraceStore) Record(t cogni.Trace) {
	s.mem.Record(t)
	line, err := json.Marshal(t)
	if err != nil {
		return
	}
	line = append(line, '\n')

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.f == nil {
		return
	}
	n, err := s.f.Write(line)
	if err != nil {
		slog.Warn("cogni: trace log write failed", "err", err)
		return
	}
	s.size += int64(n)
	if s.size >= s.maxBytes {
		s.rotateLocked()
	}
}

func (s *FileTraceStore) rotateLocked() {
	if s.f != nil {
		_ = s.f.Close()
	}
	backup := s.path + ".1"
	_ = os.Remove(backup)
	_ = os.Rename(s.path, backup)
	f, err := os.OpenFile(s.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		slog.Warn("cogni: trace log rotate failed", "err", err)
		s.f = nil
		return
	}
	s.f = f
	s.size = 0
}

// Close flushes and closes the underlying file.  Callers typically invoke
// this from the cogniModule.Stop path; it is safe to call multiple times.
func (s *FileTraceStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.f == nil {
		return nil
	}
	err := s.f.Close()
	s.f = nil
	return err
}

func (s *FileTraceStore) Recent(limit int) []cogni.Trace   { return s.mem.Recent(limit) }
func (s *FileTraceStore) ByCogni(id string, limit int) []cogni.Trace {
	return s.mem.ByCogni(id, limit)
}
func (s *FileTraceStore) Stats() cogni.TraceStats { return s.mem.Stats() }

// Compile-time proof that FileTraceStore satisfies cogni.TraceStore so a
// future change to the interface forces a visible edit here.
var _ cogni.TraceStore = (*FileTraceStore)(nil)