package cogni

import (
	"context"
	"strings"
	"testing"

	"yunque-agent/pkg/skills"
)

func TestHook_NilRegistryReturnsNil(t *testing.T) {
	if h := NewHook(nil); h != nil {
		t.Fatalf("NewHook(nil) should return nil, got %+v", h)
	}
}

func TestHook_NilHookSafe(t *testing.T) {
	var h *Hook
	if got := h.BuildContext(ContextRequest{Message: "hi"}); got != "" {
		t.Fatalf("nil hook BuildContext should be safe, got %q", got)
	}
	if got := h.Activate(ContextRequest{Message: "hi"}); got != nil {
		t.Fatalf("nil hook Activate should be safe, got %+v", got)
	}
	if got := h.ActiveIDs(ContextRequest{Message: "hi"}); got != nil {
		t.Fatalf("nil hook ActiveIDs should be safe, got %+v", got)
	}
}

func TestHook_NoActiveCogniReturnsEmpty(t *testing.T) {
	r := NewRegistry()
	h := NewHook(r)
	if got := h.BuildContext(ContextRequest{Message: "anything"}); got != "" {
		t.Fatalf("empty registry should produce empty context, got %q", got)
	}
}

func TestHook_BuildContextStaticBlock(t *testing.T) {
	r := NewRegistry()
	_ = r.Add(&Declaration{
		ID:          "reviewer",
		DisplayName: "Code Reviewer",
		Activation: ActivationRules{
			Keywords: []string{"review"},
			MinScore: 0.2,
		},
		Context: ContextInjection{Static: "你是一名严格的代码审查员。"},
	}, "test")

	got := NewHook(r).BuildContext(ContextRequest{Message: "please review this PR"})
	if !strings.Contains(got, "## 智体上下文") {
		t.Fatalf("missing parent heading in: %q", got)
	}
	if !strings.Contains(got, "### Code Reviewer") {
		t.Fatalf("missing display-name heading in: %q", got)
	}
	if !strings.Contains(got, "你是一名严格的代码审查员") {
		t.Fatalf("missing static body in: %q", got)
	}
}

func TestHook_BuildContextSkipsInactive(t *testing.T) {
	r := NewRegistry()
	_ = r.Add(&Declaration{
		ID: "reviewer",
		Activation: ActivationRules{
			Keywords: []string{"review"},
			MinScore: 0.5, // requires two keyword hits
		},
		Context: ContextInjection{Static: "should not appear"},
	}, "test")

	got := NewHook(r).BuildContext(ContextRequest{Message: "no match here"})
	if got != "" {
		t.Fatalf("inactive cogni must not contribute, got %q", got)
	}
}

func TestHook_BuildContextRendersTemplate(t *testing.T) {
	r := NewRegistry()
	_ = r.Add(&Declaration{
		ID:          "router",
		DisplayName: "Router",
		Activation:  ActivationRules{AlwaysOn: true},
		Context: ContextInjection{
			Template: "用户在 {{.Channel}} 频道说: {{.Message}}",
		},
	}, "test")

	got := NewHook(r).BuildContext(ContextRequest{
		Message:  "hello",
		TenantID: "t1",
		Channel:  "webchat",
	})
	if !strings.Contains(got, "用户在 webchat 频道说: hello") {
		t.Fatalf("template not rendered: %q", got)
	}
}

func TestHook_TemplateFallbackToStatic(t *testing.T) {
	r := NewRegistry()
	_ = r.Add(&Declaration{
		ID:         "broken",
		Activation: ActivationRules{AlwaysOn: true},
		Context: ContextInjection{
			Static:   "fallback body",
			Template: "{{.Bad}}{{.Unclosed", // parse error
		},
	}, "test")

	got := NewHook(r).BuildContext(ContextRequest{Message: "x"})
	if !strings.Contains(got, "fallback body") {
		t.Fatalf("template parse error must fall back to Static: %q", got)
	}
}

