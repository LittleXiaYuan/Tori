package planner

import (
	"context"
	"strings"
	"testing"
	"time"

	"yunque-agent/internal/observe"
)

func TestRuntimeRequestContractsStepCallbackRoundTrip(t *testing.T) {
	t.Parallel()

	if StepCallbackFromCtx(context.Background()) != nil {
		t.Fatal("expected empty context to have no step callback")
	}

	seen := make(chan observe.AgentEvent, 1)
	cb := StepCallback(func(event observe.AgentEvent) {
		seen <- event
	})

	ctx := WithStepCallback(context.Background(), cb)
	got := StepCallbackFromCtx(ctx)
	if got == nil {
		t.Fatal("expected step callback from context")
	}

	want := observe.AgentEvent{Domain: observe.DomainPlanner, Type: string(StepEventThinking), Summary: "thinking"}
	got(want)

	select {
	case event := <-seen:
		if event.Domain != want.Domain || event.Type != want.Type || event.Summary != want.Summary {
			t.Fatalf("unexpected callback event: %#v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("callback was not invoked")
	}
}

func TestRuntimeRequestContractsExecutionSummary(t *testing.T) {
	t.Parallel()

	if (*PlanResult)(nil).ExecutionSummary() != "" {
		t.Fatal("expected nil result summary to be empty")
	}

	result := &PlanResult{Plan: []PlanStep{
		{Skill: "web_search", Status: StepDone, Result: strings.Repeat("ok", 80)},
		{Skill: "use_skill", Status: StepFailed, Error: "timeout while calling tool"},
	}}

	summary := result.ExecutionSummary()
	for _, want := range []string{"[执行记录]", "web_search", "use_skill", "失败"} {
		if !strings.Contains(summary, want) {
			t.Fatalf("summary %q missing %q", summary, want)
		}
	}
	if strings.Contains(summary, strings.Repeat("ok", 80)) {
		t.Fatalf("expected long step result to be truncated, got %q", summary)
	}
}
