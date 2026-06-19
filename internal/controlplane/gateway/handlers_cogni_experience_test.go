package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"yunque-agent/internal/controlplane/tenant"
	cognikernelpack "yunque-agent/internal/packs/cognikernel"
	"yunque-agent/pkg/cogni"
	"yunque-agent/pkg/packruntime"
)

func boolRef(v bool) *bool { return &v }

func TestCogniKernelPackServesWorkflowListAndRun(t *testing.T) {
	gw, tm := newTestGatewayWithCogniKernelPack(t, packruntime.PackStatusEnabled)
	tenant := tm.Register("cogni-workflow")

	reg := cogni.NewRegistry()
	if err := reg.Add(&cogni.Declaration{
		ID:          "reviewer",
		DisplayName: "Reviewer",
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
		t.Fatalf("add cogni declaration: %v", err)
	}
	gw.SetCogniRegistry(reg, t.TempDir())
	gw.SetCogniWorkflowEngine(cogni.NewWorkflowEngine(func(ctx context.Context, skillName string, args map[string]any) (any, error) {
		if skillName != "summarize" {
			t.Fatalf("unexpected skill: %s", skillName)
		}
		return "summary:" + args["topic"].(string), nil
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/cognis/reviewer/workflows", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("workflow list status = %d, body = %s", w.Code, w.Body.String())
	}
	var list map[string]any
	if err := json.NewDecoder(w.Body).Decode(&list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if list["count"].(float64) != 1 {
		t.Fatalf("expected one workflow, got %#v", list)
	}

	body, _ := json.Marshal(map[string]any{"topic": "pack"})
	req = httptest.NewRequest(http.MethodPost, "/v1/cognis/reviewer/workflow/full_review", bytes.NewReader(body))
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("workflow run status = %d, body = %s", w.Code, w.Body.String())
	}
	var result cogni.WorkflowResult
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if !result.Success || result.Outputs["summary"] != "summary:pack" {
		t.Fatalf("unexpected workflow result: %#v", result)
	}
}

func TestCogniKernelPackServesObservabilityAndVerify(t *testing.T) {
	gw, tm := newTestGatewayWithCogniKernelPack(t, packruntime.PackStatusEnabled)
	tenant := tm.Register("cogni-observe")

	reg := cogni.NewRegistry()
	if err := reg.Add(&cogni.Declaration{
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
		t.Fatalf("add cogni declaration: %v", err)
	}
	gw.SetCogniRegistry(reg, t.TempDir())
	traces := cogni.NewInMemoryTraceStore(10)
	traces.Record(cogni.Trace{
		Activations: []cogni.TraceActivation{{
			ID:        "reviewer",
			Activated: true,
			Score:     0.9,
		}},
		DurationMs: 12,
	})
	gw.SetCogniTraceStore(traces)
	gw.SetCogniSentinel(cogni.NewSentinel(traces, reg, cogni.SentinelPolicy{}))

	req := httptest.NewRequest(http.MethodGet, "/v1/cognis/traces", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("traces status = %d, body = %s", w.Code, w.Body.String())
	}
	var traceBody map[string]any
	if err := json.NewDecoder(w.Body).Decode(&traceBody); err != nil {
		t.Fatalf("decode traces: %v", err)
	}
	if traceBody["count"].(float64) != 1 {
		t.Fatalf("expected one trace, got %#v", traceBody)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/cognis/reviewer/verify", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("verify status = %d, body = %s", w.Code, w.Body.String())
	}
	var verify map[string]any
	if err := json.NewDecoder(w.Body).Decode(&verify); err != nil {
		t.Fatalf("decode verify: %v", err)
	}
	if verify["id"] != "reviewer" || verify["passed"].(float64) != 1 {
		t.Fatalf("unexpected verify response: %#v", verify)
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/cognis/alerts/scan", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("alerts scan status = %d, body = %s", w.Code, w.Body.String())
	}
}

func TestCogniKernelPackServesExperienceRecordAndSummary(t *testing.T) {
	gw, tm := newTestGatewayWithCogniKernelPack(t, packruntime.PackStatusEnabled)
	tenant := tm.Register("cogni-experience")

	reg := cogni.NewRegistry()
	if err := reg.Add(&cogni.Declaration{ID: "reviewer", DisplayName: "Reviewer"}, "test"); err != nil {
		t.Fatalf("add cogni declaration: %v", err)
	}
	gw.SetCogniRegistry(reg, t.TempDir())

	store := cogni.NewExperienceStore("reviewer", cogni.ExperienceConfig{StoreDir: t.TempDir()})
	gw.SetCogniExperiences(map[string]*cogni.ExperienceStore{"reviewer": store})

	toolData, _ := json.Marshal(cogni.ToolExperience{
		Tool:       "review",
		Context:    "go code",
		Learned:    "check package boundaries",
		Confidence: 0.9,
	})
	body, _ := json.Marshal(map[string]any{"type": "tool_memory", "data": json.RawMessage(toolData)})
	req := httptest.NewRequest(http.MethodPost, "/v1/cognis/reviewer/experience/record", bytes.NewReader(body))
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("record status = %d, body = %s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/cognis/reviewer/experience", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("summary status = %d, body = %s", w.Code, w.Body.String())
	}
	var report map[string]any
	if err := json.NewDecoder(w.Body).Decode(&report); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if report["enabled"] != true || report["id"] != "reviewer" {
		t.Fatalf("unexpected response: %#v", report)
	}
	if len(store.ToolMemory("review")) != 1 {
		t.Fatalf("expected tool memory to be recorded")
	}
}

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