func TestHook_BuildContextStacksMultipleCognis(t *testing.T) {
	r := NewRegistry()
	_ = r.Add(&Declaration{
		ID:         "always-a",
		Activation: ActivationRules{AlwaysOn: true},
		Context:    ContextInjection{Static: "block A"},
	}, "test")
	_ = r.Add(&Declaration{
		ID:         "always-b",
		Activation: ActivationRules{AlwaysOn: true},
		Context:    ContextInjection{Static: "block B"},
	}, "test")

	got := NewHook(r).BuildContext(ContextRequest{Message: "x"})
	if !strings.Contains(got, "block A") || !strings.Contains(got, "block B") {
		t.Fatalf("expected both blocks present in %q", got)
	}
}

func TestHook_ExclusivityAppliesInActivate(t *testing.T) {
	r := NewRegistry()
	_ = r.Add(&Declaration{
		ID:        "lo",
		Exclusive: "g",
		Activation: ActivationRules{
			Keywords:      []string{"x"},
			KeywordWeight: 0.3,
			MinScore:      0.2,
		},
	}, "test")
	_ = r.Add(&Declaration{
		ID:        "hi",
		Exclusive: "g",
		Activation: ActivationRules{
			Keywords:      []string{"x"},
			KeywordWeight: 0.9,
			MinScore:      0.2,
		},
	}, "test")

	ids := NewHook(r).ActiveIDs(ContextRequest{Message: "x"})
	if len(ids) != 1 || ids[0] != "hi" {
		t.Fatalf("exclusivity must keep only highest-scoring (hi), got %v", ids)
	}
}

// FilterSkills coverage

func toolSet(in []skills.Skill) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = s.Name()
	}
	return out
}

func contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}

func TestHook_FilterSkills_NilHookIdentity(t *testing.T) {
	var h *Hook
	in := []skills.Skill{sk("a")}
	out := h.FilterSkills(ContextRequest{Message: "x"}, in)
	if len(out) != 1 || out[0].Name() != "a" {
		t.Fatalf("nil hook must return input unchanged, got %v", toolSet(out))
	}
}

func TestHook_FilterSkills_NoActiveCogniIsIdentity(t *testing.T) {
	r := NewRegistry()
	h := NewHook(r)
	in := []skills.Skill{sk("a"), sk("b")}
	out := h.FilterSkills(ContextRequest{Message: "x"}, in)
	if !equal(toolSet(out), []string{"a", "b"}) {
		t.Fatalf("no cogni → identity, got %v", toolSet(out))
	}
}

func TestHook_FilterSkills_OnlyNarrowsSurface(t *testing.T) {
	r := NewRegistry()
	_ = r.Add(&Declaration{
		ID:         "narrow",
		Activation: ActivationRules{AlwaysOn: true},
		Surface:    ToolSurface{Only: []string{"github_get_diff"}},
	}, "test")

	h := NewHook(r)
	in := []skills.Skill{sk("github_get_diff"), sk("github_post_comment"), sk("file_read")}
	out := h.FilterSkills(ContextRequest{Message: "x"}, in)
	if !equal(toolSet(out), []string{"github_get_diff"}) {
		t.Fatalf("Only must restrict to listed names, got %v", toolSet(out))
	}
}

func TestHook_SurfaceAuthoritative(t *testing.T) {
	// Identity surface (no narrowing) → not authoritative.
	rIdentity := NewRegistry()
	_ = rIdentity.Add(&Declaration{
		ID:         "ambient",
		Activation: ActivationRules{AlwaysOn: true},
	}, "test")
	if NewHook(rIdentity).SurfaceAuthoritative(ContextRequest{Message: "x"}) {
		t.Fatal("identity surface must not be authoritative")
	}

	// Non-identity surface (Only) → authoritative.
	rNarrow := NewRegistry()
	_ = rNarrow.Add(&Declaration{
		ID:         "narrow",
		Activation: ActivationRules{AlwaysOn: true},
		Surface:    ToolSurface{Only: []string{"github_get_diff"}},
	}, "test")
	if !NewHook(rNarrow).SurfaceAuthoritative(ContextRequest{Message: "x"}) {
		t.Fatal("non-identity surface must be authoritative")
	}

	// No cogni activates → not authoritative.
	rInactive := NewRegistry()
	_ = rInactive.Add(&Declaration{
		ID:         "kw",
		Activation: ActivationRules{Keywords: []string{"deploy"}},
		Surface:    ToolSurface{Only: []string{"shell"}},
	}, "test")
	if NewHook(rInactive).SurfaceAuthoritative(ContextRequest{Message: "unrelated message"}) {
		t.Fatal("inactive cogni must not be authoritative")
	}

	// Nil hook is safe.
	var nilHook *Hook
	if nilHook.SurfaceAuthoritative(ContextRequest{Message: "x"}) {
		t.Fatal("nil hook must not be authoritative")
	}
}

