package emotion

import (
	"context"
	"log/slog"
	"time"
)

// kvStore abstracts Ledger KV to avoid import cycles with internal/ledger.
type kvStore interface {
	Put(ctx context.Context, key string, value any) error
	Get(ctx context.Context, key string, dest any) (bool, error)
}

// SetPersistFile is a legacy no-op kept for backward compatibility.
func (h *History) SetPersistFile(_ string) error { return nil }

// SetKVStore enables Ledger KV-backed persistence for emotion history.
// When set, entries are periodically flushed to KV and restored on startup.
func (h *History) SetKVStore(kvs kvStore) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.kvs = kvs

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	h.loadFromKV(ctx)
}

// FlushToKV persists current entries to Ledger KV. Called during shutdown.
func (h *History) FlushToKV() {
	h.mu.RLock()
	kvs := h.kvs
	entries := make([]HistoryEntry, len(h.entries))
	copy(entries, h.entries)
	h.mu.RUnlock()

	if kvs == nil || len(entries) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := kvs.Put(ctx, "entries", entries); err != nil {
		slog.Error("emotion history: flush to KV failed", "err", err, "count", len(entries))
		return
	}
	slog.Info("emotion history: flushed to KV", "count", len(entries))
}

// loadFromKV restores entries from Ledger KV. Called during SetKVStore.
func (h *History) loadFromKV(ctx context.Context) {
	if h.kvs == nil {
		return
	}
	var entries []HistoryEntry
	found, err := h.kvs.Get(ctx, "entries", &entries)
	if err != nil {
		slog.Warn("emotion history: load from KV failed", "err", err)
		return
	}
	if !found || len(entries) == 0 {
		return
	}

	if len(entries) > h.maxSize {
		entries = entries[len(entries)-h.maxSize:]
	}
	h.entries = entries
	slog.Info("emotion history: restored from KV", "count", len(entries))
}
