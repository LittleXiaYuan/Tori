package general

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yunque-agent/pkg/skills"
)

func TestFileIndexFindAndSkip(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "node_modules", "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(p string) {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(root, "src", "report_2026.xlsx"))
	write(filepath.Join(root, "node_modules", "pkg", "report_dep.js")) // must be pruned

	ix := newFileIndex()
	ix.ensure([]string{root})

	matches := ix.find("report", 50)
	if len(matches) != 1 {
		t.Fatalf("expected 1 match (node_modules pruned), got %v", matches)
	}
	if !strings.Contains(matches[0], "report_2026.xlsx") {
		t.Fatalf("wrong match: %v", matches)
	}
	// cached second query still works (instant path)
	if got := ix.find("2026", 50); len(got) != 1 {
		t.Fatalf("cached find failed: %v", got)
	}
	if n, _ := ix.stats(); n != 1 {
		t.Fatalf("index size = %d, want 1 (node_modules pruned)", n)
	}
}

func TestFileFindSkill(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "quarterly_plan.docx"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	skill := NewFileFindSkill([]string{root})
	out, err := skill.Execute(context.Background(), map[string]any{"query": "quarterly"}, &skills.Environment{})
	if err != nil {
		t.Fatal(err)
	}
	var resp struct {
		Matches []string `json:"matches"`
		Count   int      `json:"count"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("bad json: %v (%s)", err, out)
	}
	if resp.Count != 1 || !strings.Contains(resp.Matches[0], "quarterly_plan.docx") {
		t.Fatalf("file_find returned %s", out)
	}
}
