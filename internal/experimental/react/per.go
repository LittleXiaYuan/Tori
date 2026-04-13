package react

import (
	"context"
	"fmt"

	"ledger"
)

// PlanExecuteReflect runs the three-stage reasoning loop:
//
//  1. Plan: Generate a step-by-step plan
//  2. Execute: Run the plan using ReAct loop
//  3. Reflect: Evaluate results, learn from outcome
//  4. If not satisfied, re-plan with reflection feedback
//
// The cycle repeats up to MaxAttempts times.
func (r *Runner) PlanExecuteReflect(
	ctx context.Context,
	taskID string,
	goal string,
	cfg ledger.PlanExecuteReflectConfig,
	planFn ledger.PlanFunc,
	think ledger.ThinkFunc,
	act ledger.ActFunc,
	reflectFn ledger.ReflectFunc,
) (*ledger.PERResult, error) {
	cfg.Defaults()
	tracer := r.ldg.Reasoning(taskID, cfg.Actor)

	result := &ledger.PERResult{}
	prevReflection := ""

	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		plan, err := planFn(ctx, goal, attempt, prevReflection)
		if err != nil {
			return result, fmt.Errorf("plan attempt %d: %w", attempt, err)
		}
		result.Plans = append(result.Plans, plan)

		tracer.Plan(ctx, plan, map[string]interface{}{
			"attempt": attempt,
			"goal":    ledger.TruncateStr(goal, 200),
		})

		initialObs := fmt.Sprintf("Goal: %s\nPlan:\n", goal)
		for i, step := range plan {
			initialObs += fmt.Sprintf("  %d. %s\n", i+1, step)
		}

		execResult, err := r.ReActLoop(ctx, taskID, initialObs, cfg.ReActConfig, think, act, nil)
		if err != nil {
			return result, fmt.Errorf("execute attempt %d: %w", attempt, err)
		}
		result.ExecResults = append(result.ExecResults, execResult)
		result.TotalSteps += execResult.TotalSteps

		reflection, err := reflectFn(ctx, goal, plan, execResult)
		if err != nil {
			return result, fmt.Errorf("reflect attempt %d: %w", attempt, err)
		}
		result.Reflections = append(result.Reflections, *reflection)

		tracer.Reflect(ctx,
			fmt.Sprintf("Attempt %d — Score: %.2f, Satisfied: %v", attempt, reflection.Score, reflection.Satisfied),
			reflection.Score,
			map[string]interface{}{
				"attempt":    attempt,
				"strengths":  reflection.Strengths,
				"weaknesses": reflection.Weaknesses,
				"suggestion": reflection.Suggestion,
			})

		if cfg.AutoLearn && cfg.TenantID != "" && len(reflection.Learnings) > 0 {
			for _, learning := range reflection.Learnings {
				r.ldg.Memory.PutExperience(ctx, cfg.TenantID, &taskID,
					"per.learning."+taskID,
					learning,
					reflection.Score)
			}
		}

		result.Attempts = attempt

		if reflection.Satisfied {
			result.FinalAnswer = execResult.Answer
			result.Success = true
			return result, nil
		}

		prevReflection = fmt.Sprintf("Previous attempt scored %.2f.\nWeaknesses: %v\nSuggestion: %s",
			reflection.Score, reflection.Weaknesses, reflection.Suggestion)

		tracer.Think(ctx,
			fmt.Sprintf("Not satisfied (%.2f). Replanning with feedback: %s", reflection.Score, reflection.Suggestion),
			map[string]interface{}{"attempt": attempt})
	}

	if len(result.ExecResults) > 0 {
		last := result.ExecResults[len(result.ExecResults)-1]
		result.FinalAnswer = last.Answer
	}
	return result, nil
}
