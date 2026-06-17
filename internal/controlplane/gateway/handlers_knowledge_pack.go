package gateway

import "net/http"

// HandleKnowledgePack is retained as a dead-simple compatibility bridge for
// older tests or out-of-tree code paths that still construct a knowledge pack
// against a Gateway. The /v1/knowledge/* surface is now served natively by
// internal/packs/knowledge, so no gateway-owned knowledge route should dispatch
// here.
func (g *Gateway) HandleKnowledgePack(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
}
