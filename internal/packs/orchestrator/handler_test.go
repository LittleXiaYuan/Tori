package orchestratorpack

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"yunque-agent/internal/orchestrator"
	"yunque-agent/pkg/packruntime"
)

func TestOrchestratorPackRouteSurface(t *testing.T) {
	handler := NewProvider(nil, nil, nil)
	if handler.PackID() != PackID {
		t.Fatalf("unexpected pack id: %s", handler.PackID())
	}
	var _ packruntime.Module = handler

	routes := handler.Routes()
	if len(routes) != 8 {
		t.Fatalf("expected 8 routes, got %d", len(routes))
	}
	byPath := map[string][]string{}
	for _, route := range routes {
		methods := route.Methods
		if len(methods) == 0 {
			methods = []string{route.Method}
		}
		byPath[route.Path] = methods
	}

	want := map[string][]string{
		"/v1/orchestrator/status":       {http.MethodGet},
		"/v1/orchestrator/toggle":       {http.MethodPost},
		"/v1/orchestrator/sessions":     {http.MethodGet},
		"/v1/orchestrator/detect":       {http.MethodGet},
		"/v1/orchestrator/adapters/add": {http.MethodPost},
		"/v1/orchestrator/events":       {http.MethodGet},
		"/v1/orchestrator/events/task":  {http.MethodGet},
		"/v1/orchestrator/policy":       {http.MethodGet, http.MethodPut},
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
}

func TestOrchestratorRouteSpecsSyncWithManifest(t *testing.T) {
	manifestPath := filepath.Join("..", "..", "..", "packs", "official", "orchestrator-pack", "pack.json")
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

func TestOrchestratorPackLateBoundLauncher(t *testing.T) {
	var launcher *orchestrator.Launcher
	handler := NewProvider(nil, func() *orchestrator.Launcher { return launcher }, nil)
	route := findRoute(t, handler, "/v1/orchestrator/sessions")

	rec := httptest.NewRecorder()
	route.Handler(rec, httptest.NewRequest(http.MethodGet, "/v1/orchestrator/sessions", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected nil launcher to return 200, got %d", rec.Code)
	}

	launcher = orchestrator.NewLauncher()
	rec = httptest.NewRecorder()
	route.Handler(rec, httptest.NewRequest(http.MethodGet, "/v1/orchestrator/sessions", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected late-bound launcher to return 200, got %d", rec.Code)
	}
	var body struct {
		Sessions []any `json:"sessions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode sessions: %v", err)
	}
	if body.Sessions == nil {
		t.Fatalf("expected sessions array, got %#v", body)
	}
}

func TestOrchestratorPackToggleUsesLateBoundBaseContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	handler := NewProvider(nil, nil, func() context.Context { return ctx })
	if got := handler.baseContext(); got != ctx {
		t.Fatal("expected late-bound base context")
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
