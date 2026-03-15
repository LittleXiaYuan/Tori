package channel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// ──────────────────────────────────────────────
// Satori Protocol Adapter
// Satori is a universal IM protocol specification (https://satori.js.org)
// used by Koishi and other frameworks to bridge multiple messaging platforms
// through a unified HTTP + WebSocket API.
// ──────────────────────────────────────────────

const (
	satoriMaxTextLen  = 4096
	satoriDefaultPort = "9884"
)

// Satori implements the Channel interface for the Satori universal protocol.
// It acts as a Satori client, connecting to a Satori server that bridges
// to the actual IM platform (QQ, Telegram, Discord, etc.).
type Satori struct {
	endpoint string // Satori server endpoint, e.g. "http://localhost:5140"
	token    string // Authentication token
	port     string // Local webhook server port (for receiving events)
	bindAddr string // Webhook bind address
	platform string // Platform filter (optional, e.g. "onebot", "telegram")
	selfID   string // Bot's self ID (to filter own messages)
	client   *http.Client
	msgCh    chan Message
}

// SatoriConfig holds configuration for the Satori adapter.
type SatoriConfig struct {
	Endpoint string `json:"endpoint"` // Satori server URL
	Token    string `json:"token"`    // Auth token
	Port     string `json:"port"`     // Local webhook port
	BindAddr string `json:"bind_addr,omitempty"`
	Platform string `json:"platform,omitempty"` // Optional platform filter
	SelfID   string `json:"self_id,omitempty"`  // Bot self ID
}

// NewSatori creates a Satori protocol adapter.
func NewSatori(cfg SatoriConfig) *Satori {
	port := cfg.Port
	if port == "" {
		port = satoriDefaultPort
	}
	bindAddr := cfg.BindAddr
	if bindAddr == "" {
		bindAddr = "0.0.0.0"
	}
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = "http://localhost:5140"
	}
	return &Satori{
		endpoint: endpoint,
		token:    cfg.Token,
		port:     port,
		bindAddr: bindAddr,
		platform: cfg.Platform,
		selfID:   cfg.SelfID,
		client:   &http.Client{Timeout: 15 * time.Second},
		msgCh:    make(chan Message, 100),
	}
}

func (s *Satori) Type() string { return "satori" }

// ──────────────────────────────────────────────
// Start — listen for Satori events via webhook
// ──────────────────────────────────────────────

func (s *Satori) Start(ctx context.Context, handler func(Message) Reply) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/satori/events", s.eventHandler())

	addr := s.bindAddr + ":" + s.port
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}

	var wg sync.WaitGroup

	// Message processing goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-s.msgCh:
				reply := handler(msg)
				channelID := msg.ChannelID
				platform := ""
				if msg.Extra != nil {
					platform = msg.Extra["platform"]
				}
				_ = s.sendMessage(ctx, platform, channelID, reply.Content)
			}
		}
	}()

	slog.Info("satori channel starting", "addr", addr, "endpoint", s.endpoint)
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()

	err := srv.ListenAndServe()
	wg.Wait()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

// Send pushes a message to a Satori channel.
func (s *Satori) Send(ctx context.Context, target string, reply Reply) error {
	return s.sendMessage(ctx, s.platform, target, reply.Content)
}

// ──────────────────────────────────────────────
// Event webhook handler
// ──────────────────────────────────────────────

func (s *Satori) eventHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Verify token if configured
		if s.token != "" {
			auth := r.Header.Get("Authorization")
			expected := "Bearer " + s.token
			if auth != expected {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}

		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			http.Error(w, "read body failed", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		// Satori event can be a single event or array
		var events []satoriEvent
		if len(body) > 0 && body[0] == '[' {
			if err := json.Unmarshal(body, &events); err != nil {
				http.Error(w, "invalid json", http.StatusBadRequest)
				return
			}
		} else {
			var single satoriEvent
			if err := json.Unmarshal(body, &single); err != nil {
				http.Error(w, "invalid json", http.StatusBadRequest)
				return
			}
			events = append(events, single)
		}

		for _, event := range events {
			s.processEvent(event)
		}

		w.WriteHeader(http.StatusOK)
	}
}

// processEvent handles a single Satori event.
func (s *Satori) processEvent(event satoriEvent) {
	// Filter by platform if configured
	if s.platform != "" && event.Platform != "" && event.Platform != s.platform {
		return
	}

	// Skip self messages
	if s.selfID != "" && event.User.ID == s.selfID {
		return
	}

	switch event.Type {
	case "message-created":
		content := s.extractContent(event)
		if content == "" {
			return
		}

		channelID := event.Channel.ID
		if channelID == "" {
			channelID = event.User.ID
		}

		msg := Message{
			ChannelType: "satori",
			ChannelID:   channelID,
			UserID:      event.User.ID,
			UserName:    event.User.Name,
			Content:     content,
			Extra: map[string]string{
				"platform":   event.Platform,
				"message_id": event.Message.ID,
				"guild_id":   event.Guild.ID,
				"self_id":    event.SelfID,
			},
		}
		select {
		case s.msgCh <- msg:
		default:
			slog.Warn("satori: message channel full, dropping message")
		}

	case "guild-member-added":
		slog.Info("satori: member added", "platform", event.Platform, "user", event.User.ID)
	case "guild-member-removed":
		slog.Info("satori: member removed", "platform", event.Platform, "user", event.User.ID)
	}
}

