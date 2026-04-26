package cogni

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func mockExecutor(results map[string]any) SkillExecutor {
	return func(ctx context.Context, skill string, args map[string]any) (any, error) {
		if val, ok := results[skill]; ok {
			return val, nil
		}
		return nil, fmt.Errorf("skill %q not found", skill)
	}
}

func TestWorkflowEngine_SimpleChain(t *testing.T) {
	engine := NewWorkflowEngine(mockExecutor(map[string]any{
		"get_data":    map[string]any{"items": []string{"a", "b"}},
		"process":     "processed result",
	}))

	wf := WorkflowDef{
		Name: "test-chain",
		Steps: []WorkflowStep{
			{Skill: "get_data", Output: "data"},
			{Skill: "process", Args: map[string]any{"input": "${data}"}, Output: "result"},
		},
	}

	result := engine.Run(context.Background(), wf, nil)
	if !result.Success {
		t.Fatalf("workflow failed: %s", result.Error)
	}
	if len(result.StepResults) != 2 {
		t.Errorf("step results = %d, want 2", len(result.StepResults))
	}
	if result.Outputs["result"] != "processed result" {
		t.Errorf("result = %v", result.Outputs["result"])
	}
}

func TestWorkflowEngine_VariableInterpolation(t *testing.T) {
	var capturedArgs map[string]any
	engine := NewWorkflowEngine(func(ctx context.Context, skill string, args map[string]any) (any, error) {
		capturedArgs = args
		return "ok", nil
	})

	wf := WorkflowDef{
		Name: "interpolation-test",
		Steps: []WorkflowStep{
			{
				Skill: "do_thing",
				Args: map[string]any{
					"number": "${input.pr_number}",
					"label":  "PR #${input.pr_number}",
				},
			},
		},
	}

	engine.Run(context.Background(), wf, map[string]any{"pr_number": 42})

	if capturedArgs["number"] != "42" {
		t.Errorf("number = %v", capturedArgs["number"])
	}
	if capturedArgs["label"] != "PR #42" {
		t.Errorf("label = %v", capturedArgs["label"])
	}
}

func TestWorkflowEngine_Condition_Skip(t *testing.T) {
	called := false
	engine := NewWorkflowEngine(func(ctx context.Context, skill string, args map[string]any) (any, error) {
		if skill == "skip_me" {
			called = true
		}
		return "ok", nil
	})

	wf := WorkflowDef{
		Name: "cond-test",
		Steps: []WorkflowStep{
			{Skill: "skip_me", Condition: "false"},
		},
	}

	result := engine.Run(context.Background(), wf, nil)
	if !result.Success {
		t.Fatalf("workflow failed: %s", result.Error)
	}
	if called {
		t.Error("step should have been skipped")
	}
	if !result.StepResults[0].Skipped {
		t.Error("step should be marked as skipped")
	}
}

func TestWorkflowEngine_Condition_Variable(t *testing.T) {
	engine := NewWorkflowEngine(mockExecutor(map[string]any{
		"step1": map[string]any{"has_issues": true},
		"step2": "fixed",
	}))

	wf := WorkflowDef{
		Name: "cond-var",
		Steps: []WorkflowStep{
			{Skill: "step1", Output: "check_result"},
			{Skill: "step2", Condition: "${check_result.has_issues}"},
		},
	}

	result := engine.Run(context.Background(), wf, nil)
	if !result.Success {
		t.Fatalf("workflow failed: %s", result.Error)
	}
	if result.StepResults[1].Skipped {
		t.Error("step2 should NOT be skipped when condition is true")
	}
}

func TestWorkflowEngine_StepFailure_Abort(t *testing.T) {
	engine := NewWorkflowEngine(func(ctx context.Context, skill string, args map[string]any) (any, error) {
		if skill == "failing" {
			return nil, fmt.Errorf("boom")
		}
		return "ok", nil
	})

	wf := WorkflowDef{
		Name: "fail-abort",
		Steps: []WorkflowStep{
			{Skill: "failing"},
			{Skill: "should_not_run"},
		},
	}

	result := engine.Run(context.Background(), wf, nil)
	if result.Success {
		t.Error("workflow should have failed")
	}
	if len(result.StepResults) != 1 {
		t.Errorf("should stop at first failure, got %d steps", len(result.StepResults))
	}
}

