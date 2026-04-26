package cogni

import (
	"context"
	"testing"

	"yunque-agent/pkg/plugin"
	"yunque-agent/pkg/skills"
)

type stubPlugin struct {
	name   string
	desc   string
	prompt string
	skills []skills.Skill
}

func (p *stubPlugin) Name() string          { return p.name }
func (p *stubPlugin) Description() string   { return p.desc }
func (p *stubPlugin) Skills() []skills.Skill { return p.skills }
func (p *stubPlugin) SystemPrompt() string  { return p.prompt }

type stubSkill struct {
	name string
	desc string
}

func (s *stubSkill) Name() string                { return s.name }
func (s *stubSkill) Description() string         { return s.desc }
func (s *stubSkill) Parameters() map[string]any  { return nil }
func (s *stubSkill) Execute(_ context.Context, _ map[string]any, _ *skills.Environment) (string, error) {
	return "ok", nil
}

func TestPluginToDeclaration(t *testing.T) {
	p := &stubPlugin{
		name:   "test-plugin",
		desc:   "Test Plugin",
		prompt: "You are a test assistant.",
		skills: []skills.Skill{
			&stubSkill{name: "skill_a", desc: "Skill A"},
			&stubSkill{name: "skill_b", desc: "Skill B"},
		},
	}

	d := PluginToDeclaration(p)
	if d == nil {
		t.Fatal("expected non-nil declaration")
	}

	if d.ID != "plugin:test-plugin" {
		t.Errorf("expected id 'plugin:test-plugin', got %q", d.ID)
	}
	if !d.Activation.AlwaysOn {
		t.Error("expected always_on to be true")
	}
	if d.Priority != 200 {
		t.Errorf("expected priority 200, got %d", d.Priority)
	}
	if d.Context.Static != "You are a test assistant." {
		t.Errorf("unexpected static context: %q", d.Context.Static)
	}
	if len(d.Surface.Include) != 2 {
		t.Errorf("expected 2 included skills, got %d", len(d.Surface.Include))
	}
}

func TestPluginToDeclarationNil(t *testing.T) {
	d := PluginToDeclaration(nil)
	if d != nil {
		t.Error("expected nil for nil plugin")
	}
}

func TestRegisterPlugins(t *testing.T) {
	reg := NewRegistry()
	plugins := []plugin.Plugin{
		&stubPlugin{name: "p1", desc: "Plugin 1", prompt: "prompt 1"},
		&stubPlugin{name: "p2", desc: "Plugin 2", prompt: "prompt 2"},
	}

	RegisterPlugins(reg, plugins)

	list := reg.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(list))
	}

	for _, e := range list {
		if e.Source != "plugin-adapter" {
			t.Errorf("expected source 'plugin-adapter', got %q", e.Source)
		}
	}
}
