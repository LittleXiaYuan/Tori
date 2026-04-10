// Package modes implements the multi-persona behavior engine for Yunque Agent.
//
// Three modes define how the agent thinks, judges, and expresses:
//   - Spirit  (灵): emotionally rich, opinionated, direct
//   - Companion (伴): warm but principled, clear stance with gentle delivery
//   - Scholar (学): neutral, fact-driven, no value judgment
//
// Modes are NOT prompt templates. They alter the cognitive pipeline:
// sampling parameters, context compression strategy, value judgment,
// stance generation, memory depth, and guardrail sensitivity.
package modes

import (
	"context"
	"time"

	"yunque-agent/internal/agentcore/emotion"
)

// ─── Mode identifiers ───────────────────────────────────────────────────────

// PersonaMode identifies a behavior mode.
type PersonaMode string

const (
	ModeSpirit    PersonaMode = "spirit"    // 云雀·灵 — emotionally rich, opinionated
	ModeCompanion PersonaMode = "companion" // 云雀·伴 — warm but principled
	ModeScholar   PersonaMode = "scholar"   // 云雀·学 — neutral, fact-driven
)

// AllModes lists every recognized mode.
var AllModes = []PersonaMode{ModeSpirit, ModeCompanion, ModeScholar}

// Valid returns true if m is a recognized mode.
func (m PersonaMode) Valid() bool {
	for _, v := range AllModes {
		if v == m {
			return true
		}
	}
	return false
}

// ─── Core interface ─────────────────────────────────────────────────────────

// BehaviorEngine defines the cognitive contract for a mode.
// Each mode implements this interface with fundamentally different behavior,
// not just different prompts.
type BehaviorEngine interface {
	// Mode returns the identifier.
	Mode() PersonaMode

	// Judge evaluates user input against the mode's value system.
	// Scholar mode always returns a zero Judgment.
	Judge(ctx context.Context, input string, emo *emotion.Result) (*Judgment, error)

	// Stance converts a Judgment into an expressible position.
	Stance(ctx context.Context, j *Judgment) (*Stance, error)

	// Sampling returns LLM sampling parameters tuned for this mode.
	Sampling() SamplingConfig

	// ContextStrategy returns how conversation history should be compressed.
	ContextStrategy() ContextStrategy

	// MemoryPolicy returns what and how deeply to remember.
	MemoryPolicy() MemoryPolicy

	// GuardrailOverrides returns mode-specific guardrail adjustments.
	GuardrailOverrides() GuardrailOverrides
}

// ─── Judgment ───────────────────────────────────────────────────────────────

// Judgment is the output of the value system's evaluation.
type Judgment struct {
	// Valence: positive (+1), negative (-1), or neutral (0).
	Valence int `json:"valence"`

	// Strength: how strongly the mode feels about this (0.0–1.0).
	// Below the mode's threshold, the judgment is suppressed.
	Strength float64 `json:"strength"`

	// Dimensions: per-principle scores that contributed to this judgment.
	Dimensions []DimensionScore `json:"dimensions,omitempty"`

	// Reasoning: one-sentence explanation of why (from LLM, not template).
	Reasoning string `json:"reasoning"`

	// InputEmotion: the detected user emotion that influenced judgment.
	InputEmotion *emotion.Result `json:"input_emotion,omitempty"`
}

// IsSignificant returns true if the judgment is strong enough to express.
func (j *Judgment) IsSignificant(threshold float64) bool {
	if j == nil {
		return false
	}
	return j.Valence != 0 && j.Strength >= threshold
}

// DimensionScore records one principle's contribution.
type DimensionScore struct {
	Principle  string  `json:"principle"`
	Score      float64 `json:"score"`      // -1.0 to +1.0
	Confidence float64 `json:"confidence"` // 0.0 to 1.0
	Reason     string  `json:"reason"`
}

// ─── Stance ─────────────────────────────────────────────────────────────────

