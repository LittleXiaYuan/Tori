package general

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yunque-agent/pkg/skills"
)

// A directory NOT in the global read roots becomes readable when the
// conversation passes it as a workspace path, and is otherwise denied.
func TestFileSearchHonorsWorkspacePath(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "note.txt"), []byte("hello workspace"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Global roots deliberately exclude the workspace dir.
	skill := NewFileSearchSkill([]string{t.TempDir()})

	// Without workspace env: denied.
	out, err := skill.Execute(context.Background(), map[string]any{"action": "list", "path": workspace}, &skills.Environment{})
	if err == nil && !strings.Contains(out, "denied") && strings.Contains(out, "note.txt") {
		t.Fatalf("expected denial without workspace, got: %v / %s", err, out)
	}

	// With workspace env: allowed, lists the file.
	env := &skills.Environment{WorkspacePaths: []string{workspace}}
	out, err = skill.Execute(context.Background(), map[string]any{"action": "list", "path": workspace}, env)
	if err != nil {
		t.Fatalf("list with workspace failed: %v", err)
	}
	if !strings.Contains(out, "note.txt") {
		t.Fatalf("workspace dir listing missing file: %s", out)
	}

	// And can read a file inside it.
	out, err = skill.Execute(context.Background(), map[string]any{"action": "read", "path": filepath.Join(workspace, "note.txt")}, env)
	if err != nil {
		t.Fatalf("read with workspace failed: %v", err)
	}
	if !strings.Contains(out, "hello workspace") {
		t.Fatalf("unexpected file content: %s", out)
	}
}

func TestMergeWorkspacePathsValidation(t *testing.T) {
	real := t.TempDir()
	base := []string{"/global/root"}
	env := &skills.Environment{WorkspacePaths: []string{
		real,            // valid: kept
		"relative/path", // rejected: not absolute
		filepath.Join(real, "does-not-exist"), // rejected: missing
		"",              // rejected: empty
	}}
	got := mergeWorkspacePaths(base, env)
	if len(got) != 2 || got[0] != "/global/root" || got[1] != real {
		t.Fatalf("expected [global, %s], got %v", real, got)
	}
	// nil env returns base unchanged.
	if out := mergeWorkspacePaths(base, nil); len(out) != 1 {
		t.Fatalf("nil env should return base, got %v", out)
	}
}
