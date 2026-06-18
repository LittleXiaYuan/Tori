package costpack

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"yunque-agent/internal/agentcore/costtrack"
)

func TestCostPackRoutesDeclareManifestSurface(t *testing.T) {
	handler := New(nil)
	if handler.PackID() != PackID {
		t.Fatalf("unexpected pack id: %s", handler.PackID())
	}
	routes := handler.Routes()
	if len(routes) != 7 {
		t.Fatalf("expected 7 Cost routes, got %d", len(routes))
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
			t.Fatalf("cost routes should use one method per route: %#v", route)
		}
		byPath[route.Path] = route.Method
	}

	expected := map[string]string{
		"/v1/cost/summary":       http.MethodGet,
		"/v1/cost/budget":        http.MethodPost,
		"/v1/cost/task":          http.MethodGet,
		"/v1/cost/task/timeline": http.MethodGet,
		"/v1/cost/breakdown":     http.MethodGet,
		"/v1/cost/history":       http.MethodGet,
		"/v1/cost/alerts":        http.MethodGet,
	}
	for path, method := range expected {
		if byPath[path] != method {
			t.Fatalf("expected %s to expose %s, got %q", path, method, byPath[path])
		}
	}
}

func TestCostPackRouteSpecsStayInSyncWithRoutes(t *testing.T) {
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

func TestCostPackReadsTrackerFromProviderAtRequestTime(t *testing.T) {
	var tracker *costtrack.Tracker
	handler := NewProvider(func() *costtrack.Tracker { return tracker })
	summary := routeFor(handler, "/v1/cost/summary")
	if summary == nil {
		t.Fatal("missing /v1/cost/summary route")
	}

	rec := httptest.NewRecorder()
	summary(rec, httptest.NewRequest(http.MethodGet, "/v1/cost/summary", nil))
	if !strings.Contains(rec.Body.String(), "not configured") {
		t.Fatalf("expected nil provider tracker to be reported, got %s", rec.Body.String())
	}

	tracker = costtrack.New()
	tracker.RecordExt(costtrack.RecordOpts{Model: "test-model", TokensIn: 100, TokensOut: 50})
	rec = httptest.NewRecorder()
	summary(rec, httptest.NewRequest(http.MethodGet, "/v1/cost/summary", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 after provider supplies tracker, got %d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode summary: %v", err)
	}
	if body["summary"] == nil {
		t.Fatalf("expected cost summary after late provider wiring, got %v", body)
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
