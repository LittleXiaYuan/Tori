package channel

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLINEType(t *testing.T) {
	l := NewLINE(LINEConfig{ChannelSecret: "secret", ChannelToken: "token"})
	if l.Type() != "line" {
		t.Fatalf("expected type 'line', got %q", l.Type())
	}
}

func TestLINEDefaults(t *testing.T) {
	l := NewLINE(LINEConfig{ChannelSecret: "s", ChannelToken: "t"})
	if l.port != "9883" {
		t.Errorf("default port should be 9883, got %s", l.port)
	}
	if l.bindAddr != "0.0.0.0" {
		t.Errorf("default bindAddr should be 0.0.0.0, got %s", l.bindAddr)
	}
	if l.apiBase != lineAPIBase {
		t.Errorf("default apiBase should be %s, got %s", lineAPIBase, l.apiBase)
	}
}

func TestLINECustomConfig(t *testing.T) {
	l := NewLINE(LINEConfig{
		ChannelSecret: "s",
		ChannelToken:  "t",
		Port:          "8080",
		BindAddr:      "127.0.0.1",
		APIBase:       "https://custom.api",
	})
	if l.port != "8080" {
		t.Errorf("port should be 8080, got %s", l.port)
	}
	if l.bindAddr != "127.0.0.1" {
		t.Errorf("bindAddr should be 127.0.0.1, got %s", l.bindAddr)
	}
	if l.apiBase != "https://custom.api" {
		t.Errorf("apiBase should be https://custom.api, got %s", l.apiBase)
	}
}

