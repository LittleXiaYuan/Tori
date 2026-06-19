// Package cognitivelayerpack mounts the master cognitive-layer switch as a
// native capability pack. It keeps the original admin-only semantics while Pack
// Runtime owns route enablement and method gates.
package cognitivelayerpack

import (
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"

	"yunque-agent/internal/agentcore/planner"
	"yunque-agent/pkg/packruntime"
)

const PackID = "yunque.pack.cognitive-layer"

const Route = "/v1/cognitive-layer"

type Gateway interface {
	RequireAuth(http.HandlerFunc) http.HandlerFunc
	RequireAdmin(http.HandlerFunc) http.HandlerFunc
}

type Handler struct {
	authOf  func(http.HandlerFunc) http.HandlerFunc
	adminOf func(http.HandlerFunc) http.HandlerFunc
	host    packruntime.Host
	started atomic.Bool
}

func New(gateway Gateway) *Handler {
	if gateway == nil {
		return NewProvider(nil, nil)
	}
	return NewProvider(gateway.RequireAuth, gateway.RequireAdmin)
}

func NewProvider(authOf func(http.HandlerFunc) http.HandlerFunc, adminOf func(http.HandlerFunc) http.HandlerFunc) *Handler {
	return &Handler{authOf: authOf, adminOf: adminOf}
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
		h.host.Logger().Info("cognitive-layer pack started", "pack", PackID)
	}
	return nil
}

func (h *Handler) Stop(ctx context.Context) error {
	h.started.Store(false)
	return nil
}

func (h *Handler) Routes() []packruntime.BackendRoute {
	return []packruntime.BackendRoute{
		{
			Methods: []string{http.MethodGet, http.MethodPost},
			Path:    Route,
			Handler: h.admin(h.handleSwitch),
			Auth:    packruntime.BackendRouteAuthPassthrough,
		},
	}
}

func Paths() []string { return []string{"/v1/cognitive-layer"} }

func RouteSpecs() []packruntime.BackendRouteSpec {
	return []packruntime.BackendRouteSpec{
		{Method: http.MethodGet, Path: "/v1/cognitive-layer", Description: "Read whether the cognitive layer is currently enabled."},
		{Method: http.MethodPost, Path: "/v1/cognitive-layer", Description: "Hot-toggle the whole cognitive layer at runtime."},
	}
}

func (h *Handler) admin(next http.HandlerFunc) http.HandlerFunc {
	if h.authOf == nil && h.adminOf == nil {
		return next
	}
	wrapped := next
	if h.adminOf != nil {
		wrapped = h.adminOf(wrapped)
	}
	if h.authOf != nil {
		wrapped = h.authOf(wrapped)
	}
	return wrapped
}

func (h *Handler) handleSwitch(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeState(w)
	case http.MethodPost:
		var body struct {
			Enabled *bool `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Enabled == nil {
			writeJSONStatus(w, http.StatusBadRequest, map[string]any{
				"error": "body must be {\"enabled\": true|false}",
			})
			return
		}
		planner.SetCognitiveLayerEnabled(*body.Enabled)
		writeState(w)
	default:
		writeJSONStatus(w, http.StatusMethodNotAllowed, map[string]any{"error": "GET or POST only"})
	}
}

func writeState(w http.ResponseWriter) {
	writeJSON(w, map[string]any{"enabled": planner.CognitiveLayerEnabled()})
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

func writeJSONStatus(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
