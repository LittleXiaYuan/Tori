package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	ldg "github.com/LittleXiaYuan/ledger"

	"yunque-agent/pkg/safego"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/localbrain"
	ageval "yunque-agent/internal/experimental/eval"
	agreact "yunque-agent/internal/experimental/react"
	"yunque-agent/internal/observe"
)

// runReAct executes using Ledger's ReAct loop with reasoning tracing.
// It wraps the existing skill execution infrastructure but adds structured
// reasoning, backtracking, and automatic experience recording.
func (p *Planner) runReAct(ctx context.Context, req PlanRequest) (*PlanResult, error) {
	if p.ledger == nil {
		slog.Warn("planner: ReAct mode requires Ledger, falling back to native FC")
		return p.runNativeFC(ctx, req)
	}

	env := p.buildEnv(req)
	_, ctxLayers := p.BuildMessages(ctx, req)

	taskID := req.TaskID
	if taskID == "" {
		taskID = "ephemeral-" + time.Now().Format("20060102-150405")
	}

	// Build the initial observation from the conversation
	initialObs := p.buildInitialObservation(req)

	// Build available tools description for the LLM
	allowed := allowedSkillSet(req.AllowedSkills)
	toolsDesc := p.buildToolsDescription(req.AllowedSkills)

	cfg := ldg.ReActConfig{
		MaxSteps:        p.maxSteps,
		MinConfidence:   0.3,
		BacktrackOnFail: true,
		Actor:           "planner",
	}

	var usedSkills []string
	var planSteps []PlanStep

	// ThinkFunc: uses LLM to produce thought + action
	// 集成 AgenticThinking：小模型先判断思考深度，再选择对应层级的大模型
	thinkFn := func(ctx context.Context, history []ldg.ReActStep) (*ldg.ThinkResult, error) {
		// MetaCog escalation: force expert tier when critical anomalies accumulate
		selectedTier := req.ModelOverride
		if p.metacogBridge != nil && taskID != "" && p.metacogBridge.ShouldEscalate(taskID) && selectedTier == "" {
			selectedTier = "expert"
			slog.Info("planner: metacog escalation → expert tier", "task", taskID)
		}

		// 第一阶段：Agentic Thinking 决定思考深度
		if p.agenticThinking != nil && len(history) > 0 {
			lastObs := ""
			if last := history[len(history)-1]; last.Result != nil {
				if last.Result.Error != "" {
					lastObs = "ERROR: " + last.Result.Error
				} else {
					lastObs = last.Result.Output
				}
			}

			thinkReq := localbrain.ThinkRequest{
				TaskID:           taskID,
				TenantID:         req.TenantID,
				Query:            initialObs,
				PrevActionResult: lastObs,
				StepIndex:        len(history),
				StepHistory:      convertToStepSummary(history),
			}
			if agResult, err := p.agenticThinking.Think(ctx, thinkReq); err == nil {
				// 如果 AgenticThinking 判断任务已完成
				if agResult.ShouldStop {
					return &ldg.ThinkResult{
						Thought:    agResult.Thought,
						Answer:     agResult.Thought,
						Confidence: agResult.Confidence,
					}, nil
				}
				// 根据思考深度选择模型层级
				if selectedTier == "" {
					switch agResult.Level {
					case localbrain.ThinkQuick:
						selectedTier = "fast"
					case localbrain.ThinkDeep:
						selectedTier = "expert"
					default:
						selectedTier = "smart"
					}
				}
			}
		}

		// 第二阶段：用选定层级的 LLM 执行思考
		messages := p.buildReActMessages(ctx, req, history, toolsDesc)
		client := p.LLMClientFor(selectedTier)

		reply, err := client.Chat(ctx, messages, 0.7)
		if err != nil {
			return nil, fmt.Errorf("LLM chat: %w", err)
		}

		return p.parseReActResponse(reply)
	}

	// ActFunc: executes tool/skill calls
	actFn := func(ctx context.Context, call ldg.ToolCall) (*ldg.ToolResult, error) {
		skillName := strings.TrimSpace(call.Name)
		if len(allowed) > 0 && !allowed[skillName] {
			errMsg := fmt.Sprintf("工具 %s 不在本次允许范围内，请改用已选择的工具继续。", skillName)
			planSteps = append(planSteps, PlanStep{
				ID:     len(planSteps) + 1,
				Action: skillName,
				Skill:  skillName,
				Args:   call.Args,
				Status: StepFailed,
				Error:  errMsg,
			})
			if req.StepCallback != nil {
				trEvt := observe.NewEvent(req.TraceID, observe.DomainPlanner, observe.EventToolResult,
					fmt.Sprintf("[%s] 未执行：不在本次允许工具内", skillName))
				trEvt.Meta.Skill = skillName
				trEvt.Meta.TenantID = req.TenantID
				trEvt.Detail = observe.ToolResultDetail{Skill: skillName, Error: errMsg}
				req.StepCallback(trEvt)
			}
			return &ldg.ToolResult{Error: errMsg}, nil
		}

		// Notify: tool start
		if req.StepCallback != nil {
			tsEvt := observe.NewEvent(req.TraceID, observe.DomainPlanner, observe.EventToolStart,
				fmt.Sprintf("正在调用 [%s]...", skillName))
			tsEvt.Meta.Skill = skillName
			tsEvt.Meta.TenantID = req.TenantID
			tsEvt.Detail = observe.ToolStartDetail{Skill: skillName, Args: call.Args}
			req.StepCallback(tsEvt)
		}

		exec := p.executeSkill(ctx, skillName, call.Args, env)
		skillName = exec.SkillName
		status := StepDone
		resultText := truncate(exec.Output, 200)
		errorText := ""
		if exec.Err != nil {
			status = StepFailed
			resultText = ""
			errorText = exec.Err.Error()
		}

		usedSkills = append(usedSkills, skillName)
		planSteps = append(planSteps, PlanStep{
			ID:     len(planSteps) + 1,
			Action: skillName,
			Skill:  skillName,
			Args:   exec.Args,
			Status: status,
			Result: resultText,
			Error:  errorText,
		})

		if req.StepCallback != nil {
			summary := fmt.Sprintf("[%s] 完成 (%dms)", skillName, exec.Duration.Milliseconds())
			detail := observe.ToolResultDetail{Skill: skillName, Result: truncate(exec.Output, 2000)}
			if exec.Err != nil {
				summary = fmt.Sprintf("[%s] 暂停：%s", skillName, plannerFriendlyFailureText(exec.Err.Error()))
				detail = observe.ToolResultDetail{Skill: skillName, Error: plannerFriendlyFailureText(exec.Err.Error())}
			}
			trEvt := observe.NewEvent(req.TraceID, observe.DomainPlanner, observe.EventToolResult, summary)
			trEvt.Meta.Skill = skillName
			trEvt.Meta.TenantID = req.TenantID
			trEvt.Detail = detail
			req.StepCallback(trEvt)
		}

		if exec.Err != nil {
			return &ldg.ToolResult{Error: plannerFriendlyFailureText(exec.Err.Error())}, nil
		}

		return &ldg.ToolResult{Output: exec.Output}, nil
	}

	// Step callback for UI updates
	onStep := func(step ldg.ReActStep) {
		if req.StepCallback != nil {
			thinkEvt := observe.NewEvent(req.TraceID, observe.DomainPlanner, observe.EventThinking,
				fmt.Sprintf("思考 (步骤 %d, 置信度 %.0f%%): %s", step.StepNum, step.Confidence*100, truncate(step.Thought, 100)))
			thinkEvt.Meta.TenantID = req.TenantID
			thinkEvt.Meta.TaskID = req.TaskID
			req.StepCallback(thinkEvt)
		}
	}

	// Run the ReAct loop via agentcore/react.Runner
	runner := agreact.NewRunner(p.ledger)
	result, err := runner.ReActLoop(ctx, taskID, initialObs, cfg, thinkFn, actFn, onStep)
	if err != nil {
		return nil, fmt.Errorf("ReAct loop: %w", err)
	}

	// Post-execution: self-evaluation (if task exists)
	if req.TaskID != "" && (result.Success || result.StopReason == "max_steps") {
		safego.Go("react-self-eval", func() {
			evalCtx := context.Background()
			evaluator := ageval.New(p.ledger)
			evalResult, evalErr := evaluator.Evaluate(evalCtx, req.TaskID)
			if evalErr == nil {
				slog.Info("planner: self-eval complete",
					"task", req.TaskID,
					"score", evalResult.QualityScore,
					"should_distill", evalResult.ShouldDistill)
			}
		})
	}

	return &PlanResult{
		Reply:         result.Answer,
		SkillsUsed:    usedSkills,
		Steps:         result.TotalSteps,
		Plan:          planSteps,
		ContextLayers: ctxLayers,
	}, nil
}

