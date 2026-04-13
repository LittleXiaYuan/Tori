package adaptive

import (
	"context"
	"log/slog"

	"yunque-agent/internal/experimental/trait"
)

// TraitBridge connects the trait mining system with the adaptive loop.
// When traits are mined, they become feedback signals for behavior adaptation.
type TraitBridge struct {
	loop  *Loop
	store *trait.Store
}

// NewTraitBridge creates a bridge between trait mining and adaptive behavior.
func NewTraitBridge(loop *Loop, store *trait.Store) *TraitBridge {
	return &TraitBridge{loop: loop, store: store}
}

// SyncTraitsToProfile reads current high-confidence traits and sets them
// as behavior profile settings.
func (tb *TraitBridge) SyncTraitsToProfile(maxTraits int) int {
	if maxTraits <= 0 {
		maxTraits = 10
	}
	top := tb.store.TopTraits(maxTraits)
	synced := 0
	for _, t := range top {
		if t.Confidence < 0.5 {
			continue
		}
		dim := traitDimToAdaptiveDim(t.Dimension)
		if dim == "" {
			continue
		}
		tb.loop.SetSetting(dim, t.Preference)
		synced++
	}
	if synced > 0 {
		slog.Info("trait-bridge: synced traits to profile", "count", synced)
	}
	return synced
}

// OnTraitMined is a callback to be called after a trait is mined.
// It records a preference feedback in the adaptive loop.
func (tb *TraitBridge) OnTraitMined(_ context.Context, t trait.Trait) {
	dim := traitDimToAdaptiveDim(t.Dimension)
	if dim == "" {
		return
	}
	tb.loop.RecordFeedback(Feedback{
		Type:       FeedbackPreference,
		Dimension:  dim,
		Correction: t.Preference,
		UserMessage: t.Source,
	})
}

// traitDimToAdaptiveDim maps trait dimensions to adaptive dimensions.
func traitDimToAdaptiveDim(traitDim string) string {
	mapping := map[string]string{
		trait.DimCommunicationStyle: DimResponseLength,
		trait.DimTonePreference:     DimFormality,
		trait.DimLanguagePreference: DimLanguage,
		trait.DimExpertiseLevel:     DimTechnicalLevel,
		trait.DimDomainPreference:   DimCodeStyle,
		trait.DimContentInterest:    DimExplanationDepth,
	}
	if v, ok := mapping[traitDim]; ok {
		return v
	}
	return ""
}
