package channel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	qqTokenURL       = "https://bots.qq.com/app/getAppAccessToken"
	qqAPIBase        = "https://api.sgroup.qq.com"
	qqSandboxAPIBase = "https://sandbox.api.sgroup.qq.com"
	qqMaxMsgLen      = 3000 // conservative limit for QQ text messages
)

// QQ opcode constants.
const (
	qqOpDispatch       = 0
	qqOpHeartbeat      = 1
	qqOpIdentify       = 2
	qqOpResume         = 6
	qqOpReconnect      = 7
	qqOpInvalidSession = 9
	qqOpHello          = 10
	qqOpHeartbeatAck   = 11
)

// QQ intent bit flags.
const (
	qqIntentGuilds         = 1 << 0
	qqIntentGuildMembers   = 1 << 1
	qqIntentDirectMessage  = 1 << 12
	qqIntentGroupAndC2C    = 1 << 25
	qqIntentPublicGuildMsg = 1 << 30
)

// QQConfig holds configuration for the QQ Bot channel.
type QQConfig struct {
	AppID     string
	AppSecret string
	Sandbox   bool
}

// QQ implements the Channel interface for QQ Bot (群聊 + 单聊).
type QQ struct {
	cfg    QQConfig
	client *http.Client

	mu          sync.RWMutex
	accessToken string
	tokenExpAt  time.Time

	// WebSocket state
	wsConn       *websocket.Conn
	sessionID    string
	lastSeq      int64
	heartbeatInt time.Duration

	// Message dedup
	seenMu   sync.Mutex
	seenMsgs map[string]time.Time
}

// NewQQ creates a QQ Bot channel.
func NewQQ(cfg QQConfig) *QQ {
	return &QQ{
		cfg:      cfg,
		client:   &http.Client{Timeout: 15 * time.Second},
		seenMsgs: make(map[string]time.Time),
	}
}

func (q *QQ) Type() string { return "qq" }

// apiBase returns the correct API base URL based on sandbox mode.
func (q *QQ) apiBase() string {
	if q.cfg.Sandbox {
		return qqSandboxAPIBase
	}
	return qqAPIBase
}

// ──────────────────── Token Management ────────────────────

func (q *QQ) refreshToken() error {
	body, _ := json.Marshal(map[string]string{
		"appId":        q.cfg.AppID,
		"clientSecret": q.cfg.AppSecret,
	})
	resp, err := q.client.Post(qqTokenURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("qq token request: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   string `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("qq token decode: %w", err)
	}
	if result.AccessToken == "" {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("qq token empty, response: %s", string(b))
	}

	q.mu.Lock()
	q.accessToken = result.AccessToken
	// Refresh 60s before expiry (default 7200s)
	q.tokenExpAt = time.Now().Add(7140 * time.Second)
	q.mu.Unlock()

	slog.Info("qq token refreshed", "expires_in", result.ExpiresIn)
	return nil
}

func (q *QQ) getToken() string {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.accessToken
}

func (q *QQ) tokenRefreshLoop(ctx context.Context) {
	for {
		q.mu.RLock()
		expAt := q.tokenExpAt
		q.mu.RUnlock()

		wait := time.Until(expAt) - 60*time.Second
		if wait < 30*time.Second {
			wait = 30 * time.Second
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
			if err := q.refreshToken(); err != nil {
				slog.Error("qq token refresh failed", "err", err)
				time.Sleep(30 * time.Second)
			}
		}
	}
}

// ──────────────────── WebSocket Connection ────────────────────

// qqPayload is the universal WebSocket payload structure.
type qqPayload struct {
	ID string          `json:"id,omitempty"`
	Op int             `json:"op"`
	D  json.RawMessage `json:"d,omitempty"`
	S  int64           `json:"s,omitempty"`
	T  string          `json:"t,omitempty"`
}

func (q *QQ) Start(ctx context.Context, handler func(Message) Reply) error {
	// Initial token
	if err := q.refreshToken(); err != nil {
		return fmt.Errorf("qq initial token: %w", err)
	}

	// Background token refresh
	go q.tokenRefreshLoop(ctx)

	// Connect loop with auto-reconnect
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := q.connectAndListen(ctx, handler)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		slog.Warn("qq websocket disconnected, reconnecting in 5s", "err", err)
		time.Sleep(5 * time.Second)
	}
}

