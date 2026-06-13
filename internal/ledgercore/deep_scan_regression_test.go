package ledger_test

// Regression tests for the deep-scan fixes (commit e4b55801): replay/
// materialized-view consistency, checkpoint cleanup edge cases, and IVF
// cross-tenant leakage.

import (
	"context"
	"testing"

	"yunque-agent/internal/ledgercore"
)

// retrying→running used to emit task.resumed, which replays to `ready` while
// the materialized row says `running` — replay and view diverged. It now emits
// a dedicated task.retry_succeeded (replays to `running`, distinguishable from a
// first start), and replay must keep the FIRST StartedAt set by task.started.
func TestRetryRestartReplaysToRunning(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	task, err := ldg.Tasks.CreateTask(ctx, "retry replay", ledger.TaskTypeGoal, "t1")
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	for _, to := range []ledger.TaskStatus{ledger.TaskReady, ledger.TaskRunning, ledger.TaskRetrying, ledger.TaskRunning} {
		if err := ldg.Tasks.Transition(ctx, task.ID, to, "runtime", nil); err != nil {
			t.Fatalf("Transition to %s: %v", to, err)
		}
	}

	events, err := ldg.Events.ListAll(ctx, task.ID)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	last := events[len(events)-1]
	if last.Kind != ledger.EventTaskRetrySucceeded {
		t.Fatalf("retrying→running emitted %q, want %q", last.Kind, ledger.EventTaskRetrySucceeded)
	}

	materialized, err := ldg.Tasks.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	replayed, err := ldg.Events.Replay(ctx, task.ID)
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}

	if replayed.Status != ledger.TaskRunning || materialized.Status != ledger.TaskRunning {
		t.Fatalf("status diverged: replayed=%s materialized=%s, want both running", replayed.Status, materialized.Status)
	}
	if replayed.Version != materialized.Version {
		t.Fatalf("version diverged: replayed=%d materialized=%d", replayed.Version, materialized.Version)
	}
	if replayed.RetryCount != materialized.RetryCount {
		t.Fatalf("retry count diverged: replayed=%d materialized=%d", replayed.RetryCount, materialized.RetryCount)
	}
	if replayed.StartedAt == nil || materialized.StartedAt == nil {
		t.Fatal("StartedAt must be set on both views")
	}
	// First start wins on both sides — the retry restart must not move it.
	if !replayed.StartedAt.Equal(*materialized.StartedAt) {
		t.Fatalf("StartedAt diverged: replayed=%v materialized=%v", replayed.StartedAt, materialized.StartedAt)
	}
}

// TestRetrySucceededEventKind pins the FSM mapping: retrying → running must
// resolve to the dedicated task.retry_succeeded kind, not task.started/resumed.
func TestRetrySucceededEventKind(t *testing.T) {
	kind, err := ledger.EventKindForTransition(ledger.TaskRetrying, ledger.TaskRunning)
	if err != nil {
		t.Fatalf("EventKindForTransition: %v", err)
	}
	if kind != ledger.EventTaskRetrySucceeded {
		t.Fatalf("got %s, want %s", kind, ledger.EventTaskRetrySucceeded)
	}
}

// task.created used to carry only goal/type/tenant/agent, so replay lost
// metadata, priority, max-retries, input, and parent linkage (INV-1).
func TestReplayReconstructsCreationFields(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	parent, err := ldg.Tasks.CreateTask(ctx, "parent", ledger.TaskTypeGoal, "t1")
	if err != nil {
		t.Fatalf("CreateTask parent: %v", err)
	}
	task, err := ldg.Tasks.CreateTask(ctx, "rich create", ledger.TaskTypeGoal, "t1",
		ledger.WithUserID("user-9"),
		ledger.WithInput(ledger.JSON(`{"q":"hello"}`)),
		ledger.WithMetadata(ledger.JSON(`{"origin":"test"}`)),
		ledger.WithPriority(7),
		ledger.WithMaxRetries(5),
		ledger.WithParentTask(parent.ID),
	)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	replayed, err := ldg.Events.Replay(ctx, task.ID)
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}

	if replayed.UserID != "user-9" {
		t.Fatalf("UserID = %q, want user-9", replayed.UserID)
	}
	if string(replayed.Input) != `{"q":"hello"}` {
		t.Fatalf("Input = %s, want original input", replayed.Input)
	}
	if string(replayed.Metadata) != `{"origin":"test"}` {
		t.Fatalf("Metadata = %s, want original metadata", replayed.Metadata)
	}
	if replayed.Priority != 7 {
		t.Fatalf("Priority = %d, want 7", replayed.Priority)
	}
	if replayed.MaxRetries != 5 {
		t.Fatalf("MaxRetries = %d, want 5", replayed.MaxRetries)
	}
	if replayed.ParentTaskID == nil || *replayed.ParentTaskID != parent.ID {
		t.Fatalf("ParentTaskID = %v, want %s", replayed.ParentTaskID, parent.ID)
	}
	if replayed.Version != task.Version {
		t.Fatalf("fresh-create version: replayed=%d materialized=%d", replayed.Version, task.Version)
	}
}

