package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
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
	tenantID  string
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

// ConnectedForTenant returns true when the connected browser extension belongs to the same tenant.
func (h *BrowserHub) ConnectedForTenant(tenantID string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.connected && h.tenantID != "" && h.tenantID == tenantID
}

// TenantID returns the tenant currently owning the browser extension connection.
func (h *BrowserHub) TenantID() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.tenantID
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
	if err := validateBrowserAction(action); err != nil {
		return BrowserResult{OK: false, Error: err.Error()}, err
	}

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

func validateBrowserAction(action BrowserAction) error {
	switch action.Type {
	case "browser_navigate":
		return validateBrowserURL(action.URL, true)
	case "browser_new_tab":
		if strings.TrimSpace(action.URL) == "" {
			return nil
		}
		return validateBrowserURL(action.URL, false)
	case "browser_click":
		if action.Target == nil || strings.TrimSpace(action.Target.Strategy) == "" {
			return fmt.Errorf("browser_click requires a target")
		}
	case "browser_input":
		if action.Text == "" {
			return fmt.Errorf("browser_input requires text")
		}
	case "browser_scroll":
		switch action.Direction {
		case "up", "down", "left", "right":
		default:
			return fmt.Errorf("browser_scroll direction must be up, down, left, or right")
		}
	case "browser_press_key":
		if strings.TrimSpace(action.Key) == "" {
			return fmt.Errorf("browser_press_key requires key")
		}
	case "browser_switch_tab", "browser_close_tab":
		if action.TabID <= 0 {
			return fmt.Errorf("%s requires a positive tabId", action.Type)
		}
	case "browser_screenshot", "browser_view", "browser_get_content", "browser_get_structured_content",
		"browser_move_mouse", "browser_mark_elements", "browser_unmark_elements",
		"browser_get_elements", "browser_list_tabs":
		return nil
	case "session_status":
		switch action.Status {
		case "paused", "take_over", "resumed", "running", "stopped":
			return nil
		default:
			return fmt.Errorf("session_status contains unsupported status")
		}
	default:
		return fmt.Errorf("unsupported browser action type")
	}
	return nil
}

func validateBrowserURL(raw string, requireValue bool) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		if requireValue {
			return fmt.Errorf("url is required")
		}
		return nil
	}
	if raw == "about:blank" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("url must be a valid absolute http(s) URL")
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		return nil
	default:
		return fmt.Errorf("url scheme must be http or https")
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

func (h *BrowserHub) setConn(conn *websocket.Conn, tenantID string) {
	h.mu.Lock()
	if h.conn != nil && h.conn != conn {
		h.conn.Close()
	}
	h.conn = conn
	h.connected = conn != nil
	if conn != nil {
		h.tenantID = tenantID
	} else {
		h.tenantID = ""
	}
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
	token := authTokenFromHeaders(r)
	if token == "" {
		token = authTokenFromQuery(r)
	}
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

	hub.setConn(conn, tenantID)
	slog.Info("browser extension connected", "tenant", tenantID)

	done := make(chan struct{})
	defer func() {
		close(done)
		hub.setConn(nil, "")
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
