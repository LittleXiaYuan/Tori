package channel

// ─── Channel: WeChat Official (微信公众号) ──────────────────
// Type:     "wechat_official"
// Protocol: Webhook回调 (独立端口 :9882, /wechat/callback)
// Inbound:  text, image, voice, video, location, link, event
// Outbound: text (客服消息API, 拆分600字符)
// Env vars: WECHAT_APPID, WECHAT_SECRET, WECHAT_TOKEN
// Status:   Basic — 多类型入站解析完整，出站仅文本
// ─────────────────────────────────────────────────────────────

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

	"yunque-agent/pkg/safego"
)

// WeChatOfficial implements the Channel interface for WeChat Official Account (微信公众号).
// It supports both subscription and service accounts via the official platform callback API.
type WeChatOfficial struct {
	appID          string // 公众号 AppID
	appSecret      string // 公众号 AppSecret
	token          string // 消息验证 Token
	encodingAESKey string // EncodingAESKey (可选，用于消息加密)
	port           string // 回调服务监听端口
	bindAddr       string // 回调服务绑定地址
	apiBaseURL     string // API 基础地址

	accessToken   string
	tokenExpireAt time.Time
	tokenMu       sync.RWMutex
	client        *http.Client
	msgCh         chan Message
}

// WeChatOfficialConfig 微信公众号配置
type WeChatOfficialConfig struct {
	AppID          string `json:"appid"`
	AppSecret      string `json:"app_secret"`
	Token          string `json:"token"`
	EncodingAESKey string `json:"encoding_aes_key,omitempty"`
	Port           string `json:"port"`
	BindAddr       string `json:"bind_addr,omitempty"`
	APIBaseURL     string `json:"api_base_url,omitempty"`
}

// NewWeChatOfficial creates a WeChat Official Account channel.
func NewWeChatOfficial(cfg WeChatOfficialConfig) *WeChatOfficial {
	apiBase := cfg.APIBaseURL
	if apiBase == "" {
		apiBase = "https://api.weixin.qq.com/cgi-bin"
	}
	bindAddr := cfg.BindAddr
	if bindAddr == "" {
		bindAddr = "0.0.0.0"
	}
	port := cfg.Port
	if port == "" {
		port = "9882"
	}
	return &WeChatOfficial{
		appID:          cfg.AppID,
		appSecret:      cfg.AppSecret,
		token:          cfg.Token,
		encodingAESKey: cfg.EncodingAESKey,
		port:           port,
		bindAddr:       bindAddr,
		apiBaseURL:     apiBase,
		client:         &http.Client{Timeout: 30 * time.Second},
		msgCh:          make(chan Message, 100),
	}
}

func (w *WeChatOfficial) Type() string { return "wechat_official" }

// Start begins listening for messages via the WeChat callback server.
func (w *WeChatOfficial) Start(ctx context.Context, handler func(Message) Reply) error {
	// Start token refresh loop
	go w.tokenRefreshLoop(ctx)

	mux := http.NewServeMux()
	mux.HandleFunc("/wechat/callback", w.handleCallback)

	addr := fmt.Sprintf("%s:%s", w.bindAddr, w.port)
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}

	safego.Go("wechat-official-server", func() {
		slog.Info("wechat official callback server started", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("wechat official callback server error", "err", err)
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
		case msg := <-w.msgCh:
			go func(m Message) {
				reply := handler(m)
				if !IsEmptyReply(reply) {
					if err := w.Send(ctx, m.UserID, reply); err != nil {
						slog.Warn("wechat official: reply failed", "to", m.UserID, "err", err)
					}
				}
			}(msg)
		}
	}
}

