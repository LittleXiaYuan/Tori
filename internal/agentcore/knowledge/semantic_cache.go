package knowledge

import (
	"context"
	"math"
	"sync"
	"sync/atomic"
	"time"
)

// SemanticCache caches retrieval results keyed by query embedding similarity.
// When a new query arrives, its embedding is compared against cached queries;
// if cosine similarity exceeds the threshold, the cached results are returned
// without re-running the full retrieval pipeline.
type SemanticCache struct {
	mu        sync.RWMutex
	entries   []*cacheEntry
	maxSize   int
	threshold float64       // cosine similarity threshold for cache hit (default 0.85)
	ttl       time.Duration // entry expiration duration
	version   int64         // bumped on invalidation; Lookup checks staleness

	hits   atomic.Int64
	misses atomic.Int64
}

type cacheEntry struct {
	queryVec   []float32
	queryText  string
	results    []ScoredChunk
	createdAt  time.Time
	lastAccess time.Time
}

// SemanticCacheConfig holds configuration for SemanticCache.
type SemanticCacheConfig struct {
	MaxSize   int           // max cached entries (default 128)
	Threshold float64       // similarity threshold for hit (default 0.85)
	TTL       time.Duration // entry TTL (default 10min)
}

// NewSemanticCache creates a new semantic cache.
func NewSemanticCache(cfg SemanticCacheConfig) *SemanticCache {
	if cfg.MaxSize <= 0 {
		cfg.MaxSize = 128
	}
	if cfg.Threshold <= 0 || cfg.Threshold > 1 {
		cfg.Threshold = 0.85
	}
	if cfg.TTL <= 0 {
		cfg.TTL = 10 * time.Minute
	}
	return &SemanticCache{
		entries:   make([]*cacheEntry, 0, cfg.MaxSize),
		maxSize:   cfg.MaxSize,
		threshold: cfg.Threshold,
		ttl:       cfg.TTL,
	}
}

// Lookup checks if a similar query exists in cache.
// Returns cached results and true on hit, nil and false on miss.
func (sc *SemanticCache) Lookup(queryVec []float32) ([]ScoredChunk, bool) {
	sc.mu.RLock()
	now := time.Now()
	bestIdx := -1
	bestSim := 0.0

	for i, e := range sc.entries {
		if now.Sub(e.createdAt) > sc.ttl {
			continue
		}
		sim := cosineSim32(queryVec, e.queryVec)
		if sim > bestSim {
			bestSim = sim
			bestIdx = i
		}
	}

	if bestIdx < 0 || bestSim < sc.threshold {
		sc.mu.RUnlock()
		return nil, false
	}

	results := make([]ScoredChunk, len(sc.entries[bestIdx].results))
	copy(results, sc.entries[bestIdx].results)
	sc.mu.RUnlock()

	// Update lastAccess under write lock to avoid data race
	sc.mu.Lock()
	if bestIdx < len(sc.entries) {
		sc.entries[bestIdx].lastAccess = now
	}
	sc.mu.Unlock()

	return results, true
}

// Put stores query results in cache, evicting LRU entry if full.
func (sc *SemanticCache) Put(queryVec []float32, queryText string, results []ScoredChunk) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	now := time.Now()

	// Evict expired entries first
	alive := sc.entries[:0]
	for _, e := range sc.entries {
		if now.Sub(e.createdAt) <= sc.ttl {
			alive = append(alive, e)
		}
	}
	sc.entries = alive

	// LRU eviction if still at capacity
	for len(sc.entries) >= sc.maxSize {
		lruIdx := 0
		for i := 1; i < len(sc.entries); i++ {
			if sc.entries[i].lastAccess.Before(sc.entries[lruIdx].lastAccess) {
				lruIdx = i
			}
		}
		sc.entries[lruIdx] = sc.entries[len(sc.entries)-1]
		sc.entries = sc.entries[:len(sc.entries)-1]
	}

	stored := make([]ScoredChunk, len(results))
	copy(stored, results)
	vecCopy := make([]float32, len(queryVec))
	copy(vecCopy, queryVec)

	sc.entries = append(sc.entries, &cacheEntry{
		queryVec:   vecCopy,
		queryText:  queryText,
		results:    stored,
		createdAt:  now,
		lastAccess: now,
	})
}

// RecordHit increments the hit counter (lock-free).
func (sc *SemanticCache) RecordHit() { sc.hits.Add(1) }

// RecordMiss increments the miss counter (lock-free).
func (sc *SemanticCache) RecordMiss() { sc.misses.Add(1) }

// Stats returns cache hit/miss statistics.
type CacheStats struct {
	Hits    int64   `json:"hits"`
	Misses  int64   `json:"misses"`
	Size    int     `json:"size"`
	HitRate float64 `json:"hit_rate"`
}

func (sc *SemanticCache) Stats() CacheStats {
	h := sc.hits.Load()
	m := sc.misses.Load()
	total := h + m
	hitRate := 0.0
	if total > 0 {
		hitRate = float64(h) / float64(total)
	}
	sc.mu.RLock()
	size := len(sc.entries)
	sc.mu.RUnlock()
	return CacheStats{
		Hits:    h,
		Misses:  m,
		Size:    size,
		HitRate: hitRate,
	}
}

// Invalidate clears all cached entries. Call when underlying chunks change.
func (sc *SemanticCache) Invalidate() {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.entries = sc.entries[:0]
	sc.version++
}

// Clear removes all entries and resets counters.
func (sc *SemanticCache) Clear() {
	sc.mu.Lock()
	sc.entries = sc.entries[:0]
	sc.version++
	sc.mu.Unlock()
	sc.hits.Store(0)
	sc.misses.Store(0)
}

// ──────────────────────────────────────────────
// Store integration
// ──────────────────────────────────────────────

// SetSemanticCache attaches a semantic cache to the store.
func (s *Store) SetSemanticCache(sc *SemanticCache) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.semCache = sc
}

// CachedHybridSearch performs HybridSearch with semantic caching.
// If the query embedding is similar to a cached query, returns cached results.
func (s *Store) CachedHybridSearch(ctx context.Context, query string, limit int) ([]ScoredChunk, bool) {
	s.mu.RLock()
	sem := s.semantic
	cache := s.semCache
	s.mu.RUnlock()

	if sem == nil || sem.embedder == nil || cache == nil {
		return s.HybridSearch(ctx, query, limit), false
	}

	qVec, err := sem.embedder.Embed(ctx, query)
	if err != nil {
		return s.HybridSearch(ctx, query, limit), false
	}

	if results, hit := cache.Lookup(qVec); hit {
		cache.RecordHit()
		if len(results) > limit {
			results = results[:limit]
		}
		return results, true
	}

	cache.RecordMiss()
	results := s.HybridSearch(ctx, query, limit)
	cache.Put(qVec, query, results)
	return results, false
}

// cosineSim32 computes cosine similarity between two float32 vectors.
func cosineSim32(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}
	return dot / denom
}
