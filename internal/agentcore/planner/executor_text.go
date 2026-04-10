package planner

// executor_text.go — Text-based skill execution fallback.
// Parses tool_calls JSON from free-form LLM output when native FC isn't
// available. For the native FC path, see executor_fc.go.

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/observe"
)

type skillCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"arguments"`
}

// parseSkillCalls extracts tool calls from free-form LLM text.
// Supports multiple formats that different models produce:
//   - {"tool_calls": [...]}   or  {"skill_calls": [...]}   (standard wrapper)
//   - <function_calls>\n{"name":"...", "arguments":"..."}   (Qwen-style)
func (p *Planner) parseSkillCalls(text string) []skillCall {
	// Strategy 1: standard {"tool_calls": [...]} wrapper
	if calls := p.parseWrappedCalls(text); len(calls) > 0 {
		return calls
	}

	// Strategy 2: Qwen-style <function_calls> tag with inline JSON objects
	if calls := p.parseFunctionCallsTag(text); len(calls) > 0 {
		return calls
	}

	// Strategy 3: bare skill_name followed by JSON object (no wrapper)
	// e.g. "docx_create\n{\"path\": \"...\", ...}"
	if calls := p.parseBareSkillCall(text); len(calls) > 0 {
		return calls
	}

	return nil
}

func (p *Planner) parseWrappedCalls(text string) []skillCall {
	idx := strings.Index(text, `"tool_calls"`)
	if idx < 0 {
		idx = strings.Index(text, `"skill_calls"`)
	}
	if idx < 0 {
		return nil
	}
	start := strings.LastIndex(text[:idx], "{")
	if start < 0 {
		return nil
	}
	end := findClosingBrace(text, start)
	if end < 0 {
		return nil
	}
	var wrapper struct {
		ToolCalls  []skillCall `json:"tool_calls"`
		SkillCalls []skillCall `json:"skill_calls"`
	}
	if err := json.Unmarshal([]byte(text[start:end+1]), &wrapper); err != nil {
		return nil
	}
	if len(wrapper.ToolCalls) > 0 {
		return wrapper.ToolCalls
	}
	return wrapper.SkillCalls
}

// parseFunctionCallsTag handles the <function_calls> format produced by Qwen
// and similar models: the body contains one or more JSON objects with "name"
// and "arguments" (the latter is a JSON-encoded string of the actual args).
func (p *Planner) parseFunctionCallsTag(text string) []skillCall {
	const openTag = "<function_calls>"
	const closeTag = "</function_calls>"
	start := strings.Index(text, openTag)
	if start < 0 {
		return nil
	}
	body := text[start+len(openTag):]
	if end := strings.Index(body, closeTag); end >= 0 {
		body = body[:end]
	}
	body = strings.TrimSpace(body)
	if body == "" {
		return nil
	}

	var calls []skillCall
	for len(body) > 0 {
		idx := strings.Index(body, "{")
		if idx < 0 {
			break
		}
		end := findClosingBrace(body, idx)
		if end < 0 {
			break
		}
		chunk := body[idx : end+1]
		body = body[end+1:]

		var raw struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal([]byte(chunk), &raw); err != nil || raw.Name == "" {
			continue
		}

		args := make(map[string]any)
		// "arguments" may be a JSON string (double-encoded) or a direct object.
		var argsStr string
		if json.Unmarshal(raw.Arguments, &argsStr) == nil {
			_ = json.Unmarshal([]byte(argsStr), &args)
		} else {
			_ = json.Unmarshal(raw.Arguments, &args)
		}
		calls = append(calls, skillCall{Name: raw.Name, Args: args})
	}
	return calls
}

// parseBareSkillCall detects a known skill name on its own line followed by
// a JSON object.  Example:
//
//	docx_create
//	{"path":"data/output/report.docx","title":"Report","content":"..."}
//
// This pattern appears when the LLM outputs a tool call as plain text without
// any wrapper format.
func (p *Planner) parseBareSkillCall(text string) []skillCall {
	if p.registry == nil {
		return nil
	}
	lines := strings.Split(text, "\n")
	var calls []skillCall
	for i := 0; i < len(lines); i++ {
		name := strings.TrimSpace(lines[i])
		if name == "" || strings.ContainsAny(name, " \t{}[]()\"'<>") {
			continue
		}
		resolvedName := name
		if _, ok := p.registry.Get(name); !ok {
			resolvedName = p.fuzzyMatchSkill(name)
			if resolvedName == "" {
				continue
			}
			slog.Info("planner: fuzzy matched skill name", "raw", name, "resolved", resolvedName)
		}
		// Found a known skill name; look for a JSON object in subsequent lines.
		jsonStart := -1
		for j := i + 1; j < len(lines); j++ {
			trimmed := strings.TrimSpace(lines[j])
			if trimmed == "" {
				continue
			}
			if trimmed[0] == '{' {
				jsonStart = j
				break
			}
			break
		}
		if jsonStart < 0 {
			continue
		}
		remaining := strings.Join(lines[jsonStart:], "\n")
		braceEnd := findClosingBrace(remaining, 0)
		if braceEnd < 0 {
			continue
		}
		chunk := remaining[:braceEnd+1]
		var args map[string]any
		if err := json.Unmarshal([]byte(chunk), &args); err != nil {
			continue
		}
		calls = append(calls, skillCall{Name: resolvedName, Args: args})
		slog.Info("planner: parsed bare skill call from text", "skill", resolvedName, "raw", name)
	}
	return calls
}

// fuzzyMatchSkill attempts to match a raw tool name to a registered skill.
// Rules: "docx" → "docx_create", "xlsx" → "xlsx_create", "search" → "web_search", etc.
func (p *Planner) fuzzyMatchSkill(raw string) string {
	if p.registry == nil {
		return ""
	}
	lower := strings.ToLower(raw)

	// Strategy 1: exact match with common suffixes
	for _, suffix := range []string{"_create", "_edit", "_fill"} {
		candidate := lower + suffix
		if _, ok := p.registry.Get(candidate); ok {
			return candidate
		}
	}

	// Strategy 2: registered skill that starts with the raw name
	var match string
	for _, sk := range p.registry.All() {
		skName := strings.ToLower(sk.Name())
		if strings.HasPrefix(skName, lower+"_") || strings.HasPrefix(skName, lower) {
			if match == "" || len(skName) < len(match) {
				match = sk.Name()
			}
		}
	}
	return match
}

// runTextBased: multi-step planning loop using text-parsed skill calls.
// Decompose → Execute (parallel) → Reflect → Synthesize.
func (p *Planner) runTextBased(ctx context.Context, req PlanRequest) (*PlanResult, error) {
	env := p.buildEnv(req)

	messages, ctxLayers := p.BuildMessages(ctx, req)

	var usedSkills []string
	var planSteps []PlanStep
	steps := 0

	for steps < p.maxSteps {
		steps++

		// Check for mid-execution interrupts between steps
		if shouldStop, extraMsgs := p.checkInterrupt(req, messages); shouldStop {
			return &PlanResult{Reply: "已停止当前任务。", SkillsUsed: usedSkills, Steps: steps, Plan: planSteps, ContextLayers: ctxLayers}, nil
		} else if len(extraMsgs) > 0 {
			messages = append(messages, extraMsgs...)
		}

		reply, err := p.chatFallback(ctx, req, messages)
		if err != nil {
			return nil, fmt.Errorf("planner step %d: %w", steps, err)
		}

		calls := p.parseSkillCalls(reply)
		if len(calls) == 0 {
			cleaned := p.cleanReply(reply)

			if p.reflect != nil && steps < p.maxSteps {
				userIntent := ""
				if len(req.Messages) > 0 {
					userIntent = req.Messages[len(req.Messages)-1].Content
				}
				if !p.reflect(ctx, userIntent, cleaned) {
					slog.Info("planner: reflect unsatisfied, retrying", "step", steps)
					messages = append(messages,
						llm.Message{Role: "assistant", Content: reply},
						llm.Message{Role: "user", Content: "你的回答质量不够好，请重新组织更完善的回答。"},
					)
					continue
				}
			}

			return &PlanResult{Reply: cleaned, SkillsUsed: usedSkills, Steps: steps, Plan: planSteps, ContextLayers: ctxLayers}, nil
		}

		// Execute tool calls in parallel
		type callResult struct {
			idx    int
			name   string
			output string
			err    error
		}
		ch := make(chan callResult, len(calls))
		for i, call := range calls {
			idx, c := i, call // capture loop vars
			safeToolGo(ctx, p.toolTimeout, func(toolCtx context.Context) {
				// Check for handoff (transfer_to_*) calls first
				if p.handoffReg != nil {
					if agentName, ok := p.handoffReg.IsHandoffCall(c.Name); ok {
						input, _ := c.Args["input"].(string)
						if input == "" {
							// Fallback: use first string arg as input
							for _, v := range c.Args {
								if s, ok := v.(string); ok && s != "" {
									input = s
									break
								}
							}
						}
						slog.Info("planner: handoff delegation (text)", "agent", agentName, "step", steps)
						t0 := time.Now()
						hr, err := p.handoffReg.Execute(toolCtx, req.TenantID, agentName, input)
						dur := time.Since(t0)
						if p.skillMetrics != nil {
							p.skillMetrics(c.Name, dur, err)
						}
						if p.taskFailureMon != nil {
							p.taskFailureMon.Record(err != nil)
						}
						if err != nil {
							ch <- callResult{idx: idx, name: c.Name, err: err}
						} else {
							ch <- callResult{idx: idx, name: c.Name, output: hr.Reply}
						}
						return
					}
				}

				skill, ok := p.registry.Get(c.Name)
				if !ok {
					ch <- callResult{idx: idx, name: c.Name, output: fmt.Sprintf("未知技能: %s", c.Name)}
					return
				}
				slog.Info("planner: executing skill", "skill", c.Name, "step", steps, "parallel", len(calls) > 1)
				t0 := time.Now()
				r, err := skill.Execute(toolCtx, c.Args, env)
				dur := time.Since(t0)
				if p.skillMetrics != nil {
					p.skillMetrics(c.Name, dur, err)
				}
				if p.taskFailureMon != nil {
					p.taskFailureMon.Record(err != nil)
				}
				if err != nil {
					ch <- callResult{idx: idx, name: c.Name, err: err}
				} else {
					ch <- callResult{idx: idx, name: c.Name, output: r}
				}
			})
		}

		// Collect results preserving order
		ordered := make([]callResult, len(calls))
		for range calls {
			r := <-ch
			ordered[r.idx] = r
		}

		var results []string
		for i, r := range ordered {
			usedSkills = append(usedSkills, r.name)
			step := PlanStep{
				ID:     len(planSteps) + 1,
				Action: fmt.Sprintf("调用 %s", r.name),
				Skill:  r.name,
				Args:   calls[i].Args,
				Status: StepDone,
				Result: r.output,
			}
			if r.err != nil {
				step.Status = StepFailed
				step.Error = r.err.Error()
				results = append(results, fmt.Sprintf("[%s] 执行失败: %v", r.name, r.err))
			} else {
				results = append(results, fmt.Sprintf("[%s] %s", r.name, r.output))
			}
			planSteps = append(planSteps, step)
		}

		// Build reflection prompt: show results and ask LLM to assess + continue
		reflectPrompt := "工具调用结果:\n" + strings.Join(results, "\n\n")
		if steps < p.maxSteps-1 && len(planSteps) > 0 {
			reflectPrompt += "\n\n请评估以上结果：如果信息充足，直接给出最终回答；如果还需要更多信息，继续调用工具。"
		}

		messages = append(messages,
			llm.Message{Role: "assistant", Content: reply},
			llm.Message{Role: "user", Content: reflectPrompt},
		)
	}

	var streamCB llm.StreamDeltaFunc
	if req.StepCallback != nil {
		streamCB = func(contentDelta, reasoningDelta string) {
			if reasoningDelta != "" {
				evt := observe.NewEvent(req.TraceID, observe.DomainPlanner, observe.EventThinking, reasoningDelta)
				evt.Meta.TenantID = req.TenantID
				evt.Detail = map[string]string{"stream_type": "thinking_delta"}
				req.StepCallback(evt)
			}
		}
	}
	finalResult, err := p.chatFallbackFull(ctx, req, messages, streamCB)
	if err != nil {
		return nil, fmt.Errorf("planner final: %w", err)
	}
	return &PlanResult{Reply: p.cleanReply(finalResult.Content), ReasoningContent: finalResult.ReasoningContent, SkillsUsed: usedSkills, Steps: steps, Plan: planSteps, ContextLayers: ctxLayers}, nil
}
