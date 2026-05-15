package chaosprobe

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestChaosProbeHandlerRoutesExposePackShellSurface(t *testing.T) {
	h := New(Config{DataDir: t.TempDir()})
	if h.PackID() != PackID {
		t.Fatalf("PackID = %q, want %q", h.PackID(), PackID)
	}
	routes := h.Routes()
	if len(routes) != 6 {
		t.Fatalf("expected 6 Chaos Probe routes, got %d", len(routes))
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
		"/v1/chaos-probe/status":    {http.MethodGet},
		"/v1/chaos-probe/probes":    {http.MethodGet, http.MethodPost},
		"/v1/chaos-probe/run":       {http.MethodPost},
		"/v1/chaos-probe/reports":   {http.MethodGet},
		"/v1/chaos-probe/reports/":  {http.MethodGet},
		"/v1/chaos-probe/evidence/": {http.MethodGet},
	}
	for path, methods := range expected {
		if got, want := strings.Join(byPath[path], ","), strings.Join(methods, ","); got != want {
			t.Fatalf("expected %s methods %s, got %s", path, want, got)
		}
	}
}

func TestChaosProbeRunPersistsReportAndExportsEvidence(t *testing.T) {
	now := time.Date(2026, 5, 15, 13, 0, 0, 0, time.UTC)
	h := New(Config{DataDir: t.TempDir(), Now: func() time.Time { return now }, Policy: ProbePolicy{MemoryWarnBytes: ^uint64(0)}})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chaos-probe/run", strings.NewReader(`{"probe_ids":["runtime-healthz-probe","guardrail-probe"],"persist":true,"metadata":{"source":"unit"}}`))
	h.Run(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "health_score") || !strings.Contains(w.Body.String(), "pass") {
		t.Fatalf("run status=%d body=%s", w.Code, w.Body.String())
	}
	var run struct {
		Report ChaosReport `json:"report"`
		Status string      `json:"status"`
	}
	if err := json.NewDecoder(w.Body).Decode(&run); err != nil {
		t.Fatalf("decode run: %v", err)
	}
	if run.Report.ID == "" || run.Report.GateStatus != "pass" || run.Report.ProbeCount != 2 {
		t.Fatalf("expected passing chaos probe report, got %#v", run.Report)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/chaos-probe/reports/"+run.Report.ID, nil)
	h.ReportDetail(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), run.Report.ID) {
		t.Fatalf("report detail status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/chaos-probe/evidence/"+run.Report.ID, nil)
	h.Evidence(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "json-chaos-probe-evidence") || !strings.Contains(w.Body.String(), "chaos-report.json") {
		t.Fatalf("evidence status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestChaosProbeDefinitionsRejectInvalidID(t *testing.T) {
	h := New(Config{DataDir: t.TempDir()})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chaos-probe/probes", strings.NewReader(`{"probes":[{"id":"../../bad","name":"bad","category":"storage","enabled":true,"safe":true}],"replace":true}`))
	h.Probes(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request for invalid probe id, got %d body=%s", w.Code, w.Body.String())
	}
}
