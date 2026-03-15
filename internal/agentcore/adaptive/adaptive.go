package adaptive

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ──────────────────────────────────────────────
// Adaptive Behavior Loop — "越用越懂你"
// Connects: TraitMining → Persona Adjustment → Proactive Behavior → Feedback Learning
// This is Tori's core differentiator: a closed-loop self-adapting system.
// ──────────────────────────────────────────────

// ── Feedback types ──

// FeedbackType classifies user feedback signals.
type FeedbackType string

const (
	FeedbackExplicit   FeedbackType = "explicit"   // user says "don't do X"
	FeedbackCorrection FeedbackType = "correction" // user corrects agent output
	FeedbackPreference FeedbackType = "preference" // user states a preference
	FeedbackPositive   FeedbackType = "positive"   // user approves / thanks
	FeedbackNegative   FeedbackType = "negative"   // user shows dissatisfaction
	FeedbackIgnore     FeedbackType = "ignore"     // user ignores agent suggestion
)

// Feedback captures a single user feedback signal.
type Feedback struct {
	ID          string       `json:"id"`
	Type        FeedbackType `json:"type"`
	UserMessage string       `json:"user_message"` // what the user said
	AgentAction string       `json:"agent_action"` // what the agent did
	Correction  string       `json:"correction"`   // what the user wanted instead
	Dimension   string       `json:"dimension"`    // trait dimension affected
	CreatedAt   time.Time    `json:"created_at"`
}

// ── Adaptation rules ──

// AdaptationRule defines how to adjust behavior based on accumulated feedback.
type AdaptationRule struct {
	ID          string  `json:"id"`
	Dimension   string  `json:"dimension"`    // e.g. "response_length", "formality"
	CurrentVal  string  `json:"current_val"`  // current behavior setting
	TargetVal   string  `json:"target_val"`   // desired behavior after adaptation
	Confidence  float64 `json:"confidence"`   // how sure we are (0-1)
	FeedbackCnt int     `json:"feedback_cnt"` // number of feedbacks driving this
	Active      bool    `json:"active"`
}

// ── Behavioral dimensions ──

const (
	DimResponseLength  = "response_length"   // concise vs verbose
	DimFormality       = "formality"         // casual vs formal
	DimProactivity     = "proactivity"       // reactive vs proactive
	DimCodeStyle       = "code_style"        // comments, naming conventions
	DimExplanationDepth = "explanation_depth" // brief vs detailed
	DimLanguage        = "language"          // zh vs en
	DimEmoji           = "emoji_usage"       // with/without emojis
	DimTechnicalLevel  = "technical_level"   // beginner vs expert
)

// ── Extract function ──

// ExtractFunc extracts feedback signals from a user message + agent context.
type ExtractFunc func(ctx context.Context, userMsg, agentAction string) (*Feedback, error)

// AdaptFunc generates adaptation rules from accumulated feedbacks.
type AdaptFunc func(ctx context.Context, feedbacks []Feedback) ([]AdaptationRule, error)

// ── Correction tracker ──

// CorrectionPattern tracks repeated user corrections in a specific dimension.
type CorrectionPattern struct {
	Dimension   string    `json:"dimension"`
	Pattern     string    `json:"pattern"`     // what the user keeps correcting
	Occurrences int       `json:"occurrences"` // how many times
	LastSeen    time.Time `json:"last_seen"`
	Examples    []string  `json:"examples"`    // recent examples (max 5)
}

// ── Behavior profile ──

// BehaviorProfile is the current adapted behavior settings.
type BehaviorProfile struct {
	Settings  map[string]string `json:"settings"`  // dimension -> value
	UpdatedAt time.Time         `json:"updated_at"`
	Version   int               `json:"version"`
}

// Compile generates a system prompt snippet from the profile.
func (bp *BehaviorProfile) Compile() string {
	if len(bp.Settings) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("<adaptive_behavior>\n")
	for dim, val := range bp.Settings {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", dim, val))
	}
	sb.WriteString("</adaptive_behavior>\n")
	return sb.String()
}

// ──────────────────────────────────────────────
// Loop — the main adaptive engine
// ──────────────────────────────────────────────

// Loop is the central adaptive behavior engine.
type Loop struct {
	mu             sync.RWMutex
	feedbacks      []Feedback
	corrections    map[string]*CorrectionPattern // dimension -> pattern
	rules          map[string]*AdaptationRule    // dimension -> rule
	profile        BehaviorProfile
	extractFn      ExtractFunc
	adaptFn        AdaptFunc
	maxFeedbacks   int
	adaptThreshold int // minimum feedbacks before adapting
}

