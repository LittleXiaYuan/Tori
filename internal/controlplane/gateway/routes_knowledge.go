package gateway

// registerKnowledgeRoutes registers knowledge base (RAG) routes.
func (g *Gateway) registerKnowledgeRoutes() {
	g.mux.HandleFunc("/v1/knowledge/search", g.requireAuth(g.handleKBSearch))
	g.mux.HandleFunc("/v1/knowledge/sources", g.requireAuth(g.handleKBSources))
	g.mux.HandleFunc("/v1/knowledge/stats", g.requireAuth(g.handleKBStats))
	g.mux.HandleFunc("/v1/knowledge/upload", g.requireAuth(g.handleKBUpload))
	g.mux.HandleFunc("/v1/knowledge/ingest", g.requireAuth(g.handleKBIngest))
	g.mux.HandleFunc("/v1/knowledge/import-url", g.requireAuth(g.handleKBImportURL))
	g.mux.HandleFunc("/v1/knowledge/import-repo", g.requireAuth(g.handleKBImportRepo))
	g.mux.HandleFunc("/v1/knowledge/source", g.requireAuth(g.handleKBDelete))
}
