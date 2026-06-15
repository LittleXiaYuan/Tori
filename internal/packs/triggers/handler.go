// Package triggerspack mounts the trigger HTTP surface (/v1/triggers and
// /v1/triggers/v2*) as a v2 capability pack (Tier 0 microkernel). Native pack:
// it owns the legacy (Runtime) and unified (Manager) trigger handlers, reached
// through narrow accessors. Split out of the gateway's mixed triggers grab-bag.
package triggerspack

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"yunque-agent/internal/agentcore/trigger"
	"yunque-agent/internal/apperror"
	"yunque-agent/pkg/packruntime"
)

// PackID is the stable manifest id.
const PackID = "yunque.pack.triggers"

// Gateway is the narrow host surface the triggers pack needs.
type Gateway interface {
	TriggerRuntime() *trigger.Runtime
	TriggerManager() *trigger.Manager
}

// Handler is the triggers pack backend module.
type Handler struct {
	gw      Gateway
	host    packruntime.Host
	started atomic.Bool
}

// New builds the triggers pack backed by the host accessors.
func New(gw Gateway) *Handler { return &Handler{gw: gw} }

var _ packruntime.Module = (*Handler)(nil)

func (h *Handler) PackID() string { return PackID }

func (h *Handler) Init(host packruntime.Host) error {
	h.host = host
	return nil
}

func (h *Handler) Start(ctx context.Context) error {
	h.started.Store(true)
	if h.host != nil {
		h.host.Logger().Info("triggers pack started", "pack", PackID)
	}
	return nil
}

func (h *Handler) Stop(ctx context.Context) error {
	h.started.Store(false)
	return nil
}

func (h *Handler) rt() *trigger.Runtime {
	if h.gw == nil {
		return nil
	}
	return h.gw.TriggerRuntime()
}

func (h *Handler) mgr() *trigger.Manager {
	if h.gw == nil {
		return nil
	}
	return h.gw.TriggerManager()
}

// Routes mounts the trigger surface natively.
func (h *Handler) Routes() []packruntime.BackendRoute {
	m := []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete}
	mk := func(path string, fn http.HandlerFunc) packruntime.BackendRoute {
		return packruntime.BackendRoute{Methods: m, Path: path, Handler: fn}
	}
	return []packruntime.BackendRoute{
		mk("/v1/triggers", h.handleTriggers),
		mk("/v1/triggers/emit", h.handleTriggerEmit),
		mk("/v1/triggers/v2", h.handleV2),
		mk("/v1/triggers/v2/emit", h.handleV2Emit),
		mk("/v1/triggers/v2/runs", h.handleV2Runs),
		mk("/v1/triggers/v2/events", h.handleV2Events),
	}
}

// handleTriggers manages legacy trigger CRUD + emit.
func (h *Handler) handleTriggers(w http.ResponseWriter, r *http.Request) {
	rt := h.rt()
	if rt == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "trigger runtime not available")
		return
	}
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		id := r.URL.Query().Get("id")
		if id != "" {
			t, ok := rt.Get(id)
			if !ok {
				apperror.WriteCode(w, apperror.CodeNotFound, "trigger not found")
				return
			}
			_ = json.NewEncoder(w).Encode(t)
			return
		}
		triggers := rt.List()
		_ = json.NewEncoder(w).Encode(map[string]any{"triggers": triggers, "total": len(triggers)})

	case http.MethodPost:
		var t trigger.Trigger
		if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON")
			return
		}
		if t.Name == "" {
			apperror.WriteCode(w, apperror.CodeBadRequest, "name is required")
			return
		}
		id := rt.Register(t)
		t.ID = id
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(t)

	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			apperror.WriteCode(w, apperror.CodeBadRequest, "id is required")
			return
		}
		if !rt.Remove(id) {
			apperror.WriteCode(w, apperror.CodeNotFound, "trigger not found")
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"deleted": id})

	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET/POST/DELETE only")
	}
}