// NewLoop creates a new adaptive behavior loop.
func NewLoop() *Loop {
	return &Loop{
		corrections:    make(map[string]*CorrectionPattern),
		rules:          make(map[string]*AdaptationRule),
		profile:        BehaviorProfile{Settings: make(map[string]string)},
		maxFeedbacks:   500,
		adaptThreshold: 3,
	}
}

// SetExtractFunc sets the feedback extraction function (typically LLM-powered).
func (l *Loop) SetExtractFunc(fn ExtractFunc) { l.extractFn = fn }

// SetAdaptFunc sets the adaptation rule generation function.
func (l *Loop) SetAdaptFunc(fn AdaptFunc) { l.adaptFn = fn }

// SetAdaptThreshold sets minimum feedback count before triggering adaptation.
func (l *Loop) SetAdaptThreshold(n int) { l.adaptThreshold = n }

// ── Record feedback ──

// RecordFeedback manually records a user feedback signal.
func (l *Loop) RecordFeedback(fb Feedback) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if fb.ID == "" {
		fb.ID = uuid.New().String()
	}
	if fb.CreatedAt.IsZero() {
		fb.CreatedAt = time.Now()
	}

	l.feedbacks = append(l.feedbacks, fb)
	if len(l.feedbacks) > l.maxFeedbacks {
		l.feedbacks = l.feedbacks[len(l.feedbacks)-l.maxFeedbacks:]
	}

	// Track correction patterns
	if fb.Type == FeedbackCorrection && fb.Dimension != "" {
		l.trackCorrection(fb)
	}

	slog.Debug("adaptive: feedback recorded", "type", fb.Type, "dim", fb.Dimension)
}

// ObserveInteraction extracts feedback from a user-agent interaction.
func (l *Loop) ObserveInteraction(ctx context.Context, userMsg, agentAction string) (*Feedback, error) {
	if l.extractFn == nil {
		return nil, nil
	}

	fb, err := l.extractFn(ctx, userMsg, agentAction)
	if err != nil {
		return nil, fmt.Errorf("adaptive: extract: %w", err)
	}
	if fb == nil {
		return nil, nil
	}

	l.RecordFeedback(*fb)
	return fb, nil
}

// ── Correction tracking ──

func (l *Loop) trackCorrection(fb Feedback) {
	cp, ok := l.corrections[fb.Dimension]
	if !ok {
		cp = &CorrectionPattern{
			Dimension: fb.Dimension,
		}
		l.corrections[fb.Dimension] = cp
	}
	cp.Pattern = fb.Correction
	cp.Occurrences++
	cp.LastSeen = fb.CreatedAt
	cp.Examples = append(cp.Examples, truncate(fb.UserMessage, 80))
	if len(cp.Examples) > 5 {
		cp.Examples = cp.Examples[len(cp.Examples)-5:]
	}
}

// CorrectionPatterns returns all tracked correction patterns.
func (l *Loop) CorrectionPatterns() []CorrectionPattern {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]CorrectionPattern, 0, len(l.corrections))
	for _, cp := range l.corrections {
		out = append(out, *cp)
	}
	return out
}

// ── Adaptation ──