// extractContent extracts text from Satori event message elements.
func (s *Satori) extractContent(event satoriEvent) string {
	// Satori messages use an element-based format.
	// The `content` field contains Satori message elements (XML-like or plain text).
	content := event.Message.Content

	// If message has elements, build text from them
	if len(event.Message.Elements) > 0 {
		var text string
		for _, elem := range event.Message.Elements {
			switch elem.Type {
			case "text":
				text += elem.Attrs.Content
			case "at":
				text += "@" + elem.Attrs.Name
			case "img", "image":
				text += "[图片]"
			case "audio":
				text += "[音频]"
			case "video":
				text += "[视频]"
			case "file":
				text += "[文件]"
			case "face":
				text += "[表情]"
			case "quote":
				// skip quote elements
			}
		}
		if text != "" {
			return text
		}
	}

	return content
}

// ──────────────────────────────────────────────
// Satori API — message.create
// ──────────────────────────────────────────────

func (s *Satori) sendMessage(ctx context.Context, platform, channelID, content string) error {
	parts := splitSatoriMessage(content)
	for _, part := range parts {
		payload := satoriMessageCreateReq{
			ChannelID: channelID,
			Content:   part,
		}
		if err := s.callAPI(ctx, platform, "message.create", payload); err != nil {
			return err
		}
	}
	return nil
}

func (s *Satori) callAPI(ctx context.Context, platform, action string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	// Satori API: POST {endpoint}/{version}/{action}
	url := s.endpoint + "/v1/" + action
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if s.token != "" {
		req.Header.Set("Authorization", "Bearer "+s.token)
	}
	if platform != "" {
		req.Header.Set("X-Platform", platform)
	}
	if s.selfID != "" {
		req.Header.Set("X-Self-ID", s.selfID)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("satori api %s: %w", action, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("satori api %s: status %d: %s", action, resp.StatusCode, string(body))
	}
	return nil
}

// ──────────────────────────────────────────────
// Satori Protocol Types
// See: https://satori.js.org/zh-CN/protocol/
// ──────────────────────────────────────────────

type satoriEvent struct {
	ID        int64      `json:"id"`
	Type      string     `json:"type"` // "message-created", "guild-member-added", etc.
	Platform  string     `json:"platform"`
	SelfID    string     `json:"self_id"`
	Timestamp int64      `json:"timestamp"`
	Channel   satoriObj  `json:"channel"`
	Guild     satoriObj  `json:"guild"`
	User      satoriUser `json:"user"`
	Message   satoriMsg  `json:"message"`
}

type satoriObj struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

type satoriUser struct {
	ID     string `json:"id"`
	Name   string `json:"name,omitempty"`
	Avatar string `json:"avatar,omitempty"`
}

type satoriMsg struct {
	ID       string          `json:"id"`
	Content  string          `json:"content"`
	Elements []satoriElement `json:"elements,omitempty"`
}

type satoriElement struct {
	Type  string      `json:"type"` // "text", "at", "img", "audio", "video", "file", "face", "quote"
	Attrs satoriAttrs `json:"attrs,omitempty"`
}

type satoriAttrs struct {
	Content string `json:"content,omitempty"` // for text elements
	ID      string `json:"id,omitempty"`      // for at/quote elements
	Name    string `json:"name,omitempty"`    // for at elements
	Src     string `json:"src,omitempty"`     // for img/audio/video/file elements
}

type satoriMessageCreateReq struct {
	ChannelID string `json:"channel_id"`
	Content   string `json:"content"`
}

// splitSatoriMessage splits a long text into Satori-compatible chunks.
func splitSatoriMessage(text string) []string {
	return SplitMessage(text, satoriMaxTextLen)
}

// Ensure Satori implements Channel
var (
	_ Channel     = (*Satori)(nil)
	_ GroupLister = (*Satori)(nil)
)

// ListGroups returns all guilds available via the Satori guild.list API.
func (s *Satori) ListGroups(ctx context.Context) ([]GroupInfo, error) {
	url := s.endpoint + "/v1/guild.list"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader([]byte(`{}`)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if s.token != "" {
		req.Header.Set("Authorization", "Bearer "+s.token)
	}
	if s.platform != "" {
		req.Header.Set("X-Platform", s.platform)
	}
	if s.selfID != "" {
		req.Header.Set("X-Self-ID", s.selfID)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("satori guild.list: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Data []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("satori parse guild list: %w", err)
	}

	out := make([]GroupInfo, 0, len(result.Data))
	for _, g := range result.Data {
		out = append(out, GroupInfo{
			ID:          g.ID,
			Name:        g.Name,
			ChannelType: "satori",
			ChatType:    "guild",
		})
	}
	return out, nil
}
