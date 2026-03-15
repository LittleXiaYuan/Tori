package i18n

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBundleSetAndT(t *testing.T) {
	b := NewBundle(LocaleZH)
	b.Set(LocaleZH, "hello", "你好 %s")
	b.Set(LocaleEN, "hello", "Hello %s")

	if got := b.T(LocaleZH, "hello", "世界"); got != "你好 世界" {
		t.Fatalf("expected '你好 世界', got %q", got)
	}
	if got := b.T(LocaleEN, "hello", "World"); got != "Hello World" {
		t.Fatalf("expected 'Hello World', got %q", got)
	}
}

func TestBundleFallback(t *testing.T) {
	b := NewBundle(LocaleZH)
	b.Set(LocaleZH, "only_zh", "仅中文")

	// Request English, should fall back to Chinese
	if got := b.T(LocaleEN, "only_zh"); got != "仅中文" {
		t.Fatalf("expected fallback to zh, got %q", got)
	}
}

func TestBundleKeyFallback(t *testing.T) {
	b := NewBundle(LocaleZH)

	// Unknown key returns key itself
	if got := b.T(LocaleZH, "unknown.key"); got != "unknown.key" {
		t.Fatalf("expected key as fallback, got %q", got)
	}
}

func TestBundleLoadJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "zh.json")
	os.WriteFile(path, []byte(`{"test.msg": "测试消息"}`), 0o644)

	b := NewBundle(LocaleZH)
	if err := b.LoadJSON(LocaleZH, path); err != nil {
		t.Fatalf("load json: %v", err)
	}
	if got := b.T(LocaleZH, "test.msg"); got != "测试消息" {
		t.Fatalf("expected '测试消息', got %q", got)
	}
}

func TestBundleLoadDir(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "zh.json"), []byte(`{"greet": "你好"}`), 0o644)
	os.WriteFile(filepath.Join(dir, "en.json"), []byte(`{"greet": "Hello"}`), 0o644)

	b := NewBundle(LocaleZH)
	if err := b.LoadDir(dir); err != nil {
		t.Fatalf("load dir: %v", err)
	}

	if got := b.T(LocaleZH, "greet"); got != "你好" {
		t.Fatalf("expected '你好', got %q", got)
	}
	if got := b.T(LocaleEN, "greet"); got != "Hello" {
		t.Fatalf("expected 'Hello', got %q", got)
	}
}

func TestBundleHasLocale(t *testing.T) {
	b := NewBundle(LocaleZH)
	b.Set(LocaleZH, "a", "b")

	if !b.HasLocale(LocaleZH) {
		t.Fatal("expected zh locale")
	}
	if b.HasLocale("fr") {
		t.Fatal("expected no fr locale")
	}
}

func TestBundleLocales(t *testing.T) {
	b := NewBundle(LocaleZH)
	b.Set(LocaleZH, "a", "b")
	b.Set(LocaleEN, "a", "b")

	locales := b.Locales()
	if len(locales) != 2 {
		t.Fatalf("expected 2 locales, got %d", len(locales))
	}
}

func TestBundleKeys(t *testing.T) {
	b := NewBundle(LocaleZH)
	b.Set(LocaleZH, "key1", "v1")
	b.Set(LocaleZH, "key2", "v2")

	keys := b.Keys(LocaleZH)
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(keys))
	}
}

func TestBundleCount(t *testing.T) {
	b := NewBundle(LocaleZH)
	b.Set(LocaleZH, "a", "1")
	b.Set(LocaleZH, "b", "2")

	if b.Count(LocaleZH) != 2 {
		t.Fatalf("expected 2, got %d", b.Count(LocaleZH))
	}
	if b.Count("fr") != 0 {
		t.Fatalf("expected 0 for unknown locale")
	}
}

func TestDefaultBundle(t *testing.T) {
	d := Default()
	if d == nil {
		t.Fatal("expected non-nil default bundle")
	}

	// Test built-in messages
	zh := T(LocaleZH, "agent.greeting")
	if zh == "agent.greeting" {
		t.Fatal("expected translated greeting, got key")
	}

	en := T(LocaleEN, "agent.greeting")
	if en == "agent.greeting" {
		t.Fatal("expected translated greeting, got key")
	}

	if zh == en {
		t.Fatal("expected different translations for zh and en")
	}
}

func TestDefaultBundleWithArgs(t *testing.T) {
	got := T(LocaleZH, "agent.skill_not_found", "web_search")
	if got != "未找到技能: web_search" {
		t.Fatalf("expected formatted message, got %q", got)
	}

	got = T(LocaleEN, "agent.skill_not_found", "web_search")
	if got != "Skill not found: web_search" {
		t.Fatalf("expected formatted message, got %q", got)
	}
}

func TestSetFallback(t *testing.T) {
	b := NewBundle(LocaleZH)
	b.Set(LocaleEN, "only_en", "English only")
	b.SetFallback(LocaleEN)

	if got := b.T(LocaleZH, "only_en"); got != "English only" {
		t.Fatalf("expected fallback to en, got %q", got)
	}
}

func TestLoadJSONInvalid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	os.WriteFile(path, []byte(`not json`), 0o644)

	b := NewBundle(LocaleZH)
	if err := b.LoadJSON(LocaleZH, path); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestLoadJSONNotFound(t *testing.T) {
	b := NewBundle(LocaleZH)
	if err := b.LoadJSON(LocaleZH, "/nonexistent/path.json"); err == nil {
		t.Fatal("expected error for missing file")
	}
}
