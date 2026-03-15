package plugin

import (
	"context"
	"testing"

	"yunque-agent/pkg/skills"
)

// mockPlugin implements Plugin for testing.
type mockPlugin struct {
	name   string
	desc   string
	prompt string
	skills []skills.Skill
}

func (m *mockPlugin) Name() string           { return m.name }
func (m *mockPlugin) Description() string    { return m.desc }
func (m *mockPlugin) SystemPrompt() string   { return m.prompt }
func (m *mockPlugin) Skills() []skills.Skill { return m.skills }

type dummySkill struct{ name string }

func (d *dummySkill) Name() string               { return d.name }
func (d *dummySkill) Description() string        { return "test" }
func (d *dummySkill) Parameters() map[string]any { return nil }
func (d *dummySkill) Execute(_ context.Context, _ map[string]any, _ *skills.Environment) (string, error) {
	return "ok", nil
}

func TestRegisterAndGet(t *testing.T) {
	r := NewRegistry()
	p := &mockPlugin{name: "test", desc: "test plugin"}
	r.Register(p)
	got, ok := r.Get("test")
	if !ok || got.Name() != "test" {
		t.Fatal("expected to find plugin")
	}
}

func TestSetEnabled(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockPlugin{name: "p1"})
	if !r.IsEnabled("p1") {
		t.Fatal("should be enabled by default")
	}
	r.SetEnabled("p1", false)
	if r.IsEnabled("p1") {
		t.Fatal("should be disabled")
	}
	// All() should not include disabled
	if len(r.All()) != 0 {
		t.Fatal("disabled plugin should not appear in All()")
	}
}

func TestUnregister(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockPlugin{name: "p1"})
	r.Unregister("p1")
	_, ok := r.Get("p1")
	if ok {
		t.Fatal("should not find unregistered plugin")
	}
}

func TestAllSkillsFiltersDisabled(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockPlugin{name: "p1", skills: []skills.Skill{&dummySkill{"s1"}}})
	r.Register(&mockPlugin{name: "p2", skills: []skills.Skill{&dummySkill{"s2"}, &dummySkill{"s3"}}})
	if len(r.AllSkills()) != 3 {
		t.Fatalf("expected 3 skills, got %d", len(r.AllSkills()))
	}
	r.SetEnabled("p2", false)
	if len(r.AllSkills()) != 1 {
		t.Fatalf("expected 1 skill after disabling p2, got %d", len(r.AllSkills()))
	}
}

func TestCombinedPromptFiltersDisabled(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockPlugin{name: "p1", prompt: "prompt1"})
	r.Register(&mockPlugin{name: "p2", prompt: "prompt2"})
	combined := r.CombinedPrompt()
	if combined == "" {
		t.Fatal("expected non-empty combined prompt")
	}
	r.SetEnabled("p2", false)
	combined2 := r.CombinedPrompt()
	if len(combined2) >= len(combined) {
		t.Fatal("disabling p2 should reduce prompt length")
	}
}

func TestRegisterWithSlot(t *testing.T) {
	r := NewRegistry()
	p1 := &mockPlugin{name: "memory-sqlite"}
	p2 := &mockPlugin{name: "memory-pg"}

	if err := r.RegisterWithSlot(p1, "memory"); err != nil {
		t.Fatalf("first slot register failed: %v", err)
	}
	owner, ok := r.SlotOwner("memory")
	if !ok || owner != "memory-sqlite" {
		t.Fatalf("expected memory-sqlite, got %s", owner)
	}

	// Same slot should be rejected
	if err := r.RegisterWithSlot(p2, "memory"); err == nil {
		t.Fatal("expected slot conflict error")
	}

	// Unregister frees the slot
	r.Unregister("memory-sqlite")
	if _, ok := r.SlotOwner("memory"); ok {
		t.Fatal("slot should be freed after unregister")
	}

	// Now p2 can take the slot
	if err := r.RegisterWithSlot(p2, "memory"); err != nil {
		t.Fatalf("slot should be available: %v", err)
	}
}

func TestSlotsMap(t *testing.T) {
	r := NewRegistry()
	r.RegisterWithSlot(&mockPlugin{name: "ch-tg"}, "channel-telegram")
	r.RegisterWithSlot(&mockPlugin{name: "ch-dc"}, "channel-discord")
	slots := r.Slots()
	if len(slots) != 2 {
		t.Fatalf("expected 2 slots, got %d", len(slots))
	}
	if slots["channel-telegram"] != "ch-tg" {
		t.Fatal("wrong slot owner")
	}
}

func TestAllIncludeDisabled(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockPlugin{name: "p1", desc: "d1", skills: []skills.Skill{&dummySkill{"s1"}}})
	r.Register(&mockPlugin{name: "p2", desc: "d2"})
	r.SetEnabled("p2", false)
	all := r.AllIncludeDisabled()
	if len(all) != 2 {
		t.Fatalf("expected 2 plugins, got %d", len(all))
	}
	for _, info := range all {
		if info.Name == "p2" && info.Enabled {
			t.Fatal("p2 should be disabled")
		}
	}
}
