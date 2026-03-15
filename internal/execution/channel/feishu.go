package channel

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// Feishu implements the Channel interface for Feishu/Lark Bot API.
type Feishu struct {
	appID      string
	appSecret  string
	encryptKey string // event encrypt key for signature verification
	token      string // tenant_access_token (auto-refreshed)
	tokenMu    sync.RWMutex
	client     *http.Client
	msgCh      chan Message
	cardAction CardActionHandler
}

// NewFeishu creates a Feishu channel.
func NewFeishu(appID, appSecret, encryptKey string) *Feishu {
	return &Feishu{
		appID:      appID,
		appSecret:  appSecret,
		encryptKey: encryptKey,
		client:     &http.Client{Timeout: 30 * time.Second},
		msgCh:      make(chan Message, 100),
	}
}

func (f *Feishu) Type() string { return "feishu" }

// Start listens for messages via webhook (event subscription).
// In production, register a webhook endpoint; here we use polling simulation.
func (f *Feishu) Start(ctx context.Context, handler func(Message) Reply) error {
	// Refresh token periodically
	go f.tokenRefreshLoop(ctx)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg := <-f.msgCh:
			reply := handler(msg)
			_ = f.Send(ctx, msg.ChannelID, reply)
		}
	}
}

// HandleWebhook processes incoming Feishu event callbacks.
// Mount this on your HTTP server: POST /webhook/feishu
func (f *Feishu) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	// Signature verification (production security)
	if f.encryptKey != "" {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(400)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		timestamp := r.Header.Get("X-Lark-Request-Timestamp")
		nonce := r.Header.Get("X-Lark-Request-Nonce")
		sig := r.Header.Get("X-Lark-Signature")
		if sig != "" {
			expected := f.computeSignature(timestamp, nonce, body)
			if sig != expected {
				slog.Warn("feishu webhook: signature mismatch")
				w.WriteHeader(403)
				return
			}
		}
	}

	var event struct {
		Challenge string `json:"challenge"` // URL verification
		Header    struct {
			EventType string `json:"event_type"`
		} `json:"header"`
		Event struct {
			Message struct {
				ChatID      string `json:"chat_id"`
				MessageType string `json:"message_type"`
				Content     string `json:"content"`
			} `json:"message"`
			Sender struct {
				SenderID struct {
					OpenID string `json:"open_id"`
				} `json:"sender_id"`
			} `json:"sender"`
		} `json:"event"`
	}
	json.NewDecoder(r.Body).Decode(&event)

	// URL verification challenge
	if event.Challenge != "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"challenge": event.Challenge})
		return
	}

	switch event.Header.EventType {
	case "im.message.receive_v1":
		// Extract text content
		var content struct {
			Text string `json:"text"`
		}
		json.Unmarshal([]byte(event.Event.Message.Content), &content)

		f.msgCh <- Message{
			ChannelType: "feishu",
			ChannelID:   event.Event.Message.ChatID,
			UserID:      event.Event.Sender.SenderID.OpenID,
			Content:     content.Text,
		}
	case "card.action.trigger":
		// Interactive card button callback
		f.handleCardAction(w, r)
		return
	}
	w.WriteHeader(200)
}

func (f *Feishu) Send(_ context.Context, chatID string, reply Reply) error {
	var msgType, content string

	switch reply.Format {
	case "card":
		// reply.Content is a pre-built card JSON string
		msgType = "interactive"
		content = reply.Content
	case "markdown":
		// Feishu post (rich text) with markdown-like content
		msgType = "interactive"
		card := AgentReplyCard("云雀助手", reply.Content)
		content = card.Build()
	default:
		msgType = "text"
		content = fmt.Sprintf(`{"text":"%s"}`, reply.Content)
	}

	return f.sendRaw(chatID, msgType, content)
}

// SendCard sends a pre-built card message.
func (f *Feishu) SendCard(_ context.Context, chatID string, card *Card) error {
	return f.sendRaw(chatID, "interactive", card.Build())
}

