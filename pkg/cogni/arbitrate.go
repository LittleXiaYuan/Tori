package cogni

import "sort"

// ArbitrationConfig controls per-turn capability arbitration — "which experts
// win this turn". The zero value is identity (no floor, unlimited), so the
// legacy behavior (every activated cogni composes) is preserved unless a host
// opts in.
type ArbitrationConfig struct {
	// MaxActive caps how many cognis compose in a single turn (top-K by bid).
	// 0 = unlimited.
	MaxActive int
	// MinConfidence drops activations whose score is below this floor.
	// 0 = no floor.
	MinConfidence float64
}

// IsZero reports whether the config requests no arbitration.
func (c ArbitrationConfig) IsZero() bool { return c.MaxActive <= 0 && c.MinConfidence <= 0 }

// Arbitrate selects the winning activations for a turn — the capability-level
// "bidding" that turns "every matching cogni activates" into "the best K experts
// win". Each activation's Score is its bid: highest bids win, ties break by
// priority (lower number first) then ID for determinism, MinConfidence drops
// weak bids, and MaxActive caps how many cognis compose.
//
// Input is assumed already exclusivity-filtered and Activated. A zero config
// returns the input unchanged (identity), preserving the legacy "all activated
// compose" behavior and the caller's ordering — so arbitration is strictly
// opt-in and backward compatible.
//
// This is the cheap MoE router: it shapes ONE model call's capability surface
// with a deterministic pass, instead of spawning a sub-agent loop per expert.
func Arbitrate(activations []Activation, cfg ArbitrationConfig) []Activation {
	if cfg.IsZero() {
		return activations
	}
	kept := make([]Activation, 0, len(activations))
	for _, a := range activations {
		if a.Score >= cfg.MinConfidence {
			kept = append(kept, a)
		}
	}
	sort.SliceStable(kept, func(i, j int) bool {
		if kept[i].Score != kept[j].Score {
			return kept[i].Score > kept[j].Score
		}
		pi, pj := priority(kept[i].Declaration), priority(kept[j].Declaration)
		if pi != pj {
			return pi < pj
		}
		return arbDeclID(kept[i]) < arbDeclID(kept[j])
	})
	if cfg.MaxActive > 0 && len(kept) > cfg.MaxActive {
		kept = kept[:cfg.MaxActive]
	}
	return kept
}

func arbDeclID(a Activation) string {
	if a.Declaration != nil {
		return a.Declaration.ID
	}
	return ""
}
