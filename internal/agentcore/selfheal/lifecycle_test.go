package selfheal

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"yunque-agent/internal/agentcore/skillgrowth"
)

func mockLLM(_ context.Context, _, _ string) (string, error) {
	return `{
		"name": "test-plugin",
		"description": "Test plugin",
		"language": "python",
		"skill_name": "test_skill",
		"skill_desc": "A test skill",
		"params": {"input": {"type": "string", "description": "test input"}},
		"handler_code": "#!/usr/bin/env python3\nprint('hello')"
	}`, nil
}

func TestLifecycle_GenerateCandidate(t *testing.T) {
	dir := t.TempDir()
	healer := New(filepath.Join(dir, "plugins"), mockLLM)
	lc := NewLifecycle(healer, dir)

	c, err := lc.GenerateCandidate(context.Background(), "user needs calculator")
	if err != nil {
		t.Fatal(err)
	}
	if c.State != StateCandidate {
		t.Fatalf("expected candidate state, got %s", c.State)
	}
	if c.Plugin.Name != "test-plugin" {
		t.Fatalf("unexpected plugin name: %s", c.Plugin.Name)
	}
	if c.Reason != "user needs calculator" {
		t.Fatalf("unexpected reason: %s", c.Reason)
	}
}

func TestLifecycle_PromoteAndRollback(t *testing.T) {
	dir := t.TempDir()
	healer := New(filepath.Join(dir, "plugins"), mockLLM)
	lc := NewLifecycle(healer, dir)

	c, err := lc.GenerateCandidate(context.Background(), "test reason")
	if err != nil {
		t.Fatal(err)
	}

	// Promote
	err = lc.Promote(context.Background(), c.ID)
	if err != nil {
		t.Fatal(err)
	}

	updated, ok := lc.Get(c.ID)
	if !ok {
		t.Fatal("candidate not found after promote")
	}
	if updated.State != StatePromoted {
		t.Fatalf("expected promoted, got %s", updated.State)
	}
	if updated.PromotedAt == nil {
		t.Fatal("expected PromotedAt to be set")
	}

	// Verify plugin files exist
	pluginDir := filepath.Join(dir, "plugins", "test-plugin")
	if _, err := os.Stat(filepath.Join(pluginDir, "plugin.json")); err != nil {
		t.Fatalf("plugin.json missing: %v", err)
	}

	// Rollback
	err = lc.Rollback(c.ID)
	if err != nil {
		t.Fatal(err)
	}

	updated, _ = lc.Get(c.ID)
	if updated.State != StateRolledBack {
		t.Fatalf("expected rolled_back, got %s", updated.State)
	}
	if updated.RolledBackAt == nil {
		t.Fatal("expected RolledBackAt to be set")
	}

	// Verify plugin files removed
	if _, err := os.Stat(pluginDir); !os.IsNotExist(err) {
		t.Fatal("expected plugin dir to be removed after rollback")
	}
}

func TestLifecycle_Reject(t *testing.T) {
	dir := t.TempDir()
	healer := New(filepath.Join(dir, "plugins"), mockLLM)
	lc := NewLifecycle(healer, dir)

	c, err := lc.GenerateCandidate(context.Background(), "test")
	if err != nil {
		t.Fatal(err)
	}

	err = lc.Reject(c.ID, "不需要这个技能")
	if err != nil {
		t.Fatal(err)
	}

	updated, _ := lc.Get(c.ID)
	if updated.State != StateRejected {
		t.Fatalf("expected rejected, got %s", updated.State)
	}
	if updated.RejectReason != "不需要这个技能" {
		t.Fatalf("unexpected reject reason: %s", updated.RejectReason)
	}
}

func TestLifecycle_StateTransitionErrors(t *testing.T) {
	dir := t.TempDir()
	healer := New(filepath.Join(dir, "plugins"), mockLLM)
	lc := NewLifecycle(healer, dir)

	c, _ := lc.GenerateCandidate(context.Background(), "test")

	// Reject it
	lc.Reject(c.ID, "nope")

	// Try to promote rejected candidate
	if err := lc.Promote(context.Background(), c.ID); err == nil {
		t.Fatal("expected error promoting rejected candidate")
	}

	// Try to rollback non-promoted candidate
	if err := lc.Rollback(c.ID); err == nil {
		t.Fatal("expected error rolling back rejected candidate")
	}

	// Non-existent candidate
	if err := lc.Promote(context.Background(), "nonexistent"); err == nil {
		t.Fatal("expected error for nonexistent candidate")
	}
}

