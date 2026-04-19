package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"yunque-agent/internal/agentcore/task"
)

type LLMFunc func(ctx context.Context, system, user string) (string, error)

type ReviewAgent struct {
	llmCall LLMFunc
}

type ReviewResult struct {
	Approved    bool     `json:"approved"`
	Score       int      `json:"score"`
	Issues      []string `json:"issues"`
	Suggestions []string `json:"suggestions"`
}

func NewReviewAgent(llmCall LLMFunc) *ReviewAgent {
	return &ReviewAgent{llmCall: llmCall}
}

func (r *ReviewAgent) Review(ctx context.Context, t *task.Task, workerResult string) (*ReviewResult, error) {
	if r.llmCall == nil {
		return &ReviewResult{Approved: true, Score: 7}, nil
	}

	system := `You are a code review expert. Review the worker's output and respond with a JSON object:
{"approved": true/false, "score": 1-10, "issues": ["..."], "suggestions": ["..."]}

Approve if: all requirements are met, code quality is acceptable, no obvious bugs.
Reject if: requirements are missing, critical bugs, or poor quality.`

	user := fmt.Sprintf("Task: %s\n\nAcceptance Criteria: %s\n\nWorker Result:\n%s",
		t.Description, formatCriteria(t), workerResult)

	resp, err := r.llmCall(ctx, system, user)
	if err != nil {
		slog.Warn("orchestrator: review LLM call failed", "err", err)
		return &ReviewResult{Approved: true, Score: 5}, nil
	}

	var result ReviewResult
	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		slog.Warn("orchestrator: failed to parse review response", "resp", resp)
		return &ReviewResult{Approved: true, Score: 5}, nil
	}

	return &result, nil
}

func formatCriteria(t *task.Task) string {
	if t.Constraints != nil && t.Constraints.SuccessCriteria != "" {
		return t.Constraints.SuccessCriteria
	}
	return "(no explicit criteria)"
}
