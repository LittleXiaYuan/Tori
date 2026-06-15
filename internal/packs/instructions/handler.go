// Package instructionspack mounts the user-instruction surface
// (/v1/instructions, /v1/instructions/reorder) as a v2 capability pack (Tier 0
// microkernel). It is a native pack: the CRUD + reorder logic lives here and
// talks to the instruction store through a narrow host accessor — the gateway
// no longer hosts these routes.
package instructionspack

import (
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"

	"yunque-agent/internal/agentcore/instruction"
	"yunque-agent/internal/apperror"
	"yunque-agent/internal/controlplane/gateway/gwshared"
	"yunque-agent/pkg/packruntime"
)

// PackID is the stable manifest id.
const PackID = "yunque.pack.instructions"

// Gateway is the narrow host surface the instructions pack needs: a handle to
// the instruction store, resolved per request so registration order does not
// matter.
type Gateway interface {
	InstructionStore() *instruction.Store
}

// Handler is the instructions pack backend module.
type Handler struct {
	gw      Gateway
	host    packruntime.Host
	started atomic.Bool
}

// New builds the instructions pack backed by the host's instruction store.
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
		h.host.Logger().Info("instructions pack started", "pack", PackID)
	}
	return nil
}

// Stop marks the pack stopped on disable.
func (h *Handler) Stop(ctx context.Context) error {
	h.started.Store(false)
	return nil
}

// Routes mounts the user-instruction surface natively.
func (h *Handler) Routes() []packruntime.BackendRoute {
	return []packruntime.BackendRoute{
		{
			Methods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete},
			Path:    "/v1/instructions",
			Handler: h.handleInstructions,
		},
		{
			Methods: []string{http.MethodPost},
			Path:    "/v1/instructions/reorder",
			Handler: h.handleReorder,
		},
	}
}

func (h *Handler) store() *instruction.Store {
	if h.gw == nil {
		return nil
	}
	return h.gw.InstructionStore()
}

func (h *Handler) handleInstructions(w http.ResponseWriter, r *http.Request) {
	store := h.store()
	if store == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "instruction store not configured")
		return
	}
	ctx := r.Context()
	tenantID := gwshared.TenantFromCtx(ctx)

	switch r.Method {
	case http.MethodGet:
		list, err := store.List(ctx, tenantID)
		if err != nil {
			apperror.WriteCode(w, apperror.CodeInternal, err.Error())
			return
		}
		if category := r.URL.Query().Get("category"); category != "" {
			filtered := list[:0]
			for _, inst := range list {
				if inst.Category == category {
					filtered = append(filtered, inst)
				}
			}
			list = filtered
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"instructions": list,
			"total":        len(list),
		})

	case http.MethodPost:
		var req instruction.UserInstruction
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON body")
			return
		}
		created, err := store.Create(ctx, tenantID, req)
		if err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(created)

	case http.MethodPut:
		var req instruction.UserInstruction
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON body")
			return
		}
		if req.ID == "" {
			apperror.WriteCode(w, apperror.CodeBadRequest, "instruction_id is required")
			return
		}
		if err := store.Update(ctx, tenantID, req); err != nil {
			apperror.WriteCode(w, apperror.CodeNotFound, err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "updated"})

	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			apperror.WriteCode(w, apperror.CodeBadRequest, "id query parameter required")
			return
		}
		if err := store.Delete(ctx, tenantID, id); err != nil {
			apperror.WriteCode(w, apperror.CodeNotFound, err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleReorder(w http.ResponseWriter, r *http.Request) {
	store := h.store()
	if store == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "instruction store not configured")
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()
	tenantID := gwshared.TenantFromCtx(ctx)

	var req struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON body")
		return
	}
	if len(req.IDs) == 0 {
		apperror.WriteCode(w, apperror.CodeBadRequest, "ids array cannot be empty")
		return
	}
	if err := store.Reorder(ctx, tenantID, req.IDs); err != nil {
		apperror.WriteCode(w, apperror.CodeInternal, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "reordered"})
}
