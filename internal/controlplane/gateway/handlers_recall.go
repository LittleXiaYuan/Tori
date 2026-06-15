package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

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

// MemoryOrchestrator exposes the memory orchestrator to backend packs (e.g. the
// memory pack's native recall/debug + persona handlers). May be nil if memory is
// not configured; the pack resolves it lazily so wiring order does not matter.
func (g *Gateway) MemoryOrchestrator() *memory.Orchestrator { return g.orchestrator }

// TenantOf exposes the gateway's tenant-from-context resolution to backend packs
// so their native handlers can scope to the caller's tenant.
func (g *Gateway) TenantOf(ctx context.Context) string { return tenantFromCtx(ctx) }

// Memory HTTP handlers (stats / search / recall-debug / persona / update) were
// filled into the memory pack (internal/packs/memory); they now live there and
// talk to the manager / pipeline / orchestrator directly. The pack reads the
// orchestrator through the MemoryOrchestrator() accessor above.

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
