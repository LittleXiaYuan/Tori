package persona

import (
	"testing"
)

func TestNewPresetManager(t *testing.T) {
	pm := NewPresetManager()
	if len(pm.List()) != 8 {
		t.Fatalf("expected 8 presets, got %d", len(pm.List()))
	}
}

func TestActiveDefault(t *testing.T) {
	pm := NewPresetManager()
	if pm.ActiveID() != "default" {
		t.Fatalf("expected default, got %s", pm.ActiveID())
	}
	p := pm.Active()
	if p.Name != "Default" {
		t.Fatal("wrong name")
	}
}

func TestSwitch(t *testing.T) {
	pm := NewPresetManager()
	if err := pm.Switch("jarvis"); err != nil {
		t.Fatal(err)
	}
	if pm.ActiveID() != "jarvis" {
		t.Fatal("not switched")
	}
	p := pm.Active()
	if p.Name != "Jarvis" {
		t.Fatal("wrong name")
	}
}

func TestSwitchNotFound(t *testing.T) {
	pm := NewPresetManager()
	if err := pm.Switch("nonexistent"); err == nil {
		t.Fatal("expected error")
	}
}

func TestAddCustom(t *testing.T) {
	pm := NewPresetManager()
	pm.AddCustom(Preset{
		ID:          "pirate",
		Name:        "Pirate",
		Description: "Arr matey",
		SystemNote:  "Talk like a pirate.",
	})

	if len(pm.List()) != 9 {
		t.Fatalf("expected 9, got %d", len(pm.List()))
	}

	if err := pm.Switch("pirate"); err != nil {
		t.Fatal(err)
	}
	if pm.Active().Name != "Pirate" {
		t.Fatal("wrong custom")
	}
}

func TestGet(t *testing.T) {
	pm := NewPresetManager()
	p, ok := pm.Get("tech_expert")
	if !ok {
		t.Fatal("not found")
	}
	if p.Tone != "technical, concise" {
		t.Fatal("wrong tone")
	}
}

func TestGetNotFound(t *testing.T) {
	pm := NewPresetManager()
	_, ok := pm.Get("nope")
	if ok {
		t.Fatal("should not find")
	}
}

func TestPresetSystemPrompt(t *testing.T) {
	pm := NewPresetManager()
	pm.Switch("butler")
	prompt := pm.SystemPrompt()
	if prompt == "" {
		t.Fatal("empty prompt")
	}
	if !contains(prompt, "meticulous") {
		t.Fatal("wrong prompt content")
	}
}

func TestParseCommandSwitch(t *testing.T) {
	isCmd, id := ParseCommand("/persona jarvis")
	if !isCmd || id != "jarvis" {
		t.Fatalf("expected (true, jarvis), got (%v, %s)", isCmd, id)
	}
}

func TestParseCommandList(t *testing.T) {
	isCmd, id := ParseCommand("/persona")
	if !isCmd || id != "" {
		t.Fatalf("expected (true, ''), got (%v, %s)", isCmd, id)
	}
}

func TestParseCommandNotCommand(t *testing.T) {
	isCmd, _ := ParseCommand("hello world")
	if isCmd {
		t.Fatal("should not be command")
	}
}

func TestAllPresetsHaveFields(t *testing.T) {
	pm := NewPresetManager()
	for _, p := range pm.List() {
		if p.ID == "" || p.Name == "" || p.SystemNote == "" {
			t.Fatalf("preset %q missing fields", p.ID)
		}
	}
}

func TestHasFeature(t *testing.T) {
	// Preset with no features map → all features enabled
	p := &Preset{ID: "test", Features: nil}
	if !p.HasFeature(FeatureEmotion) {
		t.Fatal("nil features should default to true")
	}

	// Preset with explicit features
	p.Features = map[string]bool{FeatureEmotion: false, FeatureSticker: true}
	if p.HasFeature(FeatureEmotion) {
		t.Fatal("emotion should be disabled")
	}
	if !p.HasFeature(FeatureSticker) {
		t.Fatal("sticker should be enabled")
	}
	// Unknown feature → default true
	if !p.HasFeature("unknown_feature") {
		t.Fatal("unknown features should default to true")
	}
}

func TestBusinessPresetEmotionDisabled(t *testing.T) {
	pm := NewPresetManager()
	biz, ok := pm.Get("business")
	if !ok {
		t.Fatal("business preset not found")
	}
	if biz.HasFeature(FeatureEmotion) {
		t.Fatal("business preset should have emotion disabled")
	}
	tech, ok := pm.Get("tech_expert")
	if !ok {
		t.Fatal("tech_expert preset not found")
	}
	if tech.HasFeature(FeatureEmotion) {
		t.Fatal("tech_expert preset should have emotion disabled")
	}
}

func TestCompanionPresetEmotionEnabled(t *testing.T) {
	pm := NewPresetManager()
	for _, id := range []string{"girlfriend", "boyfriend", "family"} {
		p, ok := pm.Get(id)
		if !ok {
			t.Fatalf("preset %q not found", id)
		}
		if !p.HasFeature(FeatureEmotion) {
			t.Fatalf("preset %q should have emotion enabled", id)
		}
	}
}

func TestActiveHasFeature(t *testing.T) {
	pm := NewPresetManager()
	// Default preset → emotion enabled
	if !pm.ActiveHasFeature(FeatureEmotion) {
		t.Fatal("default preset should have emotion enabled")
	}
	// Switch to business → emotion disabled
	pm.Switch("business")
	if pm.ActiveHasFeature(FeatureEmotion) {
		t.Fatal("business preset should have emotion disabled")
	}
}

