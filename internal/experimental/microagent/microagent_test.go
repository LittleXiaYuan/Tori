package microagent

import (
	"os"
	"path/filepath"
	"testing"
)

func tmpDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "tori-microagent-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func TestRegisterAndGet(t *testing.T) {
	r := NewRegistry()
	r.Register(&MicroAgent{Name: "go-expert", Content: "You are a Go expert", Enabled: true})
	ma, ok := r.Get("go-expert")
	if !ok || ma.Content != "You are a Go expert" {
		t.Fatal("not found")
	}
}

func TestRemove(t *testing.T) {
	r := NewRegistry()
	r.Register(&MicroAgent{Name: "temp", Enabled: true})
	if !r.Remove("temp") {
		t.Fatal("should remove")
	}
	if r.Remove("temp") {
		t.Fatal("already removed")
	}
}

func TestAll(t *testing.T) {
	r := NewRegistry()
	r.Register(&MicroAgent{Name: "a", Enabled: true})
	r.Register(&MicroAgent{Name: "b", Enabled: true})
	if len(r.All()) != 2 {
		t.Fatal("expected 2")
	}
}

func TestResolveAlwaysActive(t *testing.T) {
	r := NewRegistry()
	r.Register(&MicroAgent{Name: "base", Content: "base rules", Enabled: true, Trigger: ""})
	matched := r.Resolve("any message")
	if len(matched) != 1 {
		t.Fatalf("expected 1, got %d", len(matched))
	}
}

func TestResolveByTrigger(t *testing.T) {
	r := NewRegistry()
	r.Register(&MicroAgent{Name: "docker", Content: "Docker tips", Enabled: true, Trigger: "docker"})
	r.Register(&MicroAgent{Name: "git", Content: "Git tips", Enabled: true, Trigger: "git"})

	matched := r.Resolve("help me with docker")
	if len(matched) != 1 || matched[0].Name != "docker" {
		t.Fatal("should match docker")
	}

	matched = r.Resolve("hello world")
	if len(matched) != 0 {
		t.Fatal("should match nothing")
	}
}

func TestResolveDisabled(t *testing.T) {
	r := NewRegistry()
	r.Register(&MicroAgent{Name: "off", Content: "disabled", Enabled: false, Trigger: ""})
	if len(r.Resolve("test")) != 0 {
		t.Fatal("disabled should not match")
	}
}

func TestResolvePriority(t *testing.T) {
	r := NewRegistry()
	r.Register(&MicroAgent{Name: "low", Content: "low", Enabled: true, Priority: 1})
	r.Register(&MicroAgent{Name: "high", Content: "high", Enabled: true, Priority: 10})
	matched := r.Resolve("test")
	if matched[0].Name != "high" {
		t.Fatal("high priority should be first")
	}
}

func TestResolveByScope(t *testing.T) {
	r := NewRegistry()
	r.Register(&MicroAgent{Name: "g", Scope: ScopeGlobal, Enabled: true})
	r.Register(&MicroAgent{Name: "r", Scope: ScopeRepo, Enabled: true})
	global := r.ResolveByScope(ScopeGlobal)
	if len(global) != 1 || global[0].Name != "g" {
		t.Fatal("wrong scope filter")
	}
}

func TestCompilePrompt(t *testing.T) {
	r := NewRegistry()
	r.Register(&MicroAgent{Name: "rules", Content: "Always be concise", Enabled: true})
	prompt := r.CompilePrompt("test")
	if prompt == "" {
		t.Fatal("empty prompt")
	}
	if !contains(prompt, "Always be concise") {
		t.Fatal("missing content")
	}
}

func TestCompilePromptEmpty(t *testing.T) {
	r := NewRegistry()
	if r.CompilePrompt("test") != "" {
		t.Fatal("should be empty")
	}
}

func TestLoadFromDirectory(t *testing.T) {
	dir := tmpDir(t)

	os.WriteFile(filepath.Join(dir, "coding.md"), []byte(`---
name: coding-expert
description: Coding guidance
trigger: code
---

When working with code, follow best practices.
`), 0o644)

	os.WriteFile(filepath.Join(dir, "general.md"), []byte("Always be helpful and accurate."), 0o644)

	r := NewRegistry()
	loaded, err := LoadFromDirectory(dir, ScopeRepo, r)
	if err != nil {
		t.Fatal(err)
	}
	if loaded != 2 {
		t.Fatalf("expected 2 loaded, got %d", loaded)
	}

	ma, ok := r.Get("coding-expert")
	if !ok {
		t.Fatal("coding-expert not found")
	}
	if ma.Trigger != "code" {
		t.Fatal("trigger not parsed")
	}
	if ma.Scope != ScopeRepo {
		t.Fatal("wrong scope")
	}
}

func TestLoadFromDirectoryNotExist(t *testing.T) {
	r := NewRegistry()
	loaded, err := LoadFromDirectory("/nonexistent", ScopeGlobal, r)
	if err != nil {
		t.Fatal(err)
	}
	if loaded != 0 {
		t.Fatal("expected 0")
	}
}

func TestParseMicroAgentMDNoFrontmatter(t *testing.T) {
	ma := parseMicroAgentMD("simple", "Just plain content.", ScopeGlobal)
	if ma.Name != "simple" {
		t.Fatal("wrong name")
	}
	if ma.Content != "Just plain content." {
		t.Fatal("wrong content")
	}
}

func TestParseMicroAgentMDDisabled(t *testing.T) {
	ma := parseMicroAgentMD("off", "---\nenabled: false\n---\ncontent", ScopeGlobal)
	if ma.Enabled {
		t.Fatal("should be disabled")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
