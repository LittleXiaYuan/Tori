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
	"yunque-agent/pkg/skills"
)

// runNativeFC uses native LLM function calling (tool_calls in API response).
func (p *Planner) runNativeFC(ctx context.Context, req PlanRequest) (*PlanResult, error) {
	env := p.ensureExecutionRuntime().BuildSkillEnvironment(req, p.ensureModelRuntime(), p.contextAssembly)

	messages, ctxLayers := p.BuildMessages(ctx, req)
	userMsg := extractUserMessage(req)
	tools := p.buildFunctionDefs(userMsg, req.TenantID, req.ChannelType, req.DisableDelegation, req.AllowedSkills)
	if p.contextAssembly != nil {
		p.contextAssembly.EmitCogniTraceForRequest(req)
	}

	var usedSkills []string
	var planSteps []PlanStep
	steps := 0
	lastRecoveryFailedCount := 0
	resultState := func() PlanResultExecutionState {
		return p.ensureExecutionRuntime().PlanResultStateForRequest(PlanResultStateRequest{
			Request:       req,
			UsedSkills:    usedSkills,
			Steps:         steps,
			PlanSteps:     planSteps,
			ContextLayers: ctxLayers,
		})
	}
	toolPostprocessState := func() ToolPostprocessExecutionState {
		return p.ensureExecutionRuntime().ToolPostprocessStateForRequest(ToolPostprocessStateRequest{
			Request:         req,
			StepNumber:      steps,
			NextStepID:      len(planSteps) + 1,
			PlanSteps:       planSteps,
			LastFailedCount: lastRecoveryFailedCount,
			SkillRuntime:    p.ensureSkillRuntime(),
		})
	}

	for steps < p.maxPlanSteps() {
		steps++

		// Check for mid-execution interrupts between steps
		if shouldStop, extraMsgs := p.checkInterrupt(req, messages); shouldStop {
			return p.ensureExecutionRuntime().TaskStoppedPlanResultForRequest(TaskStoppedPlanResultRequest{
				State: resultState(),
				Reply: p.ensurePromptRuntime().TaskStoppedReply(),
			}), nil
		} else if len(extraMsgs) > 0 {
			messages = append(messages, extraMsgs...)
		}

		p.ensureExecutionRuntime().EmitStepThinkingForRequest(req, steps)

		if steps == 1 {
			totalChars := 0
			for _, m := range messages {
				totalChars += len(m.Content)
			}
			slog.Info("planner: prompt stats", "msgs", len(messages), "total_chars", totalChars, "tools", len(tools), "step", steps)
		}
		// chatWithToolsFallback walks the pool's tier fallback chain
		// (request tier → expert → smart → fast → local → primary) and
		// honors session ClientOverride and capability-aware selection.
		reply, toolCalls, lastReasoning, err := p.ensureModelRuntime().ChatWithToolsFallbackForRequest(
			ctx, req, messages, tools, p.runtimeStrategy, p.modelReasoningEvents(req), p.modelFallbackEvents(req),
		)
		if err != nil {
			if len(planSteps) > 0 {
				return p.ensureExecutionRuntime().PartialPlanResultForRequest(PartialPlanResultRequest{State: resultState(), RawError: err.Error()}), nil
			}
			return nil, fmt.Errorf("planner fc step %d: %w", steps, err)
		}

		if len(toolCalls) == 0 {
			cleaned := p.cleanReply(reply)
			if p.reflect != nil && steps < p.maxPlanSteps() {
				userIntent := ""
				if len(req.Messages) > 0 {
					userIntent = req.Messages[len(req.Messages)-1].Content
				}
				if !p.reflect(ctx, userIntent, cleaned) {
					slog.Info("planner: reflect unsatisfied, retrying", "step", steps)
					retry := p.ensureExecutionRuntime().ApplyReflectRetryForRequest(ReflectRetryRequest{
						Request:          req,
						AssistantReply:   reply,
						ReasoningContent: lastReasoning,
						RetryPrompt:      p.ensurePromptRuntime().ReflectRetryPrompt(),
						EmitEvent:        true,
					})
					messages = append(messages, retry.Messages...)
					continue
				}
			}
			cleaned, nextMoves := extractNextMoves(cleaned)
			return p.ensureExecutionRuntime().SuccessfulPlanResultForRequest(SuccessfulPlanResultRequest{
				Reply:       cleaned,
				State:       resultState(),
				Suggestions: nextMoves,
			}), nil
		}

		// Append assistant message with tool calls + reasoning (required by Kimi K2.5 etc.)
		messages = append(messages, p.ensureExecutionRuntime().AssistantToolCallMessageForRequest(AssistantToolCallMessageRequest{
			AssistantReply:   reply,
			ToolCalls:        toolCalls,
			ReasoningContent: lastReasoning,
		}))

		// Execute tool calls in parallel
		resultsCh := make(chan ToolExecutionResult, len(toolCalls))
		for i, tc := range toolCalls {
			idx, tc := i, tc // capture loop vars

			// Handoff delegations and skill generation need longer timeout
			timeout := p.perToolTimeout()
			timeout = p.ensureDelegationRuntime().HandoffTimeoutForTool(tc.Function.Name, timeout)
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
						resultsCh <- ToolExecutionResult{Index: idx, ToolCallID: tc.ID, SkillName: tc.Function.Name, Err: fmt.Errorf("tool panic: %v", r)}
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

					if !req.DisableDelegation {
						handoff := p.ensureDelegationRuntime().ExecuteHandoffForRequest(
							toolCtx, req, tc.Function.Name, args, "fc", steps,
							HandoffExecutionHooks{
								Metrics:                p.skillMetrics,
								RecordExecutionFailure: p.ensureProactiveCognition().RecordExecutionFailure,
							},
						)
						if handoff.Handled {
							if handoff.Err != nil {
								resultsCh <- ToolExecutionResult{Index: idx, ToolCallID: tc.ID, SkillName: tc.Function.Name, Args: args, Err: handoff.Err}
							} else {
								resultsCh <- ToolExecutionResult{Index: idx, ToolCallID: tc.ID, SkillName: tc.Function.Name, Args: args, Output: handoff.Reply}
							}
							return
						}
					}

					slog.Info("planner: executing skill (fc/parallel)", "skill", tc.Function.Name, "step", steps)
					p.ensureExecutionRuntime().EmitToolStartForRequest(ToolStartEventRequest{
						Request:   req,
						SkillName: tc.Function.Name,
						Args:      args,
					})
					exec := p.executeSkill(toolCtx, tc.Function.Name, args, env)
					if exec.Err != nil {
						resultsCh <- ToolExecutionResult{Index: idx, ToolCallID: tc.ID, SkillName: exec.SkillName, Args: exec.Args, Err: exec.Err}
					} else {
						resultsCh <- ToolExecutionResult{Index: idx, ToolCallID: tc.ID, SkillName: exec.SkillName, Args: exec.Args, Output: exec.Output}
					}
				}(toolCtx)
			}(toolParentCtx, timeout, idx, tc)
		}
		// Collect results in order
		tcResults := p.ensureExecutionRuntime().CollectToolResultsInOrder(resultsCh, len(toolCalls))
		for _, r := range tcResults {
			applied := p.ensureExecutionRuntime().ApplyToolResultPostprocessForState(ToolResultPostprocessApplicationRequest{
				State: toolPostprocessState(),
				Input: ToolResultPostprocessInput{
					ToolCallID:         r.ToolCallID,
					SkillName:          r.SkillName,
					Args:               r.Args,
					Output:             r.Output,
					Err:                r.Err,
					IncludeToolMessage: true,
				},
				UsedSkills: usedSkills,
				PlanSteps:  planSteps,
			})
			usedSkills = applied.UsedSkills
			planSteps = applied.PlanSteps
			messages = append(messages, applied.Processed.ToolMessage)
		}
		recovery := p.ensureExecutionRuntime().ApplyToolFailureRecoveryForRequest(
			p.ensureExecutionRuntime().ToolFailureRecoveryRequestForState(toolPostprocessState()),
		)
		lastRecoveryFailedCount = recovery.LastFailedCount
		if recovery.Applied {
			messages = append(messages, p.ensureExecutionRuntime().RecoveryPromptMessageForRequest(RecoveryPromptMessageRequest{Prompt: recovery.Prompt}))
		}

		// If the request context was cancelled (e.g. SSE disconnect) but tool
		// results are available, return them directly instead of calling the
		// LLM again (which would fail with "context canceled").
		if ctx.Err() != nil {
			if len(planSteps) > 0 {
				return p.ensureExecutionRuntime().PartialPlanResultForRequest(PartialPlanResultRequest{State: resultState(), RawError: ctx.Err().Error()}), nil
			}
			return p.ensureExecutionRuntime().TerminalPlanResultForRequest(TerminalPlanResultRequest{
				State:  resultState(),
				Reason: TerminalPlanResultContextCanceled,
			}), nil
		}
	}

	finalPrompt := p.ensureExecutionRuntime().BuildFinalAnswerPromptForRequest(FinalAnswerPromptRequest{Request: req})
	if finalPrompt.HasMessage {
		messages = append(messages, finalPrompt.Message)
	}

	reply, _, err := p.ensureModelRuntime().ChatWithToolsForRequest(ctx, req, messages, tools, 0.7)
	if err != nil {
		if len(planSteps) > 0 {
			return p.ensureExecutionRuntime().PartialPlanResultForRequest(PartialPlanResultRequest{State: resultState(), RawError: err.Error()}), nil
		}
		return p.ensureExecutionRuntime().TerminalPlanResultForRequest(TerminalPlanResultRequest{
			State:  resultState(),
			Reason: TerminalPlanResultFinalSynthesisFailed,
		}), nil
	}

	if len(usedSkills) > 0 {
		p.ensureSkillRuntime().RecordRecent(usedSkills)
	}

	return p.ensureExecutionRuntime().SuccessfulPlanResultForRequest(SuccessfulPlanResultRequest{
		Reply: p.cleanReply(reply),
		State: resultState(),
	}), nil
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
	// activated cogni's ToolSurface. The service returns input unchanged when
	// no cogni activates, so previous behaviour is preserved.
	if p.contextAssembly != nil && !disableDelegation && len(allowedSkills) == 0 {
		allSkills = p.contextAssembly.ApplyCogniSkillFilter(userMessage, tenantID, channelType, allSkills)
	}

	cats := p.registry.Categories()

	var catNames []string
	for _, c := range cats {
		catNames = append(catNames, fmt.Sprintf("%s(%d)", c.ID, len(c.SkillNames)))
	}

	// Delegation mode: planner only sees handoff tools + direct tools, exec agents handle the rest
	if !disableDelegation && p.delegationRuntime.HasHandoffAgents(4) {
		hdDefs := p.delegationRuntime.HandoffToolDefinitions()
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
			if !disableDelegation {
				for _, hd := range p.delegationRuntime.HandoffToolDefinitions() {
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

	if !disableDelegation {
		for _, hd := range p.delegationRuntime.HandoffToolDefinitions() {
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