func TestWorkflowEngine_StepFailure_Continue(t *testing.T) {
	engine := NewWorkflowEngine(func(ctx context.Context, skill string, args map[string]any) (any, error) {
		if skill == "failing" {
			return nil, fmt.Errorf("boom")
		}
		return "ok", nil
	})

	wf := WorkflowDef{
		Name: "fail-continue",
		Steps: []WorkflowStep{
			{Skill: "failing", OnError: "continue"},
			{Skill: "second", Output: "result"},
		},
	}

	result := engine.Run(context.Background(), wf, nil)
	if !result.Success {
		t.Errorf("workflow should succeed with on_error=continue, got: %s", result.Error)
	}
	if len(result.StepResults) != 2 {
		t.Errorf("should run both steps, got %d", len(result.StepResults))
	}
}

func TestWorkflowEngine_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	engine := NewWorkflowEngine(func(ctx context.Context, skill string, args map[string]any) (any, error) {
		return "ok", nil
	})

	wf := WorkflowDef{
		Name:  "cancel-test",
		Steps: []WorkflowStep{{Skill: "anything"}},
	}

	result := engine.Run(ctx, wf, nil)
	if result.Success {
		t.Error("should fail on cancelled context")
	}
}

func TestWorkflowEngine_NoExecutor(t *testing.T) {
	engine := NewWorkflowEngine(nil)
	result := engine.Run(context.Background(), WorkflowDef{Name: "test"}, nil)
	if result.Success {
		t.Error("should fail without executor")
	}
}

func TestWorkflowEngine_EmptyWorkflow(t *testing.T) {
	engine := NewWorkflowEngine(mockExecutor(nil))
	result := engine.Run(context.Background(), WorkflowDef{Name: "empty"}, nil)
	if !result.Success {
		t.Errorf("empty workflow should succeed, got: %s", result.Error)
	}
}

func TestWorkflowEngine_StepTimeout(t *testing.T) {
	engine := NewWorkflowEngine(func(ctx context.Context, skill string, args map[string]any) (any, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(5 * time.Second):
			return "ok", nil
		}
	})

	wf := WorkflowDef{
		Name: "timeout-test",
		Steps: []WorkflowStep{
			{Skill: "slow", Timeout: "50ms"},
		},
	}

	result := engine.Run(context.Background(), wf, nil)
	if result.Success {
		t.Error("should fail on timeout")
	}
}

func TestInterpolateValue(t *testing.T) {
	vars := map[string]any{
		"input":  map[string]any{"name": "world"},
		"result": "hello",
	}

	tests := []struct {
		input any
		want  string
	}{
		{"${input.name}", "world"},
		{"Hello ${input.name}!", "Hello world!"},
		{"${result}", "hello"},
		{"${missing}", "${missing}"},
		{42, ""},
	}
	for _, tt := range tests {
		got := interpolateValue(tt.input, vars)
		if s, ok := got.(string); ok && s != tt.want {
			t.Errorf("interpolate(%v) = %q, want %q", tt.input, s, tt.want)
		}
	}
}

func TestEvaluateCondition(t *testing.T) {
	vars := map[string]any{
		"data":  map[string]any{"count": 5, "empty": false},
		"input": map[string]any{"mode": "fast"},
	}

	tests := []struct {
		cond string
		want bool
	}{
		{"true", true},
		{"false", false},
		{"${data.count}", true},
		{"${data.empty}", false},
		{"${missing}", false},
		{"${input.mode} == fast", true},
		{"${input.mode} != fast", false},
		{"${input.mode} == slow", false},
	}
	for _, tt := range tests {
		if got := evaluateCondition(tt.cond, vars); got != tt.want {
			t.Errorf("evaluateCondition(%q) = %v, want %v", tt.cond, got, tt.want)
		}
	}
}

func TestResolveVarPath(t *testing.T) {
	vars := map[string]any{
		"input": map[string]any{
			"nested": map[string]any{"deep": "value"},
		},
		"top": "level",
	}

	tests := []struct {
		path string
		want any
		ok   bool
	}{
		{"top", "level", true},
		{"input.nested.deep", "value", true},
		{"missing", nil, false},
		{"input.missing", nil, false},
	}
	for _, tt := range tests {
		got, ok := resolveVarPath(tt.path, vars)
		if ok != tt.ok || (ok && fmt.Sprintf("%v", got) != fmt.Sprintf("%v", tt.want)) {
			t.Errorf("resolveVarPath(%q) = (%v, %v), want (%v, %v)", tt.path, got, ok, tt.want, tt.ok)
		}
	}
}
