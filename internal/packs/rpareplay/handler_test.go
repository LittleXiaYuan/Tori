package rpareplay

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRPAReplayHandlerRoutesExposePackShellSurface(t *testing.T) {
	h := New(Config{DataDir: t.TempDir()})

	if h.PackID() != PackID {
		t.Fatalf("PackID = %q, want %q", h.PackID(), PackID)
	}

	routes := h.Routes()
	if len(routes) != 8 {
		t.Fatalf("expected 8 RPA replay routes, got %d", len(routes))
	}

	byPath := map[string][]string{}
	for _, route := range routes {
		if route.Path == "" || route.Handler == nil {
			t.Fatalf("route must declare path and handler: %#v", route)
		}
		methods := append([]string{}, route.Methods...)
		if route.Method != "" {
			methods = append([]string{route.Method}, methods...)
		}
		if len(methods) == 0 {
			t.Fatalf("route must declare method(s): %#v", route)
		}
		byPath[route.Path] = methods
	}

	expected := map[string][]string{
		"/v1/rpa-replay/status":           {http.MethodGet},
		"/v1/rpa-replay/traces":           {http.MethodGet, http.MethodPost},
		"/v1/rpa-replay/traces/":          {http.MethodGet},
		"/v1/rpa-replay/recordings/start": {http.MethodPost},
		"/v1/rpa-replay/recordings/stop":  {http.MethodPost},
		"/v1/rpa-replay/replay":           {http.MethodPost},
		"/v1/rpa-replay/executor/plan":    {http.MethodPost},
		"/v1/rpa-replay/evidence/":        {http.MethodGet},
	}
	for path, methods := range expected {
		got := strings.Join(byPath[path], ",")
		want := strings.Join(methods, ",")
		if got != want {
			t.Fatalf("expected %s methods %s, got %s", path, want, got)
		}
	}
}

func TestRPAReplayTraceStoreReplayAndEvidence(t *testing.T) {
	now := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	h := New(Config{DataDir: t.TempDir(), Now: func() time.Time { return now }})

	traceBody := `{"slug":"export-report","name":"Export report","target_url":"https://erp.example.com/reports","parameters":{"month":{"type":"string","required":true}},"steps":[{"action":"navigate","value":"https://erp.example.com/reports?month={{month}}","assertion":{"type":"url_matches","expected":"{{month}}"}}]}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/rpa-replay/traces", strings.NewReader(traceBody))
	h.Traces(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create trace status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/rpa-replay/traces", nil)
	h.Traces(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "export-report") {
		t.Fatalf("list traces status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/rpa-replay/replay", strings.NewReader(`{"slug":"export-report","params":{"month":"2026-05"},"dry_run":true}`))
	h.Replay(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("replay status=%d body=%s", w.Code, w.Body.String())
	}
	var replay struct {
		Result ReplayResult `json:"result"`
	}
	if err := json.NewDecoder(w.Body).Decode(&replay); err != nil {
		t.Fatalf("decode replay: %v", err)
	}
	if !replay.Result.Success || !replay.Result.DryRun || replay.Result.StepsRun != 1 || replay.Result.FailedStep != -1 {
		t.Fatalf("unexpected replay result: %#v", replay.Result)
	}
	if got := replay.Result.PlannedSteps[0].Value; !strings.Contains(got, "month=2026-05") {
		t.Fatalf("expected substituted plan value, got %q", got)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/rpa-replay/executor/plan", strings.NewReader(`{"slug":"export-report","params":{"month":"2026-05"},"requested_by":"unit","reason":"executor review","dry_run":true}`))
	h.ExecutorPlan(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("executor plan status=%d body=%s", w.Code, w.Body.String())
	}
	var executorPlan struct {
		Plan ExecutorPlanReport `json:"plan"`
	}
	if err := json.NewDecoder(w.Body).Decode(&executorPlan); err != nil {
		t.Fatalf("decode executor plan: %v", err)
	}
	if !executorPlan.Plan.ExecutorPlanReady || executorPlan.Plan.ExecutorReady || !executorPlan.Plan.BrowserIntentGatePlanReady || executorPlan.Plan.BrowserIntentReady || !executorPlan.Plan.ActionTracerPlanReady || executorPlan.Plan.ActionTracerReady {
		t.Fatalf("unexpected executor plan readiness: %#v", executorPlan.Plan)
	}
	if executorPlan.Plan.ConsumesBrowserIntent || executorPlan.Plan.ExecutesBrowserActions || executorPlan.Plan.WritesBrowserState || executorPlan.Plan.WritesFiles || executorPlan.Plan.NetworkAccess {
		t.Fatalf("executor plan must remain non-destructive: %#v", executorPlan.Plan)
	}
	if executorPlan.Plan.ActionCount != 1 || executorPlan.Plan.PlannedSteps[0].Value != "https://erp.example.com/reports?month=2026-05" {
		t.Fatalf("executor plan should substitute trace params: %#v", executorPlan.Plan.PlannedSteps)
	}
	for _, artifact := range []string{"executor-handoff-plan.json", "browser-intent-gate-plan.json", "action-tracer-plan.json"} {
		if !containsString(executorPlan.Plan.Artifacts, artifact) {
			t.Fatalf("executor plan missing artifact %s: %#v", artifact, executorPlan.Plan.Artifacts)
		}
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/rpa-replay/evidence/export-report", nil)
	h.Evidence(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "json-evidence-pack") || !strings.Contains(w.Body.String(), "trace.json") || !strings.Contains(w.Body.String(), "executor-handoff-plan.json") {
		t.Fatalf("evidence status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestRPAReplayRecordingSessionCanStopIntoTrace(t *testing.T) {
	h := New(Config{DataDir: t.TempDir(), Now: func() time.Time { return time.Unix(1778846400, 0).UTC() }})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/rpa-replay/recordings/start", strings.NewReader(`{"slug":"fill-form","name":"Fill form"}`))
	h.StartRecording(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("start status=%d body=%s", w.Code, w.Body.String())
	}
	var start struct {
		Session RecordingSession `json:"session"`
	}
	if err := json.NewDecoder(w.Body).Decode(&start); err != nil {
		t.Fatalf("decode start: %v", err)
	}

	w = httptest.NewRecorder()
	stopBody := `{"session_id":"` + start.Session.ID + `","steps":[{"action":"click","selector":"#submit"}]}`
	req = httptest.NewRequest(http.MethodPost, "/v1/rpa-replay/recordings/stop", strings.NewReader(stopBody))
	h.StopRecording(w, req)
	if w.Code != http.StatusCreated || !strings.Contains(w.Body.String(), "fill-form") {
		t.Fatalf("stop status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/rpa-replay/traces/fill-form", nil)
	h.TraceDetail(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "#submit") {
		t.Fatalf("trace detail status=%d body=%s", w.Code, w.Body.String())
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
