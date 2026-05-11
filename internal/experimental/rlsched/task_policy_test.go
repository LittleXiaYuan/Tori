package rlsched

import (
	"testing"
	"time"

	"yunque-agent/internal/agentcore/task"
)

func TestPolicyStoreAppliesLearnedPriorityWhenUnset(t *testing.T) {
	base := task.NewJSONStore(t.TempDir())
	learner := NewQLearner(DefaultQLearnerConfig([]string{"priority_high", "priority_normal", "priority_low", "defer"}))
	now := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	state := EncodeTaskState(base, &task.Task{
		TenantID:    "t1",
		Constraints: &task.TaskConstraints{},
	}, now)
	learner.Update(state, "defer", 1, state)
	learner.epsilon = 0

	store := NewPolicyStore(base, learner)
	store.Now = func() time.Time { return now }
	created, err := store.Create(task.CreateRequest{
		Description: "background clean up",
		TenantID:    "t1",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Constraints == nil {
		t.Fatal("expected constraints to be initialized")
	}
	if created.Constraints.Priority != "low" {
		t.Fatalf("expected defer action to map to low priority, got %q", created.Constraints.Priority)
	}
	if got := created.Constraints.Extra[MetaPolicyAction]; got != "defer" {
		t.Fatalf("expected learned defer action metadata, got %#v", got)
	}
	if got := created.Constraints.Extra[MetaPolicyApplied]; got != true {
		t.Fatalf("expected policy applied metadata, got %#v", got)
	}
}

func TestPolicyStorePreservesExplicitPriorityButRecordsAction(t *testing.T) {
	base := task.NewJSONStore(t.TempDir())
	learner := NewQLearner(DefaultQLearnerConfig([]string{"priority_high", "priority_normal", "priority_low", "defer"}))
	now := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	state := EncodeTaskState(base, &task.Task{
		TenantID: "t1",
		Constraints: &task.TaskConstraints{
			Priority: "high",
		},
	}, now)
	learner.Update(state, "defer", 1, state)
	learner.epsilon = 0

	store := NewPolicyStore(base, learner)
	store.Now = func() time.Time { return now }
	created, err := store.Create(task.CreateRequest{
		Description: "urgent user request",
		TenantID:    "t1",
		Constraints: &task.TaskConstraints{
			Priority: "high",
		},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Constraints.Priority != "high" {
		t.Fatalf("explicit priority should be preserved, got %q", created.Constraints.Priority)
	}
	if got := TaskSchedulingAction(created); got != "defer" {
		t.Fatalf("feedback should use learned action metadata, got %q", got)
	}
	if got := created.Constraints.Extra[MetaPolicyApplied]; got != false {
		t.Fatalf("expected policy-applied=false for explicit priority, got %#v", got)
	}
}