func TestSetFeatures(t *testing.T) {
	pm := NewPresetManager()
	// Enable emotion for business
	err := pm.SetFeatures("business", map[string]bool{FeatureEmotion: true})
	if err != nil {
		t.Fatal(err)
	}
	biz, _ := pm.Get("business")
	if !biz.HasFeature(FeatureEmotion) {
		t.Fatal("emotion should now be enabled for business")
	}
	// Non-existent preset
	if err := pm.SetFeatures("nonexistent", nil); err == nil {
		t.Fatal("expected error for non-existent preset")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// ── Phase 4 tests ───────────────────────────────────────────────

func TestFloatFeature(t *testing.T) {
	p := &Preset{ID: "t"}
	// No FeatureValues → returns default
	if v := p.FloatFeature(FeatureStickerFrequency, 2); v != 2 {
		t.Fatalf("expected 2, got %v", v)
	}
	p.FeatureValues = map[string]float64{FeatureStickerFrequency: 3}
	if v := p.FloatFeature(FeatureStickerFrequency, 2); v != 3 {
		t.Fatalf("expected 3, got %v", v)
	}
}

func TestReverieInterval(t *testing.T) {
	p := &Preset{ID: "t"}
	// No FeatureValues → default used
	if v := p.ReverieInterval(30 * 60 * 1e9); v != 30*60*1e9 { // nanoseconds
		t.Fatalf("expected default interval")
	}
	p.FeatureValues = map[string]float64{FeatureReverieIntervalMin: 20}
	if v := p.ReverieInterval(30 * 60 * 1e9); v != 20*60*1e9 {
		t.Fatalf("expected 20 minutes, got %v", v)
	}
}

func TestFeatureReverie(t *testing.T) {
	pm := NewPresetManager()
	// business and tech_expert should have reverie disabled
	for _, id := range []string{"business", "tech_expert"} {
		p, _ := pm.Get(id)
		if p.HasFeature(FeatureReverie) {
			t.Fatalf("preset %q should have reverie disabled", id)
		}
	}
	// companion presets should have reverie enabled (default true)
	for _, id := range []string{"girlfriend", "boyfriend", "family"} {
		p, _ := pm.Get(id)
		if !p.HasFeature(FeatureReverie) {
			t.Fatalf("preset %q should have reverie enabled", id)
		}
	}
}

func TestStickerFrequencyPerPersona(t *testing.T) {
	pm := NewPresetManager()
	cases := map[string]float64{
		"business":    0,
		"tech_expert": 0,
		"girlfriend":  3,
		"boyfriend":   3,
		"jarvis":      1,
		"butler":      1,
		"family":      2,
		"default":     2,
	}
	for id, wantFreq := range cases {
		p, ok := pm.Get(id)
		if !ok {
			t.Fatalf("preset %q not found", id)
		}
		got := p.FloatFeature(FeatureStickerFrequency, 2)
		if got != wantFreq {
			t.Errorf("preset %q: expected sticker_frequency=%.0f, got %.0f", id, wantFreq, got)
		}
	}
}

func TestEmotionStylePerPersona(t *testing.T) {
	pm := NewPresetManager()
	cases := map[string]EmotionStyle{
		"girlfriend": EmotionStyleEmpathic,
		"boyfriend":  EmotionStylePlayful,
		"family":     EmotionStyleWarm,
		"butler":     EmotionStyleWarm,
		"business":   EmotionStyleFormal,
		"default":    EmotionStyleFormal,
	}
	for id, want := range cases {
		p, ok := pm.Get(id)
		if !ok {
			t.Fatalf("preset %q not found", id)
		}
		if p.EmotionStyle != want {
			t.Errorf("preset %q: expected EmotionStyle=%q, got %q", id, want, p.EmotionStyle)
		}
	}
}

func TestOnSwitchCallback(t *testing.T) {
	pm := NewPresetManager()
	var called int
	var lastPreset *Preset
	pm.SetOnSwitch(func(p *Preset) {
		called++
		lastPreset = p
	})

	if err := pm.Switch("jarvis"); err != nil {
		t.Fatal(err)
	}
	if called != 1 {
		t.Fatalf("expected callback called 1 time, got %d", called)
	}
	if lastPreset.ID != "jarvis" {
		t.Fatalf("expected jarvis, got %s", lastPreset.ID)
	}

	// Switching again should fire again
	pm.Switch("girlfriend")
	if called != 2 {
		t.Fatalf("expected 2 calls, got %d", called)
	}
	if lastPreset.ID != "girlfriend" {
		t.Fatalf("expected girlfriend, got %s", lastPreset.ID)
	}
}

func TestOnSwitchNotFiredWithoutCallback(t *testing.T) {
	// Should not panic when no callback is set
	pm := NewPresetManager()
	if err := pm.Switch("jarvis"); err != nil {
		t.Fatal(err)
	}
}

func TestActiveFloatFeature(t *testing.T) {
	pm := NewPresetManager()
	pm.Switch("girlfriend")
	freq := pm.ActiveFloatFeature(FeatureStickerFrequency, 2)
	if freq != 3 {
		t.Fatalf("expected girlfriend sticker_frequency=3, got %v", freq)
	}
}

func TestActiveEmotionStyle(t *testing.T) {
	pm := NewPresetManager()
	pm.Switch("boyfriend")
	if pm.ActiveEmotionStyle() != EmotionStylePlayful {
		t.Fatalf("expected playful, got %q", pm.ActiveEmotionStyle())
	}
}
