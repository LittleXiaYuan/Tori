package forkspack

import (
	"context"
	"sync/atomic"

	"yunque-agent/internal/agentcore/session"
	"yunque-agent/internal/packs/forks/forkapi"
	"yunque-agent/pkg/packruntime"
)

const PackID = "yunque.pack.forks"

// Handler exposes conversation branching as a native capability pack. Pack
// Runtime owns lifecycle, enablement and method gates, while forkapi contains
// the pack-local business HTTP implementation.
type Handler struct {
	api     *forkapi.Handler
	host    packruntime.Host
	started atomic.Bool
}

func New(tree *session.ForkTree, persister *session.ForkPersister) *Handler {
	return &Handler{api: &forkapi.Handler{ForkTree: tree, Persister: persister}}
}

func NewProvider(tree func() *session.ForkTree, persister func() *session.ForkPersister) *Handler {
	return &Handler{api: &forkapi.Handler{ForkTreeFunc: tree, PersisterFunc: persister}}
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
		h.host.Logger().Info("forks pack started", "pack", PackID)
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
		route := packruntime.BackendRoute{
			Method:  spec.Method,
			Methods: append([]string(nil), spec.Methods...),
			Path:    spec.Path,
			Handler: spec.Handler,
		}
		routes = append(routes, route)
	}
	return routes
}

func RouteSpecs() []packruntime.BackendRouteSpec {
	api := (&forkapi.Handler{}).RouteSpecs()
	routes := make([]packruntime.BackendRouteSpec, 0, len(api))
	for _, spec := range api {
		if len(spec.Methods) > 0 {
			for _, method := range spec.Methods {
				routes = append(routes, packruntime.BackendRouteSpec{
					Method:      method,
					Path:        spec.Path,
					Description: spec.Description,
				})
			}
			continue
		}
		routes = append(routes, packruntime.BackendRouteSpec{
			Method:      spec.Method,
			Path:        spec.Path,
			Description: spec.Description,
		})
	}
	return routes
}

func Paths() []string {
	seen := map[string]bool{}
	paths := []string{}
	for _, spec := range RouteSpecs() {
		if seen[spec.Path] {
			continue
		}
		seen[spec.Path] = true
		paths = append(paths, spec.Path)
	}
	return paths
}
