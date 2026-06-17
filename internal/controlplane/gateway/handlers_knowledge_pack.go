package gateway

import "net/http"

// HandleKnowledgePack is the exported bridge entrypoint for the knowledge pack
// (internal/packs/knowledge). The pack owns route registration + the pack
// enablement gate; the gateway still hosts the upload handler implementation
// during this bridge phase because upload shares the MinerU document parser with
// the admin upload path.
//
// Everything else on /v1/knowledge/* (search / sources / stats / ingest /
// source[/update] / import-url / import-repo) has been de-shelled and is served
// natively by the knowledge pack, so only upload is dispatched here.
func (g *Gateway) HandleKnowledgePack(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/v1/knowledge/upload":
		g.handleKBUpload(w, r)
	default:
		http.NotFound(w, r)
	}
}
