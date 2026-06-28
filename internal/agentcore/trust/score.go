package trust

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"os"
	"sort"
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

// Entry records trust data for one skill using Bayesian Beta distribution.
// Trust score = α/(α+β) (posterior mean of Beta(α,β)).
// Confidence = 1 - Var(Beta) = 1 - αβ/((α+β)²(α+β+1)).
// Permission upgrades require BOTH sufficient score AND high confidence,
// preventing new skills from gaining privileges with few observations.
type Entry struct {
	Score        int       `json:"score"`
	Executions   int       `json:"executions"`
	Failures     int       `json:"failures"`
	LastPromoted time.Time `json:"last_promoted,omitempty"`
	Alpha        float64   `json:"alpha"`
	BetaParam    float64   `json:"beta"`
}

// betaCapMultiplier bounds how far β can exceed α after failures.
// Without a cap, a single high-severity failure permanently suppresses the
// Bayesian score (β ≫ α drives the posterior mean → 0). Cap = 5×α + 1
// preserves penalty signal while keeping the score recoverable through
// continued successes.
const betaCapMultiplier = 5.0

// BayesianScore returns the posterior mean of the Beta(α,β) distribution,
// scaled to [0, 100] for backward compatibility with threshold-based checks.
func (e Entry) BayesianScore() float64 {
	a, b := e.effectiveAlphaBeta()
	return (a / (a + b)) * 100.0
}

// BayesianConfidence measures certainty: approaches 1.0 as observations grow.
func (e Entry) BayesianConfidence() float64 {
	a, b := e.effectiveAlphaBeta()
	n := a + b
	variance := (a * b) / (n * n * (n + 1))
	return 1.0 - math.Min(variance*400, 1.0) // scale so variance=0.0025 → confidence=0
}

func (e Entry) effectiveAlphaBeta() (float64, float64) {
	a := e.Alpha
	b := e.BetaParam
	if a < 1 {
		a = 1
	}
	if b < 1 {
		b = 1
	}
	return a, b
}

// Allowed returns the permission level this entry grants.
// Uses Bayesian score with a confidence gate: upgrades require confidence >= 0.6.
func (e Entry) Allowed() PermLevel {
	score := e.BayesianScore()
	conf := e.BayesianConfidence()

	if e.Alpha == 0 && e.BetaParam == 0 {
		score = float64(e.Score)
		conf = 1.0
	}

	if conf < 0.6 {
		if score >= 80 {
			return PermNetwork
		}
		if score >= 60 {
			return PermWrite
		}
	}

	switch {
	case score >= 80:
		return PermShell
	case score >= 60:
		return PermNetwork
	case score >= 30:
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
	t.migrateLegacyEntries()
	return t
}

// SetKVStore enables Ledger KV-backed persistence, replacing file I/O.
// Once set, all save/load operations go through Ledger KV.
func (t *Tracker) SetKVStore(kvs *iledger.KVConfigStore) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.kvs = kvs
	t.loadFromKV()
	t.migrateLegacyEntriesLocked()
}

// migrateLegacyEntries upgrades pre-Bayesian persistence entries
// (Alpha == BetaParam == 0 but Score > 0) so that the first RecordSuccess
// after upgrade does not regress score to ~50 via the (1,1) fallback.
//
// We seed Alpha/BetaParam with totalObs=20 virtual observations matching the
// stored Score. This preserves the legacy permission level while giving
// the Bayesian path a non-degenerate starting point. Persists exactly once.
func (t *Tracker) migrateLegacyEntries() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.migrateLegacyEntriesLocked()
}

