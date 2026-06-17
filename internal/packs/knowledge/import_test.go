package knowledgepack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yunque-agent/internal/agentcore/knowledge"
)

// fakeKnowledgeGateway is a minimal KnowledgeGateway for the native import
// handlers. It fakes the SSRF-safe fetch (the real SSRF guard lives in the
// gateway and is tested there) so these tests can hit a loopback httptest
// server and exercise the pack's orchestration + the knowledge domain layer.
type fakeKnowledgeGateway struct {
	outputDir string
	fetch     func(rawURL, fallbackName string) (*knowledge.ImportPage, error)
}

var _ KnowledgeGateway = (*fakeKnowledgeGateway)(nil)

func (f *fakeKnowledgeGateway) HandleKnowledgePack(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
}

func (f *fakeKnowledgeGateway) FetchImportPage(rawURL, fallbackName string) (*knowledge.ImportPage, error) {
	if f.fetch != nil {
		return f.fetch(rawURL, fallbackName)
	}
	return nil, fmt.Errorf("no fetcher configured")
}

func (f *fakeKnowledgeGateway) OutputDir() string { return f.outputDir }

func (f *fakeKnowledgeGateway) TenantOf(ctx context.Context) string { return "test-tenant" }

// httpFetchGateway fetches against a (loopback) test server without the SSRF
// guard, then builds the page via the domain layer — mirroring what the real
// gateway's FetchImportPage does minus the SSRF transport.
func httpFetchGateway(outputDir string) *fakeKnowledgeGateway {
	return &fakeKnowledgeGateway{
		outputDir: outputDir,
		fetch: func(rawURL, fallbackName string) (*knowledge.ImportPage, error) {
			resp, err := http.Get(rawURL)
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return nil, err
			}
			return knowledge.BuildPage(rawURL, fallbackName, string(body), resp.Header.Get("Content-Type"))
		},
	}
}

func postJSON(t *testing.T, handler http.HandlerFunc, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw)))
	return rec
}

func assertJSONError(t *testing.T, rec *httptest.ResponseRecorder, wantSubstr string) {
	t.Helper()
	var out struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode error body: %v: %s", err, rec.Body.String())
	}
	if out.Error == "" {
		t.Fatalf("expected error field, got: %s", rec.Body.String())
	}
	if !strings.Contains(out.Error, wantSubstr) {
		t.Fatalf("error = %q, want contains %q", out.Error, wantSubstr)
	}
}

// TestImportURLNativeSuccess covers the de-shelled import-url happy path against
// a real httptest server: the title is extracted (domain layer), the page is
// ingested as a URL source, and the response is well-formed.
func TestImportURLNativeSuccess(t *testing.T) {
	const page = `<html><head><title>Example Doc</title></head><body><main><h1>Example Doc</h1><p>Hello knowledge import.</p></main></body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = io.WriteString(w, page)
	}))
	defer srv.Close()

	h := NewHandlerWithStore(httpFetchGateway(""), knowledge.NewStore(500))
	rec := postJSON(t, h.handleImportURL, "/v1/knowledge/import-url", map[string]any{"url": srv.URL})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Imported int               `json:"imported"`
		Source   *knowledge.Source `json:"source"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("body: %v: %s", err, rec.Body.String())
	}
	if out.Imported != 1 {
		t.Fatalf("imported = %d, want 1", out.Imported)
	}
	if out.Source == nil || out.Source.Type != knowledge.SourceURL {
		t.Fatalf("source = %+v, want type=%q", out.Source, knowledge.SourceURL)
	}
	if out.Source.Name != "Example Doc" {
		t.Fatalf("source name = %q, want %q (title extracted by domain layer)", out.Source.Name, "Example Doc")
	}
}

