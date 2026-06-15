package filespack

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yunque-agent/pkg/packruntime"
)

type fakeGW struct{ dir string }

func (f fakeGW) OutputDir() string { return f.dir }

// TestFilesPackV2 verifies the files pack is a v2 Module with the expected
// route surface and that it degrades to 500 when the output dir is not
// configured (native handler, de-shelled from the gateway).
func TestFilesPackV2(t *testing.T) {
	var _ packruntime.Module = (*Handler)(nil)

	h := New(fakeGW{}) // empty output dir
	if h.PackID() != PackID {
		t.Fatalf("PackID = %q, want %q", h.PackID(), PackID)
	}
	if got := len(h.Routes()); got != 3 {
		t.Fatalf("Routes len = %d, want 3", got)
	}
	if err := h.Init(nil); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := h.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	rec := httptest.NewRecorder()
	h.handleList(rec, httptest.NewRequest(http.MethodGet, "/api/files", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("unconfigured handleList = %d, want 500", rec.Code)
	}

	if err := h.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}

// ── safeResolve rejects absolute paths ──

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

// ── safeResolve rejects traversal ──

func TestSafeResolveRejectsTraversal(t *testing.T) {
	base := t.TempDir()
	_, err := safeResolve(base, "../../etc/passwd")
	if err == nil {
		t.Fatal("expected error for path traversal, got nil")
	}
}

// ── safeResolve allows valid relative path ──

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

// ── safeResolve rejects symlink escape ──

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

// ── handleDownload rejects traversal ──

func TestFileDownloadRejectsTraversal(t *testing.T) {
	h := New(fakeGW{dir: t.TempDir()})

	req := httptest.NewRequest(http.MethodGet, "/api/files/download?path=../../etc/passwd", nil)
	w := httptest.NewRecorder()
	h.handleDownload(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// ── handleDownload rejects absolute path ──

func TestFileDownloadRejectsAbsolutePath(t *testing.T) {
	h := New(fakeGW{dir: t.TempDir()})

	req := httptest.NewRequest(http.MethodGet, "/api/files/download?path=/etc/passwd", nil)
	w := httptest.NewRecorder()
	h.handleDownload(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// ── handlePreview surfaces parse metadata for document-parser-needed files ──

func TestHandleFilePreviewIncludesParseMetadataForDocumentParserNeeded(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/申请表.pdf", []byte{0x25, 0x50, 0x44, 0x46}, 0o600); err != nil {
		t.Fatal(err)
	}
	h := New(fakeGW{dir: dir})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/files/preview?path=%E7%94%B3%E8%AF%B7%E8%A1%A8.pdf", nil)
	h.handlePreview(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if preview, _ := resp["preview"].(string); preview != "" {
		t.Fatalf("pdf preview should be empty without document parser, got %q", preview)
	}
	parse, ok := resp["parse"].(map[string]any)
	if !ok {
		t.Fatalf("expected parse metadata, got %#v", resp["parse"])
	}
	if parse["status"] != "needs_document_parser" {
		t.Fatalf("expected needs_document_parser, got %#v", parse["status"])
	}
	note, _ := parse["note"].(string)
	if !strings.Contains(note, "附件已添加") || !strings.Contains(note, "文档解析") {
		t.Fatalf("expected actionable parse note, got %q", note)
	}
}
