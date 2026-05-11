package rlsched

import (
	"testing"
	"time"

	"yunque-agent/internal/agentcore/task"
)

func TestTaskFeedbackRecordsCompletedTaskUpdate(t *testing.T) {
	store := task.NewJSONStore(t.TempDir())
	started := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	created, err := store.Create(task.CreateRequest{
		Title:       "planner route",
		Description: "verify planner scheduling feedback",
		TenantID:    "t1",
		Constraints: &task.TaskConstraints{
			Priority: "high",
		},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	created.Status = task.StatusCompleted
	created.StartedAt = &started
	created.FinishedAt = ptrTime(started.Add(2 * time.Minute))
	created.Steps = []task.Step{{ID: 1, Status: task.StepDone}}
	if err := store.Update(created); err != nil {
		t.Fatalf("Update: %v", err)
	}

	learner := NewQLearner(DefaultQLearnerConfig([]string{"priority_high", "priority_normal", "priority_low", "defer"}))
	feedback := NewTaskFeedback(learner, store)
	feedback.Now = func() time.Time { return started.Add(2 * time.Minute) }

	if !feedback.Record(EventTaskCompleted, created.ID) {
		t.Fatal("expected feedback record to update learner")
	}
	if learner.Episodes() != 1 {
		t.Fatalf("expected one learning episode, got %d", learner.Episodes())
	}
	state := EncodeTaskState(store, created, started.Add(2*time.Minute))
	if got := learner.QValue(state, "priority_high"); got <= 0 {
		t.Fatalf("expected positive Q value for completed high-priority task, got %.3f", got)
	}
}

func TestTaskFeedbackRecordsFailurePenalty(t *testing.T) {
	store := task.NewJSONStore(t.TempDir())
	created, err := store.Create(task.CreateRequest{
		Title:       "bad task",
		Description: "task that fails",
		TenantID:    "t1",
		Constraints: &task.TaskConstraints{
			Priority: "low",
		},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	created.Status = task.StatusFailed
	created.Steps = []task.Step{{ID: 1, Status: task.StepFailed, RetryCount: 2}}
	if err := store.Update(created); err != nil {
		t.Fatalf("Update: %v", err)
	}

	learner := NewQLearner(DefaultQLearnerConfig([]string{"priority_high", "priority_normal", "priority_low", "defer"}))
	feedback := NewTaskFeedback(learner, store)

	if !feedback.Record(EventTaskFailed, created.ID) {
		t.Fatal("expected failed task to update learner")
	}
	state := EncodeTaskState(store, created, time.Now())
	if got := learner.QValue(state, "priority_low"); got >= 0 {
		t.Fatalf("expected negative Q value for failed low-priority task, got %.3f", got)
	}
}

func ptrTime(t time.Time) *time.Time { return &t }
