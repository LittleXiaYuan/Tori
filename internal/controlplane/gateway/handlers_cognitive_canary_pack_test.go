package gateway

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"yunque-agent/internal/controlplane/tenant"
	cognitivecanarypack "yunque-agent/internal/packs/cognitivecanary"
	"yunque-agent/pkg/packruntime"
)

func TestCognitiveCanaryPackGateReturnsNotFoundWhenDisabled(t *testing.T) {
	gw, tm := newTestGatewayWithCognitiveCanaryPack(t, packruntime.PackStatusDisabled)
	tenant := tm.Register("cognitive-canary-disabled")

	req := httptest.NewRequest(http.MethodGet, "/v1/cognitive-canary/status", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("disabled Cognitive Canary pack should gate status, status = %d, body = %s", w.Code, w.Body.String())
	}
}

func TestCognitiveCanaryPackRoutesStatusWhenEnabled(t *testing.T) {
	gw, tm := newTestGatewayWithCognitiveCanaryPack(t, packruntime.PackStatusEnabled)
	tenant := tm.Register("cognitive-canary-enabled")

	req := httptest.NewRequest(http.MethodGet, "/v1/cognitive-canary/status", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)

	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "yunque.pack.cognitive-canary") {
		t.Fatalf("enabled Cognitive Canary pack should expose status, status = %d, body = %s", w.Code, w.Body.String())
	}
}

func TestCognitiveCanaryPackRouteSpecsGateByMethod(t *testing.T) {
	gw, tm := newTestGatewayWithCognitiveCanaryPack(t, packruntime.PackStatusEnabled)
	tenant := tm.Register("cognitive-canary-method-gate")

	req := httptest.NewRequest(http.MethodGet, "/v1/cognitive-canary/evaluate", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /v1/cognitive-canary/evaluate should be blocked by pack method gate, status = %d, body = %s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/cognitive-canary/shadow/plan", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /v1/cognitive-canary/shadow/plan should be blocked by pack method gate, status = %d, body = %s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/cognitive-canary/response-collector/writeback", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /v1/cognitive-canary/response-collector/writeback should be blocked by pack method gate, status = %d, body = %s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/cognitive-canary/response-collector/pipeline/plan", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /v1/cognitive-canary/response-collector/pipeline/plan should be blocked by pack method gate, status = %d, body = %s", w.Code, w.Body.String())
	}
}

