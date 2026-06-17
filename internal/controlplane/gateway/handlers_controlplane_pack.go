package gateway

import "net/http"

// HandleControlPlanePack is retained as a compatibility no-op entrypoint for
// older tests and modules. The control-plane pack now owns its migrated route
// handlers directly; unexpected legacy dispatches should fall through to 404.
func (g *Gateway) HandleControlPlanePack(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
}
