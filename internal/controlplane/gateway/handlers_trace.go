package gateway

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// ──────────────────────────────────────────────
// Trace API — query execution traces for audit/replay
//
// GET /v1/trace/{trace_id}     → events for a specific trace
// GET /v1/trace/recent?limit=N → most recent events
// GET /v1/trace/task/{task_id} → events for a specific task
// ──────────────────────────────────────────────

func (g *Gateway) handleTraceByID(w http.ResponseWriter, r *http.Request) {
	if g.eventTrail == nil {
		http.Error(w, "audit trail not available", http.StatusServiceUnavailable)
		return
	}

	// Extract trace_id from path: /v1/trace/{trace_id}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/v1/trace/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "trace_id required", http.StatusBadRequest)
		return
	}
	traceID := parts[0]

	events := g.eventTrail.QueryByTraceID(traceID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"trace_id": traceID,
		"count":    len(events),
		"events":   events,
	})
}

func (g *Gateway) handleTraceRecent(w http.ResponseWriter, r *http.Request) {
	if g.eventTrail == nil {
		http.Error(w, "audit trail not available", http.StatusServiceUnavailable)
		return
	}

	limit := 50
	if s := r.URL.Query().Get("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > 500 {
		limit = 500
	}

	events := g.eventTrail.Recent(limit)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"count":  len(events),
		"events": events,
	})
}

func (g *Gateway) handleTraceByTask(w http.ResponseWriter, r *http.Request) {
	if g.eventTrail == nil {
		http.Error(w, "audit trail not available", http.StatusServiceUnavailable)
		return
	}

	// Extract task_id from path: /v1/trace/task/{task_id}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/v1/trace/task/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "task_id required", http.StatusBadRequest)
		return
	}
	taskID := parts[0]

	events := g.eventTrail.QueryByTaskID(taskID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"task_id": taskID,
		"count":   len(events),
		"events":  events,
	})
}
