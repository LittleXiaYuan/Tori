package plannerrecoverypack

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"yunque-agent/pkg/packruntime"
)

type fakeGateway struct {
	called map[string]int
}

func (f *fakeGateway) mark(name string, w http.ResponseWriter) {
	if f.called == nil {
		f.called = map[string]int{}
	}
	f.called[name]++
	w.WriteHeader(http.StatusTeapot)
}

func (f *fakeGateway) HandlePlannerCheckpoints(w http.ResponseWriter, r *http.Request) {
	f.mark("checkpoints", w)
}
func (f *fakeGateway) HandlePlannerExecutionState(w http.ResponseWriter, r *http.Request) {
	f.mark("execution-state", w)
}
func (f *fakeGateway) HandlePlannerCheckpointRecover(w http.ResponseWriter, r *http.Request) {
	f.mark("recover", w)
}
func (f *fakeGateway) HandlePlannerCheckpointResumeTask(w http.ResponseWriter, r *http.Request) {
	f.mark("resume", w)
}
func (f *fakeGateway) HandlePlannerCheckpointResumePlan(w http.ResponseWriter, r *http.Request) {
	f.mark("resume-plan", w)
}
func (f *fakeGateway) HandlePlannerCheckpointResumePlanJob(w http.ResponseWriter, r *http.Request) {
	f.mark("resume-plan-jobs", w)
}

func TestPlannerRecoveryPackV2AndRouteSpecs(t *testing.T) {
	var _ packruntime.Module = (*Handler)(nil)

	h := New(nil)
	if h.PackID() != PackID {
		t.Fatalf("PackID=%q, want %q", h.PackID(), PackID)
	}
	if got := len(h.Routes()); got != 6 {
		t.Fatalf("Routes len=%d, want 6", got)
	}
	if got := len(RouteSpecs()); got != 6 {
		t.Fatalf("RouteSpecs len=%d, want 6", got)
	}
	paths := map[string]bool{}
	for _, route := range h.Routes() {
		paths[route.Path] = true
	}
	for _, spec := range RouteSpecs() {
		if !paths[spec.Path] {
			t.Fatalf("route spec path %s has no mounted route", spec.Path)
		}
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
}

func TestPlannerRecoveryDelegatesToGateway(t *testing.T) {
	gw := &fakeGateway{}
	h := New(gw)

	for _, route := range h.Routes() {
		rec := httptest.NewRecorder()
		route.Handler(rec, httptest.NewRequest(route.Method, route.Path, nil))
		if rec.Code != http.StatusTeapot {
			t.Fatalf("%s delegated status=%d, want 418", route.Path, rec.Code)
		}
	}
	for _, key := range []string{"checkpoints", "execution-state", "recover", "resume", "resume-plan", "resume-plan-jobs"} {
		if gw.called[key] != 1 {
			t.Fatalf("gateway handler %s called %d times, want 1", key, gw.called[key])
		}
	}
}

func TestPlannerRecoveryNilGatewayReturnsUnavailable(t *testing.T) {
	h := New(nil)
	rec := httptest.NewRecorder()
	h.Routes()[0].Handler(rec, httptest.NewRequest(http.MethodGet, "/v1/planner/checkpoints", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d, want 503", rec.Code)
	}
}
