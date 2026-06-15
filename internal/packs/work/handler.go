// Package workpack mounts the task + project HTTP surface as a Pack Runtime
// backend module. Bridge phase: the pack owns route registration + the
// enable/disable gate, while the gateway still hosts the handler
// implementations (via the narrow WorkGateway interface). Planner recovery,
// missions, state kernel, reflection and documents stay on the gateway;
// workflows remain in the workflowapi sub-package.
package workpack

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync/atomic"

	"yunque-agent/internal/agentcore/task"
	"yunque-agent/internal/apperror"
	"yunque-agent/internal/controlplane/gateway/workflowapi"
	"yunque-agent/internal/orchestrator"
	"yunque-agent/pkg/packruntime"
	"yunque-agent/pkg/safego"
)

const PackID = "yunque.pack.work"

// WorkGateway is the narrow gateway surface the work pack needs. The whole
// task + project surface is de-shelled into this pack; the gateway is reached
// only through these typed subsystem accessors (no bridge).
type WorkGateway interface {
	TaskStore() task.Store
	TaskRunner() *task.Runner
	TenantOf(ctx context.Context) string
	GapAnalyzer() *task.GapAnalyzer
	TemplateStore() *task.TemplateStore
	WorkMemManager() *task.WorkingMemoryManager
	ThreadManager() *task.ThreadManager
	ProjectStore() *orchestrator.ProjectStore
	WorkflowHandler() *workflowapi.Handler
}

// Handler is the work pack's backend module.
type Handler struct {
	gateway WorkGateway
	host    packruntime.Host
	started atomic.Bool
}

// NewHandler builds the work pack backed by the gateway bridge.
func NewHandler(gateway WorkGateway) *Handler { return &Handler{gateway: gateway} }

// PackID returns the stable manifest id.
func (h *Handler) PackID() string { return PackID }

// compile-time assertion: Work is a v2 capability Module (Tier 0 microkernel).
var _ packruntime.Module = (*Handler)(nil)

// Init wires the pack against the kernel Host. The pack already depends on the
// narrow WorkGateway interface, not the concrete Gateway.
func (h *Handler) Init(host packruntime.Host) error {
	h.host = host
	return nil
}

// Start marks the pack live on enable.
func (h *Handler) Start(ctx context.Context) error {
	h.started.Store(true)
	if h.host != nil {
		h.host.Logger().Info("work pack started", "pack", PackID)
	}
	return nil
}

// Stop marks the pack stopped on disable.
func (h *Handler) Stop(ctx context.Context) error {
	h.started.Store(false)
	return nil
}

// Routes mounts the task + project surface. Methods are declared broadly so the
// bridge keeps the original routes' permissive (handler-decides) method
// behavior; the manifest lists these as path-only routes so the pack gate allows
// any method.
func (h *Handler) Routes() []packruntime.BackendRoute {
	methods := []string{
		http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch,
	}
	mk := func(p string, fn http.HandlerFunc) packruntime.BackendRoute {
		return packruntime.BackendRoute{Methods: methods, Path: p, Handler: fn}
	}
	routes := []packruntime.BackendRoute{
		// Task collection + lifecycle.
		mk("/v1/tasks", h.handleTasks),
		mk("/v1/tasks/run", h.handleTaskRun),
		mk("/v1/tasks/cancel", h.handleTaskCancel),
		mk("/v1/tasks/pause", h.handleTaskPause),
		mk("/v1/tasks/resume", h.handleTaskResume),
		mk("/v1/tasks/restart", h.handleTaskRestart),
		// Gaps / working-memory / templates / threads.
		mk("/v1/tasks/gaps", h.handleGaps),
		mk("/v1/tasks/gaps/resolve", h.handleGapResolve),
		mk("/v1/tasks/memory", h.handleTaskWorkingMemory),
		mk("/v1/tasks/threads", h.handleTaskThread),
		mk("/v1/tasks/templates", h.handleTemplates),
		mk("/v1/tasks/templates/instantiate", h.handleTemplateInstantiate),
		// Projects.
		mk("/v1/projects", h.handleProjects),
		mk("/v1/projects/detail", h.handleProjectDetail),
		mk("/v1/projects/remove", h.handleProjectRemove),
	}
	// Workflow surface — merged in so tasks + workflow share one task platform.
	if wf := h.gateway.WorkflowHandler(); wf != nil {
		for _, rt := range wf.RouteSpecs() {
			routes = append(routes, mk(rt.Path, rt.Handler))
		}
	}
	return routes
}

func (h *Handler) decodeID(w http.ResponseWriter, r *http.Request) (string, bool) {
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON")
		return "", false
	}
	if req.ID == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "id is required")
		return "", false
	}
	return req.ID, true
}

// ready reports whether the task store + runner are wired, writing a 404 if not.
func (h *Handler) ready(w http.ResponseWriter) (task.Store, *task.Runner, bool) {
	store := h.gateway.TaskStore()
	runner := h.gateway.TaskRunner()
	if store == nil || runner == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "task runtime not available")
		return nil, nil, false
	}
	return store, runner, true
}

