package gateway

import "net/http"

// HandleBrowserIntentPack is retained as a compatibility no-op entrypoint.
// Browser Intent routes now live in internal/packs/browserintent; only the
// extension session grant keeps its dedicated gateway bridge below.
func (g *Gateway) HandleBrowserIntentPack(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
}

// HandleBrowserIntentSession keeps the browser extension OAuth/session grant
// semantics separate from the normal pack route auth middleware. The pack
// module still owns the path/method/enabled gate before this method is reached.
func (g *Gateway) HandleBrowserIntentSession(w http.ResponseWriter, r *http.Request) {
	g.requireBrowserSessionAuth(g.handleBrowserExtSession)(w, r)
}
