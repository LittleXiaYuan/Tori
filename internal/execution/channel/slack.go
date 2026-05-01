package channel

// ─── Channel: Slack ─────────────────────────────────────────
// Type:     "slack"
// Protocol: Webhook (Events API, 独立端口 :8444)
// Inbound:  text (Events API 签名校验)
// Outbound: text (chat.postMessage, 线程回复)
// Env vars: SLACK_BOT_TOKEN, SLACK_SIGNING_SECRET, SLACK_WEBHOOK_PATH
// Status:   Production — 签名校验完整，支持 GroupLister
// ─────────────────────────────────────────────────────────────

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"time"

	"yunque-agent/pkg/safego"
)

// Slack implements the Channel interface using the Slack Events API + Web API.
type Slack struct {
	botToken      string // xoxb-... Bot User OAuth Token
	appToken      string // xapp-... App-Level Token (for Socket Mode, optional)
	signingSecret string // for verifying webhook requests
	client        *http.Client
	webhookPath   string
}

// SlackConfig holds configuration for the Slack channel.
type SlackConfig struct {
	BotToken      string `json:"bot_token"`
	AppToken      string `json:"app_token,omitempty"`
	SigningSecret string `json:"signing_secret"`
	WebhookPath   string `json:"webhook_path"`
}

// NewSlack creates a Slack channel.
func NewSlack(cfg SlackConfig) *Slack {
	path := cfg.WebhookPath
	if path == "" {
		path = "/webhook/slack"
	}
	return &Slack{
		botToken:      cfg.BotToken,
		appToken:      cfg.AppToken,
		signingSecret: cfg.SigningSecret,
		client:        &http.Client{Timeout: 30 * time.Second},
		webhookPath:   path,
	}
}

func (s *Slack) Type() string { return "slack" }

// Start listens for Slack Events API webhooks (blocking).
func (s *Slack) Start(ctx context.Context, handler func(Message) Reply) error {
	mux := http.NewServeMux()

	mux.HandleFunc(s.webhookPath, func(rw http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			rw.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		s.handleEvent(rw, r, handler)
	})

	server := &http.Server{
		Addr:    ":8444",
		Handler: mux,
	}

	safego.Go("slack-shutdown", func() {
		<-ctx.Done()
		server.Shutdown(context.Background())
	})

	slog.Info("slack: webhook listening", "path", s.webhookPath, "addr", ":8444")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("slack: server error: %w", err)
	}
	return nil
}

// Send sends a message to a Slack channel or DM via chat.postMessage.
func (s *Slack) Send(ctx context.Context, channelID string, reply Reply) error {
	url := "https://slack.com/api/chat.postMessage"

	body := map[string]any{
		"channel": channelID,
		"text":    ContentWithButtonFallback(reply),
	}
	if reply.ReplyTo != "" {
		body["thread_ts"] = reply.ReplyTo
	}

	data, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", "Bearer "+s.botToken)

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("slack: send failed: %w", err)
	}
	defer resp.Body.Close()

	var result slackAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("slack: decode response: %w", err)
	}
	if !result.OK {
		return fmt.Errorf("slack: send error: %s", result.Error)
	}
	return nil
}

// handleEvent processes incoming Slack Events API payloads.
func (s *Slack) handleEvent(rw http.ResponseWriter, r *http.Request, handler func(Message) Reply) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := s.verifySlackSignature(r.Header, body); err != nil {
		slog.Warn("slack: request rejected", "err", err, "remote", r.RemoteAddr)
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}

	var envelope slackEventEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		return
	}

	// URL verification challenge
	if envelope.Type == "url_verification" {
		rw.Header().Set("Content-Type", "application/json")
		json.NewEncoder(rw).Encode(map[string]string{
			"challenge": envelope.Challenge,
		})
		slog.Info("slack: url verification completed")
		return
	}

	rw.WriteHeader(http.StatusOK)

	// Event callback
	if envelope.Type == "event_callback" {
		go s.processEvent(envelope.Event, handler)
	}
}

