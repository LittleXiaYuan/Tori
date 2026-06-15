package gateway

import (
	"context"
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

// MemoryManager exposes the memory manager to backend packs (e.g. the memory
// pack's native read handlers). May be nil if memory is not configured.
func (g *Gateway) MemoryManager() *memory.Manager { return g.memory }

// MemoryPipeline exposes the memory pipeline to backend packs (e.g. the memory
// pack's native /v1/memory/compact handler). May be nil if not configured.
func (g *Gateway) MemoryPipeline() *memory.Pipeline { return g.pipeline }

// TenantOf exposes the gateway's tenant-from-context resolution to backend packs
// so their native handlers can scope to the caller's tenant.
func (g *Gateway) TenantOf(ctx context.Context) string { return tenantFromCtx(ctx) }

// Memory read handlers (stats / search) were filled into the memory pack
// (internal/packs/memory); they now live there and talk to the manager directly.
// The orchestrator/pipeline-dependent handlers below remain on the gateway.

// handleMemoryRecallDebug exposes what the orchestrator recalls for a query,
// scoped to the caller's tenant: per-item score breakdown (layer, raw score,
// final score, access count, age) plus the final compiled context string that
// would be injected into the planner. Use it to validate that "记得你" recall
// actually surfaces the right memories and to debug ranking during experiments.
func (g *Gateway) handleMemoryRecallDebug(w http.ResponseWriter, r *http.Request) {
	if g.orchestrator == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "memory orchestrator not configured")
		return
	}
	tid := tenantFromCtx(r.Context())
	query := r.URL.Query().Get("q")
	limit := 10
	if r.Method == http.MethodPost {
		var req struct {
			Query string `json:"query"`
			Limit int    `json:"limit"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.Query != "" {
			query = req.Query
		}
		if req.Limit > 0 {
			limit = req.Limit
		}
	}
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	if query == "" {
		apperror.WriteCode(w, apperror.CodeMissingField, "query is required (q= or body.query)")
		return
	}

	items := g.orchestrator.Recall(r.Context(), tid, query, limit)
	type debugItem struct {
		Content     string  `json:"content"`
		Source      string  `json:"source"`
		Category    string  `json:"category,omitempty"`
		Score       float64 `json:"score"`
		RawScore    float64 `json:"raw_score"`
		AccessCount int     `json:"access_count"`
		AgeSeconds  float64 `json:"age_seconds"`
	}
	out := make([]debugItem, 0, len(items))
	for _, it := range items {
		out = append(out, debugItem{
			Content:     it.Content,
			Source:      it.Source,
			Category:    it.Category,
			Score:       it.Score,
			RawScore:    it.RawScore,
			AccessCount: it.AccessCount,
			AgeSeconds:  it.Age.Seconds(),
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"tenant":           tid,
		"query":            query,
		"items":            out,
		"count":            len(out),
		"compiled_context": g.orchestrator.CompileContext(r.Context(), tid, query),
	})
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

// Knowledge Graph HTTP handlers (/v1/graph/{entities,relations,context,stats})
// were de-shelled into the graph pack (internal/packs/graph). The pack reads the
// graph through the MemoryPipeline() accessor; the planner wiring below stays on
// the gateway because it injects graph context into the prompt path, not HTTP.

// WireGraphToPlanner connects the pipeline's knowledge graph to the planner's context injection.
// It preserves any previously-set graphContext (e.g. knowledge base retrieval, Ledger recall).
func (g *Gateway) WireGraphToPlanner() {
	if g.pipeline == nil || g.pipeline.Graph() == nil {
		return
	}
	graph := g.pipeline.Graph()

	g.planner.AppendGraphContext(func(query string) string {
		var parts []string

		// Knowledge graph entity search
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