func (f *Feishu) sendRaw(chatID, msgType, content string) error {
	f.tokenMu.RLock()
	token := f.token
	f.tokenMu.RUnlock()

	body, _ := json.Marshal(map[string]any{
		"receive_id": chatID,
		"msg_type":   msgType,
		"content":    content,
	})

	req, _ := http.NewRequest("POST", "https://open.feishu.cn/open-apis/im/v1/messages?receive_id_type=chat_id", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := f.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("feishu send %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

// CardActionHandler processes interactive card button clicks.
type CardActionHandler func(openID string, action map[string]string) *Card

// SetCardActionHandler registers a handler for card button callbacks.
func (f *Feishu) SetCardActionHandler(h CardActionHandler) {
	f.cardAction = h
}

func (f *Feishu) handleCardAction(w http.ResponseWriter, r *http.Request) {
	var cb struct {
		OpenID string            `json:"open_id"`
		Action map[string]string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&cb); err != nil {
		w.WriteHeader(400)
		return
	}

	if f.cardAction != nil {
		if reply := f.cardAction(cb.OpenID, cb.Action); reply != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"toast": map[string]string{"type": "success", "content": "已处理"},
				"card":  json.RawMessage(reply.Build()),
			})
			return
		}
	}
	w.WriteHeader(200)
}

func (f *Feishu) tokenRefreshLoop(ctx context.Context) {
	for {
		f.refreshToken()
		select {
		case <-ctx.Done():
			return
		case <-time.After(90 * time.Minute): // token expires in 2h, refresh at 90min
		}
	}
}

// computeSignature calculates the expected signature for Feishu webhook verification.
// Algorithm: SHA256(timestamp + nonce + encryptKey + body)
func (f *Feishu) computeSignature(timestamp, nonce string, body []byte) string {
	h := sha256.New()
	h.Write([]byte(timestamp + nonce + f.encryptKey))
	h.Write(body)
	return fmt.Sprintf("%x", h.Sum(nil))
}

// React adds an emoji reaction to a Feishu message.
// emoji should be a Feishu emoji type string (e.g., "THUMBSUP", "SMILE", "HEART").
// See: https://open.feishu.cn/document/server-docs/im-v1/message-reaction/create
func (f *Feishu) React(ctx context.Context, _ string, messageID string, emoji string) error {
	if emoji == "" {
		return nil // Feishu doesn't easily support removing reactions via API
	}

	f.tokenMu.RLock()
	token := f.token
	f.tokenMu.RUnlock()

	body, _ := json.Marshal(map[string]any{
		"reaction_type": map[string]string{"emoji_type": emoji},
	})

	url := fmt.Sprintf("https://open.feishu.cn/open-apis/im/v1/messages/%s/reactions", messageID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := f.client.Do(req)
	if err != nil {
		return fmt.Errorf("feishu react: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("feishu react %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

// Ensure Feishu implements optional interfaces
var (
	_ Channel     = (*Feishu)(nil)
	_ Reactor     = (*Feishu)(nil)
	_ GroupLister = (*Feishu)(nil)
)

// ListGroups returns all group chats the Feishu bot is currently in.
func (f *Feishu) ListGroups(_ context.Context) ([]GroupInfo, error) {
	f.tokenMu.RLock()
	token := f.token
	f.tokenMu.RUnlock()

	req, _ := http.NewRequest("GET", "https://open.feishu.cn/open-apis/im/v1/chats?page_size=50", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("feishu list chats: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Code int `json:"code"`
		Data struct {
			Items []struct {
				ChatID      string `json:"chat_id"`
				Name        string `json:"name"`
				ChatType    string `json:"chat_type"` // "p2p" or "group"
				MemberCount int    `json:"user_count"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("feishu parse chat list: %w", err)
	}
	if result.Code != 0 {
		return nil, fmt.Errorf("feishu chat list error code=%d", result.Code)
	}

	var out []GroupInfo
	for _, c := range result.Data.Items {
		if c.ChatType == "p2p" {
			continue // skip 1:1 chats
		}
		out = append(out, GroupInfo{
			ID:          c.ChatID,
			Name:        c.Name,
			ChannelType: "feishu",
			ChatType:    c.ChatType,
			MemberCount: c.MemberCount,
		})
	}
	return out, nil
}

func (f *Feishu) refreshToken() {
	body, _ := json.Marshal(map[string]string{
		"app_id":     f.appID,
		"app_secret": f.appSecret,
	})
	resp, err := f.client.Post("https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal", "application/json", bytes.NewReader(body))
	if err != nil {
		slog.Error("feishu token refresh", "err", err)
		return
	}
	defer resp.Body.Close()
	var result struct {
		Code              int    `json:"code"`
		TenantAccessToken string `json:"tenant_access_token"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	if result.Code == 0 && result.TenantAccessToken != "" {
		f.tokenMu.Lock()
		f.token = result.TenantAccessToken
		f.tokenMu.Unlock()
		slog.Info("feishu token refreshed")
	}
}
