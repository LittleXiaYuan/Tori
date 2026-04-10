package modes

import (
	"context"
	"testing"
)

func TestPersonaModeValid(t *testing.T) {
	cases := []struct {
		mode PersonaMode
		want bool
	}{
		{ModeSpirit, true},
		{ModeCompanion, true},
		{ModeScholar, true},
		{"unknown", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := tc.mode.Valid(); got != tc.want {
			t.Errorf("PersonaMode(%q).Valid() = %v, want %v", tc.mode, got, tc.want)
		}
	}
}

func TestJudgmentIsSignificant(t *testing.T) {
	cases := []struct {
		name      string
		j         *Judgment
		threshold float64
		want      bool
	}{
		{"nil", nil, 0.5, false},
		{"neutral", &Judgment{Valence: 0, Strength: 0.8}, 0.5, false},
		{"weak", &Judgment{Valence: 1, Strength: 0.3}, 0.5, false},
		{"strong", &Judgment{Valence: -1, Strength: 0.7}, 0.5, true},
		{"at threshold", &Judgment{Valence: 1, Strength: 0.5}, 0.5, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.j.IsSignificant(tc.threshold); got != tc.want {
				t.Errorf("IsSignificant(%v) = %v, want %v", tc.threshold, got, tc.want)
			}
		})
	}
}

func TestScholarMode(t *testing.T) {
	s := NewScholarMode()

	if s.Mode() != ModeScholar {
		t.Fatalf("Mode() = %v, want %v", s.Mode(), ModeScholar)
	}

	j, err := s.Judge(context.Background(), "test input", nil)
	if err != nil {
		t.Fatal(err)
	}
	if j.Valence != 0 || j.Strength != 0 {
		t.Errorf("Scholar Judge should be zero; got valence=%d strength=%f", j.Valence, j.Strength)
	}

	st, err := s.Stance(context.Background(), j)
	if err != nil {
		t.Fatal(err)
	}
	if st.Position != PositionNeutral {
		t.Errorf("Scholar Stance = %v, want neutral", st.Position)
	}

	// Scholar should have low temperature
	sampling := s.Sampling()
	if sampling.Temperature > 0.5 {
		t.Errorf("Scholar temperature %.2f too high", sampling.Temperature)
	}

	// Scholar should not store stances or emotions
	mem := s.MemoryPolicy()
	if mem.StoreStance || mem.StoreEmotion || mem.StoreRelation {
		t.Error("Scholar should not store stances, emotions, or relations")
	}

	// Scholar should not allow disagreement
	guardrails := s.GuardrailOverrides()
	if guardrails.AllowDisagreement {
		t.Error("Scholar should not allow disagreement")
	}
}

func TestModePresetsExist(t *testing.T) {
	for _, mode := range AllModes {
		preset, ok := ModePresets[mode]
		if !ok {
			t.Errorf("missing preset for mode %v", mode)
			continue
		}
		if preset.Name == "" {
			t.Errorf("mode %v preset has empty Name", mode)
		}
		if preset.NameEN == "" {
			t.Errorf("mode %v preset has empty NameEN", mode)
		}
	}
}

func TestModeManagerDefault(t *testing.T) {
	mm := NewModeManager(nil, nil, "zh")
	ctx := context.Background()

	mode := mm.CurrentMode(ctx, "tenant1", "")
	if mode != ModeCompanion {
		t.Errorf("default CurrentMode = %v, want companion", mode)
	}

	mm.SetDefaultMode(ModeScholar)
	mode = mm.CurrentMode(ctx, "tenant2", "")
	if mode != ModeScholar {
		t.Errorf("after SetDefaultMode, CurrentMode = %v, want scholar", mode)
	}

	// Invalid mode ignored
	mm.SetDefaultMode("invalid")
	mode = mm.CurrentMode(ctx, "tenant3", "")
	if mode != ModeScholar {
		t.Errorf("invalid SetDefaultMode should be ignored, got %v", mode)
	}
}

func TestModeManagerSetMode(t *testing.T) {
	mm := NewModeManager(nil, nil, "zh")
	ctx := context.Background()

	if err := mm.SetMode(ctx, "t1", ModeSpirit, ""); err != nil {
		t.Fatal(err)
	}
	if got := mm.CurrentMode(ctx, "t1", ""); got != ModeSpirit {
		t.Errorf("CurrentMode = %v, want spirit", got)
	}

	// Session override takes priority
	if err := mm.SetMode(ctx, "t1", ModeScholar, "s1"); err != nil {
		t.Fatal(err)
	}
	if got := mm.CurrentMode(ctx, "t1", "s1"); got != ModeScholar {
		t.Errorf("session override = %v, want scholar", got)
	}
	// Tenant mode unchanged
	if got := mm.CurrentMode(ctx, "t1", ""); got != ModeSpirit {
		t.Errorf("tenant mode after session override = %v, want spirit", got)
	}

	// Clear session override
	mm.ClearSessionOverride("s1")
	if got := mm.CurrentMode(ctx, "t1", "s1"); got != ModeSpirit {
		t.Errorf("after clear override = %v, want spirit", got)
	}
}

func TestModeManagerInvalidMode(t *testing.T) {
	mm := NewModeManager(nil, nil, "zh")
	err := mm.SetMode(context.Background(), "t1", "bad", "")
	if err == nil {
		t.Error("expected error for invalid mode")
	}
}

func TestModeManagerListModes(t *testing.T) {
	mm := NewModeManager(nil, nil, "zh")
	ctx := context.Background()

	modes := mm.ListModes(ctx, "t1", "")
	if len(modes) != len(AllModes) {
		t.Fatalf("ListModes returned %d, want %d", len(modes), len(AllModes))
	}

	// Default should mark companion as active
	activeCount := 0
	for _, m := range modes {
		if m.Active {
			activeCount++
			if m.Mode != ModeCompanion {
				t.Errorf("active mode = %v, want companion", m.Mode)
			}
		}
	}
	if activeCount != 1 {
		t.Errorf("activeCount = %d, want 1", activeCount)
	}
}

func TestModeManagerEngine(t *testing.T) {
	mm := NewModeManager(nil, nil, "zh")
	ctx := context.Background()

	// Default engine should be companion
	engine := mm.Engine(ctx, "t1", "")
	if engine.Mode() != ModeCompanion {
		t.Errorf("default engine mode = %v, want companion", engine.Mode())
	}

	// After switching, engine changes
	mm.SetMode(ctx, "t1", ModeScholar, "")
	engine = mm.Engine(ctx, "t1", "")
	if engine.Mode() != ModeScholar {
		t.Errorf("switched engine mode = %v, want scholar", engine.Mode())
	}
}
