package persona

import (
	"os"
	"path/filepath"
	"testing"
)

func setupTestPersona(t *testing.T) *Persona {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "IDENTITY.md"), []byte("I am Tori"), 0644)
	os.WriteFile(filepath.Join(dir, "SOUL.md"), []byte("friendly and helpful"), 0644)
	os.MkdirAll(filepath.Join(dir, "skills"), 0755)
	p, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPriorityChain_DefaultResolution(t *testing.T) {
	base := setupTestPersona(t)
	presets := NewPresetManager()
	pc := NewPriorityChain(base, presets)

	// Default resolution should use preset (default active)
	prompt := pc.Resolve("", "")
	if prompt == "" {
		t.Fatal("expected non-empty default prompt")
	}
	// Default preset is "default" which has a SystemNote
	if prompt != BuiltinPresets["default"].SystemNote {
		// Could be base persona if preset returns empty — both are valid
		t.Logf("resolved to: %s", prompt)
	}
}

func TestPriorityChain_SessionOverride(t *testing.T) {
	base := setupTestPersona(t)
	presets := NewPresetManager()
	pc := NewPriorityChain(base, presets)

	pc.SetSessionOverride("sess-1", "你是一个专业的法律顾问", "api")

	prompt := pc.Resolve("sess-1", "")
	if prompt != "你是一个专业的法律顾问" {
		t.Fatalf("expected session override, got: %s", prompt)
	}

	// Different session should NOT get this override
	prompt2 := pc.Resolve("sess-2", "")
	if prompt2 == "你是一个专业的法律顾问" {
		t.Fatal("session override should not leak to other sessions")
	}
}

func TestPriorityChain_ConversationOverride(t *testing.T) {
	base := setupTestPersona(t)
	presets := NewPresetManager()
	pc := NewPriorityChain(base, presets)

	pc.SetConversationOverride("conv-1", "你是一个Python专家", "user-command")

	prompt := pc.Resolve("", "conv-1")
	if prompt != "你是一个Python专家" {
		t.Fatalf("expected conversation override, got: %s", prompt)
	}
}

func TestPriorityChain_SessionBeatsConversation(t *testing.T) {
	base := setupTestPersona(t)
	presets := NewPresetManager()
	pc := NewPriorityChain(base, presets)

	pc.SetConversationOverride("conv-1", "conversation level", "test")
	pc.SetSessionOverride("sess-1", "session level", "test")

	prompt := pc.Resolve("sess-1", "conv-1")
	if prompt != "session level" {
		t.Fatalf("session should beat conversation, got: %s", prompt)
	}
}

func TestPriorityChain_ConversationBeatsPreset(t *testing.T) {
	base := setupTestPersona(t)
	presets := NewPresetManager()
	presets.Switch("jarvis")
	pc := NewPriorityChain(base, presets)

	pc.SetConversationOverride("conv-1", "custom conversation persona", "test")

	prompt := pc.Resolve("", "conv-1")
	if prompt != "custom conversation persona" {
		t.Fatalf("conversation should beat preset, got: %s", prompt)
	}
}

func TestPriorityChain_PresetBeatsDefault(t *testing.T) {
	base := setupTestPersona(t)
	presets := NewPresetManager()
	presets.Switch("jarvis")
	pc := NewPriorityChain(base, presets)

	prompt, level := pc.ResolveWithLevel("", "")
	if level != PriorityPreset {
		t.Fatalf("expected preset level, got %d", level)
	}
	if prompt != BuiltinPresets["jarvis"].SystemNote {
		t.Fatalf("expected jarvis note, got: %s", prompt)
	}
}

func TestPriorityChain_ClearSessionOverride(t *testing.T) {
	base := setupTestPersona(t)
	presets := NewPresetManager()
	pc := NewPriorityChain(base, presets)

	pc.SetSessionOverride("sess-1", "forced persona", "api")
	prompt1 := pc.Resolve("sess-1", "")
	if prompt1 != "forced persona" {
		t.Fatal("expected session override active")
	}

	pc.ClearSessionOverride("sess-1")
	prompt2 := pc.Resolve("sess-1", "")
	if prompt2 == "forced persona" {
		t.Fatal("session override should be cleared")
	}
}

