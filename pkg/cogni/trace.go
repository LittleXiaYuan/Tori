package cogni

import (
	"crypto/sha1"
	"encoding/hex"
	"sort"
	"sync"
	"time"
)

// Trace is the structured record of a single Cogni evaluation cycle.
//
// Every turn that consults a Cogni Hook produces one Trace describing:
//   - which declarations were evaluated and their per-rule scores
//   - which were activated, suppressed by exclusivity, or short-circuited
//   - the size of the system-prompt context block that ended up injected
//   - the diff between the candidate skill set and the post-Surface result
//
// Traces are intentionally cheap (no LLM calls, just rule evaluation
// metadata) so they can be retained at high volume — the InMemoryTraceStore
// keeps the most recent N. They are the workshop log that proves the cogni
// layer is doing real work, not just shuffling prompts.
type Trace struct {
	Timestamp   time.Time          `json:"timestamp"`
	TenantID    string             `json:"tenant,omitempty"`
	Channel     string             `json:"channel,omitempty"`
	MessageHash string             `json:"message_hash,omitempty"`
	MessageLen  int                `json:"message_len"`
	Activations []TraceActivation  `json:"activations,omitempty"`
	Context     TraceContext       `json:"context,omitempty"`
	ToolFilter  *TraceToolFilter   `json:"tool_filter,omitempty"`
	DurationMs  int64              `json:"duration_ms"`
}

// TraceActivation captures the per-Cogni evaluation outcome.
type TraceActivation struct {
	ID          string   `json:"id"`
	DisplayName string   `json:"display_name,omitempty"`
	Score       float64  `json:"score"`
	Activated   bool     `json:"activated"`
	Reasons     []string `json:"reasons,omitempty"`
	// Suppressed reports whether ApplyExclusivity removed this entry in
	// favor of a higher-scoring sibling in the same Exclusive group.
	Suppressed     bool   `json:"suppressed,omitempty"`
	SuppressedByID string `json:"suppressed_by,omitempty"`
}

// TraceContext describes the system-prompt addition produced by a turn.
type TraceContext struct {
	// Bytes is the size of the assembled context block injected into the
	// planner's system prompt (0 when no cogni contributed).
	Bytes int `json:"bytes"`
	// Sources lists which Cogni IDs contributed at least one block of
	// rendered content. Useful for "why did the model receive this prompt?"
	// debugging.
	Sources []string `json:"sources,omitempty"`
	// TemplateFallbacks counts how many activated cognis hit a template
	// parse/execute error and fell back to their Static block.
	TemplateFallbacks int `json:"template_fallbacks,omitempty"`
}

// TraceToolFilter records the cogni Surface filter's effect on the planner's
// candidate skill list.
type TraceToolFilter struct {
	Before          int      `json:"before"`
	After           int      `json:"after"`
	Removed         []string `json:"removed,omitempty"`
	AppliedByCognis []string `json:"applied_by,omitempty"`
	// FellBackToInput is true when the union of every activated Surface
	// produced an empty set; the planner then ignores the filter rather
	// than locking the model out of every tool.
	FellBackToInput bool `json:"fellback_to_input,omitempty"`
}

// TraceStore is a write-once / read-many sink for Trace records.
type TraceStore interface {
	Record(t Trace)
	Recent(limit int) []Trace
	ByCogni(id string, limit int) []Trace
	Stats() TraceStats
}

// TraceStats is a coarse summary of the recorded traces — useful for the
// /v1/cognis admin page (per-cogni activation rate, tool-filter ratio, etc.).
type TraceStats struct {
	TotalTurns int            `json:"total_turns"`
	PerCogni   map[string]int `json:"per_cogni,omitempty"`
}

// InMemoryTraceStore is a fixed-size ring buffer of recent traces.
// Safe for concurrent use.
type InMemoryTraceStore struct {
	mu       sync.RWMutex
	cap      int
	entries  []Trace
	stats    TraceStats
	allTime  map[string]int
}

// NewInMemoryTraceStore creates a store retaining the most recent `capacity`
// traces. capacity <= 0 falls back to 256.
func NewInMemoryTraceStore(capacity int) *InMemoryTraceStore {
	if capacity <= 0 {
		capacity = 256
	}
	return &InMemoryTraceStore{
		cap:     capacity,
		entries: make([]Trace, 0, capacity),
		allTime: make(map[string]int),
	}
}

func (s *InMemoryTraceStore) Record(t Trace) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.entries) == s.cap {
		oldest := s.entries[0]
		for _, a := range oldest.Activations {
			if a.Activated {
				if s.allTime[a.ID] > 1 {
					s.allTime[a.ID]--
				} else {
					delete(s.allTime, a.ID)
				}
			}
		}
		copy(s.entries, s.entries[1:])
		s.entries = s.entries[:s.cap-1]
	}
	s.entries = append(s.entries, t)
	s.stats.TotalTurns++
	for _, a := range t.Activations {
		if a.Activated {
			s.allTime[a.ID]++
		}
	}
}

func (s *InMemoryTraceStore) Recent(limit int) []Trace {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 || limit > len(s.entries) {
		limit = len(s.entries)
	}
	out := make([]Trace, limit)
	// most recent first
	for i := 0; i < limit; i++ {
		out[i] = s.entries[len(s.entries)-1-i]
	}
	return out
}

func (s *InMemoryTraceStore) ByCogni(id string, limit int) []Trace {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Trace, 0)
	for i := len(s.entries) - 1; i >= 0; i-- {
		t := s.entries[i]
		for _, a := range t.Activations {
			if a.ID == id {
				out = append(out, t)
				break
			}
		}
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func (s *InMemoryTraceStore) Stats() TraceStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	per := make(map[string]int, len(s.allTime))
	for k, v := range s.allTime {
		per[k] = v
	}
	return TraceStats{
		TotalTurns: s.stats.TotalTurns,
		PerCogni:   per,
	}
}

// hashMessage produces a short, stable digest used in traces. Full message
// content is not retained to avoid logging PII; the hash lets operators
// correlate traces with audit logs that DO retain the message under access
// control.
func hashMessage(msg string) string {
	if msg == "" {
		return ""
	}
	sum := sha1.Sum([]byte(msg))
	return hex.EncodeToString(sum[:6]) // 12 hex chars: short diagnostic digest (not collision-free)
}

// sortStrings is a small helper kept private to avoid pulling in the sort
// package in callers; trace fields look nicer when alphabetized.
func sortStrings(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}
