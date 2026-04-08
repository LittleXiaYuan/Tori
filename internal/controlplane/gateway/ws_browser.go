package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

var browserUpgrader = websocket.Upgrader{
	CheckOrigin:     allowBrowserWSOrigin,
	ReadBufferSize:  64 * 1024,
	WriteBufferSize: 256 * 1024,
}

// BrowserAction represents a command sent to the browser extension.
type BrowserAction struct {
	Type        string         `json:"type"`
	URL         string         `json:"url,omitempty"`
	Target      *ActionTarget  `json:"target,omitempty"`
	Text        string         `json:"text,omitempty"`
	PressEnter  bool           `json:"press_enter,omitempty"`
	Key         string         `json:"key,omitempty"`
	Direction   string         `json:"direction,omitempty"`
	ToEnd       bool           `json:"to_end,omitempty"`
	CoordinateX float64        `json:"coordinate_x,omitempty"`
	CoordinateY float64        `json:"coordinate_y,omitempty"`
	TabID       int            `json:"tabId,omitempty"`
	Status      string         `json:"status,omitempty"`
	Title       string         `json:"sessionTitle,omitempty"`
	Extra       map[string]any `json:"extra,omitempty"`
}

// ActionTarget specifies how to locate a page element.
type ActionTarget struct {
	Strategy    string  `json:"strategy"`
	Selector    string  `json:"selector,omitempty"`
	Index       int     `json:"index,omitempty"`
	CoordinateX float64 `json:"coordinateX,omitempty"`
	CoordinateY float64 `json:"coordinateY,omitempty"`
}

// BrowserCommand is a command envelope sent to the extension.
type BrowserCommand struct {
	RequestID string        `json:"requestId"`
	Action    BrowserAction `json:"action"`
}

// BrowserResult is the response from the extension.
type BrowserResult struct {
	Type       string          `json:"type"`
	RequestID  string          `json:"requestId,omitempty"`
	OK         bool            `json:"ok"`
	Error      string          `json:"error,omitempty"`
	Screenshot string          `json:"screenshot,omitempty"`
	Content    string          `json:"content,omitempty"`
	Title      string          `json:"title,omitempty"`
	URL        string          `json:"url,omitempty"`
	Status     string          `json:"status,omitempty"`
	Version    string          `json:"version,omitempty"`
	TabID      int             `json:"tabId,omitempty"`
	Takeover   bool            `json:"takeover,omitempty"`
	Total      int             `json:"total,omitempty"`
	Elements   json.RawMessage `json:"elements,omitempty"`
	Tabs       json.RawMessage `json:"tabs,omitempty"`
	Meta       json.RawMessage `json:"meta,omitempty"`
	Headings   json.RawMessage `json:"headings,omitempty"`
	Links      json.RawMessage `json:"links,omitempty"`
	Images     json.RawMessage `json:"images,omitempty"`
}

// BrowserHub manages the WebSocket connection to the browser extension.
type BrowserHub struct {
	mu        sync.Mutex
	writeMu   sync.Mutex
	conn      *websocket.Conn
	connected bool
	version   string
	pending   map[string]chan BrowserResult
	listeners []func(BrowserResult)
	seq       uint64
}

// NewBrowserHub creates a new BrowserHub.
func NewBrowserHub() *BrowserHub {
	return &BrowserHub{
		pending: make(map[string]chan BrowserResult),
	}
}

// Connected returns true if the browser extension is connected.
func (h *BrowserHub) Connected() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.connected
}

// OnEvent registers a callback for browser events (screenshots, status changes).
func (h *BrowserHub) OnEvent(fn func(BrowserResult)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.listeners = append(h.listeners, fn)
}

func (h *BrowserHub) writeMessage(messageType int, data []byte) error {
	h.mu.Lock()
	conn := h.conn
	connected := h.connected
	h.mu.Unlock()
	if !connected || conn == nil {
		return fmt.Errorf("browser extension not connected")
	}
	h.writeMu.Lock()
	defer h.writeMu.Unlock()
	return conn.WriteMessage(messageType, data)
}