// Adapt analyzes accumulated feedbacks and generates behavior adaptations.
func (l *Loop) Adapt(ctx context.Context) (adapted int, err error) {
	l.mu.Lock()
	feedbacks := make([]Feedback, len(l.feedbacks))
	copy(feedbacks, l.feedbacks)
	l.mu.Unlock()

	if len(feedbacks) < l.adaptThreshold {
		return 0, nil
	}

	var rules []AdaptationRule

	if l.adaptFn != nil {
		rules, err = l.adaptFn(ctx, feedbacks)
		if err != nil {
			return 0, fmt.Errorf("adaptive: adapt: %w", err)
		}
	} else {
		// Heuristic adaptation
		rules = l.heuristicAdapt(feedbacks)
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	for i := range rules {
		r := rules[i]
		r.Active = true
		l.rules[r.Dimension] = &r
		l.profile.Settings[r.Dimension] = r.TargetVal
		adapted++
	}

	if adapted > 0 {
		l.profile.UpdatedAt = time.Now()
		l.profile.Version++
		slog.Info("adaptive: adapted", "rules", adapted, "version", l.profile.Version)
	}
	return adapted, nil
}

// heuristicAdapt generates rules from feedback patterns without LLM.
func (l *Loop) heuristicAdapt(feedbacks []Feedback) []AdaptationRule {
	// Count feedback dimensions
	dimCounts := make(map[string]map[FeedbackType]int) // dimension -> type -> count
	dimValues := make(map[string]string)               // dimension -> latest correction

	for _, fb := range feedbacks {
		if fb.Dimension == "" {
			continue
		}
		if dimCounts[fb.Dimension] == nil {
			dimCounts[fb.Dimension] = make(map[FeedbackType]int)
		}
		dimCounts[fb.Dimension][fb.Type]++
		if fb.Correction != "" {
			dimValues[fb.Dimension] = fb.Correction
		}
	}

	var rules []AdaptationRule
	for dim, counts := range dimCounts {
		total := 0
		for _, c := range counts {
			total += c
		}
		if total < l.adaptThreshold {
			continue
		}

		corrections := counts[FeedbackCorrection]
		negatives := counts[FeedbackNegative]
		positives := counts[FeedbackPositive]

		// Only adapt if there's a clear signal
		if corrections+negatives <= positives {
			continue
		}

		target := dimValues[dim]
		if target == "" {
			continue
		}

		confidence := float64(corrections+negatives) / float64(total)
		rules = append(rules, AdaptationRule{
			ID:          uuid.New().String(),
			Dimension:   dim,
			TargetVal:   target,
			Confidence:  confidence,
			FeedbackCnt: total,
		})
	}
	return rules
}

// ── Query ──

// Profile returns the current behavior profile.
func (l *Loop) Profile() BehaviorProfile {
	l.mu.RLock()
	defer l.mu.RUnlock()
	cp := BehaviorProfile{
		Settings:  make(map[string]string, len(l.profile.Settings)),
		UpdatedAt: l.profile.UpdatedAt,
		Version:   l.profile.Version,
	}
	for k, v := range l.profile.Settings {
		cp.Settings[k] = v
	}
	return cp
}

// SetSetting manually sets a behavior dimension.
func (l *Loop) SetSetting(dimension, value string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.profile.Settings[dimension] = value
	l.profile.UpdatedAt = time.Now()
	l.profile.Version++
}

// GetSetting returns the current value for a behavior dimension.
func (l *Loop) GetSetting(dimension string) (string, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	v, ok := l.profile.Settings[dimension]
	return v, ok
}

// Rules returns all active adaptation rules.
func (l *Loop) Rules() []AdaptationRule {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]AdaptationRule, 0, len(l.rules))
	for _, r := range l.rules {
		out = append(out, *r)
	}
	return out
}

// Feedbacks returns recent feedbacks.
func (l *Loop) Feedbacks(limit int) []Feedback {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if limit <= 0 || limit > len(l.feedbacks) {
		limit = len(l.feedbacks)
	}
	start := len(l.feedbacks) - limit
	out := make([]Feedback, limit)
	copy(out, l.feedbacks[start:])
	return out
}

// ── Stats ──

// Stats returns adaptive loop statistics.
type LoopStats struct {
	TotalFeedbacks  int            `json:"total_feedbacks"`
	FeedbackByType  map[string]int `json:"feedback_by_type"`
	ActiveRules     int            `json:"active_rules"`
	CorrectionCount int            `json:"correction_count"`
	ProfileVersion  int            `json:"profile_version"`
}

func (l *Loop) Stats() LoopStats {
	l.mu.RLock()
	defer l.mu.RUnlock()

	byType := make(map[string]int)
	for _, fb := range l.feedbacks {
		byType[string(fb.Type)]++
	}

	totalCorrections := 0
	for _, cp := range l.corrections {
		totalCorrections += cp.Occurrences
	}

	return LoopStats{
		TotalFeedbacks:  len(l.feedbacks),
		FeedbackByType:  byType,
		ActiveRules:     len(l.rules),
		CorrectionCount: totalCorrections,
		ProfileVersion:  l.profile.Version,
	}
}

// ── Reset ──

// Reset clears all feedbacks and adaptations.
func (l *Loop) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.feedbacks = nil
	l.corrections = make(map[string]*CorrectionPattern)
	l.rules = make(map[string]*AdaptationRule)
	l.profile = BehaviorProfile{Settings: make(map[string]string)}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
