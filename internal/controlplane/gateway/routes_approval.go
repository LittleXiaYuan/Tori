package gateway

// registerApprovalRoutes registers human-in-the-loop approval API routes.
func (g *Gateway) registerApprovalRoutes() {
	g.mux.HandleFunc("/v1/approvals", g.requireAuth(g.handleApprovalRouteSwitch))
	g.mux.HandleFunc("/v1/approvals/approve", g.requireAuth(g.handleApprovalApprove))
	g.mux.HandleFunc("/v1/approvals/deny", g.requireAuth(g.handleApprovalDeny))
}

// registerSSERoutes registers the SSE event stream.
func (g *Gateway) registerSSERoutes() {
	g.mux.HandleFunc("/v1/events/stream", g.requireAuth(g.handleSSEStream))
}

// registerTraceRoutes registers execution trace / audit API.
func (g *Gateway) registerTraceRoutes() {
	g.mux.HandleFunc("/v1/trace/recent", g.requireAuth(g.handleTraceRecent))
	g.mux.HandleFunc("/v1/trace/task/", g.requireAuth(g.handleTraceByTask))
	g.mux.HandleFunc("/v1/trace/", g.requireAuth(g.handleTraceByID))
}
