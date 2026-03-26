package channel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// ──────────────────────────────────────────────
// ProgressSender implementations for all IM platforms
//
// Each platform that supports editing messages implements:
//   SendAndGetID — send message, return platform message ID
//   EditMessage  — edit existing message by ID
//
// Platforms WITHOUT edit support (LINE, Signal, WeChat Official, Email)
// simply don't implement this interface — the channel handler
// falls back to single-message mode automatically.
// ──────────────────────────────────────────────

// ── Discord ──

func (d *Discord) SendAndGetID(_ context.Context, target string, reply Reply) (string, error) {
	d.mu.Lock()
	session := d.session
	d.mu.Unlock()
	if session == nil {
		return "", fmt.Errorf("discord session not initialized")
	}
	content := reply.Content
	if content == "" {
		content = "..."
	}
	msg, err := session.ChannelMessageSend(target, content)
	if err != nil {
		return "", fmt.Errorf("discord send: %w", err)
	}
	return msg.ID, nil
}

func (d *Discord) EditMessage(_ context.Context, target string, messageID string, content string) error {
	d.mu.Lock()
	session := d.session
	d.mu.Unlock()
	if session == nil {
		return fmt.Errorf("discord session not initialized")
	}
	_, err := session.ChannelMessageEdit(target, messageID, content)
	return err
}

// ── Slack ──

func (s *Slack) SendAndGetID(ctx context.Context, channelID string, reply Reply) (string, error) {
	body := map[string]any{
		"channel": channelID,
		"text":    reply.Content,
	}
	if reply.ReplyTo != "" {
		body["thread_ts"] = reply.ReplyTo
	}
	data, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://slack.com/api/chat.postMessage", bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", "Bearer "+s.botToken)
	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("slack send: %w", err)
	}
	defer resp.Body.Close()
	var result struct {
		OK    bool   `json:"ok"`
		TS    string `json:"ts"`
		Error string `json:"error,omitempty"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	if !result.OK {
		return "", fmt.Errorf("slack send: %s", result.Error)
	}
	return result.TS, nil // Slack uses "ts" as message ID
}

func (s *Slack) EditMessage(ctx context.Context, channelID string, messageID string, content string) error {
	body := map[string]any{
		"channel": channelID,
		"ts":      messageID, // Slack message ID is "ts"
		"text":    content,
	}
	data, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://slack.com/api/chat.update", bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", "Bearer "+s.botToken)
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("slack update: %w", err)
	}
	resp.Body.Close()
	return nil
}

// ── Feishu ──

func (f *Feishu) SendAndGetID(_ context.Context, chatID string, reply Reply) (string, error) {
	f.tokenMu.RLock()
	token := f.token
	f.tokenMu.RUnlock()

	content := fmt.Sprintf(`{"text":"%s"}`, reply.Content)
	body, _ := json.Marshal(map[string]any{
		"receive_id": chatID,
		"msg_type":   "text",
		"content":    content,
	})
	req, _ := http.NewRequest("POST",
		"https://open.feishu.cn/open-apis/im/v1/messages?receive_id_type=chat_id",
		bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := f.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var result struct {
		Code int `json:"code"`
		Data struct {
			MessageID string `json:"message_id"`
		} `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	if result.Code != 0 {
		return "", fmt.Errorf("feishu send error code=%d", result.Code)
	}
	return result.Data.MessageID, nil
}