func TestHook_ExperiencePrunesLowSuccessTool(t *testing.T) {
	r := NewRegistry()
	_ = r.Add(&Declaration{
		ID:         "reviewer",
		Activation: ActivationRules{AlwaysOn: true},
		Surface:    ToolSurface{Only: []string{"good_tool", "bad_tool"}},
	}, "test")

	store := NewExperienceStore("reviewer", ExperienceConfig{Enabled: true, StoreDir: t.TempDir()})
	defer store.Flush()
	h := NewHook(r)
	h.SetExperienceProvider(func(id string) *ExperienceStore {
		if id == "reviewer" {
			return store
		}
		return nil
	})

	in := []skills.Skill{sk("good_tool"), sk("bad_tool")}

	// Before tuning is enabled: both tools survive even with bad data.
	for i := 0; i < 5; i++ {
		store.RecordToolOutcome("bad_tool", false)
		store.RecordToolOutcome("good_tool", true)
	}
	if got := toolSet(h.FilterSkills(ContextRequest{Message: "x"}, in)); !equal(got, []string{"good_tool", "bad_tool"}) {
		t.Fatalf("tuning disabled should keep both tools, got %v", got)
	}

	// Enable tuning → bad_tool (0%% over 5 obs) is pruned, good_tool stays.
	h.SetExperienceTuning(ExperienceTuningConfig{MinObservations: 3, MinSuccessRate: 0.4})
	got := toolSet(h.FilterSkills(ContextRequest{Message: "x"}, in))
	if !equal(got, []string{"good_tool"}) {
		t.Fatalf("low-success tool should be pruned, got %v", got)
	}
}

func TestHook_ExperiencePruneNeverEmptiesSurface(t *testing.T) {
	r := NewRegistry()
	_ = r.Add(&Declaration{
		ID:         "solo",
		Activation: ActivationRules{AlwaysOn: true},
		Surface:    ToolSurface{Only: []string{"only_tool"}},
	}, "test")
	store := NewExperienceStore("solo", ExperienceConfig{Enabled: true, StoreDir: t.TempDir()})
	defer store.Flush()
	for i := 0; i < 5; i++ {
		store.RecordToolOutcome("only_tool", false)
	}
	h := NewHook(r)
	h.SetExperienceProvider(func(string) *ExperienceStore { return store })
	h.SetExperienceTuning(ExperienceTuningConfig{MinObservations: 3, MinSuccessRate: 0.5})

	// Pruning the only tool would empty the surface → must keep it.
	got := toolSet(h.FilterSkills(ContextRequest{Message: "x"}, []skills.Skill{sk("only_tool")}))
	if !equal(got, []string{"only_tool"}) {
		t.Fatalf("must never prune surface to empty, got %v", got)
	}
}

func TestHook_ArbitrationCapsActiveCognis(t *testing.T) {
	r := NewRegistry()
	// Three always-on cognis (all score 1.0) with distinct priorities.
	_ = r.Add(&Declaration{ID: "p_high", Activation: ActivationRules{AlwaysOn: true}, Priority: 1}, "test")
	_ = r.Add(&Declaration{ID: "p_mid", Activation: ActivationRules{AlwaysOn: true}, Priority: 5}, "test")
	_ = r.Add(&Declaration{ID: "p_low", Activation: ActivationRules{AlwaysOn: true}, Priority: 9}, "test")

	// Default (no arbitration) → all three compose.
	hAll := NewHook(r)
	if got := hAll.ActiveIDs(ContextRequest{Message: "x"}); len(got) != 3 {
		t.Fatalf("default hook should activate all 3, got %v", got)
	}

	// MaxActive=2 → only the two best (lowest priority numbers) win.
	hCap := NewHook(r)
	hCap.SetArbitration(ArbitrationConfig{MaxActive: 2})
	got := hCap.ActiveIDs(ContextRequest{Message: "x"})
	if len(got) != 2 {
		t.Fatalf("arbitration should cap to 2 cognis, got %v", got)
	}
	if !contains(got, "p_high") || !contains(got, "p_mid") {
		t.Fatalf("arbitration should keep the two highest-priority cognis, got %v", got)
	}
	if contains(got, "p_low") {
		t.Fatalf("lowest-priority cogni should be capped out, got %v", got)
	}
}

