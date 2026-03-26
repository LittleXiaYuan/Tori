package gateway

// registerProviderRoutes registers LLM provider management and model routing routes.
func (g *Gateway) registerProviderRoutes() {
	g.mux.HandleFunc("/api/providers", g.requireAuth(g.handleProviderList))
	g.mux.HandleFunc("/api/providers/test", g.requireAuth(g.handleProviderTest))
	g.mux.HandleFunc("/api/providers/enable", g.requireAuth(g.handleProviderEnable))
	g.mux.HandleFunc("/api/providers/disable", g.requireAuth(g.handleProviderDisable))
	g.mux.HandleFunc("/api/providers/switch-model", g.requireAuth(g.handleProviderSwitchModel))
	g.mux.HandleFunc("/api/providers/session", g.requireAuth(g.handleProviderSessionOverride))
	g.mux.HandleFunc("/api/providers/local/discover", g.requireAuth(g.handleLocalDiscover))
	g.mux.HandleFunc("/api/providers/local/register", g.requireAuth(g.handleLocalRegister))

	// Smart router stats
	g.mux.HandleFunc("/v1/router/stats", g.requireAuth(g.handleRouterStats))
}
