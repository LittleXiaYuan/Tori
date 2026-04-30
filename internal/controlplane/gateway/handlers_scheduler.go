package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"yunque-agent/internal/apperror"
	"yunque-agent/internal/execution/scheduler"
)

func (g *Gateway) handleSchedulerJobs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	jobs := g.scheduler.List()
	json.NewEncoder(w).Encode(map[string]any{"jobs": jobs, "count": len(jobs)})
}

func (g *Gateway) handleSchedulerAdd(w http.ResponseWriter, r *http.Request) {
	tid := tenantFromCtx(r.Context())
	var req struct {
		Name     string `json:"name"`
		Prompt   string `json:"prompt"`
		Interval string `json:"interval"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" || req.Prompt == "" {
		apperror.WriteCode(w, apperror.CodeMissingField, "name and prompt are required")
		return
	}
	dur, err := time.ParseDuration(req.Interval)
	if err != nil || dur < time.Minute {
		apperror.WriteCode(w, apperror.CodeInvalidField, "invalid interval (min 1m)")
		return
	}
	job := scheduler.Job{
		ID:       fmt.Sprintf("job_%d", time.Now().UnixNano()),
		Name:     req.Name,
		TenantID: tid,
		Interval: dur,
		Prompt:   req.Prompt,
	}
	g.scheduler.Add(job)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(job)
}

func (g *Gateway) handleSchedulerRemove(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		apperror.WriteCode(w, apperror.CodeMissingField, "id is required")
		return
	}
	g.scheduler.Remove(req.ID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "removed"})
}
