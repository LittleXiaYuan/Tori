package planner

// executor_fc.go — Native function-calling execution engine.
// Handles tool call dispatch, parallel execution with safeToolGo,
// and result collection for the native LLM FC API path.

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	ctxwindow "yunque-agent/internal/agentcore/context"
	"yunque-agent/internal/agentcore/i18n"
	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/observe"
	"yunque-agent/pkg/skills"
)

// runNativeFC uses native LLM function calling (tool_calls in API response).
func (p *Planner) runNativeFC(ctx context.Context, req PlanRequest) (*PlanResult, error) {
	env := p.buildEnv(req)

	messages, ctxLayers := p.BuildMessages(ctx, req)
	userMsg := extractUserMessage(req)
	tools := p.buildFunctionDefs(userMsg, req.TenantID, req.ChannelType, req.DisableDelegation, req.AllowedSkills)
	p.maybeEmitCogniTrace(req)

	var usedSkills []string
	var planSteps []PlanStep
	steps := 0
	lastRecoveryFailedCount := 0

	for steps < p.maxSteps {
		steps++

		// Check for mid-execution interrupts between steps
		if shouldStop, extraMsgs := p.checkInterrupt(req, messages); shouldStop {
			return &PlanResult{Reply: i18n.T(p.locale, "planner.task_stopped"), SkillsUsed: usedSkills, Steps: steps, Plan: planSteps, ContextLayers: ctxLayers}, nil
		} else if len(extraMsgs) > 0 {
			messages = append(messages, extraMsgs...)
		}

		// Notify: thinking
		if req.StepCallback != nil {
			thinkEvt := observe.NewEvent(req.TraceID, observe.DomainPlanner, observe.EventThinking,
				fmt.Sprintf("正在思考 (第 %d 轮)...", steps))
			thinkEvt.Meta.TenantID = req.TenantID
			thinkEvt.Meta.TaskID = req.TaskID
			req.StepCallback(thinkEvt)
		}

		if steps == 1 {
			totalChars := 0
			for _, m := range messages {
				totalChars += len(m.Content)
			}
			slog.Info("planner: prompt stats", "msgs", len(messages), "total_chars", totalChars, "tools", len(tools), "step", steps)
		}
		// chatWithToolsFallback walks the pool's tier fallback chain
		// (request tier → expert → smart → fast → local → primary) and
		// honors session ClientOverride and capability-aware selection. A
		// session override short-circuits the chain inside clientForRequest.
		reply, toolCalls, lastReasoning, err := p.chatWithToolsFallback(ctx, req, messages, tools)
		if err != nil {
			if len(planSteps) > 0 {
				return p.partialPlanResult(req, planSteps, usedSkills, steps, ctxLayers, err.Error()), nil
			}
			return nil, fmt.Errorf("planner fc step %d: %w", steps, err)
		}

		if len(toolCalls) == 0 {
			cleaned := p.cleanReply(reply)
			if p.reflect != nil && steps < p.maxSteps {
				userIntent := ""
				if len(req.Messages) > 0 {
					userIntent = req.Messages[len(req.Messages)-1].Content
				}
				if !p.reflect(ctx, userIntent, cleaned) {
					slog.Info("planner: reflect unsatisfied, retrying", "step", steps)
					if req.StepCallback != nil {
						reflEvt := observe.NewEvent(req.TraceID, observe.DomainPlanner, observe.EventReflect,
							"🔄 回答质量不够好，正在重新思考...")
						reflEvt.Meta.TenantID = req.TenantID
						req.StepCallback(reflEvt)
					}
					messages = append(messages,
						llm.Message{Role: "assistant", Content: reply, ReasoningContent: lastReasoning},
						llm.Message{Role: "user", Content: i18n.T(p.locale, "planner.reflect_retry")},
					)
					continue
				}
			}
			cleaned, nextMoves := extractNextMoves(cleaned)
			return &PlanResult{Reply: cleaned, SkillsUsed: usedSkills, Steps: steps, Plan: planSteps, ContextLayers: ctxLayers, Suggestions: nextMoves}, nil
		}

		// Append assistant message with tool calls + reasoning (required by Kimi K2.5 etc.)
		messages = append(messages, llm.Message{Role: "assistant", Content: reply, ToolCalls: toolCalls, ReasoningContent: lastReasoning})

		// Execute tool calls in parallel
		type tcResult struct {
			idx    int
			id     string
			name   string
			args   map[string]any
			output string
			err    error
		}
		resultsCh := make(chan tcResult, len(toolCalls))
		for i, tc := range toolCalls {
			idx, tc := i, tc // capture loop vars

			// Handoff delegations and skill generation need longer timeout
			timeout := p.toolTimeout
			if p.handoffReg != nil {
				if _, isHandoff := p.handoffReg.IsHandoffCall(tc.Function.Name); isHandoff {
					timeout = 90 * time.Second
				}
			}
			if tc.Function.Name == "generate_skill" {
				timeout = 10 * time.Minute
			}

			toolParentCtx := ctx
			if tc.Function.Name == "generate_skill" {
				var gsCancel context.CancelFunc
				toolParentCtx, gsCancel = context.WithTimeout(context.Background(), 10*time.Minute)
				defer gsCancel()
			}

			go func(toolParentCtx context.Context, timeout time.Duration, idx int, tc llm.ToolCall) {
				defer func() {
					if r := recover(); r != nil {
						slog.Error("planner: tool goroutine panic", "panic", r, "skill", tc.Function.Name)
						resultsCh <- tcResult{idx: idx, id: tc.ID, name: tc.Function.Name, err: fmt.Errorf("tool panic: %v", r)}
					}
				}()
				var toolCtx context.Context
				if timeout <= 0 {
					toolCtx = toolParentCtx
				} else {
					var cancel context.CancelFunc
					toolCtx, cancel = context.WithTimeout(toolParentCtx, timeout)
					defer cancel()
				}
				func(toolCtx context.Context) {
					var args map[string]any
					json.Unmarshal([]byte(tc.Function.Arguments), &args)

					if p.handoffReg != nil && !req.DisableDelegation {
						if agentName, ok := p.handoffReg.IsHandoffCall(tc.Function.Name); ok {
							input, _ := args["input"].(string)
							slog.Info("planner: handoff delegation (fc)", "agent", agentName, "step", steps)

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
								p.skillMetrics(tc.Function.Name, dur, err)
							}
							p.proactiveCog.RecordExecutionFailure(err != nil)

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
								resultsCh <- tcResult{idx: idx, id: tc.ID, name: tc.Function.Name, args: args, err: err}
							} else {
								resultsCh <- tcResult{idx: idx, id: tc.ID, name: tc.Function.Name, args: args, output: hr.Reply}
							}
							return
						}
					}

					slog.Info("planner: executing skill (fc/parallel)", "skill", tc.Function.Name, "step", steps)
					if req.StepCallback != nil {
						tsEvt := observe.NewEvent(req.TraceID, observe.DomainPlanner, observe.EventToolStart,
							fmt.Sprintf("🔧 正在调用 [%s]...", tc.Function.Name))
						tsEvt.Meta.Skill = tc.Function.Name
						tsEvt.Detail = observe.ToolStartDetail{Skill: tc.Function.Name, Args: args}
						req.StepCallback(tsEvt)
					}
					exec := p.executeSkill(toolCtx, tc.Function.Name, args, env)
					if exec.Err != nil {
						resultsCh <- tcResult{idx: idx, id: tc.ID, name: exec.SkillName, args: exec.Args, err: exec.Err}
					} else {
						resultsCh <- tcResult{idx: idx, id: tc.ID, name: exec.SkillName, args: exec.Args, output: exec.Output}
					}
				}(toolCtx)
			}(toolParentCtx, timeout, idx, tc)
		}
		// Collect results in order
		tcResults := make([]tcResult, len(toolCalls))
		for range toolCalls {
			r := <-resultsCh
			tcResults[r.idx] = r
		}
		for _, r := range tcResults {
			usedSkills = append(usedSkills, r.name)
			step := PlanStep{
				ID:     len(planSteps) + 1,
				Action: fmt.Sprintf("调用 %s", r.name),
				Skill:  r.name,
				Args:   r.args,
				Status: StepDone,
				Result: r.output,
			}
			if r.err != nil {
				step.Status = StepFailed
				step.Error = r.err.Error()
				r.output = "暂未完成：" + plannerFriendlyFailureText(r.err.Error())
			}
			p.recordSkillRecommendationOutcome(r.name, r.err == nil)
			planSteps = append(planSteps, step)
			pruned := pruneToolResult(r.output, steps)
			messages = append(messages, buildToolResultMsg(r.id, pruned))

			// Notify: tool_result
			if req.StepCallback != nil {
				trSummary := fmt.Sprintf("✅ [%s] 完成", r.name)
				trErr := ""
				if r.err != nil {
					trSummary = fmt.Sprintf("⏸️ [%s] 暂未完成：%s", r.name, plannerFriendlyFailureText(r.err.Error()))
					trErr = plannerFriendlyFailureText(r.err.Error())
				}
				trEvt := observe.NewEvent(req.TraceID, observe.DomainPlanner, observe.EventToolResult, trSummary)
				trEvt.Meta.Skill = r.name
				trEvt.Detail = observe.ToolResultDetail{Skill: r.name, Result: truncate(r.output, 200), Error: trErr}
				req.StepCallback(trEvt)
			}
		}
		if summary, ok := buildPlannerFailureSummary(planSteps); ok && summary.FailedCount > lastRecoveryFailedCount {
			lastRecoveryFailedCount = summary.FailedCount
			p.maybeEmitFailureRecovery(req, summary)
			messages = append(messages, llm.Message{Role: "user", Content: formatFailureRecoveryPrompt(summary)})
		}

		// If the request context was cancelled (e.g. SSE disconnect) but tool
		// results are available, return them directly instead of calling the
		// LLM again (which would fail with "context canceled").
		if ctx.Err() != nil {
			if len(planSteps) > 0 {
				return p.partialPlanResult(req, planSteps, usedSkills, steps, ctxLayers, ctx.Err().Error()), nil
			}
			return &PlanResult{Reply: "连接暂时中断，现场已保留；如果任务已经推进，可以从最近可恢复任务继续。", SkillsUsed: usedSkills, Steps: steps, Plan: planSteps, ContextLayers: ctxLayers}, nil
		}
	}

	messages = append(messages, llm.Message{Role: "user", Content: "你已执行了足够多的步骤。请根据以上所有工具结果，直接给出最终回答。"})

	client := p.clientForRequest(req)
	reply, _, err := client.ChatWithTools(ctx, messages, tools, 0.7)
	if err != nil {
		if len(planSteps) > 0 {
			return p.partialPlanResult(req, planSteps, usedSkills, steps, ctxLayers, err.Error()), nil
		}
		return &PlanResult{Reply: "任务已执行 " + fmt.Sprintf("%d", steps) + " 步，现场已保留。", SkillsUsed: usedSkills, Steps: steps, Plan: planSteps, ContextLayers: ctxLayers}, nil
	}

	if len(usedSkills) > 0 {
		p.ensureSkillRuntime().RecordRecent(usedSkills)
	}

	return &PlanResult{Reply: p.cleanReply(reply), SkillsUsed: usedSkills, Steps: steps, Plan: planSteps, ContextLayers: ctxLayers}, nil
}

