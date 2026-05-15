package gateway

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"yunque-agent/internal/controlplane/tenant"
	skillanomalypack "yunque-agent/internal/packs/skillanomaly"
	"yunque-agent/pkg/packruntime"
)

func TestSkillAnomalyPackGateReturnsNotFoundWhenDisabled(t *testing.T) {
	gw, tm := newTestGatewayWithSkillAnomalyPack(t, packruntime.PackStatusDisabled)
	tenant := tm.Register("skill-anomaly-disabled")

	req := httptest.NewRequest(http.MethodGet, "/v1/skill-anomaly/status", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("disabled Skill Anomaly pack should gate status, status = %d, body = %s", w.Code, w.Body.String())
	}
}

func TestSkillAnomalyPackRoutesStatusWhenEnabled(t *testing.T) {
	gw, tm := newTestGatewayWithSkillAnomalyPack(t, packruntime.PackStatusEnabled)
	tenant := tm.Register("skill-anomaly-enabled")

	req := httptest.NewRequest(http.MethodGet, "/v1/skill-anomaly/status", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)

	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "yunque.pack.skill-anomaly") {
		t.Fatalf("enabled Skill Anomaly pack should expose status, status = %d, body = %s", w.Code, w.Body.String())
	}
}

func TestSkillAnomalyPackRouteSpecsGateByMethod(t *testing.T) {
	gw, tm := newTestGatewayWithSkillAnomalyPack(t, packruntime.PackStatusEnabled)
	tenant := tm.Register("skill-anomaly-method-gate")

	req := httptest.NewRequest(http.MethodGet, "/v1/skill-anomaly/detect", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /v1/skill-anomaly/detect should be blocked by pack method gate, status = %d, body = %s", w.Code, w.Body.String())
	}
}

func TestSkillAnomalyPackCanObserveAndDetect(t *testing.T) {
	gw, tm := newTestGatewayWithSkillAnomalyPack(t, packruntime.PackStatusEnabled)
	tenant := tm.Register("skill-anomaly-flow")

	body := `{"skill_slug":"text_processing","action":"read_file","params":{"path":"notes.md"},"success":true,"duration_ms":100}`
	for i := 0; i < 20; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/skill-anomaly/events", strings.NewReader(body))
		req.Header.Set("X-API-Key", tenant.APIKey)
		w := httptest.NewRecorder()
		gw.ServeHTTP(w, req)
		if w.Code != http.StatusCreated || !strings.Contains(w.Body.String(), "observed") {
			t.Fatalf("observe baseline status=%d body=%s", w.Code, w.Body.String())
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/skill-anomaly/detect", strings.NewReader(`{"skill_slug":"text_processing","action":"shell_exec","params":{"command":"whoami","exfil_url":"https://example.invalid"},"success":false,"duration_ms":500,"dry_run":true}`))
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "needs_approval") || !strings.Contains(w.Body.String(), "new_action") {
		t.Fatalf("detect status=%d body=%s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/skill-anomaly/audit-hook/plan", strings.NewReader(`{"skill_slug":"text_processing","action":"shell_exec","params":{"command":"whoami","exfil_url":"https://example.invalid"},"success":false,"duration_ms":500,"requested_by":"operator","reason":"review anomalous shell execution"}`))
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "approval_plan") || !strings.Contains(w.Body.String(), "trust_mutation") || !strings.Contains(w.Body.String(), "merkle_append_ready") {
		t.Fatalf("audit hook plan status=%d body=%s", w.Code, w.Body.String())
	}
}

func newTestGatewayWithSkillAnomalyPack(t *testing.T, status packruntime.PackStatus) (*Gateway, *tenant.Manager) {
	t.Helper()
	registry, err := packruntime.NewRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	_, err = registry.Install(packruntime.Manifest{
		ID:           skillanomalypack.PackID,
		Name:         "Skill Anomaly Pack",
		Version:      "0.1.0",
		Optional:     true,
		DefaultState: "disabled",
		Backend: packruntime.BackendManifest{
			Routes: []string{
				"/v1/skill-anomaly/status",
				"/v1/skill-anomaly/events",
				"/v1/skill-anomaly/profiles",
				"/v1/skill-anomaly/profiles/",
				"/v1/skill-anomaly/detect",
				"/v1/skill-anomaly/audit-hook/plan",
				"/v1/skill-anomaly/evidence/",
			},
			RouteSpecs: []packruntime.BackendRouteSpec{
				{Method: http.MethodGet, Path: "/v1/skill-anomaly/status"},
				{Method: http.MethodGet, Path: "/v1/skill-anomaly/events"},
				{Method: http.MethodPost, Path: "/v1/skill-anomaly/events"},
				{Method: http.MethodGet, Path: "/v1/skill-anomaly/profiles"},
				{Method: http.MethodGet, Path: "/v1/skill-anomaly/profiles/"},
				{Method: http.MethodPost, Path: "/v1/skill-anomaly/detect"},
				{Method: http.MethodPost, Path: "/v1/skill-anomaly/audit-hook/plan"},
				{Method: http.MethodGet, Path: "/v1/skill-anomaly/evidence/"},
			},
		},
		Frontend: packruntime.FrontendManifest{Menus: []packruntime.FrontendMenu{{Key: "skill-anomaly", Label: "Skill 异常", Path: "/packs/skill-anomaly"}}},
		SDK:      packruntime.SDKManifest{TypeScript: "yunque-client/skill-anomaly"},
		Update:   packruntime.UpdateManifest{Rollback: true},
	}, "test")
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if status == packruntime.PackStatusEnabled {
		if _, err := registry.Enable(skillanomalypack.PackID); err != nil {
			t.Fatalf("Enable: %v", err)
		}
	}
	gw, tm := newTestGatewayWithConfig(GatewayConfig{Packs: registry})
	gw.RegisterBackendPack(skillanomalypack.New(skillanomalypack.Config{DataDir: t.TempDir()}))
	return gw, tm
}
