package planner

import (
	"context"
	"sort"
	"testing"

	agentcogni "yunque-agent/internal/agentcore/cogni"
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

// denyRuntimeStub is a CogniRuntime whose Decide returns a fixed DeniedTools
// list, isolating the NativeFC risk-deny pass in buildFunctionDefs.
// decisionRuntimeStub is a CogniRuntime that returns a fixed CogniFinalDecision,
// isolating the NativeFC consumption of v2 decisions (DeniedTools narrowing +
// SkillsNeeded intent narrowing) in buildFunctionDefs.
type decisionRuntimeStub struct {
	decision agentcogni.CogniFinalDecision
}

func (s decisionRuntimeStub) Decide(context.Context, string, string, string, string) agentcogni.CogniFinalDecision {
	return s.decision
}
func (s decisionRuntimeStub) BuildContext(context.Context, string, string, string, string) string {
	return ""
}
func (s decisionRuntimeStub) FilterSkills(_ string, _ string, _ string, in []skills.Skill) []skills.Skill {
	return in
}
func (s decisionRuntimeStub) Trace(string, string, string) (CogniTraceDetail, bool) {
	return CogniTraceDetail{}, false
}
func (s decisionRuntimeStub) Tools(context.Context, string, string, string) []CogniTool { return nil }
func (s decisionRuntimeStub) SurfaceAuthoritative(string, string, string) bool          { return false }
func (s decisionRuntimeStub) RecordToolOutcome(string, string, string, string, bool)    {}

// TestBuildFunctionDefs_RiskDenyRemovesDestructiveTools proves the safety
// property end-to-end on the NativeFC path: a RiskCogni deny-list strips the
// matching tools from the FunctionDefs the model actually receives, on the
// ambient build path, even though no user restriction was applied.
func TestBuildFunctionDefs_RiskDenyRemovesDestructiveTools(t *testing.T) {
	reg := buildExpandRegistry()
	p := NewPlanner(nil, reg, 20)
	p.SetCogniRuntime(decisionRuntimeStub{decision: agentcogni.CogniFinalDecision{
		DeniedTools: []string{"file_write", "file_delete", "computer_use"},
	}})

	defs := p.buildFunctionDefs(context.Background(), "帮我处理一下文件", "t", "web", "", false, nil,
		p.ensureContextAssembly(), p.ensureDelegationRuntime(), p.ensureSkillRuntime())

	names := make([]string, 0, len(defs))
	for _, d := range defs {
		names = append(names, d.Name)
	}
	assertExcludesAll(t, names, "file_write", "file_delete", "computer_use")
	// A non-denied, non-destructive tool must survive the deny pass.
	assertContainsAll(t, names, "file_read")
}

// TestBuildFunctionDefs_RiskDenyOverridesUserAllowList proves safety is not
// bypassable: even when the user explicitly allow-lists a destructive tool via
// AllowedSkills (the chat tool drawer), the risk deny still removes it.
func TestBuildFunctionDefs_RiskDenyOverridesUserAllowList(t *testing.T) {
	reg := buildExpandRegistry()
	p := NewPlanner(nil, reg, 20)
	p.SetCogniRuntime(decisionRuntimeStub{decision: agentcogni.CogniFinalDecision{
		DeniedTools: []string{"file_delete"},
	}})

	// User explicitly tried to allow file_delete + file_read.
	defs := p.buildFunctionDefs(context.Background(), "删除这些文件", "t", "web", "", true,
		[]string{"file_delete", "file_read"},
		p.ensureContextAssembly(), p.ensureDelegationRuntime(), p.ensureSkillRuntime())

	names := make([]string, 0, len(defs))
	for _, d := range defs {
		names = append(names, d.Name)
	}
	assertExcludesAll(t, names, "file_delete")
	assertContainsAll(t, names, "file_read")
}

// TestBuildFunctionDefs_V2IntentNarrowsSurface proves "v2 定调": when IntentCogni
// returns a non-nil SkillsNeeded, buildFunctionDefs restricts the tool surface to
// the registry-resolved set for that intent (research category + uncategorized
// general tools), instead of exposing the full registry.
func TestBuildFunctionDefs_V2IntentNarrowsSurface(t *testing.T) {
	reg := buildExpandRegistry()
	p := NewPlanner(nil, reg, 20)
	p.SetCogniRuntime(decisionRuntimeStub{decision: agentcogni.CogniFinalDecision{
		SkillsNeeded: []string{"research"},
	}})

	defs := p.buildFunctionDefs(context.Background(), "帮我查点资料", "t", "web", "", false, nil,
		p.ensureContextAssembly(), p.ensureDelegationRuntime(), p.ensureSkillRuntime())

	names := make([]string, 0, len(defs))
	for _, d := range defs {
		names = append(names, d.Name)
	}
	// research category skills + uncategorized general tools survive…
	assertContainsAll(t, names, "web_search", "deep_research", "code_execute", "computer_use")
	// …but unrelated categories (file/browser) are narrowed away.
	assertExcludesAll(t, names, "file_read", "file_write", "browser_navigate")
}

// TestBuildFunctionDefs_V2ChatIntentEmptiesSurface proves the strongest signal:
// a chat intent (SkillsNeeded == []) narrows away every categorized skill, the
// token win the native scorer can't express. Uncategorized general tools remain.
func TestBuildFunctionDefs_V2ChatIntentEmptiesSurface(t *testing.T) {
	reg := buildExpandRegistry()
	p := NewPlanner(nil, reg, 20)
	p.SetCogniRuntime(decisionRuntimeStub{decision: agentcogni.CogniFinalDecision{
		SkillsNeeded: []string{}, // chat/empathy: no skills wanted
	}})

	defs := p.buildFunctionDefs(context.Background(), "今天心情不好，陪我聊聊", "t", "web", "", false, nil,
		p.ensureContextAssembly(), p.ensureDelegationRuntime(), p.ensureSkillRuntime())

	names := make([]string, 0, len(defs))
	for _, d := range defs {
		names = append(names, d.Name)
	}
	assertExcludesAll(t, names, "web_search", "file_read", "browser_navigate", "deep_research")
}

// TestBuildFunctionDefs_V2NoOpinionFallsBackToNative proves "原生兜底": when
// IntentCogni returns SkillsNeeded == nil (complex/unknown intent), v2 does not
// narrow and the full ready surface is exposed for the native scorer to rank. The
// small registry (8 < 10) skips the native dynamic filter, so all 8 survive —
// proving v2 did NOT narrow.
func TestBuildFunctionDefs_V2NoOpinionFallsBackToNative(t *testing.T) {
	reg := buildExpandRegistry()
	p := NewPlanner(nil, reg, 20)
	p.SetCogniRuntime(decisionRuntimeStub{decision: agentcogni.CogniFinalDecision{
		SkillsNeeded: nil, // no opinion
	}})

	defs := p.buildFunctionDefs(context.Background(), "帮我规划一个复杂的多步任务", "t", "web", "", false, nil,
		p.ensureContextAssembly(), p.ensureDelegationRuntime(), p.ensureSkillRuntime())

	names := make([]string, 0, len(defs))
	for _, d := range defs {
		names = append(names, d.Name)
	}
	// Full registry surface preserved (no v2 narrowing): both file and research
	// categories present.
	assertContainsAll(t, names, "file_read", "file_write", "web_search", "browser_navigate", "code_execute")
}
