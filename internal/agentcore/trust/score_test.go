package trust

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTrackerRecordSuccess(t *testing.T) {
	dir := t.TempDir()
	tracker := NewTracker(filepath.Join(dir, "trust.json"))

	tracker.RecordSuccess("skill_a")
	e := tracker.Get("skill_a")
	if e.Executions != 1 {
		t.Errorf("executions = %d, want 1", e.Executions)
	}
	if e.Alpha < 1 {
		t.Errorf("alpha = %.1f, want >= 1", e.Alpha)
	}
	if e.Score < 0 || e.Score > 100 {
		t.Errorf("score = %d, want in [0, 100]", e.Score)
	}
}

func TestTrackerRecordFailure(t *testing.T) {
	dir := t.TempDir()
	tracker := NewTracker(filepath.Join(dir, "trust.json"))

	for i := 0; i < 50; i++ {
		tracker.RecordSuccess("skill_b")
	}

	tracker.RecordFailure("skill_b", 10)
	e := tracker.Get("skill_b")
	if e.Failures != 1 {
		t.Errorf("failures = %d, want 1", e.Failures)
	}
	if e.BetaParam < 10 {
		t.Errorf("beta = %.1f, want >= 10 after severity-10 failure", e.BetaParam)
	}
	beforeFailure := Entry{Alpha: 51, BetaParam: 1}
	if e.BayesianScore() >= beforeFailure.BayesianScore() {
		t.Error("score should decrease after failure")
	}
}

func TestTrackerScoreCap(t *testing.T) {
	dir := t.TempDir()
	tracker := NewTracker(filepath.Join(dir, "trust.json"))

	for i := 0; i < 150; i++ {
		tracker.RecordSuccess("capped")
	}
	e := tracker.Get("capped")
	if e.Score > 100 {
		t.Errorf("score = %d, should be capped at 100", e.Score)
	}
}

func TestTrackerScoreFloor(t *testing.T) {
	dir := t.TempDir()
	tracker := NewTracker(filepath.Join(dir, "trust.json"))

	tracker.RecordFailure("new_skill", 50)
	e := tracker.Get("new_skill")
	if e.Score < 0 {
		t.Errorf("score = %d, should not go below 0", e.Score)
	}
}

func TestPermLevels(t *testing.T) {
	tests := []struct {
		score int
		want  PermLevel
	}{
		{0, PermReadOnly},
		{29, PermReadOnly},
		{30, PermWrite},
		{59, PermWrite},
		{60, PermNetwork},
		{79, PermNetwork},
		{80, PermShell},
		{100, PermShell},
	}
	for _, tt := range tests {
		e := Entry{Score: tt.score}
		if e.Allowed() != tt.want {
			t.Errorf("score %d: Allowed() = %s, want %s", tt.score, e.Allowed(), tt.want)
		}
	}
}

func TestCheckPermission(t *testing.T) {
	dir := t.TempDir()
	tracker := NewTracker(filepath.Join(dir, "trust.json"))

	for i := 0; i < 85; i++ {
		tracker.RecordSuccess("trusted_skill")
	}

	if !tracker.CheckPermission("trusted_skill", PermShell) {
		t.Error("85 successes should allow shell")
	}
	if tracker.CheckPermission("unknown_skill", PermWrite) {
		t.Error("unknown skill should not have write permission")
	}
}

func TestTrackerReset(t *testing.T) {
	dir := t.TempDir()
	tracker := NewTracker(filepath.Join(dir, "trust.json"))

	tracker.RecordSuccess("to_reset")
	tracker.Reset("to_reset")

	e := tracker.Get("to_reset")
	if e.Score != 0 {
		t.Errorf("score = %d, want 0 after reset", e.Score)
	}
}

func TestTrackerPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "trust.json")

	// Create and populate
	t1 := NewTracker(path)
	for i := 0; i < 50; i++ {
		t1.RecordSuccess("persisted")
	}

	// Load from same file
	t2 := NewTracker(path)
	e := t2.Get("persisted")
	if e.Score <= 0 {
		t.Errorf("persisted score = %d, want > 0", e.Score)
	}
	if e.Alpha < 50 {
		t.Errorf("persisted alpha = %.1f, want >= 50", e.Alpha)
	}
}

func TestTrackerAll(t *testing.T) {
	dir := t.TempDir()
	tracker := NewTracker(filepath.Join(dir, "trust.json"))

	tracker.RecordSuccess("a")
	tracker.RecordSuccess("b")

	all := tracker.All()
	if len(all) != 2 {
		t.Errorf("all = %d, want 2", len(all))
	}
}

func TestPermLevelString(t *testing.T) {
	if PermReadOnly.String() != "read-only" {
		t.Errorf("PermReadOnly string wrong")
	}
	if PermShell.String() != "shell" {
		t.Errorf("PermShell string wrong")
	}
}

func TestTrackerGetUnknown(t *testing.T) {
	dir := t.TempDir()
	tracker := NewTracker(filepath.Join(dir, "trust.json"))
	e := tracker.Get("nonexistent")
	if e.Score != 0 || e.Executions != 0 {
		t.Error("unknown skill should return zero entry")
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
