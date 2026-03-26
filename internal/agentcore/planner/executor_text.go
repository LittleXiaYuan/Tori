package planner

// executor_text.go — Text-based skill call execution engine.
// Handles LLM text parsing for tool_calls JSON, parallel skill dispatch,
// and multi-step planning when native FC is not available.

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"yunque-agent/internal/agentcore/llm"
)

// skillCall represents a parsed tool/skill invocation from LLM text output.
type skillCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"arguments"`
}

// parseSkillCalls extracts tool_calls from LLM text output.
func (p *Planner) parseSkillCalls(text string) []skillCall {
	// Look for JSON tool_calls in text
	idx := strings.Index(text, `"tool_calls"`)
	if idx < 0 {
		idx = strings.Index(text, `"skill_calls"`)
	}
	if idx < 0 {
		return nil
	}

	// Find enclosing braces
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

// runTextBased uses text-based skill call parsing with multi-step planning.
// Phase 1: Decompose — ask LLM to break task into steps (or handle directly for simple queries).
// Phase 2: Execute — run steps respecting dependencies, parallel when independent.
// Phase 3: Reflect — after tool results, assess if plan needs adjustment.
// Phase 4: Synthesize — produce final reply from all step results.
func (p *Planner) runTextBased(ctx context.Context, req PlanRequest) (*PlanResult, error) {
	env := p.buildEnv(req)

	messages := p.BuildMessages(ctx, req)

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

		client := p.LLMClientFor(req.ModelOverride)
		reply, err := client.Chat(ctx, messages, 0.7)
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

			return &PlanResult{Reply: cleaned, SkillsUsed: usedSkills, Steps: steps, Plan: planSteps}, nil
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

	clientFinal := p.LLMClientFor(req.ModelOverride)
	reply, err := clientFinal.Chat(ctx, messages, 0.7)
	if err != nil {
		return nil, fmt.Errorf("planner final: %w", err)
	}
	return &PlanResult{Reply: p.cleanReply(reply), SkillsUsed: usedSkills, Steps: steps, Plan: planSteps}, nil
}
