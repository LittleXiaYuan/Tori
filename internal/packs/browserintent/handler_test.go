package browserintent

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeBrowserGateway struct {
	packCalls    int
	sessionCalls int
}

func (g *fakeBrowserGateway) HandleBrowserIntentPack(w http.ResponseWriter, _ *http.Request) {
	g.packCalls++
	w.WriteHeader(http.StatusNoContent)
}

func (g *fakeBrowserGateway) HandleBrowserIntentSession(w http.ResponseWriter, _ *http.Request) {
	g.sessionCalls++
	w.WriteHeader(http.StatusAccepted)
}

func TestBrowserIntentHandlerRoutesExposeSurface(t *testing.T) {
	gateway := &fakeBrowserGateway{}
	handler := NewHandler(gateway)

	if handler.PackID() != PackID {
		t.Fatalf("PackID = %q, want %q", handler.PackID(), PackID)
	}

	routes := handler.Routes()
	if len(routes) != 13 {
		t.Fatalf("expected 13 Browser Intent routes, got %d", len(routes))
	}

	byPath := map[string]string{}
	for _, route := range routes {
		if route.Path == "" || route.Handler == nil {
			t.Fatalf("route must declare path and handler: %#v", route)
		}
		if route.Method == "" {
			t.Fatalf("route must declare method: %#v", route)
		}
		byPath[route.Path] = route.Method
	}

	expected := map[string]string{
		"/v1/browser/status":             http.MethodGet,
		"/v1/browser/config":             http.MethodGet,
		"/v1/browser/navigate":           http.MethodPost,
		"/v1/browser/screenshot":         http.MethodGet,
		"/v1/browser/ocr":                http.MethodPost,
		"/v1/browser/screenshot/latest":  http.MethodGet,
		"/v1/browser/opp/pending":        http.MethodGet,
		"/v1/browser/opp/decide":         http.MethodPost,
		"/api/browser/ext/status":        http.MethodGet,
		"/api/browser/ext/session":       http.MethodPost,
		"/api/browser/ext/action":        http.MethodPost,
		"/api/browser/ext/scenarios":     http.MethodGet,
		"/api/browser/ext/scenarios/run": http.MethodPost,
	}
	for path, method := range expected {
		if got := byPath[path]; got != method {
			t.Fatalf("expected %s to expose %s, got %q", path, method, got)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/browser/status", nil)
	w := httptest.NewRecorder()
	routes[0].Handler(w, req)
	if w.Code != http.StatusNoContent || gateway.packCalls != 1 {
		t.Fatalf("expected pack route to delegate to gateway, status=%d calls=%d", w.Code, gateway.packCalls)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/browser/ext/session", nil)
	w = httptest.NewRecorder()
	for _, route := range routes {
		if route.Path == "/api/browser/ext/session" {
			route.Handler(w, req)
			break
		}
	}
	if w.Code != http.StatusAccepted || gateway.sessionCalls != 1 {
		t.Fatalf("expected session route to use session bridge, status=%d calls=%d", w.Code, gateway.sessionCalls)
	}
}
