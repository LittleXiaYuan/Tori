package gateway

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"yunque-agent/internal/agentcore/llm"
)

func TestInferWorkspacePathsFromWindowsPathMention(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows absolute path extraction is only active on Windows-style paths")
	}
	dir := t.TempDir()

	got := inferWorkspacePathsFromMessages(nil, []llm.Message{{
		Role:    "user",
		Content: "hello云雀~请看这个代码" + dir + "，我需要你帮我完善项目",
	}})

	if len(got) != 1 {
		t.Fatalf("expected one inferred workspace path, got %v", got)
	}
	if got[0] != filepath.Clean(dir) {
		t.Fatalf("expected %q, got %q", filepath.Clean(dir), got[0])
	}
}

func TestInferWorkspacePathsFromWindowsFileMentionUsesParentDir(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows absolute path extraction is only active on Windows-style paths")
	}
	dir := t.TempDir()
	filePath := filepath.Join(dir, "Main.java")
	if err := os.WriteFile(filePath, []byte("class Main {}"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := inferWorkspacePathsFromMessages(nil, []llm.Message{{
		Role:    "user",
		Content: "请修改 " + filePath,
	}})

	if len(got) != 1 {
		t.Fatalf("expected one inferred workspace path, got %v", got)
	}
	if got[0] != filepath.Clean(dir) {
		t.Fatalf("expected %q, got %q", filepath.Clean(dir), got[0])
	}
}