func (h *Handler) handleTaskRun(w http.ResponseWriter, r *http.Request) {
	store, runner, ok := h.ready(w)
	if !ok {
		return
	}
	id, ok := h.decodeID(w, r)
	if !ok {
		return
	}
	t, ok := store.Get(id)
	if !ok {
		apperror.WriteCode(w, apperror.CodeNotFound, "task not found")
		return
	}
	if t.IsTerminal() {
		apperror.WriteCode(w, apperror.CodeBadRequest, "task already finished")
		return
	}
	safego.Go("task-run-"+id, func() {
		if err := runner.Run(context.Background(), id); err != nil {
			slog.Warn("task execution failed", "task", id, "err", err)
		}
	})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "accepted", "task_id": id})
}

func (h *Handler) handleTaskCancel(w http.ResponseWriter, r *http.Request) {
	store, runner, ok := h.ready(w)
	if !ok {
		return
	}
	id, ok := h.decodeID(w, r)
	if !ok {
		return
	}
	t, ok := store.Get(id)
	if !ok {
		apperror.WriteCode(w, apperror.CodeNotFound, "task not found")
		return
	}
	if t.IsTerminal() {
		apperror.WriteCode(w, apperror.CodeBadRequest, "task already finished")
		return
	}
	if !runner.Cancel(id) {
		apperror.WriteCode(w, apperror.CodeBadRequest, "task is not currently running")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "cancelling", "task_id": id})
}

func (h *Handler) handleTaskPause(w http.ResponseWriter, r *http.Request) {
	_, runner, ok := h.ready(w)
	if !ok {
		return
	}
	id, ok := h.decodeID(w, r)
	if !ok {
		return
	}
	if !runner.Pause(id) {
		apperror.WriteCode(w, apperror.CodeBadRequest, "task is not currently running")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "pausing", "task_id": id})
}

func (h *Handler) handleTaskResume(w http.ResponseWriter, r *http.Request) {
	store, runner, ok := h.ready(w)
	if !ok {
		return
	}
	id, ok := h.decodeID(w, r)
	if !ok {
		return
	}
	t, ok := store.Get(id)
	if !ok {
		apperror.WriteCode(w, apperror.CodeNotFound, "task not found")
		return
	}
	if !t.IsResumable() {
		apperror.WriteCode(w, apperror.CodeBadRequest, fmt.Sprintf("task in state %s is not resumable", t.Status))
		return
	}
	safego.Go("task-resume-"+id, func() {
		if err := runner.Resume(context.Background(), id); err != nil {
			slog.Warn("task resume failed", "task", id, "err", err)
		}
	})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "resuming", "task_id": id})
}

func (h *Handler) handleTaskRestart(w http.ResponseWriter, r *http.Request) {
	store, runner, ok := h.ready(w)
	if !ok {
		return
	}
	id, ok := h.decodeID(w, r)
	if !ok {
		return
	}
	t, ok := store.Get(id)
	if !ok {
		apperror.WriteCode(w, apperror.CodeNotFound, "task not found")
		return
	}
	if !t.IsTerminal() && !t.IsResumable() {
		apperror.WriteCode(w, apperror.CodeBadRequest, fmt.Sprintf("task in state %s cannot be restarted", t.Status))
		return
	}
	safego.Go("task-restart-"+id, func() {
		if err := runner.Restart(context.Background(), id); err != nil {
			slog.Warn("task restart failed", "task", id, "err", err)
		}
	})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "restarting", "task_id": id})
}

// handleTasks is the /v1/tasks collection route (method-dispatched).
func (h *Handler) handleTasks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleTaskList(w, r)
	case http.MethodPost:
		h.handleTaskCreate(w, r)
	case http.MethodDelete:
		h.handleTaskDelete(w, r)
	default:
		apperror.WriteCode(w, apperror.CodeBadRequest, "method not allowed")
	}
}

func (h *Handler) handleTaskCreate(w http.ResponseWriter, r *http.Request) {
	store, _, ok := h.ready(w)
	if !ok {
		return
	}
	var req task.CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON")
		return
	}
	req.TenantID = h.gateway.TenantOf(r.Context())
	t, err := store.Create(req)
	if err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(t)
}

func (h *Handler) handleTaskList(w http.ResponseWriter, r *http.Request) {
	store := h.gateway.TaskStore()
	if store == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "task runtime not available")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if id := r.URL.Query().Get("id"); id != "" {
		t, ok := store.Get(id)
		if !ok {
			apperror.WriteCode(w, apperror.CodeNotFound, "task not found")
			return
		}
		_ = json.NewEncoder(w).Encode(t)
		return
	}
	tasks := store.List(h.gateway.TenantOf(r.Context()), 50)
	_ = json.NewEncoder(w).Encode(tasks)
}

func (h *Handler) handleTaskDelete(w http.ResponseWriter, r *http.Request) {
	store := h.gateway.TaskStore()
	if store == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "task runtime not available")
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "id query param required")
		return
	}
	if !store.Delete(id) {
		apperror.WriteCode(w, apperror.CodeNotFound, "task not found")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"deleted": id})
}
