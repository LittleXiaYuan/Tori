package channel

import (
	"context"
	"sync"
	"testing"
)

// mockChannel records the last Send call.
type mockChannel struct {
	typ      string
	mu       sync.Mutex
	lastReply Reply
	lastTarget string
}

func (m *mockChannel) Type() string { return m.typ }
func (m *mockChannel) Start(_ context.Context, _ func(Message) Reply) error { return nil }
func (m *mockChannel) Send(_ context.Context, target string, reply Reply) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastTarget = target
	m.lastReply = reply
	return nil
}

func TestCardDispatcher_Feishu(t *testing.T) {
	ch := &mockChannel{typ: "feishu"}
	d := NewCardDispatcher(map[string]Channel{"feishu": ch})

	err := d.SendCard(context.Background(), "feishu", "chat-1",
		`{"card":"json"}`, "**md**", "plain")
	if err != nil {
		t.Fatal(err)
	}
	if ch.lastReply.Format != "card" {
		t.Fatalf("expected card format, got %s", ch.lastReply.Format)
	}
	if ch.lastReply.Content != `{"card":"json"}` {
		t.Fatalf("expected card JSON, got %s", ch.lastReply.Content)
	}
}

func TestCardDispatcher_Telegram(t *testing.T) {
	ch := &mockChannel{typ: "telegram"}
	d := NewCardDispatcher(map[string]Channel{"telegram": ch})

	err := d.SendCard(context.Background(), "telegram", "chat-2",
		`{"card":"json"}`, "**bold text**", "plain")
	if err != nil {
		t.Fatal(err)
	}
	if ch.lastReply.Format != "markdown" {
		t.Fatalf("expected markdown, got %s", ch.lastReply.Format)
	}
	if ch.lastReply.Content != "**bold text**" {
		t.Fatalf("expected markdown content, got %s", ch.lastReply.Content)
	}
}

func TestCardDispatcher_Dingtalk(t *testing.T) {
	ch := &mockChannel{typ: "dingtalk"}
	d := NewCardDispatcher(map[string]Channel{"dingtalk": ch})

	err := d.SendCard(context.Background(), "dingtalk", "conv-1",
		`{}`, "# Title", "title")
	if err != nil {
		t.Fatal(err)
	}
	if ch.lastReply.Format != "markdown" {
		t.Fatalf("dingtalk should use markdown, got %s", ch.lastReply.Format)
	}
}

func TestCardDispatcher_PlainFallback(t *testing.T) {
	ch := &mockChannel{typ: "signal"}
	d := NewCardDispatcher(map[string]Channel{"signal": ch})

	err := d.SendCard(context.Background(), "signal", "user-1",
		`{}`, "**md**", "plain fallback")
	if err != nil {
		t.Fatal(err)
	}
	if ch.lastReply.Content != "plain fallback" {
		t.Fatalf("expected plain fallback, got %s", ch.lastReply.Content)
	}
}

func TestCardDispatcher_UnknownChannel(t *testing.T) {
	d := NewCardDispatcher(map[string]Channel{})
	err := d.SendCard(context.Background(), "unknown", "x", "", "", "")
	if err == nil {
		t.Fatal("expected error for unknown channel")
	}
}
