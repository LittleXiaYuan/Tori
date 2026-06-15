// Package modespack mounts the persona-mode HTTP surface (/v1/persona/mode*) as
// a v2 capability pack (Tier 0 microkernel). It is a native pack: the handler
// logic lives here and talks to the mode manager through a narrow accessor —
// the gateway no longer hosts these routes.
package modespack

import (
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"

	"yunque-agent/internal/agentcore/modes"
	"yunque-agent/internal/apperror"
	"yunque-agent/pkg/packruntime"
)

// PackID is the stable manifest id.
const PackID = "yunque.pack.persona-modes"

// Gateway is the narrow host surface the modes pack needs: a handle to the mode
// manager, resolved per request so registration order does not matter.
type Gateway interface {
	ModeManager() *modes.ModeManager
}

// Handler is the persona-modes pack backend module.
type Handler struct {
	gw      Gateway
	host    packruntime.Host
	started atomic.Bool
}

// New builds the persona-modes pack backed by the host's mode manager accessor.
func New(gw Gateway) *Handler { return &Handler{gw: gw} }

// compile-time assertion: this is a valid v2 Module.
var _ packruntime.Module = (*Handler)(nil)

func (h *Handler) PackID() string { return PackID }

// Init wires the pack against the kernel Host.
func (h *Handler) Init(host packruntime.Host) error {
	h.host = host
	return nil
}

// Start marks the pack live on enable.
func (h *Handler) Start(ctx context.Context) error {
	h.started.Store(true)
	if h.host != nil {
		h.host.Logger().Info("persona-modes pack started", "pack", PackID)
	}
	return nil
}

// Stop marks the pack stopped on disable.
func (h *Handler) Stop(ctx context.Context) error {
	h.started.Store(false)
	return nil
}

// Routes mounts the persona-mode surface natively.
func (h *Handler) Routes() []packruntime.BackendRoute {
	methods := []string{http.MethodGet, http.MethodPost}
	return []packruntime.BackendRoute{
		{Methods: methods, Path: "/v1/persona/modes", Handler: h.handleList},
		{Methods: methods, Path: "/v1/persona/mode", Handler: h.handleSet},
		{Methods: methods, Path: "/v1/persona/mode/current", Handler: h.handleCurrent},
	}
}

func (h *Handler) mgr() *modes.ModeManager {
	if h.gw == nil {
		return nil
	}
	return h.gw.ModeManager()
}

// handleList returns all available persona modes (GET).
func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	mgr := h.mgr()
	if mgr == nil {
		http.Error(w, `{"error":"mode system not configured"}`, http.StatusNotFound)
		return
	}
	tenantID := r.URL.Query().Get("tenant_id")
	sessionID := r.URL.Query().Get("session_id")
	modeList := mgr.ListModes(r.Context(), tenantID, sessionID)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"modes": modeList,
		"total": len(modeList),
	})
}

// handleSet switches the persona mode for a tenant (POST).
func (h *Handler) handleSet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	mgr := h.mgr()
	if mgr == nil {
		http.Error(w, `{"error":"mode system not configured"}`, http.StatusNotFound)
		return
	}
	var req struct {
		TenantID  string            `json:"tenant_id"`
		Mode      modes.PersonaMode `json:"mode"`
		SessionID string            `json:"session_id,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if !req.Mode.Valid() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error":       "invalid mode",
			"valid_modes": modes.AllModes,
		})
		return
	}
	if err := mgr.SetMode(r.Context(), req.TenantID, req.Mode, req.SessionID); err != nil {
		apperror.Write(w, apperror.Wrap(apperror.CodeInternal, "set mode failed", err))
		return
	}
	modeList := mgr.ListModes(r.Context(), req.TenantID, req.SessionID)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success":      true,
		"current_mode": req.Mode,
		"modes":        modeList,
	})
}

// handleCurrent returns the current active mode for a tenant (GET).
func (h *Handler) handleCurrent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	mgr := h.mgr()
	if mgr == nil {
		http.Error(w, `{"error":"mode system not configured"}`, http.StatusNotFound)
		return
	}
	tenantID := r.URL.Query().Get("tenant_id")
	sessionID := r.URL.Query().Get("session_id")
	current := mgr.CurrentMode(r.Context(), tenantID, sessionID)
	preset := modes.ModePresets[current]
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"mode":        current,
		"name":        preset.Name,
		"name_en":     preset.NameEN,
		"description": preset.Description,
		"features":    preset.Features,
	})
}
