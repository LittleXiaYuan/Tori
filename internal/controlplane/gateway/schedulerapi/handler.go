package schedulerapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"yunque-agent/internal/apperror"
	"yunque-agent/internal/controlplane/gateway/gwshared"
	"yunque-agent/internal/execution/scheduler"
)

// Handler serves scheduler job management HTTP endpoints.
type Handler struct {
	Scheduler *scheduler.Scheduler
}

// RegisterRoutes mounts all /v1/scheduler/* endpoints.
func (h *Handler) RegisterRoutes(mux *http.ServeMux, auth gwshared.AuthFunc) {
	mux.HandleFunc("/v1/scheduler/jobs", auth(h.handleJobs))
	mux.HandleFunc("/v1/scheduler/add", auth(h.handleAdd))
	mux.HandleFunc("/v1/scheduler/remove", auth(h.handleRemove))
}

func (h *Handler) handleJobs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	jobs := h.Scheduler.List()
	json.NewEncoder(w).Encode(map[string]any{"jobs": jobs, "count": len(jobs)})
}

func (h *Handler) handleAdd(w http.ResponseWriter, r *http.Request) {
	tid := gwshared.TenantFromCtx(r.Context())
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
	h.Scheduler.Add(job)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(job)
}

func (h *Handler) handleRemove(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		apperror.WriteCode(w, apperror.CodeMissingField, "id is required")
		return
	}
	h.Scheduler.Remove(req.ID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "removed"})
}
