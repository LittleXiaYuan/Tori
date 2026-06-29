package cogni

import (
	"context"
	"testing"
)

func TestIntentCogni_Analyze_Search(t *testing.T) {
	cogni := NewIntentCogni()
	req := CogniRequest{
		Message: "帮我搜索一下 Rust 的生命周期规则",
	}

	decision := cogni.Analyze(context.Background(), req)

	if decision.Intent == nil || decision.Intent.Type != "search" {
		t.Errorf("expected intent=search, got %v", decision.Intent)
	}

	if !contains(decision.ToolsNeeded, "browser_search") {
		t.Errorf("expected browser_search in tools, got %v", decision.ToolsNeeded)
	}

	if !contains(decision.SkillsNeeded, "research") {
		t.Errorf("expected research in skills, got %v", decision.SkillsNeeded)
	}

	if decision.MemoryScope.Limit == 0 {
		t.Errorf("expected memory limit > 0, got %d", decision.MemoryScope.Limit)
	}
}

func TestIntentCogni_Analyze_Code(t *testing.T) {
	cogni := NewIntentCogni()
	req := CogniRequest{
		Message: "帮我审查这个 PR 的代码",
	}

	decision := cogni.Analyze(context.Background(), req)

	if decision.Intent == nil || decision.Intent.Type != "code" {
		t.Errorf("expected intent=code, got %v", decision.Intent)
	}

	if !containsPattern(decision.ToolsNeeded, "file_*") {
		t.Errorf("expected file_* in tools, got %v", decision.ToolsNeeded)
	}

	if !containsPattern(decision.ToolsNeeded, "github_*") {
		t.Errorf("expected github_* in tools, got %v", decision.ToolsNeeded)
	}

	if !contains(decision.SkillsNeeded, "code") {
		t.Errorf("expected code in skills, got %v", decision.SkillsNeeded)
	}
}

func TestIntentCogni_Analyze_Chat(t *testing.T) {
	cogni := NewIntentCogni()
	req := CogniRequest{
		Message: "今天心情不太好，能陪我聊聊吗",
	}

	decision := cogni.Analyze(context.Background(), req)

	if decision.Intent == nil || decision.Intent.Type != "chat" {
		t.Errorf("expected intent=chat, got %v", decision.Intent)
	}

	// Chat should not need tools or skills
	if len(decision.ToolsNeeded) != 0 {
		t.Errorf("expected no tools for chat, got %v", decision.ToolsNeeded)
	}

	if len(decision.SkillsNeeded) != 0 {
		t.Errorf("expected no skills for chat, got %v", decision.SkillsNeeded)
	}

	// Chat should focus on conversation memory
	if !contains(decision.MemoryScope.Categories, "conversation") {
		t.Errorf("expected conversation in memory categories, got %v", decision.MemoryScope.Categories)
	}
}

func TestIntentCogni_Analyze_Browser(t *testing.T) {
	cogni := NewIntentCogni()
	req := CogniRequest{
		Message: "打开浏览器访问 example.com",
	}

	decision := cogni.Analyze(context.Background(), req)

	if decision.Intent == nil || decision.Intent.Type != "browser" {
		t.Errorf("expected intent=browser, got %v", decision.Intent)
	}

	if !containsPattern(decision.ToolsNeeded, "browser_*") {
		t.Errorf("expected browser_* in tools, got %v", decision.ToolsNeeded)
	}
}

func TestIntentCogni_Analyze_Complex(t *testing.T) {
	cogni := NewIntentCogni()
	req := CogniRequest{
		Message: "帮我做一个完整的项目，包括前端、后端、数据库设计和部署",
	}

	decision := cogni.Analyze(context.Background(), req)

	if decision.Intent == nil || decision.Intent.Type != "complex" {
		t.Errorf("expected intent=complex, got %v", decision.Intent)
	}

	// Complex tasks should not restrict resources
	if decision.ToolsNeeded != nil {
		t.Errorf("expected nil tools (no restriction) for complex, got %v", decision.ToolsNeeded)
	}

	if decision.SkillsNeeded != nil {
		t.Errorf("expected nil skills (no restriction) for complex, got %v", decision.SkillsNeeded)
	}
}

func TestIntentCogni_Priority(t *testing.T) {
	cogni := NewIntentCogni()

	if cogni.Priority() != 100 {
		t.Errorf("expected priority=100 (highest), got %d", cogni.Priority())
	}
}

func TestDetectIntent(t *testing.T) {
	tests := []struct {
		message  string
		expected string
	}{
		{"搜索 Go 语言教程", "search"},
		{"find documentation", "search"},
		{"读取 main.go 文件", "code"},
		{"review this PR", "code"},
		{"打开浏览器", "browser"},
		{"navigate to example.com", "browser"},
		{"今天心情不好", "chat"},
		{"能陪我聊聊吗", "chat"},
		{"做一个完整的系统", "complex"},
	}

	for _, tt := range tests {
		t.Run(tt.message, func(t *testing.T) {
			result := detectIntent(tt.message)
			if result != tt.expected {
				t.Errorf("detectIntent(%q) = %q, want %q", tt.message, result, tt.expected)
			}
		})
	}
}

// Helper functions
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func containsPattern(slice []string, pattern string) bool {
	for _, s := range slice {
		if s == pattern {
			return true
		}
	}
	return false
}