func computeLINESignature(secret, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func TestLINEVerifySignature(t *testing.T) {
	l := NewLINE(LINEConfig{ChannelSecret: "test-secret", ChannelToken: "token"})

	body := `{"events":[]}`
	sig := computeLINESignature("test-secret", body)

	if !l.verifySignature([]byte(body), sig) {
		t.Error("valid signature should pass verification")
	}
	if l.verifySignature([]byte(body), "invalid-sig") {
		t.Error("invalid signature should fail verification")
	}
	if l.verifySignature([]byte(body), "") {
		t.Error("empty signature should fail verification")
	}
}

func TestLINEVerifySignatureEmptySecret(t *testing.T) {
	l := NewLINE(LINEConfig{ChannelSecret: "", ChannelToken: "token"})
	if l.verifySignature([]byte("body"), "sig") {
		t.Error("empty secret should fail verification")
	}
}

func TestLINEWebhookPOSTTextMessage(t *testing.T) {
	secret := "test-secret-123"
	l := NewLINE(LINEConfig{ChannelSecret: secret, ChannelToken: "token"})

	webhook := lineWebhookBody{
		Events: []lineEvent{
			{
				Type:       "message",
				ReplyToken: "reply-token-abc",
				Source: lineSource{
					Type:   "user",
					UserID: "U123",
				},
				Message: lineMessageBody{
					ID:   "msg001",
					Type: "text",
					Text: "Hello Tori",
				},
			},
		},
	}
	body, _ := json.Marshal(webhook)
	sig := computeLINESignature(secret, string(body))

	req := httptest.NewRequest(http.MethodPost, "/line/callback", strings.NewReader(string(body)))
	req.Header.Set("X-Line-Signature", sig)
	w := httptest.NewRecorder()

	handler := l.webhookHandler()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Check message was queued
	select {
	case msg := <-l.msgCh:
		if msg.Content != "Hello Tori" {
			t.Errorf("content should be 'Hello Tori', got %q", msg.Content)
		}
		if msg.UserID != "U123" {
			t.Errorf("userID should be U123, got %q", msg.UserID)
		}
		if msg.ChannelType != "line" {
			t.Errorf("channelType should be line, got %q", msg.ChannelType)
		}
		if msg.Extra["reply_token"] != "reply-token-abc" {
			t.Errorf("reply_token mismatch: %q", msg.Extra["reply_token"])
		}
	default:
		t.Fatal("no message received in channel")
	}
}

func TestLINEWebhookPOSTGroupMessage(t *testing.T) {
	secret := "group-secret"
	l := NewLINE(LINEConfig{ChannelSecret: secret, ChannelToken: "token"})

	webhook := lineWebhookBody{
		Events: []lineEvent{
			{
				Type:       "message",
				ReplyToken: "rt-group",
				Source: lineSource{
					Type:    "group",
					UserID:  "U456",
					GroupID: "G789",
				},
				Message: lineMessageBody{
					ID:   "msg002",
					Type: "text",
					Text: "Group msg",
				},
			},
		},
	}
	body, _ := json.Marshal(webhook)
	sig := computeLINESignature(secret, string(body))

	req := httptest.NewRequest(http.MethodPost, "/line/callback", strings.NewReader(string(body)))
	req.Header.Set("X-Line-Signature", sig)
	w := httptest.NewRecorder()

	l.webhookHandler()(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	msg := <-l.msgCh
	if msg.ChannelID != "G789" {
		t.Errorf("channelID should be G789 (group), got %q", msg.ChannelID)
	}
	if msg.Extra["chat_type"] != "group" {
		t.Errorf("chat_type should be group, got %q", msg.Extra["chat_type"])
	}
}

func TestLINEWebhookBadSignature(t *testing.T) {
	l := NewLINE(LINEConfig{ChannelSecret: "secret", ChannelToken: "token"})

	body := `{"events":[]}`
	req := httptest.NewRequest(http.MethodPost, "/line/callback", strings.NewReader(body))
	req.Header.Set("X-Line-Signature", "bad-signature")
	w := httptest.NewRecorder()

	l.webhookHandler()(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestLINEWebhookMethodNotAllowed(t *testing.T) {
	l := NewLINE(LINEConfig{ChannelSecret: "secret", ChannelToken: "token"})

	req := httptest.NewRequest(http.MethodGet, "/line/callback", nil)
	w := httptest.NewRecorder()

	l.webhookHandler()(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestLINEExtractContent(t *testing.T) {
	l := NewLINE(LINEConfig{ChannelSecret: "s", ChannelToken: "t"})

	tests := []struct {
		name     string
		event    lineEvent
		expected string
	}{
		{
			name: "text",
			event: lineEvent{
				Message: lineMessageBody{Type: "text", Text: "Hello"},
			},
			expected: "Hello",
		},
		{
			name: "image",
			event: lineEvent{
				Message: lineMessageBody{Type: "image"},
			},
			expected: "[图片消息]",
		},
		{
			name: "video",
			event: lineEvent{
				Message: lineMessageBody{Type: "video"},
			},
			expected: "[视频消息]",
		},
		{
			name: "audio",
			event: lineEvent{
				Message: lineMessageBody{Type: "audio"},
			},
			expected: "[音频消息]",
		},
		{
			name: "file",
			event: lineEvent{
				Message: lineMessageBody{Type: "file", FileName: "doc.pdf"},
			},
			expected: "[文件: doc.pdf]",
		},
		{
			name: "file_no_name",
			event: lineEvent{
				Message: lineMessageBody{Type: "file"},
			},
			expected: "[文件: file]",
		},
		{
			name: "location",
			event: lineEvent{
				Message: lineMessageBody{
					Type:      "location",
					Address:   "Tokyo",
					Latitude:  35.6895,
					Longitude: 139.6917,
				},
			},
			expected: "[位置: Tokyo (35.689500, 139.691700)]",
		},
		{
			name: "sticker",
			event: lineEvent{
				Message: lineMessageBody{
					Type:      "sticker",
					PackageID: "11537",
					StickerID: "52002734",
				},
			},
			expected: "[贴图: packageId=11537, stickerId=52002734]",
		},
		{
			name: "unknown",
			event: lineEvent{
				Message: lineMessageBody{Type: "unknown_type"},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := l.extractContent(tt.event)
			if got != tt.expected {
				t.Errorf("extractContent(%q) = %q, want %q", tt.name, got, tt.expected)
			}
		})
	}
}

func TestLINEPostbackEvent(t *testing.T) {
	secret := "postback-secret"
	l := NewLINE(LINEConfig{ChannelSecret: secret, ChannelToken: "token"})

	webhook := lineWebhookBody{
		Events: []lineEvent{
			{
				Type:       "postback",
				ReplyToken: "rt-pb",
				Source: lineSource{
					Type:   "user",
					UserID: "U999",
				},
				Postback: linePostback{Data: "action=buy&id=123"},
			},
		},
	}
	body, _ := json.Marshal(webhook)
	sig := computeLINESignature(secret, string(body))

	req := httptest.NewRequest(http.MethodPost, "/line/callback", strings.NewReader(string(body)))
	req.Header.Set("X-Line-Signature", sig)
	w := httptest.NewRecorder()

	l.webhookHandler()(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	msg := <-l.msgCh
	if msg.Content != "action=buy&id=123" {
		t.Errorf("postback content should be 'action=buy&id=123', got %q", msg.Content)
	}
	if msg.Extra["event_type"] != "postback" {
		t.Errorf("event_type should be postback, got %q", msg.Extra["event_type"])
	}
}

func TestLINESendWithMockAPI(t *testing.T) {
	var received []byte
	var authHeader string

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		received, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer mock.Close()

	l := NewLINE(LINEConfig{
		ChannelSecret: "s",
		ChannelToken:  "my-token-123",
		APIBase:       mock.URL,
	})

	err := l.Send(context.Background(), "U123", Reply{Content: "Hello LINE!"})
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if authHeader != "Bearer my-token-123" {
		t.Errorf("auth header should be 'Bearer my-token-123', got %q", authHeader)
	}

	var push struct {
		To       string            `json:"to"`
		Messages []json.RawMessage `json:"messages"`
	}
	if err := json.Unmarshal(received, &push); err != nil {
		t.Fatalf("failed to parse push request: %v", err)
	}
	if push.To != "U123" {
		t.Errorf("push.To should be U123, got %q", push.To)
	}
	if len(push.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(push.Messages))
	}
	var msg lineTextMessage
	if err := json.Unmarshal(push.Messages[0], &msg); err != nil {
		t.Fatalf("failed to parse message: %v", err)
	}
	if msg.Text != "Hello LINE!" {
		t.Errorf("message text mismatch: %q", msg.Text)
	}
}

func TestLINEReplyWithMockAPI(t *testing.T) {
	var receivedPath string
	var received []byte

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		received, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer mock.Close()

	l := NewLINE(LINEConfig{
		ChannelSecret: "s",
		ChannelToken:  "token",
		APIBase:       mock.URL,
	})

	err := l.replyMessage(context.Background(), "reply-token-xyz", "Hi!")
	if err != nil {
		t.Fatalf("replyMessage failed: %v", err)
	}

	if receivedPath != "/message/reply" {
		t.Errorf("path should be /message/reply, got %s", receivedPath)
	}

	var reply lineReplyRequest
	json.Unmarshal(received, &reply)
	if reply.ReplyToken != "reply-token-xyz" {
		t.Errorf("replyToken mismatch: %q", reply.ReplyToken)
	}
}

func TestLINESplitMessage(t *testing.T) {
	short := "Short message"
	parts := splitLINEMessage(short)
	if len(parts) != 1 {
		t.Errorf("short message should be 1 part, got %d", len(parts))
	}

	// Long message
	long := strings.Repeat("测试。", 3000) // 6000 chars with periods
	parts = splitLINEMessage(long)
	if len(parts) < 2 {
		t.Errorf("long message should be split into multiple parts, got %d", len(parts))
	}
	for _, p := range parts {
		if len([]rune(p)) > lineMaxTextLen {
			t.Errorf("part exceeds max length: %d runes", len([]rune(p)))
		}
	}
}

func TestLINESendLongMessage(t *testing.T) {
	callCount := 0
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		body, _ := io.ReadAll(r.Body)
		var push struct {
			To       string            `json:"to"`
			Messages []json.RawMessage `json:"messages"`
		}
		json.Unmarshal(body, &push)
		// LINE allows max 5 messages per push
		if len(push.Messages) > lineMaxMessages {
			t.Errorf("too many messages: %d (max %d)", len(push.Messages), lineMaxMessages)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer mock.Close()

	l := NewLINE(LINEConfig{
		ChannelSecret: "s",
		ChannelToken:  "t",
		APIBase:       mock.URL,
	})

	long := strings.Repeat("LINE test。", 2000)
	err := l.Send(context.Background(), "U456", Reply{Content: long})
	if err != nil {
		t.Fatalf("Send long message failed: %v", err)
	}

	if callCount != 1 {
		t.Errorf("expected 1 API call, got %d", callCount)
	}
}

func TestLINEAPIErrorHandling(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"message":"Invalid reply token"}`))
	}))
	defer mock.Close()

	l := NewLINE(LINEConfig{
		ChannelSecret: "s",
		ChannelToken:  "t",
		APIBase:       mock.URL,
	})

	err := l.Send(context.Background(), "U123", Reply{Content: "test"})
	if err == nil {
		t.Fatal("expected error for 400 response")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("error should mention status 400: %v", err)
	}
}