// buildFunctionDefs converts skill definitions to LLM FunctionDef format.
// In delegation mode (5+ handoff agents), only exposes transfer_to_* tools.
// Exec agents get domain-specific tools via their isolated RunFunc context.
// When disableDelegation is true, direct mode is forced (for subagent execution).
//
// allowedSkills, when non-empty, hard-restricts the skill universe to exactly
// those names before any further filtering. This is driven by the Cherry
// "tools" drawer: when a user explicitly checks a subset of skills, the
// planner is expected to stay inside that subset.
func (p *Planner) buildFunctionDefs(userMessage, tenantID, channelType string, disableDelegation bool, allowedSkills []string) []llm.FunctionDef {
	allSkills := p.registry.All()
	if len(allowedSkills) > 0 {
		allow := allowedSkillSet(allowedSkills)
		filtered := make([]skills.Skill, 0, len(allowedSkills))
		for _, s := range allSkills {
			if allow[s.Name()] {
				filtered = append(filtered, s)
			}
		}
		allSkills = filtered
		slog.Info("buildFunctionDefs: user-restricted skill set", "allowed", len(allowedSkills), "matched", len(allSkills))
	}

	// Filter out skills that aren't ready (missing config/dependencies)
	{
		ready := make([]skills.Skill, 0, len(allSkills))
		for _, s := range allSkills {
			if ok, reason := skills.IsReady(s); !ok {
				slog.Debug("buildFunctionDefs: skill not ready, excluding", "skill", s.Name(), "reason", reason)
				continue
			}
			ready = append(ready, s)
		}
		if excluded := len(allSkills) - len(ready); excluded > 0 {
			slog.Info("buildFunctionDefs: excluded unready skills", "count", excluded)
		}
		allSkills = ready
	}

	// Cogni surface filter — narrows the tool list to the union of every
	// activated cogni's ToolSurface. The hook returns the input unchanged
	// when no cogni activates, so the previous behaviour is preserved.
	if p.contextAssembly != nil && p.contextAssembly.HasCogniSkillFilter() && !disableDelegation && len(allowedSkills) == 0 {
		before := len(allSkills)
		allSkills = p.contextAssembly.FilterCogniSkills(userMessage, tenantID, channelType, allSkills)
		if after := len(allSkills); after != before {
			slog.Info("buildFunctionDefs: cogni surface filter applied",
				"before", before, "after", after, "msg_prefix", truncate(userMessage, 50))
		}
	}

	cats := p.registry.Categories()

	var catNames []string
	for _, c := range cats {
		catNames = append(catNames, fmt.Sprintf("%s(%d)", c.ID, len(c.SkillNames)))
	}

	// Delegation mode: planner only sees handoff tools + direct tools, exec agents handle the rest
	if !disableDelegation && p.handoffReg != nil && len(p.handoffReg.List()) >= 4 {
		hdDefs := p.handoffReg.ToolDefinitions()
		defs := make([]llm.FunctionDef, 0, len(hdDefs)+2)
		for _, hd := range hdDefs {
			fn, _ := hd["function"].(map[string]any)
			if fn == nil {
				continue
			}
			name, _ := fn["name"].(string)
			desc, _ := fn["description"].(string)
			params, _ := fn["parameters"].(map[string]any)
			defs = append(defs, llm.FunctionDef{Name: name, Description: desc, Parameters: params})
		}

		// Expose generate_skill directly in delegation mode so the planner
		// can create new skills on-demand without sub-agent delegation.
		directTools := []string{"generate_skill"}
		for _, toolName := range directTools {
			if sk, ok := p.registry.Get(toolName); ok {
				defs = append(defs, llm.FunctionDef{
					Name:        sk.Name(),
					Description: sk.Description(),
					Parameters:  sk.Parameters(),
				})
			}
		}

		slog.Info("buildFunctionDefs", "mode", "delegation", "handoff_tools", len(defs), "total_skills", len(allSkills), "msg_prefix", truncate(userMessage, 50))
		return defs
	}

	slog.Info("buildFunctionDefs", "total_skills", len(allSkills), "categories", len(cats), "cat_detail", strings.Join(catNames, ","), "msg_prefix", truncate(userMessage, 50))

	// Fallback: direct mode (no delegation agents or fewer than 4)
	// Strategy 1: Dynamic filtering by intent (threshold lowered from 25 to 10
	// so intent-based narrowing kicks in earlier, reducing tool noise for LLMs)
	if userMessage != "" && len(allSkills) > 10 && len(cats) > 0 && len(allowedSkills) == 0 {
		var skillScorer *skills.SkillScorer
		if p.skillRuntime != nil {
			skillScorer = p.skillRuntime.ScorerWithRecent()
		}
		filtered := p.registry.FilterByIntentScored(userMessage, skillScorer)
		if len(filtered) < len(allSkills) && len(filtered) > 0 {
			filtered = p.rankSkillsByRecommendation(userMessage, filtered)
			slog.Info("skill dynamic filter applied",
				"total", len(allSkills),
				"filtered", len(filtered),
				"message_prefix", truncate(userMessage, 50))
			defs := make([]llm.FunctionDef, 0, len(filtered))
			for _, s := range filtered {
				defs = append(defs, llm.FunctionDef{
					Name:        s.Name(),
					Description: s.Description(),
					Parameters:  s.Parameters(),
				})
			}
			if p.handoffReg != nil && !disableDelegation {
				for _, hd := range p.handoffReg.ToolDefinitions() {
					fn, _ := hd["function"].(map[string]any)
					if fn == nil {
						continue
					}
					name, _ := fn["name"].(string)
					desc, _ := fn["description"].(string)
					params, _ := fn["parameters"].(map[string]any)
					defs = append(defs, llm.FunctionDef{Name: name, Description: desc, Parameters: params})
				}
			}
			return defs
		}
	}

	allSkills = p.rankSkillsByRecommendation(userMessage, allSkills)

	defs := make([]llm.FunctionDef, 0, len(allSkills))
	for _, s := range allSkills {
		defs = append(defs, llm.FunctionDef{
			Name:        s.Name(),
			Description: s.Description(),
			Parameters:  s.Parameters(),
		})
	}

	if p.handoffReg != nil && !disableDelegation {
		for _, hd := range p.handoffReg.ToolDefinitions() {
			fn, _ := hd["function"].(map[string]any)
			if fn == nil {
				continue
			}
			name, _ := fn["name"].(string)
			desc, _ := fn["description"].(string)
			params, _ := fn["parameters"].(map[string]any)
			defs = append(defs, llm.FunctionDef{
				Name:        name,
				Description: desc,
				Parameters:  params,
			})
		}
	}

	return defs
}

