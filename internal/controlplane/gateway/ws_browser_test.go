package gateway

import (
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

func TestBrowserWSAcceptsAPIKeyQuery(t *testing.T) {
	gw, tm := newTestGateway()
	hub := NewBrowserHub()
	gw.SetBrowserHub(hub)
	tenant := tm.Register("browser-test")

	srv := httptest.NewServer(gw)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/browser?key=" + tenant.APIKey
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
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

	hub.setConn(nil)

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
