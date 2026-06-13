package gateway

import "net/http"

// HandleKnowledgePack is the exported bridge entrypoint for the knowledge pack
// (internal/packs/knowledge). The pack owns route registration + the pack
// enablement gate; the gateway still hosts the handler implementations during
// this bridge phase. It dispatches /v1/knowledge/* to the existing handlers by
// path, preserving their original (handler-internal) method behavior.
func (g *Gateway) HandleKnowledgePack(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/v1/knowledge/search":
		g.handleKBSearch(w, r)
	case "/v1/knowledge/sources":
		g.handleKBSources(w, r)
	case "/v1/knowledge/stats":
		g.handleKBStats(w, r)
	case "/v1/knowledge/upload":
		g.handleKBUpload(w, r)
	case "/v1/knowledge/ingest":
		g.handleKBIngest(w, r)
	case "/v1/knowledge/import-url":
		g.handleKBImportURL(w, r)
	case "/v1/knowledge/import-repo":
		g.handleKBImportRepo(w, r)
	case "/v1/knowledge/source":
		g.handleKBDelete(w, r)
	case "/v1/knowledge/source/update":
		g.handleKBUpdate(w, r)
	default:
		http.NotFound(w, r)
	}
}
