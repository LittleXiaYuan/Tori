package cogni

import (
	"context"
	"testing"

	"yunque-agent/pkg/skills"
)

// fakeSkill is a minimal skills.Skill suitable for unit tests; it does
// not need Execute to do anything because Surface only inspects Name().
type fakeSkill struct {
	name string
}

func (f *fakeSkill) Name() string               { return f.name }
func (f *fakeSkill) Description() string        { return "" }
func (f *fakeSkill) Parameters() map[string]any { return nil }
func (f *fakeSkill) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	return "", nil
}

func sk(name string) skills.Skill { return &fakeSkill{name: name} }

func TestSurface_EmptyReturnsNilOnEmptyInput(t *testing.T) {
	if out := Surface(nil, ToolSurface{}); out != nil {
		t.Fatalf("nil candidates should return nil, got %v", out)
	}
}

func TestSurface_IdentityPreservesAllUnique(t *testing.T) {
	in := []SurfaceInput{
		{Skill: sk("a"), Capsule: "x"},
		{Skill: sk("b"), Capsule: "x"},
		{Skill: sk("c"), Capsule: "y"},
	}
	out := Surface(in, ToolSurface{})
	if names := skillNames(out); !equal(names, []string{"a", "b", "c"}) {
		t.Fatalf("identity surface must preserve order, got %v", names)
	}
}

func TestSurface_DedupesByName(t *testing.T) {
	in := []SurfaceInput{
		{Skill: sk("a"), Capsule: "x"},
		{Skill: sk("a"), Capsule: "y"},
		{Skill: sk("b"), Capsule: "x"},
	}
	out := Surface(in, ToolSurface{})
	if names := skillNames(out); !equal(names, []string{"a", "b"}) {
		t.Fatalf("duplicate skill names must be deduped, got %v", names)
	}
}

func TestSurface_FromCapsulesNarrows(t *testing.T) {
	in := []SurfaceInput{
		{Skill: sk("a"), Capsule: "x"},
		{Skill: sk("b"), Capsule: "y"},
		{Skill: sk("c"), Capsule: "z"},
	}
	out := Surface(in, ToolSurface{FromCapsules: []string{"x", "z"}})
	if names := skillNames(out); !equal(names, []string{"a", "c"}) {
		t.Fatalf("FromCapsules filter wrong: got %v", names)
	}
}

func TestSurface_OnlyRestrictsByName(t *testing.T) {
	in := []SurfaceInput{
		{Skill: sk("a"), Capsule: "x"},
		{Skill: sk("b"), Capsule: "x"},
		{Skill: sk("c"), Capsule: "x"},
	}
	out := Surface(in, ToolSurface{Only: []string{"a", "c"}})
	if names := skillNames(out); !equal(names, []string{"a", "c"}) {
		t.Fatalf("Only filter wrong: got %v", names)
	}
}

func TestSurface_ExcludeRemoves(t *testing.T) {
	in := []SurfaceInput{
		{Skill: sk("a")}, {Skill: sk("b")}, {Skill: sk("c")},
	}
	out := Surface(in, ToolSurface{Exclude: []string{"b"}})
	if names := skillNames(out); !equal(names, []string{"a", "c"}) {
		t.Fatalf("Exclude wrong: got %v", names)
	}
}

func TestSurface_IncludeReAddsSkillExcludedByOnly(t *testing.T) {
	in := []SurfaceInput{
		{Skill: sk("a")}, {Skill: sk("b")}, {Skill: sk("c")},
	}
	// Only would normally drop "c"; Include re-adds it.
	out := Surface(in, ToolSurface{
		Only:    []string{"a"},
		Include: []string{"c"},
	})
	if names := skillNames(out); !equal(names, []string{"a", "c"}) {
		t.Fatalf("Include must re-add filtered names: got %v", names)
	}
}

func TestSurface_IncludeIsIdempotent(t *testing.T) {
	in := []SurfaceInput{{Skill: sk("a")}}
	out := Surface(in, ToolSurface{Include: []string{"a"}})
	if names := skillNames(out); !equal(names, []string{"a"}) {
		t.Fatalf("Include must not duplicate already-present skill: got %v", names)
	}
}