// Stance is the mode's expressible position on the user's input.
type Stance struct {
	Position  Position `json:"position"`  // support / oppose / neutral
	Intensity float64  `json:"intensity"` // 0.0–1.0
	Text      string   `json:"text"`      // natural language expression
	Reasoning string   `json:"reasoning"` // why this stance
	Tone      Tone     `json:"tone"`      // delivery style
}

// Position is the stance direction.
type Position string

const (
	PositionSupport Position = "support"
	PositionOppose  Position = "oppose"
	PositionNeutral Position = "neutral"
)

// Tone controls how the stance is delivered.
type Tone struct {
	Directness    float64 `json:"directness"`    // 0=indirect, 1=blunt
	Warmth        float64 `json:"warmth"`        // 0=cold, 1=warm
	Formality     float64 `json:"formality"`     // 0=casual, 1=formal
	Assertiveness float64 `json:"assertiveness"` // 0=passive, 1=assertive
}

// ─── Sampling ───────────────────────────────────────────────────────────────

// SamplingConfig controls LLM generation parameters per mode.
type SamplingConfig struct {
	Temperature      float64 `json:"temperature"`
	TopP             float64 `json:"top_p"`
	FrequencyPenalty float64 `json:"frequency_penalty"`
	PresencePenalty  float64 `json:"presence_penalty"`
}

// ─── Context strategy ───────────────────────────────────────────────────────

// CompressionStrategy names a context compression approach.
type CompressionStrategy string

const (
	// CompressAggressive drops aggressively, keeping only recent + task messages.
	CompressAggressive CompressionStrategy = "aggressive"
	// CompressBalanced keeps a mix of head context and recent messages.
	CompressBalanced CompressionStrategy = "balanced"
	// CompressConservative uses weighted scoring to preserve emotionally relevant history.
	CompressConservative CompressionStrategy = "conservative"
)

// ContextStrategy defines how conversation history is managed.
type ContextStrategy struct {
	MaxHistory  int                    `json:"max_history"`
	Strategy    CompressionStrategy    `json:"strategy"`
	Weights     map[string]float64     `json:"weights"` // message-type → priority
}

// ─── Memory policy ──────────────────────────────────────────────────────────

// MemoryPolicy controls what the mode remembers and how.
type MemoryPolicy struct {
	Depth         MemoryDepth `json:"depth"`
	StoreStance   bool        `json:"store_stance"`   // persist stance history
	StoreEmotion  bool        `json:"store_emotion"`  // persist emotion observations
	StoreRelation bool        `json:"store_relation"` // build relationship graph
}

// MemoryDepth controls how much context is persisted to long-term memory.
type MemoryDepth string

const (
	MemoryDeep    MemoryDepth = "deep"    // everything: facts, emotions, preferences, relations
	MemoryMedium  MemoryDepth = "medium"  // facts + preferences
	MemoryShallow MemoryDepth = "shallow" // task-related facts only
)

// ─── Guardrail overrides ────────────────────────────────────────────────────

// GuardrailOverrides lets a mode relax or tighten specific guardrails.
type GuardrailOverrides struct {
	AllowDisagreement bool    `json:"allow_disagreement"` // can the agent disagree with user
	AllowCriticism    bool    `json:"allow_criticism"`    // can the agent criticize ideas
	AgreementBias     float64 `json:"agreement_bias"`     // 0=no sycophancy, 1=always agree
	PIIProtection     bool    `json:"pii_protection"`     // always true in practice
}

// ─── Mode switch event ──────────────────────────────────────────────────────

// SwitchEvent records a mode transition for audit/analytics.
type SwitchEvent struct {
	TenantID  string      `json:"tenant_id"`
	SessionID string      `json:"session_id,omitempty"`
	From      PersonaMode `json:"from"`
	To        PersonaMode `json:"to"`
	Reason    string      `json:"reason"` // "user_request", "auto", "default"
	At        time.Time   `json:"at"`
}
