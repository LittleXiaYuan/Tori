package channel

// ─── Channel: WhatsApp ──────────────────────────────────────
// Type:     "whatsapp"
// Protocol: Webhook (Cloud API v21, 独立端口 :8443)
// Inbound:  text (仅文本，其他类型待实现)
// Outbound: text (仅纯文本，无模板/交互消息)
// Env vars: WHATSAPP_PHONE_NUMBER_ID, WHATSAPP_ACCESS_TOKEN,
//           WHATSAPP_VERIFY_TOKEN, WHATSAPP_WEBHOOK_PATH
// Status:   Stub — 基础可用，已支持 webhook 签名校验, 缺少多媒体支持
//
// TODO: [P2] 支持入站多媒体消息 (image/audio/document/video/location/sticker)
// TODO: [P2] 支持出站交互消息 (interactive buttons/lists)
// TODO: [P3] 支持 WhatsApp 模板消息 (template messages)
// TODO: [P3] 实现 ProgressSender 接口 (编辑消息)
// ─────────────────────────────────────────────────────────────

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"yunque-agent/pkg/safego"
)

// WhatsApp implements the Channel interface using the WhatsApp Cloud API.
// Requires a Meta Business account with WhatsApp Business API access.
type WhatsApp struct {
	phoneNumberID string // WhatsApp phone number ID
	token         string // permanent or temporary access token
	verifyToken   string // webhook verify token
	appSecret     string // webhook app secret for X-Hub-Signature-256
	client        *http.Client
	webhookPath   string // e.g. "/webhook/whatsapp"
}

// WhatsAppConfig holds configuration for the WhatsApp channel.
type WhatsAppConfig struct {
	PhoneNumberID string `json:"phone_number_id"`
	AccessToken   string `json:"access_token"`
	VerifyToken   string `json:"verify_token"`
	AppSecret     string `json:"app_secret"`
	WebhookPath   string `json:"webhook_path"`
}

// NewWhatsApp creates a WhatsApp channel.
func NewWhatsApp(cfg WhatsAppConfig) *WhatsApp {
	path := cfg.WebhookPath
	if path == "" {
		path = "/webhook/whatsapp"
	}
	return &WhatsApp{
		phoneNumberID: cfg.PhoneNumberID,
		token:         cfg.AccessToken,
		verifyToken:   cfg.VerifyToken,
		appSecret:     cfg.AppSecret,
		client:        &http.Client{Timeout: 30 * time.Second},
		webhookPath:   path,
	}
}

func (w *WhatsApp) Type() string { return "whatsapp" }

// Start listens for incoming WhatsApp webhook events (blocking).
func (w *WhatsApp) Start(ctx context.Context, handler func(Message) Reply) error {
	mux := http.NewServeMux()

	// Webhook verification (GET)
	mux.HandleFunc(w.webhookPath, func(rw http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.handleVerify(rw, r)
			return
		}
		if r.Method == http.MethodPost {
			w.handleMessage(rw, r, handler)
			return
		}
		rw.WriteHeader(http.StatusMethodNotAllowed)
	})

	server := &http.Server{
		Addr:    ":8443",
		Handler: mux,
	}

	safego.Go("whatsapp-shutdown", func() {
		<-ctx.Done()
		server.Shutdown(context.Background())
	})

	slog.Info("whatsapp: webhook listening", "path", w.webhookPath, "addr", ":8443")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("whatsapp: server error: %w", err)
	}
	return nil
}

