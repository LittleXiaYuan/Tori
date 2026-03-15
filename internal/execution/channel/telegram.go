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

// Telegram implements the Channel interface for Telegram Bot API.
type Telegram struct {
	token      string
	client     *http.Client
	webhookURL string // if set, use webhook mode instead of polling
	msgCh      chan Message
}

// NewTelegram creates a Telegram channel with the given bot token.
func NewTelegram(token string) *Telegram {
	return &Telegram{
		token:  token,
		client: &http.Client{Timeout: 30 * time.Second},
		msgCh:  make(chan Message, 100),
	}
}

// SetWebhook configures webhook mode. Call before Start().
// webhookURL is the public HTTPS URL, e.g. https://yourdomain.com/webhook/telegram
func (t *Telegram) SetWebhook(webhookURL string) error {
	t.webhookURL = webhookURL
	body, _ := json.Marshal(map[string]any{
		"url":             webhookURL,
		"allowed_updates": []string{"message", "callback_query"},
	})
	resp, err := t.client.Post(t.apiURL("setWebhook"), "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("setWebhook: %w", err)
	}
	defer resp.Body.Close()
	var result struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	if !result.OK {
		return fmt.Errorf("setWebhook failed: %s", result.Description)
	}
	slog.Info("telegram webhook set", "url", webhookURL)
	return nil
}

// DeleteWebhook removes the webhook and switches to polling mode.
func (t *Telegram) DeleteWebhook() error {
	t.webhookURL = ""
	resp, err := t.client.Post(t.apiURL("deleteWebhook"), "application/json", bytes.NewReader([]byte(`{}`)))
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (t *Telegram) Type() string { return "telegram" }

func (t *Telegram) Start(ctx context.Context, handler func(Message) Reply) error {
	if t.webhookURL != "" {
		return t.startWebhook(ctx, handler)
	}
	return t.startPolling(ctx, handler)
}

func (t *Telegram) startPolling(ctx context.Context, handler func(Message) Reply) error {
	offset := 0
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		updates, err := t.getUpdates(offset, 30)
		if err != nil {
			slog.Error("telegram getUpdates", "err", err)
			time.Sleep(5 * time.Second)
			continue
		}

		for _, u := range updates {
			offset = u.UpdateID + 1
			msg := t.parseUpdate(u)
			if msg == nil {
				continue
			}
			t.processMessage(ctx, *msg, handler)
		}
	}
}

func (t *Telegram) startWebhook(ctx context.Context, handler func(Message) Reply) error {
	slog.Info("telegram running in webhook mode")
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg := <-t.msgCh:
			t.processMessage(ctx, msg, handler)
		}
	}
}

// HandleWebhook processes incoming Telegram webhook updates.
// Mount on HTTP server: POST /webhook/telegram
func (t *Telegram) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	var u tgUpdate
	if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
		w.WriteHeader(400)
		return
	}
	if msg := t.parseUpdate(u); msg != nil {
		t.msgCh <- *msg
	}
	w.WriteHeader(200)
}

func (t *Telegram) parseUpdate(u tgUpdate) *Message {
	if u.Message == nil {
		return nil
	}
	msg := &Message{
		ChannelType: "telegram",
		ChannelID:   fmt.Sprintf("%d", u.Message.Chat.ID),
		UserID:      fmt.Sprintf("%d", u.Message.From.ID),
		UserName:    u.Message.From.FirstName,
		Extra: map[string]string{
			"message_id": fmt.Sprintf("%d", u.Message.MessageID),
			"chat_type":  u.Message.Chat.Type,
		},
	}

	// Handle sticker messages
	if u.Message.Sticker != nil {
		stk := u.Message.Sticker
		sc := NewSticker("", stk.FileID)
		sc.Platform = "telegram"
		sc.FileID = stk.FileID
		sc.SetName = stk.SetName
		sc.Emoji = stk.Emoji
		sc.IsAnimated = stk.IsAnimated
		sc.IsVideo = stk.IsVideo
		rm := NewRichMessage()
		rm.Add(sc)
		msg.Rich = rm
		if stk.Emoji != "" {
			msg.Content = "Sticker: " + stk.Emoji
		} else {
			msg.Content = "[贴图]"
		}
		return msg
	}

	// Handle text messages
	if u.Message.Text != "" {
		msg.Content = u.Message.Text
		return msg
	}

	// Handle photo messages with caption
	if len(u.Message.Photo) > 0 {
		// Use the largest photo (last in array)
		photo := u.Message.Photo[len(u.Message.Photo)-1]
		rm := NewRichMessage()
		img := NewImageFromURL("", "")
		img.FileID = photo.FileID
		rm.Add(img)
		msg.Rich = rm
		msg.Content = u.Message.Caption
		if msg.Content == "" {
			msg.Content = "[图片]"
		}
		return msg
	}

	// Skip unsupported message types
	return nil
}

