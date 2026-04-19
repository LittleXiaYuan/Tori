package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"

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
	TestPassed  *bool    `json:"test_passed,omitempty"`
	TestOutput  string   `json:"test_output,omitempty"`
}

func NewReviewAgent(llmCall LLMFunc) *ReviewAgent {
	return &ReviewAgent{llmCall: llmCall}
}

// Review evaluates worker output. If a TestCommand is defined on the task,
// it runs the command first and feeds the result into the LLM review.
func (r *ReviewAgent) Review(ctx context.Context, t *task.Task, workerResult string) (*ReviewResult, error) {
	workDir := resolveReviewWorkDir(t)
	testResult := r.runTestCommand(ctx, t, workDir)

	if testResult != nil && !*testResult.passed && r.llmCall == nil {
		return &ReviewResult{
			Approved:   false,
			Score:      2,
			Issues:     []string{"TestCommand failed: " + testResult.output},
			TestPassed: testResult.passed,
			TestOutput: testResult.output,
		}, nil
	}

	if r.llmCall == nil {
		approved := testResult == nil || *testResult.passed
		score := 7
		if !approved {
			score = 3
		}
		return &ReviewResult{Approved: approved, Score: score, TestPassed: boolPtr(approved)}, nil
	}

	system := `You are a code review expert. Review the worker's output and respond ONLY with a JSON object:
{"approved": true/false, "score": 1-10, "issues": ["..."], "suggestions": ["..."]}

Scoring guide:
- 9-10: Excellent, production ready
- 7-8: Good, minor improvements possible
- 5-6: Acceptable but needs polish
- 3-4: Significant issues
- 1-2: Reject, major rework needed

Consider: requirements coverage, code quality, obvious bugs, test results (if provided).`

	userParts := []string{
		fmt.Sprintf("Task: %s", t.Description),
		fmt.Sprintf("Acceptance Criteria: %s", formatCriteria(t)),
	}
	if testResult != nil {
		status := "PASSED"
		if !*testResult.passed {
			status = "FAILED"
		}
		userParts = append(userParts, fmt.Sprintf("Test Command Result [%s]:\n%s", status, testResult.output))
	}
	userParts = append(userParts, fmt.Sprintf("Worker Result:\n%s", workerResult))

	resp, err := r.llmCall(ctx, system, strings.Join(userParts, "\n\n"))
	if err != nil {
		slog.Warn("orchestrator: review LLM call failed", "err", err)
		if testResult != nil && !*testResult.passed {
			return &ReviewResult{Approved: false, Score: 3, Issues: []string{"LLM review failed + test failed"}, TestPassed: testResult.passed, TestOutput: testResult.output}, nil
		}
		return &ReviewResult{Approved: true, Score: 5}, nil
	}

	var result ReviewResult
	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		slog.Warn("orchestrator: failed to parse review response", "resp", resp)
		return &ReviewResult{Approved: true, Score: 5}, nil
	}

	if testResult != nil {
		result.TestPassed = testResult.passed
		result.TestOutput = testResult.output
		if !*testResult.passed && result.Approved {
			result.Approved = false
			if result.Score > 4 {
				result.Score = 4
			}
			result.Issues = append(result.Issues, "TestCommand failed — cannot approve")
		}
	}

	return &result, nil
}

type testCommandResult struct {
	passed *bool
	output string
}

func (r *ReviewAgent) runTestCommand(ctx context.Context, t *task.Task, workDir string) *testCommandResult {
	if t.Constraints == nil || t.Constraints.TestCommand == "" {
		return nil
	}

	cmdStr := t.Constraints.TestCommand
	slog.Info("orchestrator: running test command", "cmd", cmdStr, "workdir", workDir)

	testCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	cmd := exec.CommandContext(testCtx, "sh", "-c", cmdStr)
	if workDir != "" && workDir != "." {
		cmd.Dir = workDir
	}

	out, err := cmd.CombinedOutput()
	output := string(out)
	const maxOutput = 4096
	if len(output) > maxOutput {
		output = output[:maxOutput/2] + "\n...(truncated)...\n" + output[len(output)-maxOutput/2:]
	}

	passed := err == nil
	slog.Info("orchestrator: test command finished", "passed", passed, "output_len", len(output))

	return &testCommandResult{passed: &passed, output: output}
}

func resolveReviewWorkDir(t *task.Task) string {
	if t.Constraints != nil && t.Constraints.Extra != nil {
		if dir, ok := t.Constraints.Extra["work_dir"].(string); ok && dir != "" {
			return dir
		}
	}
	return "."
}

func formatCriteria(t *task.Task) string {
	if t.Constraints != nil && t.Constraints.SuccessCriteria != "" {
		return t.Constraints.SuccessCriteria
	}
	return "(no explicit criteria)"
}

func boolPtr(v bool) *bool { return &v }