// SendAction sends a command to the browser extension and waits for the result.
func (h *BrowserHub) SendAction(ctx context.Context, action BrowserAction) (BrowserResult, error) {
	h.mu.Lock()
	if !h.connected || h.conn == nil {
		h.mu.Unlock()
		return BrowserResult{OK: false, Error: "browser extension not connected"}, nil
	}

	reqID := fmt.Sprintf("browser-%d-%d", time.Now().UnixNano(), atomic.AddUint64(&h.seq, 1))
	ch := make(chan BrowserResult, 1)
	h.pending[reqID] = ch
	h.mu.Unlock()

	cmd := BrowserCommand{RequestID: reqID, Action: action}
	data, _ := json.Marshal(cmd)
	err := h.writeMessage(websocket.TextMessage, data)
	if err != nil {
		h.mu.Lock()
		delete(h.pending, reqID)
		h.mu.Unlock()
		return BrowserResult{OK: false, Error: err.Error()}, err
	}

	select {
	case result := <-ch:
		return result, nil
	case <-ctx.Done():
		h.mu.Lock()
		delete(h.pending, reqID)
		h.mu.Unlock()
		return BrowserResult{OK: false, Error: "timeout"}, ctx.Err()
	case <-time.After(30 * time.Second):
		h.mu.Lock()
		delete(h.pending, reqID)
		h.mu.Unlock()
		return BrowserResult{OK: false, Error: "action timeout"}, nil
	}
}

// SendActionRaw sends a raw JSON action and returns raw JSON result.
func (h *BrowserHub) SendActionRaw(ctx context.Context, actionJSON json.RawMessage) (json.RawMessage, error) {
	var action BrowserAction
	if err := json.Unmarshal(actionJSON, &action); err != nil {
		return nil, err
	}
	result, err := h.SendAction(ctx, action)
	if err != nil {
		return nil, err
	}
	return json.Marshal(result)
}

func (h *BrowserHub) failPendingLocked(message string) {
	pending := h.pending
	h.pending = make(map[string]chan BrowserResult)
	for reqID, ch := range pending {
		select {
		case ch <- BrowserResult{RequestID: reqID, OK: false, Error: message}:
		default:
		}
		close(ch)
	}
}

func (h *BrowserHub) handleResult(result BrowserResult) {
	h.mu.Lock()
	if result.RequestID != "" {
		if ch, ok := h.pending[result.RequestID]; ok {
			delete(h.pending, result.RequestID)
			h.mu.Unlock()
			ch <- result
			return
		}
	}
	listeners := make([]func(BrowserResult), len(h.listeners))
	copy(listeners, h.listeners)
	h.mu.Unlock()

	for _, fn := range listeners {
		go fn(result)
	}
}

func (h *BrowserHub) setConn(conn *websocket.Conn) {
	h.mu.Lock()
	if h.conn != nil && h.conn != conn {
		h.conn.Close()
	}
	h.conn = conn
	h.connected = conn != nil
	if conn == nil {
		h.failPendingLocked("browser extension disconnected")
	}
	h.mu.Unlock()
}

func allowBrowserWSOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	if u.Scheme == "chrome-extension" {
		return true
	}
	switch u.Hostname() {
	case "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}

// handleBrowserWS is the HTTP handler for /ws/browser.
func (g *Gateway) handleBrowserWS(w http.ResponseWriter, r *http.Request) {
	token := authTokenFromRequest(r)
	if token == "" {
		http.Error(w, "missing credentials", http.StatusUnauthorized)
		return
	}

	tenantID := ""
	switch {
	case g.tenants != nil && g.tenants.ByAPIKey(token) != nil:
		tenantID = g.tenants.ByAPIKey(token).ID
	case g.jwtCfg != nil:
		claims, err := ValidateJWT(*g.jwtCfg, token)
		if err == nil {
			tenantID = claims.TenantID
		}
	}
	if tenantID == "" {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	conn, err := browserUpgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("browser ws upgrade failed", "err", err)
		return
	}

	hub := g.browserHub
	if hub == nil {
		conn.Close()
		return
	}

	hub.setConn(conn)
	slog.Info("browser extension connected", "tenant", tenantID)

	done := make(chan struct{})
	defer func() {
		close(done)
		hub.setConn(nil)
		conn.Close()
		slog.Info("browser extension disconnected", "tenant", tenantID)
	}()

	conn.SetReadLimit(1 << 20) // 1MB max message
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		return nil
	})

	// Ping ticker
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				if err := hub.writeMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			}
		}
	}()

	for {
		conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		_, data, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				slog.Warn("browser ws read error", "err", err)
			}
			break
		}

		var result BrowserResult
		if err := json.Unmarshal(data, &result); err != nil {
			slog.Warn("browser ws invalid json", "err", err)
			continue
		}

		switch result.Type {
		case "hello":
			hub.mu.Lock()
			hub.version = result.Version
			hub.mu.Unlock()
			slog.Info("browser extension hello", "version", result.Version)

		case "action_result":
			hub.handleResult(result)

		case "session_status":
			hub.handleResult(result)

		default:
			hub.handleResult(result)
		}
	}
}
