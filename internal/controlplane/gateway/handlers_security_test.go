package gateway

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"yunque-agent/internal/agentcore/knowledge"
)

// safeResolve and handleFileDownload security tests moved to the files pack
// (internal/packs/files) along with the migrated /api/files* surface.

// ── Test 7: checkWSOrigin rejects unknown origins ──

func TestCheckWSOriginRejectsUnknown(t *testing.T) {
	g := &Gateway{allowedOrigins: []string{"https://myapp.example.com"}}

	req := httptest.NewRequest(http.MethodGet, "/ws/test", nil)
	req.Header.Set("Origin", "https://evil.com")

	if g.checkWSOrigin(req) {
		t.Fatal("expected origin to be rejected")
	}
}

// ── Test 8: checkWSOrigin allows localhost ──

func TestCheckWSOriginAllowsLocalhost(t *testing.T) {
	g := &Gateway{allowedOrigins: []string{"https://myapp.example.com"}}

	req := httptest.NewRequest(http.MethodGet, "/ws/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")

	if !g.checkWSOrigin(req) {
		t.Fatal("expected localhost origin to be allowed")
	}
}

// ── Test 9: checkWSOrigin allows configured origin ──

func TestCheckWSOriginAllowsConfigured(t *testing.T) {
	g := &Gateway{allowedOrigins: []string{"https://myapp.example.com"}}

	req := httptest.NewRequest(http.MethodGet, "/ws/test", nil)
	req.Header.Set("Origin", "https://myapp.example.com")

	if !g.checkWSOrigin(req) {
		t.Fatal("expected configured origin to be allowed")
	}
}

// ── Test 11: shared SSRF guard rejects loopback / metadata URLs ──

func TestValidateSSRFTargetRejectsPrivateTargets(t *testing.T) {
	cases := []string{
		"http://127.0.0.1:8080/metadata",
		"http://localhost:8080/metadata",
		"http://169.254.169.254/latest/meta-data/",
		"http://10.0.0.5/internal",
	}
	for _, raw := range cases {
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatalf("parse %q: %v", raw, err)
		}
		if err := validateSSRFTarget(u); err == nil {
			t.Fatalf("expected %q to be rejected by SSRF guard", raw)
		}
	}
}

// ── Test 12: Tori bind URL uses the shared SSRF guard ──

func TestValidateToriURLRejectsMetadataTarget(t *testing.T) {
	if _, err := validateToriURL("http://169.254.169.254/latest/meta-data/"); err == nil {
		t.Fatal("expected metadata Tori URL to be rejected")
	}
}

// ── Test 13: knowledge repo import is rooted under explicit import roots ──

func TestResolveKBRepoPathRejectsPathOutsideRoots(t *testing.T) {
	t.Setenv("KB_IMPORT_ALLOW_ANY", "")
	root := t.TempDir()
	outside := t.TempDir()
	if _, err := knowledge.ResolveRepoPath(root, outside); err == nil {
		t.Fatal("expected repo import outside configured roots to be rejected")
	}
}

func TestResolveKBRepoPathAllowsPathInsideOutputDir(t *testing.T) {
	t.Setenv("KB_IMPORT_ALLOW_ANY", "")
	root := t.TempDir()
	inside := filepath.Join(root, "repo")
	if err := os.MkdirAll(inside, 0o755); err != nil {
		t.Fatal(err)
	}
	resolved, err := knowledge.ResolveRepoPath(root, inside)
	if err != nil {
		t.Fatalf("expected repo import inside output dir to be allowed: %v", err)
	}
	if resolved == "" {
		t.Fatal("expected resolved path")
	}
}
