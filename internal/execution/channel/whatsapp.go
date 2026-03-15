package channel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// WhatsApp implements the Channel interface using the WhatsApp Cloud API.
// Requires a Meta Business account with WhatsApp Business API access.
type WhatsApp struct {
	phoneNumberID string // WhatsApp phone number ID
	token         string // permanent or temporary access token
	verifyToken   string // webhook verify token
	client        *http.Client
	webhookPath   string // e.g. "/webhook/whatsapp"
}

// WhatsAppConfig holds configuration for the WhatsApp channel.
type WhatsAppConfig struct {
	PhoneNumberID string `json:"phone_number_id"`
	AccessToken   string `json:"access_token"`
	VerifyToken   string `json:"verify_token"`
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

	go func() {
		<-ctx.Done()
		server.Shutdown(context.Background())
	}()

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
			"body": reply.Content,
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
	rw.WriteHeader(http.StatusOK) // always ack quickly

	var payload waWebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
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
				if reply.Content != "" {
					if err := w.Send(r.Context(), msg.From, reply); err != nil {
						slog.Warn("whatsapp: reply failed", "to", msg.From, "err", err)
					}
				}
			}
		}
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
