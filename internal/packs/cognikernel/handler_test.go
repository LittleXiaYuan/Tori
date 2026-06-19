package cognikernelpack

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"yunque-agent/pkg/cogni"
)

type fakeCogniAPI struct {
	called int
	paths  []string
}

func (api *fakeCogniAPI) ServeCogniKernel(w http.ResponseWriter, r *http.Request) {
	api.called++
	api.paths = append(api.paths, r.URL.Path)
	w.WriteHeader(http.StatusNoContent)
}

type fakeRuntimeReporter struct{}

type fakeBusProvider struct {
	bus *cogni.CogniBus
}

func (p fakeBusProvider) CogniBus() *cogni.CogniBus { return p.bus }

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
	api := &fakeCogniAPI{}
	handler := NewHandlerWithRuntimeState(api, fakeRuntimeReporter{})

	if handler.PackID() != PackID {
		t.Fatalf("PackID = %q, want %q", handler.PackID(), PackID)
	}

	routes := handler.Routes()
	if len(routes) <= 4 {
		t.Fatalf("expected delegated Cogni Kernel sub-resource routes, got %d", len(routes))
	}
	if routes[0].Path != CollectionRoute {
		t.Fatalf("collection route path = %q", routes[0].Path)
	}
	if routes[1].Path != SubResourceRoute {
		t.Fatalf("sub-resource route path = %q", routes[1].Path)
	}
	if routes[2].Path != RouteDecisionRoute {
		t.Fatalf("route decision path = %q", routes[2].Path)
	}
	if routes[3].Path != RuntimePackStateRoute {
		t.Fatalf("runtime state route path = %q", routes[3].Path)
	}
	mounted := map[string]map[string]bool{}
	for _, route := range routes {
		if mounted[route.Path] == nil {
			mounted[route.Path] = map[string]bool{}
		}
		for _, method := range route.Methods {
			mounted[route.Path][method] = true
		}
		if route.Method != "" {
			mounted[route.Path][route.Method] = true
		}
	}
	for _, spec := range RouteSpecs() {
		if !mounted[spec.Path][spec.Method] {
			t.Fatalf("routeSpec %s %s not mounted by Routes()", spec.Method, spec.Path)
		}
	}

	req := httptest.NewRequest(http.MethodGet, CollectionRoute, nil)
	w := httptest.NewRecorder()
	routes[0].Handler(w, req)
	if w.Code != http.StatusNoContent || api.called != 1 || api.paths[0] != CollectionRoute {
		t.Fatalf("expected route to delegate to API, status=%d called=%d paths=%v", w.Code, api.called, api.paths)
	}

	runtimeRoutes := handler.RuntimeRoutes()
	if len(runtimeRoutes) != 1 || runtimeRoutes[0].Path != RuntimePackStateRoute {
		t.Fatalf("unexpected runtime routes: %#v", runtimeRoutes)
	}
}

func TestCogniKernelRouteSpecsExposeConcreteSubresources(t *testing.T) {
	specs := RouteSpecs()
	seen := map[string]map[string]bool{}
	for _, spec := range specs {
		if seen[spec.Path] == nil {
			seen[spec.Path] = map[string]bool{}
		}
		seen[spec.Path][spec.Method] = true
	}

	for _, want := range []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/v1/cognis/reload"},
		{http.MethodGet, "/v1/cognis/traces"},
		{http.MethodPost, "/v1/cognis/alerts/scan"},
		{http.MethodPost, "/v1/cognis/generate"},
		{http.MethodPost, "/v1/cognis/import"},
		{http.MethodPost, "/v1/cognis/federation/discover"},
		{http.MethodPost, "/v1/cognis/route"},
		{http.MethodGet, RuntimePackStateRoute},
	} {
		if !seen[want.path][want.method] {
			t.Fatalf("missing routeSpec %s %s", want.method, want.path)
		}
	}

	paths := Paths()
	if len(paths) >= len(specs) {
		t.Fatalf("Paths should collapse duplicate method specs: paths=%d specs=%d", len(paths), len(specs))
	}
}

