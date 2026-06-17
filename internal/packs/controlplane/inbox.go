package controlplanepack

import (
	"encoding/json"
	"net/http"

	"yunque-agent/internal/apperror"
)

func (h *Handler) handleInbox(w http.ResponseWriter, r *http.Request) {
	store := h.gateway.InboxStore()
	if store == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "inbox not configured")
		return
	}
	switch r.Method {
	case http.MethodGet:
		unreadOnly := r.URL.Query().Get("unread") == "true"
		items := store.List(unreadOnly, 50)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": items,
			"count": store.Count(),
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
		item, err := store.Push(req.Source, req.Content, req.Action, req.Header)
		if err != nil {
			apperror.Write(w, apperror.Wrap(apperror.CodeInternal, "push failed", err))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(item)
	case http.MethodDelete:
		var req struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
			apperror.WriteCode(w, apperror.CodeBadRequest, "id is required")
			return
		}
		store.Delete(req.ID)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET, POST, or DELETE")
	}
}

func (h *Handler) handleInboxRead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	store := h.gateway.InboxStore()
	if store == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "inbox not configured")
		return
	}
	var req struct {
		IDs []string `json:"ids"`
		All bool     `json:"all"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	count := 0
	if req.All {
		count = store.MarkAllRead()
	} else if len(req.IDs) > 0 {
		count = store.MarkRead(req.IDs)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]int{"marked": count})
}
