package gateway

import "net/http"

// HandleMemoryPack is the exported bridge entrypoint for the memory pack
// (internal/packs/memory). The pack owns route registration + the enablement
// gate; the gateway still hosts the handler implementations during this bridge
// phase. It dispatches /v1/memory/* to the existing handlers by path. Knowledge
// graph, identity, embeddings and generic search keep their own gateway routes.
// Note: stats/search are no longer dispatched here — they were filled into the
// memory pack and are served natively there.
func (g *Gateway) HandleMemoryPack(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/v1/memory/recall/debug":
		g.handleMemoryRecallDebug(w, r)
	// /v1/memory/add and /v1/memory/compact are de-shelled — served natively by
	// the memory pack.
	case "/v1/memory/persona":
		g.handleMemoryPersonaGet(w, r)
	case "/v1/memory/update":
		g.handleMemoryPersonaUpdate(w, r)
	default:
		http.NotFound(w, r)
	}
}
