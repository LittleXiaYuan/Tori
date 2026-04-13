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
	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/observe"
)

// runNativeFC uses native LLM function calling (tool_calls in API response).
func (p *Planner) runNativeFC(ctx context.Context, req PlanRequest) (*PlanResult, error) {
	env := p.buildEnv(req)

	messages, ctxLayers := p.BuildMessages(ctx, req)
	userMsg := extractUserMessage(req)
	tools := p.buildFunctionDefs(userMsg, req.DisableDelegation)

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

		// Notify: thinking
		if req.StepCallback != nil {
			thinkEvt := observe.NewEvent(req.TraceID, observe.DomainPlanner, observe.EventThinking,
				fmt.Sprintf("正在思考 (第 %d 轮)...", steps))
			thinkEvt.Meta.TenantID = req.TenantID
			thinkEvt.Meta.TaskID = req.TaskID
			req.StepCallback(thinkEvt)
		}

		client := p.clientForRequest(req)
		var lastReasoning string
		opts := &llm.ChatWithToolsOpts{LastReasoningOut: &lastReasoning}
		reply, toolCalls, err := client.ChatWithToolsEx(ctx, messages, tools, 0.7, opts)
		if err != nil {
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
						llm.Message{Role: "user", Content: "你的回答质量不够好，请重新组织更完善的回答。"},
					)
					continue
				}
			}
			return &PlanResult{Reply: cleaned, SkillsUsed: usedSkills, Steps: steps, Plan: planSteps, ContextLayers: ctxLayers}, nil
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

			// Handoff delegations need longer timeout (browser ops, code exec, etc.)
			timeout := p.toolTimeout
			if p.handoffReg != nil {
				if _, isHandoff := p.handoffReg.IsHandoffCall(tc.Function.Name); isHandoff {
					timeout = 3 * time.Minute
				}
			}

			safeToolGo(ctx, timeout, func(toolCtx context.Context) {
				var args map[string]any
				json.Unmarshal([]byte(tc.Function.Arguments), &args)

				// Check for handoff (transfer_to_*) calls first
				if p.handoffReg != nil {
					if agentName, ok := p.handoffReg.IsHandoffCall(tc.Function.Name); ok {
						input, _ := args["input"].(string)
						slog.Info("planner: handoff delegation (fc)", "agent", agentName, "step", steps)
						t0 := time.Now()
						hr, err := p.handoffReg.Execute(toolCtx, req.TenantID, agentName, input)
						dur := time.Since(t0)
						if p.skillMetrics != nil {
							p.skillMetrics(tc.Function.Name, dur, err)
						}
						if p.taskFailureMon != nil {
							p.taskFailureMon.Record(err != nil)
						}
						if err != nil {
							resultsCh <- tcResult{idx: idx, id: tc.ID, name: tc.Function.Name, args: args, err: err}
						} else {
							resultsCh <- tcResult{idx: idx, id: tc.ID, name: tc.Function.Name, args: args, output: hr.Reply}
						}
						return
					}
				}

				skill, ok := p.registry.Get(tc.Function.Name)
				if !ok {
					// Resolve hierarchical meta-tool: use_browser{action:"browser_navigate", args:{...}} → browser_navigate(args)
					if strings.HasPrefix(tc.Function.Name, "use_") {
						actionName, _ := args["action"].(string)
						innerArgs, _ := args["args"].(map[string]any)
						if actionName != "" {
							if realSkill, found := p.registry.Get(actionName); found {
								skill = realSkill
								ok = true
								if innerArgs != nil {
									args = innerArgs
								}
							}
						}
					}
				}
				if !ok {
					resultsCh <- tcResult{idx: idx, id: tc.ID, name: tc.Function.Name, args: args, output: fmt.Sprintf("未知技能: %s", tc.Function.Name)}
					return
				}
				slog.Info("planner: executing skill (fc/parallel)", "skill", tc.Function.Name, "step", steps)
				// Notify: tool_start
				if req.StepCallback != nil {
					tsEvt := observe.NewEvent(req.TraceID, observe.DomainPlanner, observe.EventToolStart,
						fmt.Sprintf("🔧 正在调用 [%s]...", tc.Function.Name))
					tsEvt.Meta.Skill = tc.Function.Name
					tsEvt.Detail = observe.ToolStartDetail{Skill: tc.Function.Name, Args: args}
					req.StepCallback(tsEvt)
				}
				t0 := time.Now()
				r, err := skill.Execute(toolCtx, args, env)
				dur := time.Since(t0)
				if p.skillMetrics != nil {
					p.skillMetrics(tc.Function.Name, dur, err)
				}
				if p.taskFailureMon != nil {
					p.taskFailureMon.Record(err != nil)
				}
				if err != nil {
					resultsCh <- tcResult{idx: idx, id: tc.ID, name: tc.Function.Name, args: args, err: err}
				} else {
					resultsCh <- tcResult{idx: idx, id: tc.ID, name: tc.Function.Name, args: args, output: r}
				}
			})
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
				r.output = fmt.Sprintf("执行失败: %v", r.err)
			}
			planSteps = append(planSteps, step)
			pruned := pruneToolResult(r.output, steps)
			messages = append(messages, buildToolResultMsg(r.id, pruned))

			// Notify: tool_result
			if req.StepCallback != nil {
				trSummary := fmt.Sprintf("✅ [%s] 完成", r.name)
				trErr := ""
				if r.err != nil {
					trSummary = fmt.Sprintf("❌ [%s] 执行失败", r.name)
					trErr = r.err.Error()
				}
				trEvt := observe.NewEvent(req.TraceID, observe.DomainPlanner, observe.EventToolResult, trSummary)
				trEvt.Meta.Skill = r.name
				trEvt.Detail = observe.ToolResultDetail{Skill: r.name, Result: truncate(r.output, 200), Error: trErr}
				req.StepCallback(trEvt)
			}
		}
	}

	messages = append(messages, llm.Message{Role: "user", Content: "你已执行了足够多的步骤。请根据以上所有工具结果，直接给出最终回答。"})

	client := p.clientForRequest(req)
	reply, _, err := client.ChatWithTools(ctx, messages, tools, 0.7)
	if err != nil {
		summary := "任务已执行 " + fmt.Sprintf("%d", steps) + " 步，但生成总结时出错。"
		return &PlanResult{Reply: summary, SkillsUsed: usedSkills, Steps: steps, Plan: planSteps, ContextLayers: ctxLayers}, nil
	}

	// Update recency for future intent routing
	if len(usedSkills) > 0 {
		p.recentSkills = append(usedSkills, p.recentSkills...)
		if len(p.recentSkills) > 20 {
			p.recentSkills = p.recentSkills[:20]
		}
	}

	return &PlanResult{Reply: p.cleanReply(reply), SkillsUsed: usedSkills, Steps: steps, Plan: planSteps, ContextLayers: ctxLayers}, nil
}