// Send sends a text message via WhatsApp Cloud API.
func (w *WhatsApp) Send(ctx context.Context, to string, reply Reply) error {
	url := fmt.Sprintf("https://graph.facebook.com/v21.0/%s/messages", w.phoneNumberID)

	body := map[string]any{
		"messaging_product": "whatsapp",
		"to":                to,
		"type":              "text",
		"text": map[string]any{
			"body": ContentWithButtonFallback(reply),
		},
	}

	data, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+w.token)

	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("whatsapp: send failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("whatsapp: send error HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// handleVerify handles the webhook verification challenge from Meta.
func (w *WhatsApp) handleVerify(rw http.ResponseWriter, r *http.Request) {
	mode := r.URL.Query().Get("hub.mode")
	token := r.URL.Query().Get("hub.verify_token")
	challenge := r.URL.Query().Get("hub.challenge")

	if mode == "subscribe" && token == w.verifyToken {
		slog.Info("whatsapp: webhook verified")
		rw.WriteHeader(http.StatusOK)
		rw.Write([]byte(challenge))
		return
	}
	slog.Warn("whatsapp: webhook verification failed")
	rw.WriteHeader(http.StatusForbidden)
}

// handleMessage processes incoming webhook events.
func (w *WhatsApp) handleMessage(rw http.ResponseWriter, r *http.Request, handler func(Message) Reply) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		slog.Warn("whatsapp: read webhook body failed", "err", err)
		rw.WriteHeader(http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if w.appSecret != "" {
		signature := r.Header.Get("X-Hub-Signature-256")
		if !w.verifySignature(body, signature) {
			slog.Warn("whatsapp: signature verification failed", "remote", r.RemoteAddr)
			rw.WriteHeader(http.StatusUnauthorized)
			return
		}
	}

	rw.WriteHeader(http.StatusOK) // always ack quickly

	var payload waWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		slog.Warn("whatsapp: invalid webhook payload", "err", err)
		return
	}

	for _, entry := range payload.Entry {
		for _, change := range entry.Changes {
			if change.Value.Messages == nil {
				continue
			}
			for _, msg := range change.Value.Messages {
				if msg.Type != "text" {
					continue // skip non-text for now
				}

				inMsg := Message{
					ChannelType: "whatsapp",
					ChannelID:   change.Value.Metadata.PhoneNumberID,
					UserID:      msg.From,
					UserName:    w.findContactName(change.Value.Contacts, msg.From),
					Content:     msg.Text.Body,
					Extra: map[string]string{
						"message_id": msg.ID,
						"timestamp":  msg.Timestamp,
					},
				}

				reply := handler(inMsg)
				if !IsEmptyReply(reply) {
					if err := w.Send(r.Context(), msg.From, reply); err != nil {
						slog.Warn("whatsapp: reply failed", "to", msg.From, "err", err)
					}
				}
			}
		}
	}
}

func (w *WhatsApp) verifySignature(body []byte, signature string) bool {
	if w.appSecret == "" || signature == "" {
		return false
	}
	const prefix = "sha256="
	if !strings.HasPrefix(signature, prefix) {
		return false
	}
	mac := hmac.New(sha256.New, []byte(w.appSecret))
	if _, err := mac.Write(body); err != nil {
		return false
	}
	expected := mac.Sum(nil)
	actual, err := decodeHexString(strings.TrimPrefix(signature, prefix))
	if err != nil {
		return false
	}
	return hmac.Equal(expected, actual)
}

func decodeHexString(s string) ([]byte, error) {
	if len(s)%2 != 0 {
		return nil, fmt.Errorf("invalid hex length")
	}
	out := make([]byte, len(s)/2)
	for i := 0; i < len(out); i++ {
		hi, ok := fromHexNibble(s[2*i])
		if !ok {
			return nil, fmt.Errorf("invalid hex")
		}
		lo, ok := fromHexNibble(s[2*i+1])
		if !ok {
			return nil, fmt.Errorf("invalid hex")
		}
		out[i] = hi<<4 | lo
	}
	return out, nil
}

func fromHexNibble(c byte) (byte, bool) {
	switch {
	case '0' <= c && c <= '9':
		return c - '0', true
	case 'a' <= c && c <= 'f':
		return c - 'a' + 10, true
	case 'A' <= c && c <= 'F':
		return c - 'A' + 10, true
	default:
		return 0, false
	}
}

func (w *WhatsApp) findContactName(contacts []waContact, waID string) string {
	for _, c := range contacts {
		if c.WaID == waID {
			return c.Profile.Name
		}
	}
	return waID
}

// ──────────────────────────────────────────────
// WhatsApp Cloud API webhook types
// ──────────────────────────────────────────────

type waWebhookPayload struct {
	Object string    `json:"object"`
	Entry  []waEntry `json:"entry"`
}

type waEntry struct {
	ID      string     `json:"id"`
	Changes []waChange `json:"changes"`
}

type waChange struct {
	Value waValue `json:"value"`
	Field string  `json:"field"`
}

type waValue struct {
	MessagingProduct string      `json:"messaging_product"`
	Metadata         waMetadata  `json:"metadata"`
	Contacts         []waContact `json:"contacts"`
	Messages         []waMessage `json:"messages"`
}

type waMetadata struct {
	DisplayPhoneNumber string `json:"display_phone_number"`
	PhoneNumberID      string `json:"phone_number_id"`
}

type waContact struct {
	Profile struct {
		Name string `json:"name"`
	} `json:"profile"`
	WaID string `json:"wa_id"`
}

type waMessage struct {
	From      string `json:"from"`
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`
	Text      struct {
		Body string `json:"body"`
	} `json:"text"`
}