func (t *Telegram) processMessage(ctx context.Context, msg Message, handler func(Message) Reply) {
	chatID := msg.ChannelID

	// Handle built-in commands
	if cmd := t.parseCommand(msg.Content); cmd != "" {
		reply := t.handleCommand(cmd)
		_ = t.Send(ctx, chatID, reply)
		return
	}

	// Show typing indicator
	t.sendChatAction(chatID, "typing")

	reply := handler(msg)
	_ = t.Send(ctx, chatID, reply)
}

// sendChatAction sends a typing indicator to the chat.
func (t *Telegram) sendChatAction(chatID, action string) {
	body, _ := json.Marshal(map[string]string{"chat_id": chatID, "action": action})
	resp, err := t.client.Post(t.apiURL("sendChatAction"), "application/json", bytes.NewReader(body))
	if err == nil {
		resp.Body.Close()
	}
}

func (t *Telegram) Send(ctx context.Context, target string, reply Reply) error {
	// If reply includes a RichMessage with stickers/images, send them separately
	if reply.Rich != nil {
		for _, comp := range reply.Rich.Components {
			switch comp.Type() {
			case ComponentSticker:
				sc, ok := comp.(*StickerComponent)
				if ok {
					_ = t.SendSticker(ctx, target, sc)
				}
			case ComponentImage:
				ic, ok := comp.(*ImageComponent)
				if ok && ic.URL != "" {
					_ = t.sendPhoto(target, ic.URL, ic.Alt)
				}
			}
		}
	}

	// Send text content (skip if empty and we already sent rich components)
	if reply.Content == "" && reply.Rich != nil && len(reply.Rich.Components) > 0 {
		return nil
	}

	body, _ := json.Marshal(map[string]any{
		"chat_id":    target,
		"text":       reply.Content,
		"parse_mode": "Markdown",
	})
	resp, err := t.client.Post(t.apiURL("sendMessage"), "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram sendMessage %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

func (t *Telegram) apiURL(method string) string {
	return fmt.Sprintf("https://api.telegram.org/bot%s/%s", t.token, method)
}

// sendText sends a plain text message (used for emoji fallback when no sticker available).
func (t *Telegram) sendText(chatID, text string) error {
	body, _ := json.Marshal(map[string]any{"chat_id": chatID, "text": text})
	resp, err := t.client.Post(t.apiURL("sendMessage"), "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// sendPhoto sends a photo via URL to a Telegram chat.
func (t *Telegram) sendPhoto(chatID, photoURL, caption string) error {
	payload := map[string]any{"chat_id": chatID, "photo": photoURL}
	if caption != "" {
		payload["caption"] = caption
	}
	body, _ := json.Marshal(payload)
	resp, err := t.client.Post(t.apiURL("sendPhoto"), "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

type tgUpdate struct {
	UpdateID int        `json:"update_id"`
	Message  *tgMessage `json:"message"`
}

type tgMessage struct {
	MessageID int        `json:"message_id"`
	Text      string     `json:"text"`
	Chat      tgChat     `json:"chat"`
	From      tgUser     `json:"from"`
	Sticker   *tgSticker `json:"sticker,omitempty"`
	Photo     []tgPhoto  `json:"photo,omitempty"`
	Caption   string     `json:"caption,omitempty"`
}

type tgSticker struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	Type         string `json:"type"` // "regular", "mask", "custom_emoji"
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	IsAnimated   bool   `json:"is_animated"`
	IsVideo      bool   `json:"is_video"`
	Emoji        string `json:"emoji,omitempty"`
	SetName      string `json:"set_name,omitempty"`
}

type tgPhoto struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
}

type tgChat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"` // "private", "group", "supergroup", "channel"
}

type tgUser struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
}

func (t *Telegram) parseCommand(text string) string {
	if len(text) == 0 || text[0] != '/' {
		return ""
	}
	cmd := text
	if idx := bytes.IndexByte([]byte(cmd), ' '); idx > 0 {
		cmd = cmd[:idx]
	}
	if idx := bytes.IndexByte([]byte(cmd), '@'); idx > 0 {
		cmd = cmd[:idx]
	}
	return cmd
}

func (t *Telegram) handleCommand(cmd string) Reply {
	switch cmd {
	case "/start":
		return Reply{Content: "👋 你好！我是云鸢智能助手。\n\n直接发消息即可对话，也可使用命令：\n/help - 帮助\n/skills - 查看技能", Format: "text"}
	case "/help":
		return Reply{Content: "📖 *使用说明*\n\n• 直接发送文字即可对话\n• 支持多轮上下文\n• 可执行代码、搜索、分析等\n\n/skills - 查看可用技能", Format: "markdown"}
	case "/skills":
		return Reply{Content: "🔧 *可用技能*\n\n• web\\_search - 搜索\n• code\\_execute - 代码执行\n• file\\_search - 文件搜索\n• lesson\\_plan - 教案生成\n• quiz\\_generate - 出题\n• grade\\_work - 批改", Format: "markdown"}
	default:
		return Reply{Content: "未知命令: " + cmd + "\n使用 /help 查看帮助", Format: "text"}
	}
}

// React adds an emoji reaction to a Telegram message.
// Supports unicode emoji (e.g. "👍") and custom emoji IDs (numeric strings).
// Pass empty emoji to remove the bot's reaction.
func (t *Telegram) React(ctx context.Context, target string, messageID string, emoji string) error {
	var reaction []map[string]any
	if emoji != "" {
		// Check if it's a custom emoji ID (numeric string)
		isCustom := true
		for _, r := range emoji {
			if r < '0' || r > '9' {
				isCustom = false
				break
			}
		}
		if isCustom && len(emoji) > 3 {
			reaction = []map[string]any{{"type": "custom_emoji", "custom_emoji_id": emoji}}
		} else {
			reaction = []map[string]any{{"type": "emoji", "emoji": emoji}}
		}
	}
	body, _ := json.Marshal(map[string]any{
		"chat_id":    target,
		"message_id": messageID,
		"reaction":   reaction,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.apiURL("setMessageReaction"), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("telegram setMessageReaction: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram setMessageReaction %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

// SendSticker sends a sticker natively via Telegram's sendSticker API.
func (t *Telegram) SendSticker(ctx context.Context, target string, sticker *StickerComponent) error {
	// Priority: FileID → StickerURL → emoji text fallback
	stickerVal := sticker.FileID
	if stickerVal == "" {
		stickerVal = sticker.StickerURL()
	}
	if stickerVal == "" {
		// No sticker data available, fallback to emoji text
		if sticker.Emoji != "" {
			return t.sendText(target, sticker.Emoji)
		}
		return fmt.Errorf("no sticker file_id or url available")
	}
	body, _ := json.Marshal(map[string]any{"chat_id": target, "sticker": stickerVal})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.apiURL("sendSticker"), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("telegram sendSticker: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram sendSticker %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

// Ensure Telegram implements optional interfaces
var (
	_ Channel       = (*Telegram)(nil)
	_ Reactor       = (*Telegram)(nil)
	_ StickerSender = (*Telegram)(nil)
)

func (t *Telegram) getUpdates(offset, timeout int) ([]tgUpdate, error) {
	url := fmt.Sprintf("%s?offset=%d&timeout=%d", t.apiURL("getUpdates"), offset, timeout)
	resp, err := t.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result struct {
		OK     bool       `json:"ok"`
		Result []tgUpdate `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Result, nil
}
