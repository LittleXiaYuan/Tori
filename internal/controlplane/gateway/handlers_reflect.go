package gateway

import (
	"encoding/json"
	"net/http"

	"yunque-agent/internal/apperror"
)

// ────────────────────────────────────────────────────────────
// Reflection API — experience store and strategy compilation
//
// GET /v1/reflect/experiences       — list experiences (filter by source/category/outcome, ?stats=true)
// GET /v1/reflect/strategies        — compiled strategy hints for LLM context
// ────────────────────────────────────────────────────────────

func (g *Gateway) handleExperiences(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET only")
		return
	}
	if g.experienceStore == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "experience store not initialized")
		return
	}

	// Stats mode
	if r.URL.Query().Get("stats") == "true" {
		st := g.experienceStore.Stats()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(st)
		return
	}

	// Search mode
	query := r.URL.Query().Get("q")
	if query != "" {
		results := g.experienceStore.Search(query, 50)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"experiences": results, "total": len(results)})
		return
	}

	// List all
	all := g.experienceStore.All()
	// Apply filters
	source := r.URL.Query().Get("source")
	category := r.URL.Query().Get("category")
	outcome := r.URL.Query().Get("outcome")

	if source != "" || category != "" || outcome != "" {
		filtered := make([]any, 0)
		for _, e := range all {
			if source != "" && e.Source != source {
				continue
			}
			if category != "" && e.Category != category {
				continue
			}
			if outcome != "" && e.Outcome != outcome {
				continue
			}
			filtered = append(filtered, e)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"experiences": filtered, "total": len(filtered)})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"experiences": all, "total": len(all)})
}

func (g *Gateway) handleStrategies(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET only")
		return
	}
	if g.experienceStore == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "experience store not initialized")
		return
	}

	strategies := g.experienceStore.CompileStrategies(20)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"strategies": strategies})
}
