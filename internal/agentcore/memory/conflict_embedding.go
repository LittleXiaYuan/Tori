package memory

import (
	"context"
	"log/slog"
	"math"
	"sync"
	"time"
)

// SingleEmbedFunc produces a vector embedding for a single piece of text.
// It is intentionally *not* typed against a concrete embedder interface so
// that the memory package stays free of a direct embeddings dependency
// (the embeddings subsystem already imports from llm/..., and pulling it
// into memory creates a deep import graph). Callers adapt their embedder.
//
// Returning a nil slice or a non-nil error disables the gate for that call;
// the caller must never panic from a misbehaving provider.
//
// Note: the existing EmbedFunc in long.go is the *batch* shape
// (func(ctx, []string) ([][]float32, error)). Conflict detection only
// ever embeds one item at a time, so we keep this single-shot shape to
// avoid forcing callers to wrap their batch embedder per-call.
type SingleEmbedFunc func(ctx context.Context, text string) ([]float32, error)

// EmbeddingGateConfig tunes how the cosine-similarity pre-filter behaves.
type EmbeddingGateConfig struct {
	// Threshold is the minimum cosine similarity at which a stored fact is
	// considered a plausible conflict target. 0.82 is the empirical mid-point
	// where OpenAI text-embedding-3-small / BGE-m3 reliably pair semantically
	// related Chinese/English sentences without false-matching unrelated ones.
	// Set <= 0 to disable filtering (every existing item is forwarded to the
	// downstream LLM/heuristic arbiter).
	Threshold float64

	// NewContentCacheTTL controls how long embeddings for incoming content
	// stay cached. A short TTL is enough because Ingest fires the conflict
	// check synchronously per new fact.
	NewContentCacheTTL time.Duration

	// MaxCandidates is a hard cap on how many existing items survive the
	// gate, sorted by similarity descending. Prevents a single very-noisy
	// embedding from flooding the LLM arbiter. 0 means unlimited.
	MaxCandidates int

	// TenantCacheTTL controls how long per-tenant stored-memory embeddings
	// are cached. Longer values reduce embed API calls at the cost of stale
	// vectors when memories are updated/deleted. Default: 5 minutes.
	TenantCacheTTL time.Duration

	// TenantCacheMaxItems caps the number of stored-memory vectors cached
	// per tenant. Prevents unbounded memory growth for tenants with large
	// memory stores. Default: 200.
	TenantCacheMaxItems int
}

// DefaultEmbeddingGateConfig returns sensible defaults.
func DefaultEmbeddingGateConfig() EmbeddingGateConfig {
	return EmbeddingGateConfig{
		Threshold:           0.82,
		NewContentCacheTTL:  30 * time.Second,
		MaxCandidates:       10,
		TenantCacheTTL:      5 * time.Minute,
		TenantCacheMaxItems: 200,
	}
}

const tenantShardCount = 8

// embeddingGate is an internal helper on ConflictDetector. Keeping the
// embedding logic in its own type makes conflict.go easier to reason about
// and lets tests exercise the cosine math in isolation.
//
// Concurrency: recentCache uses a dedicated RWMutex (read-heavy path);
// tenantVecs are sharded across tenantShardCount buckets keyed by FNV hash
// of the tenantID, each with its own RWMutex to minimize cross-tenant lock
// contention.
type embeddingGate struct {
	cacheMu     sync.RWMutex
	embed       SingleEmbedFunc
	cfg         EmbeddingGateConfig
	recentCache map[string]cachedVec
	shards      [tenantShardCount]tenantShard
}

type tenantShard struct {
	mu   sync.RWMutex
	vecs map[string]*tenantVecCache // tenantID → cached stored-memory vectors
}

type cachedVec struct {
	vec []float32
	at  time.Time
}

// tenantVecCache holds pre-embedded vectors for a tenant's stored memories,
// avoiding O(existing) embed calls on every Ingest.
type tenantVecCache struct {
	items   map[string]cachedVec // content hash → vector
	created time.Time
}