// handleTriggerEmit emits a system event to the legacy trigger runtime.
func (h *Handler) handleTriggerEmit(w http.ResponseWriter, r *http.Request) {
	rt := h.rt()
	if rt == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "trigger runtime not available")
		return
	}
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	var payload trigger.EventPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON")
		return
	}
	if payload.Event == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "event is required")
		return
	}
	rt.Emit(r.Context(), payload)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "emitted", "event": string(payload.Event)})
}

// handleV2 is the unified trigger CRUD surface (TriggerDef).
func (h *Handler) handleV2(w http.ResponseWriter, r *http.Request) {
	mgr := h.mgr()
	if mgr == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "trigger manager not available")
		return
	}
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		id := r.URL.Query().Get("id")
		if id != "" {
			t, ok := mgr.Get(id)
			if !ok {
				apperror.WriteCode(w, apperror.CodeNotFound, "trigger not found")
				return
			}
			_ = json.NewEncoder(w).Encode(t)
			return
		}
		tenantID := r.URL.Query().Get("tenant_id")
		triggerType := r.URL.Query().Get("type")
		status := r.URL.Query().Get("status")
		triggers := mgr.List(tenantID, func(t *trigger.TriggerDef) bool {
			if triggerType != "" && string(t.Type) != triggerType {
				return false
			}
			if status != "" && string(t.Status) != status {
				return false
			}
			return true
		})
		_ = json.NewEncoder(w).Encode(map[string]any{"triggers": triggers, "total": len(triggers)})

	case http.MethodPost:
		var t trigger.TriggerDef
		if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON: "+err.Error())
			return
		}
		if t.Name == "" {
			apperror.WriteCode(w, apperror.CodeBadRequest, "name is required")
			return
		}
		if t.TenantID == "" {
			t.TenantID = "default"
		}
		if len(t.Actions) == 0 {
			apperror.WriteCode(w, apperror.CodeBadRequest, "at least one action is required")
			return
		}
		if err := mgr.Create(&t); err != nil {
			apperror.WriteCode(w, apperror.CodeInternal, err.Error())
			return
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(t)

	case http.MethodPut:
		var t trigger.TriggerDef
		if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON: "+err.Error())
			return
		}
		if t.ID == "" {
			apperror.WriteCode(w, apperror.CodeBadRequest, "id is required")
			return
		}
		if err := mgr.Update(&t); err != nil {
			apperror.WriteCode(w, apperror.CodeNotFound, err.Error())
			return
		}
		_ = json.NewEncoder(w).Encode(t)

	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			apperror.WriteCode(w, apperror.CodeBadRequest, "id is required")
			return
		}
		if err := mgr.Delete(id); err != nil {
			apperror.WriteCode(w, apperror.CodeNotFound, err.Error())
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"deleted": id})

	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET/POST/PUT/DELETE only")
	}
}

// handleV2Emit emits an event to the unified trigger manager.
func (h *Handler) handleV2Emit(w http.ResponseWriter, r *http.Request) {
	mgr := h.mgr()
	if mgr == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "trigger manager not available")
		return
	}
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	var payload trigger.EventPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if payload.Event == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "event is required")
		return
	}
	if payload.Timestamp.IsZero() {
		payload.Timestamp = time.Now()
	}
	mgr.Emit(r.Context(), payload)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "emitted", "event": string(payload.Event)})
}

// handleV2Runs lists recent runs for a trigger.
func (h *Handler) handleV2Runs(w http.ResponseWriter, r *http.Request) {
	mgr := h.mgr()
	if mgr == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "trigger manager not available")
		return
	}
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET only")
		return
	}
	triggerID := r.URL.Query().Get("trigger_id")
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	runs := mgr.GetRuns(triggerID, limit)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"runs": runs, "total": len(runs)})
}

// handleV2Events lists recent events for a trigger.
func (h *Handler) handleV2Events(w http.ResponseWriter, r *http.Request) {
	mgr := h.mgr()
	if mgr == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "trigger manager not available")
		return
	}
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET only")
		return
	}
	triggerID := r.URL.Query().Get("trigger_id")
	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	events := mgr.GetEvents(triggerID, limit)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"events": events, "total": len(events)})
}
