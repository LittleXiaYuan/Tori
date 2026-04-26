package cogni

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDeclaration_FillsIDFromFilename(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "code-reviewer.json")
	if err := os.WriteFile(path, []byte(`{"display_name":"Reviewer","activation":{"keywords":["review"]}}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	d, err := LoadDeclaration(path)
	if err != nil {
		t.Fatalf("LoadDeclaration: %v", err)
	}
	if d.ID != "code-reviewer" {
		t.Fatalf("expected id derived from filename, got %q", d.ID)
	}
	if d.DisplayName != "Reviewer" {
		t.Fatalf("expected DisplayName preserved, got %q", d.DisplayName)
	}
}

func TestLoadDeclaration_DropsCogniSuffix(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "router.cogni.json")
	if err := os.WriteFile(path, []byte(`{"activation":{"always_on":true}}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	d, err := LoadDeclaration(path)
	if err != nil {
		t.Fatalf("LoadDeclaration: %v", err)
	}
	if d.ID != "router" {
		t.Fatalf("expected id=router after stripping .cogni.json, got %q", d.ID)
	}
}

func TestLoadDeclaration_MissingFile(t *testing.T) {
	_, err := LoadDeclaration(filepath.Join(t.TempDir(), "nope.json"))
	if err == nil {
		t.Fatalf("expected error on missing file")
	}
}

func TestLoadDeclaration_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := LoadDeclaration(path)
	if err == nil || !strings.Contains(err.Error(), "parse") {
		t.Fatalf("expected parse error, got %v", err)
	}
}

