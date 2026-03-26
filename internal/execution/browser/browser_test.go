package browser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewAndClose(t *testing.T) {
	if os.Getenv("BROWSER_TEST") == "" {
		t.Skip("set BROWSER_TEST=1 to run browser tests (requires Chromium)")
	}

	dir := t.TempDir()
	e, err := New(Config{
		Headless: true,
		DataDir:  dir,
	})
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer e.Close()
}

func TestNavigateAndScreenshot(t *testing.T) {
	if os.Getenv("BROWSER_TEST") == "" {
		t.Skip("set BROWSER_TEST=1 to run browser tests (requires Chromium)")
	}

	dir := t.TempDir()
	e, err := New(Config{
		Headless: true,
		DataDir:  dir,
	})
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer e.Close()

	result, err := e.Navigate("https://example.com")
	if err != nil {
		t.Fatalf("Navigate() failed: %v", err)
	}
	if result.Title == "" {
		t.Error("expected non-empty title")
	}
	t.Logf("Title: %s", result.Title)
	t.Logf("URL: %s", result.URL)
	t.Logf("Text preview: %.200s", result.Text)

	ssPath := filepath.Join(dir, "test.png")
	if err := e.Screenshot(ssPath); err != nil {
		t.Fatalf("Screenshot() failed: %v", err)
	}
	info, _ := os.Stat(ssPath)
	if info == nil || info.Size() == 0 {
		t.Error("screenshot file is empty")
	}
	t.Logf("Screenshot: %s (%d bytes)", ssPath, info.Size())
}

func TestReadText(t *testing.T) {
	if os.Getenv("BROWSER_TEST") == "" {
		t.Skip("set BROWSER_TEST=1 to run browser tests (requires Chromium)")
	}

	dir := t.TempDir()
	e, err := New(Config{Headless: true, DataDir: dir})
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer e.Close()

	e.Navigate("https://example.com")

	text, err := e.ReadText("")
	if err != nil {
		t.Fatalf("ReadText() failed: %v", err)
	}
	if text == "" {
		t.Error("expected non-empty text")
	}

	h1, err := e.ReadText("h1")
	if err != nil {
		t.Fatalf("ReadText(h1) failed: %v", err)
	}
	t.Logf("h1: %s", h1)
}

func TestEval(t *testing.T) {
	if os.Getenv("BROWSER_TEST") == "" {
		t.Skip("set BROWSER_TEST=1 to run browser tests (requires Chromium)")
	}

	dir := t.TempDir()
	e, err := New(Config{Headless: true, DataDir: dir})
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer e.Close()

	e.Navigate("https://example.com")

	result, err := e.Eval(`() => document.title`)
	if err != nil {
		t.Fatalf("Eval() failed: %v", err)
	}
	t.Logf("eval result: %s", result)
}
