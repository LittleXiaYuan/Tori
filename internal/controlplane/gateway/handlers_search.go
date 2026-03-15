package gateway

import (
	"encoding/json"
	"net/http"
	"strconv"

	"yunque-agent/internal/apperror"
)

func (g *Gateway) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET only")
		return
	}
	if g.searchReg == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "search not configured")
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
		json.NewEncoder(w).Encode(map[string]any{"providers": []string{}})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"providers": g.searchReg.List(),
	})
}