// handleCallback processes incoming WeChat event callbacks.
func (w *WeChatOfficial) handleCallback(rw http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	signature := query.Get("signature")
	timestamp := query.Get("timestamp")
	nonce := query.Get("nonce")

	// GET = URL verification (微信服务器接入验证)
	if r.Method == http.MethodGet {
		echoStr := query.Get("echostr")
		if w.verifySignature(signature, timestamp, nonce) {
			rw.Write([]byte(echoStr))
			return
		}
		rw.WriteHeader(403)
		return
	}

	// POST = message callback
	if !w.verifySignature(signature, timestamp, nonce) {
		slog.Warn("wechat official: signature mismatch")
		rw.WriteHeader(403)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1MB limit
	if err != nil {
		rw.WriteHeader(400)
		return
	}

	// Parse XML message
	var xmlMsg wxOfficialXMLMessage
	if err := xml.Unmarshal(body, &xmlMsg); err != nil {
		slog.Warn("wechat official: invalid xml", "err", err)
		rw.WriteHeader(400)
		return
	}

	// Handle different message types
	content := w.extractContent(&xmlMsg)
	if content == "" {
		// Return "success" for events we don't process (subscribe, etc.)
		rw.WriteHeader(200)
		rw.Write([]byte("success"))
		return
	}

	msg := Message{
		ChannelType: "wechat_official",
		ChannelID:   xmlMsg.ToUserName,
		UserID:      xmlMsg.FromUserName,
		UserName:    xmlMsg.FromUserName,
		Content:     content,
		Extra: map[string]string{
			"msg_id":   fmt.Sprintf("%d", xmlMsg.MsgID),
			"msg_type": xmlMsg.MsgType,
		},
	}

	TrySendMessage(w.msgCh, msg, "wechat_official")

	// Return "success" to acknowledge receipt (async reply via customer service API)
	rw.WriteHeader(200)
	rw.Write([]byte("success"))
}

// extractContent extracts text content from different message types.
func (w *WeChatOfficial) extractContent(msg *wxOfficialXMLMessage) string {
	switch msg.MsgType {
	case "text":
		return msg.Content
	case "image":
		return "[图片] " + msg.PicURL
	case "voice":
		// If speech recognition is enabled, use Recognition field
		if msg.Recognition != "" {
			return msg.Recognition
		}
		return "[语音消息]"
	case "video", "shortvideo":
		return "[视频消息]"
	case "location":
		return fmt.Sprintf("[位置] %s (%.6f, %.6f)", msg.Label, msg.LocationX, msg.LocationY)
	case "link":
		return fmt.Sprintf("[链接] %s: %s", msg.Title, msg.URL)
	case "event":
		return w.handleEvent(msg)
	default:
		return ""
	}
}

// handleEvent handles WeChat events (subscribe, unsubscribe, menu clicks, etc.)
func (w *WeChatOfficial) handleEvent(msg *wxOfficialXMLMessage) string {
	switch msg.Event {
	case "subscribe":
		return "用户关注了公众号"
	case "CLICK":
		return msg.EventKey
	case "VIEW":
		// URL click — no content to process
		return ""
	default:
		return ""
	}
}

// Send pushes a message to a WeChat user via the Customer Service API (客服消息接口).
func (w *WeChatOfficial) Send(_ context.Context, target string, reply Reply) error {
	token := w.getAccessToken()
	if token == "" {
		return fmt.Errorf("wechat official: no access token")
	}

	// Split long messages (WeChat text limit: 600 chars for customer service API)
	parts := splitWxOfficialMessage(ContentWithButtonFallback(reply), 600)
	for _, part := range parts {
		payload := map[string]any{
			"touser":  target,
			"msgtype": "text",
			"text": map[string]string{
				"content": part,
			},
		}

		body, _ := json.Marshal(payload)
		apiURL := fmt.Sprintf("%s/message/custom/send?access_token=%s", w.apiBaseURL, token)
		resp, err := w.client.Post(apiURL, "application/json", bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("wechat official: send: %w", err)
		}
		var result wxOfficialAPIResponse
		decErr := json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()
		if decErr != nil {
			return fmt.Errorf("wechat official: decode response: %w", decErr)
		}
		if result.ErrCode != 0 {
			return fmt.Errorf("wechat official: send error %d: %s", result.ErrCode, result.ErrMsg)
		}
	}
	return nil
}

// verifySignature verifies WeChat callback signature.
// Algorithm: SHA1(sort(token, timestamp, nonce))
func (w *WeChatOfficial) verifySignature(signature, timestamp, nonce string) bool {
	strs := []string{w.token, timestamp, nonce}
	sort.Strings(strs)
	h := sha1.New()
	h.Write([]byte(strings.Join(strs, "")))
	expected := fmt.Sprintf("%x", h.Sum(nil))
	return expected == signature
}

func (w *WeChatOfficial) getAccessToken() string {
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

func (w *WeChatOfficial) refreshAccessToken() {
	url := fmt.Sprintf("%s/token?grant_type=client_credential&appid=%s&secret=%s",
		w.apiBaseURL, w.appID, w.appSecret)
	resp, err := w.client.Get(url)
	if err != nil {
		slog.Error("wechat official: token refresh failed", "err", err)
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
		slog.Error("wechat official: token refresh error", "code", result.ErrCode, "msg", result.ErrMsg)
		return
	}

	w.tokenMu.Lock()
	w.accessToken = result.AccessToken
	// Refresh 10 minutes before expiry
	w.tokenExpireAt = time.Now().Add(time.Duration(result.ExpiresIn-600) * time.Second)
	w.tokenMu.Unlock()
	slog.Info("wechat official: access token refreshed", "expires_in", result.ExpiresIn)
}

func (w *WeChatOfficial) tokenRefreshLoop(ctx context.Context) {
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

// splitWxOfficialMessage splits a long message into chunks at sentence boundaries.
func splitWxOfficialMessage(text string, maxLen int) []string {
	return SplitMessageBytes(text, maxLen)
}

// WeChat Official Account XML message types
type wxOfficialXMLMessage struct {
	XMLName      xml.Name `xml:"xml"`
	ToUserName   string   `xml:"ToUserName"`
	FromUserName string   `xml:"FromUserName"`
	CreateTime   int64    `xml:"CreateTime"`
	MsgType      string   `xml:"MsgType"`
	Content      string   `xml:"Content"`
	MsgID        int64    `xml:"MsgId"`
	PicURL       string   `xml:"PicUrl"`
	MediaID      string   `xml:"MediaId"`
	Format       string   `xml:"Format"`
	Recognition  string   `xml:"Recognition"`
	ThumbMediaID string   `xml:"ThumbMediaId"`
	LocationX    float64  `xml:"Location_X"`
	LocationY    float64  `xml:"Location_Y"`
	Scale        int      `xml:"Scale"`
	Label        string   `xml:"Label"`
	Title        string   `xml:"Title"`
	Description  string   `xml:"Description"`
	URL          string   `xml:"Url"`
	Event        string   `xml:"Event"`
	EventKey     string   `xml:"EventKey"`
}

type wxOfficialAPIResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}
