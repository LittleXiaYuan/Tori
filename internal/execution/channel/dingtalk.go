package channel

// ─── Channel: DingTalk (钉钉) ───────────────────────────────
// Type:     "dingtalk"
// Protocol: Webhook回调 (独立端口 :9881, /callback/dingtalk)
// Inbound:  text, richText (提取纯文本)
// Outbound: Markdown模板 (sampleMarkdown)
// Env vars: DINGTALK_APP_KEY, DINGTALK_APP_SECRET
// Status:   Production — HMAC签名校验完整
// ─────────────────────────────────────────────────────────────

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"yunque-agent/pkg/safego"
)

// DingTalk implements the Channel interface for DingTalk (钉钉) Bot.
// Uses HTTP webhook mode for receiving messages and REST API for sending.
type DingTalk struct {
	clientID     string // AppKey
	clientSecret string // AppSecret
	robotCode    string // 机器人编码 (same as clientID for most cases)
	port         string // 回调服务监听端口
	bindAddr     string // 回调服务绑定地址

	accessToken   string
	tokenExpireAt time.Time
	tokenMu       sync.RWMutex
	client        *http.Client
	msgCh         chan Message
}

// DingTalkConfig 钉钉配置
type DingTalkConfig struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	RobotCode    string `json:"robot_code,omitempty"` // defaults to ClientID
	Port         string `json:"port"`
	BindAddr     string `json:"bind_addr,omitempty"`
}

// NewDingTalk creates a DingTalk channel.
func NewDingTalk(cfg DingTalkConfig) *DingTalk {
	bindAddr := cfg.BindAddr
	if bindAddr == "" {
		bindAddr = "0.0.0.0"
	}
	port := cfg.Port
	if port == "" {
		port = "9881"
	}
	robotCode := cfg.RobotCode
	if robotCode == "" {
		robotCode = cfg.ClientID
	}
	return &DingTalk{
		clientID:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
		robotCode:    robotCode,
		port:         port,
		bindAddr:     bindAddr,
		client:       &http.Client{Timeout: 30 * time.Second},
		msgCh:        make(chan Message, 100),
	}
}

func (d *DingTalk) Type() string { return "dingtalk" }

// Start begins listening for messages via HTTP callback.
func (d *DingTalk) Start(ctx context.Context, handler func(Message) Reply) error {
	// Start token refresh loop
	go d.tokenRefreshLoop(ctx)

	// Start HTTP callback server
	mux := http.NewServeMux()
	mux.HandleFunc("/callback/dingtalk", d.handleCallback)

	addr := fmt.Sprintf("%s:%s", d.bindAddr, d.port)
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}

	safego.Go("dingtalk-server", func() {
		slog.Info("dingtalk callback server started", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("dingtalk callback server error", "err", err)
		}
	})

	// Process incoming messages
	for {
		select {
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			srv.Shutdown(shutdownCtx)
			return ctx.Err()
		case msg := <-d.msgCh:
			go func(m Message) {
				reply := handler(m)
				if !IsEmptyReply(reply) {
					target := m.Extra["conversation_id"]
					if target == "" {
						target = m.UserID
					}
					if err := d.Send(ctx, target, reply); err != nil {
						slog.Warn("dingtalk: reply failed", "to", target, "err", err)
					}
				}
			}(msg)
		}
	}
}

// handleCallback processes incoming DingTalk robot callback.
func (d *DingTalk) handleCallback(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		rw.WriteHeader(405)
		return
	}

	// Verify signature — fail-closed when clientSecret is configured
	timestamp := r.Header.Get("timestamp")
	sign := r.Header.Get("sign")
	if d.clientSecret != "" {
		if timestamp == "" || sign == "" {
			slog.Warn("dingtalk: missing signature headers", "remote", r.RemoteAddr)
			rw.WriteHeader(403)
			return
		}
		if !d.verifySignature(timestamp, sign) {
			slog.Warn("dingtalk: signature verification failed", "remote", r.RemoteAddr)
			rw.WriteHeader(403)
			return
		}
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1MB limit
	if err != nil {
		rw.WriteHeader(400)
		return
	}

	var callback dingtalkCallback
	if err := json.Unmarshal(body, &callback); err != nil {
		slog.Warn("dingtalk: invalid json", "err", err)
		rw.WriteHeader(400)
		return
	}

	// Extract text content
	content := ""
	switch callback.MsgType {
	case "text":
		var textContent struct {
			Content string `json:"content"`
		}
		if callback.Text != nil {
			json.Unmarshal(callback.Text, &textContent)
			content = strings.TrimSpace(textContent.Content)
		}
	case "richText":
		// Extract text from rich text content
		var richContent struct {
			RichText []struct {
				Text string `json:"text,omitempty"`
			} `json:"richText"`
		}
		if callback.Content != nil {
			json.Unmarshal(callback.Content, &richContent)
			var texts []string
			for _, rt := range richContent.RichText {
				if rt.Text != "" {
					texts = append(texts, rt.Text)
				}
			}
			content = strings.Join(texts, " ")
		}
	default:
		// For other types, try to extract text
		if callback.Text != nil {
			var textContent struct {
				Content string `json:"content"`
			}
			json.Unmarshal(callback.Text, &textContent)
			content = strings.TrimSpace(textContent.Content)
		}
	}

	if content == "" {
		rw.WriteHeader(200)
		return
	}

	chatType := "private"
	conversationID := callback.SenderStaffID
	if callback.ConversationType == "2" {
		chatType = "group"
		conversationID = callback.ConversationID
	}

	msg := Message{
		ChannelType: "dingtalk",
		ChannelID:   conversationID,
		UserID:      callback.SenderStaffID,
		UserName:    callback.SenderNick,
		Content:     content,
		Extra: map[string]string{
			"chat_type":       chatType,
			"conversation_id": callback.ConversationID,
			"msg_id":          callback.MsgID,
			"msg_type":        callback.MsgType,
			"sender_corpid":   callback.SenderCorpID,
		},
	}

	TrySendMessage(d.msgCh, msg, "dingtalk")

	rw.WriteHeader(200)
}

