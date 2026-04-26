package cogni

import (
	"sort"
	"time"
)

// HealthMetrics is the per-Cogni rollup computed over the most recent N
// traces. It is the SLA dashboard for a single declarative cognition unit.
//
// The fields are intentionally small numbers so the JSON payload is cheap
// for the admin UI to refresh on a tight loop. None of them require LLM
// calls — every value comes from the structured Trace events emitted by
// Hook.evaluate(). This is the proof that "cogni is more than prompt
// engineering": the same call that injects context also yields a SLO that
// can be charted, alerted on, and gated against.
type HealthMetrics struct {
	ID         string `json:"id"`
	Score      int    `json:"score"`     // 0..100, higher = healthier
	Status     string `json:"status"`    // "healthy" | "warn" | "unhealthy" | "idle"
	WindowSize int    `json:"window"`    // number of traces considered

	Evaluations    int     `json:"evaluations"`     // turns where the cogni was scored
	Activations    int     `json:"activations"`     // turns where it actually engaged
	Suppressed     int     `json:"suppressed"`      // turns where exclusivity removed it
	ActivationRate float64 `json:"activation_rate"` // activations / evaluations
	SuppressionRate float64 `json:"suppression_rate"`

	AvgDurationMs   int     `json:"avg_duration_ms"`   // mean turn duration when this cogni was activated
	AvgContextBytes int     `json:"avg_context_bytes"` // mean prompt addition when activated
	TemplateFallbackRate float64 `json:"template_fallback_rate"`

	// ToolFilterRatio is the average ratio of (after / before) for every
	// turn this cogni was a member of `applied_by`. <1.0 means the cogni
	// is actively narrowing the LLM's tool choices.
	ToolFilterRatio float64 `json:"tool_filter_ratio"`

	LastSeenAt string `json:"last_seen_at,omitempty"`
}

// Monitor derives HealthMetrics from a TraceStore on demand. Computation
// is cheap (O(N) over recent traces); callers can poll on a UI refresh
// without worrying about backpressure.
type Monitor struct {
	store TraceStore
}

// NewMonitor wraps a TraceStore. nil store yields a Monitor that returns
// empty metrics — useful so callers don't need to nil-check.
func NewMonitor(store TraceStore) *Monitor {
	return &Monitor{store: store}
}

// ComputeFor produces the rollup for a single cogni id. windowSize bounds
// the number of recent traces examined (0 = use store default).
func (m *Monitor) ComputeFor(id string, windowSize int) HealthMetrics {
	out := HealthMetrics{ID: id}
	if m == nil || m.store == nil {
		out.Status = "idle"
		return out
	}
	traces := m.store.Recent(windowSize)
	return computeMetrics(id, traces)
}

