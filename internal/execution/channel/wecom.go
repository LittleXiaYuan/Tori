package channel

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// WeCom implements the Channel interface for WeCom (企业微信) Bot API.
// It uses webhook callback mode for receiving messages and server API for sending.
type WeCom struct {
	corpID     string // 企业 ID
	agentID    string // 应用 AgentId
	secret     string // 应用 Secret
	token      string // 回调 Token (用于签名验证)
	aesKey     string // EncodingAESKey (消息加密，可选)
	port       string // 回调服务监听端口
	bindAddr   string // 回调服务绑定地址
	apiBaseURL string // API 基础地址

	accessToken   string
	tokenExpireAt time.Time
	tokenMu       sync.RWMutex
	client        *http.Client
	msgCh         chan Message
}

// WeComConfig 企业微信配置
type WeComConfig struct {
	CorpID     string `json:"corpid"`
	AgentID    string `json:"agent_id"`
	Secret     string `json:"secret"`
	Token      string `json:"token"`
	AESKey     string `json:"encoding_aes_key,omitempty"`
	Port       string `json:"port"`
	BindAddr   string `json:"bind_addr,omitempty"`
	APIBaseURL string `json:"api_base_url,omitempty"`
}

// NewWeCom creates a WeCom channel.
func NewWeCom(cfg WeComConfig) *WeCom {
	apiBase := cfg.APIBaseURL
	if apiBase == "" {
		apiBase = "https://qyapi.weixin.qq.com/cgi-bin"
	}
	bindAddr := cfg.BindAddr
	if bindAddr == "" {
		bindAddr = "0.0.0.0"
	}
	port := cfg.Port
	if port == "" {
		port = "9880"
	}
	return &WeCom{
		corpID:     cfg.CorpID,
		agentID:    cfg.AgentID,
		secret:     cfg.Secret,
		token:      cfg.Token,
		aesKey:     cfg.AESKey,
		port:       port,
		bindAddr:   bindAddr,
		apiBaseURL: apiBase,
		client:     &http.Client{Timeout: 30 * time.Second},
		msgCh:      make(chan Message, 100),
	}
}

func (w *WeCom) Type() string { return "wecom" }

// Start begins listening for messages via webhook callback server.
func (w *WeCom) Start(ctx context.Context, handler func(Message) Reply) error {
	// Start token refresh loop
	go w.tokenRefreshLoop(ctx)

	// Start HTTP callback server
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", w.handleCallback)

	addr := fmt.Sprintf("%s:%s", w.bindAddr, w.port)
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}

	go func() {
		slog.Info("wecom callback server started", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("wecom callback server error", "err", err)
		}
	}()

	// Process incoming messages
	for {
		select {
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			srv.Shutdown(shutdownCtx)
			return ctx.Err()
		case msg := <-w.msgCh:
			go func(m Message) {
				reply := handler(m)
				if strings.TrimSpace(ContentWithButtonFallback(reply)) != "" {
					if err := w.Send(ctx, m.UserID, reply); err != nil {
						slog.Warn("wecom: reply failed", "to", m.UserID, "err", err)
					}
				}
			}(msg)
		}
	}
}

// handleCallback processes incoming WeCom event callbacks.
func (w *WeCom) handleCallback(rw http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	msgSignature := query.Get("msg_signature")
	timestamp := query.Get("timestamp")
	nonce := query.Get("nonce")

	// GET = URL verification
	if r.Method == http.MethodGet {
		echoStr := query.Get("echostr")
		if w.verifySignature(msgSignature, timestamp, nonce, echoStr) {
			// In full implementation, decrypt echoStr with AES.
			// For plain-text mode, return echoStr directly.
			rw.Write([]byte(echoStr))
			return
		}
		rw.WriteHeader(403)
		return
	}

	// POST = message callback
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1MB limit
	if err != nil {
		rw.WriteHeader(400)
		return
	}

	// Parse XML message
	var xmlMsg wecomXMLMessage
	if err := xml.Unmarshal(body, &xmlMsg); err != nil {
		slog.Warn("wecom: invalid xml", "err", err)
		rw.WriteHeader(400)
		return
	}

	// Verify signature
	if !w.verifySignature(msgSignature, timestamp, nonce, xmlMsg.Encrypt) {
		slog.Warn("wecom: signature mismatch")
		rw.WriteHeader(403)
		return
	}

	// Process based on message type
	content := xmlMsg.Content
	if content == "" {
		rw.WriteHeader(200)
		rw.Write([]byte("success"))
		return
	}

	msg := Message{
		ChannelType: "wecom",
		ChannelID:   xmlMsg.AgentID,
		UserID:      xmlMsg.FromUserName,
		UserName:    xmlMsg.FromUserName,
		Content:     content,
		Extra: map[string]string{
			"msg_id":   xmlMsg.MsgID,
			"msg_type": xmlMsg.MsgType,
			"agent_id": xmlMsg.AgentID,
		},
	}

	TrySendMessage(w.msgCh, msg, "wecom")

	rw.WriteHeader(200)
	rw.Write([]byte("success"))
}

