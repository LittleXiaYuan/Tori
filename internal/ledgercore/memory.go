package ledger

import (
	"context"
	"sort"
	"sync"
	"time"

	"yunque-agent/internal/ledgercore/internal/ulid"
)

// MemoryStore manages structured memory entries with classification and provenance.
type MemoryStore struct {
	backend Backend
	events  *EventStore

	// Batched access count updates to reduce write amplification
	accessMu      sync.Mutex
	pendingAccess map[string]int // memoryID -> pending access count increments

	// LRU read-through cache keyed by memory ID.
	// Invalidated on Put/Delete. Safe for concurrent reads with sync.Map.
	cacheMu  sync.RWMutex
	cache    map[string]*cachedEntry
	cacheMax int // max entries (0 = disabled)
}

type cachedEntry struct {
	entry    *MemoryEntry
	cachedAt time.Time
}

const (
	defaultCacheMax = 512
	cacheTTL        = 5 * time.Minute
)

// Put creates or updates a memory entry and emits the corresponding event.
func (ms *MemoryStore) Put(ctx context.Context, m *MemoryEntry) error {
	now := time.Now()
	isNew := m.ID == ""

	if m.ID == "" {
		m.ID = ulid.New()
	}
	if m.CreatedAt.IsZero() {
		m.CreatedAt = now
	}
	m.UpdatedAt = now

	if m.Source == "" {
		m.Source = "extraction"
	}
	if m.Confidence == 0 {
		m.Confidence = 0.5
	}
	if m.Metadata == nil {
		m.Metadata = JSON("{}")
	}

	if err := ms.backend.PutMemory(ctx, m); err != nil {
		return err
	}

	ms.cacheInvalidate(m.ID)

	// Emit event
	kind := EventMemoryUpdated
	if isNew {
		kind = EventMemoryWritten
	}

	payload := MakePayload(map[string]interface{}{
		"key":  m.Key,
		"kind": m.Kind,
	})

	taskID := ""
	if m.TaskID != nil {
		taskID = *m.TaskID
	}

	// Only emit if we have a task context
	if taskID != "" {
		ms.events.Append(ctx, &Event{
			ID:        ulid.New(),
			TaskID:    taskID,
			Kind:      kind,
			Actor:     "runtime",
			Payload:   payload,
			CreatedAt: now,
		})
	}

	return nil
}

// Get retrieves a memory entry by ID and increments its access count.
// Hot entries are served from an LRU read-through cache.
// Access count updates are batched to reduce write amplification.
func (ms *MemoryStore) Get(ctx context.Context, id string) (*MemoryEntry, error) {
	if m := ms.cacheGet(id); m != nil {
		ms.bumpAccess(ctx, id)
		clone := *m
		clone.AccessCount++
		now := time.Now()
		clone.LastAccess = &now
		return &clone, nil
	}

	m, err := ms.backend.GetMemory(ctx, id)
	if err != nil {
		return nil, err
	}

	ms.cachePut(id, m)
	ms.bumpAccess(ctx, id)

	m.AccessCount++
	now := time.Now()
	m.LastAccess = &now

	return m, nil
}

func (ms *MemoryStore) bumpAccess(ctx context.Context, id string) {
	ms.accessMu.Lock()
	if ms.pendingAccess == nil {
		ms.pendingAccess = make(map[string]int)
	}
	ms.pendingAccess[id]++
	shouldFlush := len(ms.pendingAccess) >= 50
	ms.accessMu.Unlock()

	if shouldFlush {
		ms.FlushAccessCounts(ctx)
	}
}

func (ms *MemoryStore) cacheGet(id string) *MemoryEntry {
	ms.cacheMu.RLock()
	defer ms.cacheMu.RUnlock()

	if ms.cache == nil {
		return nil
	}
	ce, ok := ms.cache[id]
	if !ok {
		return nil
	}
	if time.Since(ce.cachedAt) > cacheTTL {
		return nil
	}
	return ce.entry
}

func (ms *MemoryStore) cachePut(id string, m *MemoryEntry) {
	max := ms.cacheMax
	if max <= 0 {
		max = defaultCacheMax
	}

	ms.cacheMu.Lock()
	defer ms.cacheMu.Unlock()

	if ms.cache == nil {
		ms.cache = make(map[string]*cachedEntry, max)
	}

	// Eviction: first remove expired entries, then if still over capacity
	// sort by cachedAt and batch-evict the oldest entries to reach 75% fill.
	if len(ms.cache) >= max {
		target := max / 4
		for k, v := range ms.cache {
			if time.Since(v.cachedAt) > cacheTTL {
				delete(ms.cache, k)
			}
		}
		if remaining := len(ms.cache) - (max - target); remaining > 0 {
			type aged struct {
				id       string
				cachedAt time.Time
			}
			entries := make([]aged, 0, len(ms.cache))
			for k, v := range ms.cache {
				entries = append(entries, aged{k, v.cachedAt})
			}
			sort.Slice(entries, func(i, j int) bool {
				return entries[i].cachedAt.Before(entries[j].cachedAt)
			})
			for i := 0; i < remaining && i < len(entries); i++ {
				delete(ms.cache, entries[i].id)
			}
		}
	}

	clone := *m
	ms.cache[id] = &cachedEntry{entry: &clone, cachedAt: time.Now()}
}