func (q *QQ) connectAndListen(ctx context.Context, handler func(Message) Reply) error {
	// Get gateway URL
	gwURL, err := q.getGatewayURL()
	if err != nil {
		return fmt.Errorf("qq get gateway: %w", err)
	}

	// Connect WebSocket
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, gwURL, nil)
	if err != nil {
		return fmt.Errorf("qq ws dial: %w", err)
	}
	defer conn.Close()

	q.mu.Lock()
	q.wsConn = conn
	q.mu.Unlock()

	// Read Hello
	var hello qqPayload
	if err := conn.ReadJSON(&hello); err != nil {
		return fmt.Errorf("qq read hello: %w", err)
	}
	if hello.Op != qqOpHello {
		return fmt.Errorf("qq expected Hello(op:10), got op:%d", hello.Op)
	}

	var helloData struct {
		HeartbeatInterval int `json:"heartbeat_interval"`
	}
	json.Unmarshal(hello.D, &helloData)
	q.heartbeatInt = time.Duration(helloData.HeartbeatInterval) * time.Millisecond
	slog.Info("qq ws connected", "heartbeat_ms", helloData.HeartbeatInterval)

	// Identify or Resume
	if q.sessionID != "" && q.lastSeq > 0 {
		// Resume existing session
		if err := q.sendResume(conn); err != nil {
			// Fall back to fresh identify
			q.sessionID = ""
			q.lastSeq = 0
			if err := q.sendIdentify(conn); err != nil {
				return fmt.Errorf("qq identify: %w", err)
			}
		}
	} else {
		if err := q.sendIdentify(conn); err != nil {
			return fmt.Errorf("qq identify: %w", err)
		}
	}

	// Heartbeat goroutine
	heartCtx, heartCancel := context.WithCancel(ctx)
	defer heartCancel()
	go q.heartbeatLoop(heartCtx, conn)

	// Event loop
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		var payload qqPayload
		if err := conn.ReadJSON(&payload); err != nil {
			return fmt.Errorf("qq ws read: %w", err)
		}

		// Update sequence number
		if payload.S > 0 {
			q.mu.Lock()
			q.lastSeq = payload.S
			q.mu.Unlock()
		}

		switch payload.Op {
		case qqOpDispatch:
			q.handleDispatch(ctx, payload, handler)
		case qqOpHeartbeatAck:
			// OK
		case qqOpReconnect:
			slog.Info("qq server requested reconnect")
			return fmt.Errorf("server reconnect requested")
		case qqOpInvalidSession:
			slog.Warn("qq invalid session, will re-identify")
			q.mu.Lock()
			q.sessionID = ""
			q.lastSeq = 0
			q.mu.Unlock()
			return fmt.Errorf("invalid session")
		}
	}
}

func (q *QQ) getGatewayURL() (string, error) {
	req, _ := http.NewRequest("GET", q.apiBase()+"/gateway", nil)
	req.Header.Set("Authorization", "QQBot "+q.getToken())
	resp, err := q.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.URL == "" {
		return "", fmt.Errorf("empty gateway URL")
	}
	return result.URL, nil
}

func (q *QQ) sendIdentify(conn *websocket.Conn) error {
	intents := qqIntentGroupAndC2C | qqIntentPublicGuildMsg | qqIntentGuilds | qqIntentDirectMessage
	payload := map[string]any{
		"op": qqOpIdentify,
		"d": map[string]any{
			"token":   "QQBot " + q.getToken(),
			"intents": intents,
			"shard":   []int{0, 1},
			"properties": map[string]string{
				"$os":      "linux",
				"$browser": "yunque-agent",
				"$device":  "yunque-agent",
			},
		},
	}
	slog.Info("qq sending identify", "intents", intents)
	return conn.WriteJSON(payload)
}

func (q *QQ) sendResume(conn *websocket.Conn) error {
	q.mu.RLock()
	sid := q.sessionID
	seq := q.lastSeq
	q.mu.RUnlock()

	payload := map[string]any{
		"op": qqOpResume,
		"d": map[string]any{
			"token":      "QQBot " + q.getToken(),
			"session_id": sid,
			"seq":        seq,
		},
	}
	slog.Info("qq sending resume", "session_id", sid, "seq", seq)
	return conn.WriteJSON(payload)
}

func (q *QQ) heartbeatLoop(ctx context.Context, conn *websocket.Conn) {
	ticker := time.NewTicker(q.heartbeatInt)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			q.mu.RLock()
			seq := q.lastSeq
			q.mu.RUnlock()

			var d any = seq
			if seq == 0 {
				d = nil
			}
			err := conn.WriteJSON(map[string]any{"op": qqOpHeartbeat, "d": d})
			if err != nil {
				slog.Warn("qq heartbeat send failed", "err", err)
				return
			}
		}
	}
}

// ──────────────────── Event Dispatch ────────────────────

