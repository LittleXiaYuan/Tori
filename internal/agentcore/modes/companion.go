package modes

import (
	"context"
	"fmt"
	"log/slog"

	ldg "yunque-agent/internal/ledgercore"

	"yunque-agent/internal/agentcore/emotion"
)

// companionMode implements BehaviorEngine for 云雀·伴.
//
// Companion shares Spirit's value system but delivers stances with more warmth
// and less bluntness. It still has clear opinions — good is good, bad is bad —
// but wraps them in gentler language.
//
// Key behavioral traits:
//   - Same multi-dimensional judgment as Spirit, but slightly dampened strength
//   - Higher stance threshold: only speaks up when fairly confident
//   - Balanced context compression: keeps emotional + task history
//   - Medium memory depth: stores stances and emotions, no relationship graph
//   - Moderate temperature for personality without unpredictability
type companionMode struct {
	preset    *ModePreset
	values    *ValueSystem
	stanceGen *StanceGenerator
	ledger    *ldg.Ledger
	tenantID  string
}

// NewCompanionMode creates a Companion behavior engine.
func NewCompanionMode(llmCall LLMCallFunc, ldg *ldg.Ledger, tenantID, locale string) BehaviorEngine {
	preset := ModePresets[ModeCompanion]
	vs := NewValueSystem(DefaultDimensions())
	vs.SetLLMCall(llmCall)
	vs.SetLocale(locale)

	sg := NewStanceGenerator(llmCall, locale)

	return &companionMode{
		preset:    preset,
		values:    vs,
		stanceGen: sg,
		ledger:    ldg,
		tenantID:  tenantID,
	}
}

func (c *companionMode) Mode() PersonaMode { return ModeCompanion }

// Judge evaluates input using the full value system.
// Companion mode slightly dampens judgment strength compared to Spirit,
// reflecting its more measured personality.
func (c *companionMode) Judge(ctx context.Context, input string, emo *emotion.Result) (*Judgment, error) {
	j, err := c.values.Evaluate(ctx, input, emo)
	if err != nil {
		return nil, fmt.Errorf("companion judge: %w", err)
	}

	// Companion dampens strength slightly — it's principled but not as intense.
	if j.Strength > 0 {
		j.Strength = clampFloat(j.Strength*0.9, 0, 1)
	}

	if c.ledger != nil && j.IsSignificant(c.preset.StanceThreshold) {
		c.recordJudgmentTrace(ctx, j)
	}

	return j, nil
}

// Stance converts a Judgment into a warm but principled expression.
func (c *companionMode) Stance(ctx context.Context, j *Judgment) (*Stance, error) {
	if !j.IsSignificant(c.preset.StanceThreshold) {
		return &Stance{Position: PositionNeutral}, nil
	}

	stance, err := c.stanceGen.Generate(ctx, j, c.preset.Tone, j.Reasoning)
	if err != nil {
		return nil, fmt.Errorf("companion stance: %w", err)
	}

	if c.ledger != nil && c.preset.Memory.StoreStance {
		c.persistStance(ctx, j, stance)
	}

	return stance, nil
}

func (c *companionMode) Sampling() SamplingConfig               { return c.preset.Sampling }
func (c *companionMode) ContextStrategy() ContextStrategy       { return c.preset.Context }
func (c *companionMode) MemoryPolicy() MemoryPolicy             { return c.preset.Memory }
func (c *companionMode) GuardrailOverrides() GuardrailOverrides { return c.preset.Guardrails }

func (c *companionMode) recordJudgmentTrace(ctx context.Context, j *Judgment) {
	tracer := c.ledger.Reasoning("mode:companion:"+c.tenantID, "companion")
	direction := "positive"
	if j.Valence < 0 {
		direction = "negative"
	}
	tracer.Think(ctx, fmt.Sprintf(
		"价值判断: %s (方向=%s, 强度=%.2f)", j.Reasoning, direction, j.Strength,
	), nil)
}

func (c *companionMode) persistStance(ctx context.Context, j *Judgment, stance *Stance) {
	key := fmt.Sprintf("stance:%s:%d", truncate(j.Reasoning, 40), j.Valence)
	value := fmt.Sprintf("%s|%.2f|%s", stance.Position, stance.Intensity, stance.Text)

	if err := c.ledger.Memory.PutFact(ctx, c.tenantID, key, value, "stance"); err != nil {
		slog.Debug("companion: failed to persist stance", "err", err)
	}
}
