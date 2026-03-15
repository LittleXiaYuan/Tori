package gateway

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"yunque-agent/internal/apperror"
)

func (g *Gateway) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	if g.heartbeat == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "heartbeat not configured")
		return
	}
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"running": g.heartbeat.IsRunning(),
		})
	case http.MethodPut:
		var req struct {
			Enabled  *bool `json:"enabled"`
			Interval *int  `json:"interval_minutes"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.Enabled != nil {
			g.heartbeat.SetEnabled(r.Context(), *req.Enabled)
		}
		if req.Interval != nil && *req.Interval > 0 {
			g.heartbeat.SetInterval(r.Context(), time.Duration(*req.Interval)*time.Minute)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET or PUT")
	}
}

func (g *Gateway) handleHeartbeatTrigger(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	if g.heartbeat == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "heartbeat not configured")
		return
	}
	entry := g.heartbeat.Trigger(r.Context())
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entry)
}

func (g *Gateway) handleHeartbeatLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET only")
		return
	}
	if g.heartbeat == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "heartbeat not configured")
		return
	}
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	logs := g.heartbeat.Logs(limit)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(logs)
}
