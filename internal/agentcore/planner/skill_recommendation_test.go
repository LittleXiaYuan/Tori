package planner

import (
	"testing"

	"yunque-agent/internal/experimental/recommend"
	"yunque-agent/pkg/skills"
)

func TestRankSkillsByRecommendationKeepsVisibleSurfaceAndPreferredOrder(t *testing.T) {
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
	p := &Planner{registry: reg}
	p.SetSkillRecommendationEngine(engine)
	engine.RecordOutcome("file_read", 1.0, true)
	engine.RecordOutcome("file_read", 1.0, true)
	engine.RecordOutcome("file_read", 1.0, true)
	engine.RecordOutcome("file_read", 1.0, true)
	engine.RecordOutcome("file_read", 1.0, true)
	engine.RecordOutcome("old_hidden_skill", 1.0, true)
	engine.RecordOutcome("old_hidden_skill", 1.0, true)
	engine.RecordOutcome("old_hidden_skill", 1.0, true)
	engine.RecordOutcome("old_hidden_skill", 1.0, true)
	engine.RecordOutcome("old_hidden_skill", 1.0, true)
	engine.RecordOutcome("old_hidden_skill", 1.0, true)
	engine.RecordOutcome("old_hidden_skill", 1.0, true)
	engine.RecordOutcome("old_hidden_skill", 1.0, true)
	engine.RecordOutcome("old_hidden_skill", 1.0, true)
	engine.RecordOutcome("old_hidden_skill", 1.0, true)

	candidates := []skills.Skill{
		&mockSkill{name: "web_search", desc: "search the web"},
		&mockSkill{name: "file_read", desc: "read local files"},
	}
	ranked := p.rankSkillsByRecommendation("please search and read the attached file", candidates)
	if len(ranked) != 2 {
		t.Fatalf("expected 2 ranked skills, got %d", len(ranked))
	}
	if ranked[0].Name() != "file_read" {
		t.Fatalf("expected file_read to be ranked first, got %s", ranked[0].Name())
	}
	for _, s := range ranked {
		if s.Name() == "old_hidden_skill" {
			t.Fatalf("stale hidden skill leaked into ranked surface: %#v", ranked)
		}
	}
}
