package modes

import (
	"context"
	"fmt"
	"log/slog"

	ldg "ledger"

	"yunque-agent/internal/agentcore/emotion"
)

// spiritMode implements BehaviorEngine for 云雀·灵.
//
// Spirit is the most expressive mode: high emotional range, direct opinions,
// and a low threshold for speaking up. It uses the full value system and
// records stance history to maintain consistency across conversations.
//
// Key behavioral traits:
//   - Judges input across all dimensions and expresses stance readily
//   - Uses conservative context compression to preserve emotional history
//   - Deep memory: stores stances, emotions, and relationship signals
//   - High temperature + presence penalty for varied, personality-rich output
//   - Low agreement bias: will disagree when the value system says so
type spiritMode struct {
	preset   *ModePreset
	values   *ValueSystem
	stanceGen *StanceGenerator
	ledger   *ldg.Ledger
	tenantID string
}

// NewSpiritMode creates a Spirit behavior engine.
func NewSpiritMode(llmCall LLMCallFunc, ldg *ldg.Ledger, tenantID, locale string) BehaviorEngine {
	preset := ModePresets[ModeSpirit]
	vs := NewValueSystem(DefaultDimensions())
	vs.SetLLMCall(llmCall)
	vs.SetLocale(locale)

	sg := NewStanceGenerator(llmCall, locale)

	return &spiritMode{
		preset:    preset,
		values:    vs,
		stanceGen: sg,
		ledger:    ldg,
		tenantID:  tenantID,
	}
}

func (s *spiritMode) Mode() PersonaMode { return ModeSpirit }

// Judge evaluates input using the full multi-dimensional value system.
// Spirit mode factors in user emotion to modulate empathy-aware judgment.
func (s *spiritMode) Judge(ctx context.Context, input string, emo *emotion.Result) (*Judgment, error) {
	j, err := s.values.Evaluate(ctx, input, emo)
	if err != nil {
		return nil, fmt.Errorf("spirit judge: %w", err)
	}

	// Spirit mode amplifies judgment strength slightly — it feels more strongly.
	if j.Strength > 0 {
		j.Strength = clampFloat(j.Strength*1.15, 0, 1)
	}

	// Record reasoning trace to ledger if available
	if s.ledger != nil && j.IsSignificant(s.preset.StanceThreshold) {
		s.recordJudgmentTrace(ctx, input, j)
	}

	return j, nil
}

// Stance converts a Judgment into a direct, warm expression.
func (s *spiritMode) Stance(ctx context.Context, j *Judgment) (*Stance, error) {
	if !j.IsSignificant(s.preset.StanceThreshold) {
		return &Stance{Position: PositionNeutral}, nil
	}

	stance, err := s.stanceGen.Generate(ctx, j, s.preset.Tone, j.Reasoning)
	if err != nil {
		return nil, fmt.Errorf("spirit stance: %w", err)
	}

	// Persist stance for consistency
	if s.ledger != nil && s.preset.Memory.StoreStance {
		s.persistStance(ctx, j, stance)
	}

	return stance, nil
}

func (s *spiritMode) Sampling() SamplingConfig         { return s.preset.Sampling }
func (s *spiritMode) ContextStrategy() ContextStrategy  { return s.preset.Context }
func (s *spiritMode) MemoryPolicy() MemoryPolicy        { return s.preset.Memory }
func (s *spiritMode) GuardrailOverrides() GuardrailOverrides { return s.preset.Guardrails }

// ─── Ledger integration ─────────────────────────────────────────────────────

func (s *spiritMode) recordJudgmentTrace(ctx context.Context, input string, j *Judgment) {
	tracer := s.ledger.Reasoning("mode:spirit:"+s.tenantID, "spirit")
	direction := "positive"
	if j.Valence < 0 {
		direction = "negative"
	}
	tracer.Think(ctx, fmt.Sprintf(
		"对用户输入进行了价值判断: %s (方向=%s, 强度=%.2f)",
		j.Reasoning, direction, j.Strength,
	), nil)

	if j.IsSignificant(s.preset.StanceThreshold) {
		tracer.Decide(ctx, direction, j.Reasoning, j.Strength, nil)
	}
}

func (s *spiritMode) persistStance(ctx context.Context, j *Judgment, stance *Stance) {
	key := fmt.Sprintf("stance:%s:%d", truncate(j.Reasoning, 40), j.Valence)
	value := fmt.Sprintf("%s|%.2f|%s", stance.Position, stance.Intensity, stance.Text)

	if err := s.ledger.Memory.PutFact(ctx, s.tenantID, key, value, "stance"); err != nil {
		slog.Debug("spirit: failed to persist stance", "err", err)
	}
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen])
}
