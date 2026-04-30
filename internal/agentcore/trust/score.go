package trust

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"yunque-agent/internal/agentcore/audit"
	iledger "yunque-agent/internal/ledger"
)

// ErrAdminRequired is returned when a privileged trust operation is called
// without admin role.
var ErrAdminRequired = fmt.Errorf("admin role required for this operation")

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
	mu     sync.RWMutex
	scores map[string]*Entry
	path   string // legacy JSON persistence path
	kvs    *iledger.KVConfigStore
	audit  *audit.Chain
}

// SetAudit attaches a Merkle audit chain for logging privileged operations.
func (t *Tracker) SetAudit(chain *audit.Chain) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.audit = chain
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

// SetKVStore enables Ledger KV-backed persistence, replacing file I/O.
// Once set, all save/load operations go through Ledger KV.
func (t *Tracker) SetKVStore(kvs *iledger.KVConfigStore) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.kvs = kvs
	t.loadFromKV()
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

// Seed sets a skill's trust score directly without per-promotion logging.
// Used during startup to pre-seed built-in skills to a trusted level.
func (t *Tracker) Seed(slug string, score int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	e := t.getOrCreate(slug)
	if e.Score >= score {
		return
	}
	e.Score = score
	if e.Score > 100 {
		e.Score = 100
	}
	e.LastPromoted = time.Now()
	t.save()
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

// GrantFull sets a skill's trust to the maximum level (100), granting full
// permissions including shell access. Requires admin role; the caller identity
// is recorded in the audit chain.
func (t *Tracker) GrantFull(slug, callerID, callerRole string) error {
	if callerRole != "admin" {
		slog.Warn("trust: GrantFull denied — admin required", "caller", callerID, "role", callerRole, "slug", slug)
		return ErrAdminRequired
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	e := t.getOrCreate(slug)
	e.Score = 100
	e.LastPromoted = time.Now()
	t.save()

	if t.audit != nil {
		t.audit.Append(audit.EventAuth, callerID,
			"trust_grant_full",
			fmt.Sprintf("slug=%s score=100 role=%s", slug, callerRole))
	}
	slog.Info("trust: granted full trust", "slug", slug, "score", e.Score, "by", callerID)
	return nil
}

// GrantFullAll sets all tracked skills' trust to the maximum level. Requires
// admin role; the operation is logged in the audit chain.
func (t *Tracker) GrantFullAll(callerID, callerRole string) (int, error) {
	if callerRole != "admin" {
		slog.Warn("trust: GrantFullAll denied — admin required", "caller", callerID, "role", callerRole)
		return 0, ErrAdminRequired
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	count := 0
	for _, e := range t.scores {
		if e.Score < 100 {
			e.Score = 100
			e.LastPromoted = time.Now()
			count++
		}
	}
	t.save()

	if t.audit != nil {
		t.audit.Append(audit.EventAuth, callerID,
			"trust_grant_full_all",
			fmt.Sprintf("upgraded=%d role=%s", count, callerRole))
	}
	slog.Info("trust: granted full trust to all skills", "upgraded", count, "by", callerID)
	return count, nil
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

func (t *Tracker) loadFromKV() {
	if t.kvs == nil {
		return
	}
	var scores map[string]*Entry
	found, err := t.kvs.Get(context.Background(), "scores", &scores)
	if err != nil {
		slog.Warn("trust: kv load failed", "err", err)
		return
	}
	if found && len(scores) > 0 {
		t.scores = scores
		slog.Info("trust: loaded from Ledger KV", "skills", len(scores))
	}
}

func (t *Tracker) save() {
	if t.kvs != nil {
		if err := t.kvs.Put(context.Background(), "scores", t.scores); err != nil {
			slog.Warn("trust: kv save failed, falling back to file", "err", err)
		} else {
			return
		}
	}
	if t.path == "" {
		return
	}
	data, _ := json.MarshalIndent(t.scores, "", "  ")
	os.MkdirAll("data", 0755)
	os.WriteFile(t.path, data, 0644)
}
