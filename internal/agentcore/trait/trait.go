package trait

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ──────────────────────────────────────────────
// Trait — a learned user preference
// ──────────────────────────────────────────────

// Trait represents a single dimension-preference pair mined from conversation.
type Trait struct {
	ID         string    `json:"id"`
	Dimension  string    `json:"dimension"`  // e.g. "communication_style", "domain_preference"
	Preference string    `json:"preference"` // e.g. "concise", "Go programming"
	Confidence float64   `json:"confidence"` // 0.0 - 1.0
	Source     string    `json:"source,omitempty"` // message that triggered mining
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	HitCount   int       `json:"hit_count"` // reinforcement counter
}

// ──────────────────────────────────────────────
// Predefined dimensions
// ──────────────────────────────────────────────

const (
	DimCommunicationStyle string = "communication_style"
	DimDomainPreference   string = "domain_preference"
	DimInteractionPattern string = "interaction_pattern"
	DimContentInterest    string = "content_interest"
	DimLanguagePreference string = "language_preference"
	DimTonePreference     string = "tone_preference"
	DimExpertiseLevel     string = "expertise_level"
	DimWorkSchedule       string = "work_schedule"
)

// ──────────────────────────────────────────────
// MineFunc — LLM-powered trait extraction
// ──────────────────────────────────────────────

// MineResult is a single extracted trait from a message.
type MineResult struct {
	Dimension  string  `json:"dimension"`
	Preference string  `json:"preference"`
	Confidence float64 `json:"confidence"`
}

// MineFunc extracts traits from a user message using LLM.
type MineFunc func(ctx context.Context, message string) ([]MineResult, error)

// ──────────────────────────────────────────────
// Store — persists and queries traits
// ──────────────────────────────────────────────

// Store manages learned user traits.
type Store struct {
	mu      sync.RWMutex
	traits  map[string]*Trait // key: dimension:preference
	dataDir string
}

// NewStore creates a trait store.
func NewStore(dataDir string) *Store {
	s := &Store{
		traits:  make(map[string]*Trait),
		dataDir: dataDir,
	}
	s.load()
	return s
}

func traitKey(dimension, preference string) string {
	return dimension + ":" + preference
}

// Add adds or reinforces a trait.
func (s *Store) Add(dimension, preference string, confidence float64, source string) *Trait {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := traitKey(dimension, preference)
	if existing, ok := s.traits[key]; ok {
		// Reinforce existing trait
		existing.HitCount++
		existing.UpdatedAt = time.Now()
		// Increase confidence with diminishing returns
		existing.Confidence = min64(1.0, existing.Confidence+(1.0-existing.Confidence)*0.1)
		s.persist()
		return existing
	}

	t := &Trait{
		ID:         uuid.New().String(),
		Dimension:  dimension,
		Preference: preference,
		Confidence: confidence,
		Source:     source,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		HitCount:   1,
	}
	s.traits[key] = t
	s.persist()
	return t
}

// Get returns a trait by dimension and preference.
func (s *Store) Get(dimension, preference string) (*Trait, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.traits[traitKey(dimension, preference)]
	if !ok {
		return nil, false
	}
	cp := *t
	return &cp, true
}

// ByDimension returns all traits for a dimension.
func (s *Store) ByDimension(dimension string) []Trait {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Trait
	for _, t := range s.traits {
		if t.Dimension == dimension {
			out = append(out, *t)
		}
	}
	return out
}

// All returns all traits.
func (s *Store) All() []Trait {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Trait, 0, len(s.traits))
	for _, t := range s.traits {
		out = append(out, *t)
	}
	return out
}

// TopTraits returns the N highest-confidence traits.
func (s *Store) TopTraits(n int) []Trait {
	all := s.All()
	// Simple sort by confidence desc
	for i := 0; i < len(all); i++ {
		for j := i + 1; j < len(all); j++ {
			if all[j].Confidence > all[i].Confidence {
				all[i], all[j] = all[j], all[i]
			}
		}
	}
	if n > len(all) {
		n = len(all)
	}
	return all[:n]
}

// Remove deletes a trait.
func (s *Store) Remove(dimension, preference string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.traits, traitKey(dimension, preference))
	s.persist()
}

// ForPersonaPrompt generates a persona instruction snippet from stored traits.
func (s *Store) ForPersonaPrompt(maxTraits int) string {
	top := s.TopTraits(maxTraits)
	if len(top) == 0 {
		return ""
	}
	result := "Based on learned user preferences:\n"
	for _, t := range top {
		result += fmt.Sprintf("- %s: %s (confidence: %.0f%%)\n", t.Dimension, t.Preference, t.Confidence*100)
	}
	return result
}

// ──────────────────────────────────────────────
// Miner — async trait extraction pipeline
// ──────────────────────────────────────────────

// Miner processes messages to extract traits.
type Miner struct {
	store   *Store
	mineFn  MineFunc
	minConf float64 // minimum confidence to store (default 0.3)
}

// NewMiner creates a trait miner.
func NewMiner(store *Store, fn MineFunc) *Miner {
	return &Miner{
		store:   store,
		mineFn:  fn,
		minConf: 0.3,
	}
}

// SetMinConfidence sets the minimum confidence threshold.
func (m *Miner) SetMinConfidence(c float64) { m.minConf = c }

// Mine extracts traits from a message and stores them.
func (m *Miner) Mine(ctx context.Context, message string) ([]Trait, error) {
	if m.mineFn == nil {
		return nil, fmt.Errorf("trait: no mine function set")
	}

	results, err := m.mineFn(ctx, message)
	if err != nil {
		return nil, fmt.Errorf("trait: mining failed: %w", err)
	}

	var stored []Trait
	for _, r := range results {
		if r.Confidence < m.minConf {
			continue
		}
		t := m.store.Add(r.Dimension, r.Preference, r.Confidence, message)
		stored = append(stored, *t)
		slog.Debug("trait: mined", "dim", r.Dimension, "pref", r.Preference, "conf", r.Confidence)
	}
	return stored, nil
}

// ──────────────────────────────────────────────
// Persistence
// ──────────────────────────────────────────────

func (s *Store) load() {
	path := filepath.Join(s.dataDir, "traits.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var traits []Trait
	if err := json.Unmarshal(data, &traits); err != nil {
		return
	}
	for i := range traits {
		t := traits[i]
		s.traits[traitKey(t.Dimension, t.Preference)] = &t
	}
}

func (s *Store) persist() {
	os.MkdirAll(s.dataDir, 0o755)
	path := filepath.Join(s.dataDir, "traits.json")
	traits := make([]Trait, 0, len(s.traits))
	for _, t := range s.traits {
		traits = append(traits, *t)
	}
	data, _ := json.MarshalIndent(traits, "", "  ")
	os.WriteFile(path, data, 0o644)
}

func min64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
