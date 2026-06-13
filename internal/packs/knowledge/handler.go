// Package knowledgepack mounts the knowledge-base (RAG) HTTP surface as a Pack
// Runtime backend module. This is the "bridge" migration phase: the pack owns
// route registration and the enable/disable gate, while the gateway still hosts
// the handler implementations (consumed via the narrow KnowledgeGateway
// interface). Disabling the pack removes the /v1/knowledge/* surface without
// touching the knowledge store; the implementation moves into this package in a
// later "fill the flesh" step.
package knowledgepack

import (
	"net/http"

	"yunque-agent/pkg/packruntime"
)

const PackID = "yunque.pack.knowledge"

// KnowledgeGateway is the narrow gateway surface the knowledge pack needs.
type KnowledgeGateway interface {
	HandleKnowledgePack(w http.ResponseWriter, r *http.Request)
}

// Handler is the knowledge pack's backend module.
type Handler struct {
	gateway KnowledgeGateway
}

// NewHandler builds the knowledge pack backed by the gateway bridge.
func NewHandler(gateway KnowledgeGateway) *Handler { return &Handler{gateway: gateway} }

// PackID returns the stable manifest id.
func (h *Handler) PackID() string { return PackID }

// Routes mounts the knowledge surface. Methods are declared broadly so the
// bridge preserves the original routes' permissive (handler-decides) method
// behavior; the manifest lists these as path-only routes so the pack gate
// allows any method. Tightening to exact methods is a fill-the-flesh follow-up.
func (h *Handler) Routes() []packruntime.BackendRoute {
	d := h.gateway.HandleKnowledgePack
	methods := []string{
		http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch,
	}
	paths := []string{
		"/v1/knowledge/search",
		"/v1/knowledge/sources",
		"/v1/knowledge/stats",
		"/v1/knowledge/upload",
		"/v1/knowledge/ingest",
		"/v1/knowledge/import-url",
		"/v1/knowledge/import-repo",
		"/v1/knowledge/source",
		"/v1/knowledge/source/update",
	}
	routes := make([]packruntime.BackendRoute, 0, len(paths))
	for _, p := range paths {
		routes = append(routes, packruntime.BackendRoute{Methods: methods, Path: p, Handler: d})
	}
	return routes
}
