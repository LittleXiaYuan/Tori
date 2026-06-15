package gateway

import "net/http"

// HandleKnowledgePack is the exported bridge entrypoint for the knowledge pack
// (internal/packs/knowledge). The pack owns route registration + the pack
// enablement gate; the gateway still hosts the handler implementations during
// this bridge phase. It dispatches /v1/knowledge/* to the existing handlers by
// path, preserving their original (handler-internal) method behavior.
// Note: the read routes (search / sources / stats) are no longer dispatched
// here — they were filled into the knowledge pack and are served natively there.
func (g *Gateway) HandleKnowledgePack(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/v1/knowledge/upload":
		g.handleKBUpload(w, r)
	// /v1/knowledge/ingest is de-shelled — served natively by the knowledge pack.
	case "/v1/knowledge/import-url":
		g.handleKBImportURL(w, r)
	case "/v1/knowledge/import-repo":
		g.handleKBImportRepo(w, r)
	// /v1/knowledge/source (delete) and /source/update are de-shelled — served
	// natively by the knowledge pack.
	default:
		http.NotFound(w, r)
	}
}
