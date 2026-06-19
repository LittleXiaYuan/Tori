// Package desktoppack mounts local desktop shell controls as a native
// capability pack.
package desktoppack

import (
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"

	"yunque-agent/internal/desktop"
	"yunque-agent/pkg/packruntime"
)

const PackID = "yunque.pack.desktop"

type Controller interface {
	ToggleConsole() bool
	IsConsoleHidden() bool
	SetAutoStart(bool) error
	IsAutoStartEnabled() bool
}

type nativeController struct{}

func (nativeController) ToggleConsole() bool       { return desktop.ToggleConsole() }
func (nativeController) IsConsoleHidden() bool     { return desktop.IsConsoleHidden() }
func (nativeController) SetAutoStart(v bool) error { return desktop.SetAutoStart(v) }
func (nativeController) IsAutoStartEnabled() bool  { return desktop.IsAutoStartEnabled() }

type Handler struct {
	controller Controller
	host       packruntime.Host
	started    atomic.Bool
}

func New() *Handler {
	return NewWithController(nativeController{})
}

func NewWithController(controller Controller) *Handler {
	return &Handler{controller: controller}
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
		h.host.Logger().Info("desktop pack started", "pack", PackID)
	}
	return nil
}

func (h *Handler) Stop(ctx context.Context) error {
	h.started.Store(false)
	return nil
}

func (h *Handler) Routes() []packruntime.BackendRoute {
	return []packruntime.BackendRoute{
		{Methods: []string{http.MethodGet, http.MethodPost}, Path: "/v1/desktop/console", Handler: h.Console},
		{Methods: []string{http.MethodGet, http.MethodPost}, Path: "/v1/desktop/autostart", Handler: h.AutoStart},
	}
}

func RouteSpecs() []packruntime.BackendRouteSpec {
	return []packruntime.BackendRouteSpec{
		{Method: http.MethodGet, Path: "/v1/desktop/console", Description: "Read local console visibility state."},
		{Method: http.MethodPost, Path: "/v1/desktop/console", Description: "Toggle the local console window visibility."},
		{Method: http.MethodGet, Path: "/v1/desktop/autostart", Description: "Read desktop auto-start state."},
		{Method: http.MethodPost, Path: "/v1/desktop/autostart", Description: "Toggle desktop auto-start on login."},
	}
}

func Paths() []string {
	return []string{"/v1/desktop/console", "/v1/desktop/autostart"}
}

func (h *Handler) Console(w http.ResponseWriter, r *http.Request) {
	ctrl := h.controller
	if ctrl == nil {
		writeJSON(w, map[string]any{"console_hidden": false})
		return
	}
	switch r.Method {
	case http.MethodPost:
		writeJSON(w, map[string]any{"console_hidden": ctrl.ToggleConsole()})
	default:
		writeJSON(w, map[string]any{"console_hidden": ctrl.IsConsoleHidden()})
	}
}

func (h *Handler) AutoStart(w http.ResponseWriter, r *http.Request) {
	ctrl := h.controller
	if ctrl == nil {
		writeJSON(w, map[string]any{"autostart_enabled": false})
		return
	}
	switch r.Method {
	case http.MethodPost:
		enabled := !ctrl.IsAutoStartEnabled()
		if err := ctrl.SetAutoStart(enabled); err != nil {
			writeJSON(w, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, map[string]any{"autostart_enabled": enabled})
	default:
		writeJSON(w, map[string]any{"autostart_enabled": ctrl.IsAutoStartEnabled()})
	}
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}
