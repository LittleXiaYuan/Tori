package planner

import (
	"context"
	"fmt"
	"strings"
	"time"

	"yunque-agent/internal/agentcore/llm"
)

// BuildMessages constructs the full message list using Manus-style context engineering.
//
// Layout: [stable_prefix] [dynamic_context?] [history...] [goal_recitation?] [last_user_msg+timestamp]
//
// Key principles:
//   - Stable prefix (persona+skills+domain) is a single system message — enables LLM KV-cache reuse
//   - Dynamic context (memory+graph) is a SEPARATE system message — prefix cache survives per-query changes
//   - Timestamp injected into last user message, NOT system prompt — avoids cache invalidation
//   - Goal recitation inserted before last user message in multi-turn — keeps model focused
//   - Errors preserved (append-only context) — model learns from failures
func (p *Planner) BuildMessages(ctx context.Context, req PlanRequest) ([]llm.Message, []string) {
	promptRuntime := p.ensurePromptRuntime()
	contextAssembly := p.ensureContextAssembly()
	contextWindowRuntime := p.ensureContextWindowRuntime()
	modelRuntime := p.ensureModelRuntime()

	stablePrefix := promptRuntime.BuildStablePrefix(req.DisableDelegation, req.GroupSystemPrompt, p.buildSystemPrompt, p.buildSubagentSystemPrompt)
	msgs := []llm.Message{{Role: "system", Content: stablePrefix}}

	var includedLayers []string
	if len(req.Messages) > 0 {
		msgs, includedLayers = contextAssembly.AppendDynamicContextMessage(ctx, msgs, DynamicContextAssemblyRequest{
			LastMessage: req.Messages[len(req.Messages)-1].Content,
			TenantID:    req.TenantID,
			Channel:     req.ChannelType,
			TaskContext: req.TaskContext,
			EmotionHint: req.EmotionHint,
			IntentHint:  req.IntentHint,
		}, NewPromptBuilder(p))
	}
	if workspaceContext := buildWorkspaceContextMessage(req.WorkspacePaths); workspaceContext != "" {
		msgs = append(msgs, llm.Message{Role: "system", Content: workspaceContext})
		includedLayers = append(includedLayers, "workspace")
	}
	if sessionFilesContext := buildSessionFilesContextMessage(req.SessionFiles); sessionFilesContext != "" {
		msgs = append(msgs, llm.Message{Role: "system", Content: sessionFilesContext})
		includedLayers = append(includedLayers, "session_files")
	}

	msgs = append(msgs, promptRuntime.PrepareConversationMessages(req.Messages, time.Now())...)
	msgs = contextWindowRuntime.FitMessagesForRequest(ctx, msgs, modelRuntime.ClientForRequest(req))
	return msgs, includedLayers
}

func buildWorkspaceContextMessage(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	const maxPaths = 8
	var b strings.Builder
	b.WriteString("[当前工作区]\n")
	b.WriteString("以下是用户在桌面端登记/打开的本地项目目录。处理读写文件、搜索代码、生成或修改工程文件时，优先在这些目录内定位目标；可使用 file_search 读取，使用 file_create 创建或更新文件。\n")
	written := 0
	seen := map[string]bool{}
	for _, raw := range paths {
		path := strings.TrimSpace(raw)
		if path == "" {
			continue
		}
		key := strings.ToLower(strings.TrimRight(path, `\/`))
		if seen[key] {
			continue
		}
		seen[key] = true
		if written >= maxPaths {
			b.WriteString("- ...\n")
			break
		}
		b.WriteString("- ")
		b.WriteString(path)
		b.WriteString("\n")
		written++
	}
	if written == 0 {
		return ""
	}
	return b.String()
}

// buildSessionFilesContextMessage renders the files uploaded or generated
// earlier in this conversation, so the model can reuse them by path instead
// of asking the user to re-upload (e.g. pass path straight into docx_edit /
// pptx_edit / xlsx_edit / image_generate as an argument).
func buildSessionFilesContextMessage(files []SessionFileRef) string {
	if len(files) == 0 {
		return ""
	}
	const maxFiles = 12
	var b strings.Builder
	b.WriteString("[本次对话已有文件]\n")
	b.WriteString("以下文件是用户在本次对话中上传的，或你此前调用工具生成的，均可直接复用。用户说“继续处理”“改一下”“基于刚才那个文件/图片”时，优先使用这里的 path 作为工具参数，不要要求用户重新上传或重新生成。\n")
	written := 0
	for _, f := range files {
		if written >= maxFiles {
			b.WriteString("- ...\n")
			break
		}
		name := strings.TrimSpace(f.Name)
		if name == "" {
			name = f.Path
		}
		kindLabel := "用户上传"
		if f.Kind == "generated" {
			kindLabel = "已生成"
		}
		b.WriteString(fmt.Sprintf("- %s（%s，path=%s）\n", name, kindLabel, f.Path))
		written++
	}
	if written == 0 {
		return ""
	}
	return b.String()
}
