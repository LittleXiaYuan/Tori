package gateway

import (
	"yunque-agent/internal/agentcore/knowledge"
)

// KnowledgeStore exposes the knowledge store to backend packs (e.g. the
// knowledge pack's native handlers). May be nil if the KB is not configured.
func (g *Gateway) KnowledgeStore() *knowledge.Store { return g.knowledgeStore }

// The knowledge read surface (search / sources / stats) and the write surface
// (ingest / source delete / source update) were de-shelled into the knowledge
// pack (internal/packs/knowledge); their handler logic now lives there and talks
// to the store directly. Only upload / import-url / import-repo remain on the
// gateway bridge (HandleKnowledgePack) for now.
