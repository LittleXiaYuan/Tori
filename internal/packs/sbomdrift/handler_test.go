package sbomdrift

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSBOMDriftHandlerRoutesExposePackShellSurface(t *testing.T) {
	h := New(Config{RepoRoot: t.TempDir(), DataDir: t.TempDir()})
	if h.PackID() != PackID {
		t.Fatalf("PackID = %q, want %q", h.PackID(), PackID)
	}
	routes := h.Routes()
	if len(routes) != 9 {
		t.Fatalf("expected 9 SBOM drift routes, got %d", len(routes))
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
		"/v1/sbom-drift/status":                          {http.MethodGet},
		"/v1/sbom-drift/snapshots":                       {http.MethodGet, http.MethodPost},
		"/v1/sbom-drift/snapshots/":                      {http.MethodGet},
		"/v1/sbom-drift/diff":                            {http.MethodPost},
		"/v1/sbom-drift/cyclonedx/":                      {http.MethodGet},
		"/v1/sbom-drift/ci-gate/plan":                    {http.MethodPost},
		"/v1/sbom-drift/ci-gate/baseline/writeback":      {http.MethodPost},
		"/v1/sbom-drift/ci-gate/workflow/writeback/plan": {http.MethodPost},
		"/v1/sbom-drift/evidence/":                       {http.MethodGet},
	}
	for path, methods := range expected {
		if got, want := strings.Join(byPath[path], ","), strings.Join(methods, ","); got != want {
			t.Fatalf("expected %s methods %s, got %s", path, want, got)
		}
	}
}

