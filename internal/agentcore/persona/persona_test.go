package persona

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func tempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "persona-test-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func TestNewCreatesDefaults(t *testing.T) {
	dir := tempDir(t)
	p, err := New(dir)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if p.Identity() == "" {
		t.Fatal("identity should have default content")
	}
	if p.Soul() == "" {
		t.Fatal("soul should have default content")
	}
	// Files should exist
	if _, err := os.Stat(filepath.Join(dir, "IDENTITY.md")); err != nil {
		t.Fatalf("IDENTITY.md not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "SOUL.md")); err != nil {
		t.Fatalf("SOUL.md not created: %v", err)
	}
}

func TestSetIdentityAndSoul(t *testing.T) {
	dir := tempDir(t)
	p, _ := New(dir)

	p.SetIdentity("I am Test Bot")
	if p.Identity() != "I am Test Bot" {
		t.Fatalf("identity: got %q", p.Identity())
	}
	// Verify persistence
	data, _ := os.ReadFile(filepath.Join(dir, "IDENTITY.md"))
	if string(data) != "I am Test Bot" {
		t.Fatal("identity not persisted")
	}

	p.SetSoul("Be helpful")
	if p.Soul() != "Be helpful" {
		t.Fatalf("soul: got %q", p.Soul())
	}
}

func TestAddAndDeleteSkill(t *testing.T) {
	dir := tempDir(t)
	p, _ := New(dir)

	err := p.AddSkill("coding", "write code", "You can write Go code")
	if err != nil {
		t.Fatalf("add skill: %v", err)
	}
	skills := p.Skills()
	if len(skills) != 1 || skills[0].Name != "coding" {
		t.Fatalf("skills: %+v", skills)
	}

	// File should exist
	if _, err := os.Stat(filepath.Join(dir, "skills", "coding.md")); err != nil {
		t.Fatal("skill file not created")
	}

	p.DeleteSkill("coding")
	if len(p.Skills()) != 0 {
		t.Fatal("skill not deleted")
	}
}

func TestAddSkillEmptyName(t *testing.T) {
	dir := tempDir(t)
	p, _ := New(dir)
	if err := p.AddSkill("", "desc", "content"); err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestSetSkillEnabled(t *testing.T) {
	dir := tempDir(t)
	p, _ := New(dir)
	p.AddSkill("test", "test skill", "content")

	if !p.SetSkillEnabled("test", false) {
		t.Fatal("SetSkillEnabled returned false")
	}
	skills := p.Skills()
	if skills[0].Enabled {
		t.Fatal("skill should be disabled")
	}
	if p.SetSkillEnabled("nonexistent", true) {
		t.Fatal("should return false for nonexistent")
	}
}

func TestSystemPrompt(t *testing.T) {
	dir := tempDir(t)
	p, _ := New(dir)
	p.SetIdentity("I am Bot")
	p.SetSoul("Be nice")
	p.AddSkill("math", "do math", "Calculate things")

	prompt := p.SystemPrompt()
	if !strings.Contains(prompt, "I am Bot") {
		t.Fatal("prompt missing identity")
	}
	if !strings.Contains(prompt, "Be nice") {
		t.Fatal("prompt missing soul")
	}
	if !strings.Contains(prompt, "math") {
		t.Fatal("prompt missing skill")
	}
}

func TestSystemPromptDisabledSkill(t *testing.T) {
	dir := tempDir(t)
	p, _ := New(dir)
	p.AddSkill("hidden", "hidden skill", "secret content")
	p.SetSkillEnabled("hidden", false)

	prompt := p.SystemPrompt()
	if strings.Contains(prompt, "secret content") {
		t.Fatal("disabled skill should not appear in prompt")
	}
}

func TestReload(t *testing.T) {
	dir := tempDir(t)
	p, _ := New(dir)

	// Externally modify
	os.WriteFile(filepath.Join(dir, "IDENTITY.md"), []byte("External change"), 0o644)
	p.Reload()
	if p.Identity() != "External change" {
		t.Fatalf("reload identity: got %q", p.Identity())
	}
}

func TestRenameAndResetDefaults(t *testing.T) {
	dir := tempDir(t)
	p, _ := New(dir)

	if err := p.Rename("小云"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if !strings.Contains(p.Identity(), "小云") {
		t.Fatalf("identity should contain renamed value, got %q", p.Identity())
	}

	if err := p.ResetToDefaults(); err != nil {
		t.Fatalf("reset defaults: %v", err)
	}
	if !strings.Contains(p.Identity(), "Yunque Agent") {
		t.Fatalf("expected default identity after reset, got %q", p.Identity())
	}
}

func TestParseSkillFile(t *testing.T) {
	raw := "---\nname: test-skill\ndescription: A test\nenabled: false\n---\nDo something"
	s := parseSkillFile(raw, "fallback")
	if s.Name != "test-skill" {
		t.Fatalf("name: got %s", s.Name)
	}
	if s.Description != "A test" {
		t.Fatalf("desc: got %s", s.Description)
	}
	if s.Enabled {
		t.Fatal("should be disabled")
	}
	if s.Content != "Do something" {
		t.Fatalf("content: got %q", s.Content)
	}
}

func TestParseSkillFileNoFrontmatter(t *testing.T) {
	s := parseSkillFile("Just plain text", "myname")
	if s.Name != "myname" {
		t.Fatalf("name: got %s", s.Name)
	}
	if s.Content != "Just plain text" {
		t.Fatalf("content: got %q", s.Content)
	}
}
