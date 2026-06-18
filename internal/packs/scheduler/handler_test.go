package schedulerpack

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"yunque-agent/internal/execution/scheduler"
)

func TestSchedulerPackRoutesDeclareManifestSurface(t *testing.T) {
	handler := New(nil)
	if handler.PackID() != PackID {
		t.Fatalf("unexpected pack id: %s", handler.PackID())
	}
	routes := handler.Routes()
	if len(routes) != 3 {
		t.Fatalf("expected 3 Scheduler routes, got %d", len(routes))
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
			t.Fatalf("scheduler routes should use one method per route: %#v", route)
		}
		byPath[route.Path] = route.Method
	}

	expected := map[string]string{
		"/v1/scheduler/jobs":   http.MethodGet,
		"/v1/scheduler/add":    http.MethodPost,
		"/v1/scheduler/remove": http.MethodPost,
	}
	for path, method := range expected {
		if byPath[path] != method {
			t.Fatalf("expected %s to expose %s, got %q", path, method, byPath[path])
		}
	}
}

func TestSchedulerPackRouteSpecsStayInSyncWithRoutes(t *testing.T) {
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

func TestSchedulerPackReadsSchedulerFromProviderAtRequestTime(t *testing.T) {
	var sched *scheduler.Scheduler
	handler := NewProvider(func() *scheduler.Scheduler { return sched })
	jobs := routeFor(handler, "/v1/scheduler/jobs")
	if jobs == nil {
		t.Fatal("missing /v1/scheduler/jobs route")
	}

	rec := httptest.NewRecorder()
	jobs(rec, httptest.NewRequest(http.MethodGet, "/v1/scheduler/jobs", nil))
	if !strings.Contains(rec.Body.String(), "scheduler not available") {
		t.Fatalf("expected nil provider scheduler to be reported, got %s", rec.Body.String())
	}

	sched = scheduler.New(func(ctx context.Context, job scheduler.Job) {})
	sched.Add(scheduler.Job{ID: "demo", Name: "Demo", Interval: time.Hour, Prompt: "ping"})
	rec = httptest.NewRecorder()
	jobs(rec, httptest.NewRequest(http.MethodGet, "/v1/scheduler/jobs", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 after provider supplies scheduler, got %d", rec.Code)
	}
	var body struct {
		Count int `json:"count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode jobs: %v", err)
	}
	if body.Count != 1 {
		t.Fatalf("expected provider-backed job listing, got count=%d body=%s", body.Count, rec.Body.String())
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
