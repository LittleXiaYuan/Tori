package world

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/LittleXiaYuan/ledger"
)

// Model tracks the agent's understanding of the external environment.
type Model struct {
	ldg      *ledger.Ledger
	mu       sync.RWMutex
	state    map[string]*State
	tenantID string
}

// State represents the known state of an external entity.
type State struct {
	Key          string    `json:"key"`
	Kind         StateKind `json:"kind"`
	Value        string    `json:"value"`
	Confidence   float64   `json:"confidence"`
	LastVerified time.Time `json:"last_verified"`
	UpdatedBy    string    `json:"updated_by"`
	Dependencies []string  `json:"dependencies"`
}

// StateKind classifies the type of world state being tracked.
type StateKind string

const (
	KindFile     StateKind = "file"
	KindDatabase StateKind = "database"
	KindAPI      StateKind = "api"
	KindConfig   StateKind = "config"
	KindUser     StateKind = "user"
	KindProcess  StateKind = "process"
	KindCustom   StateKind = "custom"
)

// ImpactPrediction predicts the effect of an action on the world model.
type ImpactPrediction struct {
	Action       string       `json:"action"`
	AffectedKeys []string     `json:"affected_keys"`
	Predictions  []Prediction `json:"predictions"`
	RiskLevel    string       `json:"risk_level"`
}

// Prediction describes a single predicted state change.
type Prediction struct {
	Key            string  `json:"key"`
	CurrentValue   string  `json:"current_value"`
	PredictedValue string  `json:"predicted_value"`
	Confidence     float64 `json:"confidence"`
}

// NewModel creates a world model tracker.
func NewModel(ldg *ledger.Ledger, tenantID string) *Model {
	return &Model{
		ldg:      ldg,
		state:    make(map[string]*State),
		tenantID: tenantID,
	}
}

// Update records a state change.
func (wm *Model) Update(key string, kind StateKind, value, updatedBy string, confidence float64) {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	wm.state[key] = &State{
		Key: key, Kind: kind, Value: value,
		Confidence: confidence, LastVerified: time.Now(), UpdatedBy: updatedBy,
	}
}

// Get returns the known state for a key.
func (wm *Model) Get(key string) (*State, bool) {
	wm.mu.RLock()
	defer wm.mu.RUnlock()
	s, ok := wm.state[key]
	return s, ok
}

// GetByKind returns all states of a given kind.
func (wm *Model) GetByKind(kind StateKind) []*State {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	var result []*State
	for _, s := range wm.state {
		if s.Kind == kind {
			result = append(result, s)
		}
	}
	return result
}

// Snapshot returns a copy of the entire world state.
func (wm *Model) Snapshot() map[string]*State {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	snap := make(map[string]*State, len(wm.state))
	for k, v := range wm.state {
		cp := *v
		snap[k] = &cp
	}
	return snap
}

// PredictImpact predicts what would change if a given action is executed.
func (wm *Model) PredictImpact(action string, targetKeys []string) *ImpactPrediction {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	pred := &ImpactPrediction{
		Action:       action,
		AffectedKeys: targetKeys,
		RiskLevel:    "low",
	}

	for _, key := range targetKeys {
		if s, ok := wm.state[key]; ok {
			pred.Predictions = append(pred.Predictions, Prediction{
				Key: key, CurrentValue: s.Value,
				PredictedValue: "(modified by " + action + ")",
				Confidence:     0.5,
			})

			for _, dep := range s.Dependencies {
				if _, exists := wm.state[dep]; exists {
					pred.AffectedKeys = append(pred.AffectedKeys, dep)
					pred.RiskLevel = "medium"
				}
			}
		}
	}

	if len(pred.AffectedKeys) > 5 {
		pred.RiskLevel = "high"
	}

	return pred
}

// Persist saves the world model to memory.
func (wm *Model) Persist(ctx context.Context) error {
	wm.mu.RLock()
	snap := make(map[string]*State, len(wm.state))
	for k, v := range wm.state {
		snap[k] = v
	}
	wm.mu.RUnlock()

	data, _ := json.Marshal(snap)
	return wm.ldg.Memory.Put(ctx, &ledger.MemoryEntry{
		TenantID:   wm.tenantID,
		Kind:       ledger.MemoryFact,
		Key:        "world_model.state",
		Content:    string(data),
		Source:     "world_model",
		Confidence: 0.8,
	})
}

// Load restores the world model from memory.
func (wm *Model) Load(ctx context.Context) error {
	results, err := wm.ldg.Memory.Search(ctx, ledger.MemoryQuery{
		TenantID: wm.tenantID,
		Kinds:    []ledger.MemoryKind{ledger.MemoryFact},
		Limit:    100,
	})
	if err != nil {
		return err
	}

	for _, m := range results {
		if m.Key == "world_model.state" {
			var snap map[string]*State
			if json.Unmarshal([]byte(m.Content), &snap) == nil {
				wm.mu.Lock()
				wm.state = snap
				wm.mu.Unlock()
				return nil
			}
		}
	}
	return nil
}

// StaleKeys returns state entries that haven't been verified within the given duration.
func (wm *Model) StaleKeys(maxAge time.Duration) []string {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	cutoff := time.Now().Add(-maxAge)
	var stale []string
	for k, s := range wm.state {
		if s.LastVerified.Before(cutoff) {
			stale = append(stale, k)
		}
	}
	return stale
}

// Size returns the number of tracked state entries.
func (wm *Model) Size() int {
	wm.mu.RLock()
	defer wm.mu.RUnlock()
	return len(wm.state)
}
