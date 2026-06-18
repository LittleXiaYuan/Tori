package channelspack

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"yunque-agent/internal/execution/channel"
)

type testChannel struct {
	typ         string
	reactTarget string
	reactMsg    string
	reactEmoji  string
	sticker     *channel.StickerComponent
	groups      []channel.GroupInfo
}

func (c *testChannel) Type() string { return c.typ }

func (c *testChannel) Start(ctx context.Context, handler func(channel.Message) channel.Reply) error {
	return nil
}

func (c *testChannel) Send(ctx context.Context, target string, reply channel.Reply) error {
	return nil
}

func (c *testChannel) React(ctx context.Context, target string, messageID string, emoji string) error {
	c.reactTarget = target
	c.reactMsg = messageID
	c.reactEmoji = emoji
	return nil
}

func (c *testChannel) SendSticker(ctx context.Context, target string, sticker *channel.StickerComponent) error {
	c.sticker = sticker
	return nil
}

func (c *testChannel) ListGroups(ctx context.Context) ([]channel.GroupInfo, error) {
	return c.groups, nil
}

func TestChannelsPackReact(t *testing.T) {
	reg := channel.NewRegistry()
	ch := &testChannel{typ: "telegram"}
	reg.Register(ch)
	h := NewProvider(func() *channel.Registry { return reg })

	req := httptest.NewRequest(http.MethodPost, "/v1/react", strings.NewReader(`{"channel_type":"telegram","target":"chat-1","message_id":"msg-1","emoji":"👍"}`))
	rec := httptest.NewRecorder()

	h.React(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if ch.reactTarget != "chat-1" || ch.reactMsg != "msg-1" || ch.reactEmoji != "👍" {
		t.Fatalf("reaction not forwarded: %#v", ch)
	}
}

func TestChannelsPackSendSticker(t *testing.T) {
	reg := channel.NewRegistry()
	ch := &testChannel{typ: "line"}
	reg.Register(ch)
	h := NewProvider(func() *channel.Registry { return reg })

	req := httptest.NewRequest(http.MethodPost, "/v1/sticker/send", strings.NewReader(`{"channel_type":"line","target":"room-1","package_id":"pkg","sticker_id":"stk","emoji":"smile"}`))
	rec := httptest.NewRecorder()

	h.SendSticker(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if ch.sticker == nil || ch.sticker.PackageID != "pkg" || ch.sticker.StickerID != "stk" || ch.sticker.Platform != "line" {
		t.Fatalf("sticker not forwarded correctly: %#v", ch.sticker)
	}
}

func TestChannelsPackGroups(t *testing.T) {
	reg := channel.NewRegistry()
	reg.Register(&testChannel{
		typ: "discord",
		groups: []channel.GroupInfo{{
			ID:          "guild-1",
			Name:        "Guild",
			ChannelType: "discord",
			ChatType:    "guild",
		}},
	})
	h := NewProvider(func() *channel.Registry { return reg })

	req := httptest.NewRequest(http.MethodGet, "/v1/channels/groups?type=discord", nil)
	rec := httptest.NewRecorder()

	h.Groups(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Groups []channel.GroupInfo `json:"groups"`
		Count  int                 `json:"count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Count != 1 || len(body.Groups) != 1 || body.Groups[0].ID != "guild-1" {
		t.Fatalf("unexpected groups body: %+v", body)
	}
}

func TestChannelsPackRequiresRegistry(t *testing.T) {
	h := NewProvider(nil)
	req := httptest.NewRequest(http.MethodGet, "/v1/channels/groups", nil)
	rec := httptest.NewRecorder()

	h.Groups(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d body=%s", rec.Code, rec.Body.String())
	}
}
