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
	if len(routes) != 7 {
		t.Fatalf("expected 7 SBOM drift routes, got %d", len(routes))
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
		"/v1/sbom-drift/status":       {http.MethodGet},
		"/v1/sbom-drift/snapshots":    {http.MethodGet, http.MethodPost},
		"/v1/sbom-drift/snapshots/":   {http.MethodGet},
		"/v1/sbom-drift/diff":         {http.MethodPost},
		"/v1/sbom-drift/cyclonedx/":   {http.MethodGet},
		"/v1/sbom-drift/ci-gate/plan": {http.MethodPost},
		"/v1/sbom-drift/evidence/":    {http.MethodGet},
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

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
