package knowledge

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveRepoPathAllowsPathInsideKBImportRoots(t *testing.T) {
	t.Setenv("KB_IMPORT_ALLOW_ANY", "")
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KB_IMPORT_ROOTS", root)

	resolved, err := ResolveRepoPath("", repo)
	if err != nil {
		t.Fatalf("expected repo import inside KB_IMPORT_ROOTS to be allowed: %v", err)
	}
	want, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatal(err)
	}
	if resolved != want {
		t.Fatalf("resolved = %q, want %q", resolved, want)
	}
}
