package ledger

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"yunque-agent/internal/ledgercore"

	"yunque-agent/internal/agentcore/memory"
)

// LedgerPersister replaces the JSON file Persister.
// It persists Mid and Long memory layers to Ledger Memory (SQLite),
// providing atomic writes, crash safety, and indexed retrieval.
//
// Architecture: memory layers do computation in-memory (TF-IDF, BM25, vector);
// LedgerPersister handles persistence only — load on startup, flush periodically.
type LedgerPersister struct {
	ldg               *ledger.Ledger
	mid               *memory.MidTerm
	long              *memory.LongTerm
	temporalKV        *TemporalKVStore
	temporalNamespace string
	now               func() time.Time
	mu                sync.Mutex
	dirty             bool
	stop              chan struct{}
}

type temporalMemoryRecord struct {
	TenantID   string    `json:"tenant_id"`
	Layer      string    `json:"layer"`
	ID         string    `json:"id,omitempty"`
	Key        string    `json:"key"`
	Value      string    `json:"value"`
	Source     string    `json:"source,omitempty"`
	Category   string    `json:"category,omitempty"`
	Score      float64   `json:"score,omitempty"`
	AccessCnt  int       `json:"access_cnt,omitempty"`
	LastAccess time.Time `json:"last_access,omitempty"`
	CreatedAt  time.Time `json:"created_at,omitempty"`
	ExpiresAt  time.Time `json:"expires_at,omitempty"`
	FlushedAt  time.Time `json:"flushed_at"`
}

// LedgerPersisterOption configures the Ledger-backed memory persister.
type LedgerPersisterOption func(*LedgerPersister)

// WithLedgerPersisterTemporalKV mirrors dirty Mid/Long memory flushes into the
// temporal KV adapter used by Memory Time Travel. The primary Ledger Memory
// write remains authoritative; temporal write-back is best-effort and
// rollback-friendly.
func WithLedgerPersisterTemporalKV(store *TemporalKVStore) LedgerPersisterOption {
	return func(p *LedgerPersister) {
		p.temporalKV = store
	}
}

// WithLedgerPersisterTemporalNamespace overrides the namespace used for Memory
// Time Travel snapshots. Most callers should keep the default memory_snapshot.
func WithLedgerPersisterTemporalNamespace(namespace string) LedgerPersisterOption {
	return func(p *LedgerPersister) {
		if strings.TrimSpace(namespace) != "" {
			p.temporalNamespace = strings.TrimSpace(namespace)
		}
	}
}

// WithLedgerPersisterNow overrides the flush clock, primarily for tests.
func WithLedgerPersisterNow(now func() time.Time) LedgerPersisterOption {
	return func(p *LedgerPersister) {
		if now != nil {
			p.now = now
		}
	}
}

// NewLedgerPersister creates a persister backed by Ledger Memory.
// It loads existing data immediately and starts a background flush loop.
// If a legacy JSON file exists at legacyPath, it will be migrated.
func NewLedgerPersister(ldg *ledger.Ledger, mid *memory.MidTerm, long *memory.LongTerm, legacyPath string, opts ...LedgerPersisterOption) *LedgerPersister {
	p := &LedgerPersister{
		ldg:               ldg,
		mid:               mid,
		long:              long,
		temporalNamespace: "memory_snapshot",
		now:               func() time.Time { return time.Now().UTC() },
		stop:              make(chan struct{}),
	}
	for _, opt := range opts {
		opt(p)
	}

	// Migrate legacy JSON if it exists
	if legacyPath != "" {
		p.migrateLegacy(legacyPath)
	}

	// Load from Ledger
	p.load()

	go p.flushLoop()
	return p
}

// TemporalWritebackReady reports whether the Memory Time Travel temporal mirror
// is attached. It is intentionally narrow so pack status can avoid importing
// LedgerPersister internals.
func (p *LedgerPersister) TemporalWritebackReady() bool {
	return p != nil && p.temporalKV != nil
}

// MarkDirty signals that data has changed and needs saving.
func (p *LedgerPersister) MarkDirty() {
	p.mu.Lock()
	p.dirty = true
	p.mu.Unlock()
}

// Stop flushes final state and stops the background loop.
func (p *LedgerPersister) Stop() {
	close(p.stop)
	p.flush()
}

