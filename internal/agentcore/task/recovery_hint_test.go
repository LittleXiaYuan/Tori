package task

import "testing"

func TestBuildRecoveryHintClassifiesConnectorFailure(t *testing.T) {
	hint := BuildRecoveryHint(&Task{
		ID:          "task-1",
		Title:       "生成周报",
		Description: "导出项目周报",
		Status:      StatusFailed,
		Error:       "浏览器连接器未连接",
	})

	if hint == nil {
		t.Fatal("expected recovery hint")
	}
	if hint.Category != "connector" {
		t.Fatalf("category=%q, want connector", hint.Category)
	}
	if hint.PrimaryAction.Href != "/packs/browser" {
		t.Fatalf("href=%q, want /packs/browser", hint.PrimaryAction.Href)
	}
	if hint.SecondaryActions[0].Endpoint != "/v1/tasks/restart" {
		t.Fatalf("restart endpoint=%q, want /v1/tasks/restart", hint.SecondaryActions[0].Endpoint)
	}
}

func TestBuildRecoveryHintClassifiesProviderSpecificFailures(t *testing.T) {
	tests := []struct {
		name     string
		err      string
		severity string
		summary  string
		href     string
	}{
		{
			name:     "auth error with provider context",
			err:      "provider openai returned 401 unauthorized: invalid api key",
			severity: "danger",
			summary:  "模型供应商认证失败，需要检查 API Key、Base URL 或账号权限",
			href:     "/settings/providers?tab=providers",
		},
		{
			name:     "auth error with explicit provider id",
			err:      "provider_id=qwen-backup returned 403 forbidden",
			severity: "danger",
			summary:  "模型供应商认证失败，需要检查 API Key、Base URL 或账号权限",
			href:     "/settings/providers?focus=qwen-backup",
		},
		{
			name:     "quota exhausted",
			err:      "llm call failed: 402 insufficient balance, quota exceeded",
			severity: "danger",
			summary:  "模型供应商额度或余额不足，需要充值或切换模型",
			href:     "/settings/providers?tab=providers",
		},
		{
			name:     "rate limited",
			err:      "model provider rate limit exceeded: 429 too many requests",
			severity: "warning",
			summary:  "模型供应商请求被限流，需要等待、降并发或切换模型",
			href:     "/settings/providers?tab=providers",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hint := BuildRecoveryHint(&Task{
				ID:     "task-provider",
				Title:  "生成分析报告",
				Status: StatusFailed,
				Error:  tt.err,
			})

			if hint == nil {
				t.Fatal("expected recovery hint")
			}
			if hint.Category != "provider" {
				t.Fatalf("category=%q, want provider", hint.Category)
			}
			if hint.Severity != tt.severity {
				t.Fatalf("severity=%q, want %q", hint.Severity, tt.severity)
			}
			if hint.Summary != tt.summary {
				t.Fatalf("summary=%q, want %q", hint.Summary, tt.summary)
			}
			if hint.PrimaryAction.Href != tt.href {
				t.Fatalf("href=%q, want %q", hint.PrimaryAction.Href, tt.href)
			}
		})
	}
}

