package lora

import (
	"net/http"
	"strings"
	"testing"
)

func TestLoRAPackRoutesDeclareManifestSurface(t *testing.T) {
	handler := NewHandler(Options{})
	if handler.PackID() != PackID {
		t.Fatalf("unexpected pack id: %s", handler.PackID())
	}
	routes := handler.Routes()
	if len(routes) != 9 {
		t.Fatalf("expected 9 LoRA routes, got %d", len(routes))
	}
	byPath := map[string][]string{}
	for _, route := range routes {
		if route.Path == "" {
			t.Fatalf("route path is required: %#v", route)
		}
		if route.Handler == nil {
			t.Fatalf("route handler is required: %#v", route)
		}
		methods := route.Methods
		if route.Method != "" {
			methods = append([]string{route.Method}, methods...)
		}
		if len(methods) == 0 {
			t.Fatalf("route method is required: %#v", route)
		}
		byPath[route.Path] = methods
	}

	expected := map[string]string{
		"/v1/lora/status":    http.MethodGet,
		"/v1/lora/history":   http.MethodGet,
		"/v1/lora/summary":   http.MethodGet,
		"/v1/lora/preview":   http.MethodGet,
		"/v1/lora/trigger":   http.MethodPost,
		"/v1/lora/rollback":  http.MethodPost,
		"/v1/lora/evolution": http.MethodGet,
	}
	for path, method := range expected {
		if strings.Join(byPath[path], ",") != method {
			t.Fatalf("expected %s to expose %s, got %#v", path, method, byPath[path])
		}
	}
	if strings.Join(byPath["/v1/lora/config"], ",") != "GET,PUT,PATCH" {
		t.Fatalf("expected config to expose GET,PUT,PATCH, got %#v", byPath["/v1/lora/config"])
	}
	if strings.Join(byPath["/v1/lora/distill"], ",") != "GET,POST" {
		t.Fatalf("expected distill to expose GET,POST, got %#v", byPath["/v1/lora/distill"])
	}
}
