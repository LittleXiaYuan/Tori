package cogni

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"
)

// WorkflowDef describes a multi-step workflow declared in a Cogni.
type WorkflowDef struct {
	Name        string         `json:"name" yaml:"name"`
	Description string         `json:"description,omitempty" yaml:"description,omitempty"`
	Steps       []WorkflowStep `json:"steps" yaml:"steps"`
}

// WorkflowStep is a single step in a workflow.
type WorkflowStep struct {
	Name      string         `json:"name,omitempty" yaml:"name,omitempty"`
	Skill     string         `json:"skill" yaml:"skill"`
	Args      map[string]any `json:"args,omitempty" yaml:"args,omitempty"`
	Output    string         `json:"output,omitempty" yaml:"output,omitempty"`
	Condition string         `json:"condition,omitempty" yaml:"condition,omitempty"`
	Timeout   string         `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	OnError   string         `json:"on_error,omitempty" yaml:"on_error,omitempty"` // continue | abort (default: abort)
}

// WorkflowResult is the output of a complete workflow execution.
type WorkflowResult struct {
	WorkflowName string         `json:"workflow_name"`
	Success      bool           `json:"success"`
	Outputs      map[string]any `json:"outputs"`
	StepResults  []StepResult   `json:"step_results"`
	Duration     time.Duration  `json:"duration"`
	Error        string         `json:"error,omitempty"`
}

// StepResult tracks the outcome of a single workflow step.
type StepResult struct {
	StepIndex int            `json:"step_index"`
	StepName  string         `json:"step_name"`
	Skill     string         `json:"skill"`
	Skipped   bool           `json:"skipped,omitempty"`
	Output    any            `json:"output,omitempty"`
	Duration  time.Duration  `json:"duration"`
	Error     string         `json:"error,omitempty"`
}

// SkillExecutor is the callback the workflow engine uses to run a skill.
// The host provides this — it bridges to the skill registry / planner.
type SkillExecutor func(ctx context.Context, skillName string, args map[string]any) (any, error)

// WorkflowEngine runs multi-step workflows defined in Cogni declarations.
type WorkflowEngine struct {
	executor SkillExecutor
}

func NewWorkflowEngine(executor SkillExecutor) *WorkflowEngine {
	return &WorkflowEngine{executor: executor}
}

// Run executes a workflow with the given input variables.
func (we *WorkflowEngine) Run(ctx context.Context, wf WorkflowDef, input map[string]any) *WorkflowResult {
	start := time.Now()
	result := &WorkflowResult{
		WorkflowName: wf.Name,
		Outputs:      make(map[string]any),
	}

	if we.executor == nil {
		result.Error = "workflow engine: no skill executor configured"
		return result
	}

	if input == nil {
		input = make(map[string]any)
	}

	// Variables available for interpolation: input.* + step outputs
	vars := map[string]any{
		"input": input,
	}

	slog.Info("workflow: starting",
		"name", wf.Name,
		"steps", len(wf.Steps),
	)

	for i, step := range wf.Steps {
		if ctx.Err() != nil {
			result.Error = fmt.Sprintf("workflow cancelled at step %d", i)
			result.StepResults = append(result.StepResults, StepResult{
				StepIndex: i,
				StepName:  stepName(step, i),
				Skill:     step.Skill,
				Error:     "context cancelled",
			})
			break
		}

		sr := we.runStep(ctx, step, i, vars)
		result.StepResults = append(result.StepResults, sr)

		if sr.Skipped {
			continue
		}

		if sr.Error != "" {
			onError := step.OnError
			if onError == "" {
				onError = "abort"
			}
			if onError == "abort" {
				result.Error = fmt.Sprintf("step %d (%s) failed: %s", i, stepName(step, i), sr.Error)
				break
			}
			slog.Warn("workflow: step failed, continuing",
				"step", i,
				"skill", step.Skill,
				"err", sr.Error,
			)
			continue
		}

		// Store step output in vars for subsequent steps
		if step.Output != "" && sr.Output != nil {
			vars[step.Output] = sr.Output
			result.Outputs[step.Output] = sr.Output
		}
	}

	result.Duration = time.Since(start)
	result.Success = result.Error == ""

	slog.Info("workflow: complete",
		"name", wf.Name,
		"success", result.Success,
		"steps_run", len(result.StepResults),
		"duration", result.Duration,
	)

	return result
}

func (we *WorkflowEngine) runStep(ctx context.Context, step WorkflowStep, index int, vars map[string]any) StepResult {
	name := stepName(step, index)
	start := time.Now()

	// Check condition
	if step.Condition != "" {
		if !evaluateCondition(step.Condition, vars) {
			slog.Debug("workflow: step skipped (condition false)", "step", name, "condition", step.Condition)
			return StepResult{
				StepIndex: index,
				StepName:  name,
				Skill:     step.Skill,
				Skipped:   true,
				Duration:  time.Since(start),
			}
		}
	}

	// Apply timeout if specified
	if step.Timeout != "" {
		d, err := time.ParseDuration(step.Timeout)
		if err != nil {
			slog.Warn("workflow: invalid timeout, using 60s default", "step", name, "timeout", step.Timeout, "err", err)
			d = 60 * time.Second
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, d)
		defer cancel()
	}

	// Interpolate args
	resolvedArgs := interpolateArgs(step.Args, vars)

	slog.Debug("workflow: running step",
		"step", name,
		"skill", step.Skill,
	)

	output, err := we.executor(ctx, step.Skill, resolvedArgs)

	sr := StepResult{
		StepIndex: index,
		StepName:  name,
		Skill:     step.Skill,
		Output:    output,
		Duration:  time.Since(start),
	}
	if err != nil {
		sr.Error = err.Error()
	}
	return sr
}

func stepName(step WorkflowStep, index int) string {
	if step.Name != "" {
		return step.Name
	}
	return fmt.Sprintf("step_%d_%s", index, step.Skill)
}

// interpolateArgs replaces ${var.path} references in string values with
// the corresponding variable from the vars map.
func interpolateArgs(args map[string]any, vars map[string]any) map[string]any {
	if args == nil {
		return nil
	}
	result := make(map[string]any, len(args))
	for k, v := range args {
		result[k] = interpolateValue(v, vars)
	}
	return result
}

var varPattern = regexp.MustCompile(`\$\{([^}]+)\}`)

func interpolateValue(v any, vars map[string]any) any {
	switch val := v.(type) {
	case string:
		return varPattern.ReplaceAllStringFunc(val, func(match string) string {
			path := match[2 : len(match)-1] // strip ${ and }
			if resolved, ok := resolveVarPath(path, vars); ok {
				return fmt.Sprintf("%v", resolved)
			}
			return match // keep original if not found
		})
	case map[string]any:
		return interpolateArgs(val, vars)
	case []any:
		out := make([]any, len(val))
		for i, item := range val {
			out[i] = interpolateValue(item, vars)
		}
		return out
	default:
		return v
	}
}

// resolveVarPath resolves dotted paths like "input.pr_number" or "diff_content".
func resolveVarPath(path string, vars map[string]any) (any, bool) {
	parts := strings.Split(path, ".")
	var current any = vars

	for _, part := range parts {
		part = strings.TrimSpace(part)
		switch m := current.(type) {
		case map[string]any:
			val, ok := m[part]
			if !ok {
				return nil, false
			}
			current = val
		default:
			return nil, false
		}
	}
	return current, true
}

// evaluateCondition checks if a simple condition expression is truthy.
// Supports:
//   - "${var}" — checks if var exists and is non-empty/non-zero
//   - "${var} == value" — equality check
//   - "${var} != value" — inequality check
//   - "true" / "false" — literal booleans
func evaluateCondition(condition string, vars map[string]any) bool {
	condition = strings.TrimSpace(condition)

	if condition == "true" {
		return true
	}
	if condition == "false" {
		return false
	}

	// Check for == or != operators
	for _, op := range []string{"!=", "=="} {
		if parts := strings.SplitN(condition, op, 2); len(parts) == 2 {
			left := strings.TrimSpace(parts[0])
			right := strings.TrimSpace(parts[1])

			leftVal := resolveConditionValue(left, vars)
			rightVal := resolveConditionValue(right, vars)

			if op == "==" {
				return fmt.Sprintf("%v", leftVal) == fmt.Sprintf("%v", rightVal)
			}
			return fmt.Sprintf("%v", leftVal) != fmt.Sprintf("%v", rightVal)
		}
	}

	// Simple truthy check: ${var} or ${var.path}
	resolved := varPattern.ReplaceAllStringFunc(condition, func(match string) string {
		path := match[2 : len(match)-1]
		if val, ok := resolveVarPath(path, vars); ok {
			return fmt.Sprintf("%v", val)
		}
		return ""
	})

	resolved = strings.TrimSpace(resolved)
	return resolved != "" && resolved != "0" && resolved != "false" && resolved != "<nil>" && resolved != "nil"
}

func resolveConditionValue(s string, vars map[string]any) any {
	s = strings.TrimSpace(s)
	if varPattern.MatchString(s) {
		path := s[2 : len(s)-1]
		if val, ok := resolveVarPath(path, vars); ok {
			return val
		}
	}
	// Strip quotes for literal strings
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}
