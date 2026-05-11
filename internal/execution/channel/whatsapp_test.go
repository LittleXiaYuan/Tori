package channel

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func computeWhatsAppSignature(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestWhatsAppVerifySignature(t *testing.T) {
	w := NewWhatsApp(WhatsAppConfig{AppSecret: "app-secret"})
	body := []byte(`{"object":"whatsapp_business_account"}`)
	sig := computeWhatsAppSignature("app-secret", body)

	if !w.verifySignature(body, sig) {
		t.Fatal("valid signature should pass")
	}
	if w.verifySignature(body, "sha256=bad") {
		t.Fatal("invalid signature should fail")
	}
	if w.verifySignature(body, "") {
		t.Fatal("empty signature should fail")
	}
}

func TestWhatsAppHandleMessageRejectsBadSignature(t *testing.T) {
	w := NewWhatsApp(WhatsAppConfig{AppSecret: "app-secret"})
	body := []byte(`{"entry":[{"changes":[{"value":{"messages":[{"from":"123","id":"m1","timestamp":"1","type":"text","text":{"body":"hi"}}]}}]}]}`)
	req := httptest.NewRequest(http.MethodPost, "/webhook/whatsapp", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", "sha256=bad")
	rr := httptest.NewRecorder()

	called := false
	w.handleMessage(rr, req, func(Message) Reply {
		called = true
		return Reply{}
	})

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
	if called {
		t.Fatal("handler should not be called on bad signature")
	}
}

func TestWhatsAppHandleMessageAcceptsValidSignature(t *testing.T) {
	w := NewWhatsApp(WhatsAppConfig{AppSecret: "app-secret"})
	payload := waWebhookPayload{
		Object: "whatsapp_business_account",
		Entry: []waEntry{
			{
				ID: "entry1",
				Changes: []waChange{
					{
						Field: "messages",
						Value: waValue{
							MessagingProduct: "whatsapp",
							Metadata: waMetadata{
								PhoneNumberID: "phone-1",
							},
							Contacts: []waContact{
								{
									Profile: struct {
										Name string `json:"name"`
									}{Name: "Alice"},
									WaID: "123",
								},
							},
							Messages: []waMessage{
								{
									From:      "123",
									ID:        "m1",
									Timestamp: "1",
									Type:      "text",
									Text: struct {
										Body string `json:"body"`
									}{Body: "hello"},
								},
							},
						},
					},
				},
			},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/webhook/whatsapp", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", computeWhatsAppSignature("app-secret", body))
	rr := httptest.NewRecorder()

	var got Message
	w.handleMessage(rr, req, func(m Message) Reply {
		got = m
		return Reply{}
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if got.Content != "hello" {
		t.Fatalf("content = %q, want hello", got.Content)
	}
	if got.ChannelType != "whatsapp" {
		t.Fatalf("channel type = %q, want whatsapp", got.ChannelType)
	}
	if got.UserName != "Alice" {
		t.Fatalf("user name = %q, want Alice", got.UserName)
	}
	if got.ChannelID != "phone-1" {
		t.Fatalf("channel id = %q, want phone-1", got.ChannelID)
	}
	if got.Extra["message_id"] != "m1" {
		t.Fatalf("message_id = %q, want m1", got.Extra["message_id"])
	}
}

func TestWhatsAppHandleMessageSkipsVerificationWithoutSecret(t *testing.T) {
	w := NewWhatsApp(WhatsAppConfig{})
	body := []byte(`{"entry":[]}`)
	req := httptest.NewRequest(http.MethodPost, "/webhook/whatsapp", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	called := false
	w.handleMessage(rr, req, func(Message) Reply {
		called = true
		return Reply{}
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if called {
		t.Fatal("empty payload should not call handler")
	}
}
