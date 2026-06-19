package identitypack

import (
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"

	"yunque-agent/internal/agentcore/identity"
	"yunque-agent/internal/apperror"
	"yunque-agent/pkg/packruntime"
)

const PackID = "yunque.pack.identity"

type Gateway interface {
	IdentityResolver() *identity.Resolver
}

type Handler struct {
	resolverOf func() *identity.Resolver
	host       packruntime.Host
	started    atomic.Bool
}

func New(gateway Gateway) *Handler {
	if gateway == nil {
		return NewProvider(nil)
	}
	return NewProvider(gateway.IdentityResolver)
}

func NewProvider(resolver func() *identity.Resolver) *Handler {
	return &Handler{resolverOf: resolver}
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
		h.host.Logger().Info("identity pack started", "pack", PackID)
	}
	return nil
}

func (h *Handler) Stop(ctx context.Context) error {
	h.started.Store(false)
	return nil
}

func (h *Handler) Routes() []packruntime.BackendRoute {
	return []packruntime.BackendRoute{
		{Method: http.MethodPost, Path: "/v1/identity/resolve", Handler: h.Resolve},
		{Method: http.MethodGet, Path: "/v1/identity/profiles", Handler: h.Profiles},
	}
}

func RouteSpecs() []packruntime.BackendRouteSpec {
	return []packruntime.BackendRouteSpec{
		{Method: http.MethodPost, Path: "/v1/identity/resolve", Description: "Resolve or create a unified identity profile for a channel user."},
		{Method: http.MethodGet, Path: "/v1/identity/profiles", Description: "List unified identity profiles known to the resolver."},
	}
}

func Paths() []string {
	return []string{"/v1/identity/resolve", "/v1/identity/profiles"}
}

func (h *Handler) resolver() *identity.Resolver {
	if h.resolverOf == nil {
		return nil
	}
	return h.resolverOf()
}

func (h *Handler) Resolve(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST required")
		return
	}
	resolver := h.resolver()
	if resolver == nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "identity resolver not configured"})
		return
	}
	var req struct {
		Channel     string `json:"channel"`
		UserID      string `json:"user_id"`
		DisplayName string `json:"display_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid request")
		return
	}
	profile := resolver.Resolve(req.Channel, req.UserID, req.DisplayName)
	_ = json.NewEncoder(w).Encode(profile)
}

func (h *Handler) Profiles(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET required")
		return
	}
	resolver := h.resolver()
	if resolver == nil {
		_ = json.NewEncoder(w).Encode(map[string]any{"profiles": []any{}})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"profiles": resolver.All()})
}
