package airi

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/planner"
	agentrt "yunque-agent/internal/agentcore/runtime"
	"yunque-agent/internal/controlplane/gateway"
	"yunque-agent/internal/execution/channel"
)

// Bridge is a WebSocket client that connects to Airi's server-runtime
// and registers as a native Airi module ("yunque-agent").
//
// Outbound: intercepts all Yunque replies via Gateway.AddReplyHook,
//
//	performs emotion analysis, and sends "input:text" events to Airi.
//
// Inbound:  listens for "input:text" events from Airi (user speaking
//
//	to the desktop pet) and routes them through Yunque's Planner.
type Bridge struct {
	app        *agentrt.App
	url        string
	token      string
	moduleName string
	identity   ModuleIdentity

	conn      *websocket.Conn
	connMu    sync.Mutex
	connected atomic.Bool
	stopCh    chan struct{}
	stopped   atomic.Bool

	msgSent     atomic.Int64
	msgReceived atomic.Int64

	// hookRegistered tracks whether we've already added a ReplyHook on the gateway
	hookRegistered atomic.Bool
}

// NewBridge creates a bridge from environment config.
func NewBridge(app *agentrt.App) *Bridge {
	url := os.Getenv("AIRI_URL")
	if url == "" {
		url = "ws://localhost:6121/ws"
	}
	token := os.Getenv("AIRI_TOKEN")
	moduleName := os.Getenv("AIRI_MODULE_NAME")
	if moduleName == "" {
		moduleName = "yunque-agent"
	}
	instanceID := fmt.Sprintf("yunque-%d", time.Now().UnixMilli())

	return &Bridge{
		app:        app,
		url:        url,
		token:      token,
		moduleName: moduleName,
		identity:   NewModuleIdentity(instanceID),
		stopCh:     make(chan struct{}),
	}
}

// Connected returns whether the bridge is currently connected.
func (b *Bridge) Connected() bool { return b.connected.Load() }

// URL returns the configured Airi server-runtime URL.
func (b *Bridge) URL() string { return b.url }

// ModuleName returns the registered module name.
func (b *Bridge) ModuleName() string { return b.moduleName }

// MessagesSent returns the number of messages pushed to Airi.
func (b *Bridge) MessagesSent() int64 { return b.msgSent.Load() }

// MessagesReceived returns the number of messages received from Airi.
func (b *Bridge) MessagesReceived() int64 { return b.msgReceived.Load() }

