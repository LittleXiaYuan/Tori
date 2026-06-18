package notificationspack

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"yunque-agent/internal/agentcore/notify"
)

func TestNotificationsPackRoutesDeclareManifestSurface(t *testing.T) {
	handler := New(nil)
	if handler.PackID() != PackID {
		t.Fatalf("unexpected pack id: %s", handler.PackID())
	}
	routes := handler.Routes()
	if len(routes) != 6 {
		t.Fatalf("expected 6 Notifications routes, got %d", len(routes))
	}
	byPath := map[string]string{}
	for _, route := range routes {
		if route.Path == "" {
			t.Fatalf("route path is required: %#v", route)
		}
		if route.Handler == nil {
			t.Fatalf("route handler is required: %#v", route)
		}
		if route.Method == "" {
			t.Fatalf("route method is required: %#v", route)
		}
		if len(route.Methods) > 0 {
			t.Fatalf("notification routes should use one method per route: %#v", route)
		}
		byPath[route.Path] = route.Method
	}

	expected := map[string]string{
		"/api/notify/channels": http.MethodGet,
		"/api/notify/add":      http.MethodPost,
		"/api/notify/remove":   http.MethodPost,
		"/api/notify/toggle":   http.MethodPost,
		"/api/notify/test":     http.MethodPost,
		"/api/notify/share":    http.MethodPost,
	}
	for path, method := range expected {
		if byPath[path] != method {
			t.Fatalf("expected %s to expose %s, got %q", path, method, byPath[path])
		}
	}
}

func TestNotificationsPackRouteSpecsStayInSyncWithRoutes(t *testing.T) {
	routes := New(nil).Routes()
	specs := RouteSpecs()
	if len(routes) != len(specs) {
		t.Fatalf("route/spec count mismatch: routes=%d specs=%d", len(routes), len(specs))
	}
	served := map[string]string{}
	for _, route := range routes {
		served[route.Method+" "+route.Path] = route.Path
	}
	for _, spec := range specs {
		key := spec.Method + " " + spec.Path
		if served[key] == "" {
			t.Fatalf("route spec not served by pack: %s", key)
		}
		if strings.TrimSpace(spec.Description) == "" {
			t.Fatalf("route spec description is required for %s", key)
		}
	}
}

func TestNotificationsPackReadsNotifierFromProviderAtRequestTime(t *testing.T) {
	var notifier *notify.Notifier
	handler := NewProvider(func() *notify.Notifier { return notifier })
	channels := routeFor(handler, "/api/notify/channels")
	if channels == nil {
		t.Fatal("missing /api/notify/channels route")
	}

	rec := httptest.NewRecorder()
	channels(rec, httptest.NewRequest(http.MethodGet, "/api/notify/channels", nil))
	var body struct {
		Channels []map[string]any `json:"channels"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode nil-notifier channels: %v", err)
	}
	if len(body.Channels) != 0 {
		t.Fatalf("expected no channels before notifier is wired, got %#v", body.Channels)
	}

	notifier = notify.New()
	notifier.AddChannel(&notify.Channel{ID: "demo", Type: "webhook", Name: "Demo", URL: "https://example.com/hook", Enabled: true})
	rec = httptest.NewRecorder()
	channels(rec, httptest.NewRequest(http.MethodGet, "/api/notify/channels", nil))
	body = struct {
		Channels []map[string]any `json:"channels"`
	}{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode provider-backed channels: %v", err)
	}
	if len(body.Channels) != 1 || body.Channels[0]["id"] != "demo" {
		t.Fatalf("expected provider-backed channel listing, got %#v", body.Channels)
	}
}

func routeFor(h *Handler, path string) http.HandlerFunc {
	for _, route := range h.Routes() {
		if route.Path == path {
			return route.Handler
		}
	}
	return nil
}