// Send pushes a message. Target is conversationID for group or staffID for private.
func (d *DingTalk) Send(_ context.Context, target string, reply Reply) error {
	token := d.getAccessToken()
	if token == "" {
		return fmt.Errorf("dingtalk: no access token")
	}

	// Determine message key
	msgKey := "sampleMarkdown"
	msgParam := map[string]string{
		"title": "回复",
		"text":  ContentWithButtonFallback(reply),
	}

	paramJSON, _ := json.Marshal(msgParam)

	// Try group message first, fallback to private
	if strings.Contains(target, "$") || len(target) > 20 {
		return d.sendGroupMessage(token, target, msgKey, string(paramJSON))
	}
	return d.sendPrivateMessage(token, target, msgKey, string(paramJSON))
}

// sendGroupMessage sends a message to a DingTalk group.
func (d *DingTalk) sendGroupMessage(token, conversationID, msgKey, msgParam string) error {
	payload := map[string]string{
		"msgKey":             msgKey,
		"msgParam":           msgParam,
		"openConversationId": conversationID,
		"robotCode":          d.robotCode,
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "https://api.dingtalk.com/v1.0/robot/groupMessages/send", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-acs-dingtalk-access-token", token)

	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("dingtalk: send group msg: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("dingtalk: send group msg %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

// sendPrivateMessage sends a private message to a DingTalk user.
func (d *DingTalk) sendPrivateMessage(token, staffID, msgKey, msgParam string) error {
	payload := map[string]any{
		"msgKey":    msgKey,
		"msgParam":  msgParam,
		"robotCode": d.robotCode,
		"userIds":   []string{staffID},
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "https://api.dingtalk.com/v1.0/robot/oToMessages/batchSend", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-acs-dingtalk-access-token", token)

	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("dingtalk: send private msg: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("dingtalk: send private msg %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

// verifySignature verifies DingTalk robot callback signature.
// Algorithm: HMAC-SHA256(timestamp + "\n" + secret)
func (d *DingTalk) verifySignature(timestamp, sign string) bool {
	stringToSign := timestamp + "\n" + d.clientSecret
	mac := hmac.New(sha256.New, []byte(d.clientSecret))
	mac.Write([]byte(stringToSign))
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return expected == sign
}

func (d *DingTalk) getAccessToken() string {
	d.tokenMu.RLock()
	if d.accessToken != "" && time.Now().Before(d.tokenExpireAt) {
		token := d.accessToken
		d.tokenMu.RUnlock()
		return token
	}
	d.tokenMu.RUnlock()

	d.refreshAccessToken()
	d.tokenMu.RLock()
	defer d.tokenMu.RUnlock()
	return d.accessToken
}

func (d *DingTalk) refreshAccessToken() {
	payload := map[string]string{
		"appKey":    d.clientID,
		"appSecret": d.clientSecret,
	}
	body, _ := json.Marshal(payload)

	resp, err := d.client.Post("https://api.dingtalk.com/v1.0/oauth2/accessToken", "application/json", bytes.NewReader(body))
	if err != nil {
		slog.Error("dingtalk: token refresh failed", "err", err)
		return
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken string `json:"accessToken"`
		ExpireIn    int    `json:"expireIn"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		slog.Error("dingtalk: token refresh decode failed", "err", err)
		return
	}
	if result.AccessToken == "" {
		slog.Error("dingtalk: token refresh returned empty token")
		return
	}

	d.tokenMu.Lock()
	d.accessToken = result.AccessToken
	// Refresh 10 minutes before expiry
	d.tokenExpireAt = time.Now().Add(time.Duration(result.ExpireIn-600) * time.Second)
	d.tokenMu.Unlock()
	slog.Info("dingtalk: access token refreshed", "expires_in", result.ExpireIn)
}

func (d *DingTalk) tokenRefreshLoop(ctx context.Context) {
	d.refreshAccessToken()
	ticker := time.NewTicker(90 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.refreshAccessToken()
		}
	}
}

// DingTalk callback JSON types
type dingtalkCallback struct {
	MsgID                     string          `json:"msgId"`
	MsgType                   string          `json:"msgtype"`
	Text                      json.RawMessage `json:"text,omitempty"`
	Content                   json.RawMessage `json:"content,omitempty"`
	ConversationID            string          `json:"conversationId"`
	ConversationType          string          `json:"conversationType"` // "1"=private, "2"=group
	SenderID                  string          `json:"senderId"`
	SenderStaffID             string          `json:"senderStaffId"`
	SenderNick                string          `json:"senderNick"`
	SenderCorpID              string          `json:"senderCorpId"`
	ChatbotCorpID             string          `json:"chatbotCorpId"`
	ChatbotUserID             string          `json:"chatbotUserId"`
	CreateAt                  int64           `json:"createAt"`
	IsAdmin                   bool            `json:"isAdmin"`
	SessionWebhook            string          `json:"sessionWebhook"`
	SessionWebhookExpiredTime int64           `json:"sessionWebhookExpiredTime"`
}
