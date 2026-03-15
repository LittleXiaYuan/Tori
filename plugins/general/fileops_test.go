package general

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yunque-agent/pkg/skills"
)

func TestFileWriteCreate(t *testing.T) {
	dir := t.TempDir()
	sk := NewFileWriteSkill([]string{dir})

	result, err := sk.Execute(context.Background(), map[string]any{
		"path":    filepath.Join(dir, "test.txt"),
		"content": "hello world",
	}, &skills.Environment{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "已写入") {
		t.Fatalf("unexpected result: %s", result)
	}

	data, err := os.ReadFile(filepath.Join(dir, "test.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello world" {
		t.Fatalf("expected 'hello world', got %q", string(data))
	}
}

func TestFileWriteNoOverwrite(t *testing.T) {
	dir := t.TempDir()
	sk := NewFileWriteSkill([]string{dir})

	path := filepath.Join(dir, "existing.txt")
	os.WriteFile(path, []byte("original"), 0o644)

	_, err := sk.Execute(context.Background(), map[string]any{
		"path":    path,
		"content": "new content",
	}, &skills.Environment{})
	if err == nil {
		t.Fatal("expected error for existing file in create mode")
	}
}

func TestFileWriteOverwrite(t *testing.T) {
	dir := t.TempDir()
	sk := NewFileWriteSkill([]string{dir})

	path := filepath.Join(dir, "overwrite.txt")
	os.WriteFile(path, []byte("original"), 0o644)

	_, err := sk.Execute(context.Background(), map[string]any{
		"path":    path,
		"content": "new content",
		"mode":    "overwrite",
	}, &skills.Environment{})
	if err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(path)
	if string(data) != "new content" {
		t.Fatalf("expected 'new content', got %q", string(data))
	}
}

func TestFileWriteAppend(t *testing.T) {
	dir := t.TempDir()
	sk := NewFileWriteSkill([]string{dir})

	path := filepath.Join(dir, "append.txt")
	os.WriteFile(path, []byte("line1\n"), 0o644)

	_, err := sk.Execute(context.Background(), map[string]any{
		"path":    path,
		"content": "line2\n",
		"mode":    "append",
	}, &skills.Environment{})
	if err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(path)
	if string(data) != "line1\nline2\n" {
		t.Fatalf("expected 'line1\\nline2\\n', got %q", string(data))
	}
}

func TestFileWriteAccessDenied(t *testing.T) {
	sk := NewFileWriteSkill([]string{"/allowed/only"})

	_, err := sk.Execute(context.Background(), map[string]any{
		"path":    "/tmp/evil.txt",
		"content": "bad",
	}, &skills.Environment{})
	if err == nil {
		t.Fatal("expected access denied")
	}
	if !strings.Contains(err.Error(), "access denied") {
		t.Fatalf("expected access denied error, got: %v", err)
	}
}

func TestFileWriteSubdir(t *testing.T) {
	dir := t.TempDir()
	sk := NewFileWriteSkill([]string{dir})

	subPath := filepath.Join(dir, "sub", "deep", "file.txt")
	_, err := sk.Execute(context.Background(), map[string]any{
		"path":    subPath,
		"content": "nested",
	}, &skills.Environment{})
	if err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(subPath)
	if string(data) != "nested" {
		t.Fatalf("expected 'nested', got %q", string(data))
	}
}

func TestZipPackAndUnpack(t *testing.T) {
	dir := t.TempDir()

	// Create source files
	srcDir := filepath.Join(dir, "src")
	os.MkdirAll(filepath.Join(srcDir, "sub"), 0o755)
	os.WriteFile(filepath.Join(srcDir, "a.txt"), []byte("aaa"), 0o644)
	os.WriteFile(filepath.Join(srcDir, "sub", "b.txt"), []byte("bbb"), 0o644)

	// Pack
	packSkill := NewZipPackSkill([]string{dir}, []string{dir})
	zipPath := filepath.Join(dir, "out.zip")
	result, err := packSkill.Execute(context.Background(), map[string]any{
		"sources": srcDir,
		"output":  zipPath,
	}, &skills.Environment{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "已创建") {
		t.Fatalf("unexpected pack result: %s", result)
	}

	// Verify zip exists
	if _, err := os.Stat(zipPath); err != nil {
		t.Fatal("zip file not created")
	}

	// Verify zip contents
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if len(r.File) < 2 {
		t.Fatalf("expected at least 2 files in zip, got %d", len(r.File))
	}

	// Unpack
	unpackSkill := NewZipUnpackSkill([]string{dir}, []string{dir})
	outDir := filepath.Join(dir, "extracted")
	result, err = unpackSkill.Execute(context.Background(), map[string]any{
		"zip_path":   zipPath,
		"output_dir": outDir,
	}, &skills.Environment{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "已解压") {
		t.Fatalf("unexpected unpack result: %s", result)
	}

	// Verify extracted files
	data, err := os.ReadFile(filepath.Join(outDir, "src", "a.txt"))
	if err != nil {
		t.Fatalf("extracted file not found: %v", err)
	}
	if string(data) != "aaa" {
		t.Fatalf("expected 'aaa', got %q", string(data))
	}
}

func TestZipAccessDenied(t *testing.T) {
	packSkill := NewZipPackSkill([]string{"/allowed"}, []string{"/allowed"})

	_, err := packSkill.Execute(context.Background(), map[string]any{
		"sources": "/secret/data",
		"output":  "/allowed/out.zip",
	}, &skills.Environment{})
	if err == nil {
		t.Fatal("expected access denied for sources")
	}
}

func TestZipUnpackSlipPrevention(t *testing.T) {
	dir := t.TempDir()

	// Create a malicious zip with path traversal
	zipPath := filepath.Join(dir, "evil.zip")
	zf, _ := os.Create(zipPath)
	zw := zip.NewWriter(zf)
	w, _ := zw.Create("../../etc/passwd")
	w.Write([]byte("evil"))
	zw.Close()
	zf.Close()

	sk := NewZipUnpackSkill([]string{dir}, []string{dir})
	_, err := sk.Execute(context.Background(), map[string]any{
		"zip_path":   zipPath,
		"output_dir": filepath.Join(dir, "out"),
	}, &skills.Environment{})
	if err == nil {
		t.Fatal("expected zip slip detection")
	}
	if !strings.Contains(err.Error(), "zip slip") {
		t.Fatalf("expected zip slip error, got: %v", err)
	}
}
