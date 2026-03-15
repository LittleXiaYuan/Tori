package gateway

import (
	"encoding/json"
	"net/http"

	"yunque-agent/internal/agentcore/cron"
)

func (g *Gateway) handleCronList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.cronMgr == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "cron not configured"})
		return
	}
	json.NewEncoder(w).Encode(map[string]any{"jobs": g.cronMgr.List()})
}

func (g *Gateway) handleCronAdd(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.cronMgr == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "cron not configured"})
		return
	}
	var req struct {
		Name     string       `json:"name"`
		Schedule cron.Schedule `json:"schedule"`
		Payload  cron.Payload  `json:"payload"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		json.NewEncoder(w).Encode(map[string]string{"error": "name, schedule, and payload required"})
		return
	}
	id, err := g.cronMgr.Add(req.Name, req.Schedule, req.Payload)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	job, _ := g.cronMgr.Get(id)
	json.NewEncoder(w).Encode(map[string]any{"job": job})
}

func (g *Gateway) handleCronRemove(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.cronMgr == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "cron not configured"})
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		json.NewEncoder(w).Encode(map[string]string{"error": "job id required"})
		return
	}
	if err := g.cronMgr.Remove(id); err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"deleted": id})
}

func (g *Gateway) handleCronRun(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.cronMgr == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "cron not configured"})
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		json.NewEncoder(w).Encode(map[string]string{"error": "job id required"})
		return
	}
	rec, err := g.cronMgr.RunNow(id)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]any{"run": rec})
}
