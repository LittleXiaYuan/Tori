package channel

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestKookType(t *testing.T) {
	k := NewKook(KookConfig{Token: "test-token"})
	if k.Type() != "kook" {
		t.Fatalf("expected type 'kook', got %q", k.Type())
	}
}

func TestKookDefaults(t *testing.T) {
	k := NewKook(KookConfig{Token: "token"})
	if k.apiBase != kookAPIBase {
		t.Errorf("apiBase should be %s, got %s", kookAPIBase, k.apiBase)
	}
	if k.token != "token" {
		t.Errorf("token mismatch")
	}
}

func TestKookCustomConfig(t *testing.T) {
	k := NewKook(KookConfig{Token: "tok", APIBase: "https://custom.api"})
	if k.apiBase != "https://custom.api" {
		t.Errorf("custom apiBase should be https://custom.api, got %s", k.apiBase)
	}
}

func TestKookGetMe(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user/me" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		auth := r.Header.Get("Authorization")
		if auth != "Bot my-token" {
			t.Errorf("auth header should be 'Bot my-token', got %q", auth)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"code":0,"data":{"id":"bot-123","username":"TestBot"}}`))
	}))
	defer mock.Close()

	k := NewKook(KookConfig{Token: "my-token", APIBase: mock.URL})
	id, err := k.getMe(context.Background())
	if err != nil {
		t.Fatalf("getMe failed: %v", err)
	}
	if id != "bot-123" {
		t.Errorf("bot ID should be bot-123, got %q", id)
	}
}

func TestKookGetGateway(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/gateway/index") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"code":0,"data":{"url":"wss://mock-gateway.example.com"}}`))
	}))
	defer mock.Close()

	k := NewKook(KookConfig{Token: "tok", APIBase: mock.URL})
	url, err := k.getGateway(context.Background())
	if err != nil {
		t.Fatalf("getGateway failed: %v", err)
	}
	if url != "wss://mock-gateway.example.com" {
		t.Errorf("gateway URL mismatch: %q", url)
	}
}

func TestKookGetGatewayError(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"code":40100,"message":"unauthorized"}`))
	}))
	defer mock.Close()

	k := NewKook(KookConfig{Token: "bad", APIBase: mock.URL})
	_, err := k.getGateway(context.Background())
	if err == nil {
		t.Fatal("expected error for bad gateway response")
	}
}

func TestKookSendMessage(t *testing.T) {
	var receivedPath string
	var receivedBody []byte
	var receivedAuth string

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		receivedAuth = r.Header.Get("Authorization")
		receivedBody, _ = io.ReadAll(r.Body)
		w.Write([]byte(`{"code":0}`))
	}))
	defer mock.Close()

	k := NewKook(KookConfig{Token: "send-token", APIBase: mock.URL})

	err := k.Send(context.Background(), "channel-456", Reply{Content: "Hello Kook!"})
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if receivedPath != "/message/create" {
		t.Errorf("path should be /message/create, got %s", receivedPath)
	}
	if receivedAuth != "Bot send-token" {
		t.Errorf("auth should be 'Bot send-token', got %q", receivedAuth)
	}

	var payload map[string]any
	json.Unmarshal(receivedBody, &payload)
	if payload["target_id"] != "channel-456" {
		t.Errorf("target_id mismatch: %v", payload["target_id"])
	}
	if payload["content"] != "Hello Kook!" {
		t.Errorf("content mismatch: %v", payload["content"])
	}
	if payload["type"] != float64(1) {
		t.Errorf("type should be 1, got %v", payload["type"])
	}
}

func TestKookSendDirectMessage(t *testing.T) {
	var receivedPath string

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.Write([]byte(`{"code":0}`))
	}))
	defer mock.Close()

	k := NewKook(KookConfig{Token: "tok", APIBase: mock.URL})
	err := k.sendDirectMessage(context.Background(), "user-789", "DM!")
	if err != nil {
		t.Fatalf("sendDirectMessage failed: %v", err)
	}
	if receivedPath != "/direct-message/create" {
		t.Errorf("path should be /direct-message/create, got %s", receivedPath)
	}
}

func TestKookAPIError(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"code":40003,"message":"channel not found"}`))
	}))
	defer mock.Close()

	k := NewKook(KookConfig{Token: "tok", APIBase: mock.URL})
	err := k.Send(context.Background(), "bad-channel", Reply{Content: "test"})
	if err == nil {
		t.Fatal("expected error for API error response")
	}
	if !strings.Contains(err.Error(), "40003") {
		t.Errorf("error should contain error code: %v", err)
	}
}