// Send pushes a message to a WeCom user via the Server API.
func (w *WeCom) Send(_ context.Context, target string, reply Reply) error {
	token := w.getAccessToken()
	if token == "" {
		return fmt.Errorf("wecom: no access token")
	}

	// Split long messages (WeCom text limit: 2048 chars)
	parts := splitWeComMessage(ContentWithButtonFallback(reply), 2048)
	for _, part := range parts {
		payload := map[string]any{
			"touser":  target,
			"agentid": w.agentID,
			"msgtype": "text",
			"text": map[string]string{
				"content": part,
			},
		}

		body, _ := json.Marshal(payload)
		url := fmt.Sprintf("%s/message/send?access_token=%s", w.apiBaseURL, token)
		resp, err := w.client.Post(url, "application/json", bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("wecom: send: %w", err)
		}
		defer resp.Body.Close()

		var result wecomAPIResponse
		json.NewDecoder(resp.Body).Decode(&result)
		if result.ErrCode != 0 {
			return fmt.Errorf("wecom: send error %d: %s", result.ErrCode, result.ErrMsg)
		}
	}
	return nil
}

// SendMarkdown sends a markdown message to target.
func (w *WeCom) SendMarkdown(_ context.Context, target string, content string) error {
	token := w.getAccessToken()
	if token == "" {
		return fmt.Errorf("wecom: no access token")
	}

	payload := map[string]any{
		"touser":  target,
		"agentid": w.agentID,
		"msgtype": "markdown",
		"markdown": map[string]string{
			"content": content,
		},
	}

	body, _ := json.Marshal(payload)
	url := fmt.Sprintf("%s/message/send?access_token=%s", w.apiBaseURL, token)
	resp, err := w.client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("wecom: send markdown: %w", err)
	}
	defer resp.Body.Close()

	var result wecomAPIResponse
	json.NewDecoder(resp.Body).Decode(&result)
	if result.ErrCode != 0 {
		return fmt.Errorf("wecom: send markdown error %d: %s", result.ErrCode, result.ErrMsg)
	}
	return nil
}

// verifySignature verifies WeCom callback signature.
// Algorithm: SHA1(sort(token, timestamp, nonce, encrypt))
func (w *WeCom) verifySignature(signature, timestamp, nonce, encrypt string) bool {
	strs := []string{w.token, timestamp, nonce}
	if encrypt != "" {
		strs = append(strs, encrypt)
	}
	sort.Strings(strs)
	h := sha1.New()
	h.Write([]byte(strings.Join(strs, "")))
	expected := fmt.Sprintf("%x", h.Sum(nil))
	return expected == signature
}

func (w *WeCom) getAccessToken() string {
	w.tokenMu.RLock()
	if w.accessToken != "" && time.Now().Before(w.tokenExpireAt) {
		token := w.accessToken
		w.tokenMu.RUnlock()
		return token
	}
	w.tokenMu.RUnlock()

	w.refreshAccessToken()
	w.tokenMu.RLock()
	defer w.tokenMu.RUnlock()
	return w.accessToken
}

func (w *WeCom) refreshAccessToken() {
	url := fmt.Sprintf("%s/gettoken?corpid=%s&corpsecret=%s", w.apiBaseURL, w.corpID, w.secret)
	resp, err := w.client.Get(url)
	if err != nil {
		slog.Error("wecom: token refresh failed", "err", err)
		return
	}
	defer resp.Body.Close()

	var result struct {
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	if result.ErrCode != 0 {
		slog.Error("wecom: token refresh error", "code", result.ErrCode, "msg", result.ErrMsg)
		return
	}

	w.tokenMu.Lock()
	w.accessToken = result.AccessToken
	// Refresh 10 minutes before expiry
	w.tokenExpireAt = time.Now().Add(time.Duration(result.ExpiresIn-600) * time.Second)
	w.tokenMu.Unlock()
	slog.Info("wecom: access token refreshed", "expires_in", result.ExpiresIn)
}

func (w *WeCom) tokenRefreshLoop(ctx context.Context) {
	w.refreshAccessToken()
	ticker := time.NewTicker(90 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.refreshAccessToken()
		}
	}
}

// splitWeComMessage splits a long message into chunks at sentence boundaries.
func splitWeComMessage(text string, maxLen int) []string {
	return SplitMessageBytes(text, maxLen)
}

// WeCom XML message types
type wecomXMLMessage struct {
	XMLName      xml.Name `xml:"xml"`
	ToUserName   string   `xml:"ToUserName"`
	FromUserName string   `xml:"FromUserName"`
	CreateTime   int64    `xml:"CreateTime"`
	MsgType      string   `xml:"MsgType"`
	Content      string   `xml:"Content"`
	MsgID        string   `xml:"MsgId"`
	AgentID      string   `xml:"AgentID"`
	Encrypt      string   `xml:"Encrypt"`
}

type wecomAPIResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}