func TestHook_FilterSkills_UnionAcrossActivatedCognis(t *testing.T) {
	r := NewRegistry()
	_ = r.Add(&Declaration{
		ID:         "a",
		Activation: ActivationRules{AlwaysOn: true},
		Surface:    ToolSurface{Only: []string{"x", "y"}},
	}, "test")
	_ = r.Add(&Declaration{
		ID:         "b",
		Activation: ActivationRules{AlwaysOn: true},
		Surface:    ToolSurface{Only: []string{"y", "z"}},
	}, "test")

	h := NewHook(r)
	in := []skills.Skill{sk("x"), sk("y"), sk("z"), sk("ignored")}
	out := h.FilterSkills(ContextRequest{Message: "anything"}, in)
	names := toolSet(out)
	if !contains(names, "x") || !contains(names, "y") || !contains(names, "z") {
		t.Fatalf("union must include x, y, z; got %v", names)
	}
	if contains(names, "ignored") {
		t.Fatalf("ignored must be filtered out; got %v", names)
	}
}

func TestHook_FilterSkills_IdentitySurfacePreservesAll(t *testing.T) {
	r := NewRegistry()
	_ = r.Add(&Declaration{
		ID:         "noop",
		Activation: ActivationRules{AlwaysOn: true},
		// no Surface fields set → identity
	}, "test")

	h := NewHook(r)
	in := []skills.Skill{sk("a"), sk("b"), sk("c")}
	out := h.FilterSkills(ContextRequest{Message: "x"}, in)
	if !equal(toolSet(out), []string{"a", "b", "c"}) {
		t.Fatalf("identity surface must keep input as-is, got %v", toolSet(out))
	}
}

func TestHook_FilterSkills_EmptyUnionFallsBackToInput(t *testing.T) {
	r := NewRegistry()
	_ = r.Add(&Declaration{
		ID:         "impossible",
		Activation: ActivationRules{AlwaysOn: true},
		Surface:    ToolSurface{Only: []string{"never_exists"}},
	}, "test")

	h := NewHook(r)
	in := []skills.Skill{sk("a"), sk("b")}
	out := h.FilterSkills(ContextRequest{Message: "x"}, in)
	if !equal(toolSet(out), []string{"a", "b"}) {
		t.Fatalf("empty union must fall back to input to avoid locking out the model, got %v", toolSet(out))
	}
}

func TestHook_FilterSkills_FromCapsulesRequiresOwnerFn(t *testing.T) {
	r := NewRegistry()
	_ = r.Add(&Declaration{
		ID:         "github",
		Activation: ActivationRules{AlwaysOn: true},
		Surface:    ToolSurface{FromCapsules: []string{"github"}},
	}, "test")

	h := NewHook(r)
	in := []skills.Skill{sk("github_get_diff"), sk("file_read")}
	// Without SkillOwner, FromCapsules is inert → union is empty → fallback
	// to input (prior behaviour, still preserved).
	out := h.FilterSkills(ContextRequest{Message: "x"}, in)
	if len(out) != 2 {
		t.Fatalf("without SkillOwner, FromCapsules should fall back to input; got %v", toolSet(out))
	}

	// With a SkillOwner that maps names to capsules, FromCapsules narrows.
	h.SetSkillOwner(func(name string) string {
		if strings.HasPrefix(name, "github_") {
			return "github"
		}
		return "filesystem"
	})
	out = h.FilterSkills(ContextRequest{Message: "x"}, in)
	if !equal(toolSet(out), []string{"github_get_diff"}) {
		t.Fatalf("FromCapsules with SkillOwner should narrow to github_*, got %v", toolSet(out))
	}
}

