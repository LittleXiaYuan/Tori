package forkapi

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"yunque-agent/internal/agentcore/session"
	"yunque-agent/internal/apperror"
	"yunque-agent/pkg/safego"
)

// Route declares one conversation fork HTTP route.
type Route struct {
	Method      string
	Methods     []string
	Path        string
	Description string
	Handler     http.HandlerFunc
}

// Handler serves conversation fork/branch HTTP endpoints.
type Handler struct {
	ForkTree      *session.ForkTree
	Persister     *session.ForkPersister
	ForkTreeFunc  func() *session.ForkTree
	PersisterFunc func() *session.ForkPersister
}

// RouteSpecs returns the fork surface without mounting it. Pack Runtime uses
// this to own route registration.
func (h *Handler) RouteSpecs() []Route {
	return []Route{
		{Methods: []string{http.MethodGet, http.MethodPost, http.MethodDelete}, Path: "/v1/fork", Description: "Read, create or delete a conversation fork.", Handler: h.handleFork},
		{Method: http.MethodPost, Path: "/v1/fork/branch", Description: "Create a branch from an existing conversation fork.", Handler: h.handleBranch},
		{Method: http.MethodGet, Path: "/v1/fork/list", Description: "List conversation forks for a session.", Handler: h.handleList},
	}
}

func (h *Handler) forkTree() *session.ForkTree {
	if h.ForkTreeFunc != nil {
		return h.ForkTreeFunc()
	}
	return h.ForkTree
}

func (h *Handler) persister() *session.ForkPersister {
	if h.PersisterFunc != nil {
		return h.PersisterFunc()
	}
	return h.Persister
}

func (h *Handler) persist() {
	persister := h.persister()
	tree := h.forkTree()
	if persister != nil && tree != nil {
		safego.Go("fork-tree-persist", func() {
			if err := persister.Save(tree); err != nil {
				slog.Error("fork tree persist failed", "err", err)
			}
		})
	}
}

func (h *Handler) handleFork(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	tree := h.forkTree()
	if tree == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "fork tree not configured"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		forkID := r.URL.Query().Get("id")
		if forkID != "" {
			f, ok := tree.Get(forkID)
			if !ok {
				apperror.WriteCode(w, apperror.CodeBadRequest, "fork not found")
				return
			}
			json.NewEncoder(w).Encode(f)
		} else {
			sessionID := r.URL.Query().Get("session_id")
			if sessionID == "" {
				apperror.WriteCode(w, apperror.CodeBadRequest, "session_id or id required")
				return
			}
			root, ok := tree.GetRoot(sessionID)
			if !ok {
				json.NewEncoder(w).Encode(map[string]any{"fork": nil})
				return
			}
			json.NewEncoder(w).Encode(root)
		}
	case http.MethodPost:
		var req struct {
			SessionID string                `json:"session_id"`
			Messages  []session.ForkMessage `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, "invalid request")
			return
		}
		f := tree.Create(req.SessionID, req.Messages)
		h.persist()
		json.NewEncoder(w).Encode(f)
	case http.MethodDelete:
		forkID := r.URL.Query().Get("id")
		if forkID == "" {
			apperror.WriteCode(w, apperror.CodeBadRequest, "id required")
			return
		}
		ok := tree.Delete(forkID)
		h.persist()
		json.NewEncoder(w).Encode(map[string]bool{"deleted": ok})
	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "method not allowed")
	}
}

func (h *Handler) handleBranch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	tree := h.forkTree()
	if tree == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "fork tree not configured"})
		return
	}
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST required")
		return
	}
	var req struct {
		ForkID  string `json:"fork_id"`
		AtIndex int    `json:"at_index"`
		Label   string `json:"label"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid request")
		return
	}
	branch, err := tree.Branch(req.ForkID, req.AtIndex, req.Label)
	if err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, err.Error())
		return
	}
	h.persist()
	json.NewEncoder(w).Encode(branch)
}

func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	tree := h.forkTree()
	if tree == nil {
		json.NewEncoder(w).Encode(map[string]any{"forks": []any{}})
		return
	}
	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "session_id required")
		return
	}
	forks := tree.ListBranches(sessionID)
	json.NewEncoder(w).Encode(map[string]any{"forks": forks})
}
