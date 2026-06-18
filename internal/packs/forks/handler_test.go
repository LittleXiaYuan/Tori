package forkspack

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"yunque-agent/internal/agentcore/session"
)

func TestForksPackRoutesDeclareManifestSurface(t *testing.T) {
	handler := New(nil, nil)
	if handler.PackID() != PackID {
		t.Fatalf("unexpected pack id: %s", handler.PackID())
	}
	routes := handler.Routes()
	if len(routes) != 3 {
		t.Fatalf("expected 3 Forks routes, got %d", len(routes))
	}
	byPath := map[string][]string{}
	for _, route := range routes {
		if route.Path == "" {
			t.Fatalf("route path is required: %#v", route)
		}
		if route.Handler == nil {
			t.Fatalf("route handler is required: %#v", route)
		}
		methods := append([]string{}, route.Methods...)
		if route.Method != "" {
			methods = append([]string{route.Method}, methods...)
		}
		if len(methods) == 0 {
			t.Fatalf("route method is required: %#v", route)
		}
		byPath[route.Path] = methods
	}

	if strings.Join(byPath["/v1/fork"], ",") != "GET,POST,DELETE" {
		t.Fatalf("expected /v1/fork to expose GET,POST,DELETE, got %#v", byPath["/v1/fork"])
	}
	if strings.Join(byPath["/v1/fork/branch"], ",") != http.MethodPost {
		t.Fatalf("expected /v1/fork/branch to expose POST, got %#v", byPath["/v1/fork/branch"])
	}
	if strings.Join(byPath["/v1/fork/list"], ",") != http.MethodGet {
		t.Fatalf("expected /v1/fork/list to expose GET, got %#v", byPath["/v1/fork/list"])
	}
}

func TestForksPackRouteSpecsStayInSyncWithRoutes(t *testing.T) {
	routes := New(nil, nil).Routes()
	specs := RouteSpecs()
	if len(specs) != 5 {
		t.Fatalf("expected 5 method-aware route specs, got %d", len(specs))
	}
	served := map[string]bool{}
	for _, route := range routes {
		for _, method := range route.Methods {
			served[method+" "+route.Path] = true
		}
		if route.Method != "" {
			served[route.Method+" "+route.Path] = true
		}
	}
	for _, spec := range specs {
		key := spec.Method + " " + spec.Path
		if !served[key] {
			t.Fatalf("route spec not served by pack: %s", key)
		}
		if strings.TrimSpace(spec.Description) == "" {
			t.Fatalf("route spec description is required for %s", key)
		}
	}
}

func TestForksPackReadsTreeFromProviderAtRequestTime(t *testing.T) {
	var tree *session.ForkTree
	handler := NewProvider(func() *session.ForkTree { return tree }, func() *session.ForkPersister { return nil })
	list := routeFor(handler, "/v1/fork/list")
	if list == nil {
		t.Fatal("missing /v1/fork/list route")
	}

	rec := httptest.NewRecorder()
	list(rec, httptest.NewRequest(http.MethodGet, "/v1/fork/list?session_id=s1", nil))
	var empty struct {
		Forks []any `json:"forks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &empty); err != nil {
		t.Fatalf("decode nil-tree forks: %v", err)
	}
	if len(empty.Forks) != 0 {
		t.Fatalf("expected no forks before tree is wired, got %#v", empty.Forks)
	}

	tree = session.NewForkTree()
	tree.Create("s1", []session.ForkMessage{{Role: "user", Content: "hello"}})
	rec = httptest.NewRecorder()
	list(rec, httptest.NewRequest(http.MethodGet, "/v1/fork/list?session_id=s1", nil))
	var body struct {
		Forks []session.Fork `json:"forks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode provider-backed forks: %v", err)
	}
	if len(body.Forks) != 1 || body.Forks[0].SessionID != "s1" {
		t.Fatalf("expected provider-backed fork listing, got %#v", body.Forks)
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
