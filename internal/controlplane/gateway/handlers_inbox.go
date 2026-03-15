package gateway

import (
	"encoding/json"
	"net/http"

	"yunque-agent/internal/apperror"
)

func (g *Gateway) handleInbox(w http.ResponseWriter, r *http.Request) {
	if g.inbox == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "inbox not configured")
		return
	}
	switch r.Method {
	case http.MethodGet:
		unreadOnly := r.URL.Query().Get("unread") == "true"
		items := g.inbox.List(unreadOnly, 50)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"items": items,
			"count": g.inbox.Count(),
		})
	case http.MethodPost:
		var req struct {
			Source  string         `json:"source"`
			Content string         `json:"content"`
			Action  string         `json:"action"`
			Header  map[string]any `json:"header"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Content == "" {
			apperror.WriteCode(w, apperror.CodeBadRequest, "content is required")
			return
		}
		item, err := g.inbox.Push(req.Source, req.Content, req.Action, req.Header)
		if err != nil {
			apperror.Write(w, apperror.Wrap(apperror.CodeInternal, "push failed", err))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(item)
	case http.MethodDelete:
		var req struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
			apperror.WriteCode(w, apperror.CodeBadRequest, "id is required")
			return
		}
		g.inbox.Delete(req.ID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET, POST, or DELETE")
	}
}

func (g *Gateway) handleInboxRead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	if g.inbox == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "inbox not configured")
		return
	}
	var req struct {
		IDs []string `json:"ids"`
		All bool     `json:"all"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	count := 0
	if req.All {
		count = g.inbox.MarkAllRead()
	} else if len(req.IDs) > 0 {
		count = g.inbox.MarkRead(req.IDs)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"marked": count})
}