func (q *QQ) handleDispatch(ctx context.Context, payload qqPayload, handler func(Message) Reply) {
	switch payload.T {
	case "READY":
		var ready struct {
			SessionID string `json:"session_id"`
			User      struct {
				ID       string `json:"id"`
				Username string `json:"username"`
			} `json:"user"`
		}
		json.Unmarshal(payload.D, &ready)
		q.mu.Lock()
		q.sessionID = ready.SessionID
		q.mu.Unlock()
		slog.Info("qq bot ready", "user", ready.User.Username, "session", ready.SessionID)

	case "RESUMED":
		slog.Info("qq session resumed")

	case "C2C_MESSAGE_CREATE":
		q.handleC2CMessage(ctx, payload, handler)

	case "GROUP_AT_MESSAGE_CREATE":
		q.handleGroupMessage(ctx, payload, handler)

	case "AT_MESSAGE_CREATE":
		q.handleGuildAtMessage(ctx, payload, handler)

	case "DIRECT_MESSAGE_CREATE":
		q.handleDirectMessage(ctx, payload, handler)

	default:
		slog.Debug("qq unhandled event", "type", payload.T)
	}
}

// ── C2C (单聊) ──

type qqC2CMessage struct {
	ID     string `json:"id"`
	Author struct {
		UserOpenID string `json:"user_openid"`
	} `json:"author"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
}

func (q *QQ) handleC2CMessage(ctx context.Context, payload qqPayload, handler func(Message) Reply) {
	var evt qqC2CMessage
	if err := json.Unmarshal(payload.D, &evt); err != nil {
		slog.Error("qq c2c parse", "err", err)
		return
	}
	if q.isDuplicate(evt.ID) {
		return
	}

	content := strings.TrimSpace(evt.Content)
	if content == "" {
		return
	}

	msg := Message{
		ChannelType: "qq",
		ChannelID:   evt.Author.UserOpenID, // private chat: channelID = userID
		UserID:      evt.Author.UserOpenID,
		UserName:    "",
		Content:     content,
		Extra: map[string]string{
			"msg_id":      evt.ID,
			"chat_type":   "c2c",
			"event_id":    payload.ID,
			"user_openid": evt.Author.UserOpenID,
		},
	}

	slog.Info("qq c2c message", "user", evt.Author.UserOpenID, "len", len(content))
	go q.processAndReply(ctx, msg, handler)
}

// ── 群聊@机器人 ──

type qqGroupMessage struct {
	ID          string `json:"id"`
	GroupOpenID string `json:"group_openid"`
	Author      struct {
		MemberOpenID string `json:"member_openid"`
	} `json:"author"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
}

func (q *QQ) handleGroupMessage(ctx context.Context, payload qqPayload, handler func(Message) Reply) {
	var evt qqGroupMessage
	if err := json.Unmarshal(payload.D, &evt); err != nil {
		slog.Error("qq group parse", "err", err)
		return
	}
	if q.isDuplicate(evt.ID) {
		return
	}

	content := strings.TrimSpace(evt.Content)
	if content == "" {
		return
	}

	msg := Message{
		ChannelType: "qq",
		ChannelID:   evt.GroupOpenID,
		UserID:      evt.Author.MemberOpenID,
		UserName:    "",
		Content:     content,
		Extra: map[string]string{
			"msg_id":       evt.ID,
			"chat_type":    "group",
			"group_openid": evt.GroupOpenID,
			"event_id":     payload.ID,
		},
	}

	slog.Info("qq group message", "group", evt.GroupOpenID, "user", evt.Author.MemberOpenID, "len", len(content))
	go q.processAndReply(ctx, msg, handler)
}

// ── 频道@机器人 ──