func TestSBOMDriftSnapshotDiffAndEvidence(t *testing.T) {
	repo := t.TempDir()
	writeFile(t, filepath.Join(repo, "go.mod"), `module example.com/demo

go 1.22

require (
	github.com/example/direct v1.2.3
	github.com/example/indirect v0.1.0 // indirect
)
`)
	writeFile(t, filepath.Join(repo, "web", "package.json"), `{"name":"web","dependencies":{"react":"18.2.0"},"devDependencies":{"vite":"^5.0.0"}}`)
	now := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	h := New(Config{RepoRoot: repo, DataDir: t.TempDir(), Now: func() time.Time { return now }})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/sbom-drift/snapshots", strings.NewReader(`{"id":"baseline","source":"unit-test"}`))
	h.Snapshots(w, req)
	if w.Code != http.StatusCreated || !strings.Contains(w.Body.String(), "github.com/example/direct") {
		t.Fatalf("create baseline status=%d body=%s", w.Code, w.Body.String())
	}

	writeFile(t, filepath.Join(repo, "go.mod"), `module example.com/demo

go 1.22

require (
	github.com/example/direct v2.0.0
	github.com/example/newdep v0.1.0
)
`)
	writeFile(t, filepath.Join(repo, "web", "package.json"), `{"name":"web","dependencies":{"react":"19.0.0"}}`)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/sbom-drift/diff", strings.NewReader(`{"base_id":"baseline","target_current":true}`))
	h.Diff(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("diff status=%d body=%s", w.Code, w.Body.String())
	}
	var diffResp struct {
		Diff DiffResult `json:"diff"`
	}
	if err := json.NewDecoder(w.Body).Decode(&diffResp); err != nil {
		t.Fatalf("decode diff: %v", err)
	}
	if diffResp.Diff.RiskLevel != "high" || len(diffResp.Diff.Added) == 0 || len(diffResp.Diff.Changed) == 0 {
		t.Fatalf("expected high-risk added/changed drift, got %#v", diffResp.Diff)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/sbom-drift/evidence/baseline", nil)
	h.Evidence(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "json-sbom-drift-evidence") || !strings.Contains(w.Body.String(), "snapshot.json") {
		t.Fatalf("evidence status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestSBOMDriftCycloneDXAndCIGatePlan(t *testing.T) {
	repo := t.TempDir()
	writeFile(t, filepath.Join(repo, "go.mod"), `module example.com/demo

go 1.22

require github.com/example/direct v1.2.3
`)
	now := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	h := New(Config{RepoRoot: repo, DataDir: t.TempDir(), Now: func() time.Time { return now }})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/sbom-drift/snapshots", strings.NewReader(`{"id":"baseline","source":"unit-test"}`))
	h.Snapshots(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create baseline status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/sbom-drift/cyclonedx/baseline", nil)
	h.CycloneDX(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"bomFormat":"CycloneDX"`) || !strings.Contains(w.Body.String(), "pkg:golang/github.com/example/direct@v1.2.3") {
		t.Fatalf("cyclonedx status=%d body=%s", w.Code, w.Body.String())
	}

	writeFile(t, filepath.Join(repo, "go.mod"), `module example.com/demo

go 1.22

require github.com/example/direct v2.0.0
`)
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/sbom-drift/ci-gate/plan", strings.NewReader(`{"base_id":"baseline","target_current":true,"fail_on_risk":"high","requested_by":"unit"}`))
	h.CIGatePlan(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"blocked":true`) || !strings.Contains(w.Body.String(), `"ci_gate_ready":false`) || !strings.Contains(w.Body.String(), "dist/sbom.cdx.json") {
		t.Fatalf("ci gate plan status=%d body=%s", w.Code, w.Body.String())
	}
	var planResp struct {
		Plan CIGatePlanReport `json:"plan"`
	}
	if err := json.NewDecoder(w.Body).Decode(&planResp); err != nil {
		t.Fatalf("decode ci gate plan: %v", err)
	}
	if !planResp.Plan.GovulncheckPlanReady || planResp.Plan.GovulncheckReady {
		t.Fatalf("expected plan-only govulncheck readiness, got %#v", planResp.Plan)
	}
	if planResp.Plan.GovulncheckPlan.Command != "govulncheck -json ./..." || planResp.Plan.GovulncheckPlan.ReportArtifact != "govulncheck-report.json" {
		t.Fatalf("unexpected govulncheck plan: %#v", planResp.Plan.GovulncheckPlan)
	}
	if planResp.Plan.GovulncheckPlan.WritesFiles || planResp.Plan.GovulncheckPlan.Executes || planResp.Plan.GovulncheckPlan.VulnerabilityDBFetch {
		t.Fatalf("govulncheck plan must remain non-destructive: %#v", planResp.Plan.GovulncheckPlan)
	}
	if planResp.Plan.GovulncheckPlan.ModuleCount != 1 || len(planResp.Plan.GovulncheckPlan.Packages) != 1 {
		t.Fatalf("expected one Go module in govulncheck plan, got %#v", planResp.Plan.GovulncheckPlan)
	}
}

func TestSBOMDriftCIBaselineWritebackPersistsPackLocalStoreOnly(t *testing.T) {
	repo := t.TempDir()
	writeFile(t, filepath.Join(repo, "go.mod"), `module example.com/demo

go 1.22

require github.com/example/direct v1.2.3
`)
	now := time.Date(2026, 5, 15, 13, 0, 0, 0, time.UTC)
	h := New(Config{RepoRoot: repo, DataDir: t.TempDir(), Now: func() time.Time { return now }})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/sbom-drift/snapshots", strings.NewReader(`{"id":"baseline","source":"unit-test"}`))
	h.Snapshots(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create baseline status=%d body=%s", w.Code, w.Body.String())
	}

	writeFile(t, filepath.Join(repo, "go.mod"), `module example.com/demo

go 1.22

require github.com/example/direct v2.0.0
`)
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/sbom-drift/ci-gate/baseline/writeback", strings.NewReader(`{"base_id":"baseline","target_current":true,"fail_on_risk":"high","requested_by":"unit","approval_id":"approval-sbom","request_key":"sbom-baseline"}`))
	h.CIBaselineWriteback(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ci baseline writeback status=%d body=%s", w.Code, w.Body.String())
	}
	var got struct {
		Writeback CIBaselineWritebackReport `json:"writeback"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode ci baseline writeback: %v", err)
	}
	if got.Writeback.Status != "ci_baseline_gate_record_stored_pending_ci_wiring" || !got.Writeback.CIBaselineStoreReady || !got.Writeback.CIBaselineWritebackReady || !got.Writeback.WritesCIBaselineStore {
		t.Fatalf("unexpected CI baseline writeback identity: %#v", got.Writeback)
	}
	if got.Writeback.CIGateReady || got.Writeback.GovulncheckReady || got.Writeback.VulnerabilityReady || got.Writeback.WritesCIWorkflow || got.Writeback.ExecutesGovulncheck || got.Writeback.BlocksRelease {
		t.Fatalf("CI baseline writeback must not execute scanner, write CI workflow, or block release: %#v", got.Writeback)
	}
	if got.Writeback.CIBaselineStore.RecordCount != 1 || got.Writeback.CIBaselineRecord.RequestKey != "sbom-baseline" || !got.Writeback.CIGatePlan.Blocked {
		t.Fatalf("unexpected CI baseline record/store: %#v", got.Writeback)
	}
	for _, artifact := range []string{"ci-baseline-store.json", "ci-baseline-record.json", "ci-gate-plan.json", "govulncheck-plan.json"} {
		if !containsString(got.Writeback.Artifacts, artifact) {
			t.Fatalf("writeback missing artifact %s: %#v", artifact, got.Writeback.Artifacts)
		}
	}
	if _, err := os.Stat(filepath.Join(h.dataDir, "ci-baseline-store.json")); err != nil {
		t.Fatalf("expected pack-local ci-baseline-store.json: %v", err)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/sbom-drift/ci-gate/baseline/writeback", strings.NewReader(`{"base_id":"baseline","target_current":true,"fail_on_risk":"high","request_key":"sbom-baseline"}`))
	h.CIBaselineWriteback(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("idempotent ci baseline writeback status=%d body=%s", w.Code, w.Body.String())
	}
	records, err := h.loadCIBaselineRecords()
	if err != nil {
		t.Fatalf("load CI baseline records: %v", err)
	}
	if len(records) != 1 || records[0].RequestKey != "sbom-baseline" {
		t.Fatalf("CI baseline store should replace by request key, records=%#v", records)
	}
}

func TestSBOMDriftCIWorkflowWritebackPlanConsumesPackLocalBaselineStore(t *testing.T) {
	repo := t.TempDir()
	writeFile(t, filepath.Join(repo, "go.mod"), `module example.com/demo

go 1.22

require github.com/example/direct v1.2.3
`)
	now := time.Date(2026, 5, 15, 14, 0, 0, 0, time.UTC)
	h := New(Config{RepoRoot: repo, DataDir: t.TempDir(), Now: func() time.Time { return now }})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/sbom-drift/snapshots", strings.NewReader(`{"id":"baseline","source":"unit-test"}`))
	h.Snapshots(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create baseline status=%d body=%s", w.Code, w.Body.String())
	}

	writeFile(t, filepath.Join(repo, "go.mod"), `module example.com/demo

go 1.22

require github.com/example/direct v2.0.0
`)
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/sbom-drift/ci-gate/baseline/writeback", strings.NewReader(`{"base_id":"baseline","target_current":true,"fail_on_risk":"high","requested_by":"unit","request_key":"sbom-baseline"}`))
	h.CIBaselineWriteback(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ci baseline writeback status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/sbom-drift/ci-gate/workflow/writeback/plan", strings.NewReader(`{"request_key":"sbom-baseline","workflow_path":".github/workflows/security.yml","job_name":"sbom-drift-gate","requested_by":"unit"}`))
	h.CIWorkflowWritebackPlan(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ci workflow writeback plan status=%d body=%s", w.Code, w.Body.String())
	}
	var got struct {
		Plan CIWorkflowWritebackPlanReport `json:"plan"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode CI workflow writeback plan: %v", err)
	}
	if got.Plan.Status != "ci_workflow_writeback_plan_ready_pending_ci_writer" || !got.Plan.CIWorkflowPlanReady || !got.Plan.ConsumesCIBaselineStore {
		t.Fatalf("unexpected workflow writeback plan identity: %#v", got.Plan)
	}
	if got.Plan.CIWorkflowWritebackReady || got.Plan.WritesCIWorkflow || got.Plan.ExecutesGovulncheck || got.Plan.BlocksRelease || got.Plan.GovulncheckReady {
		t.Fatalf("workflow writeback plan must stay plan-only: %#v", got.Plan)
	}
	if got.Plan.CIWorkflowHandoffPlan.WorkflowPath != ".github/workflows/security.yml" || got.Plan.CIWorkflowHandoffPlan.JobName != "sbom-drift-gate" {
		t.Fatalf("unexpected workflow handoff target: %#v", got.Plan.CIWorkflowHandoffPlan)
	}
	if !got.Plan.ReleaseBlockerPlan.WouldBlock || got.Plan.ReleaseBlockerPlan.BlocksRelease {
		t.Fatalf("release blocker must preview decision without blocking release: %#v", got.Plan.ReleaseBlockerPlan)
	}
	for _, artifact := range []string{"ci-workflow-writeback-plan.json", "ci-workflow-handoff-plan.json", "release-blocker-plan.json", "ci-baseline-store.json"} {
		if !containsString(got.Plan.Artifacts, artifact) {
			t.Fatalf("workflow writeback plan missing artifact %s: %#v", artifact, got.Plan.Artifacts)
		}
	}
	if _, err := os.Stat(filepath.Join(h.dataDir, "ci-baseline-store.json")); err != nil {
		t.Fatalf("baseline store should exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, ".github", "workflows", "security.yml")); !os.IsNotExist(err) {
		t.Fatalf("workflow writeback plan must not create workflow files, stat err=%v", err)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/sbom-drift/ci-gate/workflow/writeback/plan", strings.NewReader(`{"request_key":"sbom-baseline","workflow_path":"../security.yml"}`))
	h.CIWorkflowWritebackPlan(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid workflow path should be rejected, status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestSBOMDriftIgnoresNodeModulesPackageJSON(t *testing.T) {
	repo := t.TempDir()
	writeFile(t, filepath.Join(repo, "package.json"), `{"name":"root","dependencies":{"react":"18.2.0"}}`)
	writeFile(t, filepath.Join(repo, "node_modules", "left-pad", "package.json"), `{"name":"left-pad","dependencies":{"ignored":"1.0.0"}}`)
	h := New(Config{RepoRoot: repo, DataDir: t.TempDir()})
	snapshot, err := h.createSnapshot("baseline", "unit-test")
	if err != nil {
		t.Fatalf("createSnapshot: %v", err)
	}
	for _, component := range snapshot.Components {
		if component.Name == "ignored" || component.Name == "left-pad" {
			t.Fatalf("node_modules package should be ignored: %#v", component)
		}
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

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
