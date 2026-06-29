package cogni

import (
	"context"
	"strings"
)

// RiskCogni detects high-risk operations in user messages and filters dangerous
// tools to enforce safety guardrails. It also injects confirmation instructions
// for destructive operations.
//
// This implements a proactive safety layer: instead of letting the model decide
// whether to use dangerous tools, RiskCogni removes them from the tool list
// before the model sees it.
type RiskCogni struct {
	priority int
}

// NewRiskCogni creates a RiskCogni with the given priority.
// Recommended priority: 80 (high, between IntentCogni and EmotionCogni)
func NewRiskCogni() *RiskCogni {
	return &RiskCogni{priority: 80}
}

// Analyze implements HookV2 by detecting risk level and filtering dangerous tools.
func (c *RiskCogni) Analyze(ctx context.Context, req CogniRequest) CogniDecision {
	risk := detectRisk(req.Message)

	decision := CogniDecision{
		State: map[string]any{
			"risk": risk,
		},
	}

	switch risk {
	case "high":
		// High-risk operations: filter out dangerous tools, inject confirmation instruction
		decision.ToolsNeeded = []string{
			"file_read", "file_list", "glob", "grep", // Read-only file ops OK
			"browser_search", "web_fetch",           // Web ops OK
			// Exclude: file_write, file_delete, shell_*, git_reset, etc.
		}
		decision.BehaviorText = "【风险等级：高】涉及删除、修改配置或执行命令的操作，必须在执行前向用户确认。"

	case "medium":
		// Medium-risk: allow most tools, but inject caution instruction
		decision.ToolsNeeded = nil // Don't restrict (let IntentCogni decide)
		decision.BehaviorText = "【风险等级：中】操作可能影响系统状态，不确定时先向用户确认。"

	case "low":
		fallthrough
	default:
		// Low-risk: no restrictions
		decision.ToolsNeeded = nil
		decision.SkillsNeeded = nil
		decision.BehaviorText = ""
	}

	return decision
}

// Priority returns this Cogni's priority in decision merging.
// RiskCogni has high priority (80) to ensure safety guardrails are enforced.
func (c *RiskCogni) Priority() int {
	return c.priority
}

// detectRisk analyzes the message for dangerous operation patterns.
// Returns "high", "medium", or "low".
// Check high-risk patterns first (more specific) before medium-risk (general).
func detectRisk(message string) string {
	lower := strings.ToLower(message)

	// High-risk keywords: destructive operations
	highRiskKeywords := []string{
		"删除", "delete", "remove", "rm -rf", "drop",
		"清空", "清理", "clear", "clean",
		"重置", "reset", "revert",
		"格式化", "format",
		"停止", "kill", "terminate",
		"修改配置", "change config", "edit config", // Specific: config changes are high risk
		"执行命令", "run command", "shell", "bash",
		"强制", "force", "--force", "-f",
	}

	for _, kw := range highRiskKeywords {
		if strings.Contains(lower, kw) {
			return "high"
		}
	}

	// Medium-risk keywords: state-changing operations
	mediumRiskKeywords := []string{
		"修改", "modify", "edit", "change", "update", // General: modify/edit
		"创建", "create", "add", "new",
		"安装", "install", "setup",
		"部署", "deploy", "publish",
		"提交", "commit", "push",
		"合并", "merge",
		"重启", "restart", "reboot",
	}

	for _, kw := range mediumRiskKeywords {
		if strings.Contains(lower, kw) {
			return "medium"
		}
	}

	// Default to low risk
	return "low"
}
