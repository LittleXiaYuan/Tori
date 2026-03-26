package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"ledger"

	"yunque-agent/internal/agentcore/taskdistill"
)

// EvalResult is the structured output of a task self-evaluation.
type EvalResult struct {
	TaskID        string    `json:"task_id"`
	GoalAchieved  float64   `json:"goal_achieved"`
	Efficiency    float64   `json:"efficiency"`
	SideEffects   []string  `json:"side_effects"`
	QualityScore  float64   `json:"quality_score"`
	Reasoning     string    `json:"reasoning"`
	Suggestions   []string  `json:"suggestions"`
	ShouldDistill bool      `json:"should_distill"`
	EvaluatedAt   time.Time `json:"evaluated_at"`
}

// EvalFunc is the LLM-powered evaluation function.
type EvalFunc func(ctx context.Context, summary taskdistill.TaskEventSummary) (*EvalResult, error)

// Evaluator performs self-evaluation on completed tasks.
type Evaluator struct {
	ldg       *ledger.Ledger
	distiller *taskdistill.Distiller
	evalFn    EvalFunc
}

// New creates a self-evaluator.
func New(ldg *ledger.Ledger) *Evaluator {
	return &Evaluator{
		ldg:       ldg,
		distiller: taskdistill.New(ldg),
	}
}

// SetEvalFunc sets the LLM-powered evaluation function.
func (ev *Evaluator) SetEvalFunc(fn EvalFunc) { ev.evalFn = fn }

// SetDistiller sets the distiller for auto-distillation on low scores.
func (ev *Evaluator) SetDistiller(d *taskdistill.Distiller) { ev.distiller = d }

// Evaluate runs self-evaluation on a completed task.
func (ev *Evaluator) Evaluate(ctx context.Context, taskID string) (*EvalResult, error) {
	summary, err := ev.distiller.BuildTaskSummary(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("build summary: %w", err)
	}

	if !summary.Status.IsTerminal() {
		return nil, fmt.Errorf("task %s is not terminal (status: %s)", taskID, summary.Status)
	}

	var result *EvalResult

	if ev.evalFn != nil {
		result, err = ev.evalFn(ctx, *summary)
		if err != nil {
			return nil, fmt.Errorf("eval: %w", err)
		}
	} else {
		result = ev.heuristicEval(summary)
	}

	result.TaskID = taskID
	result.EvaluatedAt = time.Now()

	evalPayload, _ := json.Marshal(result)
	ev.ldg.Events.Append(ctx, &ledger.Event{
		TaskID:    taskID,
		Kind:      EventEvalCompleted,
		Actor:     "evaluator",
		Payload:   evalPayload,
		CreatedAt: time.Now(),
	})

	if summary.TenantID != "" {
		ev.ldg.Memory.Put(ctx, &ledger.MemoryEntry{
			TenantID:   summary.TenantID,
			TaskID:     &taskID,
			Kind:       ledger.MemoryExperience,
			Key:        "eval.score." + taskID,
			Content:    fmt.Sprintf("Task '%s' scored %.2f/1.0: %s", summary.Goal, result.QualityScore, result.Reasoning),
			Source:     "evaluation",
			Confidence: result.QualityScore,
		})
	}

	if result.ShouldDistill && ev.distiller != nil && summary.TenantID != "" {
		ev.distiller.DistillTask(ctx, taskID, summary.TenantID)
	}

	return result, nil
}

// EvaluateBatch evaluates all un-evaluated completed tasks.
func (ev *Evaluator) EvaluateBatch(ctx context.Context, tenantID string, limit int) ([]*EvalResult, error) {
	tasks, err := ev.ldg.Backend().ListTasks(ctx, ledger.TaskFilter{
		TenantID: tenantID,
		Status:   []ledger.TaskStatus{ledger.TaskCompleted, ledger.TaskFailed},
		Limit:    limit,
	})
	if err != nil {
		return nil, err
	}

	var results []*EvalResult
	for _, task := range tasks {
		existing, _ := ev.ldg.Memory.Search(ctx, ledger.MemoryQuery{
			TenantID: tenantID,
			TaskID:   &task.ID,
			Kinds:    []ledger.MemoryKind{ledger.MemoryExperience},
			Limit:    100,
		})
		alreadyEvaluated := false
		for _, m := range existing {
			if m.Key == "eval.score."+task.ID {
				alreadyEvaluated = true
				break
			}
		}
		if alreadyEvaluated {
			continue
		}

		result, err := ev.Evaluate(ctx, task.ID)
		if err != nil {
			continue
		}
		results = append(results, result)
	}

	return results, nil
}

func (ev *Evaluator) heuristicEval(s *taskdistill.TaskEventSummary) *EvalResult {
	result := &EvalResult{}

	if s.Status == ledger.TaskCompleted {
		result.GoalAchieved = 0.7
		if s.Backtracks == 0 {
			result.GoalAchieved = 0.9
		}
	} else {
		result.GoalAchieved = 0.2
	}

	idealSteps := 3.0
	actualSteps := float64(s.StepCount)
	if actualSteps == 0 {
		actualSteps = 1
	}
	result.Efficiency = clamp(idealSteps/actualSteps, 0, 1)

	backtrackPenalty := float64(s.Backtracks) * 0.1
	result.Efficiency = clamp(result.Efficiency-backtrackPenalty, 0, 1)

	result.QualityScore = result.GoalAchieved*0.6 + result.Efficiency*0.4

	if s.Status == ledger.TaskCompleted {
		result.Reasoning = fmt.Sprintf("Task completed in %d steps with %d backtracks", s.StepCount, s.Backtracks)
	} else {
		result.Reasoning = fmt.Sprintf("Task failed after %d steps: %v", s.StepCount, s.Errors)
	}

	if s.Backtracks > 2 {
		result.Suggestions = append(result.Suggestions, "Consider planning more carefully before execution to reduce backtracks")
	}
	if s.StepCount > 10 {
		result.Suggestions = append(result.Suggestions, "Task took many steps — look for opportunities to batch operations")
	}
	if s.Status == ledger.TaskFailed {
		result.Suggestions = append(result.Suggestions, "Analyze failure root cause and develop preventive strategy")
	}

	result.ShouldDistill = result.QualityScore < 0.6

	return result
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

const EventEvalCompleted ledger.EventKind = "eval.completed"
