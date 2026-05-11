package cogni

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ToolExperience records a tool's performance in a specific context.
type ToolExperience struct {
	Tool        string    `json:"tool" yaml:"tool"`
	Context     string    `json:"context" yaml:"context"`
	Result      string    `json:"result" yaml:"result"`
	Learned     string    `json:"learned" yaml:"learned"`
	Confidence  float64   `json:"confidence" yaml:"confidence"`
	VerifiedBy  string    `json:"verified_by,omitempty" yaml:"verified_by,omitempty"`
	UsedCount   int       `json:"used_count" yaml:"used_count"`
	SuccessRate float64   `json:"success_rate" yaml:"success_rate"`
	CreatedAt   time.Time `json:"created_at" yaml:"created_at"`
	LastUsed    time.Time `json:"last_used" yaml:"last_used"`
}

// BehaviorPattern represents a discovered recurring behavior.
type BehaviorPattern struct {
	ID          string    `json:"id" yaml:"id"`
	Trigger     string    `json:"trigger" yaml:"trigger"`
	Response    string    `json:"response" yaml:"response"`
	Confirmed   bool      `json:"confirmed" yaml:"confirmed"`
	SuccessRate float64   `json:"success_rate" yaml:"success_rate"`
	UsedCount   int       `json:"used_count" yaml:"used_count"`
	CreatedAt   time.Time `json:"created_at" yaml:"created_at"`
	LastUsed    time.Time `json:"last_used" yaml:"last_used"`
}

// DomainFact is a piece of domain knowledge extracted from conversations.
type DomainFact struct {
	Fact      string    `json:"fact" yaml:"fact"`
	Source    string    `json:"source" yaml:"source"`
	UsedCount int       `json:"used_count" yaml:"used_count"`
	CreatedAt time.Time `json:"created_at" yaml:"created_at"`
	LastUsed  time.Time `json:"last_used" yaml:"last_used"`
}

