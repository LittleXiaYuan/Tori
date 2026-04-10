package gateway

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// ── Test 1: safeResolve rejects absolute paths ──

func TestSafeResolveRejectsAbsolutePath(t *testing.T) {
	base := t.TempDir()
	absPath := filepath.Join(base, "..", "escape")
	if !filepath.IsAbs(absPath) {
		absPath = `C:\Windows\System32\config`
	}
	_, err := safeResolve(base, absPath)
	if err == nil {
		t.Fatal("expected error for absolute path, got nil")
	}
}

// ── Test 2: safeResolve rejects traversal ──

func TestSafeResolveRejectsTraversal(t *testing.T) {
	base := t.TempDir()
	_, err := safeResolve(base, "../../etc/passwd")
	if err == nil {
		t.Fatal("expected error for path traversal, got nil")
	}
}

// ── Test 3: safeResolve allows valid relative path ──

func TestSafeResolveAllowsValidRelative(t *testing.T) {
	base := t.TempDir()
	sub := filepath.Join(base, "subdir")
	os.MkdirAll(sub, 0o755)

	result, err := safeResolve(base, "subdir")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if result == "" {
		t.Fatal("expected non-empty path")
	}
}

// ── Test 4: safeResolve rejects symlink escape ──

func TestSafeResolveRejectsSymlinkEscape(t *testing.T) {
	base := t.TempDir()
	outside := t.TempDir()

	outsideFile := filepath.Join(outside, "secret.txt")
	os.WriteFile(outsideFile, []byte("secret"), 0o644)

	link := filepath.Join(base, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Skip("symlinks not supported on this OS/filesystem")
	}

	_, err := safeResolve(base, "escape/secret.txt")
	if err == nil {
		t.Fatal("expected error for symlink escape, got nil")
	}
}

// ── Test 5: handleFileDownload rejects traversal ──

func TestFileDownloadRejectsTraversal(t *testing.T) {
	g := &Gateway{outputDir: t.TempDir()}

	req := httptest.NewRequest(http.MethodGet, "/api/files/download?path=../../etc/passwd", nil)
	w := httptest.NewRecorder()
	g.handleFileDownload(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// ── Test 6: handleFileDownload rejects absolute path ──

func TestFileDownloadRejectsAbsolutePath(t *testing.T) {
	g := &Gateway{outputDir: t.TempDir()}

	req := httptest.NewRequest(http.MethodGet, "/api/files/download?path=/etc/passwd", nil)
	w := httptest.NewRecorder()
	g.handleFileDownload(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

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

// ── Test 10: ENABLE_TOOLS_EXEC disabled returns 403 ──

func TestToolExecDisabledByDefault(t *testing.T) {
	os.Unsetenv("ENABLE_TOOLS_EXEC")

	g := &Gateway{}
	req := httptest.NewRequest(http.MethodPost, "/v1/tools/exec", nil)
	w := httptest.NewRecorder()
	g.handleToolExec(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when ENABLE_TOOLS_EXEC not set, got %d", w.Code)
	}
}
