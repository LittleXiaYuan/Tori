package gateway

import (
	"yunque-agent/internal/agentcore/knowledge"
)

// KnowledgeStore exposes the knowledge store to backend packs (e.g. the
// knowledge pack's native handlers). May be nil if the KB is not configured.
func (g *Gateway) KnowledgeStore() *knowledge.Store { return g.knowledgeStore }

// FetchImportPage performs an SSRF-safe fetch + content extraction for a single
// URL. It is the narrow capability the knowledge pack's native import-url
// handler consumes (instead of the concrete *Gateway): the SSRF guard stays
// here because it is shared with other outbound-fetch features.
func (g *Gateway) FetchImportPage(rawURL, fallbackName string) (*knowledge.ImportPage, error) {
	return fetchKnowledgeURLPage(rawURL, fallbackName)
}

// DocumentParser exposes the configured document parser to the knowledge pack's
// native upload handler.
func (g *Gateway) DocumentParser() knowledge.DocumentParser { return g.documentParser }

// The knowledge read surface (search / sources / stats), the write surface
// (ingest / source delete / source update), the import surface (import-url /
// import-repo) and upload were de-shelled into the knowledge pack
// (internal/packs/knowledge); their handler logic now lives there and talks to
// the store directly.
