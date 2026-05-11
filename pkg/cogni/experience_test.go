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

func TestExperienceStore_ToolMemory_DeduplicatesRepeatedLessons(t *testing.T) {
	es := NewExperienceStore("test", ExperienceConfig{StoreDir: t.TempDir()})
	es.AddToolMemory(ToolExperience{Tool: "search", Context: "large query", Learned: "Paginate results", Confidence: 0.7, SuccessRate: 0.6})
	es.AddToolMemory(ToolExperience{Tool: "search", Context: "large query", Learned: "Paginate results", Confidence: 0.9, SuccessRate: 0.8, Result: "success", VerifiedBy: "review"})

	memories := es.ToolMemory("search")
	if len(memories) != 1 {
		t.Fatalf("memories = %d, want 1", len(memories))
	}
	if memories[0].UsedCount != 2 {
		t.Fatalf("used_count = %d, want 2", memories[0].UsedCount)
	}
	if memories[0].Confidence != 0.9 || memories[0].SuccessRate != 0.8 || memories[0].Result != "success" || memories[0].VerifiedBy != "review" {
		t.Fatalf("merged memory did not preserve stronger metadata: %+v", memories[0])
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

func TestExperienceStore_ConfirmPatternUpdatesProfileTimestamp(t *testing.T) {
	es := NewExperienceStore("test", ExperienceConfig{
		StoreDir:      t.TempDir(),
		RequireReview: true,
	})
	old := time.Now().Add(-24 * time.Hour)
	es.SuggestPattern(BehaviorPattern{
		ID:        "timeout-recovery",
		Trigger:   "响应超时",
		Response:  "保留轨迹并切备用模型",
		CreatedAt: old,
		LastUsed:  old,
	})

	before := es.Summary(5).UpdatedAt
	if before.IsZero() {
		t.Fatal("summary updated_at should include suggested pattern")
	}
	if !es.ConfirmPattern("timeout-recovery") {
		t.Fatal("confirm pattern")
	}
	after := es.Summary(5)
	if len(after.PendingPatterns) != 0 {
		t.Fatalf("pending patterns = %d, want 0", len(after.PendingPatterns))
	}
	if !after.UpdatedAt.After(before) {
		t.Fatalf("updated_at did not advance after confirmation: before=%s after=%s", before, after.UpdatedAt)
	}
	confirmed := es.ConfirmedPatterns()
	if len(confirmed) != 1 || confirmed[0].LastUsed.IsZero() || !confirmed[0].LastUsed.After(old) {
		t.Fatalf("confirmed pattern timestamp not refreshed: %+v", confirmed)
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

func TestExperienceStore_DomainFacts_DeduplicatesRepeatedFacts(t *testing.T) {
	es := NewExperienceStore("test", ExperienceConfig{StoreDir: t.TempDir()})
	es.AddFact(DomainFact{Fact: "云雀使用 Cogni 声明智体", Source: "chat"})
	es.AddFact(DomainFact{Fact: "云雀使用 Cogni 声明智体", Source: "chat"})

	facts := es.DomainFacts()
	if len(facts) != 1 {
		t.Fatalf("facts = %d, want 1", len(facts))
	}
	if facts[0].UsedCount != 2 {
		t.Fatalf("used_count = %d, want 2", facts[0].UsedCount)
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

func TestExperienceStore_SummaryRanksReusableProfile(t *testing.T) {
	es := NewExperienceStore("test", ExperienceConfig{
		StoreDir:      t.TempDir(),
		RequireReview: true,
	})
	now := time.Now()

	es.AddToolMemory(ToolExperience{
		Tool:      "slow-tool",
		Context:   "old context",
		Learned:   "Use fallback",
		UsedCount: 1,
		CreatedAt: now.Add(-3 * time.Hour),
		LastUsed:  now.Add(-3 * time.Hour),
	})
	es.AddToolMemory(ToolExperience{
		Tool:      "fast-tool",
		Context:   "hot context",
		Learned:   "Use indexed path",
		UsedCount: 4,
		CreatedAt: now.Add(-2 * time.Hour),
		LastUsed:  now.Add(-1 * time.Hour),
	})
	es.AddFact(DomainFact{
		Fact:      "云雀的 Planner 需要可恢复执行轨迹",
		Source:    "doc",
		UsedCount: 3,
		CreatedAt: now.Add(-90 * time.Minute),
		LastUsed:  now.Add(-30 * time.Minute),
	})
	es.AddFact(DomainFact{
		Fact:      "AaaS 对外接口保持轻量",
		Source:    "doc",
		UsedCount: 1,
		CreatedAt: now.Add(-2 * time.Hour),
		LastUsed:  now.Add(-2 * time.Hour),
	})
	es.SuggestPattern(BehaviorPattern{
		ID:        "pending-review",
		Trigger:   "模型响应失败",
		Response:  "保留轨迹并切备用引擎",
		UsedCount: 2,
		CreatedAt: now.Add(-20 * time.Minute),
		LastUsed:  now.Add(-10 * time.Minute),
	})
	es.SuggestPattern(BehaviorPattern{
		ID:        "confirmed",
		Trigger:   "测试失败",
		Response:  "先收窄到最小包",
		CreatedAt: now.Add(-10 * time.Minute),
	})
	if !es.ConfirmPattern("confirmed") {
		t.Fatal("confirm pattern")
	}

	summary := es.Summary(1)
	if summary.Stats["patterns_pending"] != 1 {
		t.Fatalf("patterns_pending = %d, want 1", summary.Stats["patterns_pending"])
	}
	if len(summary.TopTools) != 1 || summary.TopTools[0].Tool != "fast-tool" {
		t.Fatalf("top tools = %+v, want fast-tool only", summary.TopTools)
	}
	if len(summary.TopFacts) != 1 || summary.TopFacts[0].Fact != "云雀的 Planner 需要可恢复执行轨迹" {
		t.Fatalf("top facts = %+v, want Chinese planner fact only", summary.TopFacts)
	}
	if len(summary.PendingPatterns) != 1 || summary.PendingPatterns[0].ID != "pending-review" {
		t.Fatalf("pending patterns = %+v, want pending-review only", summary.PendingPatterns)
	}
	if summary.UpdatedAt.IsZero() {
		t.Fatal("updated_at should be populated")
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
