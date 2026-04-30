package forkapi

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"yunque-agent/internal/agentcore/session"
	"yunque-agent/internal/apperror"
	"yunque-agent/internal/controlplane/gateway/gwshared"
	"yunque-agent/pkg/safego"
)

// Handler serves conversation fork/branch HTTP endpoints.
type Handler struct {
	ForkTree  *session.ForkTree
	Persister *session.ForkPersister
}

// RegisterRoutes mounts all /v1/fork/* endpoints.
func (h *Handler) RegisterRoutes(mux *http.ServeMux, auth gwshared.AuthFunc) {
	mux.HandleFunc("/v1/fork", auth(h.handleFork))
	mux.HandleFunc("/v1/fork/branch", auth(h.handleBranch))
	mux.HandleFunc("/v1/fork/list", auth(h.handleList))
}

func (h *Handler) persist() {
	if h.Persister != nil && h.ForkTree != nil {
		safego.Go("fork-tree-persist", func() {
			if err := h.Persister.Save(h.ForkTree); err != nil {
				slog.Error("fork tree persist failed", "err", err)
			}
		})
	}
}

func (h *Handler) handleFork(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if h.ForkTree == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "fork tree not configured"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		forkID := r.URL.Query().Get("id")
		if forkID != "" {
			f, ok := h.ForkTree.Get(forkID)
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
			root, ok := h.ForkTree.GetRoot(sessionID)
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
		f := h.ForkTree.Create(req.SessionID, req.Messages)
		h.persist()
		json.NewEncoder(w).Encode(f)
	case http.MethodDelete:
		forkID := r.URL.Query().Get("id")
		if forkID == "" {
			apperror.WriteCode(w, apperror.CodeBadRequest, "id required")
			return
		}
		ok := h.ForkTree.Delete(forkID)
		h.persist()
		json.NewEncoder(w).Encode(map[string]bool{"deleted": ok})
	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "method not allowed")
	}
}

func (h *Handler) handleBranch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if h.ForkTree == nil {
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
	branch, err := h.ForkTree.Branch(req.ForkID, req.AtIndex, req.Label)
	if err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, err.Error())
		return
	}
	h.persist()
	json.NewEncoder(w).Encode(branch)
}

func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if h.ForkTree == nil {
		json.NewEncoder(w).Encode(map[string]any{"forks": []any{}})
		return
	}
	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "session_id required")
		return
	}
	forks := h.ForkTree.ListBranches(sessionID)
	json.NewEncoder(w).Encode(map[string]any{"forks": forks})
}
