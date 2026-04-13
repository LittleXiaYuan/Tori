package adaptive

import (
	"context"
	"testing"

	"yunque-agent/internal/experimental/trait"
)

func TestTraitBridgeSyncToProfile(t *testing.T) {
	loop := NewLoop()
	store := trait.NewStore(t.TempDir())
	store.Add(trait.DimCommunicationStyle, "concise", 0.9, "user said be brief")
	store.Add(trait.DimTonePreference, "casual", 0.8, "user prefers casual")
	store.Add(trait.DimLanguagePreference, "zh", 0.6, "user speaks chinese")

	bridge := NewTraitBridge(loop, store)
	synced := bridge.SyncTraitsToProfile(10)
	if synced != 3 {
		t.Fatalf("expected 3 synced, got %d", synced)
	}

	val, ok := loop.GetSetting(DimResponseLength)
	if !ok || val != "concise" {
		t.Fatalf("expected concise, got %q", val)
	}
	val, ok = loop.GetSetting(DimFormality)
	if !ok || val != "casual" {
		t.Fatalf("expected casual, got %q", val)
	}
	val, ok = loop.GetSetting(DimLanguage)
	if !ok || val != "zh" {
		t.Fatalf("expected zh, got %q", val)
	}
}

func TestTraitBridgeLowConfidenceSkipped(t *testing.T) {
	loop := NewLoop()
	store := trait.NewStore(t.TempDir())
	store.Add(trait.DimCommunicationStyle, "verbose", 0.3, "weak signal")

	bridge := NewTraitBridge(loop, store)
	synced := bridge.SyncTraitsToProfile(10)
	if synced != 0 {
		t.Fatalf("expected 0 synced (low confidence), got %d", synced)
	}
}

func TestTraitBridgeOnTraitMined(t *testing.T) {
	loop := NewLoop()
	store := trait.NewStore(t.TempDir())
	bridge := NewTraitBridge(loop, store)

	bridge.OnTraitMined(context.Background(), trait.Trait{
		Dimension:  trait.DimTonePreference,
		Preference: "formal",
		Source:     "user said be more formal",
	})

	fbs := loop.Feedbacks(10)
	if len(fbs) != 1 {
		t.Fatalf("expected 1 feedback, got %d", len(fbs))
	}
	if fbs[0].Type != FeedbackPreference {
		t.Fatalf("expected preference type, got %s", fbs[0].Type)
	}
	if fbs[0].Dimension != DimFormality {
		t.Fatalf("expected formality, got %s", fbs[0].Dimension)
	}
}

func TestTraitBridgeUnmappedDimension(t *testing.T) {
	loop := NewLoop()
	store := trait.NewStore(t.TempDir())
	bridge := NewTraitBridge(loop, store)

	// WorkSchedule has no adaptive mapping
	bridge.OnTraitMined(context.Background(), trait.Trait{
		Dimension:  trait.DimWorkSchedule,
		Preference: "night owl",
	})
	if len(loop.Feedbacks(10)) != 0 {
		t.Fatal("unmapped dimension should not generate feedback")
	}
}

func TestTraitDimMapping(t *testing.T) {
	tests := []struct {
		traitDim string
		want     string
	}{
		{trait.DimCommunicationStyle, DimResponseLength},
		{trait.DimTonePreference, DimFormality},
		{trait.DimLanguagePreference, DimLanguage},
		{trait.DimExpertiseLevel, DimTechnicalLevel},
		{trait.DimWorkSchedule, ""},
		{"unknown_dim", ""},
	}
	for _, tt := range tests {
		got := traitDimToAdaptiveDim(tt.traitDim)
		if got != tt.want {
			t.Errorf("traitDimToAdaptiveDim(%q) = %q, want %q", tt.traitDim, got, tt.want)
		}
	}
}
