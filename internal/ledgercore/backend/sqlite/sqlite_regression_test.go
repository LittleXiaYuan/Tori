package sqlite

// Regression tests for the deep-scan fixes (commit e4b55801): data loss on
// memory upsert, sync seq-cache divergence, replay ordering, LIKE wildcard
// injection, offset paging, and transactional task deletion.

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"yunque-agent/internal/ledgercore"
)

func newTestBackend(t *testing.T) *Backend {
	t.Helper()
	b, err := New(filepath.Join(t.TempDir(), "ledger.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { b.Close() })
	if err := b.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return b
}

func testMemory(id, tenant, content string, updatedAt time.Time) *ledger.MemoryEntry {
	return &ledger.MemoryEntry{
		ID:         id,
		TenantID:   tenant,
		Kind:       ledger.MemoryFact,
		Key:        id,
		Content:    content,
		Source:     "test",
		Confidence: 0.9,
		Metadata:   ledger.JSON("{}"),
		CreatedAt:  updatedAt,
		UpdatedAt:  updatedAt,
	}
}

// PutMemory used INSERT OR REPLACE, whose delete-then-insert cascaded into the
// embeddings table and silently dropped the vector on every memory update.
func TestPutMemoryUpsertPreservesEmbedding(t *testing.T) {
	b := newTestBackend(t)
	ctx := context.Background()

	now := time.Now()
	if err := b.PutMemory(ctx, testMemory("mem-upsert", "t1", "v1 content", now)); err != nil {
		t.Fatalf("PutMemory: %v", err)
	}
	vec := []float32{1, 0, 0, 0}
	if err := b.PutEmbedding(ctx, "mem-upsert", vec); err != nil {
		t.Fatalf("PutEmbedding: %v", err)
	}

	// Update the same memory — the embedding must survive.
	if err := b.PutMemory(ctx, testMemory("mem-upsert", "t1", "v2 content", now.Add(time.Second))); err != nil {
		t.Fatalf("PutMemory update: %v", err)
	}

	results, err := b.SearchByVector(ctx, ledger.VectorQuery{
		TenantID:  "t1",
		Embedding: vec,
		Limit:     5,
		MinScore:  0.5,
	})
	if err != nil {
		t.Fatalf("SearchByVector: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("embedding lost after memory update: got %d vector hits, want 1", len(results))
	}
	if results[0].Entry.Content != "v2 content" {
		t.Fatalf("memory content = %q, want updated v2 content", results[0].Entry.Content)
	}
}

// SearchMemories ignored Offset, so batch consumers (memory decay) looped over
// the same first page forever.
func TestSearchMemoriesOffsetPaging(t *testing.T) {
	b := newTestBackend(t)
	ctx := context.Background()

	now := time.Now()
	ids := []string{"m-0", "m-1", "m-2", "m-3", "m-4"}
	for i, id := range ids {
		// Distinct updated_at so ORDER BY updated_at DESC is deterministic.
		if err := b.PutMemory(ctx, testMemory(id, "t1", "content "+id, now.Add(-time.Duration(i)*time.Minute))); err != nil {
			t.Fatalf("PutMemory %s: %v", id, err)
		}
	}

	seen := map[string]bool{}
	for offset := 0; offset < len(ids); offset += 2 {
		page, err := b.SearchMemories(ctx, ledger.MemoryQuery{TenantID: "t1", Limit: 2, Offset: offset})
		if err != nil {
			t.Fatalf("SearchMemories offset %d: %v", offset, err)
		}
		for _, m := range page {
			if seen[m.ID] {
				t.Fatalf("memory %s returned twice across pages (offset not applied)", m.ID)
			}
			seen[m.ID] = true
		}
	}
	if len(seen) != len(ids) {
		t.Fatalf("paging covered %d of %d memories", len(seen), len(ids))
	}
}

// User-supplied query text reached LIKE unescaped, so "%" and "_" acted as
// wildcards instead of literals.
func TestSearchMemoriesEscapesLikeWildcards(t *testing.T) {
	b := newTestBackend(t)
	ctx := context.Background()

	now := time.Now()
	if err := b.PutMemory(ctx, testMemory("m-percent", "t1", "progress: 100% done", now)); err != nil {
		t.Fatal(err)
	}
	if err := b.PutMemory(ctx, testMemory("m-plain", "t1", "progress: 100x done", now.Add(time.Second))); err != nil {
		t.Fatal(err)
	}

	results, err := b.SearchMemories(ctx, ledger.MemoryQuery{TenantID: "t1", Query: "100%", Limit: 10})
	if err != nil {
		t.Fatalf("SearchMemories: %v", err)
	}
	if len(results) != 1 || results[0].ID != "m-percent" {
		ids := make([]string, 0, len(results))
		for _, m := range results {
			ids = append(ids, m.ID)
		}
		t.Fatalf(`query "100%%" matched %v, want only m-percent (literal %%)`, ids)
	}

	if err := b.PutMemory(ctx, testMemory("m-underscore", "t1", "key a_b set", now.Add(2*time.Second))); err != nil {
		t.Fatal(err)
	}
	if err := b.PutMemory(ctx, testMemory("m-noscore", "t1", "key axb set", now.Add(3*time.Second))); err != nil {
		t.Fatal(err)
	}
	results, err = b.SearchMemories(ctx, ledger.MemoryQuery{TenantID: "t1", Query: "a_b", Limit: 10})
	if err != nil {
		t.Fatalf("SearchMemories: %v", err)
	}
	if len(results) != 1 || results[0].ID != "m-underscore" {
		t.Fatalf(`query "a_b" should match the literal underscore row only, got %d rows`, len(results))
	}
}

// Sync applies remote events with explicit seqs; the in-process seq cache was
// not advanced past them, so the next auto-seq append reused the same seq and
// hit the UNIQUE(task_id, seq) constraint.
func TestAppendEventExplicitSeqAdvancesAutoSeqCache(t *testing.T) {
	b := newTestBackend(t)
	ctx := context.Background()

	task := testTask("task-seq-cache")
	if err := b.CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	// Two auto-seq appends load and advance the cache to 2.
	for i := 0; i < 2; i++ {
		e := &ledger.Event{
			ID: "evt-auto-" + string(rune('a'+i)), TaskID: task.ID,
			Kind: ledger.EventTaskReady, Actor: "test",
			Payload: ledger.JSON("{}"), CreatedAt: time.Now(),
		}
		if err := b.AppendEvent(ctx, e); err != nil {
			t.Fatalf("auto append %d: %v", i, err)
		}
	}

	// A sync-applied event lands with explicit seq 3.
	synced := &ledger.Event{
		ID: "evt-synced", TaskID: task.ID, Seq: 3,
		Kind: ledger.EventTaskStarted, Actor: "sync",
		Payload: ledger.JSON("{}"), CreatedAt: time.Now(),
	}
	if err := b.AppendEvent(ctx, synced); err != nil {
		t.Fatalf("explicit-seq append: %v", err)
	}

	// The next auto-seq append must continue after the synced seq.
	next := &ledger.Event{
		ID: "evt-after-sync", TaskID: task.ID,
		Kind: ledger.EventTaskCompleted, Actor: "test",
		Payload: ledger.JSON("{}"), CreatedAt: time.Now(),
	}
	if err := b.AppendEvent(ctx, next); err != nil {
		t.Fatalf("auto append after synced seq must not hit UNIQUE conflict: %v", err)
	}
	if next.Seq != 4 {
		t.Fatalf("next auto seq = %d, want 4 (cache advanced past synced seq)", next.Seq)
	}
}

// Replay and sync depend on per-task seq order; created_at can be skewed for
// synced or caller-stamped events, so single-task queries must sort by seq.
func TestQueryEventsSingleTaskOrdersBySeq(t *testing.T) {
	b := newTestBackend(t)
	ctx := context.Background()

	task := testTask("task-event-order")
	if err := b.CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	base := time.Now()
	// Insert with explicit seqs whose timestamps run BACKWARDS.
	for i, seq := range []int64{1, 2, 3} {
		e := &ledger.Event{
			ID: "evt-" + string(rune('a'+i)), TaskID: task.ID, Seq: seq,
			Kind: ledger.EventTaskReady, Actor: "test",
			Payload:   ledger.JSON("{}"),
			CreatedAt: base.Add(-time.Duration(i) * time.Hour),
		}
		if err := b.AppendEvent(ctx, e); err != nil {
			t.Fatalf("AppendEvent seq %d: %v", seq, err)
		}
	}

	events, err := b.QueryEvents(ctx, ledger.EventQuery{TaskID: task.ID})
	if err != nil {
		t.Fatalf("QueryEvents: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("got %d events, want 3", len(events))
	}
	for i, e := range events {
		if e.Seq != int64(i+1) {
			t.Fatalf("event[%d].Seq = %d, want %d (single-task order must be seq, not created_at)", i, e.Seq, i+1)
		}
	}
}

// DeleteTask used to be a fake delete; now it must remove the task and every
// dependent row transactionally, and reset the seq cache for ID reuse.
func TestDeleteTaskRemovesDependentsAndResetsSeqCache(t *testing.T) {
	b := newTestBackend(t)
	ctx := context.Background()

	task := testTask("task-delete")
	other := testTask("task-delete-other")
	if err := b.CreateTask(ctx, task); err != nil {
		t.Fatal(err)
	}
	if err := b.CreateTask(ctx, other); err != nil {
		t.Fatal(err)
	}

	evt := &ledger.Event{
		ID: "evt-del-1", TaskID: task.ID, Kind: ledger.EventTaskReady,
		Actor: "test", Payload: ledger.JSON("{}"), CreatedAt: time.Now(),
	}
	if err := b.AppendEvent(ctx, evt); err != nil {
		t.Fatal(err)
	}
	if err := b.SaveCheckpoint(ctx, &ledger.Checkpoint{
		ID: "cp-del", TaskID: task.ID, EventSeq: 1, TaskState: ledger.JSON("{}"),
		WorkingMem: ledger.JSON("{}"), Reason: "manual", CreatedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	if err := b.SaveArtifact(ctx, &ledger.Artifact{
		ID: "art-del", TaskID: task.ID, Name: "out.txt", Kind: "file",
		Metadata: ledger.JSON("{}"), CreatedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	if err := b.CreateDependency(ctx, &ledger.TaskDependency{
		ID: "dep-del", FromTaskID: task.ID, ToTaskID: other.ID,
		Kind: ledger.DepBlocking, CreatedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}

	if err := b.DeleteTask(ctx, task.ID); err != nil {
		t.Fatalf("DeleteTask: %v", err)
	}

	if _, err := b.GetTask(ctx, task.ID); err == nil {
		t.Fatal("task row must be gone")
	}
	if n, _ := b.CountEvents(ctx, task.ID); n != 0 {
		t.Fatalf("events remain after delete: %d", n)
	}
	if cps, _ := b.ListCheckpoints(ctx, task.ID, 10); len(cps) != 0 {
		t.Fatalf("checkpoints remain after delete: %d", len(cps))
	}
	if arts, _ := b.ListArtifacts(ctx, task.ID); len(arts) != 0 {
		t.Fatalf("artifacts remain after delete: %d", len(arts))
	}
	if deps, _ := b.ListDependencies(ctx, other.ID); len(deps) != 0 {
		t.Fatalf("dependency links remain after delete: %d", len(deps))
	}
	if err := b.DeleteTask(ctx, task.ID); err != ledger.ErrTaskNotFound {
		t.Fatalf("second delete = %v, want ErrTaskNotFound", err)
	}

	// A future task may reuse the ID; its events must start from seq 1 again.
	if err := b.CreateTask(ctx, testTask(task.ID)); err != nil {
		t.Fatalf("recreate task: %v", err)
	}
	fresh := &ledger.Event{
		ID: "evt-del-2", TaskID: task.ID, Kind: ledger.EventTaskReady,
		Actor: "test", Payload: ledger.JSON("{}"), CreatedAt: time.Now(),
	}
	if err := b.AppendEvent(ctx, fresh); err != nil {
		t.Fatalf("append after recreate: %v", err)
	}
	if fresh.Seq != 1 {
		t.Fatalf("seq after ID reuse = %d, want 1 (cache must reset on delete)", fresh.Seq)
	}
}