func TestCognitiveCanaryPackCanSaveScenariosAndEvaluate(t *testing.T) {
	gw, tm := newTestGatewayWithCognitiveCanaryPack(t, packruntime.PackStatusEnabled)
	tenant := tm.Register("cognitive-canary-flow")

	scenarioBody := `{"scenarios":[{"id":"runtime-quality-check","name":"Runtime quality check","category":"planner","question":"How should the agent handle a failing runtime check?","stable_response":"Check the health endpoint, inspect logs, and roll back if the config change caused the failure.","canary_response":"Check the health endpoint, inspect logs, capture evidence, and prepare rollback if the config change caused the failure.","expected_keywords":["health","logs","rollback"],"stable_latency_ms":800,"canary_latency_ms":850,"enabled":true,"weight":1}],"replace":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/cognitive-canary/scenarios", strings.NewReader(scenarioBody))
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusCreated || !strings.Contains(w.Body.String(), "saved") {
		t.Fatalf("save scenarios status=%d body=%s", w.Code, w.Body.String())
	}

	evaluateBody := `{"scenario_ids":["runtime-quality-check"],"persist":false,"candidate_version":"1.1.0-rc1"}`
	req = httptest.NewRequest(http.MethodPost, "/v1/cognitive-canary/evaluate", strings.NewReader(evaluateBody))
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "quality_score") || !strings.Contains(w.Body.String(), "promotion_decision") {
		t.Fatalf("evaluate cognitive canary status=%d body=%s", w.Code, w.Body.String())
	}

	planBody := `{"candidate_version":"1.1.0-rc1","stable_version":"1.0.0","traffic_percent":5,"requested_by":"unit"}`
	req = httptest.NewRequest(http.MethodPost, "/v1/cognitive-canary/shadow/plan", strings.NewReader(planBody))
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"shadow_plan_ready":true`) || !strings.Contains(w.Body.String(), `"response_collector_plan_ready":true`) || !strings.Contains(w.Body.String(), `"response_collector_ready":false`) || !strings.Contains(w.Body.String(), `"auto_rollback_ready":false`) {
		t.Fatalf("shadow plan status=%d body=%s", w.Code, w.Body.String())
	}

	writebackBody := `{"candidate_version":"1.1.0-rc1","stable_version":"1.0.0","sample_percent":5,"requested_by":"unit","reason":"persist response collector store"}`
	req = httptest.NewRequest(http.MethodPost, "/v1/cognitive-canary/response-collector/writeback", strings.NewReader(writebackBody))
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusAccepted || !strings.Contains(w.Body.String(), `"response_collector_writeback_ready":true`) || !strings.Contains(w.Body.String(), `"writes_response_collector_store":true`) || !strings.Contains(w.Body.String(), `"response_collector_ready":false`) || !strings.Contains(w.Body.String(), `"shadow_traffic_ready":false`) {
		t.Fatalf("response collector writeback status=%d body=%s", w.Code, w.Body.String())
	}

	pipelineBody := `{"requested_by":"unit","reason":"plan live collector pipeline handoff"}`
	req = httptest.NewRequest(http.MethodPost, "/v1/cognitive-canary/response-collector/pipeline/plan", strings.NewReader(pipelineBody))
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"response_collector_pipeline_plan_ready":true`) || !strings.Contains(w.Body.String(), `"consumes_response_collector_store":true`) || !strings.Contains(w.Body.String(), `"response_collector_ready":false`) || !strings.Contains(w.Body.String(), `"prometheus_ready":false`) {
		t.Fatalf("response collector pipeline plan status=%d body=%s", w.Code, w.Body.String())
	}
}

func newTestGatewayWithCognitiveCanaryPack(t *testing.T, status packruntime.PackStatus) (*Gateway, *tenant.Manager) {
	t.Helper()
	registry, err := packruntime.NewRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	_, err = registry.Install(packruntime.Manifest{
		ID:           cognitivecanarypack.PackID,
		Name:         "Cognitive Canary Pack",
		Version:      "0.1.0",
		Optional:     true,
		DefaultState: "disabled",
		Backend: packruntime.BackendManifest{
			Routes: []string{
				"/v1/cognitive-canary/status",
				"/v1/cognitive-canary/scenarios",
				"/v1/cognitive-canary/evaluate",
				"/v1/cognitive-canary/shadow/plan",
				"/v1/cognitive-canary/response-collector/writeback",
				"/v1/cognitive-canary/response-collector/pipeline/plan",
				"/v1/cognitive-canary/reports",
				"/v1/cognitive-canary/reports/",
				"/v1/cognitive-canary/evidence/",
			},
			RouteSpecs: []packruntime.BackendRouteSpec{
				{Method: http.MethodGet, Path: "/v1/cognitive-canary/status"},
				{Method: http.MethodGet, Path: "/v1/cognitive-canary/scenarios"},
				{Method: http.MethodPost, Path: "/v1/cognitive-canary/scenarios"},
				{Method: http.MethodPost, Path: "/v1/cognitive-canary/evaluate"},
				{Method: http.MethodPost, Path: "/v1/cognitive-canary/shadow/plan"},
				{Method: http.MethodPost, Path: "/v1/cognitive-canary/response-collector/writeback"},
				{Method: http.MethodPost, Path: "/v1/cognitive-canary/response-collector/pipeline/plan"},
				{Method: http.MethodGet, Path: "/v1/cognitive-canary/reports"},
				{Method: http.MethodGet, Path: "/v1/cognitive-canary/reports/"},
				{Method: http.MethodGet, Path: "/v1/cognitive-canary/evidence/"},
			},
		},
		Frontend: packruntime.FrontendManifest{Menus: []packruntime.FrontendMenu{{Key: "cognitive-canary", Label: "Cognitive Canary", Path: "/packs/cognitive-canary"}}},
		SDK:      packruntime.SDKManifest{TypeScript: "yunque-client/cognitive-canary"},
		Update:   packruntime.UpdateManifest{Rollback: true},
	}, "test")
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if status == packruntime.PackStatusEnabled {
		if _, err := registry.Enable(cognitivecanarypack.PackID); err != nil {
			t.Fatalf("Enable: %v", err)
		}
	}
	gw, tm := newTestGatewayWithConfig(GatewayConfig{Packs: registry})
	gw.RegisterBackendPack(cognitivecanarypack.New(cognitivecanarypack.Config{DataDir: t.TempDir()}))
	return gw, tm
}
