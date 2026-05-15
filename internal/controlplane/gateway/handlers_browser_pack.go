package gateway

import "net/http"

// HandleBrowserIntentPack is the bridge endpoint used by the Browser Intent
// capability pack. It preserves the existing Gateway implementation while
// moving route ownership and enablement gates into Pack Runtime.
func (g *Gateway) HandleBrowserIntentPack(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/v1/browser/status":
		g.handleBrowserStatus(w, r)
	case "/v1/browser/config":
		g.handleBrowserConfig(w, r)
	case "/v1/browser/navigate":
		g.handleBrowserNavigate(w, r)
	case "/v1/browser/screenshot":
		g.handleBrowserScreenshot(w, r)
	case "/v1/browser/ocr":
		g.handleBrowserOCR(w, r)
	case "/v1/browser/screenshot/latest":
		g.handleBrowserScreenshotLatest(w, r)
	case "/v1/browser/opp/pending":
		g.handleOPPPending(w, r)
	case "/v1/browser/opp/decide":
		g.handleOPPDecide(w, r)
	case "/api/browser/ext/status":
		g.handleBrowserExtStatus(w, r)
	case "/api/browser/ext/action":
		g.handleBrowserExtAction(w, r)
	case "/api/browser/ext/scenarios":
		g.handleBrowserScenarios(w, r)
	case "/api/browser/ext/scenarios/run":
		g.handleBrowserRunScenario(w, r)
	default:
		http.NotFound(w, r)
	}
}

// HandleBrowserIntentSession keeps the browser extension OAuth/session grant
// semantics separate from the normal pack route auth middleware. The pack
// module still owns the path/method/enabled gate before this method is reached.
func (g *Gateway) HandleBrowserIntentSession(w http.ResponseWriter, r *http.Request) {
	g.requireBrowserSessionAuth(g.handleBrowserExtSession)(w, r)
}
