package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"yunque-agent/internal/controlplane/tenant"
	cognikernelpack "yunque-agent/internal/packs/cognikernel"
	"yunque-agent/pkg/cogni"
	"yunque-agent/pkg/packruntime"
)

func TestCogniExperienceConfirmPattern(t *testing.T) {
	gw, tm := newTestGatewayWithCogniKernelPack(t, packruntime.PackStatusEnabled)
	tenant := tm.Register("cogni-experience-confirm")

	reg := cogni.NewRegistry()
	if err := reg.Add(&cogni.Declaration{ID: "reviewer", DisplayName: "Reviewer"}, "test"); err != nil {
		t.Fatalf("add cogni declaration: %v", err)
	}
	gw.SetCogniRegistry(reg, t.TempDir())

	store := cogni.NewExperienceStore("reviewer", cogni.ExperienceConfig{
		StoreDir:      t.TempDir(),
		RequireReview: true,
	})
	store.SuggestPattern(cogni.BehaviorPattern{
		ID:       "pat-timeout",
		Trigger:  "响应超时",
		Response: "保留轨迹并切换备用模型",
	})
	gw.SetCogniExperiences(map[string]*cogni.ExperienceStore{"reviewer": store})

	req := httptest.NewRequest(http.MethodPost, "/v1/cognis/reviewer/experience/patterns/pat-timeout/confirm", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()

	gw.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["status"] != "ok" || body["confirmed"] != true {
		t.Fatalf("unexpected response: %#v", body)
	}
	patterns := store.Patterns()
	if len(patterns) != 1 || !patterns[0].Confirmed {
		t.Fatalf("pattern was not confirmed: %+v", patterns)
	}
}

func TestCogniExperienceConfirmPatternNotFound(t *testing.T) {
	gw, tm := newTestGatewayWithCogniKernelPack(t, packruntime.PackStatusEnabled)
	tenant := tm.Register("cogni-experience-confirm-missing")

	reg := cogni.NewRegistry()
	if err := reg.Add(&cogni.Declaration{ID: "reviewer"}, "test"); err != nil {
		t.Fatalf("add cogni declaration: %v", err)
	}
	gw.SetCogniRegistry(reg, t.TempDir())
	gw.SetCogniExperiences(map[string]*cogni.ExperienceStore{
		"reviewer": cogni.NewExperienceStore("reviewer", cogni.ExperienceConfig{StoreDir: t.TempDir()}),
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/cognis/reviewer/experience/patterns/missing/confirm", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()

	gw.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
}

func TestCogniKernelPackGateReturnsNotFoundWhenDisabled(t *testing.T) {
	gw, tm := newTestGatewayWithCogniKernelPack(t, packruntime.PackStatusDisabled)
	tenant := tm.Register("cogni-pack-disabled")

	reg := cogni.NewRegistry()
	if err := reg.Add(&cogni.Declaration{ID: "reviewer"}, "test"); err != nil {
		t.Fatalf("add cogni declaration: %v", err)
	}
	gw.SetCogniRegistry(reg, t.TempDir())

	req := httptest.NewRequest(http.MethodGet, "/v1/cognis", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()

	gw.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("disabled Cogni Kernel pack should gate /v1/cognis, status = %d, body = %s", w.Code, w.Body.String())
	}
}

func TestCogniKernelRuntimePackStateRouteIsExactPackGated(t *testing.T) {
	reporter := fakeCogniKernelRuntimeStateReporter{
		report: cognikernelpack.RuntimeStateReport{
			PackID:                    cognikernelpack.PackID,
			Stage:                     "runtime-loop-pack-state-gate",
			PackInstalled:             true,
			PackEnabled:               true,
			RuntimeLoopPackStateReady: true,
			RuntimeLoopRunning:        true,
			Artifacts:                 []string{"cogni-runtime-pack-state.json"},
		},
	}
	gw, tm := newTestGatewayWithCogniKernelPackAndReporter(t, packruntime.PackStatusEnabled, reporter)
	tenant := tm.Register("cogni-pack-runtime-state")

	reg := cogni.NewRegistry()
	if err := reg.Add(&cogni.Declaration{ID: "reviewer"}, "test"); err != nil {
		t.Fatalf("add cogni declaration: %v", err)
	}
	gw.SetCogniRegistry(reg, t.TempDir())

	req := httptest.NewRequest(http.MethodGet, "/v1/cognis/runtime/pack-state", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("runtime pack-state route status = %d, body = %s", w.Code, w.Body.String())
	}
	var body cognikernelpack.RuntimeStateReport
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode runtime state report: %v", err)
	}
	if !body.RuntimeLoopPackStateReady || !body.RuntimeLoopRunning || len(body.Artifacts) == 0 {
		t.Fatalf("unexpected runtime state report: %#v", body)
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/cognis/runtime/pack-state", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("runtime pack-state route should be GET-only, status = %d, body = %s", w.Code, w.Body.String())
	}

	if _, err := gw.packRegistry.Disable(cognikernelpack.PackID); err != nil {
		t.Fatalf("Disable Cogni Kernel pack: %v", err)
	}
	req = httptest.NewRequest(http.MethodGet, "/v1/cognis/runtime/pack-state", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("disabled pack should gate exact runtime pack-state route, status = %d, body = %s", w.Code, w.Body.String())
	}
}

func newTestGatewayWithCogniKernelPack(t *testing.T, status packruntime.PackStatus) (*Gateway, *tenant.Manager) {
	return newTestGatewayWithCogniKernelPackAndReporter(t, status, nil)
}

func newTestGatewayWithCogniKernelPackAndReporter(t *testing.T, status packruntime.PackStatus, reporter cognikernelpack.RuntimeStateReporter) (*Gateway, *tenant.Manager) {
	t.Helper()
	registry, err := packruntime.NewRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	_, err = registry.Install(packruntime.Manifest{
		ID:           cognikernelpack.PackID,
		Name:         "Cogni Kernel Pack",
		Version:      "0.1.0",
		Optional:     true,
		DefaultState: "enabled",
		Backend: packruntime.BackendManifest{
			Routes:     cognikernelpack.Paths(),
			RouteSpecs: cognikernelpack.RouteSpecs(),
		},
		Frontend: packruntime.FrontendManifest{Menus: []packruntime.FrontendMenu{{Key: "cognis", Label: "智体内核", Path: "/packs/cognis"}}},
		SDK:      packruntime.SDKManifest{TypeScript: "yunque-client/cognis"},
	}, "test")
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if status == packruntime.PackStatusDisabled {
		if _, err := registry.Disable(cognikernelpack.PackID); err != nil {
			t.Fatalf("Disable: %v", err)
		}
	}
	gw, tm := newTestGatewayWithConfig(GatewayConfig{Packs: registry})
	if reporter == nil {
		gw.RegisterBackendPack(cognikernelpack.NewHandler(gw))
	} else {
		handler := cognikernelpack.NewHandlerWithRuntimeState(gw, reporter)
		gw.SetCogniKernelRuntimeStateHandler(handler.HandleRuntimePackState)
		gw.RegisterBackendPack(handler)
	}
	return gw, tm
}

type fakeCogniKernelRuntimeStateReporter struct {
	report cognikernelpack.RuntimeStateReport
}

func (r fakeCogniKernelRuntimeStateReporter) CogniKernelRuntimeState() cognikernelpack.RuntimeStateReport {
	return r.report
}
