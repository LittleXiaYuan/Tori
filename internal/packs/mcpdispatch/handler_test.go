package mcpdispatchpack

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"yunque-agent/pkg/packruntime"
)

func TestMCPDispatchPackRouteSurface(t *testing.T) {
	handler := NewProvider(nil, nil)
	if handler.PackID() != PackID {
		t.Fatalf("unexpected pack id: %s", handler.PackID())
	}
	var _ packruntime.Module = handler

	routes := handler.Routes()
	if len(routes) != 7 {
		t.Fatalf("expected 7 routes, got %d", len(routes))
	}
	byPath := map[string][]string{}
	byAuth := map[string]packruntime.BackendRouteAuthMode{}
	for _, route := range routes {
		methods := route.Methods
		if len(methods) == 0 {
			methods = []string{route.Method}
		}
		byPath[route.Path] = methods
		byAuth[route.Path] = route.Auth
	}

	want := map[string][]string{
		"/mcp/v1":              {http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodPost},
		"/v1/workers":          {http.MethodGet},
		"/v1/workers/detail":   {http.MethodGet},
		"/v1/workers/remove":   {http.MethodPost},
		"/v1/dispatch/queue":   {http.MethodGet},
		"/v1/dispatch/enqueue": {http.MethodPost},
		"/v1/workers/config":   {http.MethodGet},
	}
	for path, methods := range want {
		got, ok := byPath[path]
		if !ok {
			t.Fatalf("missing route %s", path)
		}
		if len(got) != len(methods) {
			t.Fatalf("%s methods = %v, want %v", path, got, methods)
		}
		for i := range methods {
			if got[i] != methods[i] {
				t.Fatalf("%s methods = %v, want %v", path, got, methods)
			}
		}
	}
	if byAuth["/mcp/v1"] != packruntime.BackendRouteAuthPassthrough {
		t.Fatalf("/mcp/v1 must use passthrough auth, got %q", byAuth["/mcp/v1"])
	}
}

func TestMCPDispatchRouteSpecsSyncWithManifest(t *testing.T) {
	manifestPath := filepath.Join("..", "..", "..", "packs", "official", "mcp-dispatch-pack", "pack.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var manifest packruntime.Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}

	got := map[string]bool{}
	for _, spec := range RouteSpecs() {
		got[spec.Method+" "+spec.Path] = true
	}
	for _, spec := range manifest.Backend.RouteSpecs {
		key := spec.Method + " " + spec.Path
		if !got[key] {
			t.Fatalf("manifest routeSpec not served by pack: %s", key)
		}
		delete(got, key)
	}
	if len(got) != 0 {
		t.Fatalf("pack has routeSpecs missing from manifest: %v", got)
	}
}

func TestMCPDispatchMethodSensitiveAuth(t *testing.T) {
	authCalls := 0
	handler := NewProvider(func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			authCalls++
			http.Error(w, "auth required", http.StatusUnauthorized)
		}
	}, nil)

	route := findRoute(t, handler, "/mcp/v1")
	getRec := httptest.NewRecorder()
	route.Handler(getRec, httptest.NewRequest(http.MethodGet, "/mcp/v1", nil))
	if authCalls != 0 {
		t.Fatalf("GET probe must not call auth, calls=%d", authCalls)
	}

	postRec := httptest.NewRecorder()
	route.Handler(postRec, httptest.NewRequest(http.MethodPost, "/mcp/v1", nil))
	if authCalls != 1 {
		t.Fatalf("POST must call host auth exactly once, calls=%d", authCalls)
	}
	if postRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthenticated POST to be 401, got %d", postRec.Code)
	}
}

func findRoute(t *testing.T, handler *Handler, path string) packruntime.BackendRoute {
	t.Helper()
	for _, route := range handler.Routes() {
		if route.Path == path {
			return route
		}
	}
	t.Fatalf("route %s not found", path)
	return packruntime.BackendRoute{}
}
