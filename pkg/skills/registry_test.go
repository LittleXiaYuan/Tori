package skills

import (
	"context"
	"testing"
)

type testSkill struct{ name string }

func (s testSkill) Name() string               { return s.name }
func (s testSkill) Description() string        { return s.name + " desc" }
func (s testSkill) Parameters() map[string]any { return map[string]any{"type": "object"} }
func (s testSkill) Execute(context.Context, map[string]any, *Environment) (string, error) {
	return "", nil
}

func names(r *Registry) map[string]bool {
	out := map[string]bool{}
	for _, s := range r.All() {
		out[s.Name()] = true
	}
	return out
}

// TestReplaceAll_PreservesRegisteredSkills is the regression guard for the
// hot-reload data-loss bug: ReplaceAll swaps the plugin baseline but must keep
// skills added via Register (MCP, file, marketplace, dynamic, generate_skill).
func TestReplaceAll_PreservesRegisteredSkills(t *testing.T) {
	r := NewRegistry()

	// Boot: plugin baseline, then post-init Register'd extras (MCP/file/etc).
	r.ReplaceAll([]Skill{testSkill{"plugin_a"}, testSkill{"plugin_b"}})
	r.Register(testSkill{"mcp_tool"})
	r.Register(testSkill{"file_skill"})

	got := names(r)
	for _, want := range []string{"plugin_a", "plugin_b", "mcp_tool", "file_skill"} {
		if !got[want] {
			t.Fatalf("after boot, missing %q: %v", want, got)
		}
	}

	// Plugin hot-reload / admin toggle: new baseline, plugin_b gone, plugin_c new.
	r.ReplaceAll([]Skill{testSkill{"plugin_a"}, testSkill{"plugin_c"}})
	got = names(r)

	// Registered extras MUST survive the baseline swap.
	if !got["mcp_tool"] || !got["file_skill"] {
		t.Fatalf("ReplaceAll wiped Register'd skills: %v", got)
	}
	// New baseline applied.
	if !got["plugin_a"] || !got["plugin_c"] {
		t.Fatalf("new baseline not applied: %v", got)
	}
	// Old plugin removed (plugins stay swappable).
	if got["plugin_b"] {
		t.Fatalf("stale plugin should be swapped out: %v", got)
	}
}

func TestRemove_UnpinsSkill(t *testing.T) {
	r := NewRegistry()
	r.ReplaceAll([]Skill{testSkill{"plugin_a"}})
	r.Register(testSkill{"mcp_tool"})

	r.Remove("mcp_tool")
	// After Remove, a later ReplaceAll must NOT resurrect the removed skill.
	r.ReplaceAll([]Skill{testSkill{"plugin_a"}})
	if names(r)["mcp_tool"] {
		t.Fatalf("removed skill should not be resurrected by ReplaceAll")
	}
}

func TestClear_DropsPins(t *testing.T) {
	r := NewRegistry()
	r.Register(testSkill{"mcp_tool"})
	r.Clear()
	r.ReplaceAll([]Skill{testSkill{"plugin_a"}})
	if names(r)["mcp_tool"] {
		t.Fatalf("Clear should drop pinned skills; got resurrection")
	}
}

func TestUnbackedIntentBuckets(t *testing.T) {
	r := NewRegistry()
	// No categories defined → every intent keyword bucket is inert.
	if got := len(r.UnbackedIntentBuckets()); got == 0 {
		t.Fatal("with no categories, all intent buckets should be reported unbacked")
	}

	// Define browser + connector (production reality) → they drop out of the list,
	// but file/image/research/workflow remain unbacked.
	r.DefineCategory(SkillCategory{ID: "browser", Name: "browser"})
	r.DefineCategory(SkillCategory{ID: "connector", Name: "connector"})
	dead := map[string]bool{}
	for _, b := range r.UnbackedIntentBuckets() {
		dead[b] = true
	}
	if dead["browser"] || dead["connector"] {
		t.Fatalf("defined categories must not be reported unbacked: %v", dead)
	}
	for _, want := range []string{"file", "image", "research", "workflow"} {
		if !dead[want] {
			t.Fatalf("expected %q to be reported as unbacked intent bucket: %v", want, dead)
		}
	}
}

func TestReplaceAll_PinnedWinsOnCollision(t *testing.T) {
	r := NewRegistry()
	r.Register(testSkill{"shared"}) // pinned version
	r.ReplaceAll([]Skill{testSkill{"shared"}, testSkill{"plugin_a"}})
	// Both present; pinned overlay keeps "shared" available (no panic / no drop).
	if !names(r)["shared"] || !names(r)["plugin_a"] {
		t.Fatalf("collision handling wrong: %v", names(r))
	}
}