// buildInitialObservation constructs the starting observation from the request.
func (p *Planner) buildInitialObservation(req PlanRequest) string {
	if len(req.Messages) == 0 {
		return "No user message."
	}
	lastMsg := req.Messages[len(req.Messages)-1]
	obs := "User says: " + lastMsg.Content

	if req.TaskContext != "" {
		obs += "\n\nTask context:\n" + req.TaskContext
	}
	return obs
}

// buildToolsDescription creates a text description of available tools for the LLM.
func (p *Planner) buildToolsDescription(allowedSkills []string) string {
	var b strings.Builder
	b.WriteString("Available tools:\n")
	allowed := allowedSkillSet(allowedSkills)

	if p.registry != nil {
		for _, skill := range p.registry.All() {
			if len(allowed) > 0 && !allowed[skill.Name()] {
				continue
			}
			b.WriteString(fmt.Sprintf("- %s: %s\n", skill.Name(), skill.Description()))
		}
	}
	return b.String()
}

// buildReActMessages constructs the LLM prompt for the next ReAct step.
func (p *Planner) buildReActMessages(ctx context.Context, req PlanRequest, history []ldg.ReActStep, toolsDesc string) []llm.Message {
	sysPrompt := p.buildSystemPrompt()
	if p.personaPrompt != nil {
		if pp := p.personaPrompt(); pp != "" {
			sysPrompt += "\n\n" + pp
		}
	}

	reactInstructions := `
你正在使用 ReAct (Reasoning + Acting) 模式。

每一步，你需要输出一个 JSON 对象，格式如下：

当你需要调用工具时：
{"thought": "你的推理过程", "action": "tool_name", "args": {...}, "confidence": 0.8}

当你准备给出最终回答时：
{"thought": "总结推理", "answer": "最终回答", "confidence": 0.9}

` + toolsDesc

	sysPrompt += "\n\n" + reactInstructions

	msgs := []llm.Message{{Role: "system", Content: sysPrompt}}

	// Add dynamic context
	if len(req.Messages) > 0 {
		pb := NewPromptBuilder(p)
		assembled := pb.BuildDynamicContext(ctx, DynamicContextRequest{
			LastMessage: req.Messages[len(req.Messages)-1].Content,
			TenantID:    req.TenantID,
			Channel:     req.ChannelType,
			TaskContext: req.TaskContext,
			EmotionHint: req.EmotionHint,
		})
		if assembled != "" {
			msgs = append(msgs, llm.Message{
				Role:    "system",
				Content: "[动态上下文]\n" + assembled,
			})
		}
	}

	// Add conversation history
	if len(req.Messages) > 0 {
		msgs = append(msgs, req.Messages...)
	}

	// Add ReAct history
	if len(history) > 0 {
		var historyText strings.Builder
		historyText.WriteString("Previous reasoning steps:\n\n")
		for _, step := range history {
			historyText.WriteString(fmt.Sprintf("Step %d:\n", step.StepNum))
			historyText.WriteString(fmt.Sprintf("  Observation: %s\n", truncate(step.Observation, 300)))
			historyText.WriteString(fmt.Sprintf("  Thought: %s\n", step.Thought))
			if step.Action != nil {
				historyText.WriteString(fmt.Sprintf("  Action: %s\n", step.Action.Name))
			}
			if step.Result != nil {
				if step.Result.Error != "" {
					historyText.WriteString(fmt.Sprintf("  Result: ERROR - %s\n", truncate(step.Result.Error, 200)))
				} else {
					historyText.WriteString(fmt.Sprintf("  Result: %s\n", truncate(step.Result.Output, 300)))
				}
			}
			historyText.WriteString("\n")
		}
		msgs = append(msgs, llm.Message{
			Role:    "system",
			Content: historyText.String(),
		})
	}

	// MetaCog correction hints: inject anomaly-based guidance when detected
	if p.metacogBridge != nil && req.TaskID != "" {
		if hint := p.metacogBridge.CorrectionHint(req.TaskID); hint != "" {
			msgs = append(msgs, llm.Message{
				Role:    "system",
				Content: hint,
			})
		}
	}

	msgs = append(msgs, llm.Message{
		Role:    "user",
		Content: "基于上述观察和历史，输出你的下一步推理和动作（JSON 格式）。",
	})

	return msgs
}

