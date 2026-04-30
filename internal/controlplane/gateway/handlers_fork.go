package gateway

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"yunque-agent/internal/agentcore/session"
	"yunque-agent/internal/apperror"
	"yunque-agent/pkg/safego"
)

func (g *Gateway) persistForkTree() {
	if g.forkPersister != nil && g.forkTree != nil {
		safego.Go("fork-tree-persist", func() {
			if err := g.forkPersister.Save(g.forkTree); err != nil {
				slog.Error("fork tree persist failed", "err", err)
			}
		})
	}
}

func (g *Gateway) handleFork(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.forkTree == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "fork tree not configured"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		forkID := r.URL.Query().Get("id")
		if forkID != "" {
			f, ok := g.forkTree.Get(forkID)
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
			root, ok := g.forkTree.GetRoot(sessionID)
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
		f := g.forkTree.Create(req.SessionID, req.Messages)
		g.persistForkTree()
		json.NewEncoder(w).Encode(f)
	case http.MethodDelete:
		forkID := r.URL.Query().Get("id")
		if forkID == "" {
			apperror.WriteCode(w, apperror.CodeBadRequest, "id required")
			return
		}
		ok := g.forkTree.Delete(forkID)
		g.persistForkTree()
		json.NewEncoder(w).Encode(map[string]bool{"deleted": ok})
	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "method not allowed")
	}
}

func (g *Gateway) handleForkBranch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.forkTree == nil {
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
	branch, err := g.forkTree.Branch(req.ForkID, req.AtIndex, req.Label)
	if err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, err.Error())
		return
	}
	g.persistForkTree()
	json.NewEncoder(w).Encode(branch)
}

func (g *Gateway) handleForkList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.forkTree == nil {
		json.NewEncoder(w).Encode(map[string]any{"forks": []any{}})
		return
	}
	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "session_id required")
		return
	}
	forks := g.forkTree.ListBranches(sessionID)
	json.NewEncoder(w).Encode(map[string]any{"forks": forks})
}
