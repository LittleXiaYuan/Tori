package task

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildStepCard_Done(t *testing.T) {
	cardJSON := buildStepCard("重构认证模块", 3, 10, "修改 auth.go", "done", "已完成修改")
	var card map[string]any
	if err := json.Unmarshal([]byte(cardJSON), &card); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	header, _ := card["header"].(map[string]any)
	title, _ := header["title"].(map[string]any)
	content, _ := title["content"].(string)
	if !strings.Contains(content, "3/10") {
		t.Fatalf("title should contain step progress, got %q", content)
	}
	if !strings.Contains(content, "完成") {
		t.Fatalf("title should contain 完成, got %q", content)
	}
	template, _ := header["template"].(string)
	if template != "blue" {
		t.Fatalf("expected blue, got %s", template)
	}
}

func TestBuildStepCard_Failed(t *testing.T) {
	cardJSON := buildStepCard("部署服务", 5, 8, "执行测试", "failed", "exit code 1")
	var card map[string]any
	json.Unmarshal([]byte(cardJSON), &card)
	header, _ := card["header"].(map[string]any)
	template, _ := header["template"].(string)
	if template != "red" {
		t.Fatalf("expected red for failed, got %s", template)
	}
}

func TestBuildTaskCompletedCard(t *testing.T) {
	cardJSON := buildTaskCompletedCard("优化数据库查询", "tsk-123", "查询时间从 2s 降到 200ms")
	var card map[string]any
	if err := json.Unmarshal([]byte(cardJSON), &card); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	header, _ := card["header"].(map[string]any)
	title, _ := header["title"].(map[string]any)
	content, _ := title["content"].(string)
	if !strings.Contains(content, "完成") {
		t.Fatalf("expected 完成, got %q", content)
	}
	template, _ := header["template"].(string)
	if template != "green" {
		t.Fatalf("expected green, got %s", template)
	}
}

func TestBuildTaskFailedCard(t *testing.T) {
	cardJSON := buildTaskFailedCard("升级依赖", "tsk-456", "npm install failed")
	var card map[string]any
	if err := json.Unmarshal([]byte(cardJSON), &card); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	header, _ := card["header"].(map[string]any)
	template, _ := header["template"].(string)
	if template != "red" {
		t.Fatalf("expected red, got %s", template)
	}
	// Should have a retry button
	elements, _ := card["elements"].([]any)
	found := false
	for _, el := range elements {
		m, _ := el.(map[string]any)
		if m["tag"] == "action" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected action block with retry button")
	}
}

func TestTextProgressBar(t *testing.T) {
	tests := []struct {
		done, total int
		wantLen     int
	}{
		{0, 10, 12},  // [----------]
		{5, 10, 12},  // [#####-----]
		{10, 10, 12}, // [##########]
	}
	for _, tc := range tests {
		bar := textProgressBar(tc.done, tc.total)
		if len(bar) != tc.wantLen {
			t.Fatalf("done=%d total=%d: expected len %d, got %d (%q)",
				tc.done, tc.total, tc.wantLen, len(bar), bar)
		}
	}
}
