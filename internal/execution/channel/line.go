package channel

// ─── Channel: LINE ──────────────────────────────────────────
// Type:     "line"
// Protocol: Webhook (HMAC签名校验) + Push API
// Inbound:  text, image, video, audio, sticker, location
// Outbound: text, sticker (reply_token优先, push兜底)
// Env vars: LINE_CHANNEL_SECRET, LINE_CHANNEL_TOKEN
// Status:   Production — 签名校验完整, 支持 StickerSender
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
	"sync"
	"time"

	"yunque-agent/pkg/safego"
)

// ──────────────────────────────────────────────
// LINE Messaging API Adapter
// ──────────────────────────────────────────────

const (
	lineAPIBase     = "https://api.line.me/v2/bot"
	lineMaxTextLen  = 5000 // LINE text message limit
	lineMaxMessages = 5    // LINE reply/push max messages per call
)

// LINE implements the Channel interface for LINE Messaging API.
type LINE struct {
	channelSecret string
	channelToken  string
	port          string
	bindAddr      string
	apiBase       string
	client        *http.Client
	msgCh         chan Message
}

// LINEConfig holds configuration for the LINE channel.
type LINEConfig struct {
	ChannelSecret string `json:"channel_secret"`
	ChannelToken  string `json:"channel_token"`
	Port          string `json:"port"`
	BindAddr      string `json:"bind_addr,omitempty"`
	APIBase       string `json:"api_base,omitempty"` // for testing
}

// NewLINE creates a LINE Messaging API channel.
func NewLINE(cfg LINEConfig) *LINE {
	port := cfg.Port
	if port == "" {
		port = "9883"
	}
	bindAddr := cfg.BindAddr
	if bindAddr == "" {
		bindAddr = "0.0.0.0"
	}
	apiBase := cfg.APIBase
	if apiBase == "" {
		apiBase = lineAPIBase
	}
	return &LINE{
		channelSecret: cfg.ChannelSecret,
		channelToken:  cfg.ChannelToken,
		port:          port,
		bindAddr:      bindAddr,
		apiBase:       apiBase,
		client:        &http.Client{Timeout: 15 * time.Second},
		msgCh:         make(chan Message, 100),
	}
}

func (l *LINE) Type() string { return "line" }

// ──────────────────────────────────────────────
// Start — listen for webhook callbacks
// ──────────────────────────────────────────────

func (l *LINE) Start(ctx context.Context, handler func(Message) Reply) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/line/callback", l.webhookHandler())

	addr := l.bindAddr + ":" + l.port
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}

	var wg sync.WaitGroup

	// Message processing goroutine
	wg.Add(1)
	safego.Go("line-msg-processor", func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-l.msgCh:
				reply := handler(msg)
				replyToken := ""
				if msg.Extra != nil {
					replyToken = msg.Extra["reply_token"]
				}
				out := ContentWithButtonFallback(reply)
				if replyToken != "" {
					if err := l.replyMessage(ctx, replyToken, out); err != nil {
						slog.Warn("line: reply failed, trying push", "err", err, "user_id", msg.UserID)
						_ = l.pushMessage(ctx, msg.UserID, out)
					}
				} else {
					_ = l.pushMessage(ctx, msg.UserID, out)
				}
			}
		}
	})

	slog.Info("line channel starting", "addr", addr)
	safego.Go("line-shutdown", func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	})

	err := srv.ListenAndServe()
	wg.Wait()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

// Send pushes a proactive message to a LINE user, including sticker support.
func (l *LINE) Send(ctx context.Context, target string, reply Reply) error {
	// Handle rich message components (stickers, images, etc.)
	if reply.Rich != nil {
		for _, comp := range reply.Rich.Components {
			if comp.Type() == ComponentSticker {
				if sc, ok := comp.(*StickerComponent); ok {
					_ = l.SendSticker(ctx, target, sc)
				}
			}
		}
	}

	// Send text content
	text := ContentWithButtonFallback(reply)
	if text != "" {
		return l.pushMessage(ctx, target, text)
	}
	return nil
}

// ──────────────────────────────────────────────
// Webhook handler
// ──────────────────────────────────────────────

func (l *LINE) webhookHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1MB limit
		if err != nil {
			http.Error(w, "read body failed", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		// Verify signature
		signature := r.Header.Get("X-Line-Signature")
		if !l.verifySignature(body, signature) {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}

		var webhook lineWebhookBody
		if err := json.Unmarshal(body, &webhook); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}

		// Process events
		for _, event := range webhook.Events {
			l.processEvent(event)
		}

		w.WriteHeader(http.StatusOK)
	}
}