func extractUserMessage(req PlanRequest) string {
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			return req.Messages[i].Content
		}
	}
	return ""
}

// pruneToolResult applies progressive compression to tool outputs.
// Later steps get more budget since they're closer to the final answer.
func pruneToolResult(output string, stepNum int) string {
	maxBytes := 12000
	switch {
	case stepNum <= 2:
		maxBytes = 8000
	case stepNum <= 5:
		maxBytes = 5000
	case stepNum <= 8:
		maxBytes = 3000
	default:
		maxBytes = 2000
	}
	if len(output) <= maxBytes {
		return output
	}
	return ctxwindow.PruneToolOutput(output, maxBytes)
}

// buildToolResultMsg creates a tool result message. If the output contains
// a "_screenshot_b64" field, it builds a multimodal message (text + image)
// so vision-capable models can "see" the page. Non-vision models auto-strip
// images via the existing stripImages fallback in functions.go.
func buildToolResultMsg(toolCallID, output string) llm.Message {
	var parsed map[string]string
	if json.Unmarshal([]byte(output), &parsed) == nil {
		if b64, ok := parsed["_screenshot_b64"]; ok && b64 != "" {
			text := parsed["text"]
			return llm.Message{
				Role:       "tool",
				ToolCallID: toolCallID,
				ContentParts: []llm.ContentPart{
					{Type: "text", Text: text},
					{Type: "image_url", ImageURL: &llm.MediaURL{URL: "data:image/jpeg;base64," + b64}},
				},
			}
		}
	}
	return llm.ToolResultMessage(toolCallID, output)
}
