package cognikernelpack

import (
	"bytes"
	"context"
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

type fakeDependencyProvider struct {
	bus         *cogni.CogniBus
	federation  *cogni.CogniFederation
	tracker     *cogni.CostTracker
	experiences map[string]*cogni.ExperienceStore
	registry    *cogni.Registry
	workflow    *cogni.WorkflowEngine
}

func (p fakeDependencyProvider) CogniBus() *cogni.CogniBus { return p.bus }

func (p fakeDependencyProvider) CogniFederation() *cogni.CogniFederation { return p.federation }

func (p fakeDependencyProvider) CogniCostTracker() *cogni.CostTracker { return p.tracker }

func (p fakeDependencyProvider) CogniExperiences() map[string]*cogni.ExperienceStore {
	return p.experiences
}

func (p fakeDependencyProvider) CogniRegistry() *cogni.Registry { return p.registry }

func (p fakeDependencyProvider) CogniWorkflowEngine() *cogni.WorkflowEngine { return p.workflow }

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
	if !mounted[RouteDecisionRoute][http.MethodPost] {
		t.Fatalf("route decision route is not mounted")
	}
	if !mounted[RuntimePackStateRoute][http.MethodGet] {
		t.Fatalf("runtime state route is not mounted")
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

func TestCogniKernelRouterOwnsFederationAndEconomics(t *testing.T) {
	api := &fakeCogniAPI{}
	registry := cogni.NewRegistry()
	if err := registry.Add(&cogni.Declaration{ID: "reviewer"}, "test"); err != nil {
		t.Fatalf("add cogni: %v", err)
	}
	federation := cogni.NewCogniFederation("local", "http://local", registry)
	tracker := cogni.NewCostTracker()
	tracker.Record(cogni.CostEntry{CogniID: "reviewer", Cost: 0.5, Tokens: 1000, Operation: "route"})
	router := NewRouterWithDeps(api, fakeRuntimeReporter{}, Dependencies{
		FederationProvider:  fakeDependencyProvider{federation: federation},
		CostTrackerProvider: fakeDependencyProvider{tracker: tracker},
	})

	req := httptest.NewRequest(http.MethodGet, FederationRoute, nil)
	w := httptest.NewRecorder()
	router.HandleFederationStatus(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("federation status=%d body=%s", w.Code, w.Body.String())
	}
	var status map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if status["enabled"] != true || status["self_id"] != "local" {
		t.Fatalf("unexpected federation status: %#v", status)
	}

	body, _ := json.Marshal(cogni.FederationPeer{ID: "peer-1", Name: "Peer", URL: "http://peer"})
	req = httptest.NewRequest(http.MethodPost, FederationPeersRoute, bytes.NewReader(body))
	w = httptest.NewRecorder()
	router.HandleFederationPeers(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("add peer status=%d body=%s", w.Code, w.Body.String())
	}
	req = httptest.NewRequest(http.MethodGet, FederationPeersRoute, nil)
	w = httptest.NewRecorder()
	router.HandleFederationPeers(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list peers status=%d body=%s", w.Code, w.Body.String())
	}
	var peers map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &peers); err != nil {
		t.Fatalf("decode peers: %v", err)
	}
	if peers["count"].(float64) != 1 {
		t.Fatalf("expected one peer, got %#v", peers)
	}

	req = httptest.NewRequest(http.MethodGet, EconomicsRoute, nil)
	w = httptest.NewRecorder()
	router.HandleEconomics(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("economics status=%d body=%s", w.Code, w.Body.String())
	}
	var economics map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &economics); err != nil {
		t.Fatalf("decode economics: %v", err)
	}
	if economics["enabled"] != true {
		t.Fatalf("expected economics enabled, got %#v", economics)
	}
	summary := economics["summary"].(map[string]any)
	if _, ok := summary["reviewer"]; !ok {
		t.Fatalf("expected reviewer summary, got %#v", economics)
	}
	if api.called != 0 {
		t.Fatalf("federation/economics should not delegate to API adapter, called=%d", api.called)
	}
}