func TestCogniKernelRouterOwnsRuntimeStateBeforeAPIDelegation(t *testing.T) {
	api := &fakeCogniAPI{}
	router := NewRouter(api, fakeRuntimeReporter{})

	req := httptest.NewRequest(http.MethodGet, RuntimePackStateRoute, nil)
	w := httptest.NewRecorder()
	router.HandleRuntimePackState(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("runtime route status=%d body=%s", w.Code, w.Body.String())
	}
	if api.called != 0 {
		t.Fatalf("runtime pack-state should not delegate to API adapter, called=%d", api.called)
	}

	req = httptest.NewRequest(http.MethodGet, SubResourceRoute+"reviewer", nil)
	w = httptest.NewRecorder()
	router.ServeCogniKernel(w, req)
	if w.Code != http.StatusNoContent || api.called != 1 {
		t.Fatalf("sub-resource route should delegate to API adapter, status=%d called=%d", w.Code, api.called)
	}
}

func TestCogniKernelRouterOwnsRouteDecision(t *testing.T) {
	api := &fakeCogniAPI{}
	bus := cogni.NewCogniBus(cogni.NewEvaluator(), cogni.DefaultBusConfig())
	bus.Register(&cogni.Declaration{
		ID: "reviewer",
		Activation: cogni.ActivationRules{
			Keywords: []string{"review"},
			MinScore: 0.2,
		},
	})
	bus.Register(&cogni.Declaration{
		ID: "translator",
		Activation: cogni.ActivationRules{
			Keywords: []string{"translate"},
			MinScore: 0.2,
		},
	})
	router := NewRouterWithBus(api, fakeRuntimeReporter{}, fakeBusProvider{bus: bus})

	body, _ := json.Marshal(map[string]any{"message": "please review my code"})
	req := httptest.NewRequest(http.MethodPost, RouteDecisionRoute, bytes.NewReader(body))
	w := httptest.NewRecorder()
	router.HandleRouteDecision(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if api.called != 0 {
		t.Fatalf("route decision should not delegate to API adapter, called=%d", api.called)
	}
	var result cogni.RouteResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(result.SelectedIDs) != 1 || result.SelectedIDs[0] != "reviewer" {
		t.Fatalf("expected reviewer to win, got %v (bids=%v)", result.SelectedIDs, result.AllBids)
	}
}

func TestCogniKernelRouterRouteDecisionRequiresMessage(t *testing.T) {
	bus := cogni.NewCogniBus(cogni.NewEvaluator(), cogni.DefaultBusConfig())
	router := NewRouterWithBus(&fakeCogniAPI{}, fakeRuntimeReporter{}, fakeBusProvider{bus: bus})

	req := httptest.NewRequest(http.MethodPost, RouteDecisionRoute, bytes.NewReader([]byte(`{}`)))
	w := httptest.NewRecorder()
	router.HandleRouteDecision(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestCogniKernelRouterRouteDecisionRequiresBus(t *testing.T) {
	router := NewRouterWithBus(&fakeCogniAPI{}, fakeRuntimeReporter{}, nil)

	body, _ := json.Marshal(map[string]any{"message": "hello"})
	req := httptest.NewRequest(http.MethodPost, RouteDecisionRoute, bytes.NewReader(body))
	w := httptest.NewRecorder()
	router.HandleRouteDecision(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestCogniKernelRouterReportsMissingAPIAdapter(t *testing.T) {
	router := NewRouter(nil, fakeRuntimeReporter{})

	req := httptest.NewRequest(http.MethodGet, CollectionRoute, nil)
	w := httptest.NewRecorder()
	router.ServeCogniKernel(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("missing API adapter status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestCogniKernelRuntimePackStateRoute(t *testing.T) {
	handler := NewHandlerWithRuntimeState(&fakeCogniAPI{}, fakeRuntimeReporter{})

	req := httptest.NewRequest(http.MethodGet, RuntimePackStateRoute, nil)
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
