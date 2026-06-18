package notificationspack

import (
	"context"
	"sync/atomic"

	"yunque-agent/internal/agentcore/notify"
	"yunque-agent/internal/controlplane/gateway/notifyapi"
	"yunque-agent/pkg/packruntime"
)

const PackID = "yunque.pack.notifications"

// Handler exposes notification channels and sharing as a native capability pack.
// notifyapi remains the business implementation; Pack Runtime owns lifecycle,
// enablement and method gates.
type Handler struct {
	api     *notifyapi.Handler
	host    packruntime.Host
	started atomic.Bool
}

func New(notifier *notify.Notifier) *Handler {
	return &Handler{api: &notifyapi.Handler{Notifier: notifier}}
}

func NewProvider(notifier func() *notify.Notifier) *Handler {
	return &Handler{api: &notifyapi.Handler{NotifierFunc: notifier}}
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
		h.host.Logger().Info("notifications pack started", "pack", PackID)
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
	api := (&notifyapi.Handler{}).RouteSpecs()
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
