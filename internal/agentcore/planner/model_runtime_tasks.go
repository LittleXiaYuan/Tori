package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/plan"
	"yunque-agent/pkg/jsonutil"
)

// MissionParseResult is the structured intent returned from NL mission parsing.
type MissionParseResult struct {
	Type        string         `json:"type"`        // "task" | "workflow" | "cron" | "trigger"
	Name        string         `json:"name"`        // suggested mission name
	Description string         `json:"description"` // cleaned description
	Config      map[string]any `json:"config"`      // type-specific config (cron_expr, event_type, steps, etc.)
	Confidence  float64        `json:"confidence"`  // 0-1 how confident the parse is
	Explanation string         `json:"explanation"` // why this type was chosen
}

const missionParsePrompt = `You are a mission intent classifier. Given a user's natural language description,
determine what type of automated mission to create and extract structured parameters.

Respond ONLY with a JSON object (no markdown, no explanation outside JSON):
{
  "type": "task" | "workflow" | "cron" | "trigger",
  "name": "concise mission name in the user's language",
  "description": "cleaned one-line description",
  "config": {
    // For "cron": include "cron_expr" (standard 5-field cron), "message" (what to send to agent)
    // For "trigger": include "event_type", "condition", "action_type"
    // For "workflow": include "steps" (array of step descriptions)
    // For "task": include "goal" (task goal)
  },
  "confidence": 0.0-1.0,
  "explanation": "one sentence explaining the classification"
}

Rules:
- If user mentions time/schedule/daily/weekly/hourly → "cron"
- If user mentions "when X happens" / event-driven / condition → "trigger"
- If user mentions multi-step / pipeline / flow / DAG → "workflow"
- Otherwise → "task" (one-off agent task)
- For cron, always generate a valid 5-field cron expression
- Confidence < 0.5 means you're unsure — the user should verify`

// GenerateConversationTitle uses the fast model tier to produce a compact session title.
func (s *ModelRuntimeService) GenerateConversationTitle(ctx context.Context, userMsg, assistReply string) string {
	if s == nil {
		return ""
	}
	userMsg = clipRunes(userMsg, 300)
	assistReply = clipRunes(assistReply, 300)

	msgs := []llm.Message{
		{Role: "system", Content: "你是一个对话标题生成器。根据用户的第一条消息和助手的回复，生成一个简短的对话标题（5-15个字）。只输出标题文本，不要加引号、标点或解释。"},
		{Role: "user", Content: fmt.Sprintf("用户消息：%s\n助手回复：%s", userMsg, assistReply)},
	}

	title, err := s.ChatForRequestTier(ctx, PlanRequest{}, "fast", msgs, 0.3)
	if err != nil {
		slog.Debug("model runtime: auto-title generation failed", "err", err)
		return ""
	}

	title = strings.TrimSpace(title)
	title = strings.Trim(title, "\"'「」《》【】")
	title = strings.TrimSpace(title)
	if len([]rune(title)) > 30 {
		title = string([]rune(title)[:30])
	}
	return title
}

// StarterSuggestion is one personalized chat opener shown on the empty chat
// screen: a short chip label plus the full prompt sent when it is tapped.
type StarterSuggestion struct {
	Label  string `json:"label"`  // short chip text, 2-6 chars ideally
	Prompt string `json:"prompt"` // full prompt sent on click
}

const starterSuggestionsPrompt = `你为「云雀」AI 工作助手生成对话首页的开场建议。
根据用户近期对话、已安装能力（Pack）和当前时段，给出 4 条**贴合该用户**的开场建议。

只输出 JSON 数组，不要 markdown、不要解释：
[{"label":"简短动词短语(2-6字)","prompt":"用户视角的第一句话，可直接发送(15-40字)"}]

规则：
- label 是按钮上的短词（如「写周报」「查资料」「改代码」），prompt 是点击后真正发送的完整请求
- 紧扣用户近期在做的事；若信息不足，给通用但实用的工作场景，不要空泛寒暄
- prompt 用第一人称、命令式中文，像用户自己打的字
- 必须正好 4 条，label 互不重复`

// GenerateStarterSuggestions uses the fast model tier to produce personalized
// empty-screen chat openers from lightweight profile context (recent
// conversation titles/summaries, installed pack names, time of day). Returns a
// non-nil error on any failure so callers can fall back to a curated static set.
func (s *ModelRuntimeService) GenerateStarterSuggestions(ctx context.Context, profile string) ([]StarterSuggestion, error) {
	if s == nil {
		return nil, fmt.Errorf("model runtime not configured")
	}
	msgs := []llm.Message{
		{Role: "system", Content: starterSuggestionsPrompt},
		{Role: "user", Content: clipRunes(profile, 1600)},
	}
	reply, err := s.ChatForRequestTier(ctx, PlanRequest{}, "fast", msgs, 0.5)
	if err != nil {
		return nil, fmt.Errorf("LLM call failed: %w", err)
	}

	var out []StarterSuggestion
	if err := json.Unmarshal([]byte(reply), &out); err != nil {
		cleaned := jsonutil.Extract(reply)
		if err2 := json.Unmarshal([]byte(cleaned), &out); err2 != nil {
			slog.Debug("model runtime: failed to parse starter suggestions JSON", "raw", reply, "err", err2)
			return nil, fmt.Errorf("parse suggestions: %w", err2)
		}
	}

	// Sanitize: trim, drop empties, clip lengths, cap at 4.
	cleaned := out[:0]
	for _, sug := range out {
		sug.Label = strings.TrimSpace(sug.Label)
		sug.Prompt = strings.TrimSpace(sug.Prompt)
		if sug.Label == "" || sug.Prompt == "" {
			continue
		}
		sug.Label = clipRunes(sug.Label, 12)
		sug.Prompt = clipRunes(sug.Prompt, 200)
		cleaned = append(cleaned, sug)
		if len(cleaned) >= 4 {
			break
		}
	}
	if len(cleaned) == 0 {
		return nil, fmt.Errorf("no usable suggestions in model reply")
	}
	return cleaned, nil
}

