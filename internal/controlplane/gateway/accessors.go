package gateway

import (
	"yunque-agent/internal/agentcore/emotion"
	"yunque-agent/internal/agentcore/identity"
	reflectpkg "yunque-agent/internal/experimental/reflect"
)

// GetIdentityResolver returns the identity resolver.
func (g *Gateway) GetIdentityResolver() *identity.Resolver {
	return g.identityRes
}

// GetEmotionAnalyzer returns the emotion analyzer.
func (g *Gateway) GetEmotionAnalyzer() *emotion.Analyzer {
	return g.emotionAnalyzer
}

// GetStickerMap returns the sticker map.
func (g *Gateway) GetStickerMap() *emotion.StickerMap {
	return g.stickerMap
}

// GetEmotionHistory returns the emotion history.
func (g *Gateway) GetEmotionHistory() *emotion.History {
	return g.emotionHistory
}

// GetExperienceStore returns the experience store used for task reflection.
// Returns (store, ok) where ok indicates whether the store exists.
func (g *Gateway) GetExperienceStore() (*reflectpkg.ExperienceStore, bool) {
	return g.experienceStore, g.experienceStore != nil
}