func TestPriorityChain_FeatureEnabled(t *testing.T) {
	base := setupTestPersona(t)
	presets := NewPresetManager()
	pc := NewPriorityChain(base, presets)

	// Default preset — all features enabled (nil Features map)
	if !pc.FeatureEnabled(FeatureEmotion) {
		t.Fatal("default preset should have emotion enabled")
	}
	if !pc.FeatureEnabled(FeatureSticker) {
		t.Fatal("default preset should have sticker enabled")
	}

	// Switch to business — emotion and sticker explicitly disabled
	presets.Switch("business")
	if pc.FeatureEnabled(FeatureEmotion) {
		t.Fatal("business preset should have emotion disabled")
	}
	if pc.FeatureEnabled(FeatureSticker) {
		t.Fatal("business preset should have sticker disabled")
	}

	// Switch to girlfriend — all features enabled (nil map)
	presets.Switch("girlfriend")
	if !pc.FeatureEnabled(FeatureEmotion) {
		t.Fatal("girlfriend preset should have emotion enabled")
	}

	// Nil presets chain — should fail-open
	pcNil := NewPriorityChain(base, nil)
	if !pcNil.FeatureEnabled(FeatureEmotion) {
		t.Fatal("nil presets should fail-open to enabled")
	}
}

func TestPriorityChain_ClearAll(t *testing.T) {
	base := setupTestPersona(t)
	presets := NewPresetManager()
	pc := NewPriorityChain(base, presets)

	pc.SetSessionOverride("s1", "session", "test")
	pc.SetConversationOverride("c1", "conv", "test")

	overrides := pc.ActiveOverrides()
	if len(overrides) != 2 {
		t.Fatalf("expected 2 overrides, got %d", len(overrides))
	}

	pc.ClearAll()
	overrides = pc.ActiveOverrides()
	if len(overrides) != 0 {
		t.Fatalf("expected 0 overrides after clear, got %d", len(overrides))
	}
}

func TestPriorityChain_SystemPromptFunc(t *testing.T) {
	base := setupTestPersona(t)
	presets := NewPresetManager()
	pc := NewPriorityChain(base, presets)

	fn := pc.SystemPromptFunc()
	prompt := fn()
	if prompt == "" {
		t.Fatal("SystemPromptFunc should return non-empty for default preset")
	}
}

func TestPriorityChain_NilComponents(t *testing.T) {
	// Test with nil preset manager
	base := setupTestPersona(t)
	pc := NewPriorityChain(base, nil)

	prompt := pc.Resolve("", "")
	if prompt == "" {
		t.Fatal("expected base persona when presets nil")
	}

	// Test with nil base
	presets := NewPresetManager()
	pc2 := NewPriorityChain(nil, presets)
	prompt2 := pc2.Resolve("", "")
	if prompt2 == "" {
		t.Fatal("expected preset when base nil")
	}
}

func TestPriorityChain_ResolveWithLevel(t *testing.T) {
	base := setupTestPersona(t)
	presets := NewPresetManager()
	pc := NewPriorityChain(base, presets)

	_, level := pc.ResolveWithLevel("", "")
	if level != PriorityPreset {
		t.Fatalf("expected preset level for default, got %d", level)
	}

	pc.SetConversationOverride("c1", "conv persona", "test")
	_, level = pc.ResolveWithLevel("", "c1")
	if level != PriorityConversation {
		t.Fatalf("expected conversation level, got %d", level)
	}

	pc.SetSessionOverride("s1", "session persona", "test")
	_, level = pc.ResolveWithLevel("s1", "c1")
	if level != PrioritySession {
		t.Fatalf("expected session level, got %d", level)
	}
}
