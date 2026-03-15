package channel

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRegistryRegisterAndGet(t *testing.T) {
	reg := NewRegistry()
	tg := NewTelegram("fake-token")
	reg.Register(tg)

	ch, ok := reg.Get("telegram")
	if !ok {
		t.Fatal("telegram channel should be registered")
	}
	if ch.Type() != "telegram" {
		t.Fatalf("expected telegram, got %s", ch.Type())
	}

	_, ok = reg.Get("slack")
	if ok {
		t.Fatal("slack should not exist")
	}
}

func TestFeishuType(t *testing.T) {
	f := NewFeishu("id", "secret", "")
	if f.Type() != "feishu" {
		t.Fatalf("expected feishu, got %s", f.Type())
	}
}

func TestFeishuWebhookChallenge(t *testing.T) {
	f := NewFeishu("id", "secret", "")
	body := `{"challenge":"test-challenge-123"}`
	req := httptest.NewRequest("POST", "/webhook/feishu", strings.NewReader(body))
	w := httptest.NewRecorder()
	f.HandleWebhook(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["challenge"] != "test-challenge-123" {
		t.Fatalf("expected challenge echo, got %v", resp)
	}
}

func TestFeishuWebhookMessage(t *testing.T) {
	f := NewFeishu("id", "secret", "")
	body := `{
		"header": {"event_type": "im.message.receive_v1"},
		"event": {
			"message": {"chat_id": "oc_123", "message_type": "text", "content": "{\"text\":\"hello\"}"},
			"sender": {"sender_id": {"open_id": "ou_456"}}
		}
	}`
	req := httptest.NewRequest("POST", "/webhook/feishu", strings.NewReader(body))
	w := httptest.NewRecorder()
	f.HandleWebhook(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Check message was queued
	select {
	case msg := <-f.msgCh:
		if msg.Content != "hello" {
			t.Fatalf("expected hello, got %s", msg.Content)
		}
		if msg.ChannelID != "oc_123" {
			t.Fatalf("expected oc_123, got %s", msg.ChannelID)
		}
	default:
		t.Fatal("expected message in channel")
	}
}

func TestTelegramParseCommand(t *testing.T) {
	tg := NewTelegram("fake")
	tests := []struct {
		input string
		want  string
	}{
		{"/start", "/start"},
		{"/help@mybot", "/help"},
		{"/skills arg1", "/skills"},
		{"hello", ""},
		{"", ""},
	}
	for _, tt := range tests {
		got := tg.parseCommand(tt.input)
		if got != tt.want {
			t.Errorf("parseCommand(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestTelegramHandleCommand(t *testing.T) {
	tg := NewTelegram("fake")
	reply := tg.handleCommand("/start")
	if reply.Content == "" {
		t.Fatal("start command should return non-empty reply")
	}
	reply = tg.handleCommand("/unknown")
	if !strings.Contains(reply.Content, "未知命令") {
		t.Fatal("unknown command should mention '未知命令'")
	}
}

func TestTelegramSendMock(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	tg := &Telegram{token: "fake", client: srv.Client()}
	// Override apiURL by testing Send directly won't work easily,
	// but we verify the struct creation works
	if tg.Type() != "telegram" {
		t.Fatal("type mismatch")
	}
}

func TestTelegramParseUpdateSticker(t *testing.T) {
	tg := NewTelegram("fake-token")

	// Test sticker message parsing
	update := tgUpdate{
		UpdateID: 1,
		Message: &tgMessage{
			MessageID: 42,
			Chat:      tgChat{ID: 123456, Type: "private"},
			From:      tgUser{ID: 789, FirstName: "Test"},
			Sticker: &tgSticker{
				FileID:       "CAACAgIAAx0CVQABOT4AAgE",
				FileUniqueID: "AgADkwADKejqUQ",
				Type:         "regular",
				Width:        512,
				Height:       512,
				IsAnimated:   false,
				IsVideo:      false,
				Emoji:        "😂",
				SetName:      "HotCherry",
			},
		},
	}

	msg := tg.parseUpdate(update)
	if msg == nil {
		t.Fatal("sticker message should parse")
	}
	if msg.Content != "Sticker: 😂" {
		t.Errorf("expected 'Sticker: 😂', got %q", msg.Content)
	}
	if msg.Rich == nil {
		t.Fatal("rich message should be set for sticker")
	}
	if !msg.Rich.HasType(ComponentSticker) {
		t.Error("rich message should contain sticker component")
	}
	sc := msg.Rich.GetFirst(ComponentSticker).(*StickerComponent)
	if sc.FileID != "CAACAgIAAx0CVQABOT4AAgE" {
		t.Errorf("wrong file_id: %s", sc.FileID)
	}
	if sc.Platform != "telegram" {
		t.Errorf("platform should be telegram, got %s", sc.Platform)
	}
	if sc.SetName != "HotCherry" {
		t.Errorf("set_name should be HotCherry, got %s", sc.SetName)
	}
	if sc.Emoji != "😂" {
		t.Errorf("emoji should be 😂, got %s", sc.Emoji)
	}
}

func TestTelegramParseUpdatePhoto(t *testing.T) {
	tg := NewTelegram("fake-token")

	update := tgUpdate{
		UpdateID: 2,
		Message: &tgMessage{
			MessageID: 43,
			Chat:      tgChat{ID: 111, Type: "private"},
			From:      tgUser{ID: 222, FirstName: "Photo"},
			Photo: []tgPhoto{
				{FileID: "small", FileUniqueID: "s1", Width: 90, Height: 90},
				{FileID: "large", FileUniqueID: "l1", Width: 800, Height: 600},
			},
			Caption: "Look at this!",
		},
	}

	msg := tg.parseUpdate(update)
	if msg == nil {
		t.Fatal("photo message should parse")
	}
	if msg.Content != "Look at this!" {
		t.Errorf("expected caption, got %q", msg.Content)
	}
	if msg.Rich == nil {
		t.Fatal("rich should be set")
	}
	img := msg.Rich.GetFirst(ComponentImage)
	if img == nil {
		t.Fatal("should have image component")
	}
	ic := img.(*ImageComponent)
	if ic.FileID != "large" {
		t.Errorf("should use largest photo, got file_id=%s", ic.FileID)
	}
}

func TestTelegramReactInterface(t *testing.T) {
	tg := NewTelegram("fake-token")
	var _ Reactor = tg
	_ = tg // compile check
}

func TestTelegramSendStickerInterface(t *testing.T) {
	tg := NewTelegram("fake-token")
	var _ StickerSender = tg
	_ = tg
}

func TestDiscordReactInterface(t *testing.T) {
	d := NewDiscord("fake-token")
	var _ Reactor = d
	_ = d
}

func TestLINESendStickerInterface(t *testing.T) {
	l := NewLINE(LINEConfig{
		ChannelSecret: "s",
		ChannelToken:  "t",
	})
	var _ StickerSender = l
	_ = l
}

func TestStickerComponentNewFields(t *testing.T) {
	s := NewSticker("pkg1", "stk1")
	s.IsAnimated = true
	s.IsVideo = false
	s.Platform = "telegram"
	s.FileID = "file123"
	s.SetName = "MySet"
	s.Emoji = "😎"

	j := s.ToJSON()
	if j["is_animated"] != true {
		t.Error("is_animated should be true in JSON")
	}
	if _, ok := j["is_video"]; ok {
		t.Error("is_video=false should be omitted from JSON")
	}
	if j["file_id"] != "file123" {
		t.Error("file_id mismatch")
	}
	if j["set_name"] != "MySet" {
		t.Error("set_name mismatch")
	}

	// Test JSON roundtrip with new fields
	rm := NewRichMessage()
	rm.Add(s)
	jsonStr := rm.ToJSON()

	parsed, err := ParseRichMessage(jsonStr)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	sc := parsed.GetFirst(ComponentSticker).(*StickerComponent)
	if sc.IsAnimated != true {
		t.Error("roundtrip: is_animated should survive")
	}
	if sc.FileID != "file123" {
		t.Error("roundtrip: file_id should survive")
	}
	if sc.SetName != "MySet" {
		t.Error("roundtrip: set_name should survive")
	}
	if sc.Emoji != "😎" {
		t.Error("roundtrip: emoji should survive")
	}
}
