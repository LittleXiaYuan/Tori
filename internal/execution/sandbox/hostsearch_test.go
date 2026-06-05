package sandbox

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSearchHostFilesSkipsNoiseDirs verifies filename search prunes high-volume
// noise directories (node_modules/.git/…) so a rare-match query can't walk the
// whole tree, while still finding real files.
func TestSearchHostFilesSkipsNoiseDirs(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "src"))
	mustMkdir(t, filepath.Join(root, "node_modules", "pkg"))
	mustMkdir(t, filepath.Join(root, ".git"))
	mustWrite(t, filepath.Join(root, "src", "deck_create.go"))
	mustWrite(t, filepath.Join(root, "node_modules", "pkg", "deck_create_dep.js"))
	mustWrite(t, filepath.Join(root, ".git", "deck_create_obj"))

	policy := DefaultPolicy()
	policy.HostReadPaths = []string{root}
	sb, err := New(os.TempDir(), policy)
	if err != nil {
		t.Fatal(err)
	}
	defer sb.Cleanup()

	matches, err := sb.SearchHostFiles(root, "deck_create")
	if err != nil {
		t.Fatalf("SearchHostFiles: %v", err)
	}
	joined := strings.Join(matches, "|")
	if !strings.Contains(joined, "deck_create.go") {
		t.Fatalf("expected to find src/deck_create.go, got %v", matches)
	}
	for _, m := range matches {
		s := filepath.ToSlash(m)
		if strings.Contains(s, "node_modules/") || strings.Contains(s, ".git/") {
			t.Fatalf("skip-list failed: noise dir leaked into results: %v", matches)
		}
	}
	t.Logf("ok: %d match(es), noise dirs pruned: %v", len(matches), matches)
}

func mustMkdir(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, p string) {
	t.Helper()
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}
