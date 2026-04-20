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
}

// DefaultEmbeddingGateConfig returns sensible defaults.
func DefaultEmbeddingGateConfig() EmbeddingGateConfig {
	return EmbeddingGateConfig{
		Threshold:          0.82,
		NewContentCacheTTL: 30 * time.Second,
		MaxCandidates:      10,
	}
}

// embeddingGate is an internal helper on ConflictDetector. Keeping the
// embedding logic in its own type makes conflict.go easier to reason about
// and lets tests exercise the cosine math in isolation.
type embeddingGate struct {
	mu          sync.Mutex
	embed       SingleEmbedFunc
	cfg         EmbeddingGateConfig
	recentCache map[string]cachedVec
}

type cachedVec struct {
	vec []float32
	at  time.Time
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
	d.embGate = &embeddingGate{
		embed:       embed,
		cfg:         cfg,
		recentCache: make(map[string]cachedVec),
	}
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
		vec, err := g.embedCached(ctx, item.Content)
		if err != nil || len(vec) == 0 {
			// Failing to embed a *specific* existing item is non-fatal; drop
			// that one and let the rest through. We do NOT degrade the whole
			// call because the caller may be processing dozens of items and
			// one provider glitch should not poison all of them.
			continue
		}
		score := cosineSimilarity32(newVec, vec)
		if score < g.cfg.Threshold {
			continue
		}
		candidates = append(candidates, scored{item: item, score: score})
	}

	// Sort candidates by descending score (simple insertion sort is fine for
	// MaxCandidates-level N; we typically stay under 32 here).
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

// embedCached is a tiny TTL cache so the same incoming content is not
// re-embedded when the orchestrator loops through overlapping recall sets.
func (g *embeddingGate) embedCached(ctx context.Context, text string) ([]float32, error) {
	now := time.Now()
	g.mu.Lock()
	if hit, ok := g.recentCache[text]; ok && now.Sub(hit.at) < g.cfg.NewContentCacheTTL {
		vec := hit.vec
		g.mu.Unlock()
		return vec, nil
	}
	g.mu.Unlock()

	vec, err := g.embed(ctx, text)
	if err != nil {
		return nil, err
	}

	g.mu.Lock()
	defer g.mu.Unlock()
	g.recentCache[text] = cachedVec{vec: vec, at: now}
	// Bound cache size. The cache is populated inside a per-ingest loop, so
	// a naïve cap is enough; when we hit the limit, drop the oldest entry.
	//
	// Seed oldestKey with *any* existing key (not empty) so that Windows'
	// ~1ms timer resolution – where dozens of consecutive inserts can
	// share a `time.Now()` – cannot leave us with `oldestKey == ""`, which
	// would turn the delete into a silent no-op and let the cache grow
	// unbounded.
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
