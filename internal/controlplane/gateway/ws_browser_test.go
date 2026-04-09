package gateway

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestBrowserWSRequiresAuth(t *testing.T) {
	gw, _ := newTestGateway()
	gw.SetBrowserHub(NewBrowserHub())

	srv := httptest.NewServer(gw)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/browser"
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if conn != nil {
		conn.Close()
	}
	if err == nil {
		t.Fatal("expected websocket auth failure")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		if resp == nil {
			t.Fatalf("expected 401 response, got nil response and err=%v", err)
		}
		t.Fatalf("expected 401 response, got %d", resp.StatusCode)
	}
}

func TestBrowserWSRejectsAPIKeyQuery(t *testing.T) {
	gw, tm := newTestGateway()
	hub := NewBrowserHub()
	gw.SetBrowserHub(hub)
	tenant := tm.Register("browser-test")

	srv := httptest.NewServer(gw)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/browser?key=" + tenant.APIKey
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if conn != nil {
		conn.Close()
	}
	if err == nil {
		t.Fatal("expected websocket dial failure for raw query api key")
	}
}

func TestBrowserWSAcceptsAPIKeyHeader(t *testing.T) {
	gw, tm := newTestGateway()
	hub := NewBrowserHub()
	gw.SetBrowserHub(hub)
	tenant := tm.Register("browser-test")

	srv := httptest.NewServer(gw)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/browser"
	header := http.Header{}
	header.Set("X-API-Key", tenant.APIKey)
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("expected websocket dial success, got %v", err)
	}

	if !waitForCondition(2*time.Second, hub.Connected) {
		conn.Close()
		t.Fatal("browser hub never reported connected")
	}

	_ = conn.Close()
	if !waitForCondition(2*time.Second, func() bool { return !hub.Connected() }) {
		t.Fatal("browser hub never reported disconnected")
	}
}

