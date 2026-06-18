package connectorspack

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"yunque-agent/internal/connectors"
)

func TestConnectorsPackRoutesDeclareManifestSurface(t *testing.T) {
	handler := New(nil)
	if handler.PackID() != PackID {
		t.Fatalf("unexpected pack id: %s", handler.PackID())
	}
	routes := handler.Routes()
	if len(routes) != 5 {
		t.Fatalf("expected 5 Connectors routes, got %d", len(routes))
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
			t.Fatalf("connector routes should use one method per route: %#v", route)
		}
		byPath[route.Path] = route.Method
	}

	expected := map[string]string{
		"/api/connectors":            http.MethodGet,
		"/api/connectors/detail":     http.MethodGet,
		"/api/connectors/connect":    http.MethodPost,
		"/api/connectors/disconnect": http.MethodPost,
		"/api/connectors/execute":    http.MethodPost,
	}
	for path, method := range expected {
		if byPath[path] != method {
			t.Fatalf("expected %s to expose %s, got %q", path, method, byPath[path])
		}
	}
}

func TestConnectorsPackRouteSpecsStayInSyncWithRoutes(t *testing.T) {
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

func TestConnectorsPackReadsRegistryFromProviderAtRequestTime(t *testing.T) {
	var registry *connectors.Registry
	handler := NewProvider(func() *connectors.Registry { return registry })
	list := routeFor(handler, "/api/connectors")
	if list == nil {
		t.Fatal("missing /api/connectors route")
	}

	rec := httptest.NewRecorder()
	list(rec, httptest.NewRequest(http.MethodGet, "/api/connectors", nil))
	if !strings.Contains(rec.Body.String(), "connector system not initialized") {
		t.Fatalf("expected nil provider registry to be reported, got %s", rec.Body.String())
	}

	registry = connectors.NewRegistry()
	registry.RegisterDef(&connectors.ConnectorDef{
		ID:          "demo",
		Name:        "Demo",
		Description: "Demo connector",
		Icon:        "plug",
		Category:    "test",
		AuthType:    "token",
	})
	rec = httptest.NewRecorder()
	list(rec, httptest.NewRequest(http.MethodGet, "/api/connectors", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 after provider supplies registry, got %d", rec.Code)
	}
	var body struct {
		Connectors []map[string]any `json:"connectors"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode connectors: %v", err)
	}
	if len(body.Connectors) != 1 || body.Connectors[0]["id"] != "demo" {
		t.Fatalf("expected provider-backed connector listing, got %#v", body.Connectors)
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
