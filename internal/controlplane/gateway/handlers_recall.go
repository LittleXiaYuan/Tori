package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"yunque-agent/internal/agentcore/memory"
	"yunque-agent/internal/agentcore/websearch"
	"yunque-agent/internal/apperror"
)

// from handlers_memory.go
func (g *Gateway) handleMemoryStats(w http.ResponseWriter, r *http.Request) {
	tid := tenantFromCtx(r.Context())
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(g.memory.Stats(tid))
}

func (g *Gateway) handleMemorySearch(w http.ResponseWriter, r *http.Request) {
	tid := tenantFromCtx(r.Context())
	var req struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Limit <= 0 {
		req.Limit = 10
	}
	items, _ := g.memory.SearchAll(r.Context(), tid, req.Query, req.Limit)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"results": items, "count": len(items)})
}

func (g *Gateway) handleMemoryAdd(w http.ResponseWriter, r *http.Request) {
	tid := tenantFromCtx(r.Context())
	var req struct {
		Key    string `json:"key"`
		Value  string `json:"value"`
		Layer  string `json:"layer"` // "short", "mid", "long"
		Source string `json:"source"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Value == "" {
		apperror.WriteCode(w, apperror.CodeMissingField, "value is required")
		return
	}
	item := memory.Item{Key: req.Key, Value: req.Value, Source: req.Source}
	var err error
	switch req.Layer {
	case "long":
		err = g.memory.AddLong(r.Context(), tid, item)
	case "short":
		err = g.memory.Short.Put(r.Context(), tid, item)
	default:
		err = g.memory.AddMid(r.Context(), tid, item)
	}
	if err != nil {
		apperror.Write(w, apperror.Wrap(apperror.CodeStorageError, "memory add failed", err))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (g *Gateway) handleMemoryCompact(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	tid := tenantFromCtx(r.Context())
	var req struct {
		TargetCount int `json:"target_count"`
		DecayDays   int `json:"decay_days"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.TargetCount <= 0 {
		req.TargetCount = 0 // auto
	}
	if g.pipeline == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "memory pipeline not configured")
		return
	}
	result, err := g.pipeline.Compact(r.Context(), tid, req.TargetCount, req.DecayDays)
	if err != nil {
		apperror.Write(w, apperror.Wrap(apperror.CodeInternal, "compact failed", err))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (g *Gateway) handleMemoryPersonaGet(w http.ResponseWriter, r *http.Request) {
	if g.orchestrator == nil || g.orchestrator.Editable() == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "memory orchestrator not configured")
		return
	}
	blocks := g.orchestrator.Editable().AllBlocks()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"blocks": blocks})
}

func (g *Gateway) handleMemoryPersonaUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	if g.orchestrator == nil || g.orchestrator.Editable() == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "memory orchestrator not configured")
		return
	}
	var req struct {
		ID      string `json:"id"`
		Label   string `json:"label"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteCode(w, apperror.CodeInvalidField, "invalid request")
		return
	}

	editable := g.orchestrator.Editable()
	if req.ID == "" {
		if req.Content == "" {
			apperror.WriteCode(w, apperror.CodeInvalidField, "content cannot be empty for new block")
			return
		}
		label := fmt.Sprintf("%s-%d", req.Label, time.Now().UnixNano())
		editable.AddBlock(label, req.Content, 0)
	} else {
		targetLabel := req.Label
		if targetLabel == "" {
			for _, b := range editable.AllBlocks() {
				if b.ID == req.ID {
					targetLabel = b.Label
					break
				}
			}
		}

		if targetLabel == "" {
			apperror.WriteCode(w, apperror.CodeNotFound, "memory block not found")
			return
		}

		if req.Content == "" {
			editable.RemoveBlock(targetLabel)
		} else {
			res := editable.Edit(memory.EditRequest{
				BlockLabel: targetLabel,
				Op:         memory.OpRethink,
				NewText:    req.Content,
			})
			if !res.Success {
				apperror.WriteCode(w, apperror.CodeInternal, res.Error)
				return
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

//  from handlers_graph.go
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
// It preserves any previously-set graphContext (e.g. knowledge base retrieval, Ledger recall).
func (g *Gateway) WireGraphToPlanner() {
	if g.pipeline == nil || g.pipeline.Graph() == nil {
		return
	}
	graph := g.pipeline.Graph()
	prevGraph := g.planner.GraphContext() // preserve KB + Ledger callbacks

	g.planner.SetGraphContext(func(query string) string {
		var parts []string

		// 1) Previous context (KB retrieval + Ledger recall)
		if prevGraph != nil {
			if prev := prevGraph(query); prev != "" {
				parts = append(parts, prev)
			}
		}

		// 2) Knowledge graph entity search
		entities := graph.SearchEntities(query, 5)
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

// from handlers_search.go
func (g *Gateway) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET only")
		return
	}
	if !g.searchOn.Load() {
		apperror.WriteCode(w, apperror.CodeBadRequest, "search is disabled")
		return
	}
	query := r.URL.Query().Get("q")
	if query == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "q is required")
		return
	}
	limit := 5
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	if g.searchReg == nil || len(g.searchReg.List()) == 0 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"results":   []websearch.Result{},
			"total":     0,
			"enabled":   false,
			"providers": []string{},
		})
		return
	}
	provider := r.URL.Query().Get("provider")
	var results any
	var err error
	if provider != "" {
		results, err = g.searchReg.SearchWith(r.Context(), provider, query, limit)
	} else {
		results, err = g.searchReg.Search(r.Context(), query, limit)
	}
	if err != nil {
		apperror.Write(w, apperror.Wrap(apperror.CodeInternal, "search failed", err))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"results": results,
	})
}

func (g *Gateway) handleSearchProviders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET only")
		return
	}
	if g.searchReg == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"enabled": false, "providers": []string{}})
		return
	}
	providers := g.searchReg.List()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"enabled":   g.searchOn.Load() && len(providers) > 0,
		"providers": providers,
	})
}
