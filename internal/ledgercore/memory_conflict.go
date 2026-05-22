// TODO(boundary): ConflictResolver's LLM judge path should migrate to yunque-agent/internal/agentcore/memory/.
// Simple strategies (newer_wins, higher_confidence_wins, merge) can stay as Ledger data-level operations.
// The ConflictLLMJudge strategy requires application-level intelligence.
// See BOUNDARY.md for details.
package ledger

import (
	"context"
	"time"
)

// ConflictStrategy determines how to resolve conflicting memories.
type ConflictStrategy string

const (
	ConflictNewerWins            ConflictStrategy = "newer_wins"
	ConflictHigherConfidenceWins ConflictStrategy = "higher_confidence_wins"
	ConflictMerge                ConflictStrategy = "merge"
	ConflictLLMJudge             ConflictStrategy = "llm_judge"
)

// ConflictResult describes the outcome of a conflict resolution.
type ConflictResult struct {
	Strategy ConflictStrategy `json:"strategy"`
	Winner   *MemoryEntry     `json:"winner"`
	Loser    *MemoryEntry     `json:"loser"`
	Merged   bool             `json:"merged"`
	Reason   string           `json:"reason"`
}

// ConflictJudgeFunc is an LLM-powered conflict resolver.
// Given two conflicting memories, it decides which to keep (or merges them).
type ConflictJudgeFunc func(ctx context.Context, existing, incoming *MemoryEntry) (*ConflictJudgement, error)

// ConflictJudgement is the LLM's decision on a memory conflict.
type ConflictJudgement struct {
	KeepExisting  bool    `json:"keep_existing"`
	MergedContent string  `json:"merged_content,omitempty"` // if non-empty, merge rather than pick one
	Confidence    float64 `json:"confidence"`
	Reason        string  `json:"reason"`
}

// ConflictResolver detects and resolves memory conflicts.
type ConflictResolver struct {
	backend  Backend
	memory   *MemoryStore
	strategy ConflictStrategy
	judgeFn  ConflictJudgeFunc
}

// NewConflictResolver creates a memory conflict resolver.
func NewConflictResolver(backend Backend, memory *MemoryStore, strategy ConflictStrategy) *ConflictResolver {
	return &ConflictResolver{
		backend:  backend,
		memory:   memory,
		strategy: strategy,
	}
}

// SetJudge sets the LLM-powered judge function (required for ConflictLLMJudge strategy).
func (cr *ConflictResolver) SetJudge(fn ConflictJudgeFunc) { cr.judgeFn = fn }

// CheckAndResolve checks if an incoming memory conflicts with existing ones
// and resolves any conflicts according to the configured strategy.
//
// Conflict detection uses two passes:
//  1. Exact key match (same key, different content) — deterministic, fast.
//  2. Content similarity search — catches semantically contradictory
//     memories stored under different keys (e.g. user.city vs user.location).
//
// Returns nil if no conflict was found.
func (cr *ConflictResolver) CheckAndResolve(ctx context.Context, incoming *MemoryEntry) (*ConflictResult, error) {
	// Pass 1: exact key match (original logic)
	byKey, err := cr.backend.SearchMemories(ctx, MemoryQuery{
		TenantID: incoming.TenantID,
		Query:    incoming.Key,
		Kinds:    []MemoryKind{incoming.Kind},
		Limit:    20,
	})
	if err != nil {
		return nil, err
	}
	for _, m := range byKey {
		if m.Key == incoming.Key && m.ID != incoming.ID && m.Content != incoming.Content {
			return cr.resolve(ctx, m, incoming)
		}
	}

	// Pass 2: content-based similarity search for cross-key conflicts.
	// Uses the content as the search query to find semantically related
	// memories that may contradict the incoming one.
	if incoming.Content != "" {
		byContent, err := cr.backend.SearchMemories(ctx, MemoryQuery{
			TenantID: incoming.TenantID,
			Query:    incoming.Content,
			Kinds:    []MemoryKind{incoming.Kind},
			Limit:    10,
		})
		if err != nil {
			return nil, err
		}
		for _, m := range byContent {
			if m.ID == incoming.ID {
				continue
			}
			// Only flag as conflict if content differs but key topic overlaps.
			// Heuristic: if both keys share a common prefix (e.g. "user.")
			// or content tokens have high overlap, they likely describe the
			// same subject. This avoids false positives from unrelated memories
			// that happen to share search keywords.
			if m.Content != incoming.Content && keysShareTopic(m.Key, incoming.Key) {
				return cr.resolve(ctx, m, incoming)
			}
		}
	}

	return nil, nil
}

// keysShareTopic checks if two memory keys likely refer to the same subject.
// Returns true if they share a common dot-separated prefix of length >= 1
// (e.g. "user.city" and "user.location" share "user").
func keysShareTopic(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	partsA := splitKeyPrefix(a)
	partsB := splitKeyPrefix(b)
	if partsA == "" || partsB == "" {
		return false
	}
	return partsA == partsB
}

func splitKeyPrefix(key string) string {
	for i, r := range key {
		if r == '.' {
			return key[:i]
		}
	}
	return ""
}

