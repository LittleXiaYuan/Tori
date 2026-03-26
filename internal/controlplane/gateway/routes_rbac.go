package gateway

// registerRBACRoutes registers role-based access control API routes.
func (g *Gateway) registerRBACRoutes() {
	g.mux.HandleFunc("/v1/rbac/roles", g.requireAuth(g.handleRBACRolesSwitch))
	g.mux.HandleFunc("/v1/rbac/assign", g.requireAuth(g.handleRBACAssign))
	g.mux.HandleFunc("/v1/rbac/revoke", g.requireAuth(g.handleRBACRevoke))
	g.mux.HandleFunc("/v1/rbac/check", g.requireAuth(g.handleRBACCheck))
	g.mux.HandleFunc("/v1/rbac/my-roles", g.requireAuth(g.handleRBACMyRoles))
}