func TestBuildRecoveryHintAddsStableGroupKey(t *testing.T) {
	providerHint := BuildRecoveryHint(&Task{
		ID:     "task-provider-a",
		Title:  "生成分析报告",
		Status: StatusFailed,
		Error:  "provider_id=qwen-backup returned 403 forbidden",
	})
	if providerHint == nil {
		t.Fatal("expected provider recovery hint")
	}
	if providerHint.GroupKey != "provider|/settings/providers?focus=qwen-backup" {
		t.Fatalf("provider group_key=%q", providerHint.GroupKey)
	}

	otherProviderHint := BuildRecoveryHint(&Task{
		ID:     "task-provider-b",
		Title:  "生成另一个报告",
		Status: StatusFailed,
		Error:  "provider_id=qwen-backup returned 401 unauthorized",
	})
	if otherProviderHint == nil {
		t.Fatal("expected second provider recovery hint")
	}
	if otherProviderHint.GroupKey != providerHint.GroupKey {
		t.Fatalf("provider group keys differ: %q vs %q", otherProviderHint.GroupKey, providerHint.GroupKey)
	}

	connectorHint := BuildRecoveryHint(&Task{
		ID:     "task-connector-a",
		Title:  "同步 GitHub",
		Status: StatusFailed,
		Error:  "connector github returned 401 unauthorized: token expired",
	})
	if connectorHint == nil {
		t.Fatal("expected connector recovery hint")
	}
	if connectorHint.GroupKey != "connector|/settings/connectors?focus=github" {
		t.Fatalf("connector group_key=%q", connectorHint.GroupKey)
	}
}

func TestBuildRecoveryHintClassifiesConnectorSpecificFailures(t *testing.T) {
	tests := []struct {
		name     string
		err      string
		severity string
		summary  string
		href     string
	}{
		{
			name:     "auth error with connector context",
			err:      "connector github returned 401 unauthorized: token expired",
			severity: "danger",
			summary:  "连接器认证或凭据失效，需要重新授权",
			href:     "/settings/connectors?focus=github",
		},
		{
			name:     "allowlist action denied",
			err:      "connector github execute delete_repo not allowed by allowlist",
			severity: "danger",
			summary:  "连接器动作超出 Allowlist，需要检查能力边界或改写任务动作",
			href:     "/settings/connectors?focus=github",
		},
		{
			name:     "connector rate limited",
			err:      "connector slack rate limit: 429 too many requests",
			severity: "warning",
			summary:  "连接器请求被限流，需要等待或降低调用频率",
			href:     "/settings/connectors?focus=slack",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hint := BuildRecoveryHint(&Task{
				ID:     "task-connector",
				Title:  "同步外部应用",
				Status: StatusFailed,
				Error:  tt.err,
			})

			if hint == nil {
				t.Fatal("expected recovery hint")
			}
			if hint.Category != "connector" {
				t.Fatalf("category=%q, want connector", hint.Category)
			}
			if hint.Severity != tt.severity {
				t.Fatalf("severity=%q, want %q", hint.Severity, tt.severity)
			}
			if hint.Summary != tt.summary {
				t.Fatalf("summary=%q, want %q", hint.Summary, tt.summary)
			}
			if hint.PrimaryAction.Href != tt.href {
				t.Fatalf("href=%q, want %q", hint.PrimaryAction.Href, tt.href)
			}
		})
	}
}

func TestBuildRecoveryHintClassifiesSkillAndToolFailures(t *testing.T) {
	tests := []struct {
		name     string
		err      string
		category string
		summary  string
		href     string
	}{
		{
			name:     "unknown skill",
			err:      "planner step failed: unknown skill document_writer",
			category: "skill",
			summary:  "技能不可用，需要安装、启用或更换执行技能",
			href:     "/skills",
		},
		{
			name:     "legacy unknown skill tool",
			err:      "planner step failed: unknown skill missing_tool",
			category: "tool",
			summary:  "工具不可用，需要启用工具或调整任务能力边界",
			href:     "/tools",
		},
		{
			name:     "tool unavailable",
			err:      "tool unavailable: browser_extract is disabled",
			category: "tool",
			summary:  "工具不可用，需要启用工具或调整任务能力边界",
			href:     "/tools",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hint := BuildRecoveryHint(&Task{
				ID:     "task-capability",
				Title:  "执行能力调用",
				Status: StatusFailed,
				Error:  tt.err,
			})

			if hint == nil {
				t.Fatal("expected recovery hint")
			}
			if hint.Category != tt.category {
				t.Fatalf("category=%q, want %q", hint.Category, tt.category)
			}
			if hint.Summary != tt.summary {
				t.Fatalf("summary=%q, want %q", hint.Summary, tt.summary)
			}
			if hint.PrimaryAction.Href != tt.href {
				t.Fatalf("href=%q, want %q", hint.PrimaryAction.Href, tt.href)
			}
		})
	}
}