// TestImportURLErrors covers request validation, fetch failures, and the
// not-configured guard (all returning an understandable JSON error).
func TestImportURLErrors(t *testing.T) {
	store := knowledge.NewStore(500)

	// Empty URL → "url required" (fetch must not be attempted).
	noFetch := &fakeKnowledgeGateway{fetch: func(string, string) (*knowledge.ImportPage, error) {
		t.Fatal("fetch should not be called for an empty url")
		return nil, nil
	}}
	hEmpty := NewHandlerWithStore(noFetch, store)
	recEmpty := postJSON(t, hEmpty.handleImportURL, "/v1/knowledge/import-url", map[string]any{"url": "   "})
	assertJSONError(t, recEmpty, "url required")

	// Fetch failure → error surfaced.
	failGW := &fakeKnowledgeGateway{fetch: func(string, string) (*knowledge.ImportPage, error) {
		return nil, fmt.Errorf("fetch failed: boom")
	}}
	hFail := NewHandlerWithStore(failGW, store)
	recFail := postJSON(t, hFail.handleImportURL, "/v1/knowledge/import-url", map[string]any{"url": "https://example.com"})
	assertJSONError(t, recFail, "fetch failed")

	// No store wired → not configured.
	hNoStore := NewHandler(&fakeKnowledgeGateway{})
	recNC := postJSON(t, hNoStore.handleImportURL, "/v1/knowledge/import-url", map[string]any{"url": "https://example.com"})
	assertJSONError(t, recNC, "not configured")
}

// TestImportRepoNative covers import-repo validation, the security root gate
// (path outside roots → 403) and the happy path (path under outputDir → repo
// source ingested).
func TestImportRepoNative(t *testing.T) {
	t.Setenv("KB_IMPORT_ALLOW_ANY", "")
	root := t.TempDir()
	repo := filepath.Join(root, "myrepo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	h := NewHandlerWithStore(httpFetchGateway(root), knowledge.NewStore(500))

	// Missing path → "path required".
	recMissing := postJSON(t, h.handleImportRepo, "/v1/knowledge/import-repo", map[string]any{})
	assertJSONError(t, recMissing, "path required")

	// Path outside the configured roots → 403.
	outside := t.TempDir()
	recOut := postJSON(t, h.handleImportRepo, "/v1/knowledge/import-repo", map[string]any{"path": outside})
	if recOut.Code != http.StatusForbidden {
		t.Fatalf("outside-roots status = %d, want 403: %s", recOut.Code, recOut.Body.String())
	}

	// Path under the output dir root → success, ingested as a repo source.
	recOK := postJSON(t, h.handleImportRepo, "/v1/knowledge/import-repo", map[string]any{"path": repo})
	if recOK.Code != http.StatusOK {
		t.Fatalf("inside-root status = %d, want 200: %s", recOK.Code, recOK.Body.String())
	}
	var out struct {
		Source *knowledge.Source `json:"source"`
	}
	if err := json.Unmarshal(recOK.Body.Bytes(), &out); err != nil {
		t.Fatalf("body: %v: %s", err, recOK.Body.String())
	}
	if out.Source == nil || out.Source.Type != knowledge.SourceRepo {
		t.Fatalf("source = %+v, want type=%q", out.Source, knowledge.SourceRepo)
	}
}

// TestKnowledgeRoutesConsistentWithManifest asserts the pack mounts exactly the
// official manifest's /v1/knowledge/* path set, each with a non-nil handler and
// declared methods, and with no duplicate path (which would panic the mux on
// registration) — i.e. manifest backend.routes ↔ handler.Routes() consistency.
func TestKnowledgeRoutesConsistentWithManifest(t *testing.T) {
	manifestPath := filepath.Join("..", "..", "..", "packs", "official", "knowledge-pack", "pack.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var manifest struct {
		Backend struct {
			Routes []string `json:"routes"`
		} `json:"backend"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	manifestRoutes := map[string]bool{}
	for _, p := range manifest.Backend.Routes {
		manifestRoutes[p] = true
	}
	if len(manifestRoutes) == 0 {
		t.Fatal("manifest declares no backend routes")
	}

	h := NewHandlerWithStore(&fakeKnowledgeGateway{}, knowledge.NewStore(500))
	seen := map[string]bool{}
	for _, rt := range h.Routes() {
		if seen[rt.Path] {
			t.Fatalf("duplicate route %q (would panic on mux register)", rt.Path)
		}
		seen[rt.Path] = true
		if rt.Handler == nil {
			t.Fatalf("route %q has nil handler", rt.Path)
		}
		if len(rt.Methods) == 0 {
			t.Fatalf("route %q declares no methods", rt.Path)
		}
		if !manifestRoutes[rt.Path] {
			t.Fatalf("route %q not in manifest backend.routes", rt.Path)
		}
	}
	for p := range manifestRoutes {
		if !seen[p] {
			t.Fatalf("manifest route %q missing from Routes()", p)
		}
	}
}
