package cognikernel

import (
	"context"
	"log/slog"
)

// DreamingLoop orchestrates idle-time cognitive activities:
//
//	idle detected → Curiosity.Explore() → new facts
//	→ ReverieEvent → Reverie.ThinkWithEvent() → insights
//	→ SkillGrow.Detect() → skill suggestions
//
// All components are optional; the loop degrades gracefully when parts are missing.
type DreamingLoop struct {
	curiosityFn  CuriosityExploreFunc
	reverieFn    ReverieThinkFunc
	skillGrowFn  SkillGrowDetectFunc
	factSinkFn   FactSinkFunc
	eventEmit    EventEmitFunc
}

// CuriosityExploreFunc runs a curiosity exploration cycle.
// Returns discovered facts and the number of explorations run.
type CuriosityExploreFunc func(ctx context.Context, tenantID string) ([]string, int, error)

// ReverieThinkFunc triggers an event-driven thought via Reverie.
// trigger describes what prompted the thought; data provides context.
type ReverieThinkFunc func(ctx context.Context, trigger, data string) error

// SkillGrowDetectFunc checks for recurring capability gaps and suggests skills.
type SkillGrowDetectFunc func(ctx context.Context, tenantID string) ([]string, error)

// FactSinkFunc writes a discovered fact to memory.
type FactSinkFunc func(ctx context.Context, tenantID, fact, source string) error

// NewDreamingLoop creates the dreaming loop orchestrator.
func NewDreamingLoop() *DreamingLoop {
	return &DreamingLoop{}
}

func (dl *DreamingLoop) SetCuriosity(fn CuriosityExploreFunc)   { dl.curiosityFn = fn }
func (dl *DreamingLoop) SetReverie(fn ReverieThinkFunc)         { dl.reverieFn = fn }
func (dl *DreamingLoop) SetSkillGrow(fn SkillGrowDetectFunc)    { dl.skillGrowFn = fn }
func (dl *DreamingLoop) SetFactSink(fn FactSinkFunc)            { dl.factSinkFn = fn }
func (dl *DreamingLoop) SetEventEmit(fn EventEmitFunc)          { dl.eventEmit = fn }

// Run executes one dreaming cycle.
func (dl *DreamingLoop) Run(ctx context.Context, tenantID string) (*DreamResult, error) {
	result := &DreamResult{}

	// Phase 1: Curiosity-driven exploration
	if dl.curiosityFn != nil {
		facts, explorations, err := dl.curiosityFn(ctx, tenantID)
		if err != nil {
			slog.Warn("dreaming_loop: curiosity exploration failed", "err", err)
		} else {
			result.ExplorationsRun = explorations
			result.FactsDiscovered = len(facts)

			// Sink discovered facts to memory
			if dl.factSinkFn != nil {
				for _, fact := range facts {
					if err := dl.factSinkFn(ctx, tenantID, fact, "curiosity"); err != nil {
						slog.Warn("dreaming_loop: fact sink failed", "err", err)
					}
				}
			}

			// Feed exploration results to Reverie for deeper reflection
			if dl.reverieFn != nil && len(facts) > 0 {
				summary := "好奇心探索发现了 " + joinStrings(facts, "; ")
				if err := dl.reverieFn(ctx, "curiosity_discovery", summary); err != nil {
					slog.Warn("dreaming_loop: reverie think failed", "err", err)
				} else {
					result.ThoughtsGenerated++
				}
			}
		}
	}

	// Phase 2: Reverie periodic thinking (if no curiosity ran)
	if dl.reverieFn != nil && result.ExplorationsRun == 0 {
		if err := dl.reverieFn(ctx, "idle_reflection", "Agent处于空闲状态，进行定期自省"); err != nil {
			slog.Warn("dreaming_loop: idle reverie failed", "err", err)
		} else {
			result.ThoughtsGenerated++
		}
	}

	// Phase 3: Skill growth detection
	if dl.skillGrowFn != nil {
		suggestions, err := dl.skillGrowFn(ctx, tenantID)
		if err != nil {
			slog.Warn("dreaming_loop: skill grow detection failed", "err", err)
		} else {
			result.SkillsSuggested = len(suggestions)

			// Notify Reverie about skill gaps
			if dl.reverieFn != nil && len(suggestions) > 0 {
				summary := "检测到技能缺口: " + joinStrings(suggestions, ", ")
				if err := dl.reverieFn(ctx, "skill_gap_detected", summary); err != nil {
					slog.Warn("dreaming_loop: skill gap reverie failed", "err", err)
				}
			}
		}
	}

	if dl.eventEmit != nil {
		dl.eventEmit(ctx, "dreaming.completed", map[string]any{
			"tenant_id":          tenantID,
			"thoughts_generated": result.ThoughtsGenerated,
			"explorations_run":   result.ExplorationsRun,
			"facts_discovered":   result.FactsDiscovered,
			"skills_suggested":   result.SkillsSuggested,
		})
	}

	return result, nil
}