func TestHook_FilterSkills_ExcludeRemovesNamed(t *testing.T) {
	r := NewRegistry()
	_ = r.Add(&Declaration{
		ID:         "guard",
		Activation: ActivationRules{AlwaysOn: true},
		Surface:    ToolSurface{Exclude: []string{"shell_exec"}},
	}, "test")

	h := NewHook(r)
	in := []skills.Skill{sk("shell_exec"), sk("file_read"), sk("file_write")}
	out := h.FilterSkills(ContextRequest{Message: "x"}, in)
	names := toolSet(out)
	if contains(names, "shell_exec") {
		t.Fatalf("Exclude must remove shell_exec; got %v", names)
	}
	if !contains(names, "file_read") || !contains(names, "file_write") {
		t.Fatalf("Exclude must keep other skills; got %v", names)
	}
}

// MemoryQuery coverage

func TestHook_MemoryQuery_SkippedWhenNoCallback(t *testing.T) {
	r := NewRegistry()
	_ = r.Add(&Declaration{
		ID:         "m",
		Activation: ActivationRules{AlwaysOn: true},
		Context:    ContextInjection{Static: "base", MemoryQuery: "ctx for {message}"},
	}, "test")

	got := NewHook(r).BuildContext(ContextRequest{Message: "hello"})
	if !strings.Contains(got, "base") {
		t.Fatalf("static body must still render: %q", got)
	}
	if strings.Contains(got, "相关记忆") {
		t.Fatalf("no callback → no recall block, got %q", got)
	}
}

func TestHook_MemoryQuery_SubstitutesMessage(t *testing.T) {
	r := NewRegistry()
	_ = r.Add(&Declaration{
		ID:         "m",
		Activation: ActivationRules{AlwaysOn: true},
		Context:    ContextInjection{MemoryQuery: "context for {message}"},
	}, "test")

	var seenQuery string
	h := NewHook(r)
	h.SetMemorySearch(func(_ context.Context, _ string, q string) string {
		seenQuery = q
		return "recalled facts about hello"
	})

	got := h.BuildContext(ContextRequest{Message: "hello", TenantID: "t1"})
	if seenQuery != "context for hello" {
		t.Fatalf("placeholder must be substituted, got %q", seenQuery)
	}
	if !strings.Contains(got, "#### 相关记忆") {
		t.Fatalf("recall block must be injected under header: %q", got)
	}
	if !strings.Contains(got, "recalled facts about hello") {
		t.Fatalf("recalled body missing: %q", got)
	}
}

func TestHook_MemoryQuery_EmptyRecallDoesNotAddHeader(t *testing.T) {
	r := NewRegistry()
	_ = r.Add(&Declaration{
		ID:         "m",
		Activation: ActivationRules{AlwaysOn: true},
		Context:    ContextInjection{Static: "static body", MemoryQuery: "ignored"},
	}, "test")

	h := NewHook(r)
	h.SetMemorySearch(func(_ context.Context, _ string, _ string) string { return "  " })

	got := h.BuildContext(ContextRequest{Message: "x"})
	if strings.Contains(got, "相关记忆") {
		t.Fatalf("empty recall must not add header: %q", got)
	}
	if !strings.Contains(got, "static body") {
		t.Fatalf("static body must still be present: %q", got)
	}
}

func TestHook_MemoryQuery_StacksOnTopOfStatic(t *testing.T) {
	r := NewRegistry()
	_ = r.Add(&Declaration{
		ID:         "m",
		Activation: ActivationRules{AlwaysOn: true},
		Context:    ContextInjection{Static: "base prompt", MemoryQuery: "query"},
	}, "test")

	h := NewHook(r)
	h.SetMemorySearch(func(_ context.Context, _ string, _ string) string { return "facts" })
	got := h.BuildContext(ContextRequest{Message: "x"})

	if !strings.Contains(got, "base prompt") || !strings.Contains(got, "facts") {
		t.Fatalf("both static and recall should appear: %q", got)
	}
	// static before recall
	if strings.Index(got, "base prompt") > strings.Index(got, "facts") {
		t.Fatalf("static must render before recall: %q", got)
	}
}

func TestHook_ContextSkippedForDisabledRegistration(t *testing.T) {
	r := NewRegistry()
	_ = r.Add(&Declaration{
		ID:         "off",
		Activation: ActivationRules{AlwaysOn: true},
		Context:    ContextInjection{Static: "should not appear"},
	}, "test")
	if err := r.SetEnabled("off", false); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if got := NewHook(r).BuildContext(ContextRequest{Message: "x"}); got != "" {
		t.Fatalf("disabled cogni must not contribute, got %q", got)
	}
}