// migrateLegacyEntriesLocked is the same as migrateLegacyEntries but
// assumes the caller already holds t.mu. Used from SetKVStore which loads
// under its own lock and must keep the migration atomic with the load.
func (t *Tracker) migrateLegacyEntriesLocked() {
	migrated := 0
	for _, e := range t.scores {
		if e.Alpha == 0 && e.BetaParam == 0 && e.Score > 0 {
			ratio := float64(e.Score) / 100.0
			const totalObs = 20.0
			e.Alpha = ratio*totalObs + 1
			e.BetaParam = (1-ratio)*totalObs + 1
			migrated++
		}
	}
	if migrated > 0 {
		slog.Info("trust: migrated legacy entries to Bayesian fields", "count", migrated)
		t.save()
	}
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

// NamedEntry pairs a skill slug with its trust Entry, for ranked listings.
type NamedEntry struct {
	Name  string
	Score int
}

// Top returns up to n skills sorted by score descending. n<=0 means all.
// Used by the runtime-grade context layer (#4) to tell the LLM which skills
// exist and how trusted each is, so it stops hallucinating tools.
func (t *Tracker) Top(n int) []NamedEntry {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make([]NamedEntry, 0, len(t.scores))
	for name, e := range t.scores {
		out = append(out, NamedEntry{Name: name, Score: e.Score})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Name < out[j].Name
	})
	if n > 0 && len(out) > n {
		out = out[:n]
	}
	return out
}

// Seed sets a skill's trust score directly without per-promotion logging.
// Used during startup to pre-seed built-in skills to a trusted level.
// Initializes Beta parameters to match the target score with high confidence.
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
	ratio := float64(score) / 100.0
	totalObs := 20.0
	e.Alpha = ratio*totalObs + 1
	e.BetaParam = (1-ratio)*totalObs + 1
	e.LastPromoted = time.Now()
	t.save()
}

// SeedMany pre-seeds several skills to the given score and persists ONCE at the
// end, instead of once per skill. Seed() rewrites the whole JSON file on every
// call, so seeding the ~34 built-in skills individually on first run meant ~34
// disk writes on the boot critical path (~0.2s). Returns the number actually
// seeded (skills already at/above the score are left untouched).
func (t *Tracker) SeedMany(slugs []string, score int) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	seeded := 0
	for _, slug := range slugs {
		e := t.getOrCreate(slug)
		if e.Score >= score {
			continue
		}
		e.Score = score
		if e.Score > 100 {
			e.Score = 100
		}
		ratio := float64(score) / 100.0
		totalObs := 20.0
		e.Alpha = ratio*totalObs + 1
		e.BetaParam = (1-ratio)*totalObs + 1
		e.LastPromoted = time.Now()
		seeded++
	}
	if seeded > 0 {
		t.save()
	}
	return seeded
}

// RecordSuccess increments trust after a successful, safe execution.
// Updates both legacy Score and Bayesian Alpha (success count).
func (t *Tracker) RecordSuccess(slug string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	e := t.getOrCreate(slug)
	e.Executions++
	e.Alpha++
	oldLevel := e.Allowed()
	e.Score = int(e.BayesianScore())
	if e.Allowed() > oldLevel {
		e.LastPromoted = time.Now()
		slog.Info("trust: promoted", "slug", slug, "level", e.Allowed().String(),
			"bayes_score", fmt.Sprintf("%.1f", e.BayesianScore()),
			"confidence", fmt.Sprintf("%.2f", e.BayesianConfidence()))
	}
	t.save()
}

// RecordFailure decreases trust after a dangerous or erroneous behavior.
// Updates Bayesian Beta (failure count) with severity weighting, capped at
// betaCapMultiplier×Alpha so a single bad observation cannot permanently
// suppress the score.
func (t *Tracker) RecordFailure(slug string, severity int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	e := t.getOrCreate(slug)
	e.Failures++
	newBeta := e.BetaParam + float64(severity)
	cap := math.Max(e.Alpha*betaCapMultiplier, e.BetaParam+1) + 1
	if newBeta > cap {
		newBeta = cap
	}
	e.BetaParam = newBeta
	e.Score = int(e.BayesianScore())
	slog.Warn("trust: penalized", "slug", slug, "severity", severity,
		"bayes_score", fmt.Sprintf("%.1f", e.BayesianScore()),
		"confidence", fmt.Sprintf("%.2f", e.BayesianConfidence()))
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