func TestCogniKernelRouterFederationAndEconomicsMissingDeps(t *testing.T) {
	router := NewRouter(&fakeCogniAPI{}, fakeRuntimeReporter{})

	req := httptest.NewRequest(http.MethodGet, FederationRoute, nil)
	w := httptest.NewRecorder()
	router.HandleFederationStatus(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("federation status=%d body=%s", w.Code, w.Body.String())
	}
	var status map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode federation status: %v", err)
	}
	if status["enabled"] != false {
		t.Fatalf("expected disabled federation status, got %#v", status)
	}

	req = httptest.NewRequest(http.MethodPost, FederationDiscoverRoute, nil)
	w = httptest.NewRecorder()
	router.HandleFederationDiscover(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("discover status=%d body=%s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, EconomicsRoute, nil)
	w = httptest.NewRecorder()
	router.HandleEconomics(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("economics status=%d body=%s", w.Code, w.Body.String())
	}
	var economics map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &economics); err != nil {
		t.Fatalf("decode economics: %v", err)
	}
	if economics["enabled"] != false {
		t.Fatalf("expected disabled economics, got %#v", economics)
	}
}

func TestCogniKernelRouterOwnsExperience(t *testing.T) {
	api := &fakeCogniAPI{}
	store := cogni.NewExperienceStore("reviewer", cogni.ExperienceConfig{
		StoreDir:      t.TempDir(),
		RequireReview: true,
	})
	router := NewRouterWithDeps(api, fakeRuntimeReporter{}, Dependencies{
		ExperienceProvider: fakeDependencyProvider{experiences: map[string]*cogni.ExperienceStore{"reviewer": store}},
	})

	toolData, _ := json.Marshal(cogni.ToolExperience{
		Tool:       "review",
		Context:    "go code",
		Learned:    "check package boundaries",
		Confidence: 0.9,
	})
	body, _ := json.Marshal(map[string]any{"type": "tool_memory", "data": json.RawMessage(toolData)})
	req := httptest.NewRequest(http.MethodPost, "/v1/cognis/reviewer/experience/record", bytes.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeCogniKernel(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("tool memory record status=%d body=%s", w.Code, w.Body.String())
	}

	patternData, _ := json.Marshal(cogni.BehaviorPattern{
		ID:       "pat-timeout",
		Trigger:  "timeout",
		Response: "switch provider",
	})
	body, _ = json.Marshal(map[string]any{"type": "pattern", "data": json.RawMessage(patternData)})
	req = httptest.NewRequest(http.MethodPost, "/v1/cognis/reviewer/experience/record", bytes.NewReader(body))
	w = httptest.NewRecorder()
	router.ServeCogniKernel(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("pattern record status=%d body=%s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/cognis/reviewer/experience/patterns/pat-timeout/confirm", nil)
	w = httptest.NewRecorder()
	router.ServeCogniKernel(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("confirm pattern status=%d body=%s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/cognis/reviewer/experience", nil)
	w = httptest.NewRecorder()
	router.ServeCogniKernel(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("experience get status=%d body=%s", w.Code, w.Body.String())
	}
	var report map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &report); err != nil {
		t.Fatalf("decode experience: %v", err)
	}
	if report["enabled"] != true || report["id"] != "reviewer" {
		t.Fatalf("unexpected experience report: %#v", report)
	}
	patterns := store.Patterns()
	if len(patterns) != 1 || !patterns[0].Confirmed {
		t.Fatalf("pattern was not confirmed: %+v", patterns)
	}
	if len(store.ToolMemory("review")) != 1 {
		t.Fatalf("expected tool memory to be recorded")
	}
	if api.called != 0 {
		t.Fatalf("experience routes should not delegate to API adapter, called=%d", api.called)
	}
}

func TestCogniKernelRouterOwnsWorkflow(t *testing.T) {
	api := &fakeCogniAPI{}
	registry := cogni.NewRegistry()
	if err := registry.Add(&cogni.Declaration{
		ID: "reviewer",
		Workflows: []cogni.WorkflowDef{{
			Name: "full_review",
			Steps: []cogni.WorkflowStep{{
				Name:   "summarize",
				Skill:  "summarize",
				Args:   map[string]any{"topic": "${input.topic}"},
				Output: "summary",
			}},
		}},
	}, "test"); err != nil {
		t.Fatalf("add cogni: %v", err)
	}
	engine := cogni.NewWorkflowEngine(func(ctx context.Context, skillName string, args map[string]any) (any, error) {
		if skillName != "summarize" {
			t.Fatalf("unexpected skill: %s", skillName)
		}
		return "summary:" + args["topic"].(string), nil
	})
	router := NewRouterWithDeps(api, fakeRuntimeReporter{}, Dependencies{
		RegistryProvider: fakeDependencyProvider{registry: registry},
		WorkflowProvider: fakeDependencyProvider{workflow: engine},
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/cognis/reviewer/workflows", nil)
	w := httptest.NewRecorder()
	router.ServeCogniKernel(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("workflow list status=%d body=%s", w.Code, w.Body.String())
	}
	var list map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if list["count"].(float64) != 1 {
		t.Fatalf("expected one workflow, got %#v", list)
	}

	body, _ := json.Marshal(map[string]any{"topic": "pack"})
	req = httptest.NewRequest(http.MethodPost, "/v1/cognis/reviewer/workflow/full_review", bytes.NewReader(body))
	w = httptest.NewRecorder()
	router.ServeCogniKernel(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("workflow run status=%d body=%s", w.Code, w.Body.String())
	}
	var result cogni.WorkflowResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if !result.Success || result.Outputs["summary"] != "summary:pack" {
		t.Fatalf("unexpected workflow result: %#v", result)
	}
	if api.called != 0 {
		t.Fatalf("workflow routes should not delegate to API adapter, called=%d", api.called)
	}
}

func TestCogniKernelRouterWorkflowErrors(t *testing.T) {
	registry := cogni.NewRegistry()
	if err := registry.Add(&cogni.Declaration{ID: "reviewer"}, "test"); err != nil {
		t.Fatalf("add cogni: %v", err)
	}
	router := NewRouterWithDeps(&fakeCogniAPI{}, fakeRuntimeReporter{}, Dependencies{
		RegistryProvider: fakeDependencyProvider{registry: registry},
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/cognis/reviewer/workflow/missing", bytes.NewReader([]byte(`{}`)))
	w := httptest.NewRecorder()
	router.ServeCogniKernel(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("missing workflow engine status=%d body=%s", w.Code, w.Body.String())
	}

	engine := cogni.NewWorkflowEngine(nil)
	router = NewRouterWithDeps(&fakeCogniAPI{}, fakeRuntimeReporter{}, Dependencies{
		RegistryProvider: fakeDependencyProvider{registry: registry},
		WorkflowProvider: fakeDependencyProvider{workflow: engine},
	})
	req = httptest.NewRequest(http.MethodGet, "/v1/cognis/missing/workflows", nil)
	w = httptest.NewRecorder()
	router.ServeCogniKernel(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("missing cogni status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestCogniKernelRouterExperienceMissingStore(t *testing.T) {
	router := NewRouter(&fakeCogniAPI{}, fakeRuntimeReporter{})

	req := httptest.NewRequest(http.MethodGet, "/v1/cognis/reviewer/experience", nil)
	w := httptest.NewRecorder()
	router.ServeCogniKernel(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("get missing store status=%d body=%s", w.Code, w.Body.String())
	}
	var report map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &report); err != nil {
		t.Fatalf("decode missing store report: %v", err)
	}
	if report["enabled"] != false {
		t.Fatalf("expected disabled experience report, got %#v", report)
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/cognis/reviewer/experience/record", bytes.NewReader([]byte(`{}`)))
	w = httptest.NewRecorder()
	router.ServeCogniKernel(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("record missing store status=%d body=%s", w.Code, w.Body.String())
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
