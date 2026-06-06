package main

import (
	"context"
	"testing"

	"yunque-agent/pkg/cogni"
	"yunque-agent/pkg/skills"
)

type catTestSkill struct{ n string }

func (s catTestSkill) Name() string               { return s.n }
func (s catTestSkill) Description() string        { return s.n + " tool" }
func (s catTestSkill) Parameters() map[string]any { return map[string]any{"type": "object"} }
func (s catTestSkill) Execute(context.Context, map[string]any, *skills.Environment) (string, error) {
	return "", nil
}

// TestAutoOrganizerGroupsCategorizedSkillsIntoCognis is the end-to-end proof
// that the new category taxonomy actually flows into Cogni: real
// categorizeSkillName → skills.Registry categories → cogni.AutoOrganizer.Sync
// → one auto:<category> Cogni per domain whose surface includes that domain's
// skills. No LLM required (deterministic grouping path).
func TestAutoOrganizerGroupsCategorizedSkillsIntoCognis(t *testing.T) {
	reg := skills.NewRegistry()
	for _, n := range []string{
		"docx_create", "file_open", // file
		"image_generate",              // image
		"web_search", "deep_research", // research
		"run_workflow", "orchestrate_task", // workflow
		"browser_click", // browser (assigned by its own rule)
		"code_execute",  // uncategorized → general (always-available)
	} {
		reg.Register(catTestSkill{n})
	}

	// Mirror init_tasks ordering: browser/connector first, then prefix taxonomy.
	reg.DefineCategory(skills.SkillCategory{ID: "browser", Name: "browser"})
	reg.AssignCategory("browser_click", "browser")
	for _, id := range []string{"file", "image", "research", "workflow"} {
		reg.DefineCategory(skills.SkillCategory{ID: id, Name: id})
	}
	for _, s := range reg.All() {
		if reg.CategoryOf(s.Name()) != "" {
			continue
		}
		if cat := categorizeSkillName(s.Name()); cat != "" {
			reg.AssignCategory(s.Name(), cat)
		}
	}

	cogniReg := cogni.NewRegistry()
	ao := cogni.NewAutoOrganizer(cogniReg, func() []cogni.SkillInfo {
		all := reg.All()
		out := make([]cogni.SkillInfo, len(all))
		for i, s := range all {
			out[i] = cogni.SkillInfo{Name: s.Name(), Description: s.Description(), Category: reg.CategoryOf(s.Name())}
		}
		return out
	})

	res := ao.Sync(context.Background())
	if res.Created == 0 {
		t.Fatalf("auto-organizer created no cognis: %#v", res)
	}

	wantContains := map[string][]string{
		"auto:file":     {"docx_create", "file_open"},
		"auto:image":    {"image_generate"},
		"auto:research": {"web_search", "deep_research"},
		"auto:workflow": {"run_workflow", "orchestrate_task"},
	}
	for id, wantSkills := range wantContains {
		decl, ok := cogniReg.Get(id)
		if !ok {
			t.Fatalf("missing auto cogni %q", id)
		}
		have := map[string]bool{}
		for _, n := range decl.Surface.Include {
			have[n] = true
		}
		for _, w := range wantSkills {
			if !have[w] {
				t.Fatalf("cogni %q missing skill %q (include=%v)", id, w, decl.Surface.Include)
			}
		}
		if !decl.Experience.Enabled {
			t.Fatalf("auto cogni %q should enable experience for self-tuning", id)
		}
	}

	// Domain isolation: browser stays in its own cogni, not leaked into file.
	if d, ok := cogniReg.Get("auto:file"); ok {
		for _, n := range d.Surface.Include {
			if n == "browser_click" {
				t.Fatalf("browser_click leaked into auto:file: %v", d.Surface.Include)
			}
		}
	}

	// Uncategorized general tools land in auto:general (always-available), never
	// pulled into a domain cogni.
	gen, ok := cogniReg.Get("auto:general")
	if !ok {
		t.Fatal("expected auto:general cogni for uncategorized skills")
	}
	foundGeneral := false
	for _, n := range gen.Surface.Include {
		if n == "code_execute" {
			foundGeneral = true
		}
	}
	if !foundGeneral {
		t.Fatalf("code_execute should be in auto:general, got %v", gen.Surface.Include)
	}
}
