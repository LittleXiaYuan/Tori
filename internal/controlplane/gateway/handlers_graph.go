package gateway

import (
	"encoding/json"
	"net/http"
	"strings"

	"yunque-agent/internal/agentcore/memory"
	"yunque-agent/internal/apperror"
)

// --- Knowledge Graph API ---

func (g *Gateway) handleGraphEntities(w http.ResponseWriter, r *http.Request) {
	if g.pipeline == nil || g.pipeline.Graph() == nil {
		json.NewEncoder(w).Encode(map[string]any{"entities": []any{}})
		return
	}
	graph := g.pipeline.Graph()
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

func (g *Gateway) handleGraphRelations(w http.ResponseWriter, r *http.Request) {
	if g.pipeline == nil || g.pipeline.Graph() == nil {
		json.NewEncoder(w).Encode(map[string]any{"relations": []any{}})
		return
	}
	graph := g.pipeline.Graph()
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		entityID := r.URL.Query().Get("entity_id")
		if entityID == "" {
			apperror.WriteCode(w, apperror.CodeBadRequest, "entity_id required")
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

func (g *Gateway) handleGraphContext(w http.ResponseWriter, r *http.Request) {
	if g.pipeline == nil || g.pipeline.Graph() == nil {
		json.NewEncoder(w).Encode(map[string]string{"context": ""})
		return
	}
	graph := g.pipeline.Graph()
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

func (g *Gateway) handleGraphStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.pipeline == nil || g.pipeline.Graph() == nil {
		json.NewEncoder(w).Encode(map[string]int{"entities": 0, "relations": 0})
		return
	}
	json.NewEncoder(w).Encode(g.pipeline.Graph().Stats())
}

// WireGraphToPlanner connects the pipeline's knowledge graph to the planner's context injection.
// Call this after creating both pipeline and planner.
func (g *Gateway) WireGraphToPlanner() {
	if g.pipeline == nil || g.pipeline.Graph() == nil {
		return
	}
	graph := g.pipeline.Graph()
	g.planner.SetGraphContext(func(query string) string {
		// Search graph for entities matching the user query
		entities := graph.SearchEntities(query, 5)
		if len(entities) == 0 {
			return ""
		}
		var parts []string
		for _, e := range entities {
			ctx := graph.ContextFor(e.ID)
			if ctx != "" {
				parts = append(parts, ctx)
			}
		}
		if len(parts) == 0 {
			return ""
		}
		return strings.Join(parts, "\n---\n")
	})
}