func TestBuildRecoveryHintClassifiesApprovalFailure(t *testing.T) {
	hint := BuildRecoveryHint(&Task{
		ID:     "task-approval",
		Title:  "执行高风险操作",
		Status: StatusFailed,
		Error:  "approval required before deleting workspace files",
	})

	if hint == nil {
		t.Fatal("expected recovery hint")
	}
	if hint.Category != "approval" {
		t.Fatalf("category=%q, want approval", hint.Category)
	}
	if hint.PrimaryAction.Href != "/approvals" {
		t.Fatalf("href=%q, want /approvals", hint.PrimaryAction.Href)
	}
}

func TestBuildRecoveryHintClassifiesDependencyBlock(t *testing.T) {
	hint := BuildRecoveryHint(&Task{
		ID:     "task-dependency",
		Title:  "恢复依赖阻塞",
		Status: StatusInterrupted,
		Error:  "步骤 2 等待依赖步骤完成：1",
		Steps: []Step{
			{ID: 1, Action: "前置步骤", Status: StepPending},
			{ID: 2, Action: "等待后续", Status: StepPending, DependsOn: []int{1}},
		},
	})

	if hint == nil {
		t.Fatal("expected recovery hint")
	}
	if hint.Category != "dependency" {
		t.Fatalf("category=%q, want dependency", hint.Category)
	}
	if hint.Summary != "任务依赖未满足，需要先查看执行链" {
		t.Fatalf("summary=%q", hint.Summary)
	}
	if hint.PrimaryAction.Href != "/task-detail?id=task-dependency&tab=execution" {
		t.Fatalf("href=%q, want execution tab", hint.PrimaryAction.Href)
	}
}

func TestBuildRecoveryHintClassifiesSandboxFailure(t *testing.T) {
	hint := BuildRecoveryHint(&Task{
		ID:     "task-sandbox",
		Title:  "操作桌面应用",
		Status: StatusFailed,
		Error:  "desktop sandbox unavailable: computer-use bridge disconnected",
	})

	if hint == nil {
		t.Fatal("expected recovery hint")
	}
	if hint.Category != "sandbox" {
		t.Fatalf("category=%q, want sandbox", hint.Category)
	}
	if hint.Summary != "桌面沙箱不可用，需要检查 Computer Use 能力" {
		t.Fatalf("summary=%q", hint.Summary)
	}
	if hint.PrimaryAction.Href != "/packs/computer-use" {
		t.Fatalf("href=%q, want /packs/computer-use", hint.PrimaryAction.Href)
	}
}

func TestBuildRecoveryHintKeepsExplicitHint(t *testing.T) {
	explicit := &RecoveryHint{
		Category: "custom",
		Severity: "danger",
		Summary:  "custom recovery",
		PrimaryAction: RecoveryAction{
			ID:    "custom_action",
			Label: "Custom",
			Href:  "/custom",
		},
		Source: "runner",
	}
	hint := BuildRecoveryHint(&Task{ID: "task-1", Status: StatusFailed, RecoveryHint: explicit})

	if hint == nil || hint.Category != "custom" || hint.PrimaryAction.Href != "/custom" {
		t.Fatalf("explicit hint not preserved: %#v", hint)
	}
	hint.PrimaryAction.Href = "/changed"
	if explicit.PrimaryAction.Href != "/custom" {
		t.Fatal("BuildRecoveryHint returned mutable explicit hint")
	}
	if hint.GroupKey != "custom|/custom" {
		t.Fatalf("explicit hint group_key=%q, want custom|/custom", hint.GroupKey)
	}
	if explicit.GroupKey != "" {
		t.Fatalf("explicit hint mutated with group_key=%q", explicit.GroupKey)
	}
}