// verifySlackSignature validates X-Slack-Signature using HMAC-SHA256.
// Signature = 'v0=' + HMAC-SHA256(signingSecret, 'v0:' + timestamp + ':' + body).
// Fails closed: if signingSecret is not configured, all requests are rejected.
func (s *Slack) verifySlackSignature(h http.Header, body []byte) error {
	if s.signingSecret == "" {
		return fmt.Errorf("slack signing secret not configured")
	}

	sig := h.Get("X-Slack-Signature")
	ts := h.Get("X-Slack-Request-Timestamp")
	if sig == "" || ts == "" {
		return fmt.Errorf("missing signature headers")
	}

	tsInt, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp")
	}
	if delta := math.Abs(float64(time.Now().Unix() - tsInt)); delta > 300 {
		return fmt.Errorf("timestamp outside allowed window (%vs)", delta)
	}

	baseString := "v0:" + ts + ":" + string(body)
	mac := hmac.New(sha256.New, []byte(s.signingSecret))
	mac.Write([]byte(baseString))
	expected := "v0=" + hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return fmt.Errorf("signature mismatch")
	}
	return nil
}

func (s *Slack) processEvent(event slackEvent, handler func(Message) Reply) {
	// Only handle message events from users (not bots)
	if event.Type != "message" && event.Type != "app_mention" {
		return
	}
	if event.BotID != "" || event.SubType != "" {
		return // skip bot messages and subtypes (edits, deletes, etc.)
	}

	msg := Message{
		ChannelType: "slack",
		ChannelID:   event.Channel,
		UserID:      event.User,
		UserName:    event.User, // would need users.info API for display name
		Content:     event.Text,
		Extra: map[string]string{
			"ts":        event.TS,
			"thread_ts": event.ThreadTS,
			"event_ts":  event.EventTS,
		},
	}
	if event.ThreadTS != "" {
		msg.ReplyTo = event.ThreadTS
	}

	reply := handler(msg)
	if !IsEmptyReply(reply) {
		// Reply in thread if the original message was in a thread
		if event.ThreadTS != "" {
			reply.ReplyTo = event.ThreadTS
		} else {
			reply.ReplyTo = event.TS // start a thread
		}
		if err := s.Send(context.Background(), event.Channel, reply); err != nil {
			slog.Warn("slack: reply failed", "channel", event.Channel, "err", err)
		}
	}
}

// ──────────────────────────────────────────────
// Slack API types
// ──────────────────────────────────────────────

type slackAPIResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

type slackEventEnvelope struct {
	Token     string     `json:"token"`
	Type      string     `json:"type"` // "url_verification" or "event_callback"
	Challenge string     `json:"challenge,omitempty"`
	TeamID    string     `json:"team_id"`
	Event     slackEvent `json:"event"`
}

type slackEvent struct {
	Type     string `json:"type"` // "message", "app_mention"
	SubType  string `json:"subtype,omitempty"`
	Channel  string `json:"channel"`
	User     string `json:"user"`
	Text     string `json:"text"`
	TS       string `json:"ts"`
	ThreadTS string `json:"thread_ts,omitempty"`
	EventTS  string `json:"event_ts"`
	BotID    string `json:"bot_id,omitempty"`
}

// ListGroups returns all Slack channels/groups the bot is a member of.
func (s *Slack) ListGroups(ctx context.Context) ([]GroupInfo, error) {
	url := "https://slack.com/api/conversations.list?types=public_channel,private_channel,mpim&limit=200"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+s.botToken)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("slack conversations.list: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		OK       bool `json:"ok"`
		Channels []struct {
			ID         string `json:"id"`
			Name       string `json:"name"`
			IsMember   bool   `json:"is_member"`
			IsPrivate  bool   `json:"is_private"`
			NumMembers int    `json:"num_members"`
		} `json:"channels"`
		Error string `json:"error,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("slack parse channels: %w", err)
	}
	if !result.OK {
		return nil, fmt.Errorf("slack conversations.list: %s", result.Error)
	}

	var out []GroupInfo
	for _, ch := range result.Channels {
		if !ch.IsMember {
			continue
		}
		chatType := "channel"
		if ch.IsPrivate {
			chatType = "private_channel"
		}
		out = append(out, GroupInfo{
			ID:          ch.ID,
			Name:        ch.Name,
			ChannelType: "slack",
			ChatType:    chatType,
			MemberCount: ch.NumMembers,
		})
	}
	return out, nil
}
