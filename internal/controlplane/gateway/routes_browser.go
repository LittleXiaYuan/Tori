package gateway

// registerBrowserRoutes registers browser engine management API endpoints.
func (g *Gateway) registerBrowserRoutes() {
	// Engine control
	g.mux.HandleFunc("/v1/browser/status", g.requireAuth(g.handleBrowserStatus))
	g.mux.HandleFunc("/v1/browser/config", g.requireAuth(g.handleBrowserConfig))

	// Screenshot stream
	g.mux.HandleFunc("/v1/browser/screenshot/latest", g.requireAuth(g.handleBrowserScreenshotLatest))

	// OPP — human-in-the-loop for browser tasks
	g.mux.HandleFunc("/v1/browser/opp/pending", g.requireAuth(g.handleOPPPending))
	g.mux.HandleFunc("/v1/browser/opp/decide", g.requireAuth(g.handleOPPDecide))
}
