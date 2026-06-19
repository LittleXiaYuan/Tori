package subagentspack

import (
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"

	"yunque-agent/internal/agentcore/subagent"
	"yunque-agent/internal/apperror"
	"yunque-agent/pkg/packruntime"
)

const PackID = "yunque.pack.subagents"

type Gateway interface {
	SubagentManager() *subagent.Manager
}

type Handler struct {
	managerOf func() *subagent.Manager
	host      packruntime.Host
	started   atomic.Bool
}

func New(gateway Gateway) *Handler {
	if gateway == nil {
		return NewProvider(nil)
	}
	return NewProvider(gateway.SubagentManager)
}

func NewProvider(manager func() *subagent.Manager) *Handler {
	return &Handler{managerOf: manager}
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
		h.host.Logger().Info("subagents pack started", "pack", PackID)
	}
	return nil
}

func (h *Handler) Stop(ctx context.Context) error {
	h.started.Store(false)
	return nil
}

func (h *Handler) Routes() []packruntime.BackendRoute {
	return []packruntime.BackendRoute{
		{Methods: []string{http.MethodGet, http.MethodPost, http.MethodDelete}, Path: "/v1/subagent", Handler: h.Subagent},
		{Method: http.MethodPost, Path: "/v1/subagent/message", Handler: h.Message},
	}
}

func RouteSpecs() []packruntime.BackendRouteSpec {
	return []packruntime.BackendRouteSpec{
		{Method: http.MethodGet, Path: "/v1/subagent", Description: "List subagents or read one subagent by id."},
		{Method: http.MethodPost, Path: "/v1/subagent", Description: "Spawn a subagent with optional skills."},
		{Method: http.MethodDelete, Path: "/v1/subagent", Description: "Destroy a subagent by id."},
		{Method: http.MethodPost, Path: "/v1/subagent/message", Description: "Append messages to a subagent context."},
	}
}

func Paths() []string {
	return []string{"/v1/subagent", "/v1/subagent/message"}
}

func (h *Handler) manager() *subagent.Manager {
	if h.managerOf == nil {
		return nil
	}
	return h.managerOf()
}

func (h *Handler) Subagent(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	manager := h.manager()
	if manager == nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "subagent manager not configured"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		id := r.URL.Query().Get("id")
		if id != "" {
			sa, ok := manager.Get(id)
			if !ok {
				apperror.WriteCode(w, apperror.CodeBadRequest, "subagent not found")
				return
			}
			_ = json.NewEncoder(w).Encode(sa)
		} else {
			parentID := r.URL.Query().Get("parent_id")
			_ = json.NewEncoder(w).Encode(map[string]any{"subagents": manager.List(parentID)})
		}
	case http.MethodPost:
		var req struct {
			ParentID    string   `json:"parent_id"`
			Name        string   `json:"name"`
			Description string   `json:"description"`
			Skills      []string `json:"skills"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, "invalid request")
			return
		}
		sa, err := manager.Spawn(req.ParentID, req.Name, req.Description, req.Skills)
		if err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, err.Error())
			return
		}
		_ = json.NewEncoder(w).Encode(sa)
	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			apperror.WriteCode(w, apperror.CodeBadRequest, "id required")
			return
		}
		ok := manager.Destroy(id)
		_ = json.NewEncoder(w).Encode(map[string]bool{"deleted": ok})
	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "method not allowed")
	}
}

func (h *Handler) Message(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	manager := h.manager()
	if manager == nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "subagent manager not configured"})
		return
	}
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST required")
		return
	}
	var req struct {
		ID       string           `json:"id"`
		Messages []map[string]any `json:"messages"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid request")
		return
	}
	if err := manager.AppendMessages(req.ID, req.Messages); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, err.Error())
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}
