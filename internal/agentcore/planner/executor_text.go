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
	idx := firstCallMarkerIndex(text, `"tool_calls"`, `"skill_calls"`, `"tool_call"`, `"function_call"`)
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
		ToolCalls    []json.RawMessage `json:"tool_calls"`
		SkillCalls   []json.RawMessage `json:"skill_calls"`
		ToolCall     json.RawMessage   `json:"tool_call"`
		FunctionCall json.RawMessage   `json:"function_call"`
	}
	if err := json.Unmarshal([]byte(text[start:end+1]), &wrapper); err != nil {
		return nil
	}
	if len(wrapper.ToolCalls) > 0 {
		return parseRawSkillCalls(wrapper.ToolCalls)
	}
	if len(wrapper.SkillCalls) > 0 {
		return parseRawSkillCalls(wrapper.SkillCalls)
	}
	if len(wrapper.ToolCall) > 0 && string(wrapper.ToolCall) != "null" {
		if call, ok := parseRawSkillCall(wrapper.ToolCall); ok {
			return []skillCall{call}
		}
	}
	if len(wrapper.FunctionCall) > 0 && string(wrapper.FunctionCall) != "null" {
		if call, ok := parseRawSkillCall(wrapper.FunctionCall); ok {
			return []skillCall{call}
		}
	}
	return nil
}

func firstCallMarkerIndex(text string, markers ...string) int {
	best := -1
	for _, marker := range markers {
		idx := strings.Index(text, marker)
		if idx >= 0 && (best < 0 || idx < best) {
			best = idx
		}
	}
	return best
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

		call, ok := parseRawSkillCall(json.RawMessage(chunk))
		if !ok {
			continue
		}
		calls = append(calls, call)
	}
	return calls
}

func parseRawSkillCalls(rawCalls []json.RawMessage) []skillCall {
	calls := make([]skillCall, 0, len(rawCalls))
	for _, raw := range rawCalls {
		call, ok := parseRawSkillCall(raw)
		if ok {
			calls = append(calls, call)
		}
	}
	return calls
}

func parseRawSkillCall(raw json.RawMessage) (skillCall, bool) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return skillCall{}, false
	}

	name := rawJSONString(obj["name"])
	argsRaw := firstRaw(obj["arguments"], obj["args"])
	if fnRaw, ok := obj["function"]; ok {
		var fn map[string]json.RawMessage
		if json.Unmarshal(fnRaw, &fn) == nil {
			if fnName := rawJSONString(fn["name"]); fnName != "" {
				name = fnName
			}
			if len(argsRaw) == 0 {
				argsRaw = firstRaw(fn["arguments"], fn["args"])
			}
		}
	}

	name = strings.TrimSpace(name)
	if name == "" {
		return skillCall{}, false
	}
	return skillCall{Name: name, Args: parseSkillCallArgs(argsRaw)}, true
}

func firstRaw(values ...json.RawMessage) json.RawMessage {
	for _, value := range values {
		if len(value) > 0 && string(value) != "null" {
			return value
		}
	}
	return nil
}

func rawJSONString(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return ""
}

