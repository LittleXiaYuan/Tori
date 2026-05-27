package ledger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestInitLedgerAtRecoveringQuarantinesCorruptSQLite(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "ledger.db")
	if err := os.WriteFile(dbPath, []byte("not a sqlite database"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dbPath+"-wal", []byte("wal"), 0o644); err != nil {
		t.Fatal(err)
	}

	fixedNow := time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC)
	ldg, report, err := InitLedgerAtRecovering(dbPath, filepath.Join(dir, "quarantine"), func() time.Time { return fixedNow })
	if err != nil {
		t.Fatalf("InitLedgerAtRecovering: %v", err)
	}
	defer ldg.Close()
	if report == nil {
		t.Fatal("expected recovery report")
	}
	if !strings.Contains(report.QuarantineDir, "ledger-corrupt-20260523-120000") {
		t.Fatalf("unexpected quarantine dir: %s", report.QuarantineDir)
	}
	if len(report.Files) == 0 {
		t.Fatalf("expected at least db quarantined, got %#v", report.Files)
	}
	for _, moved := range report.Files {
		if _, err := os.Stat(moved); err != nil {
			t.Fatalf("quarantined file missing %s: %v", moved, err)
		}
	}
	if err := ldg.HealthCheck(t.Context()); err != nil {
		t.Fatalf("fresh ledger health check: %v", err)
	}
}