func (f *Feishu) EditMessage(ctx context.Context, _ string, messageID string, content string) error {
	f.tokenMu.RLock()
	token := f.token
	f.tokenMu.RUnlock()

	body, _ := json.Marshal(map[string]any{
		"msg_type": "text",
		"content":  fmt.Sprintf(`{"text":"%s"}`, content),
	})
	url := fmt.Sprintf("https://open.feishu.cn/open-apis/im/v1/messages/%s", messageID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := f.client.Do(req)
	if err != nil {
		return fmt.Errorf("feishu edit: %w", err)
	}
	resp.Body.Close()
	return nil
}

// ── DingTalk ──
// DingTalk group webhook doesn't return message ID or support editing.
// Implement no-op stubs so it satisfies the interface but won't actually edit.

func (d *DingTalk) SendAndGetID(ctx context.Context, target string, reply Reply) (string, error) {
	err := d.Send(ctx, target, reply)
	return "", err // no message ID available
}

func (d *DingTalk) EditMessage(_ context.Context, _ string, _ string, _ string) error {
	return nil // not supported
}

// ── Kook ──

func (k *Kook) SendAndGetID(ctx context.Context, target string, reply Reply) (string, error) {
	content := reply.Content
	if content == "" {
		content = "..."
	}
	payload := map[string]any{
		"type":      1, // text
		"target_id": target,
		"content":   content,
	}
	data, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		k.apiBase+"/message/create", bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bot "+k.token)
	resp, err := k.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("kook send: %w", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var result struct {
		Code int `json:"code"`
		Data struct {
			MsgID string `json:"msg_id"`
		} `json:"data"`
	}
	json.Unmarshal(b, &result)
	return result.Data.MsgID, nil
}

func (k *Kook) EditMessage(ctx context.Context, _ string, messageID string, content string) error {
	payload := map[string]any{
		"msg_id":  messageID,
		"content": content,
	}
	return k.callAPI(ctx, "/message/update", payload)
}

// ── WeCom (企业微信) ──
// WeCom webhook doesn't support message editing.

func (w *WeCom) SendAndGetID(ctx context.Context, target string, reply Reply) (string, error) {
	err := w.Send(ctx, target, reply)
	return "", err
}

func (w *WeCom) EditMessage(_ context.Context, _ string, _ string, _ string) error {
	return nil
}

// ── QQ ──
// QQ Bot passive reply model doesn't easily support editing.

func (q *QQ) SendAndGetID(_ context.Context, _ string, reply Reply) (string, error) {
	// QQ passive reply requires msg_id from the original event,
	// which is not available here. Return empty — handler falls back to no-edit mode.
	return "", nil
}

func (q *QQ) EditMessage(_ context.Context, _ string, _ string, _ string) error {
	return nil // QQ doesn't support message editing via bot API
}

// ── WhatsApp ──

func (w *WhatsApp) SendAndGetID(ctx context.Context, to string, reply Reply) (string, error) {
	content := reply.Content
	if content == "" {
		content = "..."
	}
	body, _ := json.Marshal(map[string]any{
		"messaging_product": "whatsapp",
		"to":                to,
		"type":              "text",
		"text":              map[string]string{"body": content},
	})
	url := fmt.Sprintf("https://graph.facebook.com/v21.0/%s/messages", w.phoneNumberID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+w.token)
	resp, err := w.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("whatsapp send: %w", err)
	}
	defer resp.Body.Close()
	var result struct {
		Messages []struct {
			ID string `json:"id"`
		} `json:"messages"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	if len(result.Messages) > 0 {
		return result.Messages[0].ID, nil
	}
	return "", nil
}

func (w *WhatsApp) EditMessage(_ context.Context, _ string, _ string, _ string) error {
	// WhatsApp Cloud API doesn't support editing messages
	return nil
}

// ── Satori (通用协议) ──

func (s *Satori) SendAndGetID(ctx context.Context, target string, reply Reply) (string, error) {
	content := reply.Content
	if content == "" {
		content = "..."
	}
	payload := satoriMessageCreateReq{
		ChannelID: target,
		Content:   content,
	}
	data, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		s.endpoint+"/v1/message.create", bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if s.token != "" {
		req.Header.Set("Authorization", "Bearer "+s.token)
	}
	if s.platform != "" {
		req.Header.Set("X-Platform", s.platform)
	}
	if s.selfID != "" {
		req.Header.Set("X-Self-ID", s.selfID)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("satori send: %w", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	var msgs []struct {
		ID string `json:"id"`
	}
	json.Unmarshal(b, &msgs)
	if len(msgs) > 0 {
		return msgs[0].ID, nil
	}
	return "", nil
}

func (s *Satori) EditMessage(ctx context.Context, target string, messageID string, content string) error {
	payload := map[string]any{
		"channel_id": target,
		"message_id": messageID,
		"content":    content,
	}
	return s.callAPI(ctx, s.platform, "message.update", payload)
}

// ── Interface assertions ──

var (
	_ ProgressSender = (*Discord)(nil)
	_ ProgressSender = (*Slack)(nil)
	_ ProgressSender = (*Feishu)(nil)
	_ ProgressSender = (*DingTalk)(nil)
	_ ProgressSender = (*Kook)(nil)
	_ ProgressSender = (*WeCom)(nil)
	_ ProgressSender = (*QQ)(nil)
	_ ProgressSender = (*WhatsApp)(nil)
	_ ProgressSender = (*Satori)(nil)
)
