package gateway

import (
	"yunque-agent/internal/agentcore/modes"
	"yunque-agent/internal/agentcore/planner"
)

// Emotion HTTP handlers (/v1/emotion/history, /v1/emotion/stickers) were
// de-shelled into the emotion pack (internal/packs/emotion). The gateway only
// exposes the EmotionHistory() / StickerMap() accessors (see gateway_setters.go).

//  from handlers_modes.go — the persona-mode HTTP handlers were de-shelled into
// the modes pack (internal/packs/modes). Only the manager accessor remains.

// ModeManager exposes the persona mode manager to backend packs (the modes pack).
func (g *Gateway) ModeManager() *modes.ModeManager { return g.modeManager }

//  from handlers_reverie.go 
// 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€
// Reverie API 鈥?visualization and operations endpoints
//
// GET  /v1/reverie/journal   鈥?list thoughts (filter, paginate)
// GET  /v1/reverie/stats     鈥?summary statistics
// GET  /v1/reverie/config    鈥?current configuration
// PUT  /v1/reverie/config    鈥?update configuration
// POST /v1/reverie/think     鈥?manually trigger a think cycle
// DELETE /v1/reverie/thought 鈥?delete a specific thought by ID
// 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€

// Reverie HTTP handlers were de-shelled into the reverie pack
// (internal/packs/reverie). The gateway only exposes the accessors below; the
// admin-only /v1/cognitive-layer and /v1/reverie/dream/status stay on the gateway.

// Reverie exposes the inner-monologue engine to backend packs (the reverie pack).
func (g *Gateway) Reverie() *planner.Reverie { return g.reverie }

// ReverieChannelTypes lists registered channel types for the reverie targets view.
func (g *Gateway) ReverieChannelTypes() []string {
	if g.channelReg == nil {
		return nil
	}
	var out []string
	for _, ch := range g.channelReg.All() {
		out = append(out, ch.Type())
	}
	return out
}


