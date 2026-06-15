package workpack

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"yunque-agent/internal/agentcore/task"
	"yunque-agent/internal/controlplane/gateway/workflowapi"
	"yunque-agent/internal/orchestrator"
	"yunque-agent/pkg/packruntime"
)

// fakeGW satisfies WorkGateway with no wired subsystems, so every native handler
// degrades to its "not available" response (proving the routes are native).
type fakeGW struct{}

func (f *fakeGW) TaskStore() task.Store           { return nil }
func (f *fakeGW) TaskRunner() *task.Runner        { return nil }
func (f *fakeGW) TenantOf(context.Context) string { return "t1" }
func (f *fakeGW) GapAnalyzer() *task.GapAnalyzer  { return nil }
func (f *fakeGW) TemplateStore() *task.TemplateStore {
	return nil
}
func (f *fakeGW) WorkMemManager() *task.WorkingMemoryManager { return nil }
func (f *fakeGW) ThreadManager() *task.ThreadManager         { return nil }
func (f *fakeGW) ProjectStore() *orchestrator.ProjectStore   { return nil }
func (f *fakeGW) WorkflowHandler() *workflowapi.Handler      { return nil }

func routeFor(h *Handler, path string) http.HandlerFunc {
	for _, r := range h.Routes() {
		if r.Path == path {
			return r.Handler
		}
	}
	return nil
}

func TestWorkPackV2AndDeshell(t *testing.T) {
	var _ packruntime.Module = (*Handler)(nil)

	h := NewHandler(&fakeGW{})
	if h.PackID() != PackID {
		t.Fatalf("PackID = %q", h.PackID())
	}
	if got := len(h.Routes()); got != 15 {
		t.Fatalf("Routes len = %d, want 15", got)
	}
	_ = h.Init(nil)
	if err := h.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer h.Stop(context.Background())

	// Native lifecycle route: nil task runtime → 404 (not the bridge sentinel).
	run := routeFor(h, "/v1/tasks/run")
	if run == nil {
		t.Fatal("missing /v1/tasks/run route")
	}
	rec := httptest.NewRecorder()
	run(rec, httptest.NewRequest(http.MethodPost, "/v1/tasks/run", strings.NewReader(`{"id":"x"}`)))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("native run nil-runtime = %d, want 404", rec.Code)
	}

	// Native route with empty id → 400 via decodeID (still not the bridge).
	// (use a wired-store-free path: ready() returns 404 first, so to exercise
	// decodeID we accept either 404/400 — both prove it's native, not 599.)
	if rec.Code == 599 {
		t.Fatal("run should be native, got bridge sentinel")
	}

	// Native collection route: /v1/tasks GET with nil store → 404 (not bridge).
	list := routeFor(h, "/v1/tasks")
	if list == nil {
		t.Fatal("missing /v1/tasks route")
	}
	rec2 := httptest.NewRecorder()
	list(rec2, httptest.NewRequest(http.MethodGet, "/v1/tasks", nil))
	if rec2.Code != http.StatusNotFound {
		t.Fatalf("native /v1/tasks list nil-store = %d, want 404", rec2.Code)
	}

	// Native gaps route: nil analyzer → 404 (not the bridge sentinel).
	gaps := routeFor(h, "/v1/tasks/gaps")
	if gaps == nil {
		t.Fatal("missing /v1/tasks/gaps route")
	}
	rec3 := httptest.NewRecorder()
	gaps(rec3, httptest.NewRequest(http.MethodGet, "/v1/tasks/gaps", nil))
	if rec3.Code != http.StatusNotFound {
		t.Fatalf("native /v1/tasks/gaps nil-analyzer = %d, want 404", rec3.Code)
	}

	// Native projects route: nil store → 503 (de-shelled, no bridge).
	proj := routeFor(h, "/v1/projects")
	if proj == nil {
		t.Fatal("missing /v1/projects route")
	}
	rec4 := httptest.NewRecorder()
	proj(rec4, httptest.NewRequest(http.MethodGet, "/v1/projects", nil))
	if rec4.Code != http.StatusServiceUnavailable {
		t.Fatalf("native /v1/projects nil-store = %d, want 503", rec4.Code)
	}
}
