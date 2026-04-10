package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"yunque-agent/internal/appdir"
)

// Channel represents a configured notification channel.
type Channel struct {
	ID      string `json:"id"`
	Type    string `json:"type"` // "webhook", "dingtalk", "feishu", "wechat_work", "email_smtp"
	Name    string `json:"name"`
	URL     string `json:"url,omitempty"`
	Secret  string `json:"secret,omitempty"` // signing secret for DingTalk, etc.
	Enabled bool   `json:"enabled"`

	// Email-specific fields
	SMTPHost string `json:"smtp_host,omitempty"`
	SMTPPort int    `json:"smtp_port,omitempty"`
	SMTPUser string `json:"smtp_user,omitempty"`
	SMTPPass string `json:"smtp_pass,omitempty"`
	EmailTo  string `json:"email_to,omitempty"`
}

// Event represents a notification event.
type Event struct {
	Type    string `json:"type"` // "task_complete", "task_failed", "research_done", "approval_needed"
	Title   string `json:"title"`
	Message string `json:"message"`
	TaskID  string `json:"task_id,omitempty"`
	URL     string `json:"url,omitempty"` // link back to Yunque UI
}

// Notifier manages notification channels and dispatches events.
type Notifier struct {
	mu       sync.RWMutex
	channels map[string]*Channel
	client   *http.Client
	store    string
}

// New creates a new Notifier.
func New() *Notifier {
	n := &Notifier{
		channels: make(map[string]*Channel),
		client:   &http.Client{Timeout: 10 * time.Second},
		store:    filepath.Join(appdir.DataDir(), "notify_channels.json"),
	}
	n.load()
	return n
}

// AddChannel registers a notification channel.
func (n *Notifier) AddChannel(ch *Channel) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.channels[ch.ID] = ch
	n.saveLocked()
}

// RemoveChannel unregisters a notification channel.
func (n *Notifier) RemoveChannel(id string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	delete(n.channels, id)
	n.saveLocked()
}

// ListChannels returns all channels.
func (n *Notifier) ListChannels() []*Channel {
	n.mu.RLock()
	defer n.mu.RUnlock()
	out := make([]*Channel, 0, len(n.channels))
	for _, ch := range n.channels {
		out = append(out, ch)
	}
	return out
}

// GetChannel returns a channel by ID.
func (n *Notifier) GetChannel(id string) *Channel {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.channels[id]
}

// UpdateChannel replaces an existing channel configuration.
func (n *Notifier) UpdateChannel(ch *Channel) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.channels[ch.ID] = ch
	n.saveLocked()
}

// Send dispatches an event to all enabled channels.
func (n *Notifier) Send(ctx context.Context, event Event) {
	n.mu.RLock()
	channels := make([]*Channel, 0, len(n.channels))
	for _, ch := range n.channels {
		if ch.Enabled {
			channels = append(channels, ch)
		}
	}
	n.mu.RUnlock()

	for _, ch := range channels {
		go func(ch *Channel) {
			if err := n.sendToChannel(ctx, ch, event); err != nil {
				slog.Warn("notification send failed", "channel", ch.ID, "type", ch.Type, "err", err)
			} else {
				slog.Info("notification sent", "channel", ch.ID, "event_type", event.Type)
			}
		}(ch)
	}
}

// SendToChannel dispatches an event to a single enabled channel.
func (n *Notifier) SendToChannel(ctx context.Context, id string, event Event) error {
	ch := n.GetChannel(id)
	if ch == nil {
		return fmt.Errorf("channel not found: %s", id)
	}
	if !ch.Enabled {
		return fmt.Errorf("channel disabled: %s", id)
	}
	return n.sendToChannel(ctx, ch, event)
}

func (n *Notifier) sendToChannel(ctx context.Context, ch *Channel, event Event) error {
	switch ch.Type {
	case "webhook":
		return n.sendWebhook(ctx, ch, event)
	case "dingtalk":
		return n.sendDingTalk(ctx, ch, event)
	case "feishu":
		return n.sendFeishu(ctx, ch, event)
	case "wechat_work":
		return n.sendWechatWork(ctx, ch, event)
	default:
		return fmt.Errorf("unsupported channel type: %s", ch.Type)
	}
}

func (n *Notifier) sendWebhook(ctx context.Context, ch *Channel, event Event) error {
	body, _ := json.Marshal(event)
	req, err := http.NewRequestWithContext(ctx, "POST", ch.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "YunqueAgent/1.0")

	resp, err := n.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned %d", resp.StatusCode)
	}
	return nil
}

func (n *Notifier) sendDingTalk(ctx context.Context, ch *Channel, event Event) error {
	payload := map[string]any{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"title": fmt.Sprintf("[云雀] %s", event.Title),
			"text": fmt.Sprintf("## %s\n\n%s\n\n> %s",
				event.Title, event.Message, formatTime()),
		},
	}
	return n.postJSON(ctx, ch.URL, payload)
}

func (n *Notifier) sendFeishu(ctx context.Context, ch *Channel, event Event) error {
	payload := map[string]any{
		"msg_type": "interactive",
		"card": map[string]any{
			"header": map[string]any{
				"title": map[string]any{
					"tag":     "plain_text",
					"content": fmt.Sprintf("[云雀] %s", event.Title),
				},
				"template": eventColor(event.Type),
			},
			"elements": []map[string]any{
				{
					"tag": "markdown",
					"content": fmt.Sprintf("%s\n\n时间: %s",
						event.Message, formatTime()),
				},
			},
		},
	}
	return n.postJSON(ctx, ch.URL, payload)
}

func (n *Notifier) sendWechatWork(ctx context.Context, ch *Channel, event Event) error {
	payload := map[string]any{
		"msgtype": "markdown",
		"markdown": map[string]any{
			"content": fmt.Sprintf("## <font color=\"info\">[云雀] %s</font>\n\n%s\n\n> 时间: %s",
				event.Title, event.Message, formatTime()),
		},
	}
	return n.postJSON(ctx, ch.URL, payload)
}

func (n *Notifier) postJSON(ctx context.Context, url string, payload any) error {
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

func formatTime() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

func eventColor(eventType string) string {
	if strings.Contains(eventType, "fail") || strings.Contains(eventType, "error") {
		return "red"
	}
	if strings.Contains(eventType, "approval") || strings.Contains(eventType, "warning") {
		return "orange"
	}
	return "green"
}

func (n *Notifier) load() {
	data, err := os.ReadFile(n.store)
	if err != nil {
		return
	}
	var channels []*Channel
	if err := json.Unmarshal(data, &channels); err != nil {
		slog.Warn("notify: failed to load channels", "err", err)
		return
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	for _, ch := range channels {
		if ch != nil && ch.ID != "" {
			n.channels[ch.ID] = ch
		}
	}
}

func (n *Notifier) saveLocked() {
	if err := os.MkdirAll(filepath.Dir(n.store), 0o755); err != nil {
		slog.Warn("notify: failed to create store dir", "err", err)
		return
	}
	channels := make([]*Channel, 0, len(n.channels))
	for _, ch := range n.channels {
		copyCh := *ch
		channels = append(channels, &copyCh)
	}
	data, err := json.MarshalIndent(channels, "", "  ")
	if err != nil {
		slog.Warn("notify: failed to encode channels", "err", err)
		return
	}
	if err := os.WriteFile(n.store, data, 0o644); err != nil {
		slog.Warn("notify: failed to save channels", "err", err)
	}
}
