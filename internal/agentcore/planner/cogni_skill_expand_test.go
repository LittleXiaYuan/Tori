package planner

import (
	"sort"
	"testing"

	"yunque-agent/pkg/skills"
)

// buildExpandRegistry creates a registry with a representative spread of skills:
// categorized (file/research/browser) and uncategorized general tools.
func buildExpandRegistry() *skills.Registry {
	reg := skills.NewRegistry()
	for _, s := range []*mockSkill{
		{name: "file_read", desc: "read a file"},
		{name: "file_write", desc: "write a file"},
		{name: "file_delete", desc: "delete a file"},
		{name: "web_search", desc: "search the web"},
		{name: "deep_research", desc: "deep research"},
		{name: "browser_navigate", desc: "navigate browser"},
		{name: "code_execute", desc: "run code"}, // uncategorized → always available
		{name: "computer_use", desc: "control computer"}, // uncategorized
	} {
		reg.Register(s)
	}
	reg.DefineCategory(skills.SkillCategory{ID: "file", Name: "文件"})
	for _, n := range []string{"file_read", "file_write", "file_delete"} {
		reg.AssignCategory(n, "file")
	}
	reg.DefineCategory(skills.SkillCategory{ID: "research", Name: "研究"})
	for _, n := range []string{"web_search", "deep_research"} {
		reg.AssignCategory(n, "research")
	}
	reg.DefineCategory(skills.SkillCategory{ID: "browser", Name: "浏览器"})
	reg.AssignCategory("browser_navigate", "browser")
	return reg
}

func TestExpandCogniSkills_NoOpinion_ReturnsNil(t *testing.T) {
	p := NewPlanner(nil, buildExpandRegistry(), 8)
	// nil categories + nil globs + no denies → no opinion → don't narrow.
	if got := p.expandCogniSkills(nil, nil, nil); got != nil {
		t.Errorf("expected nil (no narrowing) for empty decision, got %v", got)
	}
}

func TestExpandCogniSkills_CategoryFilter(t *testing.T) {
	p := NewPlanner(nil, buildExpandRegistry(), 8)
	// "research" category → its skills + all uncategorized general tools.
	got := p.expandCogniSkills([]string{"research"}, nil, nil)
	assertContainsAll(t, got, "web_search", "deep_research", "code_execute", "computer_use")
	assertExcludesAll(t, got, "file_read", "file_write", "browser_navigate")
}

func TestExpandCogniSkills_ToolGlobAddsAcrossCategories(t *testing.T) {
	p := NewPlanner(nil, buildExpandRegistry(), 8)
	// research category + "file_*" glob → research skills, file_* skills, general.
	got := p.expandCogniSkills([]string{"research"}, []string{"file_*"}, nil)
	assertContainsAll(t, got, "web_search", "file_read", "file_write", "code_execute")
	assertExcludesAll(t, got, "browser_navigate")
}

func TestExpandCogniSkills_DenyStripsDestructive(t *testing.T) {
	p := NewPlanner(nil, buildExpandRegistry(), 8)
	// file category allowed, but deny file_write/file_delete (the safety pass).
	got := p.expandCogniSkills([]string{"file"}, nil, []string{"file_write", "file_delete"})
	assertContainsAll(t, got, "file_read", "code_execute")
	assertExcludesAll(t, got, "file_write", "file_delete")
}

func TestExpandCogniSkills_DenyOnlyNarrowsFromFullRegistry(t *testing.T) {
	p := NewPlanner(nil, buildExpandRegistry(), 8)
	// Only a deny (no allow restriction) → start from full registry, subtract.
	got := p.expandCogniSkills(nil, nil, []string{"computer_use", "code_execute"})
	if got == nil {
		t.Fatal("expected a narrowed list when a deny is present, got nil")
	}
	assertContainsAll(t, got, "file_read", "web_search", "browser_navigate")
	assertExcludesAll(t, got, "computer_use", "code_execute")
}

func TestExpandCogniSkills_EmptyCategories_EmpathyMode(t *testing.T) {
	p := NewPlanner(nil, buildExpandRegistry(), 8)
	// Explicit empty (not nil) categories = "I want no skills" (empathy mode).
	// Uncategorized general tools still pass (they are never narrowed out), but
	// every categorized skill is excluded.
	got := p.expandCogniSkills([]string{}, []string{}, nil)
	if got == nil {
		t.Fatal("expected non-nil narrowed list for explicit empty decision")
	}
	assertExcludesAll(t, got, "file_read", "web_search", "browser_navigate")
}

func assertContainsAll(t *testing.T, got []string, want ...string) {
	t.Helper()
	set := make(map[string]bool, len(got))
	for _, g := range got {
		set[g] = true
	}
	for _, w := range want {
		if !set[w] {
			sort.Strings(got)
			t.Errorf("expected %q in result, got %v", w, got)
		}
	}
}

func assertExcludesAll(t *testing.T, got []string, unwanted ...string) {
	t.Helper()
	set := make(map[string]bool, len(got))
	for _, g := range got {
		set[g] = true
	}
	for _, u := range unwanted {
		if set[u] {
			sort.Strings(got)
			t.Errorf("expected %q to be excluded, got %v", u, got)
		}
	}
}
