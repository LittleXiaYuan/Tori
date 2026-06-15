// Package statepack mounts the state-kernel HTTP surface (/v1/state*) as a v2
// capability pack (Tier 0 microkernel). Native pack: it owns the structured
// state snapshot/goals/focus/resources, reaching the kernel through a narrow
// accessor. Split out of the misnamed registerTaskRoutes grab-bag.
package statepack

import (
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"

	"yunque-agent/internal/agentcore/state"
	"yunque-agent/internal/apperror"
	"yunque-agent/pkg/packruntime"
)

// PackID is the stable manifest id.
const PackID = "yunque.pack.state"

// Gateway is the narrow host surface the state pack needs.
type Gateway interface {
	StateKernel() *state.Kernel
}

// Handler is the state pack backend module.
type Handler struct {
	gw      Gateway
	host    packruntime.Host
	started atomic.Bool
}

// New builds the state pack backed by the host's state-kernel accessor.
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
		h.host.Logger().Info("state pack started", "pack", PackID)
	}
	return nil
}

func (h *Handler) Stop(ctx context.Context) error {
	h.started.Store(false)
	return nil
}

// Routes mounts the state surface natively.
func (h *Handler) Routes() []packruntime.BackendRoute {
	m := []string{http.MethodGet, http.MethodPost, http.MethodDelete}
	mk := func(p string, fn http.HandlerFunc) packruntime.BackendRoute {
		return packruntime.BackendRoute{Methods: m, Path: p, Handler: fn}
	}
	return []packruntime.BackendRoute{
		mk("/v1/state", h.handleSnapshot),
		mk("/v1/state/goals", h.handleGoals),
		mk("/v1/state/focus", h.handleFocus),
		mk("/v1/state/resources", h.handleResources),
	}
}

func (h *Handler) kernel(w http.ResponseWriter) (*state.Kernel, bool) {
	var sk *state.Kernel
	if h.gw != nil {
		sk = h.gw.StateKernel()
	}
	if sk == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "state kernel not initialized")
		return nil, false
	}
	return sk, true
}

func (h *Handler) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	sk, ok := h.kernel(w)
	if !ok {
		return
	}
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeBadRequest, "method not allowed")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(sk.TakeSnapshot())
}

func (h *Handler) handleGoals(w http.ResponseWriter, r *http.Request) {
	sk, ok := h.kernel(w)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(sk.Goals())

	case http.MethodPost:
		var req struct {
			ID          string   `json:"id"`
			Title       string   `json:"title"`
			Description string   `json:"description"`
			Priority    int      `json:"priority"`
			Status      string   `json:"status"`
			Progress    float64  `json:"progress"`
			ParentGoal  string   `json:"parent_goal"`
			TaskIDs     []string `json:"task_ids"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON")
			return
		}
		if req.Title == "" {
			apperror.WriteCode(w, apperror.CodeBadRequest, "title required")
			return
		}
		if req.ID != "" {
			if sk.UpdateGoal(req.ID, func(g *state.Goal) {
				if req.Title != "" {
					g.Title = req.Title
				}
				if req.Description != "" {
					g.Description = req.Description
				}
				if req.Priority > 0 {
					g.Priority = req.Priority
				}
				if req.Status != "" {
					g.Status = req.Status
				}
				if req.Progress > 0 {
					g.Progress = req.Progress
				}
			}) {
				sk.Save()
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]string{"id": req.ID, "status": "updated"})
				return
			}
		}
		id := sk.AddGoal(state.Goal{
			ID:          req.ID,
			Title:       req.Title,
			Description: req.Description,
			Priority:    req.Priority,
			ParentGoal:  req.ParentGoal,
			TaskIDs:     req.TaskIDs,
		})
		sk.Save()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"id": id, "status": "created"})

	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			apperror.WriteCode(w, apperror.CodeBadRequest, "id required")
			return
		}
		if !sk.RemoveGoal(id) {
			apperror.WriteCode(w, apperror.CodeNotFound, "goal not found")
			return
		}
		sk.Save()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})

	default:
		apperror.WriteCode(w, apperror.CodeBadRequest, "method not allowed")
	}
}

func (h *Handler) handleFocus(w http.ResponseWriter, r *http.Request) {
	sk, ok := h.kernel(w)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"focus": sk.Focus()})

	case http.MethodPost:
		var req struct {
			Focus  string   `json:"focus"`
			Topics []string `json:"topics"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON")
			return
		}
		if req.Focus != "" {
			sk.SetFocus(req.Focus)
		}
		for _, t := range req.Topics {
			sk.AddTopic(t)
		}
		sk.Save()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

	default:
		apperror.WriteCode(w, apperror.CodeBadRequest, "method not allowed")
	}
}

func (h *Handler) handleResources(w http.ResponseWriter, r *http.Request) {
	sk, ok := h.kernel(w)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(sk.Resources())

	case http.MethodPost:
		var req state.Resource
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON")
			return
		}
		if req.Path == "" {
			apperror.WriteCode(w, apperror.CodeBadRequest, "path required")
			return
		}
		sk.TrackResource(req)
		sk.Save()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "tracked"})

	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			apperror.WriteCode(w, apperror.CodeBadRequest, "id required")
			return
		}
		if !sk.ReleaseResource(id) {
			apperror.WriteCode(w, apperror.CodeNotFound, "resource not found")
			return
		}
		sk.Save()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "released"})

	default:
		apperror.WriteCode(w, apperror.CodeBadRequest, "method not allowed")
	}
}
