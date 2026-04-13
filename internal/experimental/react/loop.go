package react

import (
	"context"
	"fmt"
	"time"

	"ledger"
)

// Runner executes ReAct and Plan-Execute-Reflect loops,
// using Ledger's ReasoningTracer for structured trace recording.
type Runner struct {
	ldg *ledger.Ledger
}

// NewRunner creates a ReAct runner backed by a Ledger instance.
func NewRunner(ldg *ledger.Ledger) *Runner {
	return &Runner{ldg: ldg}
}

// ReActLoop runs the Observe→Think→Act cycle with automatic reasoning trace recording.
//
// Flow per step:
//  1. Think: LLM observes history and produces Thought + Action (or Answer)
//  2. Trace: Thought is recorded as reasoning.thought + reasoning.decision
//  3. Act: If Action is non-nil, execute the tool
//  4. Observe: Tool result becomes the next step's observation
//  5. Check: If confidence drops below min, record reasoning.reflect
//  6. Repeat until Action is nil (done), max steps, or error
func (r *Runner) ReActLoop(ctx context.Context, taskID string, initialObs string, cfg ledger.ReActConfig, think ledger.ThinkFunc, act ledger.ActFunc, onStep ledger.ReActOnStep) (*ledger.ReActResult, error) {
	cfg.Defaults()
	tracer := r.ldg.Reasoning(taskID, cfg.Actor)

	result := &ledger.ReActResult{}
	var history []ledger.ReActStep
	currentObs := initialObs

	if currentObs != "" {
		tracer.Observe(ctx, currentObs, nil)
	}

	for step := 1; step <= cfg.MaxSteps; step++ {
		select {
		case <-ctx.Done():
			result.StopReason = "cancelled"
			result.Steps = history
			result.TotalSteps = len(history)
			return result, ctx.Err()
		default:
		}

		stepStart := time.Now()

		tr, err := think(ctx, history)
		if err != nil {
			result.StopReason = "error"
			result.Steps = history
			result.TotalSteps = len(history)
			return result, fmt.Errorf("react step %d think: %w", step, err)
		}

		tracer.Think(ctx, tr.Thought, map[string]interface{}{
			"step": step,
		})

		current := ledger.ReActStep{
			StepNum:     step,
			Observation: currentObs,
			Thought:     tr.Thought,
			Confidence:  tr.Confidence,
		}

		if tr.Confidence > 0 && tr.Confidence < cfg.MinConfidence {
			tracer.Reflect(ctx,
				fmt.Sprintf("Low confidence (%.2f) at step %d — may need to reconsider approach", tr.Confidence, step),
				tr.Confidence, nil)
		}

		if tr.Action == nil {
			current.DurationMs = time.Since(stepStart).Milliseconds()
			history = append(history, current)

			tracer.Decide(ctx, "finish", "reasoning complete", tr.Confidence, map[string]interface{}{
				"step":   step,
				"answer": ledger.TruncateStr(tr.Answer, 200),
			})

			result.Answer = tr.Answer
			result.Success = true
			result.StopReason = "answer"
			result.Steps = history
			result.TotalSteps = len(history)

			if onStep != nil {
				onStep(current)
			}
			return result, nil
		}

		current.Action = tr.Action
		tracer.Decide(ctx, tr.Action.Name, tr.Thought, tr.Confidence, map[string]interface{}{
			"step": step,
			"args": tr.Action.Args,
		})

		toolResult, err := act(ctx, *tr.Action)
		if err != nil {
			toolResult = &ledger.ToolResult{Error: err.Error()}
		}
		current.Result = toolResult

		if toolResult.Error != "" {
			currentObs = fmt.Sprintf("Tool %s failed: %s", tr.Action.Name, toolResult.Error)
			tracer.Observe(ctx, currentObs, map[string]interface{}{
				"step":   step,
				"tool":   tr.Action.Name,
				"status": "error",
			})

			if cfg.BacktrackOnFail {
				result.Backtracks++
				tracer.Backtrack(ctx,
					fmt.Sprintf("Tool %s failed: %s", tr.Action.Name, toolResult.Error),
					"retry with different approach",
					map[string]interface{}{"step": step})
			}
		} else {
			currentObs = toolResult.Output
			tracer.Observe(ctx, ledger.TruncateStr(currentObs, 500), map[string]interface{}{
				"step":   step,
				"tool":   tr.Action.Name,
				"status": "ok",
			})
		}

		current.DurationMs = time.Since(stepStart).Milliseconds()
		history = append(history, current)

		if onStep != nil {
			onStep(current)
		}
	}

	result.StopReason = "max_steps"
	result.Steps = history
	result.TotalSteps = len(history)

	tracer.Reflect(ctx,
		fmt.Sprintf("Reached max steps (%d) without final answer", cfg.MaxSteps),
		0.3, map[string]interface{}{
			"max_steps":  cfg.MaxSteps,
			"backtracks": result.Backtracks,
		})

	return result, nil
}