func TestBrowserExtSessionIssuesTicket(t *testing.T) {
	gw, tm := newTestGateway()
	tenant := tm.Register("browser-test")
	req := httptest.NewRequest(http.MethodPost, "/api/browser/ext/session", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()

	gw.handleBrowserExtSession(w, req.WithContext(contextWithTenant(req.Context(), tenant.ID)))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		OK     bool   `json:"ok"`
		WSURL  string `json:"ws_url"`
		Ticket string `json:"ticket"`
		Nonce  string `json:"nonce"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.OK || resp.Ticket == "" || resp.Nonce == "" || !strings.Contains(resp.WSURL, "/ws/browser") {
		t.Fatalf("unexpected session response: %+v", resp)
	}
}

func TestBrowserWSAcceptsTicketHandshake(t *testing.T) {
	gw, tm := newTestGateway()
	hub := NewBrowserHub()
	gw.SetBrowserHub(hub)
	tenant := tm.Register("browser-test")
	record, err := gw.browserSessions.Issue(tenant.ID)
	if err != nil {
		t.Fatalf("issue browser session: %v", err)
	}

	srv := httptest.NewServer(gw)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/browser?ticket=" + record.Ticket + "&nonce=" + record.Nonce
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("expected websocket dial success, got %v", err)
	}
	defer conn.Close()

	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read challenge: %v", err)
	}
	var msg BrowserResult
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("decode challenge: %v", err)
	}
	if msg.Type != "challenge" || msg.Challenge == "" || msg.Nonce != record.Nonce {
		t.Fatalf("unexpected challenge payload: %+v", msg)
	}

	sum := sha256.Sum256([]byte(record.Ticket + ":" + record.Nonce + ":" + msg.Challenge))
	proof := hex.EncodeToString(sum[:])
	if err := conn.WriteJSON(BrowserResult{Type: "challenge_response", Proof: proof}); err != nil {
		t.Fatalf("write proof: %v", err)
	}

	if !waitForCondition(2*time.Second, func() bool { return hub.ConnectedForTenant(tenant.ID) }) {
		t.Fatal("browser hub never reported tenant-scoped connected state")
	}
}

func TestBrowserWSTicketRejectsNonceMismatch(t *testing.T) {
	gw, tm := newTestGateway()
	hub := NewBrowserHub()
	gw.SetBrowserHub(hub)
	tenant := tm.Register("browser-test")
	record, err := gw.browserSessions.Issue(tenant.ID)
	if err != nil {
		t.Fatalf("issue browser session: %v", err)
	}

	srv := httptest.NewServer(gw)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/browser?ticket=" + record.Ticket + "&nonce=wrong"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if conn != nil {
		conn.Close()
	}
	if err == nil {
		t.Fatal("expected websocket dial failure when nonce mismatches")
	}
}

func TestBrowserWSTicketCannotBeReused(t *testing.T) {
	gw, tm := newTestGateway()
	hub := NewBrowserHub()
	gw.SetBrowserHub(hub)
	tenant := tm.Register("browser-test")
	record, err := gw.browserSessions.Issue(tenant.ID)
	if err != nil {
		t.Fatalf("issue browser session: %v", err)
	}

	srv := httptest.NewServer(gw)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/browser?ticket=" + record.Ticket + "&nonce=" + record.Nonce
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("first dial should succeed: %v", err)
	}
	defer conn.Close()

	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read challenge: %v", err)
	}
	var msg BrowserResult
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("decode challenge: %v", err)
	}
	sum := sha256.Sum256([]byte(record.Ticket + ":" + record.Nonce + ":" + msg.Challenge))
	proof := hex.EncodeToString(sum[:])
	if err := conn.WriteJSON(BrowserResult{Type: "challenge_response", Proof: proof}); err != nil {
		t.Fatalf("write proof: %v", err)
	}
	if !waitForCondition(2*time.Second, func() bool { return hub.ConnectedForTenant(tenant.ID) }) {
		t.Fatal("browser hub never reported tenant-scoped connected state")
	}

	reuseConn, _, reuseErr := websocket.DefaultDialer.Dial(wsURL, nil)
	if reuseConn != nil {
		reuseConn.Close()
	}
	if reuseErr == nil {
		t.Fatal("expected reused ticket dial to fail")
	}
}

func waitForCondition(timeout time.Duration, fn func() bool) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return fn()
}

func TestBrowserHubPendingRequestsFailOnDisconnect(t *testing.T) {
	hub := NewBrowserHub()
	pendingCh := make(chan BrowserResult, 1)

	hub.mu.Lock()
	hub.connected = true
	hub.pending["req-1"] = pendingCh
	hub.mu.Unlock()

	hub.setConn(nil, "", false, "")

	select {
	case result := <-pendingCh:
		if result.Error != "browser extension disconnected" {
			t.Fatalf("unexpected error: %#v", result)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected pending request to fail immediately")
	}

	hub.mu.Lock()
	remaining := len(hub.pending)
	hub.mu.Unlock()
	if remaining != 0 {
		t.Fatalf("expected pending requests to be cleared, got %d", remaining)
	}
}

func TestBrowserHubConnectedForTenant(t *testing.T) {
	hub := NewBrowserHub()
	hub.mu.Lock()
	hub.connected = true
	hub.tenantID = "tenant-a"
	hub.mu.Unlock()

	if !hub.ConnectedForTenant("tenant-a") {
		t.Fatal("expected matching tenant to see connected browser hub")
	}
	if hub.ConnectedForTenant("tenant-b") {
		t.Fatal("expected different tenant to be denied browser hub access")
	}
}

func TestBrowserStatusIsTenantScoped(t *testing.T) {
	gw, tm := newTestGateway()
	hub := NewBrowserHub()
	gw.SetBrowserHub(hub)
	owner := tm.Register("owner")
	other := tm.Register("other")

	hub.mu.Lock()
	hub.connected = true
	hub.tenantID = owner.ID
	hub.version = "0.2.0"
	hub.mu.Unlock()

	ownerReq := httptest.NewRequest("GET", "/v1/browser/status", nil)
	ownerReq.Header.Set("X-API-Key", owner.APIKey)
	ownerRes := httptest.NewRecorder()
	gw.ServeHTTP(ownerRes, ownerReq)
	if ownerRes.Code != http.StatusOK || !strings.Contains(ownerRes.Body.String(), `"connected":true`) {
		t.Fatalf("expected owner to see connected browser, got %d body=%s", ownerRes.Code, ownerRes.Body.String())
	}

	otherReq := httptest.NewRequest("GET", "/v1/browser/status", nil)
	otherReq.Header.Set("X-API-Key", other.APIKey)
	otherRes := httptest.NewRecorder()
	gw.ServeHTTP(otherRes, otherReq)
	if otherRes.Code != http.StatusOK || !strings.Contains(otherRes.Body.String(), `"connected":false`) {
		t.Fatalf("expected other tenant to see disconnected browser, got %d body=%s", otherRes.Code, otherRes.Body.String())
	}
}

func TestValidateBrowserActionRejectsInvalidInput(t *testing.T) {
	tests := []BrowserAction{
		{Type: "browser_navigate", URL: "javascript:alert(1)"},
		{Type: "browser_scroll", Direction: "sideways"},
		{Type: "browser_input"},
		{Type: "browser_switch_tab", TabID: 0},
		{Type: "session_status", Status: "hijack"},
		{Type: "browser_root_shell"},
	}

	for _, tc := range tests {
		if err := validateBrowserAction(tc); err == nil {
			t.Fatalf("expected validation failure for %+v", tc)
		}
	}
}

func TestValidateBrowserActionAcceptsSupportedInput(t *testing.T) {
	tests := []BrowserAction{
		{Type: "browser_navigate", URL: "https://example.com"},
		{Type: "browser_new_tab", URL: "about:blank"},
		{Type: "browser_scroll", Direction: "down"},
		{Type: "browser_input", Text: "hello"},
		{Type: "browser_switch_tab", TabID: 1},
		{Type: "session_status", Status: "take_over"},
	}

	for _, tc := range tests {
		if err := validateBrowserAction(tc); err != nil {
			t.Fatalf("expected validation success for %+v, got %v", tc, err)
		}
	}
}