func TestLifecycle_ListAndCount(t *testing.T) {
	dir := t.TempDir()
	healer := New(filepath.Join(dir, "plugins"), mockLLM)
	lc := NewLifecycle(healer, dir)

	lc.GenerateCandidate(context.Background(), "reason-1")
	c2, _ := lc.GenerateCandidate(context.Background(), "reason-2")
	lc.Reject(c2.ID, "nope")

	all := lc.List("")
	if len(all) != 2 {
		t.Fatalf("expected 2, got %d", len(all))
	}

	candidates := lc.List(StateCandidate)
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}

	rejected := lc.List(StateRejected)
	if len(rejected) != 1 {
		t.Fatalf("expected 1 rejected, got %d", len(rejected))
	}

	counts := lc.Count()
	if counts[StateCandidate] != 1 || counts[StateRejected] != 1 {
		t.Fatalf("unexpected counts: %v", counts)
	}
}

func TestLifecycle_Cleanup(t *testing.T) {
	dir := t.TempDir()
	healer := New(filepath.Join(dir, "plugins"), mockLLM)
	lc := NewLifecycle(healer, dir)

	c, _ := lc.GenerateCandidate(context.Background(), "test")
	lc.Reject(c.ID, "nope")

	// Cleanup with 0 duration should remove all rejected
	removed := lc.Cleanup(0)
	if removed != 1 {
		t.Fatalf("expected 1 removed, got %d", removed)
	}

	if len(lc.List("")) != 0 {
		t.Fatal("expected empty after cleanup")
	}
}

func TestLifecycle_Persistence(t *testing.T) {
	dir := t.TempDir()
	healer := New(filepath.Join(dir, "plugins"), mockLLM)

	// Create and save
	lc1 := NewLifecycle(healer, dir)
	lc1.GenerateCandidate(context.Background(), "test-persist")

	// Load from file
	lc2 := NewLifecycle(healer, dir)
	all := lc2.List("")
	if len(all) != 1 {
		t.Fatalf("expected 1 candidate after reload, got %d", len(all))
	}
	if all[0].Reason != "test-persist" {
		t.Fatalf("unexpected reason after reload: %s", all[0].Reason)
	}
}

func TestLifecycle_ValidationWarnings(t *testing.T) {
	dir := t.TempDir()
	// Mock LLM that returns code with a dangerous pattern
	dangerousLLM := func(_ context.Context, _, _ string) (string, error) {
		return `{
			"name": "bad-plugin",
			"description": "Dangerous",
			"language": "python",
			"skill_name": "bad_skill",
			"skill_desc": "Bad",
			"params": {},
			"handler_code": "#!/usr/bin/env python3\nimport os\nos.system('rm -rf /')"
		}`, nil
	}
	healer := New(filepath.Join(dir, "plugins"), dangerousLLM)
	lc := NewLifecycle(healer, dir)

	c, err := lc.GenerateCandidate(context.Background(), "test dangerous")
	if err != nil {
		t.Fatal(err)
	}
	if len(c.ValidationErrors) == 0 {
		t.Fatal("expected validation warnings for dangerous code")
	}

	// Promote should fail due to validation
	err = lc.Promote(context.Background(), c.ID)
	if err == nil {
		t.Fatal("expected promote to fail for dangerous plugin")
	}

	// State should remain candidate (not promoted)
	updated, _ := lc.Get(c.ID)
	if updated.State != StateCandidate {
		t.Fatalf("expected candidate state after failed promote, got %s", updated.State)
	}
}

func TestLifecycle_DoublePromote(t *testing.T) {
	dir := t.TempDir()
	healer := New(filepath.Join(dir, "plugins"), mockLLM)
	lc := NewLifecycle(healer, dir)

	c, _ := lc.GenerateCandidate(context.Background(), "test")
	lc.Promote(context.Background(), c.ID)

	// Double promote should fail
	err := lc.Promote(context.Background(), c.ID)
	if err == nil {
		t.Fatal("expected error on double promote")
	}
}

func TestLifecycleSkillGrowthAdapters(t *testing.T) {
	dir := t.TempDir()
	healer := New(filepath.Join(dir, "plugins"), mockLLM)
	lc := NewLifecycle(healer, dir)

	candidate, err := lc.GenerateSkillGrowthCandidate(context.Background(), skillgrowth.Gap{
		CapabilityID:   "cap.selfheal",
		Description:    "user needs calculator",
		FailureContext: "missing calculator",
	})
	if err != nil {
		t.Fatal(err)
	}
	if candidate.CapabilityID != "cap.selfheal" {
		t.Fatalf("unexpected capability: %s", candidate.CapabilityID)
	}
	if candidate.Source != "selfheal.lifecycle" {
		t.Fatalf("unexpected source: %s", candidate.Source)
	}

	result, err := lc.PromoteCandidate(context.Background(), candidate.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.RegisteredName != "test_skill" {
		t.Fatalf("unexpected registered name: %s", result.RegisteredName)
	}

	if err := lc.RollbackCandidate(context.Background(), candidate.ID, "test rollback"); err != nil {
		t.Fatal(err)
	}
	updated, _ := lc.Get(candidate.ID)
	if updated.State != StateRolledBack {
		t.Fatalf("expected rolled_back, got %s", updated.State)
	}
}

// Make sure time.Duration is used in the test (suppress unused import)
var _ = time.Second
