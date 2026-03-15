package trust

import (
	"encoding/json"
	"log/slog"
	"os"
	"sync"
	"time"
)

// PermLevel defines what a skill is allowed to do at a given trust level.
type PermLevel int

const (
	PermReadOnly PermLevel = iota // score 0-29
	PermWrite                     // score 30-59
	PermNetwork                   // score 60-79
	PermShell                     // score 80+ (still needs user confirm)
)

func (p PermLevel) String() string {
	switch p {
	case PermReadOnly:
		return "read-only"
	case PermWrite:
		return "write"
	case PermNetwork:
		return "network"
	case PermShell:
		return "shell"
	}
	return "unknown"
}

// Entry records trust data for one skill.
type Entry struct {
	Score        int       `json:"score"`
	Executions   int       `json:"executions"`
	Failures     int       `json:"failures"`
	LastPromoted time.Time `json:"last_promoted,omitempty"`
}

// Allowed returns the permission level this entry grants.
func (e Entry) Allowed() PermLevel {
	switch {
	case e.Score >= 80:
		return PermShell
	case e.Score >= 60:
		return PermNetwork
	case e.Score >= 30:
		return PermWrite
	default:
		return PermReadOnly
	}
}

// Tracker manages trust scores for all skills.
type Tracker struct {
	mu      sync.RWMutex
	scores  map[string]*Entry
	path    string // persistence path
}

// NewTracker creates a trust tracker, optionally loading from file.
func NewTracker(persistPath string) *Tracker {
	t := &Tracker{
		scores: make(map[string]*Entry),
		path:   persistPath,
	}
	t.load()
	return t
}

// Get returns the trust entry for a skill (zero-value if unknown).
func (t *Tracker) Get(slug string) Entry {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if e, ok := t.scores[slug]; ok {
		return *e
	}
	return Entry{}
}

// RecordSuccess increments trust after a successful, safe execution.
func (t *Tracker) RecordSuccess(slug string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	e := t.getOrCreate(slug)
	e.Executions++
	oldLevel := e.Allowed()
	e.Score++
	if e.Score > 100 {
		e.Score = 100
	}
	if e.Allowed() > oldLevel {
		e.LastPromoted = time.Now()
		slog.Info("trust: promoted", "slug", slug, "level", e.Allowed().String(), "score", e.Score)
	}
	t.save()
}

// RecordFailure decreases trust after a dangerous or erroneous behavior.
func (t *Tracker) RecordFailure(slug string, severity int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	e := t.getOrCreate(slug)
	e.Failures++
	e.Score -= severity
	if e.Score < 0 {
		e.Score = 0
	}
	slog.Warn("trust: penalized", "slug", slug, "severity", severity, "score", e.Score)
	t.save()
}

// RecordDanger is a heavy penalty (e.g. user-reported problem).
func (t *Tracker) RecordDanger(slug string) {
	t.RecordFailure(slug, 50)
}

// Reset clears a skill's trust back to zero.
func (t *Tracker) Reset(slug string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.scores, slug)
	t.save()
}

// CheckPermission returns true if the skill has enough trust for the requested level.
func (t *Tracker) CheckPermission(slug string, required PermLevel) bool {
	return t.Get(slug).Allowed() >= required
}

// All returns all tracked entries.
func (t *Tracker) All() map[string]Entry {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make(map[string]Entry, len(t.scores))
	for k, v := range t.scores {
		out[k] = *v
	}
	return out
}

func (t *Tracker) getOrCreate(slug string) *Entry {
	if e, ok := t.scores[slug]; ok {
		return e
	}
	e := &Entry{}
	t.scores[slug] = e
	return e
}

func (t *Tracker) load() {
	if t.path == "" {
		return
	}
	data, err := os.ReadFile(t.path)
	if err != nil {
		return
	}
	json.Unmarshal(data, &t.scores)
}

func (t *Tracker) save() {
	if t.path == "" {
		return
	}
	data, _ := json.MarshalIndent(t.scores, "", "  ")
	os.MkdirAll("data", 0755)
	os.WriteFile(t.path, data, 0644)
}
