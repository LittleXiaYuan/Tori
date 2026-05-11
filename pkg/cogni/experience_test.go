package cogni

import (
	"context"
	"testing"
	"time"
)

func TestExperienceStore_ToolMemory(t *testing.T) {
	es := NewExperienceStore("test-cogni", ExperienceConfig{
		Enabled:  true,
		StoreDir: t.TempDir(),
	})

	es.AddToolMemory(ToolExperience{
		Tool:       "github_get_diff",
		Context:    "PR > 500 lines",
		Result:     "timeout",
		Learned:    "Use get_file_changes first for large PRs",
		Confidence: 0.9,
	})

	memories := es.ToolMemory("github_get_diff")
	if len(memories) != 1 {
		t.Fatalf("memories = %d, want 1", len(memories))
	}
	if memories[0].Learned != "Use get_file_changes first for large PRs" {
		t.Errorf("Learned = %q", memories[0].Learned)
	}
}

func TestExperienceStore_ToolMemory_AllTools(t *testing.T) {
	es := NewExperienceStore("test", ExperienceConfig{StoreDir: t.TempDir()})
	es.AddToolMemory(ToolExperience{Tool: "a", Learned: "x"})
	es.AddToolMemory(ToolExperience{Tool: "b", Learned: "y"})

	all := es.ToolMemory("")
	if len(all) != 2 {
		t.Errorf("all = %d, want 2", len(all))
	}
}

func TestExperienceStore_Patterns(t *testing.T) {
	es := NewExperienceStore("test", ExperienceConfig{
		StoreDir:      t.TempDir(),
		RequireReview: true,
	})

	es.SuggestPattern(BehaviorPattern{
		Trigger:  "deploy failed",
		Response: "Check env vars first",
	})

	patterns := es.Patterns()
	if len(patterns) != 1 {
		t.Fatalf("patterns = %d, want 1", len(patterns))
	}
	if patterns[0].Confirmed {
		t.Error("should not be confirmed when RequireReview=true")
	}

	es.ConfirmPattern(patterns[0].ID)
	confirmed := es.ConfirmedPatterns()
	if len(confirmed) != 1 {
		t.Errorf("confirmed = %d, want 1", len(confirmed))
	}
}

func TestExperienceStore_Patterns_AutoConfirm(t *testing.T) {
	es := NewExperienceStore("test", ExperienceConfig{
		StoreDir:      t.TempDir(),
		RequireReview: false,
	})

	es.SuggestPattern(BehaviorPattern{Trigger: "x", Response: "y"})
	if !es.Patterns()[0].Confirmed {
		t.Error("should be auto-confirmed when RequireReview=false")
	}
}

func TestExperienceStore_DomainFacts(t *testing.T) {
	es := NewExperienceStore("test", ExperienceConfig{
		StoreDir: t.TempDir(),
		MaxFacts: 3,
	})

	for i := 0; i < 5; i++ {
		es.AddFact(DomainFact{
			Fact:   "fact " + string(rune('A'+i)),
			Source: "test",
		})
	}

	facts := es.DomainFacts()
	if len(facts) > 3 {
		t.Errorf("facts = %d, should be capped at 3", len(facts))
	}
}

func TestExperienceStore_ContextHints(t *testing.T) {
	es := NewExperienceStore("test", ExperienceConfig{
		StoreDir:      t.TempDir(),
		MinConfidence: 0.5,
	})

	es.AddToolMemory(ToolExperience{
		Tool:       "search",
		Context:    "large query",
		Learned:    "Paginate results",
		Confidence: 0.9,
	})

	hints := es.ContextHints(context.Background(), "how to search")
	if hints == "" {
		t.Error("expected non-empty hints")
	}
}

