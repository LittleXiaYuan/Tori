package memory

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// snapshot is the on-disk format for memory persistence.
type snapshot struct {
	Mid  map[string][]Item `json:"mid"`
	Long map[string][]Item `json:"long"`
}

// Persister auto-saves Mid and Long memory layers to a JSON file.
type Persister struct {
	path  string
	mid   *MidTerm
	long  *LongTerm
	mu    sync.Mutex
	dirty bool
	stop  chan struct{}
}

// NewPersister creates a persister that saves to the given file path.
// It loads existing data immediately and starts a background flush loop.
func NewPersister(path string, mid *MidTerm, long *LongTerm) *Persister {
	dir := filepath.Dir(path)
	if dir != "" {
		os.MkdirAll(dir, 0o755)
	}

	p := &Persister{
		path: path,
		mid:  mid,
		long: long,
		stop: make(chan struct{}),
	}
	p.load()
	go p.flushLoop()
	return p
}

// MarkDirty signals that data has changed and needs saving.
func (p *Persister) MarkDirty() {
	p.mu.Lock()
	p.dirty = true
	p.mu.Unlock()
}

// Stop flushes final state and stops the background loop.
func (p *Persister) Stop() {
	close(p.stop)
	p.flush()
}

func (p *Persister) load() {
	data, err := os.ReadFile(p.path)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Error("memory persist load", "err", err)
		}
		// Try backup if main file missing or unreadable
		data, err = os.ReadFile(p.path + ".bak")
		if err != nil {
			return
		}
		slog.Warn("memory: loaded from backup file", "path", p.path+".bak")
	}

	var snap snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		slog.Error("memory persist parse", "err", err)
		// Try backup on corrupt main file
		bakData, bakErr := os.ReadFile(p.path + ".bak")
		if bakErr != nil {
			return
		}
		if err := json.Unmarshal(bakData, &snap); err != nil {
			slog.Error("memory persist parse backup also failed", "err", err)
			return
		}
		slog.Warn("memory: recovered from backup after corrupt main file")
	}

	ctx := context.Background()
	count := 0

	for tid, items := range snap.Mid {
		for _, item := range items {
			_ = p.mid.Put(ctx, tid, item)
			count++
		}
	}
	for tid, items := range snap.Long {
		for _, item := range items {
			_ = p.long.Put(ctx, tid, item)
			count++
		}
	}

	slog.Info("memory loaded from disk", "path", p.path, "items", count)
}

func (p *Persister) flush() {
	p.mu.Lock()
	if !p.dirty {
		p.mu.Unlock()
		return
	}
	p.mu.Unlock()

	snap := snapshot{
		Mid:  p.exportMid(),
		Long: p.exportLong(),
	}

	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		slog.Error("memory persist marshal", "err", err)
		return
	}

	tmpPath := p.path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		slog.Error("memory persist write", "err", err)
		return
	}

	p.mu.Lock()
	p.dirty = false
	p.mu.Unlock()
	// Keep one backup of the previous version for crash safety
	if _, statErr := os.Stat(p.path); statErr == nil {
		bakPath := p.path + ".bak"
		_ = os.Remove(bakPath)
		_ = os.Rename(p.path, bakPath)
	}
	if err := os.Rename(tmpPath, p.path); err != nil {
		slog.Error("memory persist rename", "err", err)
		return
	}

	total := 0
	for _, items := range snap.Mid {
		total += len(items)
	}
	for _, items := range snap.Long {
		total += len(items)
	}
	slog.Debug("memory flushed to disk", "path", p.path, "items", total)
}

func (p *Persister) flushLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-p.stop:
			return
		case <-ticker.C:
			p.flush()
		}
	}
}

func (p *Persister) exportMid() map[string][]Item {
	p.mid.mu.RLock()
	defer p.mid.mu.RUnlock()
	out := make(map[string][]Item)
	for tid, m := range p.mid.items {
		for _, item := range m {
			out[tid] = append(out[tid], item)
		}
	}
	return out
}

func (p *Persister) exportLong() map[string][]Item {
	p.long.mu.RLock()
	defer p.long.mu.RUnlock()
	out := make(map[string][]Item)
	for tid, items := range p.long.items {
		out[tid] = append(out[tid], items...)
	}
	return out
}