// ParseMissionIntent classifies a natural language description into a structured mission intent.
func (s *ModelRuntimeService) ParseMissionIntent(ctx context.Context, description string) (MissionParseResult, error) {
	messages := []llm.Message{
		{Role: "system", Content: missionParsePrompt},
		{Role: "user", Content: description},
	}

	reply, err := s.ChatForRequestTier(ctx, PlanRequest{}, "fast", messages, 0.3)
	if err != nil {
		return MissionParseResult{}, fmt.Errorf("LLM call failed: %w", err)
	}

	var result MissionParseResult
	if err := json.Unmarshal([]byte(reply), &result); err != nil {
		cleaned := jsonutil.Extract(reply)
		if err2 := json.Unmarshal([]byte(cleaned), &result); err2 != nil {
			slog.Warn("model runtime: failed to parse mission JSON", "raw", reply, "err", err2)
			result = MissionParseResult{
				Type:        "task",
				Name:        description,
				Description: description,
				Config:      map[string]any{"goal": description},
				Confidence:  0.3,
				Explanation: "Could not reliably classify — defaulting to one-off task.",
			}
		}
	}
	return result, nil
}

// DecomposeLongHorizonDAG asks the model runtime to turn a goal into a bounded
// DAG. Planner owns orchestration; model runtime owns the long-horizon prompt
// and request-level LLM call for decomposition.
func (s *ModelRuntimeService) DecomposeLongHorizonDAG(ctx context.Context, req PlanRequest, skillList, goal string) ([]plan.PlanStep, error) {
	prompt := fmt.Sprintf(`将目标分解为步骤。可用工具：
%s
目标：%s
返回 JSON 数组：[{"description":"","skill":"","args":{},"depends_on":[]}]
规则：独立步骤不加依赖，3-8步，只返回JSON`, skillList, goal)

	reply, err := s.ChatForRequest(ctx, req, []llm.Message{
		{Role: "system", Content: "你是任务规划器，只输出 JSON 数组。"},
		{Role: "user", Content: prompt},
	}, 0.3)
	if err != nil {
		return nil, err
	}
	steps, err := parseDAGSteps(reply)
	if err != nil {
		return nil, err
	}
	return ensureInitialDAGMinimumSteps(steps), nil
}

// ReviseLongHorizonDAG asks the model runtime to re-plan the remaining DAG
// after a failed step. Planner supplies the current plan snapshot and owns the
// retry lifecycle; model runtime owns the prompt and LLM call shape.
func (s *ModelRuntimeService) ReviseLongHorizonDAG(ctx context.Context, req PlanRequest, goal, status string, failedStep int) ([]plan.PlanStep, error) {
	prompt := fmt.Sprintf("任务: %s\n状态:\n%s\n步骤 %d 失败，重新规划剩余部分。返回JSON数组。",
		goal, status, failedStep)
	reply, err := s.ChatForRequest(ctx, req, []llm.Message{
		{Role: "system", Content: "你是任务规划器，根据失败提出替代方案，只输出JSON数组。"},
		{Role: "user", Content: prompt},
	}, 0.4)
	if err != nil {
		return nil, err
	}
	return parseDAGSteps(reply)
}

// ExecuteLongHorizonReasoningStep runs a reasoning-only DAG step through the
// model runtime. Skill steps still execute through the execution runtime.
func (s *ModelRuntimeService) ExecuteLongHorizonReasoningStep(ctx context.Context, req PlanRequest, tier, prompt string) (string, error) {
	return s.ChatForRequestTier(ctx, req, tier, []llm.Message{
		{Role: "system", Content: "基于信息完成分析，直接给出结果。"},
		{Role: "user", Content: prompt},
	}, 0.7)
}

// SynthesizeLongHorizonResult turns completed DAG outputs into a final reply.
// If the model path is unavailable, callers can use the returned fallback text.
func (s *ModelRuntimeService) SynthesizeLongHorizonResult(ctx context.Context, req PlanRequest, goal, results string) (string, error) {
	return s.ChatForRequest(ctx, req, []llm.Message{
		{Role: "system", Content: "根据执行结果给出完整回复。Markdown格式。"},
		{Role: "user", Content: fmt.Sprintf("目标: %s\n结果:\n%s", goal, results)},
	}, 0.7)
}

func clipRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}
