package planner

import (
	"testing"

	"yunque-agent/internal/cognicore/recommend"
	"yunque-agent/pkg/skills"
)

func TestSkillRuntimeServiceScorerWithRecent(t *testing.T) {
	scorer := &skills.SkillScorer{}
	service := NewSkillRuntimeService(nil)
	service.SetScorer(scorer)
	service.RecordRecent([]string{"web_search", "file_read"})

	got := service.ScorerWithRecent()
	if got != scorer {
		t.Fatal("expected same scorer pointer")
	}
	if len(got.RecentSkills) != 2 || got.RecentSkills[0] != "web_search" || got.RecentSkills[1] != "file_read" {
		t.Fatalf("unexpected recent skills: %#v", got.RecentSkills)
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