// buildFunctionDefs converts skill definitions to LLM FunctionDef format.
// In delegation mode (5+ handoff agents), only exposes transfer_to_* tools.
// Exec agents get domain-specific tools via their isolated RunFunc context.
// When disableDelegation is true, direct mode is forced (for subagent execution).
func (p *Planner) buildFunctionDefs(userMessage string, disableDelegation bool) []llm.FunctionDef {
	allSkills := p.registry.All()
	cats := p.registry.Categories()

	var catNames []string
	for _, c := range cats {
		catNames = append(catNames, fmt.Sprintf("%s(%d)", c.ID, len(c.SkillNames)))
	}

	// Delegation mode: planner only sees handoff tools, exec agents handle the rest
	if !disableDelegation && p.handoffReg != nil && len(p.handoffReg.List()) >= 4 {
		hdDefs := p.handoffReg.ToolDefinitions()
		defs := make([]llm.FunctionDef, 0, len(hdDefs))
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
		slog.Info("buildFunctionDefs", "mode", "delegation", "handoff_tools", len(defs), "total_skills", len(allSkills), "msg_prefix", truncate(userMessage, 50))
		return defs
	}

	slog.Info("buildFunctionDefs", "total_skills", len(allSkills), "categories", len(cats), "cat_detail", strings.Join(catNames, ","), "msg_prefix", truncate(userMessage, 50))

	// Fallback: direct mode (no delegation agents or fewer than 4)
	// Strategy 1: Dynamic filtering by intent
	if userMessage != "" && len(allSkills) > 25 && len(cats) > 0 {
		scorer := p.skillScorer
		if scorer != nil && len(p.recentSkills) > 0 {
			scorer.RecentSkills = p.recentSkills
		}
		filtered := p.registry.FilterByIntentScored(userMessage, scorer)
		if len(filtered) < len(allSkills) && len(filtered) > 0 {
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
			if p.handoffReg != nil {
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

	defs := make([]llm.FunctionDef, 0, len(allSkills))
	for _, s := range allSkills {
		defs = append(defs, llm.FunctionDef{
			Name:        s.Name(),
			Description: s.Description(),
			Parameters:  s.Parameters(),
		})
	}

	if p.handoffReg != nil {
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
// Earlier steps get compressed more aggressively to save context for later steps.
func pruneToolResult(output string, stepNum int) string {
	maxBytes := 8000
	if stepNum > 3 {
		maxBytes = 4000
	}
	if stepNum > 6 {
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
