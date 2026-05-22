package ledger

import (
	"context"
	"log/slog"
	"math"
	"time"
)

// MemoryLifecycle manages memory consolidation, decay, and garbage collection.
type MemoryLifecycle struct {
	backend  Backend
	vector   *VectorIndex

	decayHalfLifeDays float64 // half-life for confidence decay (default 30)
	gcMinConfidence   float64 // memories below this confidence are GC'd (default 0.05)
	consolidateThresh float64 // cosine similarity threshold for merging (default 0.92)
}

// NewMemoryLifecycle creates a lifecycle manager.
func NewMemoryLifecycle(backend Backend, vector *VectorIndex) *MemoryLifecycle {
	return &MemoryLifecycle{
		backend:           backend,
		vector:            vector,
		decayHalfLifeDays: 30,
		gcMinConfidence:   0.05,
		consolidateThresh: 0.92,
	}
}

// SetDecayHalfLife sets the half-life in days for confidence decay.
func (ml *MemoryLifecycle) SetDecayHalfLife(days float64) { ml.decayHalfLifeDays = days }

// SetGCThreshold sets the minimum confidence below which memories are garbage collected.
func (ml *MemoryLifecycle) SetGCThreshold(threshold float64) { ml.gcMinConfidence = threshold }

// RunDecay applies FSRS-based confidence decay to all memories.
// Uses paginated processing to handle arbitrarily large memory stores
// without loading everything into memory at once.
func (ml *MemoryLifecycle) RunDecay(ctx context.Context, tenantID string) (int, error) {
	const pageSize = 500
	updated := 0
	offset := 0

	for {
		entries, err := ml.backend.SearchMemories(ctx, MemoryQuery{
			TenantID: tenantID,
			Limit:    pageSize,
			Offset:   offset,
		})
		if err != nil {
			return updated, err
		}
		if len(entries) == 0 {
			break
		}

		n := ml.decayPage(ctx, entries)
		updated += n
		offset += len(entries)

		if len(entries) < pageSize {
			break
		}
	}

	return updated, nil
}

// decayPage processes a single page of memory entries.
func (ml *MemoryLifecycle) decayPage(ctx context.Context, entries []*MemoryEntry) int {
	updated := 0
	now := time.Now()
	baseStability := ml.decayHalfLifeDays * 24 * float64(time.Hour)

	// FSRS parameters (same as yunque-agent's orchestrator)
	const w1, w2, w3 = 0.9, 0.5, 2.0

	for _, m := range entries {
		ref := m.UpdatedAt
		if m.LastAccess != nil && m.LastAccess.After(m.UpdatedAt) {
			ref = *m.LastAccess
		}
		ageSec := now.Sub(ref).Seconds()

		// FSRS stability: each access simulates a successful review
		stability := baseStability
		difficulty := 5.0
		n := m.AccessCount
		for i := 0; i < n && i < 50; i++ {
			frac := float64(i+1) / float64(n+1)
			t := ageSec * frac
			r := math.Exp(-t / stability)
			if r < 0.01 {
				r = 0.01
			}
			growth := 1.0 + math.Exp(w1)*math.Pow(difficulty, -w2)*(math.Exp(w3*(1.0-r))-1.0)
			if growth < 1.0 {
				growth = 1.0
			}
			if growth > 10.0 {
				growth = 10.0
			}
			stability *= growth
		}

		decayFactor := math.Exp(-0.693 * ageSec / stability)
		newConf := m.Confidence * decayFactor
		if newConf > m.Confidence {
			newConf = m.Confidence
		}

		if math.Abs(newConf-m.Confidence) > 0.01 {
			m.Confidence = newConf
			m.UpdatedAt = now
			ml.backend.PutMemory(ctx, m)
			updated++
		}
	}

	return updated
}

// RunGC removes memories with confidence below the threshold.
func (ml *MemoryLifecycle) RunGC(ctx context.Context, tenantID string) (int, error) {
	entries, err := ml.backend.SearchMemories(ctx, MemoryQuery{
		TenantID: tenantID,
		Limit:    5000,
	})
	if err != nil {
		return 0, err
	}

	removed := 0
	now := time.Now()

	for _, m := range entries {
		shouldRemove := false

		if m.Confidence < ml.gcMinConfidence {
			shouldRemove = true
		}
		if m.ExpiresAt != nil && m.ExpiresAt.Before(now) {
			shouldRemove = true
		}

		if shouldRemove {
			ml.backend.DeleteMemory(ctx, m.ID)
			removed++
		}
	}

	slog.Info("lifecycle: gc complete", "tenant", tenantID, "removed", removed, "total", len(entries))
	return removed, nil
}