// load reads all memories from Ledger and populates the in-memory layers.
func (p *LedgerPersister) load() {
	ctx := context.Background()
	tenantID := os.Getenv("DEFAULT_TENANT_ID")
	if tenantID == "" {
		tenantID = "default"
	}

	// Load Mid-term (facts)
	midEntries, err := p.ldg.Memory.Search(ctx, ledger.MemoryQuery{
		TenantID: tenantID,
		Kinds:    []ledger.MemoryKind{ledger.MemoryFact},
		Limit:    10000,
	})

	midCount := 0
	if err == nil {
		for _, entry := range midEntries {
			item := ledgerEntryToItem(entry)
			_ = p.mid.Put(ctx, entry.TenantID, item)
			midCount++
		}
	} else {
		slog.Warn("ledger persister: failed to load mid-term memories", "err", err)
	}

	// Load Long-term (knowledge + experience)
	longEntries, err := p.ldg.Memory.Search(ctx, ledger.MemoryQuery{
		TenantID: tenantID,
		Kinds:    []ledger.MemoryKind{ledger.MemoryRule, ledger.MemoryExperience},
		Limit:    10000,
	})

	longCount := 0
	if err == nil {
		for _, entry := range longEntries {
			// Skip auto-collected training-data experiences. The DataCollector
			// stores raw conversation pairs (JSON {"user_message":...}) in the
			// Ledger for nightly training export — they are NOT user-facing
			// memory. Loading them into the recall layer pollutes "记得你"
			// recall (raw JSON, heavy BM25 weight, ranks above clean facts) and
			// wastes prompt tokens. They stay in the Ledger for export.
			if entry.Source == "training_data" {
				continue
			}
			item := ledgerEntryToItem(entry)
			_ = p.long.Put(ctx, entry.TenantID, item)
			longCount++
		}
	} else {
		slog.Warn("ledger persister: failed to load long-term memories", "err", err)
	}

	slog.Info("ledger persister: loaded from Ledger",
		"tenant", tenantID, "mid", midCount, "long", longCount)
}

// flush writes all dirty memory items to Ledger.
func (p *LedgerPersister) flush() {
	p.mu.Lock()
	if !p.dirty {
		p.mu.Unlock()
		return
	}
	p.dirty = false
	p.mu.Unlock()

	ctx := context.Background()
	total := 0
	temporalTotal := 0
	failed := 0
	flushedAt := p.now().UTC()

	// Export and persist Mid-term items
	midItems := p.exportMid()
	for tenantID, items := range midItems {
		for _, item := range items {
			entry := itemToLedgerEntry(tenantID, item, ledger.MemoryFact)
			if err := p.ldg.Memory.Put(ctx, entry); err != nil {
				slog.Warn("ledger persister: mid flush error", "key", item.Key, "err", err)
				failed++
			} else {
				total++
				if err := p.writeTemporalMemory(ctx, "mid", tenantID, item, flushedAt); err != nil {
					slog.Warn("ledger persister: temporal mid write-back error", "key", item.Key, "err", err)
				} else if p.temporalKV != nil {
					temporalTotal++
				}
			}
		}
	}

	// Export and persist Long-term items
	longItems := p.exportLong()
	for tenantID, items := range longItems {
		for _, item := range items {
			kind := ledger.MemoryRule
			if item.Category == "experience" {
				kind = ledger.MemoryExperience
			}
			entry := itemToLedgerEntry(tenantID, item, kind)
			if err := p.ldg.Memory.Put(ctx, entry); err != nil {
				slog.Warn("ledger persister: long flush error", "key", item.Key, "err", err)
				failed++
			} else {
				total++
				if err := p.writeTemporalMemory(ctx, "long", tenantID, item, flushedAt); err != nil {
					slog.Warn("ledger persister: temporal long write-back error", "key", item.Key, "err", err)
				} else if p.temporalKV != nil {
					temporalTotal++
				}
			}
		}
	}

	// Re-mark dirty when any write failed so the next flush retries instead
	// of letting in-memory state silently diverge from the ledger.
	if failed > 0 {
		p.mu.Lock()
		p.dirty = true
		p.mu.Unlock()
	}

	slog.Debug("ledger persister: flushed to Ledger", "items", total, "temporal_items", temporalTotal)
}

