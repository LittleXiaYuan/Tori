package plugin

import (
	"context"
	"encoding/json"
	"os"
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

func TestAllIncludeDisabledShowsSlot(t *testing.T) {
	r := NewRegistry()
	r.RegisterWithSlot(&mockPlugin{name: "mem-redis"}, "memory")
	r.Register(&mockPlugin{name: "plain"})

	all := r.AllIncludeDisabled()
	for _, info := range all {
		if info.Name == "mem-redis" && info.Slot != "memory" {
			t.Fatalf("expected slot 'memory', got %q", info.Slot)
		}
		if info.Name == "plain" && info.Slot != "" {
			t.Fatalf("expected empty slot for plain plugin, got %q", info.Slot)
		}
	}
}

func TestLoaderUsesSlotFromManifest(t *testing.T) {
	dir := t.TempDir()

	writePluginManifest := func(name, slot string) string {
		pluginDir := dir + "/" + name
		os.MkdirAll(pluginDir, 0755)
		manifest := map[string]any{
			"name":        name,
			"description": "test",
			"language":    "python",
		}
		if slot != "" {
			manifest["slot"] = slot
		}
		data, _ := json.Marshal(manifest)
		os.WriteFile(pluginDir+"/plugin.json", data, 0644)
		return pluginDir
	}

	writePluginManifest("mem-a", "memory")
	writePluginManifest("mem-b", "memory")
	writePluginManifest("no-slot", "")

	reg := NewRegistry()
	loader := NewLoader(dir, reg, nil)
	loaded := loader.LoadAll()

	// mem-a should load with slot, mem-b should be rejected (slot conflict), no-slot should load normally
	if loaded != 2 {
		t.Fatalf("expected 2 loaded (mem-a + no-slot), got %d", loaded)
	}

	owner, ok := reg.SlotOwner("memory")
	if !ok || owner != "mem-a" {
		t.Fatalf("expected slot owner 'mem-a', got %q (ok=%v)", owner, ok)
	}

	_, okA := reg.Get("mem-a")
	_, okB := reg.Get("mem-b")
	_, okN := reg.Get("no-slot")
	if !okA {
		t.Fatal("mem-a should be registered")
	}
	if okB {
		t.Fatal("mem-b should NOT be registered (slot conflict)")
	}
	if !okN {
		t.Fatal("no-slot should be registered")
	}
}

func TestLoaderReloadPreservesSlot(t *testing.T) {
	dir := t.TempDir()
	pluginDir := dir + "/slotted"
	os.MkdirAll(pluginDir, 0755)
	manifest := map[string]any{
		"name":        "slotted",
		"description": "test",
		"language":    "python",
		"slot":        "channel-tg",
	}
	data, _ := json.Marshal(manifest)
	os.WriteFile(pluginDir+"/plugin.json", data, 0644)

	reg := NewRegistry()
	loader := NewLoader(dir, reg, nil)
	loader.LoadAll()

	owner, _ := reg.SlotOwner("channel-tg")
	if owner != "slotted" {
		t.Fatal("first load should own slot")
	}

	// Reload — same plugin re-registers same slot (should succeed)
	loaded := loader.LoadAll()
	if loaded != 1 {
		t.Fatalf("reload expected 1, got %d", loaded)
	}
	owner2, _ := reg.SlotOwner("channel-tg")
	if owner2 != "slotted" {
		t.Fatal("reload should preserve slot ownership")
	}
}