func TestSurface_MaxToolsCaps(t *testing.T) {
	in := []SurfaceInput{
		{Skill: sk("a")}, {Skill: sk("b")}, {Skill: sk("c")}, {Skill: sk("d")},
	}
	out := Surface(in, ToolSurface{MaxTools: 2})
	if names := skillNames(out); !equal(names, []string{"a", "b"}) {
		t.Fatalf("MaxTools must cap and preserve order: got %v", names)
	}
}

func TestSurface_FullPipeline(t *testing.T) {
	in := []SurfaceInput{
		{Skill: sk("alpha"), Capsule: "x"},
		{Skill: sk("beta"), Capsule: "x"},
		{Skill: sk("gamma"), Capsule: "y"},
		{Skill: sk("delta"), Capsule: "y"},
		{Skill: sk("eps"), Capsule: "z"},
	}
	// FromCapsules narrows to {x, y}: alpha, beta, gamma, delta
	// Only restricts to {alpha, gamma}: alpha, gamma
	// Exclude drops {alpha}: gamma
	// Include adds {eps}: gamma, eps
	// MaxTools = 5, no cap
	out := Surface(in, ToolSurface{
		FromCapsules: []string{"x", "y"},
		Only:         []string{"alpha", "gamma"},
		Exclude:      []string{"alpha"},
		Include:      []string{"eps"},
		MaxTools:     5,
	})
	if names := skillNames(out); !equal(names, []string{"gamma", "eps"}) {
		t.Fatalf("full-pipeline surface wrong: got %v", names)
	}
}

func TestMergeSurfaces_DedupesAcrossInputs(t *testing.T) {
	a := []skills.Skill{sk("a"), sk("b")}
	b := []skills.Skill{sk("b"), sk("c")}
	c := []skills.Skill{sk("d")}
	out := MergeSurfaces(a, b, c)
	if names := skillNames(out); !equal(names, []string{"a", "b", "c", "d"}) {
		t.Fatalf("MergeSurfaces wrong: got %v", names)
	}
}

func TestMergeSurfaces_EmptyInputsSafe(t *testing.T) {
	if out := MergeSurfaces(); out != nil {
		t.Fatalf("MergeSurfaces() should be nil-safe; got %v", out)
	}
	if out := MergeSurfaces(nil, nil); out != nil {
		t.Fatalf("MergeSurfaces(nil, nil) should be nil-safe; got %v", out)
	}
}

func TestSurfaceMCPTools_OnlyRestricts(t *testing.T) {
	in := []MCPToolInfo{
		{Name: "create_issue"}, {Name: "list_pull_requests"}, {Name: "delete_repository"},
	}
	out := SurfaceMCPTools(in, ToolSurface{Only: []string{"create_issue"}})
	if names := mcpToolNames(out); !equal(names, []string{"create_issue"}) {
		t.Fatalf("Only filter wrong for MCP: got %v", names)
	}
}

func TestSurfaceMCPTools_ExcludeAndInclude(t *testing.T) {
	in := []MCPToolInfo{{Name: "a"}, {Name: "b"}, {Name: "c"}}
	out := SurfaceMCPTools(in, ToolSurface{
		Only:    []string{"a"},
		Include: []string{"c"},
	})
	if names := mcpToolNames(out); !equal(names, []string{"a", "c"}) {
		t.Fatalf("Include must re-add MCP tools: got %v", names)
	}
}

func TestSurfaceMCPTools_MaxToolsCaps(t *testing.T) {
	in := []MCPToolInfo{{Name: "a"}, {Name: "b"}, {Name: "c"}}
	out := SurfaceMCPTools(in, ToolSurface{MaxTools: 2})
	if names := mcpToolNames(out); !equal(names, []string{"a", "b"}) {
		t.Fatalf("MaxTools wrong for MCP: got %v", names)
	}
}

func TestMergeMCPTools_DedupesByName(t *testing.T) {
	a := []MCPToolInfo{{Name: "shared", Server: "s1"}, {Name: "only-a", Server: "s1"}}
	b := []MCPToolInfo{{Name: "shared", Server: "s2"}, {Name: "only-b", Server: "s2"}}
	out := MergeMCPTools(a, b)
	if names := mcpToolNames(out); !equal(names, []string{"shared", "only-a", "only-b"}) {
		t.Fatalf("MergeMCPTools wrong: got %v", names)
	}
	if out[0].Server != "s1" {
		t.Fatalf("first cogni should win on name collision, got server %q", out[0].Server)
	}
}

// helpers

func skillNames(in []skills.Skill) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		out = append(out, s.Name())
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
