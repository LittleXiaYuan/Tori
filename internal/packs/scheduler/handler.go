package schedulerpack

import (
	"context"
	"sync/atomic"

	"yunque-agent/internal/controlplane/gateway/schedulerapi"
	"yunque-agent/internal/execution/scheduler"
	"yunque-agent/pkg/packruntime"
)

const PackID = "yunque.pack.scheduler"

// Handler exposes scheduled job management as a native capability pack.
// schedulerapi remains the business implementation; Pack Runtime owns
// lifecycle, enablement and method gates.
type Handler struct {
	api     *schedulerapi.Handler
	host    packruntime.Host
	started atomic.Bool
}

func New(scheduler *scheduler.Scheduler) *Handler {
	return &Handler{api: &schedulerapi.Handler{Scheduler: scheduler}}
}

func NewProvider(scheduler func() *scheduler.Scheduler) *Handler {
	return &Handler{api: &schedulerapi.Handler{SchedulerFunc: scheduler}}
}

func (h *Handler) PackID() string { return PackID }

var _ packruntime.Module = (*Handler)(nil)

func (h *Handler) Init(host packruntime.Host) error {
	h.host = host
	return nil
}

func (h *Handler) Start(ctx context.Context) error {
	h.started.Store(true)
	if h.host != nil {
		h.host.Logger().Info("scheduler pack started", "pack", PackID)
	}
	return nil
}

func (h *Handler) Stop(ctx context.Context) error {
	h.started.Store(false)
	return nil
}

func (h *Handler) Routes() []packruntime.BackendRoute {
	specs := h.api.RouteSpecs()
	routes := make([]packruntime.BackendRoute, 0, len(specs))
	for _, spec := range specs {
		routes = append(routes, packruntime.BackendRoute{
			Method:  spec.Method,
			Path:    spec.Path,
			Handler: spec.Handler,
		})
	}
	return routes
}

func RouteSpecs() []packruntime.BackendRouteSpec {
	api := (&schedulerapi.Handler{}).RouteSpecs()
	routes := make([]packruntime.BackendRouteSpec, 0, len(api))
	for _, spec := range api {
		routes = append(routes, packruntime.BackendRouteSpec{
			Method:      spec.Method,
			Path:        spec.Path,
			Description: spec.Description,
		})
	}
	return routes
}

func Paths() []string {
	specs := RouteSpecs()
	paths := make([]string, 0, len(specs))
	for _, spec := range specs {
		paths = append(paths, spec.Path)
	}
	return paths
}
