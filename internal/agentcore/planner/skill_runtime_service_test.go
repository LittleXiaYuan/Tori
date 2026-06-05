package planner

import (
	"testing"

	"yunque-agent/internal/cognicore/recommend"
	"yunque-agent/pkg/skills"
)

func TestSkillRuntimeServiceScorerWithRecent(t *testing.T) {
	// With a base scorer: returns a fresh scorer carrying both success rates and
	// the tracked recency (and never the shared base pointer).
	base := &skills.SkillScorer{SuccessRates: map[string]float64{"web_search": 0.9}}
	service := NewSkillRuntimeService(nil)
	service.SetScorer(base)
	service.RecordRecent([]string{"web_search", "file_read"})

	got := service.ScorerWithRecent()
	if got == nil {
		t.Fatal("expected scorer")
	}
	if got == base {
		t.Fatal("should return a fresh scorer, not the shared base")
	}
	if got.SuccessRates["web_search"] != 0.9 {
		t.Fatalf("success rates not carried: %#v", got.SuccessRates)
	}
	if len(got.RecentSkills) != 2 || got.RecentSkills[0] != "web_search" || got.RecentSkills[1] != "file_read" {
		t.Fatalf("unexpected recent skills: %#v", got.RecentSkills)
	}
}

func TestSkillRuntimeServiceScorerWithRecentActivatesRecencyWithoutBaseScorer(t *testing.T) {
	// Production reality: SetScorer is never called. The recency signal is still
	// tracked, so ScorerWithRecent must surface it (previously returned nil).
	service := NewSkillRuntimeService(nil)
	service.RecordRecent([]string{"browser_click"})

	got := service.ScorerWithRecent()
	if got == nil {
		t.Fatal("recency-only scorer should not be nil when recent skills exist")
	}
	if len(got.RecentSkills) != 1 || got.RecentSkills[0] != "browser_click" {
		t.Fatalf("unexpected recent skills: %#v", got.RecentSkills)
	}

	// Genuinely no signal → nil.
	if NewSkillRuntimeService(nil).ScorerWithRecent() != nil {
		t.Fatal("expected nil scorer when there is no base scorer and no recency")
	}
}

func TestSkillRuntimeServiceRecommendationKeepsVisibleSurface(t *testing.T) {
	reg := skills.NewRegistry()
	reg.Register(&mockSkill{name: "web_search", desc: "search the web"})
	reg.Register(&mockSkill{name: "file_read", desc: "read local files"})
	reg.Register(&mockSkill{name: "old_hidden_skill", desc: "legacy hidden search helper"})
	reg.DefineCategory(skills.SkillCategory{
		ID:          "research",
		Name:        "Research",
		Description: "research tools",
		SkillNames:  []string{"web_search"},
	})
	reg.DefineCategory(skills.SkillCategory{
		ID:          "file",
		Name:        "File",
		Description: "file tools",
		SkillNames:  []string{"file_read"},
	})

	engine := recommend.NewEngine()
	service := NewSkillRuntimeService(reg)
	service.SetRecommendationEngine(engine)
	for i := 0; i < 5; i++ {
		engine.RecordOutcome("file_read", 1.0, true)
	}
	for i := 0; i < 10; i++ {
		engine.RecordOutcome("old_hidden_skill", 1.0, true)
	}

	ranked := service.RankByRecommendation("please search and read the attached file", []skills.Skill{
		&mockSkill{name: "web_search", desc: "search the web"},
		&mockSkill{name: "file_read", desc: "read local files"},
	})
	if len(ranked) != 2 {
		t.Fatalf("expected 2 ranked skills, got %d", len(ranked))
	}
	if ranked[0].Name() != "file_read" {
		t.Fatalf("expected file_read first, got %s", ranked[0].Name())
	}
	for _, s := range ranked {
		if s.Name() == "old_hidden_skill" {
			t.Fatalf("hidden skill leaked into ranked surface: %#v", ranked)
		}
	}
}
