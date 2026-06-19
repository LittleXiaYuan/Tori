// Package plannerrecoverypack mounts planner checkpoint recovery as a native
// capability pack. The recovery implementation is still hosted by the gateway
// while it is being carved into a shared service; this pack owns route
// registration and Pack Runtime enablement.
package plannerrecoverypack

import (
	"context"
	"net/http"
	"sync/atomic"

	"yunque-agent/pkg/packruntime"
)

const PackID = "yunque.pack.planner-recovery"

type Gateway interface {
	HandlePlannerCheckpoints(http.ResponseWriter, *http.Request)
	HandlePlannerExecutionState(http.ResponseWriter, *http.Request)
	HandlePlannerCheckpointRecover(http.ResponseWriter, *http.Request)
	HandlePlannerCheckpointResumeTask(http.ResponseWriter, *http.Request)
	HandlePlannerCheckpointResumePlan(http.ResponseWriter, *http.Request)
	HandlePlannerCheckpointResumePlanJob(http.ResponseWriter, *http.Request)
}

type Handler struct {
	gateway Gateway
	host    packruntime.Host
	started atomic.Bool
}

func New(gateway Gateway) *Handler {
	return &Handler{gateway: gateway}
}

var _ packruntime.Module = (*Handler)(nil)

func (h *Handler) PackID() string { return PackID }

func (h *Handler) Init(host packruntime.Host) error {
	h.host = host
	return nil
}

func (h *Handler) Start(ctx context.Context) error {
	h.started.Store(true)
	if h.host != nil {
		h.host.Logger().Info("planner recovery pack started", "pack", PackID)
	}
	return nil
}

func (h *Handler) Stop(ctx context.Context) error {
	h.started.Store(false)
	return nil
}

func (h *Handler) Routes() []packruntime.BackendRoute {
	return []packruntime.BackendRoute{
		{Method: http.MethodGet, Path: "/v1/planner/checkpoints", Handler: h.handlePlannerCheckpoints},
		{Method: http.MethodGet, Path: "/v1/planner/execution-state", Handler: h.handlePlannerExecutionState},
		{Method: http.MethodPost, Path: "/v1/planner/checkpoints/recover", Handler: h.handlePlannerCheckpointRecover},
		{Method: http.MethodPost, Path: "/v1/planner/checkpoints/resume", Handler: h.handlePlannerCheckpointResumeTask},
		{Method: http.MethodPost, Path: "/v1/planner/checkpoints/resume-plan", Handler: h.handlePlannerCheckpointResumePlan},
		{Method: http.MethodGet, Path: "/v1/planner/checkpoints/resume-plan/jobs", Handler: h.handlePlannerCheckpointResumePlanJob},
	}
}

func RouteSpecs() []packruntime.BackendRouteSpec {
	return []packruntime.BackendRouteSpec{
		{Method: http.MethodGet, Path: "/v1/planner/checkpoints", Description: "List recoverable planner checkpoints."},
		{Method: http.MethodGet, Path: "/v1/planner/execution-state", Description: "Read a normalized planner recovery scene."},
		{Method: http.MethodPost, Path: "/v1/planner/checkpoints/recover", Description: "Build a recovery prompt and recovery plan from a checkpoint."},
		{Method: http.MethodPost, Path: "/v1/planner/checkpoints/resume", Description: "Create a task from a planner checkpoint recovery plan."},
		{Method: http.MethodPost, Path: "/v1/planner/checkpoints/resume-plan", Description: "Resume a planner checkpoint directly through the planner DAG runner."},
		{Method: http.MethodGet, Path: "/v1/planner/checkpoints/resume-plan/jobs", Description: "Read async planner resume job status."},
	}
}

func Paths() []string {
	return []string{
		"/v1/planner/checkpoints",
		"/v1/planner/execution-state",
		"/v1/planner/checkpoints/recover",
		"/v1/planner/checkpoints/resume",
		"/v1/planner/checkpoints/resume-plan",
		"/v1/planner/checkpoints/resume-plan/jobs",
	}
}

func (h *Handler) handle(fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if fn == nil {
			http.Error(w, "planner recovery pack not wired", http.StatusServiceUnavailable)
			return
		}
		fn(w, r)
	}
}

func (h *Handler) handlePlannerCheckpoints(w http.ResponseWriter, r *http.Request) {
	if h.gateway == nil {
		h.handle(nil)(w, r)
		return
	}
	h.handle(h.gateway.HandlePlannerCheckpoints)(w, r)
}

func (h *Handler) handlePlannerExecutionState(w http.ResponseWriter, r *http.Request) {
	if h.gateway == nil {
		h.handle(nil)(w, r)
		return
	}
	h.handle(h.gateway.HandlePlannerExecutionState)(w, r)
}

func (h *Handler) handlePlannerCheckpointRecover(w http.ResponseWriter, r *http.Request) {
	if h.gateway == nil {
		h.handle(nil)(w, r)
		return
	}
	h.handle(h.gateway.HandlePlannerCheckpointRecover)(w, r)
}

func (h *Handler) handlePlannerCheckpointResumeTask(w http.ResponseWriter, r *http.Request) {
	if h.gateway == nil {
		h.handle(nil)(w, r)
		return
	}
	h.handle(h.gateway.HandlePlannerCheckpointResumeTask)(w, r)
}

func (h *Handler) handlePlannerCheckpointResumePlan(w http.ResponseWriter, r *http.Request) {
	if h.gateway == nil {
		h.handle(nil)(w, r)
		return
	}
	h.handle(h.gateway.HandlePlannerCheckpointResumePlan)(w, r)
}

func (h *Handler) handlePlannerCheckpointResumePlanJob(w http.ResponseWriter, r *http.Request) {
	if h.gateway == nil {
		h.handle(nil)(w, r)
		return
	}
	h.handle(h.gateway.HandlePlannerCheckpointResumePlanJob)(w, r)
}
