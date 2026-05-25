package planner

import (
	"strings"
	"testing"
	"time"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/pkg/skills"
)

func TestPromptRuntimeServiceNativeFCInvalidatesCache(t *testing.T) {
	service := NewPromptRuntimeService()
	if service.NativeFC() {
		t.Fatal("native FC should default to false")
	}
	if service.sysPromptVer != 0 {
		t.Fatalf("unexpected initial prompt version: %d", service.sysPromptVer)
	}
	service.SetNativeFC(true)
	if !service.NativeFC() {
		t.Fatal("native FC should be enabled")
	}
	if service.sysPromptVer != 1 {
		t.Fatalf("expected cache invalidation, got version %d", service.sysPromptVer)
	}
}

func TestPromptRuntimeServiceBuildSystemPromptUsesSkillIndex(t *testing.T) {
	reg := skills.NewRegistry()
	reg.Register(dummyPlannerSkill("demo_skill"))
	service := NewPromptRuntimeService()
	service.SetSkillIndex(func() []SkillIndexEntry {
		return []SkillIndexEntry{{Slug: "demo_skill", Description: "demo description"}}
	})

	prompt := service.BuildSystemPrompt(reg)
	if !strings.Contains(prompt, "demo_skill") {
		t.Fatalf("expected prompt to mention skill/index, got %q", prompt)
	}
}

func TestPromptRuntimeServiceLocaleFallback(t *testing.T) {
	service := NewPromptRuntimeService()
	if got := service.Locale(); got != "zh-CN" {
		t.Fatalf("expected zh-CN default locale, got %q", got)
	}
	service.SetLocale("en")
	if got := service.Translate("planner.task_stopped"); got != "Task stopped." {
		t.Fatalf("unexpected translation: %q", got)
	}
	if got := service.TaskStoppedReply(); got != "Task stopped." {
		t.Fatalf("unexpected task stopped reply: %q", got)
	}
	if got := service.ReflectRetryPrompt(); got != "Your response quality was insufficient. Please reorganize a more thorough answer." {
		t.Fatalf("unexpected reflect retry prompt: %q", got)
	}
}

func TestPromptRuntimeServiceBuildStablePrefix(t *testing.T) {
	service := NewPromptRuntimeService()
	service.SetPersonaPrompt(func() string { return "persona" })
	service.SetDomainPrompt("domain")

	got := service.BuildStablePrefix(false, "group", func() string {
		return "system"
	}, func() string {
		t.Fatal("subagent prompt should not be used when delegation is enabled")
		return ""
	})

	for _, want := range []string{"persona", "system", "domain", "group"} {
		if !strings.Contains(got, want) {
			t.Fatalf("stable prefix missing %q: %q", want, got)
		}
	}

	subagent := service.BuildStablePrefix(true, "", func() string {
		t.Fatal("default prompt should not be used for subagent mode")
		return ""
	}, func() string {
		return "subagent"
	})
	if !strings.Contains(subagent, "persona") || !strings.Contains(subagent, "subagent") || !strings.Contains(subagent, "domain") {
		t.Fatalf("unexpected subagent stable prefix: %q", subagent)
	}
}

func TestPromptRuntimeServicePrepareConversationMessages(t *testing.T) {
	service := NewPromptRuntimeService()
	now := time.Date(2026, 5, 23, 16, 58, 0, 0, time.Local)
	msgs := service.PrepareConversationMessages([]llm.Message{
		{Role: "user", Content: strings.Repeat("目标", 80)},
		{Role: "assistant", Content: "收到"},
		{Role: "user", Content: "继续"},
	}, now)

	if len(msgs) != 4 {
		t.Fatalf("expected goal recitation inserted, got %d messages: %#v", len(msgs), msgs)
	}
	if !strings.HasPrefix(msgs[3].Content, "[时间: 2026-05-23 16:58]\n继续") {
		t.Fatalf("expected timestamp on last user message, got %q", msgs[3].Content)
	}
	if msgs[2].Role != "system" || !strings.Contains(msgs[2].Content, "[任务焦点] 用户的核心目标: ") {
		t.Fatalf("expected focus recitation system message, got %#v", msgs[2])
	}
	if len([]rune(msgs[2].Content)) > len([]rune("[任务焦点] 用户的核心目标: "))+103 {
		t.Fatalf("expected focus recitation to be truncated, got %q", msgs[2].Content)
	}
}

func TestPromptRuntimeServicePrepareConversationMessagesPreservesMultimodalParts(t *testing.T) {
	service := NewPromptRuntimeService()
	now := time.Date(2026, 5, 23, 17, 1, 0, 0, time.Local)
	msgs := service.PrepareConversationMessages([]llm.Message{
		{
			Role: "user",
			ContentParts: []llm.ContentPart{
				{Type: "image_url", ImageURL: &llm.MediaURL{URL: "data:image/png;base64,abc"}},
			},
		},
	}, now)

	if len(msgs) != 1 || len(msgs[0].ContentParts) != 2 {
		t.Fatalf("expected timestamp text part prepended, got %#v", msgs)
	}
	if msgs[0].ContentParts[0].Type != "text" || !strings.HasPrefix(msgs[0].ContentParts[0].Text, "[时间: 2026-05-23 17:01]") {
		t.Fatalf("unexpected first content part: %#v", msgs[0].ContentParts[0])
	}
	if msgs[0].ContentParts[1].Type != "image_url" {
		t.Fatalf("expected original image part preserved, got %#v", msgs[0].ContentParts)
	}
}
