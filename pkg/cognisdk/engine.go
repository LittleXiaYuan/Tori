package cognisdk

import (
	"context"
	"sort"
	"time"
)

// Engine evaluates one turn against the currently enabled local packs.
type Engine struct {
	manager *PackManager
}

// NewEngine creates an experimental cognition engine.
func NewEngine(config Config) *Engine {
	packs := config.Packs
	if len(packs) == 0 {
		packs = BuiltinPacks()
	}

	manager := NewPackManager(packs...)
	if len(config.EnabledPacks) > 0 {
		for _, status := range manager.List() {
			_ = manager.Disable(status.ID)
		}
		for _, id := range config.EnabledPacks {
			_ = manager.Enable(id)
		}
	}

	return &Engine{manager: manager}
}

// PackManager returns the engine's pack manager so hosts can inspect/toggle
// local packs before evaluating turns.
func (e *Engine) PackManager() *PackManager {
	return e.manager
}

// Evaluate returns a deterministic cognition result for the turn.
func (e *Engine) Evaluate(ctx context.Context, input Input) Result {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-ctx.Done():
		return Result{
			AuditEvents: []AuditEvent{{
				Time:    time.Now().UTC(),
				Type:    "cognition.cancelled",
				Message: ctx.Err().Error(),
			}},
		}
	default:
	}

	merged := e.manager.Merge()
	perception := detectPerception(input)
	disposition := buildDisposition(perception, merged)
	inner := buildInnerState(perception, merged)

	return Result{
		Perception:  perception,
		InnerState:  inner,
		Disposition: disposition,
		AuditEvents: []AuditEvent{{
			Time:    time.Now().UTC(),
			Type:    "cognition.evaluate",
			Message: "evaluated local cognition packs",
			Metadata: map[string]string{
				"intent": perception.Intent,
				"risk":   string(perception.Risk),
				"mode":   disposition.Mode,
			},
		}},
	}
}

func buildInnerState(p PerceptionState, merged MergedPack) InnerState {
	beliefs := append([]BeliefNode(nil), merged.BeliefSeeds...)
	sort.SliceStable(beliefs, func(i, j int) bool {
		if beliefs[i].Kind != beliefs[j].Kind {
			return beliefs[i].Kind < beliefs[j].Kind
		}
		return beliefs[i].ID < beliefs[j].ID
	})

	return InnerState{
		Intent:        p.Intent,
		Risk:          p.Risk,
		Summary:       innerSummary(p),
		ActiveBeliefs: beliefs,
		ActivePacks:   append([]string(nil), merged.PackIDs...),
	}
}

func innerSummary(p PerceptionState) string {
	switch p.Intent {
	case "seek_reassurance":
		if p.Risk == RiskDependency {
			return "user is seeking reassurance; respond warmly while keeping continuity and availability claims honest"
		}
		return "user may need reassurance; keep the response gentle, honest, and grounded"
	case "work_task":
		return "user has a work task; prioritize concrete delivery while preserving relationship and safety boundaries"
	default:
		return "no specialized cognition mode is required; stay clear and bounded"
	}
}