func TestLoadDeclaration_FailsValidation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	// invalid regex must fail Validate even after we autopopulate the ID
	if err := os.WriteFile(path, []byte(`{"activation":{"regex":["[unclosed"]}}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := LoadDeclaration(path)
	if err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestLoadDeclarationsFromDir_MissingDirIsNotError(t *testing.T) {
	decls, errs, err := LoadDeclarationsFromDir(filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatalf("missing dir should not error, got %v", err)
	}
	if len(decls) != 0 || len(errs) != 0 {
		t.Fatalf("expected empty result, got %v / %v", decls, errs)
	}
}

func TestLoadDeclarationsFromDir_AggregatesErrors(t *testing.T) {
	dir := t.TempDir()
	mustWrite := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	mustWrite("good.json", `{"id":"good","activation":{"keywords":["ok"]}}`)
	mustWrite("bad.json", `not json`)
	mustWrite("ignored.txt", `should be skipped`)

	decls, errs, err := LoadDeclarationsFromDir(dir)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(decls) != 1 || decls[0].ID != "good" {
		t.Fatalf("expected one good decl, got %+v", decls)
	}
	if len(errs) != 1 || !strings.Contains(errs[0].Path, "bad.json") {
		t.Fatalf("expected one error referencing bad.json, got %+v", errs)
	}
}

// ── YAML tests ──

func TestLoadDeclaration_YAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "code-reviewer.yaml")
	yamlContent := `
display_name: Code Reviewer
description: Intelligent code review
activation:
  keywords:
    - review
    - PR
  min_score: 0.3
context:
  static: You are a code reviewer.
surface:
  exclude:
    - shell_exec
`
	if err := os.WriteFile(path, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	d, err := LoadDeclaration(path)
	if err != nil {
		t.Fatalf("LoadDeclaration YAML: %v", err)
	}
	if d.ID != "code-reviewer" {
		t.Errorf("ID = %q, want code-reviewer", d.ID)
	}
	if d.DisplayName != "Code Reviewer" {
		t.Errorf("DisplayName = %q", d.DisplayName)
	}
	if len(d.Activation.Keywords) != 2 {
		t.Errorf("Keywords = %v, want 2", d.Activation.Keywords)
	}
	if d.Activation.MinScore != 0.3 {
		t.Errorf("MinScore = %f, want 0.3", d.Activation.MinScore)
	}
	if d.Context.Static != "You are a code reviewer." {
		t.Errorf("Static = %q", d.Context.Static)
	}
	if len(d.Surface.Exclude) != 1 || d.Surface.Exclude[0] != "shell_exec" {
		t.Errorf("Exclude = %v", d.Surface.Exclude)
	}
}

func TestLoadDeclaration_YML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "my-agent.yml")
	if err := os.WriteFile(path, []byte(`id: my-agent
activation:
  always_on: true
`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	d, err := LoadDeclaration(path)
	if err != nil {
		t.Fatalf("LoadDeclaration YML: %v", err)
	}
	if d.ID != "my-agent" {
		t.Errorf("ID = %q", d.ID)
	}
	if !d.Activation.AlwaysOn {
		t.Error("AlwaysOn should be true")
	}
}

func TestLoadDeclaration_CogniYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "router.cogni.yaml")
	if err := os.WriteFile(path, []byte(`activation:
  always_on: true
`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	d, err := LoadDeclaration(path)
	if err != nil {
		t.Fatalf("LoadDeclaration cogni.yaml: %v", err)
	}
	if d.ID != "router" {
		t.Errorf("ID = %q, want router", d.ID)
	}
}

func TestLoadDeclaration_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(path, []byte(":\n  :\n    - [invalid yaml"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := LoadDeclaration(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoadDeclarationsFromDir_MixedFormats(t *testing.T) {
	dir := t.TempDir()
	mustWrite := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	mustWrite("a.json", `{"id":"a","activation":{"keywords":["hello"]}}`)
	mustWrite("b.yaml", "id: b\nactivation:\n  keywords:\n    - world\n")
	mustWrite("c.yml", "id: c\nactivation:\n  always_on: true\n")
	mustWrite("ignore.txt", "should be skipped")

	decls, errs, err := LoadDeclarationsFromDir(dir)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(decls) != 3 {
		t.Fatalf("expected 3 declarations, got %d", len(decls))
	}

	ids := map[string]bool{}
	for _, d := range decls {
		ids[d.ID] = true
	}
	for _, want := range []string{"a", "b", "c"} {
		if !ids[want] {
			t.Errorf("missing declaration %q", want)
		}
	}
}

func TestDetectFormat(t *testing.T) {
	tests := []struct {
		path, want string
	}{
		{"foo.json", "json"},
		{"foo.yaml", "yaml"},
		{"foo.yml", "yaml"},
		{"foo.cogni.json", "json"},
		{"foo.cogni.yaml", "yaml"},
		{"foo.cogni.yml", "yaml"},
		{"foo.txt", "json"},
	}
	for _, tt := range tests {
		if got := detectFormat(tt.path); got != tt.want {
			t.Errorf("detectFormat(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestIsCogniFile(t *testing.T) {
	yes := []string{"a.json", "b.yaml", "c.yml", "d.cogni.json", "e.cogni.yaml"}
	no := []string{"a.txt", "b.go", "c.md"}

	for _, f := range yes {
		if !isCogniFile(f) {
			t.Errorf("isCogniFile(%q) = false, want true", f)
		}
	}
	for _, f := range no {
		if isCogniFile(f) {
			t.Errorf("isCogniFile(%q) = true, want false", f)
		}
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.json")
	in := &Declaration{
		ID:          "x",
		DisplayName: "X",
		Activation: ActivationRules{
			Keywords: []string{"hello"},
			MinScore: 0.3,
		},
	}
	if err := SaveDeclaration(in, path); err != nil {
		t.Fatalf("save: %v", err)
	}
	out, err := LoadDeclaration(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if out.ID != in.ID || out.DisplayName != in.DisplayName ||
		len(out.Activation.Keywords) != 1 || out.Activation.Keywords[0] != "hello" {
		t.Fatalf("round trip mismatch: in=%+v out=%+v", in, out)
	}
}
