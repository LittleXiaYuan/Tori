package gateway

import (
	"encoding/json"
	"net/http"

	"yunque-agent/internal/agentcore/memory"
	"yunque-agent/internal/apperror"
)

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