// Run starts the bridge, connecting to Airi and reconnecting on failure.
// This blocks until Stop() is called.
func (b *Bridge) Run() {
	enabled := os.Getenv("AIRI_ENABLED")
	if enabled != "true" && enabled != "1" {
		slog.Info("[airi-bridge] AIRI_ENABLED is not set, bridge disabled")
		return
	}

	slog.Info("[airi-bridge] starting", "url", b.url, "module", b.moduleName)

	// Note: reply hook registration is deferred to after first successful connection,
	// because the gateway may not be initialized yet during plugin loading.

	backoff := time.Second
	maxBackoff := 30 * time.Second

	for {
		select {
		case <-b.stopCh:
			slog.Info("[airi-bridge] stopped")
			return
		default:
		}

		err := b.connect()
		if err != nil {
			slog.Warn("[airi-bridge] connection failed", "err", err, "retry_in", backoff)
		} else {
			// Connected successfully
			backoff = time.Second // reset backoff on successful connection
			// Try registering reply hook now (gateway should be ready by now)
			b.registerReplyHook()
			// Run the read loop (blocks until disconnect)
			b.readLoop()
		}

		// Wait before reconnecting
		select {
		case <-b.stopCh:
			return
		case <-time.After(backoff):
		}

		// Exponential backoff
		backoff = backoff * 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

// Stop gracefully stops the bridge.
func (b *Bridge) Stop() {
	if b.stopped.CompareAndSwap(false, true) {
		close(b.stopCh)
		b.disconnect()
	}
}

// connect establishes a WebSocket connection and performs the Airi handshake.
func (b *Bridge) connect() error {
	b.connMu.Lock()
	defer b.connMu.Unlock()

	dialer := websocket.DefaultDialer
	dialer.HandshakeTimeout = 10 * time.Second

	conn, _, err := dialer.Dial(b.url, nil)
	if err != nil {
		return fmt.Errorf("dial %s: %w", b.url, err)
	}
	b.conn = conn

	// Step 1: Authenticate (if token is set)
	if b.token != "" {
		authEvent := NewAuthenticateEvent(b.token, b.identity)
		if err := b.sendEventLocked(authEvent); err != nil {
			conn.Close()
			return fmt.Errorf("send authenticate: %w", err)
		}

		// Wait for authenticated response
		_, msg, err := conn.ReadMessage()
		if err != nil {
			conn.Close()
			return fmt.Errorf("read authenticate response: %w", err)
		}

		event, err := ParseEvent(msg)
		if err != nil {
			conn.Close()
			return fmt.Errorf("parse authenticate response: %w", err)
		}

		if event.Type == "error" {
			errData, _ := ParseData[ErrorData](event)
			conn.Close()
			return fmt.Errorf("authentication failed: %s", errData.Message)
		}

		if event.Type != "module:authenticated" {
			conn.Close()
			return fmt.Errorf("unexpected response type: %s", event.Type)
		}

		slog.Info("[airi-bridge] authenticated with Airi server-runtime")
	}

	// Step 2: Announce as a module
	announceEvent := NewAnnounceEvent(b.moduleName, b.identity)
	if err := b.sendEventLocked(announceEvent); err != nil {
		conn.Close()
		return fmt.Errorf("send announce: %w", err)
	}

	b.connected.Store(true)
	slog.Info("[airi-bridge] connected and announced as module", "name", b.moduleName)

	// Start heartbeat goroutine
	go b.heartbeatLoop()

	return nil
}

// sendEventLocked writes an event while connMu is already held (used inside connect).
func (b *Bridge) sendEventLocked(event AiriEvent) error {
	if b.conn == nil {
		return fmt.Errorf("not connected")
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	return b.conn.WriteMessage(websocket.TextMessage, data)
}

// disconnect closes the WebSocket connection.
func (b *Bridge) disconnect() {
	b.connMu.Lock()
	defer b.connMu.Unlock()

	b.connected.Store(false)
	if b.conn != nil {
		b.conn.Close()
		b.conn = nil
	}
}

// readLoop reads messages from the WebSocket and dispatches them.
func (b *Bridge) readLoop() {
	for {
		select {
		case <-b.stopCh:
			return
		default:
		}

		b.connMu.Lock()
		conn := b.conn
		b.connMu.Unlock()

		if conn == nil {
			return
		}

		_, msg, err := conn.ReadMessage()
		if err != nil {
			if !b.stopped.Load() {
				slog.Warn("[airi-bridge] read error, will reconnect", "err", err)
			}
			b.connected.Store(false)
			return
		}

		b.handleMessage(msg)
	}
}

// handleMessage processes an incoming event from Airi.
func (b *Bridge) handleMessage(raw []byte) {
	event, err := ParseEvent(raw)
	if err != nil {
		slog.Debug("[airi-bridge] unparseable message", "err", err)
		return
	}

	switch event.Type {
	case "transport:connection:heartbeat":
		// Heartbeat pong — ignore
		return

	case "module:authenticated":
		slog.Debug("[airi-bridge] re-authenticated")

	case "module:announced":
		data, _ := ParseData[AnnouncedData](event)
		if data != nil {
			slog.Debug("[airi-bridge] module announced", "name", data.Name)
		}

	case "registry:modules:sync":
		data, _ := ParseData[RegistryModulesSyncData](event)
		if data != nil {
			slog.Debug("[airi-bridge] registry sync", "modules", len(data.Modules))
		}

	case "input:text":
		b.handleInboundText(event)

	case "error":
		errData, _ := ParseData[ErrorData](event)
		if errData != nil {
			slog.Warn("[airi-bridge] error from server", "message", errData.Message)
		}

	default:
		slog.Debug("[airi-bridge] unhandled event type", "type", event.Type)
	}
}

// handleInboundText processes an "input:text" from Airi (user speaking to the desktop pet).
// It routes the message through Yunque's Planner and sends the reply back as another input:text.
func (b *Bridge) handleInboundText(event *AiriEvent) {
	// Don't process our own messages
	if event.Metadata != nil && event.Metadata.Source != nil {
		if event.Metadata.Source.Plugin != nil && event.Metadata.Source.Plugin.ID == "yunque-agent" {
			return
		}
	}

	data, err := ParseData[InputTextData](event)
	if err != nil || data == nil || data.Text == "" {
		return
	}

	b.msgReceived.Add(1)
	slog.Info("[airi-bridge] inbound text from Airi", "text", data.Text)

	// Route through Yunque Planner
	go b.processInboundMessage(data.Text)
}

// processInboundMessage sends a user message through the Planner and pushes the reply to Airi.
func (b *Bridge) processInboundMessage(text string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	airiSys := ""
	if p, ok := b.app.Get("airi_plugin"); ok {
		if ap, ok := p.(*Plugin); ok {
			airiSys = ap.airiSystemPrompt()
		}
	}
	if airiSys == "" {
		airiSys = "你正在通过 Airi 桌宠界面与用户面对面交流。回复要简短自然，像一个活泼的桌面伙伴。"
	}

	msgs := []llm.Message{
		{Role: "system", Content: airiSys},
		{Role: "user", Content: text},
	}

	planReq := planner.PlanRequest{
		Messages:          msgs,
		TenantID:          "default",
		DisableTools:      true,
		DisableDelegation: true,
	}

	result, err := b.app.Planner.Run(ctx, planReq)
	if err != nil {
		slog.Warn("[airi-bridge] planner error for inbound message", "err", err)
		return
	}

	b.pushReplyToAiri(ctx, result.Reply)
}

func (b *Bridge) pushReplyToAiri(_ context.Context, replyText string) {
	slog.Info("[airi-bridge] pushReplyToAiri called", "connected", b.connected.Load(), "len", len(replyText))
	if !b.connected.Load() || replyText == "" {
		slog.Info("[airi-bridge] pushReplyToAiri skipped", "connected", b.connected.Load(), "empty", replyText == "")
		return
	}

	cleaned := trimToolCalls(replyText)
	if cleaned == "" {
		slog.Info("[airi-bridge] pushReplyToAiri: cleaned text is empty, skipping")
		return
	}

	event := NewGenAIChatMessageEvent(cleaned, b.identity)
	if err := b.sendEvent(event); err != nil {
		slog.Warn("[airi-bridge] send reply failed", "err", err)
		return
	}

	b.msgSent.Add(1)
	slog.Info("[airi-bridge] pushed reply to Airi OK", "length", len(cleaned))
}

func (b *Bridge) PushTextToAiri(text string) {
	slog.Info("[airi-bridge] PushTextToAiri called", "connected", b.connected.Load(), "len", len(text))
	if !b.connected.Load() || text == "" {
		slog.Info("[airi-bridge] PushTextToAiri skipped", "connected", b.connected.Load(), "empty", text == "")
		return
	}
	event := NewGenAIChatMessageEvent(text, b.identity)
	raw, _ := json.Marshal(event)
	slog.Info("[airi-bridge] sending event to Airi", "type", event.Type, "data_preview", string(raw)[:min(200, len(raw))])
	if err := b.sendEvent(event); err != nil {
		slog.Warn("[airi-bridge] push text failed", "err", err)
		return
	}
	b.msgSent.Add(1)
	slog.Info("[airi-bridge] PushTextToAiri OK", "length", len(text))
}

// registerReplyHook adds a ReplyHook to the Gateway to intercept all outgoing replies.
// Called lazily after each successful connection since the gateway may not exist yet during plugin init.
func (b *Bridge) registerReplyHook() {
	if !b.hookRegistered.CompareAndSwap(false, true) {
		return // already registered
	}

	// Try immediately
	if b.tryRegisterHook() {
		return
	}

	// Gateway not ready yet — start a background poller
	slog.Info("[airi-bridge] gateway not ready, will poll for it in background")
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		deadline := time.After(30 * time.Second)

		for {
			select {
			case <-b.stopCh:
				return
			case <-deadline:
				slog.Warn("[airi-bridge] gave up waiting for gateway after 30s")
				b.hookRegistered.Store(false)
				return
			case <-ticker.C:
				if b.tryRegisterHook() {
					return
				}
			}
		}
	}()
}

// tryRegisterHook attempts to register the reply hook on the gateway. Returns true on success.
func (b *Bridge) tryRegisterHook() bool {
	rawGw, ok := b.app.Get(agentrt.CompGateway)
	if !ok {
		return false
	}
	gw := rawGw.(*gateway.Gateway)

	gw.AddReplyHook(func(ctx context.Context, msg channel.Message, reply channel.Reply) {
		if !b.connected.Load() {
			return
		}
		b.pushReplyToAiri(ctx, reply.Content)
	})

	slog.Info("[airi-bridge] reply hook registered on gateway")
	return true
}

// heartbeatLoop sends periodic heartbeat pings to keep the connection alive.
func (b *Bridge) heartbeatLoop() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-b.stopCh:
			return
		case <-ticker.C:
			if !b.connected.Load() {
				return
			}
			event := NewHeartbeatPingEvent(b.identity)
			if err := b.sendEvent(event); err != nil {
				slog.Debug("[airi-bridge] heartbeat send failed", "err", err)
				return
			}
		}
	}
}

