package gateway

// registerMCPDispatchRoutes mounts the MCP dispatch server endpoint
// and worker management API.
func (g *Gateway) registerMCPDispatchRoutes() {
	// MCP dispatch routes (/mcp/v1, /v1/workers*, /v1/dispatch*) are owned by
	// the MCP dispatch pack (internal/packs/mcpdispatch), mounted via
	// gw.RegisterModule. The pack preserves the original method-sensitive auth:
	// GET/HEAD/OPTIONS probes are open, JSON-RPC POST is authenticated.
}
