package selfheal

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// LifecycleState represents the current state of a skill candidate.
type LifecycleState string

const (
	StateCandidate  LifecycleState = "candidate"   // generated, awaiting approval
	StatePromoted   LifecycleState = "promoted"    // approved and hot-loaded
	StateRejected   LifecycleState = "rejected"    // explicitly rejected
	StateRolledBack LifecycleState = "rolled_back" // was promoted, then rolled back
)

// Candidate wraps a generated plugin with lifecycle metadata.
type Candidate struct {
	ID               string          `json:"id"`
	Plugin           GeneratedPlugin `json:"plugin"`
	State            LifecycleState  `json:"state"`
	Reason           string          `json:"reason,omitempty"`        // why it was generated
	RejectReason     string          `json:"reject_reason,omitempty"` // why it was rejected
	CreatedAt        time.Time       `json:"created_at"`
	PromotedAt       *time.Time      `json:"promoted_at,omitempty"`
	RolledBackAt     *time.Time      `json:"rolled_back_at,omitempty"`
	ValidationErrors []string        `json:"validation_errors,omitempty"` // from ValidatePlugin
}

// Lifecycle manages the candidate → promote → rollback lifecycle for generated skills.
type Lifecycle struct {
	mu         sync.RWMutex
	healer     *Healer
	candidates map[string]*Candidate // id → candidate
	dataDir    string
	nextID     int
}

// NewLifecycle creates a lifecycle manager backed by the given healer.
func NewLifecycle(healer *Healer, dataDir string) *Lifecycle {
	if dataDir == "" {
		dataDir = "data"
	}
	lc := &Lifecycle{
		healer:     healer,
		candidates: make(map[string]*Candidate),
		dataDir:    dataDir,
		nextID:     1,
	}
	lc.load()
	return lc
}

// GenerateCandidate creates a new candidate skill via LLM but does NOT hot-load it.
// The candidate remains in "candidate" state until explicitly promoted or rejected.
func (lc *Lifecycle) GenerateCandidate(ctx context.Context, reason string) (*Candidate, error) {
	gp, err := lc.healer.Generate(ctx, reason)
	if err != nil {
		return nil, fmt.Errorf("generate candidate: %w", err)
	}

	// Run validation but don't block on it — just record warnings
	errs := lc.healer.ValidatePlugin(gp)
	var validationWarnings []string
	for _, ve := range errs {
		validationWarnings = append(validationWarnings, ve.Error())
	}

	lc.mu.Lock()
	id := fmt.Sprintf("candidate-%d", lc.nextID)
	lc.nextID++
	c := &Candidate{
		ID:               id,
		Plugin:           *gp,
		State:            StateCandidate,
		Reason:           reason,
		CreatedAt:        time.Now(),
		ValidationErrors: validationWarnings,
	}
	lc.candidates[id] = c
	lc.mu.Unlock()

	lc.save()
	slog.Info("lifecycle: candidate created", "id", id, "plugin", gp.Name, "skill", gp.SkillName, "warnings", len(validationWarnings))
	return c, nil
}

// Promote validates and hot-loads a candidate skill into the running agent.
func (lc *Lifecycle) Promote(ctx context.Context, candidateID string) error {
	lc.mu.Lock()
	c, ok := lc.candidates[candidateID]
	if !ok {
		lc.mu.Unlock()
		return fmt.Errorf("candidate %q not found", candidateID)
	}
	if c.State != StateCandidate {
		lc.mu.Unlock()
		return fmt.Errorf("candidate %q is in state %q, expected %q", candidateID, c.State, StateCandidate)
	}
	lc.mu.Unlock()

	// Hot-load (validates internally)
	if err := lc.healer.HotLoad(ctx, &c.Plugin); err != nil {
		return fmt.Errorf("promote %q: %w", candidateID, err)
	}

	lc.mu.Lock()
	now := time.Now()
	c.State = StatePromoted
	c.PromotedAt = &now
	lc.mu.Unlock()

	lc.save()
	slog.Info("lifecycle: candidate promoted", "id", candidateID, "plugin", c.Plugin.Name)
	return nil
}

