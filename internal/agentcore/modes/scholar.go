package modes

import (
	"context"

	"yunque-agent/internal/agentcore/emotion"
)

// scholarMode implements BehaviorEngine for 云雀·学.
//
// Scholar is the neutral, fact-driven mode. It has no value system,
// never takes stances, and optimizes for accuracy and efficiency.
//
// Key behavioral traits:
//   - Judge() always returns zero Judgment (no opinions)
//   - Stance() always returns neutral (no position)
//   - Aggressive context compression: only keeps task-relevant history
//   - Shallow memory: only task-related facts
//   - Low temperature for deterministic, precise output
//   - Zero agreement bias: neither agrees nor disagrees, just states facts
type scholarMode struct {
	preset *ModePreset
}

// NewScholarMode creates a Scholar behavior engine.
// Scholar needs no LLM call or ledger — it doesn't judge or remember stances.
func NewScholarMode() BehaviorEngine {
	return &scholarMode{
		preset: ModePresets[ModeScholar],
	}
}

func (s *scholarMode) Mode() PersonaMode { return ModeScholar }

// Judge always returns a zero Judgment. Scholar doesn't form opinions.
func (s *scholarMode) Judge(_ context.Context, _ string, _ *emotion.Result) (*Judgment, error) {
	return &Judgment{Valence: 0, Strength: 0, Reasoning: "scholar: no value judgment"}, nil
}

// Stance always returns neutral. Scholar doesn't take positions.
func (s *scholarMode) Stance(_ context.Context, _ *Judgment) (*Stance, error) {
	return &Stance{Position: PositionNeutral}, nil
}

func (s *scholarMode) Sampling() SamplingConfig              { return s.preset.Sampling }
func (s *scholarMode) ContextStrategy() ContextStrategy       { return s.preset.Context }
func (s *scholarMode) MemoryPolicy() MemoryPolicy             { return s.preset.Memory }
func (s *scholarMode) GuardrailOverrides() GuardrailOverrides  { return s.preset.Guardrails }