func tenantShardIdx(tenantID string) int {
	var h uint32 = 2166136261
	for i := 0; i < len(tenantID); i++ {
		h ^= uint32(tenantID[i])
		h *= 16777619
	}
	return int(h % tenantShardCount)
}

// SetEmbeddingGate enables embedding-based candidate filtering before the
// LLM / heuristic arbitration runs. Passing a nil embed disables the gate.
//
// This matches TECH-DEBT-2026-04-18 item #11 / orchestrator.go's
// TODO(memory.conflict): "Embed content once, keep top-K above a cosine
// threshold, delegate the arbitration to the existing LLM path, fall back
// to keyword matching when no embedder is configured."
func (d *ConflictDetector) SetEmbeddingGate(embed SingleEmbedFunc, cfg EmbeddingGateConfig) {
	if cfg.NewContentCacheTTL <= 0 {
		cfg.NewContentCacheTTL = 30 * time.Second
	}
	if embed == nil {
		d.embGate = nil
		return
	}
	if cfg.TenantCacheTTL <= 0 {
		cfg.TenantCacheTTL = 5 * time.Minute
	}
	if cfg.TenantCacheMaxItems <= 0 {
		cfg.TenantCacheMaxItems = 200
	}
	gate := &embeddingGate{
		embed:       embed,
		cfg:         cfg,
		recentCache: make(map[string]cachedVec),
	}
	for i := range gate.shards {
		gate.shards[i].vecs = make(map[string]*tenantVecCache)
	}
	d.embGate = gate
}

// filterByEmbedding returns the subset of `existing` items whose content is
// at least `cfg.Threshold` cosine-similar to `newContent`. Items are sorted
// by descending similarity and capped at cfg.MaxCandidates.
//
// Returns (filteredItems, true) when the gate actually ran, or
// (originalItems, false) when the gate could not run — in which case the
// caller should behave as if no gate existed. The signal is deliberately
// separate from the slice so the caller does not silently drop items on
// a transient embedder failure.
func (g *embeddingGate) filterByEmbedding(
	ctx context.Context,
	newContent string,
	existing []RecallItem,
) ([]RecallItem, bool) {
	return g.filterByEmbeddingForTenant(ctx, "", newContent, existing)
}

// filterByEmbeddingForTenant is the tenant-aware variant that leverages
// per-tenant vector caches to avoid re-embedding stored memories on every
// Ingest call — resolving orchestrator.go TODO #4.
func (g *embeddingGate) filterByEmbeddingForTenant(
	ctx context.Context,
	tenantID string,
	newContent string,
	existing []RecallItem,
) ([]RecallItem, bool) {
	if g == nil || g.embed == nil || len(existing) == 0 || newContent == "" {
		return existing, false
	}

	newVec, err := g.embedCached(ctx, newContent)
	if err != nil || len(newVec) == 0 {
		slog.Warn("conflict.embeddingGate: failed to embed new content, skipping gate", "err", err)
		return existing, false
	}

	type scored struct {
		item  RecallItem
		score float64
	}
	var candidates []scored
	for _, item := range existing {
		if item.Content == "" {
			continue
		}
		vec, err := g.embedForTenant(ctx, tenantID, item.Content)
		if err != nil || len(vec) == 0 {
			continue
		}
		score := cosineSimilarity32(newVec, vec)
		if score < g.cfg.Threshold {
			continue
		}
		candidates = append(candidates, scored{item: item, score: score})
	}

	for i := 1; i < len(candidates); i++ {
		for j := i; j > 0 && candidates[j-1].score < candidates[j].score; j-- {
			candidates[j-1], candidates[j] = candidates[j], candidates[j-1]
		}
	}
	if g.cfg.MaxCandidates > 0 && len(candidates) > g.cfg.MaxCandidates {
		candidates = candidates[:g.cfg.MaxCandidates]
	}

	out := make([]RecallItem, len(candidates))
	for i, c := range candidates {
		out[i] = c.item
	}
	return out, true
}

