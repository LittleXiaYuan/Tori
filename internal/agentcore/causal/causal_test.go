package causal

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ledger"
	lsqlite "ledger/backend/sqlite"
)

func newTestLedger(t *testing.T) *ledger.Ledger {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	b, err := lsqlite.New(dbPath)
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}

	ldg, err := ledger.Open(b)
	if err != nil {
		t.Fatalf("open ledger: %v", err)
	}
	t.Cleanup(func() { ldg.Close() })
	return ldg
}

func TestInferCausalityDecisionToAction(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	// Create task first
	task, err := ldg.Tasks.CreateTask(ctx, "Test task", ledger.TaskTypeGoal, "system")
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	taskID := task.ID

	// Decision → Step
	ldg.Events.Append(ctx, &ledger.Event{
		ID:        "evt-1",
		TaskID:    taskID,
		Kind:      ledger.EventReasoningDecision,
		Actor:     "planner",
		Payload:   ledger.MakePayload(map[string]interface{}{"decision": "call_api"}),
		CreatedAt: time.Now(),
	})
	ldg.Events.Append(ctx, &ledger.Event{
		ID:        "evt-2",
		TaskID:    taskID,
		Kind:      ledger.EventStepStarted,
		Actor:     "planner",
		Payload:   ledger.MakePayload(map[string]interface{}{"step": "call_api"}),
		CreatedAt: time.Now(),
	})

	ce := New(ldg)
	links, err := ce.InferCausality(ctx, taskID)
	if err != nil {
		t.Fatalf("InferCausality: %v", err)
	}
	if len(links) == 0 {
		t.Fatal("expected at least one causal link")
	}

	found := false
	for _, l := range links {
		if l.CauseKind == ledger.EventReasoningDecision && l.EffectKind == ledger.EventStepStarted {
			found = true
			if l.Strength < 0.8 {
				t.Errorf("expected high strength, got %v", l.Strength)
			}
		}
	}
	if !found {
		t.Error("expected Decision→Step link")
	}
}

func TestInferCausalityFailureToBacktrack(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	task, err := ldg.Tasks.CreateTask(ctx, "Test task", ledger.TaskTypeGoal, "system")
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	taskID := task.ID

	ldg.Events.Append(ctx, &ledger.Event{
		ID: "evt-1", TaskID: taskID, Kind: ledger.EventToolFailed,
		Actor: "tools", Payload: ledger.MakePayload(map[string]interface{}{}), CreatedAt: time.Now(),
	})
	ldg.Events.Append(ctx, &ledger.Event{
		ID: "evt-2", TaskID: taskID, Kind: ledger.EventReasoningBacktrack,
		Actor: "planner", Payload: ledger.MakePayload(map[string]interface{}{}), CreatedAt: time.Now(),
	})

	ce := New(ldg)
	links, err := ce.InferCausality(ctx, taskID)
	if err != nil {
		t.Fatalf("InferCausality: %v", err)
	}

	found := false
	for _, l := range links {
		if l.CauseKind == ledger.EventToolFailed && l.EffectKind == ledger.EventReasoningBacktrack {
			found = true
		}
	}
	if !found {
		t.Error("expected ToolFailed→Backtrack link")
	}
}

func TestBuildTimeline(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	task, err := ldg.Tasks.CreateTask(ctx, "Test task", ledger.TaskTypeGoal, "system")
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	taskID := task.ID

	t1 := time.Now()
	ldg.Events.Append(ctx, &ledger.Event{
		ID: "evt-1", TaskID: taskID, Kind: ledger.EventStepStarted,
		Actor: "p", Payload: ledger.MakePayload(nil), CreatedAt: t1,
	})
	ldg.Events.Append(ctx, &ledger.Event{
		ID: "evt-2", TaskID: taskID, Kind: ledger.EventStepStarted,
		Actor: "p", Payload: ledger.MakePayload(nil), CreatedAt: t1.Add(5 * time.Second),
	})

	ce := New(ldg)
	timeline, err := ce.BuildTimeline(ctx, taskID)
	if err != nil {
		t.Fatalf("BuildTimeline: %v", err)
	}
	// 3 events: task.created (auto) + 2 step events
	if len(timeline) < 2 {
		t.Fatalf("timeline len = %d, want >= 2", len(timeline))
	}
}

func TestFindRootCauseNoFailure(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	task, err := ldg.Tasks.CreateTask(ctx, "Test task", ledger.TaskTypeGoal, "system")
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	taskID := task.ID

	// Only success events
	ldg.Events.Append(ctx, &ledger.Event{
		ID: "evt-1", TaskID: taskID, Kind: ledger.EventStepStarted,
		Actor: "p", Payload: ledger.MakePayload(nil), CreatedAt: time.Now(),
	})

	ce := New(ldg)
	_, err = ce.FindRootCause(ctx, taskID)
	if err == nil {
		t.Error("expected error for no failure event")
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
