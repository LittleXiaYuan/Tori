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

func TestSatoriType(t *testing.T) {
	s := NewSatori(SatoriConfig{Endpoint: "http://localhost:5140"})
	if s.Type() != "satori" {
		t.Fatalf("expected type 'satori', got %q", s.Type())
	}
}

func TestSatoriDefaults(t *testing.T) {
	s := NewSatori(SatoriConfig{})
	if s.endpoint != "http://localhost:5140" {
		t.Errorf("default endpoint should be http://localhost:5140, got %s", s.endpoint)
	}
	if s.port != satoriDefaultPort {
		t.Errorf("default port should be %s, got %s", satoriDefaultPort, s.port)
	}
	if s.bindAddr != "0.0.0.0" {
		t.Errorf("default bindAddr should be 0.0.0.0, got %s", s.bindAddr)
	}
}

func TestSatoriCustomConfig(t *testing.T) {
	s := NewSatori(SatoriConfig{
		Endpoint: "https://custom.satori",
		Token:    "my-token",
		Port:     "8888",
		BindAddr: "127.0.0.1",
		Platform: "onebot",
		SelfID:   "bot-1",
	})
	if s.endpoint != "https://custom.satori" {
		t.Errorf("endpoint mismatch")
	}
	if s.token != "my-token" {
		t.Errorf("token mismatch")
	}
	if s.port != "8888" {
		t.Errorf("port mismatch")
	}
	if s.platform != "onebot" {
		t.Errorf("platform mismatch")
	}
	if s.selfID != "bot-1" {
		t.Errorf("selfID mismatch")
	}
}