// sendEvent marshals and sends an AiriEvent over the WebSocket.
func (b *Bridge) sendEvent(event AiriEvent) error {
	b.connMu.Lock()
	defer b.connMu.Unlock()

	if b.conn == nil {
		return fmt.Errorf("not connected")
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	return b.conn.WriteMessage(websocket.TextMessage, data)
}

// ── Utility ──

// trimToolCalls strips raw JSON tool_calls from the reply content.
// Returns empty string if the entire content is a tool_calls block.
func trimToolCalls(content string) string {
	trimmed := content
	for {
		i := 0
		for i < len(trimmed) && (trimmed[i] == ' ' || trimmed[i] == '\t' || trimmed[i] == '\n' || trimmed[i] == '\r') {
			i++
		}
		if i < len(trimmed) && trimmed[i] == '{' {
			depth := 0
			end := -1
			for j := i; j < len(trimmed); j++ {
				if trimmed[j] == '{' {
					depth++
				} else if trimmed[j] == '}' {
					depth--
					if depth == 0 {
						end = j
						break
					}
				}
			}
			if end > 0 {
				block := trimmed[i : end+1]
				var parsed map[string]json.RawMessage
				if err := json.Unmarshal([]byte(block), &parsed); err == nil {
					if _, hasTC := parsed["tool_calls"]; hasTC {
						trimmed = trimmed[end+1:]
						continue
					}
					if _, hasSC := parsed["skill_calls"]; hasSC {
						trimmed = trimmed[end+1:]
						continue
					}
				}
			}
		}
		break
	}

	return strings.TrimSpace(trimmed)
}
