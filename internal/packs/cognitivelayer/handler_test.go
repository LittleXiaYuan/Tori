package cognitivelayerpack

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"yunque-agent/internal/agentcore/planner"
	"yunque-agent/pkg/packruntime"
)

func TestCognitiveLayerPackV2RouteSpecsAndAdminComposition(t *testing.T) {
	var _ packruntime.Module = (*Handler)(nil)

	var calls []string
	auth := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			calls = append(calls, "auth")
			next(w, r)
		}
	}
	admin := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			calls = append(calls, "admin")
			next(w, r)
		}
	}

	h := NewProvider(auth, admin)
	if h.PackID() != PackID {
		t.Fatalf("PackID=%q, want %q", h.PackID(), PackID)
	}
	if got := len(h.Routes()); got != 1 {
		t.Fatalf("Routes len=%d, want 1", got)
	}
	route := h.Routes()[0]
	if route.Path != Route || route.Auth != packruntime.BackendRouteAuthPassthrough {
		t.Fatalf("unexpected route: %#v", route)
	}
	if got := len(RouteSpecs()); got != 2 {
		t.Fatalf("RouteSpecs len=%d, want 2", got)
	}
	if err := h.Init(nil); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := h.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := h.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	rec := httptest.NewRecorder()
	route.Handler(rec, httptest.NewRequest(http.MethodGet, Route, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Join(calls, ",") != "auth,admin" {
		t.Fatalf("expected auth then admin wrappers, got %v", calls)
	}
}

func TestCognitiveLayerPackReadsAndTogglesPlannerSwitch(t *testing.T) {
	original := planner.CognitiveLayerEnabled()
	t.Cleanup(func() { planner.SetCognitiveLayerEnabled(original) })

	h := NewProvider(nil, nil)
	route := h.Routes()[0]

	planner.SetCognitiveLayerEnabled(true)
	rec := httptest.NewRecorder()
	route.Handler(rec, httptest.NewRequest(http.MethodPost, Route, strings.NewReader(`{"enabled":false}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("toggle status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode toggle: %v", err)
	}
	if body.Enabled || planner.CognitiveLayerEnabled() {
		t.Fatalf("expected cognitive layer to be disabled, body=%s", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	route.Handler(rec, httptest.NewRequest(http.MethodGet, Route, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("read status=%d body=%s", rec.Code, rec.Body.String())
	}
	body.Enabled = true
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode read: %v", err)
	}
	if body.Enabled {
		t.Fatalf("expected read to report disabled, body=%s", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	route.Handler(rec, httptest.NewRequest(http.MethodPost, Route, strings.NewReader(`{}`)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid toggle status=%d body=%s", rec.Code, rec.Body.String())
	}
}
