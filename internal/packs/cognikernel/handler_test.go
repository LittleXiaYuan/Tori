package cognikernelpack

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type fakeCogniGateway struct {
	called int
}

func (g *fakeCogniGateway) HandleCogniKernelPack(w http.ResponseWriter, _ *http.Request) {
	g.called++
	w.WriteHeader(http.StatusNoContent)
}

type fakeRuntimeReporter struct{}

func (fakeRuntimeReporter) CogniKernelRuntimeState() RuntimeStateReport {
	return RuntimeStateReport{
		PackID:                    PackID,
		Stage:                     "runtime-loop-pack-state-gate",
		PackInstalled:             true,
		PackEnabled:               true,
		PackStatus:                "enabled",
		RuntimeLoopPackStateReady: true,
		RuntimeLoopRunning:        true,
		StopsRuntimeLoops:         true,
		StartsRuntimeLoops:        true,
		ClearsRuntimeState:        true,
		SentinelReady:             true,
		SchedulerReady:            true,
		BusReady:                  true,
		ExperienceStoreReady:      true,
		ActiveBusCognis:           2,
		ExperienceStoreCount:      1,
		GeneratedAt:               time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC),
		Capabilities:              []string{"cognis.runtime.pack_state"},
		Artifacts:                 []string{"cogni-runtime-pack-state.json"},
	}
}

func TestCogniKernelHandlerRoutesExposeSurface(t *testing.T) {
	gateway := &fakeCogniGateway{}
	handler := NewHandlerWithRuntimeState(gateway, fakeRuntimeReporter{})

	if handler.PackID() != PackID {
		t.Fatalf("PackID = %q, want %q", handler.PackID(), PackID)
	}

	routes := handler.Routes()
	if len(routes) != 3 {
		t.Fatalf("expected 3 Cogni Kernel routes, got %d", len(routes))
	}
	if routes[0].Path != "/v1/cognis" {
		t.Fatalf("collection route path = %q", routes[0].Path)
	}
	if routes[1].Path != "/v1/cognis/" {
		t.Fatalf("sub-resource route path = %q", routes[1].Path)
	}
	if routes[2].Path != "/v1/cognis/runtime/pack-state" {
		t.Fatalf("runtime state route path = %q", routes[2].Path)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/cognis", nil)
	w := httptest.NewRecorder()
	routes[0].Handler(w, req)
	if w.Code != http.StatusNoContent || gateway.called != 1 {
		t.Fatalf("expected route to delegate to gateway, status=%d called=%d", w.Code, gateway.called)
	}

	runtimeRoutes := handler.RuntimeRoutes()
	if len(runtimeRoutes) != 1 || runtimeRoutes[0].Path != "/v1/cognis/runtime/pack-state" {
		t.Fatalf("unexpected runtime routes: %#v", runtimeRoutes)
	}
}

func TestCogniKernelRuntimePackStateRoute(t *testing.T) {
	handler := NewHandlerWithRuntimeState(&fakeCogniGateway{}, fakeRuntimeReporter{})

	req := httptest.NewRequest(http.MethodGet, "/v1/cognis/runtime/pack-state", nil)
	w := httptest.NewRecorder()
	handler.HandleRuntimePackState(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var report RuntimeStateReport
	if err := json.NewDecoder(w.Body).Decode(&report); err != nil {
		t.Fatalf("decode report: %v", err)
	}
	if !report.RuntimeLoopPackStateReady || !report.StopsRuntimeLoops || !report.RuntimeLoopRunning {
		t.Fatalf("unexpected runtime pack-state report: %#v", report)
	}
}
