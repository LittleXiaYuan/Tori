package cognikernelpack

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
	traces      cogni.TraceStore
	sentinel    *cogni.Sentinel
	dir         string
}

func (p fakeDependencyProvider) CogniBus() *cogni.CogniBus { return p.bus }

func (p fakeDependencyProvider) CogniFederation() *cogni.CogniFederation { return p.federation }

func (p fakeDependencyProvider) CogniCostTracker() *cogni.CostTracker { return p.tracker }

func (p fakeDependencyProvider) CogniExperiences() map[string]*cogni.ExperienceStore {
	return p.experiences
}

func (p fakeDependencyProvider) CogniRegistry() *cogni.Registry { return p.registry }

func (p fakeDependencyProvider) CogniWorkflowEngine() *cogni.WorkflowEngine { return p.workflow }

func (p fakeDependencyProvider) CogniTraceStore() cogni.TraceStore { return p.traces }

func (p fakeDependencyProvider) CogniSentinel() *cogni.Sentinel { return p.sentinel }

func (p fakeDependencyProvider) CogniDirectory() string { return p.dir }

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

	registry := cogni.NewRegistry()
	handler = NewHandlerWithDeps(api, fakeRuntimeReporter{}, Dependencies{
		RegistryProvider: fakeDependencyProvider{registry: registry},
	})
	routes = handler.Routes()

	req := httptest.NewRequest(http.MethodGet, CollectionRoute, nil)
	w := httptest.NewRecorder()
	routes[0].Handler(w, req)
	if w.Code != http.StatusOK || api.called != 0 {
		t.Fatalf("expected collection route to be pack-owned, status=%d called=%d paths=%v", w.Code, api.called, api.paths)
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

	req = httptest.NewRequest(http.MethodPost, SubResourceRoute+"reviewer/evolve", nil)
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

func TestCogniKernelRouterOwnsRegistryLifecycle(t *testing.T) {
	api := &fakeCogniAPI{}
	registry := cogni.NewRegistry()
	store := cogni.NewInMemoryTraceStore(10)
	store.Record(cogni.Trace{
		Activations: []cogni.TraceActivation{{
			ID:        "reviewer",
			Activated: true,
			Score:     0.9,
		}},
	})
	router := NewRouterWithDeps(api, fakeRuntimeReporter{}, Dependencies{
		RegistryProvider:   fakeDependencyProvider{registry: registry},
		TraceStoreProvider: fakeDependencyProvider{traces: store},
	})

	body, _ := json.Marshal(cogni.Declaration{
		ID:          "reviewer",
		DisplayName: "Reviewer",
		Activation:  cogni.ActivationRules{AlwaysOn: true},
	})
	req := httptest.NewRequest(http.MethodPost, CollectionRoute, bytes.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeCogniKernel(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, CollectionRoute, nil)
	w = httptest.NewRecorder()
	router.ServeCogniKernel(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", w.Code, w.Body.String())
	}
	var list map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if list["count"].(float64) != 1 {
		t.Fatalf("expected one cogni, got %#v", list)
	}
	health := list["health"].(map[string]any)
	if _, ok := health["reviewer"]; !ok {
		t.Fatalf("expected reviewer health summary, got %#v", health)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/cognis/reviewer", nil)
	w = httptest.NewRecorder()
	router.ServeCogniKernel(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("get status=%d body=%s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/cognis/reviewer/disable", nil)
	w = httptest.NewRecorder()
	router.ServeCogniKernel(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("disable status=%d body=%s", w.Code, w.Body.String())
	}
	if registry.IsEnabled("reviewer") {
		t.Fatalf("reviewer should be disabled")
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/cognis/reviewer/enable", nil)
	w = httptest.NewRecorder()
	router.ServeCogniKernel(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("enable status=%d body=%s", w.Code, w.Body.String())
	}
	if !registry.IsEnabled("reviewer") {
		t.Fatalf("reviewer should be enabled")
	}

	req = httptest.NewRequest(http.MethodDelete, "/v1/cognis/reviewer", nil)
	w = httptest.NewRecorder()
	router.ServeCogniKernel(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("delete status=%d body=%s", w.Code, w.Body.String())
	}
	if _, ok := registry.Get("reviewer"); ok {
		t.Fatalf("reviewer should be removed")
	}
	if api.called != 0 {
		t.Fatalf("registry lifecycle should not delegate to API adapter, called=%d", api.called)
	}
}

func TestCogniKernelRouterOwnsReload(t *testing.T) {
	api := &fakeCogniAPI{}
	registry := cogni.NewRegistry()
	dir := t.TempDir()
	decl := &cogni.Declaration{
		ID:         "from-disk",
		Activation: cogni.ActivationRules{AlwaysOn: true},
	}
	if err := cogni.SaveDeclaration(decl, filepath.Join(dir, "from-disk.json")); err != nil {
		t.Fatalf("save declaration: %v", err)
	}
	router := NewRouterWithDeps(api, fakeRuntimeReporter{}, Dependencies{
		RegistryProvider:  fakeDependencyProvider{registry: registry},
		DirectoryProvider: fakeDependencyProvider{dir: dir},
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/cognis/reload", nil)
	w := httptest.NewRecorder()
	router.ServeCogniKernel(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("reload status=%d body=%s", w.Code, w.Body.String())
	}
	var result map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode reload: %v", err)
	}
	if result["added"].(float64) != 1 || result["dir"] != dir {
		t.Fatalf("unexpected reload result: %#v", result)
	}
	if _, ok := registry.Get("from-disk"); !ok {
		t.Fatalf("from-disk declaration should be loaded")
	}

	if err := os.Remove(filepath.Join(dir, "from-disk.json")); err != nil {
		t.Fatalf("remove declaration: %v", err)
	}
	req = httptest.NewRequest(http.MethodPost, "/v1/cognis/reload", nil)
	w = httptest.NewRecorder()
	router.ServeCogniKernel(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("reload remove status=%d body=%s", w.Code, w.Body.String())
	}
	if _, ok := registry.Get("from-disk"); ok {
		t.Fatalf("from-disk declaration should be removed after reload")
	}
	if api.called != 0 {
		t.Fatalf("reload should not delegate to API adapter, called=%d", api.called)
	}
}

func TestCogniKernelRouterOwnsBundleImportAndPersistence(t *testing.T) {
	api := &fakeCogniAPI{}
	dir := t.TempDir()
	registry := cogni.NewRegistry()
	if err := registry.Add(&cogni.Declaration{
		ID:          "existing-cogni",
		Description: "original version",
		Activation:  cogni.ActivationRules{AlwaysOn: true},
	}, "test"); err != nil {
		t.Fatalf("add existing cogni: %v", err)
	}
	router := NewRouterWithDeps(api, fakeRuntimeReporter{}, Dependencies{
		RegistryProvider:  fakeDependencyProvider{registry: registry},
		DirectoryProvider: fakeDependencyProvider{dir: dir},
	})

	bundle := cogni.Bundle{
		Schema: cogni.BundleSchema,
		Cognis: []*cogni.Declaration{
			{
				ID:          "new-cogni-1",
				DisplayName: "New Cogni 1",
				Description: "First new cogni",
				Activation:  cogni.ActivationRules{Keywords: []string{"test"}},
			},
			{
				ID:          "new-cogni-2",
				DisplayName: "New Cogni 2",
				Description: "Second new cogni",
				Activation:  cogni.ActivationRules{AlwaysOn: true},
			},
			{
				ID:          "existing-cogni",
				Description: "updated version",
				Activation:  cogni.ActivationRules{AlwaysOn: true},
			},
		},
	}
	body, err := json.Marshal(bundle)
	if err != nil {
		t.Fatalf("marshal bundle: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/cognis/import?overwrite=true", bytes.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeCogniKernel(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("import status=%d body=%s", w.Code, w.Body.String())
	}

	var summary cogni.ImportSummary
	if err := json.NewDecoder(w.Body).Decode(&summary); err != nil {
		t.Fatalf("decode summary: %v", err)
	}
	if len(summary.Added) != 2 || len(summary.Updated) != 1 || len(summary.Failed) != 0 {
		t.Fatalf("unexpected import summary: %+v", summary)
	}
	for _, filename := range []string{"new-cogni-1.json", "new-cogni-2.json", "existing-cogni.json"} {
		filePath := filepath.Join(dir, filename)
		data, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("expected persisted declaration %s: %v", filename, err)
		}
		var decl cogni.Declaration
		if err := json.Unmarshal(data, &decl); err != nil {
			t.Fatalf("persisted declaration %s is invalid JSON: %v", filename, err)
		}
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/cognis/export?ids=new-cogni-1,existing-cogni&notes=portable", nil)
	w = httptest.NewRecorder()
	router.ServeCogniKernel(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("export status=%d body=%s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Content-Disposition"); got != "attachment; filename=\"cogni-bundle.json\"" {
		t.Fatalf("unexpected content disposition: %q", got)
	}
	var exported cogni.Bundle
	if err := json.NewDecoder(w.Body).Decode(&exported); err != nil {
		t.Fatalf("decode exported bundle: %v", err)
	}
	if exported.Notes != "portable" || len(exported.Cognis) != 2 {
		t.Fatalf("unexpected exported bundle: %+v", exported)
	}
	if api.called != 0 {
		t.Fatalf("bundle routes should not delegate to API adapter, called=%d", api.called)
	}
}

func TestCogniKernelRouterImportSkipsFailedAndHandlesEmptyDirectory(t *testing.T) {
	registry := cogni.NewRegistry()
	dir := t.TempDir()
	router := NewRouterWithDeps(&fakeCogniAPI{}, fakeRuntimeReporter{}, Dependencies{
		RegistryProvider:  fakeDependencyProvider{registry: registry},
		DirectoryProvider: fakeDependencyProvider{dir: dir},
	})

	bundle := cogni.Bundle{
		Schema: cogni.BundleSchema,
		Cognis: []*cogni.Declaration{
			{
				ID:          "valid-cogni",
				Description: "This one is valid",
				Activation:  cogni.ActivationRules{AlwaysOn: true},
			},
			{
				Description: "This one is invalid",
				Activation:  cogni.ActivationRules{AlwaysOn: true},
			},
			{
				ID:         "invalid-score",
				Activation: cogni.ActivationRules{MinScore: 2.0},
			},
		},
	}
	body, _ := json.Marshal(bundle)
	req := httptest.NewRequest(http.MethodPost, "/v1/cognis/import", bytes.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeCogniKernel(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("import status=%d body=%s", w.Code, w.Body.String())
	}

	var summary cogni.ImportSummary
	if err := json.NewDecoder(w.Body).Decode(&summary); err != nil {
		t.Fatalf("decode summary: %v", err)
	}
	if len(summary.Added) != 1 || len(summary.Failed) != 2 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if _, err := os.Stat(filepath.Join(dir, "valid-cogni.json")); err != nil {
		t.Fatalf("expected valid cogni to be persisted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "invalid-score.json")); !os.IsNotExist(err) {
		t.Fatalf("invalid-score.json should not exist, stat err=%v", err)
	}

	registry = cogni.NewRegistry()
	router = NewRouterWithDeps(&fakeCogniAPI{}, fakeRuntimeReporter{}, Dependencies{
		RegistryProvider: fakeDependencyProvider{registry: registry},
	})
	body, _ = json.Marshal(cogni.Bundle{
		Schema: cogni.BundleSchema,
		Cognis: []*cogni.Declaration{{
			ID:         "memory-only",
			Activation: cogni.ActivationRules{AlwaysOn: true},
		}},
	})
	req = httptest.NewRequest(http.MethodPost, "/v1/cognis/import", bytes.NewReader(body))
	w = httptest.NewRecorder()
	router.ServeCogniKernel(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("memory-only import status=%d body=%s", w.Code, w.Body.String())
	}
	if _, ok := registry.Get("memory-only"); !ok {
		t.Fatalf("memory-only cogni should still be imported without a directory")
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

func TestCogniKernelRouterOwnsObservability(t *testing.T) {
	api := &fakeCogniAPI{}
	store := cogni.NewInMemoryTraceStore(10)
	store.Record(cogni.Trace{
		Activations: []cogni.TraceActivation{{
			ID:        "reviewer",
			Activated: true,
			Score:     0.9,
		}},
		DurationMs: 12,
	})
	router := NewRouterWithDeps(api, fakeRuntimeReporter{}, Dependencies{
		TraceStoreProvider: fakeDependencyProvider{traces: store},
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/cognis/traces", nil)
	w := httptest.NewRecorder()
	router.HandleTracesAll(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("traces status=%d body=%s", w.Code, w.Body.String())
	}
	var traces map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &traces); err != nil {
		t.Fatalf("decode traces: %v", err)
	}
	if traces["count"].(float64) != 1 {
		t.Fatalf("expected one trace, got %#v", traces)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/cognis/reviewer/health", nil)
	w = httptest.NewRecorder()
	router.ServeCogniKernel(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("health status=%d body=%s", w.Code, w.Body.String())
	}
	var health cogni.HealthMetrics
	if err := json.Unmarshal(w.Body.Bytes(), &health); err != nil {
		t.Fatalf("decode health: %v", err)
	}
	if health.ID != "reviewer" || health.Status == "" {
		t.Fatalf("unexpected health: %#v", health)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/cognis/stats", nil)
	w = httptest.NewRecorder()
	router.HandleTraceStats(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("stats status=%d body=%s", w.Code, w.Body.String())
	}
	var stats cogni.TraceStats
	if err := json.Unmarshal(w.Body.Bytes(), &stats); err != nil {
		t.Fatalf("decode stats: %v", err)
	}
	if stats.TotalTurns != 1 || stats.PerCogni["reviewer"] != 1 {
		t.Fatalf("unexpected stats: %#v", stats)
	}
	if api.called != 0 {
		t.Fatalf("observability routes should not delegate to API adapter, called=%d", api.called)
	}
}

func TestCogniKernelRouterOwnsAlertsAndVerify(t *testing.T) {
	api := &fakeCogniAPI{}
	registry := cogni.NewRegistry()
	if err := registry.Add(&cogni.Declaration{
		ID: "reviewer",
		Activation: cogni.ActivationRules{
			Keywords: []string{"review"},
			MinScore: 0.2,
		},
		Checks: []cogni.ActivationCheck{{
			Name:         "review matches",
			Message:      "please review",
			ExpectActive: boolRef(true),
		}},
	}, "test"); err != nil {
		t.Fatalf("add cogni: %v", err)
	}
	store := cogni.NewInMemoryTraceStore(10)
	sentinel := cogni.NewSentinel(store, registry, cogni.SentinelPolicy{})
	router := NewRouterWithDeps(api, fakeRuntimeReporter{}, Dependencies{
		RegistryProvider:   fakeDependencyProvider{registry: registry},
		SentinelProvider:   fakeDependencyProvider{sentinel: sentinel},
		TraceStoreProvider: fakeDependencyProvider{traces: store},
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/cognis/verify", nil)
	w := httptest.NewRecorder()
	router.HandleVerifyAll(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("verify all status=%d body=%s", w.Code, w.Body.String())
	}
	var verify map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &verify); err != nil {
		t.Fatalf("decode verify: %v", err)
	}
	results := verify["results"].(map[string]any)
	if _, ok := results["reviewer"]; !ok {
		t.Fatalf("expected reviewer verify results, got %#v", verify)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/cognis/reviewer/verify", nil)
	w = httptest.NewRecorder()
	router.ServeCogniKernel(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("verify by id status=%d body=%s", w.Code, w.Body.String())
	}
	var byID map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &byID); err != nil {
		t.Fatalf("decode by id: %v", err)
	}
	if byID["id"] != "reviewer" || byID["passed"].(float64) != 1 {
		t.Fatalf("unexpected verify by id: %#v", byID)
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/cognis/alerts/scan", nil)
	w = httptest.NewRecorder()
	router.HandleAlertsScan(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("alerts scan status=%d body=%s", w.Code, w.Body.String())
	}
	req = httptest.NewRequest(http.MethodGet, "/v1/cognis/alerts", nil)
	w = httptest.NewRecorder()
	router.HandleAlerts(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("alerts status=%d body=%s", w.Code, w.Body.String())
	}
	if api.called != 0 {
		t.Fatalf("alerts/verify should not delegate to API adapter, called=%d", api.called)
	}
}

func boolRef(v bool) *bool { return &v }

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

	req := httptest.NewRequest(http.MethodPost, SubResourceRoute+"reviewer/evolve", nil)
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
