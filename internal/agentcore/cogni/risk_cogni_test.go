package cogni

import (
	"context"
	"strings"
	"testing"
)

func TestRiskCogni_Analyze_High(t *testing.T) {
	cogni := NewRiskCogni()
	req := CogniRequest{
		Message: "删除所有日志文件",
	}

	decision := cogni.Analyze(context.Background(), req)

	if decision.State["risk"] != "high" {
		t.Errorf("expected risk=high, got %v", decision.State["risk"])
	}

	// Should filter to read-only tools
	if !contains(decision.ToolsNeeded, "file_read") {
		t.Errorf("expected file_read in allowed tools, got %v", decision.ToolsNeeded)
	}

	// Should NOT include dangerous tools (they're excluded, not in the list)
	// The whitelist approach means we don't have file_write/file_delete

	// Should inject confirmation instruction
	if decision.BehaviorText == "" {
		t.Errorf("expected behavioral guidance for high risk")
	}
	if !strings.Contains(decision.BehaviorText, "确认") && !strings.Contains(decision.BehaviorText, "风险") {
		t.Errorf("expected confirmation or risk mention in behavior text, got: %s", decision.BehaviorText)
	}
}

func TestRiskCogni_Analyze_HighRisk_ForceDelete(t *testing.T) {
	cogni := NewRiskCogni()
	req := CogniRequest{
		Message: "rm -rf /tmp/*",
	}

	decision := cogni.Analyze(context.Background(), req)

	if decision.State["risk"] != "high" {
		t.Errorf("expected risk=high for rm -rf, got %v", decision.State["risk"])
	}
}

func TestRiskCogni_Analyze_Medium(t *testing.T) {
	cogni := NewRiskCogni()
	req := CogniRequest{
		Message: "修改代码中的变量名",
	}

	decision := cogni.Analyze(context.Background(), req)

	if decision.State["risk"] != "medium" {
		t.Errorf("expected risk=medium, got %v", decision.State["risk"])
	}

	// Should not restrict tools (nil means no opinion)
	if decision.ToolsNeeded != nil {
		t.Errorf("expected nil tools for medium risk, got %v", decision.ToolsNeeded)
	}

	// Should inject caution instruction
	if decision.BehaviorText == "" {
		t.Errorf("expected behavioral guidance for medium risk")
	}
}

func TestRiskCogni_Analyze_Low(t *testing.T) {
	cogni := NewRiskCogni()
	req := CogniRequest{
		Message: "帮我查一下这个函数的实现",
	}

	decision := cogni.Analyze(context.Background(), req)

	if decision.State["risk"] != "low" {
		t.Errorf("expected risk=low, got %v", decision.State["risk"])
	}

	// Should not restrict resources
	if decision.ToolsNeeded != nil {
		t.Errorf("expected nil tools for low risk, got %v", decision.ToolsNeeded)
	}

	// Should not inject behavior text
	if decision.BehaviorText != "" {
		t.Errorf("expected no behavior text for low risk, got %q", decision.BehaviorText)
	}
}

func TestRiskCogni_Priority(t *testing.T) {
	cogni := NewRiskCogni()

	if cogni.Priority() != 80 {
		t.Errorf("expected priority=80 (high), got %d", cogni.Priority())
	}
}

func TestDetectRisk(t *testing.T) {
	tests := []struct {
		message  string
		expected string
	}{
		// High risk
		{"删除所有文件", "high"},
		{"delete all logs", "high"},
		{"rm -rf /tmp", "high"},
		{"清空数据库", "high"},
		{"drop table users", "high"},
		{"强制重置", "high"},
		{"kill -9 process", "high"},
		{"执行 shell 命令", "high"},

		// Medium risk
		{"修改端口配置", "medium"},
		{"edit config file", "medium"},
		{"创建新用户", "medium"},
		{"install package", "medium"},
		{"部署到生产环境", "medium"},
		{"commit changes", "medium"},
		{"merge branch", "medium"},

		// Low risk
		{"查看日志", "low"},
		{"read file", "low"},
		{"搜索文档", "low"},
		{"list files", "low"},
		{"帮我分析代码", "low"},
	}

	for _, tt := range tests {
		t.Run(tt.message, func(t *testing.T) {
			result := detectRisk(tt.message)
			if result != tt.expected {
				t.Errorf("detectRisk(%q) = %q, want %q", tt.message, result, tt.expected)
			}
		})
	}
}

func TestRiskCogni_WithIntentCogni_Merge(t *testing.T) {
	// Test that RiskCogni and IntentCogni work together
	intentCogni := NewIntentCogni()
	riskCogni := NewRiskCogni()

	// Scenario: user wants to delete files (code intent + high risk)
	req := CogniRequest{
		Message: "删除项目中的所有临时文件",
	}

	intentDecision := intentCogni.Analyze(context.Background(), req)
	riskDecision := riskCogni.Analyze(context.Background(), req)

	// Merge decisions
	cognis := []CogniWithPriority{
		{Decision: intentDecision, Priority: intentCogni.Priority()}, // 100
		{Decision: riskDecision, Priority: riskCogni.Priority()},     // 80
	}

	final := MergeDecisions(cognis)

	// Intent should be from IntentCogni (higher priority)
	if final.Intent == nil {
		t.Errorf("expected intent from IntentCogni, got nil")
	}

	// Tools: union of IntentCogni's [file_*, code_*] and RiskCogni's [file_read, ...]
	// Result should include both sets
	if len(final.ToolsNeeded) == 0 {
		t.Errorf("expected merged tools, got empty")
	}

	// BehaviorText should include RiskCogni's warning (both have behavior text, merged)
	if final.BehaviorText == "" {
		t.Errorf("expected behavioral guidance from RiskCogni")
	}

	// State should include risk level
	if final.State["risk"] != "high" {
		t.Errorf("expected risk=high in merged state, got %v", final.State["risk"])
	}
}