// Reject marks a candidate as rejected without installing it.
func (lc *Lifecycle) Reject(candidateID, reason string) error {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	c, ok := lc.candidates[candidateID]
	if !ok {
		return fmt.Errorf("candidate %q not found", candidateID)
	}
	if c.State != StateCandidate {
		return fmt.Errorf("candidate %q is in state %q, expected %q", candidateID, c.State, StateCandidate)
	}

	c.State = StateRejected
	c.RejectReason = reason
	lc.save()
	slog.Info("lifecycle: candidate rejected", "id", candidateID, "reason", reason)
	return nil
}

// Rollback removes a previously promoted skill from the running agent.
func (lc *Lifecycle) Rollback(candidateID string) error {
	lc.mu.Lock()
	c, ok := lc.candidates[candidateID]
	if !ok {
		lc.mu.Unlock()
		return fmt.Errorf("candidate %q not found", candidateID)
	}
	if c.State != StatePromoted {
		lc.mu.Unlock()
		return fmt.Errorf("candidate %q is in state %q, expected %q", candidateID, c.State, StatePromoted)
	}
	lc.mu.Unlock()

	lc.healer.Rollback(c.Plugin.Name, c.Plugin.SkillName)

	lc.mu.Lock()
	now := time.Now()
	c.State = StateRolledBack
	c.RolledBackAt = &now
	lc.mu.Unlock()

	lc.save()
	slog.Info("lifecycle: candidate rolled back", "id", candidateID, "plugin", c.Plugin.Name)
	return nil
}

// List returns all candidates, optionally filtered by state.
func (lc *Lifecycle) List(filterState LifecycleState) []Candidate {
	lc.mu.RLock()
	defer lc.mu.RUnlock()
	var out []Candidate
	for _, c := range lc.candidates {
		if filterState == "" || c.State == filterState {
			out = append(out, *c)
		}
	}
	return out
}

// Get returns a specific candidate by ID.
func (lc *Lifecycle) Get(id string) (*Candidate, bool) {
	lc.mu.RLock()
	defer lc.mu.RUnlock()
	c, ok := lc.candidates[id]
	if !ok {
		return nil, false
	}
	cp := *c
	return &cp, true
}

// Count returns counts by state.
func (lc *Lifecycle) Count() map[LifecycleState]int {
	lc.mu.RLock()
	defer lc.mu.RUnlock()
	counts := map[LifecycleState]int{}
	for _, c := range lc.candidates {
		counts[c.State]++
	}
	return counts
}

// Cleanup removes all rejected and rolled-back candidates older than the given duration.
func (lc *Lifecycle) Cleanup(olderThan time.Duration) int {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	cutoff := time.Now().Add(-olderThan)
	removed := 0
	for id, c := range lc.candidates {
		if (c.State == StateRejected || c.State == StateRolledBack) && !c.CreatedAt.After(cutoff) {
			delete(lc.candidates, id)
			removed++
		}
	}
	if removed > 0 {
		lc.save()
	}
	return removed
}

// persistence

func (lc *Lifecycle) filePath() string {
	return filepath.Join(lc.dataDir, "skill_lifecycle.json")
}

func (lc *Lifecycle) save() {
	data, err := json.MarshalIndent(lc.candidates, "", "  ")
	if err != nil {
		slog.Warn("lifecycle: save failed", "err", err)
		return
	}
	os.MkdirAll(lc.dataDir, 0o755)
	os.WriteFile(lc.filePath(), data, 0o644)
}

func (lc *Lifecycle) load() {
	data, err := os.ReadFile(lc.filePath())
	if err != nil {
		return // no prior state
	}
	var candidates map[string]*Candidate
	if err := json.Unmarshal(data, &candidates); err != nil {
		slog.Warn("lifecycle: load failed", "err", err)
		return
	}
	lc.candidates = candidates
	// Update nextID
	for _, c := range candidates {
		var n int
		fmt.Sscanf(c.ID, "candidate-%d", &n)
		if n >= lc.nextID {
			lc.nextID = n + 1
		}
	}
}