type qqGuildMessage struct {
	ID        string `json:"id"`
	ChannelID string `json:"channel_id"`
	GuildID   string `json:"guild_id"`
	Author    struct {
		ID       string `json:"id"`
		Username string `json:"username"`
		Bot      bool   `json:"bot"`
	} `json:"author"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
}

func (q *QQ) handleGuildAtMessage(ctx context.Context, payload qqPayload, handler func(Message) Reply) {
	var evt qqGuildMessage
	if err := json.Unmarshal(payload.D, &evt); err != nil {
		slog.Error("qq guild parse", "err", err)
		return
	}
	if evt.Author.Bot || q.isDuplicate(evt.ID) {
		return
	}

	content := strings.TrimSpace(evt.Content)
	if content == "" {
		return
	}

	msg := Message{
		ChannelType: "qq",
		ChannelID:   evt.ChannelID,
		UserID:      evt.Author.ID,
		UserName:    evt.Author.Username,
		Content:     content,
		Extra: map[string]string{
			"msg_id":     evt.ID,
			"chat_type":  "guild",
			"guild_id":   evt.GuildID,
			"channel_id": evt.ChannelID,
			"event_id":   payload.ID,
		},
	}

	slog.Info("qq guild message", "channel", evt.ChannelID, "user", evt.Author.Username)
	go q.processAndReply(ctx, msg, handler)
}

// ── 频道私信 ──

func (q *QQ) handleDirectMessage(ctx context.Context, payload qqPayload, handler func(Message) Reply) {
	var evt qqGuildMessage
	if err := json.Unmarshal(payload.D, &evt); err != nil {
		slog.Error("qq dm parse", "err", err)
		return
	}
	if evt.Author.Bot || q.isDuplicate(evt.ID) {
		return
	}

	content := strings.TrimSpace(evt.Content)
	if content == "" {
		return
	}

	msg := Message{
		ChannelType: "qq",
		ChannelID:   evt.Author.ID,
		UserID:      evt.Author.ID,
		UserName:    evt.Author.Username,
		Content:     content,
		Extra: map[string]string{
			"msg_id":     evt.ID,
			"chat_type":  "dm",
			"guild_id":   evt.GuildID,
			"channel_id": evt.ChannelID,
			"event_id":   payload.ID,
		},
	}

	slog.Info("qq dm message", "user", evt.Author.Username)
	go q.processAndReply(ctx, msg, handler)
}

// ──────────────────── Message Sending ────────────────────

func (q *QQ) processAndReply(ctx context.Context, msg Message, handler func(Message) Reply) {
	reply := handler(msg)
	if err := q.SendWithExtra(ctx, "", reply, msg.Extra); err != nil {
		slog.Error("qq send reply failed", "err", err, "chat_type", msg.Extra["chat_type"])
	}
}

// Send sends a message. For QQ, the `target` is ignored in favor of Extra metadata.
// Use SendWithExtra for the full passive-reply flow.
func (q *QQ) Send(_ context.Context, target string, reply Reply) error {
	// Bare Send without Extra — cannot do passive reply, log warning
	if target == "" {
		slog.Warn("qq Send called without target/Extra, message dropped")
		return nil
	}
	// Can't do much without msg_id for QQ's passive reply, but try best-effort
	return nil
}

// SendWithExtra sends a passive reply using msg_id from the original event.
func (q *QQ) SendWithExtra(_ context.Context, target string, reply Reply, extra map[string]string) error {
	chatType := extra["chat_type"]
	msgID := extra["msg_id"]
	content := ContentWithButtonFallback(reply)
	if content == "" {
		return nil
	}

	chunks := splitQQMessage(content)
	for i, chunk := range chunks {
		var url string
		body := map[string]any{
			"content":  chunk,
			"msg_type": 0,
		}

		// First chunk uses msg_id for passive reply; subsequent use msg_seq
		if msgID != "" {
			body["msg_id"] = msgID
			body["msg_seq"] = i + 1
		}

		switch chatType {
		case "c2c":
			openid := extra["user_openid"]
			url = q.apiBase() + "/v2/users/" + openid + "/messages"
		case "group":
			groupOpenID := extra["group_openid"]
			url = q.apiBase() + "/v2/groups/" + groupOpenID + "/messages"
		case "guild":
			channelID := extra["channel_id"]
			url = q.apiBase() + "/channels/" + channelID + "/messages"
		case "dm":
			guildID := extra["guild_id"]
			url = q.apiBase() + "/dms/" + guildID + "/messages"
		default:
			return fmt.Errorf("qq unknown chat_type: %s", chatType)
		}

		if err := q.doSendMessage(url, body); err != nil {
			return fmt.Errorf("qq send chunk %d: %w", i, err)
		}
	}
	return nil
}

func (q *QQ) doSendMessage(url string, body map[string]any) error {
	data, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", url, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "QQBot "+q.getToken())

	resp, err := q.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("qq api %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

// ──────────────────── Helpers ────────────────────

func (q *QQ) isDuplicate(msgID string) bool {
	q.seenMu.Lock()
	defer q.seenMu.Unlock()

	now := time.Now()
	// Cleanup old entries
	for k, t := range q.seenMsgs {
		if now.Sub(t) > 2*time.Minute {
			delete(q.seenMsgs, k)
		}
	}

	if _, ok := q.seenMsgs[msgID]; ok {
		slog.Debug("qq duplicate msg skipped", "id", msgID)
		return true
	}
	q.seenMsgs[msgID] = now
	return false
}

func splitQQMessage(text string) []string {
	return SplitMessage(text, qqMaxMsgLen)
}
