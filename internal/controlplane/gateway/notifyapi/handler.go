package notifyapi

import (
	"encoding/json"
	"net/http"

	"yunque-agent/internal/agentcore/notify"
	"yunque-agent/internal/controlplane/gateway/gwshared"
)

// Handler serves notification channel management HTTP endpoints.
type Handler struct {
	Notifier *notify.Notifier
}

// RegisterRoutes mounts all /api/notify/* endpoints.
func (h *Handler) RegisterRoutes(mux *http.ServeMux, auth gwshared.AuthFunc) {
	mux.HandleFunc("/api/notify/channels", auth(h.handleChannels))
	mux.HandleFunc("/api/notify/add", auth(h.handleAdd))
	mux.HandleFunc("/api/notify/remove", auth(h.handleRemove))
	mux.HandleFunc("/api/notify/toggle", auth(h.handleToggle))
	mux.HandleFunc("/api/notify/test", auth(h.handleTest))
}

func (h *Handler) handleChannels(w http.ResponseWriter, r *http.Request) {
	if h.Notifier == nil {
		gwshared.WriteJSON(w, map[string]any{"channels": []any{}})
		return
	}
	channels := h.Notifier.ListChannels()
	safe := make([]map[string]any, 0, len(channels))
	for _, ch := range channels {
		safe = append(safe, map[string]any{
			"id":      ch.ID,
			"type":    ch.Type,
			"name":    ch.Name,
			"enabled": ch.Enabled,
			"url":     maskURL(ch.URL),
		})
	}
	gwshared.WriteJSON(w, map[string]any{"channels": safe})
}

func (h *Handler) handleAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		gwshared.WriteJSONStatus(w, http.StatusMethodNotAllowed, map[string]any{"error": "POST required"})
		return
	}
	if h.Notifier == nil {
		gwshared.WriteJSONStatus(w, http.StatusServiceUnavailable, map[string]any{"error": "notifier not available"})
		return
	}
	var ch notify.Channel
	if err := json.NewDecoder(r.Body).Decode(&ch); err != nil {
		gwshared.WriteJSONStatus(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if ch.ID == "" || ch.Type == "" || ch.Name == "" {
		gwshared.WriteJSONStatus(w, http.StatusBadRequest, map[string]any{"error": "id, type, and name are required"})
		return
	}
	ch.Enabled = true
	h.Notifier.AddChannel(&ch)
	gwshared.WriteJSON(w, map[string]any{"ok": true})
}

func (h *Handler) handleRemove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		gwshared.WriteJSONStatus(w, http.StatusMethodNotAllowed, map[string]any{"error": "POST required"})
		return
	}
	if h.Notifier == nil {
		gwshared.WriteJSONStatus(w, http.StatusServiceUnavailable, map[string]any{"error": "notifier not available"})
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		var body struct {
			ID string `json:"id"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		id = body.ID
	}
	if id == "" {
		gwshared.WriteJSONStatus(w, http.StatusBadRequest, map[string]any{"error": "id required"})
		return
	}
	h.Notifier.RemoveChannel(id)
	gwshared.WriteJSON(w, map[string]any{"ok": true})
}

func (h *Handler) handleToggle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		gwshared.WriteJSONStatus(w, http.StatusMethodNotAllowed, map[string]any{"error": "POST required"})
		return
	}
	if h.Notifier == nil {
		gwshared.WriteJSONStatus(w, http.StatusServiceUnavailable, map[string]any{"error": "notifier not available"})
		return
	}
	var body struct {
		ID      string `json:"id"`
		Enabled bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		gwshared.WriteJSONStatus(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	ch := h.Notifier.GetChannel(body.ID)
	if ch == nil {
		gwshared.WriteJSONStatus(w, http.StatusNotFound, map[string]any{"error": "channel not found"})
		return
	}
	ch.Enabled = body.Enabled
	h.Notifier.UpdateChannel(ch)
	gwshared.WriteJSON(w, map[string]any{"ok": true})
}

func (h *Handler) handleTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		gwshared.WriteJSONStatus(w, http.StatusMethodNotAllowed, map[string]any{"error": "POST required"})
		return
	}
	if h.Notifier == nil {
		gwshared.WriteJSONStatus(w, http.StatusServiceUnavailable, map[string]any{"error": "notifier not available"})
		return
	}
	var body struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		gwshared.WriteJSONStatus(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	ch := h.Notifier.GetChannel(body.ID)
	if ch == nil {
		gwshared.WriteJSONStatus(w, http.StatusNotFound, map[string]any{"error": "channel not found"})
		return
	}
	event := notify.Event{
		Type:    "test",
		Title:   "测试通知",
		Message: "这是一条来自云雀AI的测试通知。如果你看到这条消息，说明通知渠道配置成功。",
	}
	if err := h.Notifier.SendToChannel(r.Context(), body.ID, event); err != nil {
		gwshared.WriteJSONStatus(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	gwshared.WriteJSON(w, map[string]any{"ok": true})
}

func maskURL(u string) string {
	if len(u) <= 20 {
		return u
	}
	return u[:20] + "..."
}
