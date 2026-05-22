package ledger_test

import (
	"context"
	"testing"

	lsqlite "yunque-agent/internal/ledgercore/backend/sqlite"
)

func TestHealthCheck(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	if err := ldg.HealthCheck(ctx); err != nil {
		t.Fatalf("HealthCheck: %v", err)
	}
}

func TestCheckpoint(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	if err := ldg.Checkpoint(ctx); err != nil {
		t.Fatalf("Checkpoint: %v", err)
	}
}

func TestBackendStats(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	// Insert some data
	ldg.Memory.PutFact(ctx, "t1", "test.key", "test value", "test")
	ldg.Memory.PutFact(ctx, "t1", "test.key2", "test value 2", "test")

	b, ok := ldg.Backend().(*lsqlite.Backend)
	if !ok {
		t.Skip("backend is not SQLite")
	}

	stats, err := b.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}

	if stats.PageSize <= 0 {
		t.Errorf("expected positive page size, got %d", stats.PageSize)
	}
	if stats.PageCount <= 0 {
		t.Errorf("expected positive page count, got %d", stats.PageCount)
	}
	if stats.DBSizeBytes <= 0 {
		t.Errorf("expected positive DB size, got %d", stats.DBSizeBytes)
	}
	if stats.TableCounts["memories"] < 2 {
		t.Errorf("expected at least 2 memories, got %d", stats.TableCounts["memories"])
	}
	t.Logf("stats: size=%d pages=%d tables=%v", stats.DBSizeBytes, stats.PageCount, stats.TableCounts)
}

func TestBackendCheckpoint(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	b, ok := ldg.Backend().(*lsqlite.Backend)
	if !ok {
		t.Skip("backend is not SQLite")
	}

	walPages, checkpointed, err := b.Checkpoint(ctx)
	if err != nil {
		t.Fatalf("Checkpoint: %v", err)
	}
	t.Logf("checkpoint: wal_pages=%d checkpointed=%d", walPages, checkpointed)
}

func TestBackendHealthCheck(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	b, ok := ldg.Backend().(*lsqlite.Backend)
	if !ok {
		t.Skip("backend is not SQLite")
	}

	if err := b.HealthCheck(ctx); err != nil {
		t.Fatalf("HealthCheck: %v", err)
	}
}