// ComputeAll returns one HealthMetrics per cogni that appears in any of
// the most recent `windowSize` traces. The result is sorted by ID for
// stable UI rendering.
func (m *Monitor) ComputeAll(windowSize int) []HealthMetrics {
	if m == nil || m.store == nil {
		return nil
	}
	traces := m.store.Recent(windowSize)
	if len(traces) == 0 {
		return nil
	}

	seen := make(map[string]bool)
	for _, t := range traces {
		for _, a := range t.Activations {
			seen[a.ID] = true
		}
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	out := make([]HealthMetrics, 0, len(ids))
	for _, id := range ids {
		out = append(out, computeMetrics(id, traces))
	}
	return out
}

func computeMetrics(id string, traces []Trace) HealthMetrics {
	out := HealthMetrics{ID: id, WindowSize: len(traces)}
	if len(traces) == 0 {
		out.Status = "idle"
		out.Score = 50 // neutral when no data
		return out
	}

	var (
		durSum, durN              int64
		ctxSum, ctxN              int
		toolBefore, toolAfter int
		fellbackTurns         int
		fallbackTurns         int // template fallback turns where this cogni contributed
		lastSeen              time.Time
	)

	for _, t := range traces {
		// find this cogni's evaluation in the trace
		var entry *TraceActivation
		for i := range t.Activations {
			if t.Activations[i].ID == id {
				entry = &t.Activations[i]
				break
			}
		}
		if entry == nil {
			continue
		}
		out.Evaluations++
		if entry.Activated {
			out.Activations++
			if t.Timestamp.After(lastSeen) {
				lastSeen = t.Timestamp
			}
			if t.DurationMs > 0 {
				durSum += t.DurationMs
				durN++
			}
			// Context bytes attribution: divide equally across sources.
			if t.Context.Bytes > 0 && len(t.Context.Sources) > 0 {
				for _, src := range t.Context.Sources {
					if src == id {
						share := t.Context.Bytes / len(t.Context.Sources)
						ctxSum += share
						ctxN++
						break
					}
				}
			}
			if t.Context.TemplateFallbacks > 0 {
				fallbackTurns++
			}
			// Tool filter contribution:
			if t.ToolFilter != nil {
				for _, applier := range t.ToolFilter.AppliedByCognis {
					if applier == id {
						toolBefore += t.ToolFilter.Before
						toolAfter += t.ToolFilter.After
						if t.ToolFilter.FellBackToInput {
							fellbackTurns++
						}
						break
					}
				}
			}
		} else if entry.Suppressed {
			out.Suppressed++
		}
	}

	if out.Evaluations > 0 {
		out.ActivationRate = round3(float64(out.Activations) / float64(out.Evaluations))
		out.SuppressionRate = round3(float64(out.Suppressed) / float64(out.Evaluations))
	}
	if out.Activations > 0 {
		out.TemplateFallbackRate = round3(float64(fallbackTurns) / float64(out.Activations))
	}
	if durN > 0 {
		out.AvgDurationMs = int(durSum / durN)
	}
	if ctxN > 0 {
		out.AvgContextBytes = ctxSum / ctxN
	}
	if toolBefore > 0 {
		out.ToolFilterRatio = round3(float64(toolAfter) / float64(toolBefore))
	}
	if !lastSeen.IsZero() {
		out.LastSeenAt = lastSeen.UTC().Format(time.RFC3339)
	}
	out.Score = scoreOf(out, fellbackTurns)
	out.Status = statusOfScore(out)
	return out
}

// scoreOf is a heuristic 0..100 health score:
//   - Activations earn baseline; large-volume confidence weighting
//   - Template fallbacks subtract aggressively (a parse error is a bug)
//   - Heavy tool surface fallback (FellBackToInput) subtracts (filter is impotent)
//   - Suppression rate >50% suggests the rule is dead weight
//   - No activations at all → 50 (idle, neither healthy nor sick)
func scoreOf(m HealthMetrics, surfaceFallbackTurns int) int {
	if m.Activations == 0 {
		if m.Suppressed > 0 && m.SuppressionRate > 0.8 {
			return 25 // always loses exclusivity → likely misconfigured
		}
		return 50
	}
	score := 70.0
	if m.ActivationRate >= 0.05 {
		score += 10
	}
	if m.ActivationRate >= 0.20 {
		score += 5
	}
	if m.TemplateFallbackRate > 0.05 {
		score -= 25 * m.TemplateFallbackRate * 4 // up to -25 at 25% fallback
	}
	if surfaceFallbackTurns > 0 && m.Activations > 0 {
		ratio := float64(surfaceFallbackTurns) / float64(m.Activations)
		score -= 20 * ratio
	}
	if m.SuppressionRate > 0.5 {
		score -= 10
	}
	if m.ToolFilterRatio > 0 && m.ToolFilterRatio < 0.5 {
		score += 5 // actively narrowing surface is "doing something"
	}
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	return int(score + 0.5)
}

func statusOfScore(m HealthMetrics) string {
	switch {
	case m.Activations == 0 && m.Suppressed == 0:
		return "idle"
	case m.Score >= 70:
		return "healthy"
	case m.Score >= 40:
		return "warn"
	default:
		return "unhealthy"
	}
}
