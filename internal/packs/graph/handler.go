// Package graphpack mounts the knowledge-graph surface (/v1/graph/entities,
// /v1/graph/relations, /v1/graph/context, /v1/graph/stats) as a v2 capability
// pack (Tier 0 microkernel). It is a native pack: the entity/relation CRUD and
// context/stats logic live here and talk to the memory pipeline's graph through
// a narrow host accessor — the gateway no longer hosts these routes.
package graphpack

import (
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"

	"yunque-agent/internal/agentcore/memory"
	"yunque-agent/internal/apperror"
	"yunque-agent/pkg/packruntime"
)

// PackID is the stable manifest id.
const PackID = "yunque.pack.graph"

// Gateway is the narrow host surface the graph pack needs: a handle to the
// memory pipeline (whose Graph() carries the knowledge graph), resolved per
// request so registration order does not matter.
type Gateway interface {
	MemoryPipeline() *memory.Pipeline
}

// Handler is the graph pack backend module.
type Handler struct {
	gw      Gateway
	host    packruntime.Host
	started atomic.Bool
}

// New builds the graph pack backed by the host's memory pipeline accessor.
func New(gw Gateway) *Handler { return &Handler{gw: gw} }

// compile-time assertion: this is a valid v2 Module.
var _ packruntime.Module = (*Handler)(nil)

func (h *Handler) PackID() string { return PackID }

// Init wires the pack against the kernel Host.
func (h *Handler) Init(host packruntime.Host) error {
	h.host = host
	return nil
}

// Start marks the pack live on enable.
func (h *Handler) Start(ctx context.Context) error {
	h.started.Store(true)
	if h.host != nil {
		h.host.Logger().Info("graph pack started", "pack", PackID)
	}
	return nil
}

// Stop marks the pack stopped on disable.
func (h *Handler) Stop(ctx context.Context) error {
	h.started.Store(false)
	return nil
}

// Routes mounts the knowledge-graph surface natively.
func (h *Handler) Routes() []packruntime.BackendRoute {
	return []packruntime.BackendRoute{
		{
			Methods: []string{http.MethodGet, http.MethodPost, http.MethodDelete},
			Path:    "/v1/graph/entities",
			Handler: h.handleEntities,
		},
		{
			Methods: []string{http.MethodGet, http.MethodPost},
			Path:    "/v1/graph/relations",
			Handler: h.handleRelations,
		},
		{Methods: []string{http.MethodGet}, Path: "/v1/graph/context", Handler: h.handleContext},
		{Methods: []string{http.MethodGet}, Path: "/v1/graph/stats", Handler: h.handleStats},
	}
}

// graph returns the knowledge graph, or nil when memory/pipeline is unconfigured.
func (h *Handler) graph() *memory.Graph {
	if h.gw == nil {
		return nil
	}
	pipeline := h.gw.MemoryPipeline()
	if pipeline == nil {
		return nil
	}
	return pipeline.Graph()
}

func (h *Handler) handleEntities(w http.ResponseWriter, r *http.Request) {
	graph := h.graph()
	if graph == nil {
		json.NewEncoder(w).Encode(map[string]any{"entities": []any{}})
		return
	}
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		query := r.URL.Query().Get("q")
		if query != "" {
			results := graph.SearchEntities(query, 50)
			json.NewEncoder(w).Encode(map[string]any{"entities": results})
		} else {
			json.NewEncoder(w).Encode(map[string]any{"entities": graph.SearchEntities("", 100)})
		}
	case http.MethodPost:
		var req memory.Entity
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, "invalid entity")
			return
		}
		e := graph.PutEntity(req)
		json.NewEncoder(w).Encode(e)
	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			apperror.WriteCode(w, apperror.CodeBadRequest, "id required")
			return
		}
		graph.RemoveEntity(id)
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "method not allowed")
	}
}

func (h *Handler) handleRelations(w http.ResponseWriter, r *http.Request) {
	graph := h.graph()
	if graph == nil {
		json.NewEncoder(w).Encode(map[string]any{"relations": []any{}})
		return
	}
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		entityID := r.URL.Query().Get("entity_id")
		if entityID == "" {
			// Return all relations (for graph visualization)
			rels := graph.AllRelations(500)
			json.NewEncoder(w).Encode(map[string]any{"relations": rels})
			return
		}
		rels := graph.GetRelations(entityID)
		json.NewEncoder(w).Encode(map[string]any{"relations": rels})
	case http.MethodPost:
		var req memory.Relation
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, "invalid relation")
			return
		}
		rel := graph.PutRelation(req)
		json.NewEncoder(w).Encode(rel)
	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "method not allowed")
	}
}

func (h *Handler) handleContext(w http.ResponseWriter, r *http.Request) {
	graph := h.graph()
	if graph == nil {
		json.NewEncoder(w).Encode(map[string]string{"context": ""})
		return
	}
	w.Header().Set("Content-Type", "application/json")

	entityID := r.URL.Query().Get("entity_id")
	if entityID == "" {
		// Search by name
		name := r.URL.Query().Get("name")
		if name != "" {
			if e, ok := graph.FindByName(name); ok {
				entityID = e.ID
			}
		}
	}
	if entityID == "" {
		json.NewEncoder(w).Encode(map[string]string{"context": ""})
		return
	}

	ctx := graph.ContextFor(entityID)
	neighbors := graph.Neighbors(entityID, 2)
	json.NewEncoder(w).Encode(map[string]any{
		"context":   ctx,
		"neighbors": neighbors,
	})
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	graph := h.graph()
	if graph == nil {
		json.NewEncoder(w).Encode(map[string]int{"entities": 0, "relations": 0})
		return
	}
	json.NewEncoder(w).Encode(graph.Stats())
}
