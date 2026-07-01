package planner

import (
	"context"
	"strings"
	"testing"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/pkg/skills"
)

func TestRuntimeRequestMessagesAssemblesStableDynamicConversationAndLayers(t *testing.T) {
	p := NewPlanner(nil, skills.NewRegistry(), 8)
	p.SetDomainPrompt("领域提示")
	p.SetMemory(func(_ context.Context, tenantID, query string) string {
		if tenantID != "tenant-msg" || !strings.Contains(query, "长期任务") {
			t.Fatalf("unexpected memory query: tenant=%q query=%q", tenantID, query)
		}
		return "memory context for " + tenantID
	})

	msgs, layers := p.BuildMessages(context.Background(), PlanRequest{
		Messages: []llm.Message{
			{Role: "user", Content: strings.Repeat("核心目标", 40)},
			{Role: "assistant", Content: "收到"},
			{Role: "user", Content: "继续推进长期任务"},
		},
		TenantID:    "tenant-msg",
		ChannelType: "web",
	})

	if len(msgs) < 4 {
		t.Fatalf("expected stable prefix, dynamic context, focus recitation and user message, got %#v", msgs)
	}
	if msgs[0].Role != "system" || !strings.Contains(msgs[0].Content, "领域提示") {
		t.Fatalf("stable prefix missing domain prompt: %#v", msgs[0])
	}
	if msgs[1].Role != "system" || !strings.HasPrefix(msgs[1].Content, "[动态上下文]\n") || !strings.Contains(msgs[1].Content, "memory context for tenant-msg") {
		t.Fatalf("dynamic context message not assembled as separate system message: %#v", msgs[1])
	}
	if len(layers) == 0 {
		t.Fatalf("expected dynamic context layers, got %#v", layers)
	}

	foundGoalRecitation := false
	foundTimestampedLastUser := false
	for _, msg := range msgs {
		if msg.Role == "system" && strings.Contains(msg.Content, "[任务焦点] 用户的核心目标:") {
			foundGoalRecitation = true
		}
		if msg.Role == "user" && strings.Contains(msg.Content, "继续推进长期任务") && strings.Contains(msg.Content, "[时间:") {
			foundTimestampedLastUser = true
		}
	}
	if !foundGoalRecitation {
		t.Fatalf("expected prompt runtime goal recitation in assembled messages: %#v", msgs)
	}
	if !foundTimestampedLastUser {
		t.Fatalf("expected timestamp on last user message: %#v", msgs)
	}
}

func TestRuntimeRequestMessagesIncludesWorkspaceContext(t *testing.T) {
	p := NewPlanner(nil, skills.NewRegistry(), 8)

	msgs, layers := p.BuildMessages(context.Background(), PlanRequest{
		Messages: []llm.Message{{Role: "user", Content: "请修改校园管理项目里的页面"}},
		WorkspacePaths: []string{
			`C:\Users\Administrator\Documents\校园管理`,
			`C:\Users\Administrator\Documents\校园管理\`,
			`C:\Code\AI\云雀\yunque-agent`,
		},
	})

	foundWorkspace := false
	for _, msg := range msgs {
		if msg.Role == "system" && strings.Contains(msg.Content, "[当前工作区]") {
			foundWorkspace = true
			if !strings.Contains(msg.Content, `C:\Users\Administrator\Documents\校园管理`) {
				t.Fatalf("workspace context missing project path: %q", msg.Content)
			}
			if strings.Count(msg.Content, "校园管理") != 1 {
				t.Fatalf("workspace context should deduplicate paths: %q", msg.Content)
			}
		}
	}
	if !foundWorkspace {
		t.Fatalf("expected workspace context system message, got %#v", msgs)
	}
	if !containsString(layers, "workspace") {
		t.Fatalf("expected workspace layer marker, got %#v", layers)
	}
}

func TestRuntimeRequestMessagesIncludesSessionFilesContext(t *testing.T) {
	p := NewPlanner(nil, skills.NewRegistry(), 8)

	msgs, layers := p.BuildMessages(context.Background(), PlanRequest{
		Messages: []llm.Message{{Role: "user", Content: "把刚才那张图改成雪山背景"}},
		SessionFiles: []SessionFileRef{
			{Path: "data/uploads/default/cat.png", Name: "cat.png", Kind: "uploaded"},
			{Path: "data/output/report.docx", Name: "report.docx", Kind: "generated"},
		},
	})

	foundSessionFiles := false
	for _, msg := range msgs {
		if msg.Role == "system" && strings.Contains(msg.Content, "[本次对话已有文件]") {
			foundSessionFiles = true
			if !strings.Contains(msg.Content, "cat.png") || !strings.Contains(msg.Content, "data/uploads/default/cat.png") {
				t.Fatalf("session files context missing uploaded file: %q", msg.Content)
			}
			if !strings.Contains(msg.Content, "report.docx") || !strings.Contains(msg.Content, "data/output/report.docx") {
				t.Fatalf("session files context missing generated file: %q", msg.Content)
			}
			if !strings.Contains(msg.Content, "用户上传") || !strings.Contains(msg.Content, "已生成") {
				t.Fatalf("session files context should label file provenance: %q", msg.Content)
			}
		}
	}
	if !foundSessionFiles {
		t.Fatalf("expected session files context system message, got %#v", msgs)
	}
	if !containsString(layers, "session_files") {
		t.Fatalf("expected session_files layer marker, got %#v", layers)
	}
}

func TestRuntimeRequestMessagesOmitsSessionFilesContextWhenEmpty(t *testing.T) {
	p := NewPlanner(nil, skills.NewRegistry(), 8)

	msgs, layers := p.BuildMessages(context.Background(), PlanRequest{
		Messages: []llm.Message{{Role: "user", Content: "你好"}},
	})

	for _, msg := range msgs {
		if strings.Contains(msg.Content, "[本次对话已有文件]") {
			t.Fatalf("did not expect session files context with no files: %#v", msgs)
		}
	}
	if containsString(layers, "session_files") {
		t.Fatalf("did not expect session_files layer marker, got %#v", layers)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