// parseReActResponse extracts structured thought/action from LLM response.
func (p *Planner) parseReActResponse(reply string) (*ldg.ThinkResult, error) {
	// Try to parse as JSON
	reply = strings.TrimSpace(reply)

	// Extract JSON from markdown code block if present
	if idx := strings.Index(reply, "```json"); idx >= 0 {
		start := idx + 7
		end := strings.Index(reply[start:], "```")
		if end > 0 {
			reply = strings.TrimSpace(reply[start : start+end])
		}
	} else if idx := strings.Index(reply, "```"); idx >= 0 {
		start := idx + 3
		if nl := strings.Index(reply[start:], "\n"); nl >= 0 {
			start += nl + 1
		}
		end := strings.Index(reply[start:], "```")
		if end > 0 {
			reply = strings.TrimSpace(reply[start : start+end])
		}
	}

	// Try to find JSON object
	if idx := strings.Index(reply, "{"); idx >= 0 {
		if end := findClosingBrace(reply, idx); end >= 0 {
			reply = reply[idx : end+1]
		}
	}

	var parsed struct {
		Thought    string                 `json:"thought"`
		Action     string                 `json:"action"`
		Args       map[string]interface{} `json:"args"`
		Answer     string                 `json:"answer"`
		Confidence float64                `json:"confidence"`
	}

	if err := json.Unmarshal([]byte(reply), &parsed); err != nil {
		// Fallback: treat entire reply as final answer
		return &ldg.ThinkResult{
			Thought:    "Produced direct response",
			Answer:     reply,
			Confidence: 0.7,
		}, nil
	}

	result := &ldg.ThinkResult{
		Thought:    parsed.Thought,
		Confidence: parsed.Confidence,
	}

	if parsed.Action != "" {
		result.Action = &ldg.ToolCall{
			Name: strings.TrimSpace(parsed.Action),
			Args: parsed.Args,
		}
	} else {
		result.Answer = parsed.Answer
		if result.Answer == "" {
			result.Answer = parsed.Thought
		}
	}

	if result.Confidence == 0 {
		result.Confidence = 0.7
	}

	return result, nil
}

// convertToStepSummary converts Ledger ReActSteps to localbrain StepSummary format.
func convertToStepSummary(steps []ldg.ReActStep) []localbrain.StepSummary {
	summaries := make([]localbrain.StepSummary, 0, len(steps))
	for _, s := range steps {
		summary := localbrain.StepSummary{
			Success: s.Result == nil || s.Result.Error == "",
		}
		if s.Action != nil {
			summary.Action = s.Action.Name
		} else {
			summary.Action = "(think)"
		}
		if s.Result != nil {
			if s.Result.Error != "" {
				summary.Result = s.Result.Error
			} else {
				summary.Result = truncate(s.Result.Output, 200)
			}
		}
		summaries = append(summaries, summary)
	}
	return summaries
}
