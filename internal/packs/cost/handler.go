package costpack

import (
	"context"
	"sync/atomic"

	"yunque-agent/internal/agentcore/costtrack"
	"yunque-agent/internal/controlplane/gateway/costapi"
	"yunque-agent/pkg/packruntime"
)

const PackID = "yunque.pack.cost"

// Handler exposes cost governance as a native capability pack. The existing
// costapi implementation remains the business logic; Pack Runtime owns
// lifecycle, enablement and method gates.
type Handler struct {
	api     *costapi.Handler
	host    packruntime.Host
	started atomic.Bool
}

func New(tracker *costtrack.Tracker) *Handler {
	return &Handler{api: &costapi.Handler{Tracker: tracker}}
}

func NewProvider(tracker func() *costtrack.Tracker) *Handler {
	return &Handler{api: &costapi.Handler{TrackerFunc: tracker}}
}

func (h *Handler) PackID() string { return PackID }

// compile-time assertion: Cost is a v2 capability Module (Tier 0 microkernel).
var _ packruntime.Module = (*Handler)(nil)

func (h *Handler) Init(host packruntime.Host) error {
	h.host = host
	return nil
}

func (h *Handler) Start(ctx context.Context) error {
	h.started.Store(true)
	if h.host != nil {
		h.host.Logger().Info("cost pack started", "pack", PackID)
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
	api := (&costapi.Handler{}).RouteSpecs()
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

func MethodsByPath() map[string]string {
	specs := RouteSpecs()
	out := make(map[string]string, len(specs))
	for _, spec := range specs {
		out[spec.Path] = spec.Method
	}
	return out
}