func (cr *ConflictResolver) resolve(ctx context.Context, existing, incoming *MemoryEntry) (*ConflictResult, error) {
	switch cr.strategy {
	case ConflictNewerWins:
		return cr.resolveNewerWins(ctx, existing, incoming)
	case ConflictHigherConfidenceWins:
		return cr.resolveHigherConfidence(ctx, existing, incoming)
	case ConflictMerge:
		return cr.resolveMerge(ctx, existing, incoming)
	case ConflictLLMJudge:
		return cr.resolveLLMJudge(ctx, existing, incoming)
	default:
		return cr.resolveNewerWins(ctx, existing, incoming)
	}
}

func (cr *ConflictResolver) resolveNewerWins(ctx context.Context, existing, incoming *MemoryEntry) (*ConflictResult, error) {
	// Newer memory wins; demote the older one
	existing.Confidence *= 0.5
	existing.UpdatedAt = time.Now()
	if err := cr.backend.PutMemory(ctx, existing); err != nil {
		return nil, err
	}

	return &ConflictResult{
		Strategy: ConflictNewerWins,
		Winner:   incoming,
		Loser:    existing,
		Reason:   "Newer information supersedes older memory",
	}, nil
}

func (cr *ConflictResolver) resolveHigherConfidence(ctx context.Context, existing, incoming *MemoryEntry) (*ConflictResult, error) {
	if incoming.Confidence >= existing.Confidence {
		existing.Confidence *= 0.5
		existing.UpdatedAt = time.Now()
		if err := cr.backend.PutMemory(ctx, existing); err != nil {
			return nil, err
		}
		return &ConflictResult{
			Strategy: ConflictHigherConfidenceWins,
			Winner:   incoming,
			Loser:    existing,
			Reason:   "Higher confidence memory wins",
		}, nil
	}

	// Existing wins ???demote incoming
	incoming.Confidence *= 0.5
	return &ConflictResult{
		Strategy: ConflictHigherConfidenceWins,
		Winner:   existing,
		Loser:    incoming,
		Reason:   "Existing memory has higher confidence",
	}, nil
}

func (cr *ConflictResolver) resolveMerge(ctx context.Context, existing, incoming *MemoryEntry) (*ConflictResult, error) {
	// Simple merge: combine content, take higher confidence, increase access count
	existing.Content = existing.Content + "\n[Updated] " + incoming.Content
	if incoming.Confidence > existing.Confidence {
		existing.Confidence = incoming.Confidence
	}
	existing.AccessCount += incoming.AccessCount
	existing.UpdatedAt = time.Now()
	if err := cr.backend.PutMemory(ctx, existing); err != nil {
		return nil, err
	}

	return &ConflictResult{
		Strategy: ConflictMerge,
		Winner:   existing,
		Merged:   true,
		Reason:   "Merged incoming content into existing memory",
	}, nil
}

func (cr *ConflictResolver) resolveLLMJudge(ctx context.Context, existing, incoming *MemoryEntry) (*ConflictResult, error) {
	if cr.judgeFn == nil {
		return cr.resolveNewerWins(ctx, existing, incoming)
	}

	judgement, err := cr.judgeFn(ctx, existing, incoming)
	if err != nil {
		return cr.resolveNewerWins(ctx, existing, incoming)
	}

	if judgement.MergedContent != "" {
		existing.Content = judgement.MergedContent
		existing.Confidence = judgement.Confidence
		existing.UpdatedAt = time.Now()
		if err := cr.backend.PutMemory(ctx, existing); err != nil {
			return nil, err
		}
		return &ConflictResult{
			Strategy: ConflictLLMJudge,
			Winner:   existing,
			Merged:   true,
			Reason:   judgement.Reason,
		}, nil
	}

	if judgement.KeepExisting {
		incoming.Confidence *= 0.3
		return &ConflictResult{
			Strategy: ConflictLLMJudge,
			Winner:   existing,
			Loser:    incoming,
			Reason:   judgement.Reason,
		}, nil
	}

	existing.Confidence *= 0.3
	existing.UpdatedAt = time.Now()
	if err := cr.backend.PutMemory(ctx, existing); err != nil {
		return nil, err
	}
	return &ConflictResult{
		Strategy: ConflictLLMJudge,
		Winner:   incoming,
		Loser:    existing,
		Reason:   judgement.Reason,
	}, nil
}

// ── ConflictResolver access from Ledger ──

// ConflictResolver creates a conflict resolver from the Ledger instance.
func (l *Ledger) ConflictResolver(strategy ConflictStrategy) *ConflictResolver {
	return NewConflictResolver(l.backend, l.Memory, strategy)
}

// ── Memory.PutWithConflictCheck convenience method ──

// PutWithConflictCheck stores a memory entry, automatically resolving conflicts.
func (ms *MemoryStore) PutWithConflictCheck(ctx context.Context, m *MemoryEntry, cr *ConflictResolver) (*ConflictResult, error) {
	if cr != nil {
		result, err := cr.CheckAndResolve(ctx, m)
		if err != nil {
			return nil, err
		}
		if result != nil {
			// If the loser is the incoming memory, still save but with reduced confidence
			if result.Loser != nil && result.Loser == m {
				m.Confidence = result.Loser.Confidence
			}
			if result.Merged {
				return result, nil // existing was already updated
			}
		}
	}

	if err := ms.Put(ctx, m); err != nil {
		return nil, err
	}
	return nil, nil
}
