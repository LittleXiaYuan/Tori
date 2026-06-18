package gateway

import "net/http"

// HandleBrowserIntentSession keeps the browser extension OAuth/session grant
// semantics separate from the normal pack route auth middleware. The pack
// module still owns the path/method/enabled gate before this method is reached.
func (g *Gateway) HandleBrowserIntentSession(w http.ResponseWriter, r *http.Request) {
	g.requireBrowserSessionAuth(g.handleBrowserExtSession)(w, r)
}
