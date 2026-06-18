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

// Route declares one scheduler HTTP route.
type Route struct {
	Method      string
	Path        string
	Description string
	Handler     http.HandlerFunc
}

// Handler serves scheduler job management HTTP endpoints.
type Handler struct {
	Scheduler     *scheduler.Scheduler
	SchedulerFunc func() *scheduler.Scheduler
}

// RouteSpecs returns the scheduler surface without mounting it. Pack Runtime
// uses this to own route registration while preserving the existing handler
// implementation.
func (h *Handler) RouteSpecs() []Route {
	return []Route{
		{Method: http.MethodGet, Path: "/v1/scheduler/jobs", Description: "List scheduled jobs.", Handler: h.handleJobs},
		{Method: http.MethodPost, Path: "/v1/scheduler/add", Description: "Add a scheduled job.", Handler: h.handleAdd},
		{Method: http.MethodPost, Path: "/v1/scheduler/remove", Description: "Remove a scheduled job.", Handler: h.handleRemove},
	}
}

// RegisterRoutes mounts all /v1/scheduler/* endpoints.
func (h *Handler) RegisterRoutes(mux *http.ServeMux, auth gwshared.AuthFunc) {
	for _, route := range h.RouteSpecs() {
		mux.HandleFunc(route.Path, auth(route.Handler))
	}
}

func (h *Handler) scheduler() *scheduler.Scheduler {
	if h.SchedulerFunc != nil {
		return h.SchedulerFunc()
	}
	return h.Scheduler
}

func (h *Handler) handleJobs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	sched := h.scheduler()
	if sched == nil {
		json.NewEncoder(w).Encode(map[string]any{"jobs": []any{}, "count": 0, "error": "scheduler not available"})
		return
	}
	jobs := sched.List()
	json.NewEncoder(w).Encode(map[string]any{"jobs": jobs, "count": len(jobs)})
}

func (h *Handler) handleAdd(w http.ResponseWriter, r *http.Request) {
	sched := h.scheduler()
	if sched == nil {
		gwshared.WriteJSONStatus(w, http.StatusServiceUnavailable, map[string]any{"error": "scheduler not available"})
		return
	}
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
	sched.Add(job)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(job)
}

func (h *Handler) handleRemove(w http.ResponseWriter, r *http.Request) {
	sched := h.scheduler()
	if sched == nil {
		gwshared.WriteJSONStatus(w, http.StatusServiceUnavailable, map[string]any{"error": "scheduler not available"})
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		apperror.WriteCode(w, apperror.CodeMissingField, "id is required")
		return
	}
	sched.Remove(req.ID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "removed"})
}
