package ledger_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"yunque-agent/internal/ledgercore"
)

func TestExportEmpty(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	var buf bytes.Buffer
	cfg := ledger.ExportConfig{
		TenantID: "t1",
		Format:   ledger.FormatAlpaca,
	}

	result, err := ldg.ExportTrainingData(ctx, &buf, cfg)
	if err != nil {
		t.Fatalf("ExportTrainingData: %v", err)
	}
	if result.SamplesWritten != 0 {
		t.Errorf("expected 0 samples from empty ledger, got %d", result.SamplesWritten)
	}
}

func TestExportWithTask(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	task, err := ldg.Tasks.CreateTask(ctx, "Generate report from data", ledger.TaskTypeGoal, "t1")
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	ldg.Tasks.Transition(ctx, task.ID, ledger.TaskReady, "planner", nil)
	ldg.Tasks.Transition(ctx, task.ID, ledger.TaskRunning, "runner", nil)

	// Add step events
	for i := 0; i < 3; i++ {
		ldg.Events.Append(ctx, &ledger.Event{
			TaskID:    task.ID,
			Kind:      ledger.EventStepCompleted,
			Actor:     "runner",
			Payload:   []byte(`{"step": "analysis"}`),
			CreatedAt: time.Now(),
		})
	}

	// Add reflection event with score
	ldg.Events.Append(ctx, &ledger.Event{
		TaskID:    task.ID,
		Kind:      ledger.EventReasoningReflect,
		Actor:     "reflector",
		Payload:   []byte(`{"score": 0.85, "reflection": "Good execution"}`),
		CreatedAt: time.Now(),
	})

	ldg.Tasks.Transition(ctx, task.ID, ledger.TaskCompleted, "runner", nil)

	var buf bytes.Buffer
	cfg := ledger.ExportConfig{
		TenantID: "t1",
		Format:   ledger.FormatAlpaca,
		MinScore: 0.5,
		MinSteps: 1,
	}

	result, err := ldg.ExportTrainingData(ctx, &buf, cfg)
	if err != nil {
		t.Fatalf("ExportTrainingData: %v", err)
	}
	t.Logf("export result: tasks=%d qualified=%d samples=%d", result.TotalTasks, result.QualifiedTasks, result.SamplesWritten)
}
