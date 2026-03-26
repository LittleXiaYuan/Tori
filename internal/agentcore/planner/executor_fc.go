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

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/execution/browser"
	"yunque-agent/internal/observe"
)

// runNativeFC uses native LLM function calling (tool_calls in API response).
func (p *Planner) runNativeFC(ctx context.Context, req PlanRequest) (*PlanResult, error) {
	env := p.buildEnv(req)

	messages := p.BuildMessages(ctx, req)
	tools := p.buildFunctionDefs()

	var usedSkills []string
	var planSteps []PlanStep
	steps := 0

	for steps < p.maxSteps {
		steps++

		// Check for mid-execution interrupts between steps
		if shouldStop, extraMsgs := p.checkInterrupt(req, messages); shouldStop {
			return &PlanResult{Reply: "已停止当前任务。", SkillsUsed: usedSkills, Steps: steps, Plan: planSteps}, nil
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

		client := p.LLMClientFor(req.ModelOverride)
		reply, toolCalls, err := client.ChatWithTools(ctx, messages, tools, 0.7)
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
						llm.Message{Role: "assistant", Content: reply},
						llm.Message{Role: "user", Content: "你的回答质量不够好，请重新组织更完善的回答。"},
					)
					continue
				}
			}
			return &PlanResult{Reply: cleaned, SkillsUsed: usedSkills, Steps: steps, Plan: planSteps}, nil
		}

		// Append assistant message with tool calls reference
		messages = append(messages, llm.Message{Role: "assistant", Content: reply, ToolCalls: toolCalls})

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
			safeToolGo(ctx, p.toolTimeout, func(toolCtx context.Context) {
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

				// Check for browser tool calls (browser_*)
				if p.browserDispatch != nil && strings.HasPrefix(tc.Function.Name, "browser_") {
					slog.Info("planner: browser tool call", "tool", tc.Function.Name, "step", steps)
					if req.StepCallback != nil {
						tsEvt := observe.NewEvent(req.TraceID, observe.DomainPlanner, observe.EventToolStart,
							fmt.Sprintf("🌐 正在调用 [%s]...", tc.Function.Name))
						tsEvt.Meta.Skill = tc.Function.Name
						tsEvt.Detail = observe.ToolStartDetail{Skill: tc.Function.Name, Args: args}
						req.StepCallback(tsEvt)
					}
					t0 := time.Now()
					br := p.browserDispatch.Dispatch(tc.Function.Name, args)
					dur := time.Since(t0)
					if p.skillMetrics != nil {
						var brErr error
						if br.Error != "" {
							brErr = fmt.Errorf("%s", br.Error)
						}
						p.skillMetrics(tc.Function.Name, dur, brErr)
					}
					out, _ := json.Marshal(br)
					resultsCh <- tcResult{idx: idx, id: tc.ID, name: tc.Function.Name, args: args, output: string(out)}
					return
				}

				skill, ok := p.registry.Get(tc.Function.Name)
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
			messages = append(messages, llm.ToolResultMessage(r.id, r.output))

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

	client := p.LLMClientFor(req.ModelOverride)
	reply, _, err := client.ChatWithTools(ctx, messages, tools, 0.7)
	if err != nil {
		return nil, fmt.Errorf("planner fc final: %w", err)
	}
	return &PlanResult{Reply: p.cleanReply(reply), SkillsUsed: usedSkills, Steps: steps, Plan: planSteps}, nil
}

// buildFunctionDefs converts skill definitions to LLM FunctionDef format.
func (p *Planner) buildFunctionDefs() []llm.FunctionDef {
	allSkills := p.registry.All()
	defs := make([]llm.FunctionDef, 0, len(allSkills))
	for _, s := range allSkills {
		defs = append(defs, llm.FunctionDef{
			Name:        s.Name(),
			Description: s.Description(),
			Parameters:  s.Parameters(),
		})
	}

	// Append handoff tool definitions (transfer_to_*)
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

	// Append browser tool definitions if browser is enabled
	if p.browserDispatch != nil {
		for _, bd := range browser.ToolDefinitions() {
			fn, _ := bd["function"].(map[string]any)
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