func (ms *MemoryStore) cacheInvalidate(id string) {
	ms.cacheMu.Lock()
	delete(ms.cache, id)
	ms.cacheMu.Unlock()
}

// SetCacheSize configures the maximum number of cached memory entries.
// Set to 0 to disable caching.
func (ms *MemoryStore) SetCacheSize(max int) {
	ms.cacheMu.Lock()
	ms.cacheMax = max
	if max == 0 {
		ms.cache = nil
	}
	ms.cacheMu.Unlock()
}

// FlushAccessCounts persists all pending access count updates.
func (ms *MemoryStore) FlushAccessCounts(ctx context.Context) {
	ms.accessMu.Lock()
	pending := ms.pendingAccess
	ms.pendingAccess = make(map[string]int)
	ms.accessMu.Unlock()

	for id, count := range pending {
		m, err := ms.backend.GetMemory(ctx, id)
		if err != nil {
			continue
		}
		m.AccessCount += count
		now := time.Now()
		m.LastAccess = &now
		m.UpdatedAt = now
		ms.backend.PutMemory(ctx, m) // best-effort
	}
}

// Search finds memories matching the query criteria.
func (ms *MemoryStore) Search(ctx context.Context, q MemoryQuery) ([]*MemoryEntry, error) {
	return ms.backend.SearchMemories(ctx, q)
}

// Count returns the total number of memories matching the query.
func (ms *MemoryStore) Count(ctx context.Context, q MemoryQuery) (int, error) {
	q.Limit = 5000
	entries, err := ms.backend.SearchMemories(ctx, q)
	if err != nil {
		return 0, err
	}
	return len(entries), nil
}

// SearchByKey finds memories with an exact key match.
func (ms *MemoryStore) SearchByKey(ctx context.Context, tenantID, key string) ([]*MemoryEntry, error) {
	return ms.backend.SearchMemories(ctx, MemoryQuery{
		TenantID: tenantID,
		Key:      key,
		Limit:    100,
	})
}

// Delete removes a memory entry and emits a deletion event.
func (ms *MemoryStore) Delete(ctx context.Context, id string) error {
	m, err := ms.backend.GetMemory(ctx, id)
	if err != nil {
		return err
	}

	if err := ms.backend.DeleteMemory(ctx, id); err != nil {
		return err
	}

	ms.cacheInvalidate(id)

	// Emit event if task-scoped
	if m.TaskID != nil {
		payload := MakePayload(map[string]interface{}{
			"key":  m.Key,
			"kind": m.Kind,
		})
		ms.events.Append(ctx, &Event{
			ID:        ulid.New(),
			TaskID:    *m.TaskID,
			Kind:      EventMemoryDeleted,
			Actor:     "runtime",
			Payload:   payload,
			CreatedAt: time.Now(),
		})
	}

	return nil
}

// PutFact is a convenience method for storing a fact-type memory.
func (ms *MemoryStore) PutFact(ctx context.Context, tenantID, key, content, source string) error {
	return ms.Put(ctx, &MemoryEntry{
		TenantID:   tenantID,
		Kind:       MemoryFact,
		Key:        key,
		Content:    content,
		Source:     source,
		Confidence: 0.8,
	})
}

// PutExperience stores a success/failure experience memory.
func (ms *MemoryStore) PutExperience(ctx context.Context, tenantID string, taskID *string, key, content string, confidence float64) error {
	return ms.Put(ctx, &MemoryEntry{
		TenantID:   tenantID,
		TaskID:     taskID,
		Kind:       MemoryExperience,
		Key:        key,
		Content:    content,
		Source:     "extraction",
		Confidence: confidence,
	})
}

// PutPreference stores a user preference memory.
func (ms *MemoryStore) PutPreference(ctx context.Context, tenantID, key, content string) error {
	return ms.Put(ctx, &MemoryEntry{
		TenantID:   tenantID,
		Kind:       MemoryPreference,
		Key:        key,
		Content:    content,
		Source:     "user",
		Confidence: 1.0,
	})
}
