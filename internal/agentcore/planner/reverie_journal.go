package planner

import (
	"fmt"
	"time"
)

// JournalContext returns the most relevant thoughts for injection into the system prompt.
// Scores by trigram similarity (70%) + significance (20%) + freshness (10%).
func (r *Reverie) JournalContext(maxThoughts int, query string) string {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.journal) == 0 {
		return ""
	}

	// Filter: only inject thoughts that are both significant AND relevant.
	// Higher threshold ensures only genuinely useful context enters the prompt.
	minSig := r.cfg.MinSignificance
	if minSig < 0.6 {
		minSig = 0.6
	}

	qTrigrams := buildTrigrams(query)
	hasQuery := len(qTrigrams) > 0

	// Minimum relevance gate: thoughts must share at least some content overlap
	// to be worth injecting. Without a query, only very high-significance thoughts qualify.
	const minRelevance = 0.08

	type scored struct {
		t     Thought
		score float64
		sim   float64
	}
	var candidates []scored
	now := time.Now()
	for _, t := range r.journal {
		if t.Significance < minSig {
			continue
		}

		// Freshness decay: thoughts older than 24h are penalized
		age := now.Sub(t.CreatedAt)
		freshness := 1.0
		if age > 24*time.Hour {
			freshness = 0.7
		} else if age > 6*time.Hour {
			freshness = 0.85
		}

		// Category boost: actionable categories get priority
		catBoost := 0.0
		switch t.Category {
		case "insight":
			catBoost = 0.10
		case "concern":
			catBoost = 0.08
		case "idea":
			catBoost = 0.05
		case "observation":
			catBoost = 0.0 // generic observations are lowest value
		}

		var finalScore float64
		var sim float64
		if hasQuery {
			// Query-aware: relevance is primary, significance is secondary
			sim = trigramSimilarity(qTrigrams, buildTrigrams(t.Content))
			if sim < minRelevance {
				continue // not relevant enough — skip entirely
			}
			finalScore = sim*0.70 + (t.Significance+catBoost)*0.20 + freshness*0.10
		} else {
			// No query: only very high-significance, fresh thoughts
			if t.Significance < 0.8 {
				continue
			}
			finalScore = (t.Significance + catBoost) * freshness
		}
		candidates = append(candidates, scored{t, finalScore, sim})
	}
	if len(candidates) == 0 {
		return ""
	}

	// Sort by final score descending
	for i := 1; i < len(candidates); i++ {
		for j := i; j > 0 && candidates[j].score > candidates[j-1].score; j-- {
			candidates[j], candidates[j-1] = candidates[j-1], candidates[j]
		}
	}
	if len(candidates) > maxThoughts {
		candidates = candidates[:maxThoughts]
	}

	var parts []string
	for _, c := range candidates {
		parts = append(parts, fmt.Sprintf("- [%s] %s", c.t.Category, truncateStr(c.t.Content, 120)))
	}
	return "相关的内心洞察:\n" + join(parts, "\n")
}

// buildTrigrams returns the set of character trigrams for s.
// Using a map deduplicates repeated trigrams so Jaccard is not inflated by repetition.
func buildTrigrams(s string) map[string]struct{} {
	runes := []rune(s)
	set := make(map[string]struct{}, len(runes))
	for i := 0; i+2 < len(runes); i++ {
		set[string(runes[i:i+3])] = struct{}{}
	}
	return set
}

// trigramSimilarity returns the Jaccard coefficient between two trigram sets.
// Range [0, 1]: 0 = no overlap, 1 = identical.
func trigramSimilarity(a, b map[string]struct{}) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	intersection := 0
	for k := range a {
		if _, ok := b[k]; ok {
			intersection++
		}
	}
	union := len(a) + len(b) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}
