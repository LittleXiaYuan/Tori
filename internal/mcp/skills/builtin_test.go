package skills

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestListTools(t *testing.T) {
	b := NewBuiltin(t.TempDir())
	tools, err := b.ListTools(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 5 {
		t.Fatalf("expected 5 tools, got %d", len(tools))
	}
	names := map[string]bool{}
	for _, tool := range tools {
		names[tool.Name] = true
	}
	for _, want := range []string{"web_fetch", "code_exec", "file_read", "file_write", "file_list"} {
		if !names[want] {
			t.Fatalf("missing tool: %s", want)
		}
	}
}

func TestFileWriteRead(t *testing.T) {
	dir := t.TempDir()
	b := NewBuiltin(dir)
	ctx := context.Background()

	// Write
	res, err := b.CallTool(ctx, "file_write", map[string]any{"path": "test.txt", "content": "hello world"})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatal("write failed:", res.Content[0].Text)
	}

	// Read
	res, err = b.CallTool(ctx, "file_read", map[string]any{"path": "test.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Content[0].Text != "hello world" {
		t.Fatalf("expected 'hello world', got %q", res.Content[0].Text)
	}
}

func TestFileList(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("bb"), 0644)
	os.MkdirAll(filepath.Join(dir, "sub"), 0755)

	b := NewBuiltin(dir)
	res, err := b.CallTool(context.Background(), "file_list", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	text := res.Content[0].Text
	if len(text) == 0 {
		t.Fatal("empty listing")
	}
}

func TestPathEscapePrevention(t *testing.T) {
	b := NewBuiltin(t.TempDir())
	ctx := context.Background()

	res, _ := b.CallTool(ctx, "file_read", map[string]any{"path": "../../etc/passwd"})
	if !res.IsError {
		t.Fatal("expected path escape error")
	}
}

func TestPathEscapePreventionSiblingPrefix(t *testing.T) {
	dir := t.TempDir()
	b := NewBuiltin(dir)
	ctx := context.Background()

	res, _ := b.CallTool(ctx, "file_write", map[string]any{
		"path":    "../" + filepath.Base(dir) + "_escape/evil.txt",
		"content": "bad",
	})
	if !res.IsError {
		t.Fatal("expected sibling prefix escape error")
	}
}

func TestUnknownTool(t *testing.T) {
	b := NewBuiltin(t.TempDir())
	_, err := b.CallTool(context.Background(), "nonexistent", nil)
	if err == nil {
		t.Fatal("expected ErrToolNotFound")
	}
}
