package skillanomaly

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSkillAnomalyHandlerRoutesExposePackShellSurface(t *testing.T) {
	h := New(Config{DataDir: t.TempDir()})
	if h.PackID() != PackID {
		t.Fatalf("PackID = %q, want %q", h.PackID(), PackID)
	}
	routes := h.Routes()
	if len(routes) != 7 {
		t.Fatalf("expected 7 Skill Anomaly routes, got %d", len(routes))
	}
	byPath := map[string][]string{}
	for _, route := range routes {
		methods := append([]string{}, route.Methods...)
		if route.Method != "" {
			methods = append([]string{route.Method}, methods...)
		}
		if route.Path == "" || route.Handler == nil || len(methods) == 0 {
			t.Fatalf("route must declare path, handler and method(s): %#v", route)
		}
		byPath[route.Path] = methods
	}
	expected := map[string][]string{
		"/v1/skill-anomaly/status":          {http.MethodGet},
		"/v1/skill-anomaly/events":          {http.MethodGet, http.MethodPost},
		"/v1/skill-anomaly/profiles":        {http.MethodGet},
		"/v1/skill-anomaly/profiles/":       {http.MethodGet},
		"/v1/skill-anomaly/detect":          {http.MethodPost},
		"/v1/skill-anomaly/audit-hook/plan": {http.MethodPost},
		"/v1/skill-anomaly/evidence/":       {http.MethodGet},
	}
	for path, methods := range expected {
		if got, want := strings.Join(byPath[path], ","), strings.Join(methods, ","); got != want {
			t.Fatalf("expected %s methods %s, got %s", path, want, got)
		}
	}
}

func TestSkillAnomalyObserveDetectAndEvidence(t *testing.T) {
	now := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	counter := 0
	h := New(Config{DataDir: t.TempDir(), Policy: DetectionPolicy{MinObservations: 3, WindowSize: 10}, Now: func() time.Time {
		counter++
		return now.Add(time.Duration(counter) * time.Minute)
	}})

	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/v1/skill-anomaly/events", strings.NewReader(`{"skill_slug":"text_processing","action":"read_file","params":{"path":"notes.md"},"success":true,"duration_ms":100}`))
		h.Events(w, req)
		if w.Code != http.StatusCreated || !strings.Contains(w.Body.String(), "observed") {
			t.Fatalf("observe baseline status=%d body=%s", w.Code, w.Body.String())
		}
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/skill-anomaly/detect", strings.NewReader(`{"skill_slug":"text_processing","action":"shell_exec","params":{"command":"whoami","exfil_url":"https://example.invalid"},"success":false,"duration_ms":500,"dry_run":true}`))
	h.Detect(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "needs_approval") || !strings.Contains(w.Body.String(), "new_action") || !strings.Contains(w.Body.String(), "new_param_keys") {
		t.Fatalf("detect status=%d body=%s", w.Code, w.Body.String())
	}
	var detected struct {
		Result DetectionResult `json:"result"`
	}
	if err := json.NewDecoder(w.Body).Decode(&detected); err != nil {
		t.Fatalf("decode detect: %v", err)
	}
	if !detected.Result.NeedsApproval || detected.Result.Score < 7 {
		t.Fatalf("expected high anomaly score and needs approval, got %#v", detected.Result)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/skill-anomaly/audit-hook/plan", strings.NewReader(`{"skill_slug":"text_processing","action":"shell_exec","params":{"command":"whoami","exfil_url":"https://example.invalid"},"success":false,"duration_ms":500,"requested_by":"operator","reason":"review anomalous shell execution"}`))
	h.AuditHookPlan(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "approval_plan") || !strings.Contains(w.Body.String(), "trust_mutation") || !strings.Contains(w.Body.String(), "merkle_append_ready") {
		t.Fatalf("audit hook plan status=%d body=%s", w.Code, w.Body.String())
	}
	var auditPlan struct {
		Plan AuditHookPlanReport `json:"plan"`
	}
	if err := json.NewDecoder(w.Body).Decode(&auditPlan); err != nil {
		t.Fatalf("decode audit hook plan: %v", err)
	}
	if !auditPlan.Plan.ApprovalRequired || auditPlan.Plan.AuditHookReady || auditPlan.Plan.TrustMutationReady || auditPlan.Plan.ApprovalWritebackReady {
		t.Fatalf("expected non-destructive approval plan boundaries, got %#v", auditPlan.Plan)
	}
	if auditPlan.Plan.TrustMutation.Delta >= 0 || auditPlan.Plan.ApprovalQueue.Reason != "review anomalous shell execution" {
		t.Fatalf("unexpected trust/approval plan: %#v", auditPlan.Plan)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/skill-anomaly/profiles/text_processing", nil)
	h.ProfileDetail(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "read_file") {
		t.Fatalf("profile status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/skill-anomaly/evidence/text_processing", nil)
	h.Evidence(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "json-skill-anomaly-evidence") || !strings.Contains(w.Body.String(), "detection-policy.json") || !strings.Contains(w.Body.String(), "audit-hook-plan.json") || !strings.Contains(w.Body.String(), "trust-mutation-plan.json") {
		t.Fatalf("evidence status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestSkillAnomalyRejectsInvalidSkillSlug(t *testing.T) {
	h := New(Config{DataDir: t.TempDir()})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/skill-anomaly/events", strings.NewReader(`{"skill_slug":"../../bad","action":"read"}`))
	h.Events(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request for invalid skill slug, got %d body=%s", w.Code, w.Body.String())
	}
}
