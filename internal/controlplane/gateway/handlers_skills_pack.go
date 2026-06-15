package gateway

import "net/http"

// HandleSkillsPack is the exported bridge entrypoint for the skills pack
// (internal/packs/skills). The pack owns route registration + the enablement
// gate; the gateway still hosts the handler implementations during this bridge
// phase. It dispatches the core /v1/skills/* surface (list / scan / dynamic
// review) by path, preserving each handler's original method behavior. SkillHub
// (/api/skillhub/*) and the skill market (/v1/market/*) keep their own gateway
// routes for now (ownership TBD per the migration plan).
// Note: /v1/skills (listing) is no longer dispatched here — it was filled into
// the skills pack and is served natively there.
func (g *Gateway) HandleSkillsPack(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/v1/skills/scan":
		g.handleSkillsScan(w, r)
	// /v1/skills/dynamic, /approve, /reject are de-shelled — served natively by
	// the skills pack. Only scan remains on the gateway bridge for now.
	default:
		http.NotFound(w, r)
	}
}