// ExperienceSummary is a compact, UI-friendly view of a Cogni's reusable
// experience profile. It keeps the detailed stores available through their
// existing accessors while giving dashboards a small ranked projection.
type ExperienceSummary struct {
	Stats           map[string]int    `json:"stats"`
	TopTools        []ToolExperience  `json:"top_tools"`
	TopFacts        []DomainFact      `json:"top_facts"`
	PendingPatterns []BehaviorPattern `json:"pending_patterns"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

// ExperienceConfig controls experience behavior.
type ExperienceConfig struct {
	Enabled       bool    `json:"enabled" yaml:"enabled"`
	StoreDir      string  `json:"store" yaml:"store"`
	MinConfidence float64 `json:"min_confidence,omitempty" yaml:"min_confidence,omitempty"`
	HalfLifeDays  int     `json:"half_life_days,omitempty" yaml:"half_life_days,omitempty"` // default 90
	MaxFacts      int     `json:"max_facts,omitempty" yaml:"max_facts,omitempty"`           // default 500
	AutoRecord    bool    `json:"auto_record,omitempty" yaml:"auto_record,omitempty"`
	RequireReview bool    `json:"require_review,omitempty" yaml:"require_review,omitempty"`
}

// ExperienceStore manages a Cogni's accumulated experience with persistence,
// decay, and injection capabilities.
type ExperienceStore struct {
	mu sync.RWMutex

	cogniID string
	cfg     ExperienceConfig
	dataDir string

	toolMemory []ToolExperience
	patterns   []BehaviorPattern
	facts      []DomainFact
}

func NewExperienceStore(cogniID string, cfg ExperienceConfig) *ExperienceStore {
	if cfg.HalfLifeDays <= 0 {
		cfg.HalfLifeDays = 90
	}
	if cfg.MaxFacts <= 0 {
		cfg.MaxFacts = 500
	}
	if cfg.MinConfidence <= 0 {
		cfg.MinConfidence = 0.7
	}

	dataDir := cfg.StoreDir
	if dataDir == "" {
		dataDir = filepath.Join(".experience", cogniID)
	}

	es := &ExperienceStore{
		cogniID: cogniID,
		cfg:     cfg,
		dataDir: dataDir,
	}
	es.load()
	return es
}

// ── Tool Memory ──

func (es *ExperienceStore) AddToolMemory(exp ToolExperience) {
	es.mu.Lock()
	defer es.mu.Unlock()

	if exp.CreatedAt.IsZero() {
		exp.CreatedAt = time.Now()
	}
	exp.LastUsed = time.Now()
	if exp.UsedCount <= 0 {
		exp.UsedCount = 1
	}

	for i := range es.toolMemory {
		existing := &es.toolMemory[i]
		if existing.Tool == exp.Tool && existing.Context == exp.Context && existing.Learned == exp.Learned {
			existing.UsedCount += exp.UsedCount
			existing.LastUsed = exp.LastUsed
			if exp.Confidence > existing.Confidence {
				existing.Confidence = exp.Confidence
			}
			if exp.SuccessRate > 0 {
				existing.SuccessRate = exp.SuccessRate
			}
			if exp.Result != "" {
				existing.Result = exp.Result
			}
			if exp.VerifiedBy != "" {
				existing.VerifiedBy = exp.VerifiedBy
			}
			es.persist()
			return
		}
	}

	es.toolMemory = append(es.toolMemory, exp)
	es.persist()
}

func (es *ExperienceStore) ToolMemory(tool string) []ToolExperience {
	es.mu.RLock()
	defer es.mu.RUnlock()

	var result []ToolExperience
	for _, tm := range es.toolMemory {
		if tool == "" || tm.Tool == tool {
			result = append(result, tm)
		}
	}
	return result
}

// ── Behavior Patterns ──

func (es *ExperienceStore) SuggestPattern(pattern BehaviorPattern) {
	es.mu.Lock()
	defer es.mu.Unlock()

	if pattern.ID == "" {
		pattern.ID = fmt.Sprintf("pat-%d", time.Now().UnixMilli())
	}
	if pattern.CreatedAt.IsZero() {
		pattern.CreatedAt = time.Now()
	}
	pattern.Confirmed = !es.cfg.RequireReview

	es.patterns = append(es.patterns, pattern)
	es.persist()

	slog.Info("experience: pattern suggested",
		"cogni", es.cogniID,
		"id", pattern.ID,
		"confirmed", pattern.Confirmed,
	)
}

func (es *ExperienceStore) ConfirmPattern(id string) bool {
	es.mu.Lock()
	defer es.mu.Unlock()

	for i := range es.patterns {
		if es.patterns[i].ID == id {
			es.patterns[i].Confirmed = true
			es.patterns[i].LastUsed = time.Now()
			es.persist()
			return true
		}
	}
	return false
}

func (es *ExperienceStore) Patterns() []BehaviorPattern {
	es.mu.RLock()
	defer es.mu.RUnlock()

	result := make([]BehaviorPattern, len(es.patterns))
	copy(result, es.patterns)
	return result
}

func (es *ExperienceStore) ConfirmedPatterns() []BehaviorPattern {
	es.mu.RLock()
	defer es.mu.RUnlock()

	var result []BehaviorPattern
	for _, p := range es.patterns {
		if p.Confirmed {
			result = append(result, p)
		}
	}
	return result
}

// ── Domain Facts ──

func (es *ExperienceStore) AddFact(fact DomainFact) {
	es.mu.Lock()
	defer es.mu.Unlock()

	if fact.CreatedAt.IsZero() {
		fact.CreatedAt = time.Now()
	}
	fact.LastUsed = time.Now()
	if fact.UsedCount <= 0 {
		fact.UsedCount = 1
	}

	for i := range es.facts {
		existing := &es.facts[i]
		if existing.Fact == fact.Fact && existing.Source == fact.Source {
			existing.UsedCount += fact.UsedCount
			existing.LastUsed = fact.LastUsed
			es.persist()
			return
		}
	}

	// Enforce max facts
	if len(es.facts) >= es.cfg.MaxFacts {
		sort.Slice(es.facts, func(i, j int) bool {
			return es.facts[i].UsedCount < es.facts[j].UsedCount
		})
		es.facts = es.facts[1:]
	}

	es.facts = append(es.facts, fact)
	es.persist()
}

func (es *ExperienceStore) DomainFacts() []DomainFact {
	es.mu.RLock()
	defer es.mu.RUnlock()

	result := make([]DomainFact, len(es.facts))
	copy(result, es.facts)
	return result
}

// Summary returns a ranked, compact profile for review surfaces and SDKs.
func (es *ExperienceStore) Summary(limit int) ExperienceSummary {
	es.mu.RLock()
	defer es.mu.RUnlock()

	if limit <= 0 {
		limit = 5
	}

	topTools := make([]ToolExperience, len(es.toolMemory))
	copy(topTools, es.toolMemory)
	sort.SliceStable(topTools, func(i, j int) bool {
		return experienceMoreRecentlyUseful(
			topTools[i].UsedCount, topTools[i].LastUsed, topTools[i].CreatedAt,
			topTools[j].UsedCount, topTools[j].LastUsed, topTools[j].CreatedAt,
		)
	})
	topTools = limitToolExperience(topTools, limit)

	topFacts := make([]DomainFact, len(es.facts))
	copy(topFacts, es.facts)
	sort.SliceStable(topFacts, func(i, j int) bool {
		return experienceMoreRecentlyUseful(
			topFacts[i].UsedCount, topFacts[i].LastUsed, topFacts[i].CreatedAt,
			topFacts[j].UsedCount, topFacts[j].LastUsed, topFacts[j].CreatedAt,
		)
	})
	topFacts = limitDomainFacts(topFacts, limit)

	var pending []BehaviorPattern
	for _, pattern := range es.patterns {
		if !pattern.Confirmed {
			pending = append(pending, pattern)
		}
	}
	sort.SliceStable(pending, func(i, j int) bool {
		return experienceMoreRecentlyUseful(
			pending[i].UsedCount, pending[i].LastUsed, pending[i].CreatedAt,
			pending[j].UsedCount, pending[j].LastUsed, pending[j].CreatedAt,
		)
	})
	pending = limitBehaviorPatterns(pending, limit)

	return ExperienceSummary{
		Stats:           statsForExperience(es.toolMemory, es.patterns, es.facts),
		TopTools:        topTools,
		TopFacts:        topFacts,
		PendingPatterns: pending,
		UpdatedAt:       latestExperienceTime(es.toolMemory, es.patterns, es.facts),
	}
}

// ── Experience Injection ──

// ContextHints generates a text block to inject into the system prompt
// based on relevant accumulated experience. The message is used to find
// the most relevant experience entries.
func (es *ExperienceStore) ContextHints(_ context.Context, message string) string {
	es.mu.Lock()
	defer es.mu.Unlock()

	var parts []string
	now := time.Now()
	used := false

	// Relevant tool experiences
	for i := range es.toolMemory {
		tm := &es.toolMemory[i]
		if tm.Learned == "" || tm.Confidence < es.cfg.MinConfidence {
			continue
		}
		weight := es.decayWeight(tm.LastUsed, now)
		if weight < 0.1 {
			continue
		}
		if strings.Contains(strings.ToLower(message), strings.ToLower(tm.Tool)) ||
			strings.Contains(strings.ToLower(message), strings.ToLower(tm.Context)) {
			tm.UsedCount++
			tm.LastUsed = now
			used = true
			parts = append(parts, fmt.Sprintf("- [工具经验] %s: %s (置信度: %.0f%%)",
				tm.Tool, tm.Learned, tm.Confidence*100))
		}
	}

	// Confirmed behavior patterns
	for i := range es.patterns {
		p := &es.patterns[i]
		if !p.Confirmed {
			continue
		}
		weight := es.decayWeight(p.LastUsed, now)
		if weight < 0.1 {
			continue
		}
		if strings.Contains(strings.ToLower(message), strings.ToLower(p.Trigger)) {
			p.UsedCount++
			p.LastUsed = now
			used = true
			parts = append(parts, fmt.Sprintf("- [行为模式] 当 \"%s\" → %s (成功率: %.0f%%)",
				p.Trigger, p.Response, p.SuccessRate*100))
		}
	}

	// Domain facts (top 5 most relevant)
	factCount := 0
	for i := range es.facts {
		f := &es.facts[i]
		if factCount >= 5 {
			break
		}
		weight := es.decayWeight(f.LastUsed, now)
		if weight < 0.1 {
			continue
		}
		if strings.Contains(strings.ToLower(message), strings.ToLower(prefixRunes(f.Fact, 20))) {
			f.UsedCount++
			f.LastUsed = now
			used = true
			parts = append(parts, fmt.Sprintf("- [领域知识] %s", f.Fact))
			factCount++
		}
	}

	if len(parts) == 0 {
		return ""
	}
	if used {
		es.persist()
	}

	return "## 相关经验\n" + strings.Join(parts, "\n")
}

// ── Decay ──

func (es *ExperienceStore) decayWeight(lastUsed, now time.Time) float64 {
	if lastUsed.IsZero() {
		return 0.5
	}
	daysSince := now.Sub(lastUsed).Hours() / 24.0
	halfLife := float64(es.cfg.HalfLifeDays)
	return math.Pow(0.5, daysSince/halfLife)
}

// RunDecay removes experiences whose weight has dropped below threshold.
func (es *ExperienceStore) RunDecay() int {
	es.mu.Lock()
	defer es.mu.Unlock()

	now := time.Now()
	threshold := 0.05
	removed := 0

	var filteredTM []ToolExperience
	for _, tm := range es.toolMemory {
		if es.decayWeight(tm.LastUsed, now) >= threshold {
			filteredTM = append(filteredTM, tm)
		} else {
			removed++
		}
	}
	es.toolMemory = filteredTM

	var filteredFacts []DomainFact
	for _, f := range es.facts {
		if es.decayWeight(f.LastUsed, now) >= threshold {
			filteredFacts = append(filteredFacts, f)
		} else {
			removed++
		}
	}
	es.facts = filteredFacts

	if removed > 0 {
		es.persist()
		slog.Info("experience: decay removed entries",
			"cogni", es.cogniID,
			"removed", removed,
		)
	}
	return removed
}

// ── Export / Import ──

type experienceBundle struct {
	CogniID    string            `json:"cogni_id"`
	ToolMemory []ToolExperience  `json:"tool_memory"`
	Patterns   []BehaviorPattern `json:"patterns"`
	Facts      []DomainFact      `json:"facts"`
	ExportedAt time.Time         `json:"exported_at"`
}

func (es *ExperienceStore) Export() ([]byte, error) {
	es.mu.RLock()
	defer es.mu.RUnlock()

	bundle := experienceBundle{
		CogniID:    es.cogniID,
		ToolMemory: es.toolMemory,
		Patterns:   es.patterns,
		Facts:      es.facts,
		ExportedAt: time.Now(),
	}
	return json.MarshalIndent(bundle, "", "  ")
}

func (es *ExperienceStore) Import(data []byte) error {
	var bundle experienceBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return fmt.Errorf("experience: import parse: %w", err)
	}

	es.mu.Lock()
	defer es.mu.Unlock()

	es.toolMemory = append(es.toolMemory, bundle.ToolMemory...)
	es.patterns = append(es.patterns, bundle.Patterns...)
	es.facts = append(es.facts, bundle.Facts...)

	es.persist()
	slog.Info("experience: imported",
		"cogni", es.cogniID,
		"tools", len(bundle.ToolMemory),
		"patterns", len(bundle.Patterns),
		"facts", len(bundle.Facts),
	)
	return nil
}

// ── Stats ──

func (es *ExperienceStore) Stats() map[string]int {
	es.mu.RLock()
	defer es.mu.RUnlock()
	return statsForExperience(es.toolMemory, es.patterns, es.facts)
}

func statsForExperience(toolMemory []ToolExperience, patterns []BehaviorPattern, facts []DomainFact) map[string]int {
	return map[string]int{
		"tool_memories":      len(toolMemory),
		"patterns_total":     len(patterns),
		"patterns_confirmed": countConfirmed(patterns),
		"patterns_pending":   len(patterns) - countConfirmed(patterns),
		"domain_facts":       len(facts),
	}
}

func countConfirmed(patterns []BehaviorPattern) int {
	n := 0
	for _, p := range patterns {
		if p.Confirmed {
			n++
		}
	}
	return n
}

// ── Persistence ──

func (es *ExperienceStore) persist() {
	if es.dataDir == "" {
		return
	}
	os.MkdirAll(es.dataDir, 0755)

	writeJSON(filepath.Join(es.dataDir, "tool_memory.json"), es.toolMemory)
	writeJSON(filepath.Join(es.dataDir, "patterns.json"), es.patterns)
	writeJSON(filepath.Join(es.dataDir, "domain_facts.json"), es.facts)
}

func (es *ExperienceStore) load() {
	if es.dataDir == "" {
		return
	}
	readJSON(filepath.Join(es.dataDir, "tool_memory.json"), &es.toolMemory)
	readJSON(filepath.Join(es.dataDir, "patterns.json"), &es.patterns)
	readJSON(filepath.Join(es.dataDir, "domain_facts.json"), &es.facts)
}

func writeJSON(path string, v any) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(path, data, 0644)
}

func readJSON(path string, v any) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	json.Unmarshal(data, v)
}

func prefixRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}

func experienceMoreRecentlyUseful(leftCount int, leftLastUsed, leftCreatedAt time.Time, rightCount int, rightLastUsed, rightCreatedAt time.Time) bool {
	if leftCount != rightCount {
		return leftCount > rightCount
	}
	leftSeen := leftLastUsed
	if leftSeen.IsZero() {
		leftSeen = leftCreatedAt
	}
	rightSeen := rightLastUsed
	if rightSeen.IsZero() {
		rightSeen = rightCreatedAt
	}
	return leftSeen.After(rightSeen)
}

func latestExperienceTime(tools []ToolExperience, patterns []BehaviorPattern, facts []DomainFact) time.Time {
	var latest time.Time
	consider := func(ts time.Time) {
		if ts.After(latest) {
			latest = ts
		}
	}
	for _, tool := range tools {
		consider(tool.CreatedAt)
		consider(tool.LastUsed)
	}
	for _, pattern := range patterns {
		consider(pattern.CreatedAt)
		consider(pattern.LastUsed)
	}
	for _, fact := range facts {
		consider(fact.CreatedAt)
		consider(fact.LastUsed)
	}
	return latest
}

func limitToolExperience(items []ToolExperience, limit int) []ToolExperience {
	if len(items) <= limit {
		return items
	}
	return items[:limit]
}

func limitDomainFacts(items []DomainFact, limit int) []DomainFact {
	if len(items) <= limit {
		return items
	}
	return items[:limit]
}

func limitBehaviorPatterns(items []BehaviorPattern, limit int) []BehaviorPattern {
	if len(items) <= limit {
		return items
	}
	return items[:limit]
}