func TestExperienceStore_ContextHintsMarksExperienceUsed(t *testing.T) {
	es := NewExperienceStore("test", ExperienceConfig{
		StoreDir:      t.TempDir(),
		MinConfidence: 0.5,
	})
	old := time.Now().Add(-24 * time.Hour)
	es.AddToolMemory(ToolExperience{
		Tool:        "search",
		Context:     "large query",
		Learned:     "Paginate results",
		Confidence:  0.9,
		UsedCount:   1,
		SuccessRate: 0.8,
		LastUsed:    old,
	})
	es.SuggestPattern(BehaviorPattern{
		ID:          "deploy-first",
		Trigger:     "deploy failed",
		Response:    "Check env vars first",
		Confirmed:   true,
		SuccessRate: 0.9,
		UsedCount:   1,
		LastUsed:    old,
	})
	es.AddFact(DomainFact{
		Fact:      "青岛开源生态需要持续运营",
		Source:    "test",
		UsedCount: 1,
		LastUsed:  old,
	})

	hints := es.ContextHints(context.Background(), "deploy failed: search 青岛开源生态需要持续运营")
	if hints == "" {
		t.Fatal("expected non-empty hints")
	}
	if es.toolMemory[0].UsedCount != 2 || !es.toolMemory[0].LastUsed.After(old) {
		t.Fatalf("tool memory usage not updated: count=%d last=%s", es.toolMemory[0].UsedCount, es.toolMemory[0].LastUsed)
	}
	if es.patterns[0].UsedCount != 2 || !es.patterns[0].LastUsed.After(old) {
		t.Fatalf("pattern usage not updated: count=%d last=%s", es.patterns[0].UsedCount, es.patterns[0].LastUsed)
	}
	if es.facts[0].UsedCount != 2 || !es.facts[0].LastUsed.After(old) {
		t.Fatalf("fact usage not updated: count=%d last=%s", es.facts[0].UsedCount, es.facts[0].LastUsed)
	}
}

func TestExperienceStore_ContextHints_NoMatch(t *testing.T) {
	es := NewExperienceStore("test", ExperienceConfig{StoreDir: t.TempDir()})
	hints := es.ContextHints(context.Background(), "unrelated query")
	if hints != "" {
		t.Errorf("expected empty hints, got: %s", hints)
	}
}

func TestExperienceStore_DecayWeight(t *testing.T) {
	es := NewExperienceStore("test", ExperienceConfig{HalfLifeDays: 90})

	now := time.Now()
	fresh := es.decayWeight(now, now)
	if fresh != 1.0 {
		t.Errorf("fresh weight = %f, want 1.0", fresh)
	}

	halfLife := es.decayWeight(now.Add(-90*24*time.Hour), now)
	if halfLife < 0.45 || halfLife > 0.55 {
		t.Errorf("half-life weight = %f, want ~0.5", halfLife)
	}
}

func TestExperienceStore_ExportImport(t *testing.T) {
	dir1 := t.TempDir()
	es1 := NewExperienceStore("export-test", ExperienceConfig{StoreDir: dir1})
	es1.AddToolMemory(ToolExperience{Tool: "t1", Learned: "lesson1", Confidence: 0.9})
	es1.AddFact(DomainFact{Fact: "fact1", Source: "test"})

	data, err := es1.Export()
	if err != nil {
		t.Fatalf("export: %v", err)
	}

	dir2 := t.TempDir()
	es2 := NewExperienceStore("import-test", ExperienceConfig{StoreDir: dir2})
	if err := es2.Import(data); err != nil {
		t.Fatalf("import: %v", err)
	}

	if len(es2.ToolMemory("t1")) != 1 {
		t.Error("imported tool memory missing")
	}
	if len(es2.DomainFacts()) != 1 {
		t.Error("imported facts missing")
	}
}

func TestExperienceStore_Stats(t *testing.T) {
	es := NewExperienceStore("test", ExperienceConfig{StoreDir: t.TempDir()})
	es.AddToolMemory(ToolExperience{Tool: "a"})
	es.SuggestPattern(BehaviorPattern{Trigger: "x", Response: "y"})
	es.AddFact(DomainFact{Fact: "f"})

	stats := es.Stats()
	if stats["tool_memories"] != 1 {
		t.Errorf("tool_memories = %d", stats["tool_memories"])
	}
	if stats["patterns_total"] != 1 {
		t.Errorf("patterns = %d", stats["patterns_total"])
	}
	if stats["domain_facts"] != 1 {
		t.Errorf("facts = %d", stats["domain_facts"])
	}
}

func TestExperienceStore_Persistence(t *testing.T) {
	dir := t.TempDir()
	es1 := NewExperienceStore("persist-test", ExperienceConfig{StoreDir: dir})
	es1.AddToolMemory(ToolExperience{Tool: "t1", Learned: "l1", Confidence: 0.8})

	es2 := NewExperienceStore("persist-test", ExperienceConfig{StoreDir: dir})
	if len(es2.ToolMemory("t1")) != 1 {
		t.Error("data not persisted/loaded")
	}
}