func parseSkillCallArgs(raw json.RawMessage) map[string]any {
	args := map[string]any{}
	if len(raw) == 0 || string(raw) == "null" {
		return args
	}
	if err := json.Unmarshal(raw, &args); err == nil && args != nil {
		return args
	}
	var argsStr string
	if err := json.Unmarshal(raw, &argsStr); err == nil {
		argsStr = strings.TrimSpace(argsStr)
		if argsStr == "" {
			return args
		}
		if err := json.Unmarshal([]byte(argsStr), &args); err == nil && args != nil {
			return args
		}
		return map[string]any{"input": argsStr}
	}
	return args
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
	p.maybeEmitCogniTrace(req)

	var usedSkills []string
	var planSteps []PlanStep
	steps := 0
	lastRecoveryFailedCount := 0

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
			if len(planSteps) > 0 {
				return p.partialPlanResult(req, planSteps, usedSkills, steps, ctxLayers, err.Error()), nil
			}
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

			cleaned, nextMoves := extractNextMoves(cleaned)
			return &PlanResult{Reply: cleaned, SkillsUsed: usedSkills, Steps: steps, Plan: planSteps, ContextLayers: ctxLayers, Suggestions: nextMoves}, nil
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
			timeout := p.toolTimeout
			if p.handoffReg != nil {
				if _, isHandoff := p.handoffReg.IsHandoffCall(c.Name); isHandoff {
					timeout = 90 * time.Second
				}
			}
			safeToolGo(ctx, timeout, func(toolCtx context.Context) {
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
						if req.StepCallback != nil {
							evt := observe.NewEvent(req.TraceID, observe.DomainAgent, observe.EventHandoffStart,
								fmt.Sprintf("🤖 委派 [%s]：%s", agentName, truncate(input, 80)))
							evt.Meta.TenantID = req.TenantID
							evt.Meta.Skill = agentName
							evt.Detail = observe.HandoffDetail{Agent: agentName, Input: truncate(input, 200)}
							req.StepCallback(evt)
						}
						cbCtx := toolCtx
						if req.StepCallback != nil {
							cbCtx = WithStepCallback(toolCtx, req.StepCallback)
						}
						t0 := time.Now()
						hr, err := p.handoffReg.Execute(cbCtx, req.TenantID, agentName, input, req.ModelOverride)
						dur := time.Since(t0)
						if p.skillMetrics != nil {
							p.skillMetrics(c.Name, dur, err)
						}
						if p.taskFailureMon != nil {
							p.taskFailureMon.Record(err != nil)
						}
						if req.StepCallback != nil {
							doneEvt := observe.NewEvent(req.TraceID, observe.DomainAgent, observe.EventHandoffDone,
								fmt.Sprintf("✅ [%s] 完成 (%.1fs)", agentName, dur.Seconds()))
							doneEvt.Meta.TenantID = req.TenantID
							doneEvt.Meta.Skill = agentName
							detail := observe.HandoffDetail{Agent: agentName, DurMs: dur.Milliseconds()}
							if err != nil {
								doneEvt.Summary = handoffFailureSummary(agentName, err)
								detail = buildHandoffFailureDetail(agentName, dur, err)
							} else {
								detail.Reply = truncate(hr.Reply, 200)
							}
							doneEvt.Detail = detail
							req.StepCallback(doneEvt)
						}
						if err != nil {
							ch <- callResult{idx: idx, name: c.Name, err: err}
						} else {
							ch <- callResult{idx: idx, name: c.Name, output: hr.Reply}
						}
						return
					}
				}

				slog.Info("planner: executing skill", "skill", c.Name, "step", steps, "parallel", len(calls) > 1)
				if req.StepCallback != nil {
					tsEvt := observe.NewEvent(req.TraceID, observe.DomainPlanner, observe.EventToolStart,
						fmt.Sprintf("🔧 正在调用 [%s]...", c.Name))
					tsEvt.Meta.Skill = c.Name
					tsEvt.Meta.TenantID = req.TenantID
					tsEvt.Detail = observe.ToolStartDetail{Skill: c.Name, Args: c.Args}
					req.StepCallback(tsEvt)
				}
				exec := p.executeSkill(toolCtx, c.Name, c.Args, env)
				if exec.Err != nil {
					ch <- callResult{idx: idx, name: exec.SkillName, err: exec.Err}
				} else {
					ch <- callResult{idx: idx, name: exec.SkillName, output: exec.Output}
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
				results = append(results, fmt.Sprintf("[%s] 暂未完成：%s", r.name, plannerFriendlyFailureText(r.err.Error())))
			} else {
				results = append(results, fmt.Sprintf("[%s] %s", r.name, r.output))
			}
			planSteps = append(planSteps, step)
			if req.StepCallback != nil {
				trSummary := fmt.Sprintf("✅ [%s] 完成", r.name)
				detail := observe.ToolResultDetail{Skill: r.name, Result: truncate(r.output, 200)}
				if r.err != nil {
					friendly := plannerFriendlyFailureText(r.err.Error())
					trSummary = fmt.Sprintf("⏸️ [%s] 暂未完成：%s", r.name, friendly)
					detail = observe.ToolResultDetail{Skill: r.name, Error: friendly}
				}
				trEvt := observe.NewEvent(req.TraceID, observe.DomainPlanner, observe.EventToolResult, trSummary)
				trEvt.Meta.Skill = r.name
				trEvt.Meta.TenantID = req.TenantID
				trEvt.Detail = detail
				req.StepCallback(trEvt)
			}
		}

		// Build reflection prompt: show results and ask LLM to assess + continue
		reflectPrompt := "工具调用结果:\n" + strings.Join(results, "\n\n")
		if summary, ok := buildPlannerFailureSummary(planSteps); ok && summary.FailedCount > lastRecoveryFailedCount {
			lastRecoveryFailedCount = summary.FailedCount
			p.maybeEmitFailureRecovery(req, summary)
			reflectPrompt += "\n\n" + formatFailureRecoveryPrompt(summary)
		}
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
		if len(planSteps) > 0 {
			return p.partialPlanResult(req, planSteps, usedSkills, steps, ctxLayers, err.Error()), nil
		}
		return nil, fmt.Errorf("planner final: %w", err)
	}
	return &PlanResult{Reply: p.cleanReply(finalResult.Content), ReasoningContent: finalResult.ReasoningContent, SkillsUsed: usedSkills, Steps: steps, Plan: planSteps, ContextLayers: ctxLayers}, nil
}