func TestKookWebhookChallenge(t *testing.T) {
	k := NewKook(KookConfig{Token: "tok"})

	challenge := kookWebhookEvent{
		S: 0,
		D: kookWebhookData{
			ChannelType: "WEBHOOK_CHALLENGE",
			Challenge:   "my-challenge-token",
		},
	}
	body, _ := json.Marshal(challenge)

	req := httptest.NewRequest(http.MethodPost, "/kook/callback", strings.NewReader(string(body)))
	w := httptest.NewRecorder()

	k.KookWebhookHandler()(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["challenge"] != "my-challenge-token" {
		t.Errorf("should echo challenge token, got %q", resp["challenge"])
	}
}

func TestKookWebhookMethodNotAllowed(t *testing.T) {
	k := NewKook(KookConfig{Token: "tok"})

	req := httptest.NewRequest(http.MethodGet, "/kook/callback", nil)
	w := httptest.NewRecorder()

	k.KookWebhookHandler()(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestKookProcessWebhookEvent(t *testing.T) {
	k := NewKook(KookConfig{Token: "tok"})
	k.botID = "bot-id"

	tests := []struct {
		name    string
		event   kookWebhookEvent
		wantNil bool
		content string
	}{
		{
			name: "text_message",
			event: kookWebhookEvent{
				D: kookWebhookData{
					ChannelType: "GROUP",
					Type:        1,
					TargetID:    "ch-001",
					AuthorID:    "user-1",
					Content:     "Hello!",
					MsgID:       "msg-1",
				},
			},
			content: "Hello!",
		},
		{
			name: "image_message",
			event: kookWebhookEvent{
				D: kookWebhookData{
					ChannelType: "GROUP",
					Type:        2,
					TargetID:    "ch-001",
					AuthorID:    "user-2",
					Content:     "https://img.kookapp.cn/image.png",
					MsgID:       "msg-2",
				},
			},
			content: "[图片消息] https://img.kookapp.cn/image.png",
		},
		{
			name: "kmarkdown",
			event: kookWebhookEvent{
				D: kookWebhookData{
					ChannelType: "GROUP",
					Type:        9,
					TargetID:    "ch-001",
					AuthorID:    "user-3",
					Content:     "**bold** text",
					MsgID:       "msg-3",
				},
			},
			content: "**bold** text",
		},
		{
			name: "bot_self_message",
			event: kookWebhookEvent{
				D: kookWebhookData{
					ChannelType: "GROUP",
					Type:        1,
					TargetID:    "ch-001",
					AuthorID:    "bot-id",
					Content:     "I said this",
					MsgID:       "msg-4",
				},
			},
			wantNil: true,
		},
		{
			name: "system_message",
			event: kookWebhookEvent{
				D: kookWebhookData{
					ChannelType: "GROUP",
					Type:        255,
					TargetID:    "ch-001",
					AuthorID:    "system",
					Content:     "user joined",
					MsgID:       "msg-5",
				},
			},
			wantNil: true,
		},
		{
			name: "card_message",
			event: kookWebhookEvent{
				D: kookWebhookData{
					ChannelType: "PERSON",
					Type:        10,
					TargetID:    "user-5",
					AuthorID:    "user-5",
					Content:     "[{\"type\":\"card\"}]",
					MsgID:       "msg-6",
				},
			},
			content: "[卡片消息]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, _ := json.Marshal(tt.event)
			msg := k.ProcessWebhookEvent(data)
			if tt.wantNil {
				if msg != nil {
					t.Errorf("expected nil message, got %+v", msg)
				}
				return
			}
			if msg == nil {
				t.Fatal("expected non-nil message")
			}
			if msg.Content != tt.content {
				t.Errorf("content = %q, want %q", msg.Content, tt.content)
			}
			if msg.ChannelType != "kook" {
				t.Errorf("channelType should be kook, got %q", msg.ChannelType)
			}
		})
	}
}

func TestKookMsgItemToMessage(t *testing.T) {
	item := kookMsgItem{
		id:          "msg-001",
		channelID:   "ch-001",
		channelType: "GROUP",
		authorID:    "user-1",
		authorName:  "TestUser",
		content:     "Hello",
		msgType:     1,
	}

	msg := item.toMessage()
	if msg.ChannelType != "kook" {
		t.Errorf("channelType should be kook, got %q", msg.ChannelType)
	}
	if msg.Content != "Hello" {
		t.Errorf("content should be Hello, got %q", msg.Content)
	}
	if msg.UserName != "TestUser" {
		t.Errorf("userName should be TestUser, got %q", msg.UserName)
	}

	// Image type
	item.msgType = 2
	item.content = "https://img.example.com/pic.png"
	msg = item.toMessage()
	if !strings.HasPrefix(msg.Content, "[图片消息]") {
		t.Errorf("image message should start with [图片消息], got %q", msg.Content)
	}
}

func TestKookSplitMessage(t *testing.T) {
	short := "Short"
	parts := splitKookMessage(short)
	if len(parts) != 1 {
		t.Errorf("short message should be 1 part, got %d", len(parts))
	}

	long := strings.Repeat("测试。", 3000)
	parts = splitKookMessage(long)
	if len(parts) < 2 {
		t.Errorf("long message should split, got %d parts", len(parts))
	}
}

func TestKookSendLongMessage(t *testing.T) {
	callCount := 0
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Write([]byte(`{"code":0}`))
	}))
	defer mock.Close()

	k := NewKook(KookConfig{Token: "tok", APIBase: mock.URL})
	long := strings.Repeat("Kook测试。", 2000)
	err := k.Send(context.Background(), "ch-001", Reply{Content: long})
	if err != nil {
		t.Fatalf("Send long failed: %v", err)
	}
	if callCount < 2 {
		t.Errorf("long message should require multiple API calls, got %d", callCount)
	}
}
