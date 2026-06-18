package connectorspack

import (
	"context"
	"sync/atomic"

	"yunque-agent/internal/connectors"
	"yunque-agent/internal/controlplane/gateway/connectorapi"
	"yunque-agent/pkg/packruntime"
)

const PackID = "yunque.pack.connectors"

// Handler exposes external connectors as a native capability pack. The
// connectorapi package remains the business implementation; Pack Runtime owns
// lifecycle, enablement and method gates.
type Handler struct {
	api     *connectorapi.Handler
	host    packruntime.Host
	started atomic.Bool
}

func New(registry *connectors.Registry) *Handler {
	return &Handler{api: &connectorapi.Handler{Registry: registry}}
}

func NewProvider(registry func() *connectors.Registry) *Handler {
	return &Handler{api: &connectorapi.Handler{RegistryFunc: registry}}
}

func (h *Handler) PackID() string { return PackID }

// compile-time assertion: Connectors is a v2 capability Module.
var _ packruntime.Module = (*Handler)(nil)

func (h *Handler) Init(host packruntime.Host) error {
	h.host = host
	return nil
}

func (h *Handler) Start(ctx context.Context) error {
	h.started.Store(true)
	if h.host != nil {
		h.host.Logger().Info("connectors pack started", "pack", PackID)
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
	api := (&connectorapi.Handler{}).RouteSpecs()
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