// embedForTenant looks up a stored-memory vector from the per-tenant shard
// cache first, falling back to embedCached (and populating the shard) on miss.
func (g *embeddingGate) embedForTenant(ctx context.Context, tenantID, content string) ([]float32, error) {
	if tenantID == "" {
		return g.embedCached(ctx, content)
	}

	shard := &g.shards[tenantShardIdx(tenantID)]

	// Fast path: RLock for cache hit
	shard.mu.RLock()
	tc := shard.vecs[tenantID]
	if tc != nil && time.Since(tc.created) <= g.cfg.TenantCacheTTL {
		if hit, ok := tc.items[content]; ok {
			shard.mu.RUnlock()
			return hit.vec, nil
		}
	}
	shard.mu.RUnlock()

	vec, err := g.embedCached(ctx, content)
	if err != nil || len(vec) == 0 {
		return vec, err
	}

	// Slow path: write lock to populate cache
	shard.mu.Lock()
	defer shard.mu.Unlock()

	tc = shard.vecs[tenantID]
	if tc == nil || time.Since(tc.created) > g.cfg.TenantCacheTTL {
		tc = &tenantVecCache{items: make(map[string]cachedVec), created: time.Now()}
		shard.vecs[tenantID] = tc
	}
	if len(tc.items) < g.cfg.TenantCacheMaxItems {
		tc.items[content] = cachedVec{vec: vec, at: time.Now()}
	}

	// Per-shard eviction: cap at 64/tenantShardCount tenants per shard
	maxPerShard := 64 / tenantShardCount
	if maxPerShard < 4 {
		maxPerShard = 4
	}
	if len(shard.vecs) > maxPerShard {
		var oldestTenant string
		var oldest time.Time
		first := true
		for tid, tvc := range shard.vecs {
			if first || tvc.created.Before(oldest) {
				oldest = tvc.created
				oldestTenant = tid
				first = false
			}
		}
		delete(shard.vecs, oldestTenant)
	}
	return vec, nil
}

// InvalidateTenantCache removes the cached vectors for a specific tenant,
// useful when memories are bulk-updated or deleted.
func (g *embeddingGate) InvalidateTenantCache(tenantID string) {
	if g == nil {
		return
	}
	shard := &g.shards[tenantShardIdx(tenantID)]
	shard.mu.Lock()
	delete(shard.vecs, tenantID)
	shard.mu.Unlock()
}

// embedCached is a tiny TTL cache so the same incoming content is not
// re-embedded when the orchestrator loops through overlapping recall sets.
// Uses RWMutex: RLock for cache-hit fast path, Lock only on miss+store.
func (g *embeddingGate) embedCached(ctx context.Context, text string) ([]float32, error) {
	now := time.Now()

	// Fast path: RLock for cache hit
	g.cacheMu.RLock()
	if hit, ok := g.recentCache[text]; ok && now.Sub(hit.at) < g.cfg.NewContentCacheTTL {
		vec := hit.vec
		g.cacheMu.RUnlock()
		return vec, nil
	}
	g.cacheMu.RUnlock()

	vec, err := g.embed(ctx, text)
	if err != nil {
		return nil, err
	}

	// Slow path: write lock to store result
	g.cacheMu.Lock()
	defer g.cacheMu.Unlock()
	g.recentCache[text] = cachedVec{vec: vec, at: now}
	if len(g.recentCache) > 512 {
		var oldestKey string
		var oldest time.Time
		first := true
		for k, v := range g.recentCache {
			if first || v.at.Before(oldest) {
				oldest = v.at
				oldestKey = k
				first = false
			}
		}
		delete(g.recentCache, oldestKey)
	}
	return vec, nil
}

// cosineSimilarity32 is duplicated here (the embeddings package has a public
// version) to keep the memory package import graph flat. Sharing that
// function would pull memory -> agentcore/embeddings -> agentcore/llm, which
// is the exact dependency cycle TECH-DEBT-2026-04-18 §4.5 flags on planner.
func cosineSimilarity32(a, b []float32) float64 {
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
