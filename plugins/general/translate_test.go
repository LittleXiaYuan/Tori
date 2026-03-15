package general

import (
	"context"
	"strings"
	"testing"

	"yunque-agent/pkg/skills"
)

func mockLLMCall(reply string) func(ctx context.Context, system, user string) (string, error) {
	return func(ctx context.Context, system, user string) (string, error) {
		return reply, nil
	}
}

func TestTranslateSkill_Basic(t *testing.T) {
	s := NewTranslateSkill()
	env := &skills.Environment{
		LLMCall: mockLLMCall("Hello World"),
	}
	result, err := s.Execute(context.Background(), map[string]any{
		"text":        "你好世界",
		"target_lang": "英语",
	}, env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Hello World" {
		t.Fatalf("expected 'Hello World', got %q", result)
	}
}

func TestTranslateSkill_WithStyle(t *testing.T) {
	var capturedSystem string
	s := NewTranslateSkill()
	env := &skills.Environment{
		LLMCall: func(ctx context.Context, system, user string) (string, error) {
			capturedSystem = system
			return "Bonjour le monde", nil
		},
	}
	result, err := s.Execute(context.Background(), map[string]any{
		"text":        "Hello world",
		"target_lang": "法语",
		"style":       "formal",
	}, env)
	if err != nil {
		t.Fatal(err)
	}
	if result != "Bonjour le monde" {
		t.Fatalf("unexpected result: %q", result)
	}
	if !strings.Contains(capturedSystem, "正式") {
		t.Fatal("expected formal style in system prompt")
	}
}

func TestTranslateSkill_WithSourceLang(t *testing.T) {
	var capturedSystem string
	s := NewTranslateSkill()
	env := &skills.Environment{
		LLMCall: func(ctx context.Context, system, user string) (string, error) {
			capturedSystem = system
			return "こんにちは世界", nil
		},
	}
	_, err := s.Execute(context.Background(), map[string]any{
		"text":        "Hello world",
		"target_lang": "日语",
		"source_lang": "英语",
	}, env)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(capturedSystem, "源语言：英语") {
		t.Fatal("expected source lang in prompt")
	}
}

func TestTranslateSkill_EmptyText(t *testing.T) {
	s := NewTranslateSkill()
	env := &skills.Environment{LLMCall: mockLLMCall("")}
	_, err := s.Execute(context.Background(), map[string]any{
		"text":        "",
		"target_lang": "英语",
	}, env)
	if err == nil || !strings.Contains(err.Error(), "text is required") {
		t.Fatalf("expected text required error, got %v", err)
	}
}

func TestTranslateSkill_EmptyTargetLang(t *testing.T) {
	s := NewTranslateSkill()
	env := &skills.Environment{LLMCall: mockLLMCall("")}
	_, err := s.Execute(context.Background(), map[string]any{
		"text":        "hello",
		"target_lang": "",
	}, env)
	if err == nil || !strings.Contains(err.Error(), "target_lang is required") {
		t.Fatalf("expected target_lang required error, got %v", err)
	}
}

func TestTranslateSkill_UnsupportedLang(t *testing.T) {
	s := NewTranslateSkill()
	env := &skills.Environment{LLMCall: mockLLMCall("")}
	_, err := s.Execute(context.Background(), map[string]any{
		"text":        "hello",
		"target_lang": "火星语",
	}, env)
	if err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("expected unsupported language error, got %v", err)
	}
}

func TestTranslateSkill_TextTooLong(t *testing.T) {
	s := NewTranslateSkill()
	env := &skills.Environment{LLMCall: mockLLMCall("")}
	longText := strings.Repeat("a", 10001)
	_, err := s.Execute(context.Background(), map[string]any{
		"text":        longText,
		"target_lang": "中文",
	}, env)
	if err == nil || !strings.Contains(err.Error(), "too long") {
		t.Fatalf("expected too long error, got %v", err)
	}
}

func TestTranslateSkill_NoLLM(t *testing.T) {
	s := NewTranslateSkill()
	env := &skills.Environment{}
	_, err := s.Execute(context.Background(), map[string]any{
		"text":        "hello",
		"target_lang": "中文",
	}, env)
	if err == nil || !strings.Contains(err.Error(), "LLM not available") {
		t.Fatalf("expected LLM not available error, got %v", err)
	}
}

func TestTranslateSkill_StripQuotes(t *testing.T) {
	s := NewTranslateSkill()
	env := &skills.Environment{
		LLMCall: mockLLMCall(`"Hello World"`),
	}
	result, err := s.Execute(context.Background(), map[string]any{
		"text":        "你好世界",
		"target_lang": "english",
	}, env)
	if err != nil {
		t.Fatal(err)
	}
	if result != "Hello World" {
		t.Fatalf("expected quotes stripped, got %q", result)
	}
}

func TestTranslateSkill_Metadata(t *testing.T) {
	s := NewTranslateSkill()
	if s.Name() != "translate" {
		t.Fatalf("expected name 'translate', got %q", s.Name())
	}
	params := s.Parameters()
	props, ok := params["properties"].(map[string]any)
	if !ok {
		t.Fatal("expected properties map")
	}
	if _, ok := props["text"]; !ok {
		t.Fatal("missing 'text' parameter")
	}
	if _, ok := props["target_lang"]; !ok {
		t.Fatal("missing 'target_lang' parameter")
	}
}

func TestBuildTranslatePrompt(t *testing.T) {
	p := buildTranslatePrompt("英语", "", "")
	if !strings.Contains(p, "英语") {
		t.Fatal("expected target lang in prompt")
	}
	if !strings.Contains(p, "自动检测") {
		t.Fatal("expected auto-detect when no source lang")
	}

	p2 := buildTranslatePrompt("日语", "中文", "technical")
	if !strings.Contains(p2, "源语言：中文") {
		t.Fatal("expected source lang")
	}
	if !strings.Contains(p2, "技术性") {
		t.Fatal("expected technical style")
	}
}
