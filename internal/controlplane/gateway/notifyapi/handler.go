package notifyapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"yunque-agent/internal/agentcore/notify"
	"yunque-agent/internal/controlplane/gateway/gwshared"
)

// Route declares one notification HTTP route.
type Route struct {
	Method      string
	Path        string
	Description string
	Handler     http.HandlerFunc
}

// Handler serves notification channel management HTTP endpoints.
type Handler struct {
	Notifier     *notify.Notifier
	NotifierFunc func() *notify.Notifier
}

func (h *Handler) notifier() *notify.Notifier {
	if h.NotifierFunc != nil {
		return h.NotifierFunc()
	}
	return h.Notifier
}

// RouteSpecs returns the notification surface without mounting it. Pack Runtime
// uses this to own route registration while preserving the existing handler
// implementation.
func (h *Handler) RouteSpecs() []Route {
	return []Route{
		{Method: http.MethodGet, Path: "/api/notify/channels", Description: "List configured notification channels.", Handler: h.handleChannels},
		{Method: http.MethodPost, Path: "/api/notify/add", Description: "Add a notification channel.", Handler: h.handleAdd},
		{Method: http.MethodPost, Path: "/api/notify/remove", Description: "Remove a notification channel.", Handler: h.handleRemove},
		{Method: http.MethodPost, Path: "/api/notify/toggle", Description: "Enable or disable a notification channel.", Handler: h.handleToggle},
		{Method: http.MethodPost, Path: "/api/notify/test", Description: "Send a test notification to one channel.", Handler: h.handleTest},
		{Method: http.MethodPost, Path: "/api/notify/share", Description: "Share a task/session result through one notification channel.", Handler: h.handleShare},
	}
}

// RegisterRoutes mounts all /api/notify/* endpoints.
func (h *Handler) RegisterRoutes(mux *http.ServeMux, auth gwshared.AuthFunc) {
	for _, route := range h.RouteSpecs() {
		mux.HandleFunc(route.Path, auth(route.Handler))
	}
}

func (h *Handler) handleChannels(w http.ResponseWriter, r *http.Request) {
	notifier := h.notifier()
	if notifier == nil {
		gwshared.WriteJSON(w, map[string]any{"channels": []any{}})
		return
	}
	channels := notifier.ListChannels()
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
	notifier := h.notifier()
	if notifier == nil {
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
	notifier.AddChannel(&ch)
	gwshared.WriteJSON(w, map[string]any{"ok": true})
}

func (h *Handler) handleRemove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		gwshared.WriteJSONStatus(w, http.StatusMethodNotAllowed, map[string]any{"error": "POST required"})
		return
	}
	notifier := h.notifier()
	if notifier == nil {
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
	notifier.RemoveChannel(id)
	gwshared.WriteJSON(w, map[string]any{"ok": true})
}

func (h *Handler) handleToggle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		gwshared.WriteJSONStatus(w, http.StatusMethodNotAllowed, map[string]any{"error": "POST required"})
		return
	}
	notifier := h.notifier()
	if notifier == nil {
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
	ch := notifier.GetChannel(body.ID)
	if ch == nil {
		gwshared.WriteJSONStatus(w, http.StatusNotFound, map[string]any{"error": "channel not found"})
		return
	}
	ch.Enabled = body.Enabled
	notifier.UpdateChannel(ch)
	gwshared.WriteJSON(w, map[string]any{"ok": true})
}