// Reasoning/step events never touch the materialized task row, so replaying
// them must not advance the task version either — otherwise replayed tasks
// break optimistic locking.
func TestReplayVersionIgnoresReasoningEvents(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	task, err := ldg.Tasks.CreateTask(ctx, "version alignment", ledger.TaskTypeGoal, "t1")
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if err := ldg.Tasks.Transition(ctx, task.ID, ledger.TaskReady, "runtime", nil); err != nil {
		t.Fatalf("Transition: %v", err)
	}

	tracer := ldg.Reasoning(task.ID, "planner")
	tracer.Think(ctx, "thinking hard", nil)
	tracer.Decide(ctx, "go", "reason", 0.9, nil)

	materialized, err := ldg.Tasks.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	replayed, err := ldg.Events.Replay(ctx, task.ID)
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	if replayed.Version != materialized.Version {
		t.Fatalf("version diverged with reasoning events: replayed=%d materialized=%d", replayed.Version, materialized.Version)
	}
	if replayed.Status != materialized.Status {
		t.Fatalf("status diverged: replayed=%s materialized=%s", replayed.Status, materialized.Status)
	}
}

// Cleanup(keepCount=0) used to panic with index -1; it must delete every
// checkpoint instead.
func TestCheckpointCleanupZeroKeepCount(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	task, err := ldg.Tasks.CreateTask(ctx, "cleanup zero", ledger.TaskTypeGoal, "t1")
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	for i := 0; i < 2; i++ {
		if _, err := ldg.Checkpoints.Save(ctx, task.ID, i, ledger.JSON(`{}`), "manual"); err != nil {
			t.Fatalf("Save checkpoint %d: %v", i, err)
		}
	}

	if err := ldg.Checkpoints.Cleanup(ctx, task.ID, 0); err != nil {
		t.Fatalf("Cleanup(0): %v", err)
	}
	cps, err := ldg.Backend().ListCheckpoints(ctx, task.ID, 10)
	if err != nil {
		t.Fatalf("ListCheckpoints: %v", err)
	}
	if len(cps) != 0 {
		t.Fatalf("Cleanup(0) left %d checkpoints, want 0", len(cps))
	}
}

// The IVF index spans all tenants. searchIVF used to return raw index hits,
// leaking other tenants' memories (and ignoring kind filters).
func TestSearchIVFFiltersTenantAndKind(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	put := func(tenant string, kind ledger.MemoryKind, key string, vec []float32) *ledger.MemoryEntry {
		t.Helper()
		m := &ledger.MemoryEntry{
			TenantID: tenant, Kind: kind, Key: key,
			Content: "content " + key, Source: "test", Confidence: 0.9,
		}
		if err := ldg.Memory.Put(ctx, m); err != nil {
			t.Fatalf("Put %s: %v", key, err)
		}
		if err := ldg.Vector.Put(ctx, m.ID, vec); err != nil {
			t.Fatalf("Vector.Put %s: %v", key, err)
		}
		return m
	}

	ldg.Vector.SetDimensions(4)
	ldg.Vector.EnableIVF(ledger.IVFConfig{NumClusters: 2, MinPointsToTrain: 4, MaxIterations: 5})

	// Tenant A corpus (trains the index). One entry sits very close to the
	// probe vector so the tenant-scoped query still has a legitimate hit.
	probe := []float32{1, 0, 0, 0}
	put("tenant-a", ledger.MemoryFact, "a-near", []float32{0.95, 0.05, 0, 0})
	put("tenant-a", ledger.MemoryFact, "a-1", []float32{0, 1, 0, 0})
	put("tenant-a", ledger.MemoryFact, "a-2", []float32{0, 0.9, 0.1, 0})
	put("tenant-a", ledger.MemoryPreference, "a-pref", []float32{0.9, 0.1, 0, 0})
	if err := ldg.Vector.TrainIVF(ctx, "tenant-a"); err != nil {
		t.Fatalf("TrainIVF: %v", err)
	}
	if clusters, _ := ldg.Vector.IVFStats(); clusters == 0 {
		t.Fatal("IVF index must be trained for this regression test")
	}

	// Tenant B's memory lands in the shared index post-training and matches
	// the probe vector exactly — the strongest possible leak candidate.
	leak := put("tenant-b", ledger.MemoryFact, "b-secret", probe)

	results, err := ldg.Vector.Search(ctx, ledger.VectorQuery{
		TenantID:  "tenant-a",
		Embedding: probe,
		Limit:     10,
		MinScore:  0.1,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("tenant-scoped search should still return tenant-a hits")
	}
	for _, r := range results {
		if r.Entry.ID == leak.ID || r.Entry.TenantID != "tenant-a" {
			t.Fatalf("cross-tenant leak: got entry %s of tenant %s", r.Entry.Key, r.Entry.TenantID)
		}
	}

	// Kind filter: preferences only.
	results, err = ldg.Vector.Search(ctx, ledger.VectorQuery{
		TenantID:  "tenant-a",
		Embedding: probe,
		Kinds:     []ledger.MemoryKind{ledger.MemoryPreference},
		Limit:     10,
		MinScore:  0.1,
	})
	if err != nil {
		t.Fatalf("Search with kinds: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("kind-filtered search should return the preference entry")
	}
	for _, r := range results {
		if r.Entry.Kind != ledger.MemoryPreference {
			t.Fatalf("kind filter leaked %s entry %s", r.Entry.Kind, r.Entry.Key)
		}
	}
}