func TestSatoriEventHandlerTextMessage(t *testing.T) {
	s := NewSatori(SatoriConfig{Token: "test-token"})

	event := satoriEvent{
		Type:     "message-created",
		Platform: "telegram",
		SelfID:   "bot-100",
		Channel:  satoriObj{ID: "ch-001"},
		User:     satoriUser{ID: "user-1", Name: "TestUser"},
		Message:  satoriMsg{ID: "msg-1", Content: "Hello Satori"},
	}
	body, _ := json.Marshal(event)

	req := httptest.NewRequest(http.MethodPost, "/satori/events", strings.NewReader(string(body)))
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()

	s.eventHandler()(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	select {
	case msg := <-s.msgCh:
		if msg.Content != "Hello Satori" {
			t.Errorf("content should be 'Hello Satori', got %q", msg.Content)
		}
		if msg.UserID != "user-1" {
			t.Errorf("userID should be user-1, got %q", msg.UserID)
		}
		if msg.ChannelType != "satori" {
			t.Errorf("channelType should be satori, got %q", msg.ChannelType)
		}
		if msg.Extra["platform"] != "telegram" {
			t.Errorf("platform should be telegram, got %q", msg.Extra["platform"])
		}
	default:
		t.Fatal("no message received in channel")
	}
}

func TestSatoriEventHandlerArrayEvents(t *testing.T) {
	s := NewSatori(SatoriConfig{})

	events := []satoriEvent{
		{
			Type:    "message-created",
			Channel: satoriObj{ID: "ch-1"},
			User:    satoriUser{ID: "u1"},
			Message: satoriMsg{Content: "First"},
		},
		{
			Type:    "message-created",
			Channel: satoriObj{ID: "ch-1"},
			User:    satoriUser{ID: "u2"},
			Message: satoriMsg{Content: "Second"},
		},
	}
	body, _ := json.Marshal(events)

	req := httptest.NewRequest(http.MethodPost, "/satori/events", strings.NewReader(string(body)))
	w := httptest.NewRecorder()

	s.eventHandler()(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	msg1 := <-s.msgCh
	msg2 := <-s.msgCh
	if msg1.Content != "First" {
		t.Errorf("first message content mismatch: %q", msg1.Content)
	}
	if msg2.Content != "Second" {
		t.Errorf("second message content mismatch: %q", msg2.Content)
	}
}

func TestSatoriEventHandlerUnauthorized(t *testing.T) {
	s := NewSatori(SatoriConfig{Token: "secret-token"})

	body := `{"type":"message-created","message":{"content":"hi"}}`
	req := httptest.NewRequest(http.MethodPost, "/satori/events", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer wrong-token")
	w := httptest.NewRecorder()

	s.eventHandler()(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestSatoriEventHandlerNoTokenRequired(t *testing.T) {
	s := NewSatori(SatoriConfig{}) // no token

	body := `{"type":"message-created","channel":{"id":"ch"},"user":{"id":"u"},"message":{"content":"open"}}`
	req := httptest.NewRequest(http.MethodPost, "/satori/events", strings.NewReader(body))
	w := httptest.NewRecorder()

	s.eventHandler()(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 without token, got %d", w.Code)
	}
}

func TestSatoriEventHandlerMethodNotAllowed(t *testing.T) {
	s := NewSatori(SatoriConfig{})

	req := httptest.NewRequest(http.MethodGet, "/satori/events", nil)
	w := httptest.NewRecorder()

	s.eventHandler()(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestSatoriEventSelfFilter(t *testing.T) {
	s := NewSatori(SatoriConfig{SelfID: "bot-self"})

	event := satoriEvent{
		Type:    "message-created",
		Channel: satoriObj{ID: "ch"},
		User:    satoriUser{ID: "bot-self"},
		Message: satoriMsg{Content: "self message"},
	}
	body, _ := json.Marshal(event)

	req := httptest.NewRequest(http.MethodPost, "/satori/events", strings.NewReader(string(body)))
	w := httptest.NewRecorder()

	s.eventHandler()(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Self message should be filtered out
	select {
	case msg := <-s.msgCh:
		t.Errorf("self message should be filtered, got %+v", msg)
	default:
		// expected
	}
}

func TestSatoriEventPlatformFilter(t *testing.T) {
	s := NewSatori(SatoriConfig{Platform: "onebot"})

	// Event from different platform should be filtered
	event := satoriEvent{
		Type:     "message-created",
		Platform: "telegram",
		Channel:  satoriObj{ID: "ch"},
		User:     satoriUser{ID: "u"},
		Message:  satoriMsg{Content: "wrong platform"},
	}
	body, _ := json.Marshal(event)

	req := httptest.NewRequest(http.MethodPost, "/satori/events", strings.NewReader(string(body)))
	w := httptest.NewRecorder()

	s.eventHandler()(w, req)

	select {
	case msg := <-s.msgCh:
		t.Errorf("platform-filtered message should not arrive, got %+v", msg)
	default:
		// expected
	}

	// Event from correct platform should pass
	event.Platform = "onebot"
	body, _ = json.Marshal(event)
	req = httptest.NewRequest(http.MethodPost, "/satori/events", strings.NewReader(string(body)))
	w = httptest.NewRecorder()

	s.eventHandler()(w, req)

	select {
	case msg := <-s.msgCh:
		if msg.Extra["platform"] != "onebot" {
			t.Errorf("platform should be onebot, got %q", msg.Extra["platform"])
		}
	default:
		t.Fatal("expected message from matching platform")
	}
}

func TestSatoriExtractContentElements(t *testing.T) {
	s := NewSatori(SatoriConfig{})

	tests := []struct {
		name     string
		event    satoriEvent
		expected string
	}{
		{
			name:     "plain_text",
			event:    satoriEvent{Message: satoriMsg{Content: "Hello"}},
			expected: "Hello",
		},
		{
			name: "text_elements",
			event: satoriEvent{
				Message: satoriMsg{
					Elements: []satoriElement{
						{Type: "text", Attrs: satoriAttrs{Content: "Hello "}},
						{Type: "at", Attrs: satoriAttrs{Name: "Bot"}},
						{Type: "text", Attrs: satoriAttrs{Content: " world"}},
					},
				},
			},
			expected: "Hello @Bot world",
		},
		{
			name: "image_element",
			event: satoriEvent{
				Message: satoriMsg{
					Elements: []satoriElement{
						{Type: "img", Attrs: satoriAttrs{Src: "https://example.com/pic.png"}},
					},
				},
			},
			expected: "[图片]",
		},
		{
			name: "audio_element",
			event: satoriEvent{
				Message: satoriMsg{
					Elements: []satoriElement{
						{Type: "audio"},
					},
				},
			},
			expected: "[音频]",
		},
		{
			name: "video_element",
			event: satoriEvent{
				Message: satoriMsg{
					Elements: []satoriElement{
						{Type: "video"},
					},
				},
			},
			expected: "[视频]",
		},
		{
			name: "file_element",
			event: satoriEvent{
				Message: satoriMsg{
					Elements: []satoriElement{
						{Type: "file"},
					},
				},
			},
			expected: "[文件]",
		},
		{
			name: "face_element",
			event: satoriEvent{
				Message: satoriMsg{
					Elements: []satoriElement{
						{Type: "face"},
					},
				},
			},
			expected: "[表情]",
		},
		{
			name: "quote_skipped",
			event: satoriEvent{
				Message: satoriMsg{
					Elements: []satoriElement{
						{Type: "quote", Attrs: satoriAttrs{ID: "msg-old"}},
						{Type: "text", Attrs: satoriAttrs{Content: "reply text"}},
					},
				},
			},
			expected: "reply text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.extractContent(tt.event)
			if got != tt.expected {
				t.Errorf("extractContent(%q) = %q, want %q", tt.name, got, tt.expected)
			}
		})
	}
}

func TestSatoriSendMessage(t *testing.T) {
	var receivedPath string
	var receivedBody []byte
	var receivedAuth string
	var receivedPlatform string

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		receivedAuth = r.Header.Get("Authorization")
		receivedPlatform = r.Header.Get("X-Platform")
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer mock.Close()

	s := NewSatori(SatoriConfig{
		Endpoint: mock.URL,
		Token:    "api-token",
		Platform: "onebot",
		SelfID:   "bot-1",
	})

	err := s.Send(context.Background(), "ch-001", Reply{Content: "Hello!"})
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if receivedPath != "/v1/message.create" {
		t.Errorf("path should be /v1/message.create, got %s", receivedPath)
	}
	if receivedAuth != "Bearer api-token" {
		t.Errorf("auth should be 'Bearer api-token', got %q", receivedAuth)
	}
	if receivedPlatform != "onebot" {
		t.Errorf("X-Platform should be onebot, got %q", receivedPlatform)
	}

	var payload satoriMessageCreateReq
	json.Unmarshal(receivedBody, &payload)
	if payload.ChannelID != "ch-001" {
		t.Errorf("channelID should be ch-001, got %q", payload.ChannelID)
	}
	if payload.Content != "Hello!" {
		t.Errorf("content should be Hello!, got %q", payload.Content)
	}
}

func TestSatoriSendAPIError(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"message":"channel not found"}`))
	}))
	defer mock.Close()

	s := NewSatori(SatoriConfig{Endpoint: mock.URL})
	err := s.Send(context.Background(), "bad-ch", Reply{Content: "test"})
	if err == nil {
		t.Fatal("expected error for 400 response")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("error should mention 400: %v", err)
	}
}

func TestSatoriSplitMessage(t *testing.T) {
	short := "Short"
	parts := splitSatoriMessage(short)
	if len(parts) != 1 {
		t.Errorf("short message should be 1 part, got %d", len(parts))
	}

	long := strings.Repeat("测试。", 3000)
	parts = splitSatoriMessage(long)
	if len(parts) < 2 {
		t.Errorf("long message should split, got %d parts", len(parts))
	}
	for _, p := range parts {
		if len([]rune(p)) > satoriMaxTextLen {
			t.Errorf("part exceeds max length: %d runes", len([]rune(p)))
		}
	}
}

func TestSatoriSendLongMessage(t *testing.T) {
	callCount := 0
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer mock.Close()

	s := NewSatori(SatoriConfig{Endpoint: mock.URL})
	long := strings.Repeat("Satori测试。", 2000)
	err := s.Send(context.Background(), "ch", Reply{Content: long})
	if err != nil {
		t.Fatalf("Send long failed: %v", err)
	}
	if callCount < 2 {
		t.Errorf("long message should require multiple API calls, got %d", callCount)
	}
}