// verifySignature validates the LINE webhook signature using HMAC-SHA256.
func (l *LINE) verifySignature(body []byte, signature string) bool {
	if l.channelSecret == "" || signature == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(l.channelSecret))
	mac.Write(body)
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

// processEvent handles a single LINE webhook event.
func (l *LINE) processEvent(event lineEvent) {
	switch event.Type {
	case "message":
		content := l.extractContent(event)
		if content == "" {
			return
		}
		// Build source info
		userID := event.Source.UserID
		channelID := event.Source.GroupID
		if channelID == "" {
			channelID = event.Source.RoomID
		}
		if channelID == "" {
			channelID = userID // 1-on-1 chat
		}

		chatType := event.Source.Type // "user", "group", "room"

		msg := Message{
			ChannelType: "line",
			ChannelID:   channelID,
			UserID:      userID,
			Content:     content,
			Extra: map[string]string{
				"reply_token": event.ReplyToken,
				"message_id":  event.Message.ID,
				"chat_type":   chatType,
				"source_type": event.Source.Type,
			},
		}

		// Attach structured rich message for media types
		msg.Rich = l.buildRichMessage(event)

		select {
		case l.msgCh <- msg:
		default:
			slog.Warn("line: message channel full, dropping message")
		}

	case "follow":
		slog.Info("line: user followed", "user_id", event.Source.UserID)
	case "unfollow":
		slog.Info("line: user unfollowed", "user_id", event.Source.UserID)
	case "join":
		slog.Info("line: joined group/room", "source", event.Source.Type, "id", event.Source.GroupID+event.Source.RoomID)
	case "postback":
		// Postback data from buttons/quick replies
		if event.Postback.Data != "" {
			msg := Message{
				ChannelType: "line",
				ChannelID:   event.Source.GroupID,
				UserID:      event.Source.UserID,
				Content:     event.Postback.Data,
				Extra: map[string]string{
					"reply_token": event.ReplyToken,
					"event_type":  "postback",
				},
			}
			if msg.ChannelID == "" {
				msg.ChannelID = event.Source.UserID
			}
			select {
			case l.msgCh <- msg:
			default:
			}
		}
	}
}

// extractContent extracts text content from a LINE message event.
func (l *LINE) extractContent(event lineEvent) string {
	msg := event.Message
	switch msg.Type {
	case "text":
		return msg.Text
	case "image":
		return "[图片消息]"
	case "video":
		return "[视频消息]"
	case "audio":
		return "[音频消息]"
	case "file":
		name := msg.FileName
		if name == "" {
			name = "file"
		}
		return fmt.Sprintf("[文件: %s]", name)
	case "location":
		return fmt.Sprintf("[位置: %s (%.6f, %.6f)]", msg.Address, msg.Latitude, msg.Longitude)
	case "sticker":
		return fmt.Sprintf("[贴图: packageId=%s, stickerId=%s]", msg.PackageID, msg.StickerID)
	default:
		return ""
	}
}

// buildRichMessage creates a structured RichMessage from a LINE event for media types.
func (l *LINE) buildRichMessage(event lineEvent) *RichMessage {
	msg := event.Message
	switch msg.Type {
	case "image":
		rm := NewRichMessage()
		img := NewImageFromURL("", "")
		img.FileID = msg.ID // LINE content API uses message ID to download
		rm.Add(img)
		return rm
	case "sticker":
		rm := NewRichMessage()
		s := NewSticker(msg.PackageID, msg.StickerID)
		s.Platform = "line"
		s.URL = fmt.Sprintf("https://stickershop.line-scdn.net/stickershop/v1/sticker/%s/iPhone/sticker.png", msg.StickerID)
		rm.Add(s)
		return rm
	case "audio":
		rm := NewRichMessage()
		a := NewAudio("", 0)
		a.FileID = msg.ID
		rm.Add(a)
		return rm
	case "video":
		rm := NewRichMessage()
		v := NewVideo("", 0)
		v.FileID = msg.ID
		rm.Add(v)
		return rm
	case "file":
		rm := NewRichMessage()
		f := NewFile("", msg.FileName)
		f.FileID = msg.ID
		rm.Add(f)
		return rm
	default:
		return nil
	}
}

// ──────────────────────────────────────────────
// Reply / Push message API
// ──────────────────────────────────────────────

