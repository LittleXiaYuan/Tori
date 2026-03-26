package gateway

// registerWorkflowRoutes registers workflow API routes.
func (g *Gateway) registerWorkflowRoutes() {
	g.mux.HandleFunc("/v1/workflows", g.requireAuth(g.handleWorkflowRouteSwitch))
	g.mux.HandleFunc("/v1/workflows/run", g.requireAuth(g.handleWorkflowRun))
	g.mux.HandleFunc("/v1/workflows/instances", g.requireAuth(g.handleWorkflowInstances))
	g.mux.HandleFunc("/v1/workflows/cancel", g.requireAuth(g.handleWorkflowCancel))
}
