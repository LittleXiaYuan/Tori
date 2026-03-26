package gateway

// registerReverieRoutes registers Reverie (inner monologue) visualization and operation routes.
func (g *Gateway) registerReverieRoutes() {
	g.mux.HandleFunc("/v1/reverie/journal", g.requireAuth(g.handleReverieJournal))
	g.mux.HandleFunc("/v1/reverie/stats", g.requireAuth(g.handleReverieStats))
	g.mux.HandleFunc("/v1/reverie/config", g.requireAuth(g.handleReverieConfig))
	g.mux.HandleFunc("/v1/reverie/think", g.requireAuth(g.handleReverieThink))
	g.mux.HandleFunc("/v1/reverie/thought", g.requireAuth(g.handleReverieDeleteThought))
	g.mux.HandleFunc("/v1/reverie/targets", g.requireAuth(g.handleReverieTargets))
	g.mux.HandleFunc("/v1/reverie/actions", g.requireAuth(g.handleReverieActions))
}