// RunConsolidate merges semantically similar memories to reduce redundancy.
// Requires vector index to be configured.
func (ml *MemoryLifecycle) RunConsolidate(ctx context.Context, tenantID string) (int, error) {
	if ml.vector == nil || !ml.vector.Enabled() {
		return 0, nil
	}

	entries, err := ml.backend.SearchMemories(ctx, MemoryQuery{
		TenantID: tenantID,
		Limit:    1000,
	})
	if err != nil {
		return 0, err
	}

	type embedEntry struct {
		mem   *MemoryEntry
		embed []float32
	}
	var withEmbed []embedEntry
	for _, m := range entries {
		if len(m.Embedding) > 0 {
			withEmbed = append(withEmbed, embedEntry{mem: m, embed: m.Embedding})
		}
	}

	merged := 0
	consumed := make(map[string]bool)

	for i := 0; i < len(withEmbed); i++ {
		a := withEmbed[i]
		if consumed[a.mem.ID] {
			continue
		}
		for j := i + 1; j < len(withEmbed); j++ {
			b := withEmbed[j]
			if consumed[b.mem.ID] {
				continue
			}
			if a.mem.Kind != b.mem.Kind {
				continue
			}
			sim := CosineSimilarity(a.embed, b.embed)
			if sim >= ml.consolidateThresh {
				a.mem.Confidence = math.Min(1.0, a.mem.Confidence+b.mem.Confidence*0.3)
				a.mem.AccessCount += b.mem.AccessCount
				// Preserve information from both memories: keep the longer
				// content as the base but append unique details from the
				// shorter one (separated by a marker) so nothing is silently
				// dropped during consolidation.
				longer, shorter := a.mem.Content, b.mem.Content
				if len(longer) < len(shorter) {
					longer, shorter = shorter, longer
				}
				if shorter != "" && shorter != longer {
					a.mem.Content = longer + "\n[consolidated] " + shorter
				} else {
					a.mem.Content = longer
				}
				a.mem.UpdatedAt = time.Now()
				ml.backend.PutMemory(ctx, a.mem)
				ml.backend.DeleteMemory(ctx, b.mem.ID)
				consumed[b.mem.ID] = true
				merged++
			}
		}
	}

	slog.Info("lifecycle: consolidate complete", "tenant", tenantID, "merged", merged, "candidates", len(withEmbed))
	return merged, nil
}

// RunExpireStale marks memories that have never been accessed and are older
// than maxAge as expired. This prevents orphaned memories from accumulating.
func (ml *MemoryLifecycle) RunExpireStale(ctx context.Context, tenantID string, maxAge time.Duration) (int, error) {
	if maxAge == 0 {
		maxAge = 90 * 24 * time.Hour // default: 90 days
	}
	entries, err := ml.backend.SearchMemories(ctx, MemoryQuery{
		TenantID: tenantID,
		Limit:    5000,
	})
	if err != nil {
		return 0, err
	}

	expired := 0
	cutoff := time.Now().Add(-maxAge)
	for _, m := range entries {
		if m.AccessCount == 0 && m.CreatedAt.Before(cutoff) && m.ExpiresAt == nil {
			now := time.Now()
			m.ExpiresAt = &now // mark for next GC cycle
			m.UpdatedAt = now
			ml.backend.PutMemory(ctx, m)
			expired++
		}
	}

	slog.Info("lifecycle: expire stale complete", "tenant", tenantID, "expired", expired)
	return expired, nil
}

// LifecycleResult summarizes one run of all lifecycle operations.
type LifecycleResult struct {
	Decayed      int `json:"decayed"`
	GarbageCol   int `json:"garbage_collected"`
	Consolidated int `json:"consolidated"`
	Expired      int `json:"expired"`
}

// RunAll runs all lifecycle operations in order: decay ???expire stale ???GC ???consolidation.
// This is the recommended single entry point for the nighttime scheduler.
func (ml *MemoryLifecycle) RunAll(ctx context.Context, tenantID string) (*LifecycleResult, error) {
	result := &LifecycleResult{}
	var err error

	result.Decayed, err = ml.RunDecay(ctx, tenantID)
	if err != nil {
		return result, err
	}

	result.Expired, _ = ml.RunExpireStale(ctx, tenantID, 0)

	result.GarbageCol, err = ml.RunGC(ctx, tenantID)
	if err != nil {
		return result, err
	}

	result.Consolidated, _ = ml.RunConsolidate(ctx, tenantID)

	// Auto-retrain IVF index when incremental drift exceeds threshold
	if ml.vector != nil && ml.vector.IVFNeedsRetrain() {
		if err := ml.vector.TrainIVF(ctx, tenantID); err != nil {
			slog.Warn("lifecycle: IVF retrain failed", "err", err)
		} else {
			slog.Info("lifecycle: IVF retrained due to incremental drift", "tenant", tenantID)
		}
	}

	slog.Info("lifecycle: RunAll complete",
		"tenant", tenantID,
		"decayed", result.Decayed,
		"expired", result.Expired,
		"gc", result.GarbageCol,
		"consolidated", result.Consolidated,
	)
	return result, nil
}
