package gateway

// registerMemoryRoutes registers memory, graph, identity, embeddings, and search routes.
func (g *Gateway) registerMemoryRoutes() {
	// Memory
	g.mux.HandleFunc("/v1/memory/stats", g.requireAuth(g.handleMemoryStats))
	g.mux.HandleFunc("/v1/memory/search", g.requireAuth(g.handleMemorySearch))
	g.mux.HandleFunc("/v1/memory/add", g.requireAuth(g.handleMemoryAdd))
	g.mux.HandleFunc("/v1/memory/compact", g.requireAuth(g.handleMemoryCompact))

	// Knowledge Graph
	g.mux.HandleFunc("/v1/graph/entities", g.requireAuth(g.handleGraphEntities))
	g.mux.HandleFunc("/v1/graph/relations", g.requireAuth(g.handleGraphRelations))
	g.mux.HandleFunc("/v1/graph/context", g.requireAuth(g.handleGraphContext))
	g.mux.HandleFunc("/v1/graph/stats", g.requireAuth(g.handleGraphStats))

	// Identity
	g.mux.HandleFunc("/v1/identity/resolve", g.requireAuth(g.handleIdentityResolve))
	g.mux.HandleFunc("/v1/identity/profiles", g.requireAuth(g.handleIdentityProfiles))

	// Embeddings
	g.mux.HandleFunc("/v1/embeddings", g.requireAuth(g.handleEmbeddings))

	// Search
	g.mux.HandleFunc("/v1/search", g.requireAuth(g.handleSearch))
	g.mux.HandleFunc("/v1/search/providers", g.requireAuth(g.handleSearchProviders))
}