func (h *Handler) handleTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		gwshared.WriteJSONStatus(w, http.StatusMethodNotAllowed, map[string]any{"error": "POST required"})
		return
	}
	notifier := h.notifier()
	if notifier == nil {
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
	ch := notifier.GetChannel(body.ID)
	if ch == nil {
		gwshared.WriteJSONStatus(w, http.StatusNotFound, map[string]any{"error": "channel not found"})
		return
	}
	event := notify.Event{
		Type:    "test",
		Title:   "测试通知",
		Message: "这是一条来自云雀AI的测试通知。如果你看到这条消息，说明通知渠道配置成功。",
	}
	if err := notifier.SendToChannel(r.Context(), body.ID, event); err != nil {
		gwshared.WriteJSONStatus(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	gwshared.WriteJSON(w, map[string]any{"ok": true})
}

func (h *Handler) handleShare(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		gwshared.WriteJSONStatus(w, http.StatusMethodNotAllowed, map[string]any{"error": "POST required"})
		return
	}
	notifier := h.notifier()
	if notifier == nil {
		gwshared.WriteJSONStatus(w, http.StatusServiceUnavailable, map[string]any{"error": "notifier not available"})
		return
	}
	var body struct {
		ChannelID string `json:"channel_id"`
		Title     string `json:"title"`
		Message   string `json:"message"`
		SessionID string `json:"session_id"`
		TaskID    string `json:"task_id"`
		URL       string `json:"url"`
		Files     []struct {
			Name string `json:"name"`
			Path string `json:"path"`
			Size int64  `json:"size"`
		} `json:"files"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		gwshared.WriteJSONStatus(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	body.ChannelID = strings.TrimSpace(body.ChannelID)
	body.Title = strings.TrimSpace(body.Title)
	body.Message = strings.TrimSpace(body.Message)
	body.SessionID = strings.TrimSpace(body.SessionID)
	body.TaskID = strings.TrimSpace(body.TaskID)
	if body.SessionID == "" {
		body.SessionID = body.TaskID
	}
	if body.ChannelID == "" {
		gwshared.WriteJSONStatus(w, http.StatusBadRequest, map[string]any{"error": "channel_id required"})
		return
	}
	if body.Title == "" {
		body.Title = "云雀协作同步"
	}
	if body.Message == "" && len(body.Files) == 0 {
		gwshared.WriteJSONStatus(w, http.StatusBadRequest, map[string]any{"error": "message or files required"})
		return
	}
	ch := notifier.GetChannel(body.ChannelID)
	if ch == nil {
		gwshared.WriteJSONStatus(w, http.StatusNotFound, map[string]any{"error": "channel not found"})
		return
	}

	binding, err := notifier.CreateShareBinding(ch, body.SessionID, body.TaskID, body.Title)
	if err != nil {
		gwshared.WriteJSONStatus(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	message := formatShareMessage(body.Message, body.URL, body.Files, binding.Code)
	event := notify.Event{
		Type:    "chat_share",
		Title:   body.Title,
		Message: message,
		TaskID:  body.TaskID,
		URL:     body.URL,
	}
	if err := notifier.SendToChannel(r.Context(), body.ChannelID, event); err != nil {
		gwshared.WriteJSONStatus(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	gwshared.WriteJSON(w, map[string]any{
		"ok":      true,
		"sent_at": time.Now().Format(time.RFC3339),
		"share": map[string]any{
			"code":       binding.Code,
			"session_id": binding.SessionID,
			"created_at": binding.CreatedAt.Format(time.RFC3339),
		},
		"channel": map[string]any{
			"id":   ch.ID,
			"type": ch.Type,
			"name": ch.Name,
		},
	})
}

func formatShareMessage(message string, url string, files []struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Size int64  `json:"size"`
}, shareCode string) string {
	var b strings.Builder
	if message != "" {
		b.WriteString(message)
	}
	if len(files) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString("产物文件：")
		for _, f := range files {
			name := strings.TrimSpace(f.Name)
			if name == "" {
				name = strings.TrimSpace(f.Path)
			}
			if name == "" {
				continue
			}
			b.WriteString("\n- ")
			b.WriteString(name)
			if f.Size > 0 {
				b.WriteString(fmt.Sprintf(" (%s)", formatShareSize(f.Size)))
			}
			if f.Path != "" {
				b.WriteString("\n  ")
				b.WriteString(f.Path)
			}
		}
	}
	if url != "" {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString("打开云雀任务：")
		b.WriteString(url)
	}
	if shareCode != "" {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString("协作码：")
		b.WriteString(shareCode)
		b.WriteString("\n在 IM 中回复：/yq ")
		b.WriteString(shareCode)
		b.WriteString(" 你的问题")
	}
	out := strings.TrimSpace(b.String())
	if len([]rune(out)) > 12000 {
		runes := []rune(out)
		out = string(runes[:12000]) + "\n\n...已截断"
	}
	return out
}

func formatShareSize(size int64) string {
	if size >= 1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(size)/1024/1024)
	}
	if size >= 1024 {
		return fmt.Sprintf("%.1f KB", float64(size)/1024)
	}
	return fmt.Sprintf("%d B", size)
}

func maskURL(u string) string {
	if len(u) <= 20 {
		return u
	}
	return u[:20] + "..."
}