func (p *LedgerPersister) flushLoop() {
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

// migrateLegacy imports data from the old JSON file into Ledger.
func (p *LedgerPersister) migrateLegacy(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return // no legacy file, nothing to do
	}

	type snapshot struct {
		Mid  map[string][]memory.Item `json:"mid"`
		Long map[string][]memory.Item `json:"long"`
	}
	var snap snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		slog.Warn("ledger persister: legacy JSON parse failed", "err", err)
		return
	}

	ctx := context.Background()
	count := 0

	for tenantID, items := range snap.Mid {
		for _, item := range items {
			entry := itemToLedgerEntry(tenantID, item, ledger.MemoryFact)
			if err := p.ldg.Memory.Put(ctx, entry); err == nil {
				count++
			}
		}
	}
	for tenantID, items := range snap.Long {
		for _, item := range items {
			entry := itemToLedgerEntry(tenantID, item, ledger.MemoryRule)
			if err := p.ldg.Memory.Put(ctx, entry); err == nil {
				count++
			}
		}
	}

	// Rename legacy file to .migrated
	_ = os.Rename(path, path+".migrated")
	slog.Info("ledger persister: migrated legacy JSON to Ledger",
		"items", count, "old_file", path)
}

// --- Conversion helpers ---

func itemToLedgerEntry(tenantID string, item memory.Item, kind ledger.MemoryKind) *ledger.MemoryEntry {
	meta := map[string]interface{}{
		"category":    item.Category,
		"source":      item.Source,
		"access_cnt":  item.AccessCnt,
		"last_access": item.LastAccess,
		"score":       item.Score,
	}
	metaJSON, _ := json.Marshal(meta)

	return &ledger.MemoryEntry{
		ID:          item.ID,
		TenantID:    tenantID,
		Kind:        kind,
		Key:         item.Key,
		Content:     item.Value,
		Source:      item.Source,
		Confidence:  item.Score,
		AccessCount: item.AccessCnt,
		Metadata:    ledger.JSON(metaJSON),
	}
}

func ledgerEntryToItem(entry *ledger.MemoryEntry) memory.Item {
	var meta map[string]interface{}
	if entry.Metadata != nil {
		_ = json.Unmarshal([]byte(entry.Metadata), &meta)
	}

	category := ""
	if meta != nil {
		if c, ok := meta["category"].(string); ok {
			category = c
		}
	}

	accessCnt := entry.AccessCount
	var lastAccess time.Time
	if entry.LastAccess != nil {
		lastAccess = *entry.LastAccess
	}

	return memory.Item{
		ID:         entry.ID,
		Key:        entry.Key,
		Value:      entry.Content,
		Source:     entry.Source,
		Category:   category,
		Score:      entry.Confidence,
		AccessCnt:  accessCnt,
		LastAccess: lastAccess,
		CreatedAt:  entry.CreatedAt,
	}
}

// --- Export helpers (mirror memory.Persister's export methods) ---

func (p *LedgerPersister) exportMid() map[string][]memory.Item {
	return p.mid.ExportAll()
}

func (p *LedgerPersister) exportLong() map[string][]memory.Item {
	return p.long.ExportAll()
}

func (p *LedgerPersister) writeTemporalMemory(ctx context.Context, layer, tenantID string, item memory.Item, flushedAt time.Time) error {
	if p.temporalKV == nil {
		return nil
	}
	record := temporalMemoryRecord{
		TenantID:   tenantID,
		Layer:      layer,
		ID:         item.ID,
		Key:        item.Key,
		Value:      item.Value,
		Source:     item.Source,
		Category:   item.Category,
		Score:      item.Score,
		AccessCnt:  item.AccessCnt,
		LastAccess: item.LastAccess,
		CreatedAt:  item.CreatedAt,
		ExpiresAt:  item.ExpiresAt,
		FlushedAt:  flushedAt,
	}
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return p.temporalKV.PutRawVersionedAt(ctx, p.temporalNamespace, temporalMemoryKey(layer, tenantID, item), data, flushedAt)
}

func temporalMemoryKey(layer, tenantID string, item memory.Item) string {
	identity := strings.TrimSpace(item.Key)
	if layer == "long" && strings.TrimSpace(item.ID) != "" {
		identity = item.ID
	}
	if identity == "" {
		identity = strings.TrimSpace(item.ID)
	}
	if identity == "" {
		sum := sha256.Sum256([]byte(layer + "\x00" + tenantID + "\x00" + item.Value))
		identity = hex.EncodeToString(sum[:8])
	}
	return fmt.Sprintf("%s/%s/%s", tenantID, layer, identity)
}
