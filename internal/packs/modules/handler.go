package modulespack

import (
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"

	agentrt "yunque-agent/internal/agentcore/runtime"
	"yunque-agent/internal/apperror"
	"yunque-agent/pkg/packruntime"
)

const PackID = "yunque.pack.modules"

type Gateway interface {
	ModuleRegistry() *agentrt.ModuleRegistry
	ModuleProfile() string
}

type Handler struct {
	modulesOf func() *agentrt.ModuleRegistry
	profileOf func() string
	host      packruntime.Host
	started   atomic.Bool
}

func New(gateway Gateway) *Handler {
	if gateway == nil {
		return NewProvider(nil, nil)
	}
	return NewProvider(gateway.ModuleRegistry, gateway.ModuleProfile)
}

func NewProvider(modules func() *agentrt.ModuleRegistry, profile func() string) *Handler {
	return &Handler{modulesOf: modules, profileOf: profile}
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
		h.host.Logger().Info("modules pack started", "pack", PackID)
	}
	return nil
}

func (h *Handler) Stop(ctx context.Context) error {
	h.started.Store(false)
	return nil
}

func (h *Handler) Routes() []packruntime.BackendRoute {
	return []packruntime.BackendRoute{
		{Method: http.MethodGet, Path: "/v1/modules", Handler: h.Modules},
	}
}

func RouteSpecs() []packruntime.BackendRouteSpec {
	return []packruntime.BackendRouteSpec{
		{Method: http.MethodGet, Path: "/v1/modules", Description: "List registered runtime modules and the current profile."},
	}
}

func Paths() []string {
	return []string{"/v1/modules"}
}

func (h *Handler) modules() *agentrt.ModuleRegistry {
	if h.modulesOf == nil {
		return nil
	}
	return h.modulesOf()
}

func (h *Handler) profile() string {
	if h.profileOf == nil {
		return ""
	}
	return h.profileOf()
}

func (h *Handler) Modules(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET required")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	registry := h.modules()
	if registry == nil {
		_ = json.NewEncoder(w).Encode(map[string]any{"modules": []any{}, "profile": ""})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"modules": registry.List(),
		"profile": h.profile(),
	})
}