// replyMessage uses the reply token (valid for 30s) to send a response.
func (l *LINE) replyMessage(ctx context.Context, replyToken, text string) error {
	parts := splitLINEMessage(text)
	messages := make([]lineTextMessage, 0, len(parts))
	for _, p := range parts {
		if len(messages) >= lineMaxMessages {
			break
		}
		messages = append(messages, lineTextMessage{Type: "text", Text: p})
	}

	payload := lineReplyRequest{
		ReplyToken: replyToken,
		Messages:   messages,
	}
	return l.callAPI(ctx, "/message/reply", payload)
}

// pushMessage proactively sends a message to a user/group.
func (l *LINE) pushMessage(ctx context.Context, to, text string) error {
	parts := splitLINEMessage(text)
	messages := make([]interface{}, 0, len(parts))
	for _, p := range parts {
		if len(messages) >= lineMaxMessages {
			break
		}
		messages = append(messages, lineTextMessage{Type: "text", Text: p})
	}

	payload := linePushRequest{
		To:       to,
		Messages: messages,
	}
	return l.callAPI(ctx, "/message/push", payload)
}

func (l *LINE) callAPI(ctx context.Context, path string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, l.apiBase+path, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+l.channelToken)

	resp, err := l.client.Do(req)
	if err != nil {
		return fmt.Errorf("line api %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("line api %s: status %d: %s", path, resp.StatusCode, string(body))
	}
	return nil
}

// splitLINEMessage splits a long text into LINE-compatible chunks.
func splitLINEMessage(text string) []string {
	return SplitMessage(text, lineMaxTextLen)
}

// ──────────────────────────────────────────────
// LINE Webhook JSON Structures
// ──────────────────────────────────────────────

type lineWebhookBody struct {
	Destination string      `json:"destination"`
	Events      []lineEvent `json:"events"`
}

type lineEvent struct {
	Type       string          `json:"type"`
	Timestamp  int64           `json:"timestamp"`
	ReplyToken string          `json:"replyToken"`
	Source     lineSource      `json:"source"`
	Message    lineMessageBody `json:"message,omitempty"`
	Postback   linePostback    `json:"postback,omitempty"`
}

type lineSource struct {
	Type    string `json:"type"` // "user", "group", "room"
	UserID  string `json:"userId"`
	GroupID string `json:"groupId,omitempty"`
	RoomID  string `json:"roomId,omitempty"`
}

type lineMessageBody struct {
	ID        string  `json:"id"`
	Type      string  `json:"type"` // "text", "image", "video", "audio", "file", "location", "sticker"
	Text      string  `json:"text,omitempty"`
	FileName  string  `json:"fileName,omitempty"`
	Address   string  `json:"address,omitempty"`
	Latitude  float64 `json:"latitude,omitempty"`
	Longitude float64 `json:"longitude,omitempty"`
	PackageID string  `json:"packageId,omitempty"`
	StickerID string  `json:"stickerId,omitempty"`
}

type linePostback struct {
	Data string `json:"data"`
}

type lineTextMessage struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type lineReplyRequest struct {
	ReplyToken string            `json:"replyToken"`
	Messages   []lineTextMessage `json:"messages"`
}

type linePushRequest struct {
	To       string        `json:"to"`
	Messages []interface{} `json:"messages"`
}

// lineStickerMessage is a LINE sticker message object.
type lineStickerMessage struct {
	Type      string `json:"type"`
	PackageID string `json:"packageId"`
	StickerID string `json:"stickerId"`
}

// SendSticker sends a LINE sticker message natively.
func (l *LINE) SendSticker(ctx context.Context, target string, sticker *StickerComponent) error {
	if sticker.PackageID == "" || sticker.StickerID == "" {
		// Fallback to emoji text if no LINE sticker IDs
		if sticker.Emoji != "" {
			return l.pushMessage(ctx, target, sticker.Emoji)
		}
		return fmt.Errorf("LINE sticker requires packageId and stickerId")
	}
	payload := linePushRequest{
		To: target,
		Messages: []interface{}{
			lineStickerMessage{
				Type:      "sticker",
				PackageID: sticker.PackageID,
				StickerID: sticker.StickerID,
			},
		},
	}
	return l.callAPI(ctx, "/message/push", payload)
}

// Ensure LINE implements optional interfaces
var (
	_ Channel       = (*LINE)(nil)
	_ StickerSender = (*LINE)(nil)
)
