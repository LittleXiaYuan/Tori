package gateway

import (
	"encoding/json"
	"net/http"

	"yunque-agent/internal/agentcore/notify"
)

func (g *Gateway) handleNotifyChannels(w http.ResponseWriter, r *http.Request) {
	if g.notifier == nil {
		writeJSON(w, map[string]any{"channels": []any{}})
		return
	}
	channels := g.notifier.ListChannels()
	// Mask secrets in response
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
	writeJSON(w, map[string]any{"channels": safe})
}

func (g *Gateway) handleNotifyAddChannel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONStatus(w, http.StatusMethodNotAllowed, map[string]any{"error": "POST required"})
		return
	}
	if g.notifier == nil {
		writeJSONStatus(w, http.StatusServiceUnavailable, map[string]any{"error": "notifier not available"})
		return
	}
	var ch notify.Channel
	if err := json.NewDecoder(r.Body).Decode(&ch); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if ch.ID == "" || ch.Type == "" || ch.Name == "" {
		writeJSONStatus(w, http.StatusBadRequest, map[string]any{"error": "id, type, and name are required"})
		return
	}
	ch.Enabled = true
	g.notifier.AddChannel(&ch)
	writeJSON(w, map[string]any{"ok": true})
}

func (g *Gateway) handleNotifyRemoveChannel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONStatus(w, http.StatusMethodNotAllowed, map[string]any{"error": "POST required"})
		return
	}
	if g.notifier == nil {
		writeJSONStatus(w, http.StatusServiceUnavailable, map[string]any{"error": "notifier not available"})
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
		writeJSONStatus(w, http.StatusBadRequest, map[string]any{"error": "id required"})
		return
	}
	g.notifier.RemoveChannel(id)
	writeJSON(w, map[string]any{"ok": true})
}

func (g *Gateway) handleNotifyToggle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONStatus(w, http.StatusMethodNotAllowed, map[string]any{"error": "POST required"})
		return
	}
	if g.notifier == nil {
		writeJSONStatus(w, http.StatusServiceUnavailable, map[string]any{"error": "notifier not available"})
		return
	}
	var body struct {
		ID      string `json:"id"`
		Enabled bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	ch := g.notifier.GetChannel(body.ID)
	if ch == nil {
		writeJSONStatus(w, http.StatusNotFound, map[string]any{"error": "channel not found"})
		return
	}
	ch.Enabled = body.Enabled
	g.notifier.UpdateChannel(ch)
	writeJSON(w, map[string]any{"ok": true})
}

func (g *Gateway) handleNotifyTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONStatus(w, http.StatusMethodNotAllowed, map[string]any{"error": "POST required"})
		return
	}
	if g.notifier == nil {
		writeJSONStatus(w, http.StatusServiceUnavailable, map[string]any{"error": "notifier not available"})
		return
	}
	var body struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	ch := g.notifier.GetChannel(body.ID)
	if ch == nil {
		writeJSONStatus(w, http.StatusNotFound, map[string]any{"error": "channel not found"})
		return
	}
	event := notify.Event{
		Type:    "test",
		Title:   "测试通知",
		Message: "这是一条来自云雀AI的测试通知。如果你看到这条消息，说明通知渠道配置成功。",
	}
	if err := g.notifier.SendToChannel(r.Context(), body.ID, event); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func maskURL(u string) string {
	if len(u) <= 20 {
		return u
	}
	return u[:20] + "..."
}
