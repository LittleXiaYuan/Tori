package sqlite

import (
	"context"
	"path/filepath"
	"testing"
)

func TestMigrateRecordsCurrentSchemaVersion(t *testing.T) {
	b, err := New(filepath.Join(t.TempDir(), "ledger.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	ctx := context.Background()
	if err := b.Migrate(ctx); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	var version int
	if err := b.db.QueryRowContext(ctx,
		`SELECT version FROM schema_migrations WHERE id = 'ledger_sqlite'`,
	).Scan(&version); err != nil {
		t.Fatalf("read schema version: %v", err)
	}
	if version != currentSchemaVersion {
		t.Fatalf("schema version = %d, want %d", version, currentSchemaVersion)
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	b, err := New(filepath.Join(t.TempDir(), "ledger.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	ctx := context.Background()
	for i := 0; i < 2; i++ {
		if err := b.Migrate(ctx); err != nil {
			t.Fatalf("Migrate #%d: %v", i+1, err)
		}
	}

	var rows int
	if err := b.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM schema_migrations WHERE id = 'ledger_sqlite'`,
	).Scan(&rows); err != nil {
		t.Fatalf("count schema rows: %v", err)
	}
	if rows != 1 {
		t.Fatalf("schema migration rows = %d, want 1", rows)
	}
}

func TestHealthCheckValidatesForeignKeysAndSchemaVersion(t *testing.T) {
	b, err := New(filepath.Join(t.TempDir(), "ledger.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	ctx := context.Background()
	if err := b.Migrate(ctx); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if err := b.HealthCheck(ctx); err != nil {
		t.Fatalf("HealthCheck after migrate: %v", err)
	}

	if _, err := b.db.ExecContext(ctx, `UPDATE schema_migrations SET version = version - 1 WHERE id = 'ledger_sqlite'`); err != nil {
		t.Fatalf("downgrade schema version: %v", err)
	}
	if err := b.HealthCheck(ctx); err == nil {
		t.Fatal("expected stale schema version to fail health check")
	}
}

func TestHealthCheckRejectsDisabledForeignKeys(t *testing.T) {
	b, err := New(filepath.Join(t.TempDir(), "ledger.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	ctx := context.Background()
	if err := b.Migrate(ctx); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if _, err := b.db.ExecContext(ctx, `PRAGMA foreign_keys = OFF`); err != nil {
		t.Fatalf("disable foreign keys: %v", err)
	}
	if err := b.HealthCheck(ctx); err == nil {
		t.Fatal("expected disabled foreign keys to fail health check")
	}
}
