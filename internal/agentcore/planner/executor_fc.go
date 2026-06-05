package planner

// executor_fc.go — Native function-calling execution engine.
// Handles tool call dispatch, parallel execution with safeToolGo,
// and result collection for the native LLM FC API path.

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"log/slog"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/pkg/skills"
)

// runNativeFC uses native LLM function calling (tool_calls in API response).
func (p *Planner) runNativeFC(ctx context.Context, req PlanRequest) (*PlanResult, error) {
	contextAssembly := p.ensureContextAssembly()
	delegationRuntime := p.ensureDelegationRuntime()
	executionRuntime := p.ensureExecutionRuntime()
	modelRuntime := p.ensureModelRuntime()
	promptRuntime := p.ensurePromptRuntime()
	skillRuntime := p.ensureSkillRuntime()
	runtimeStrategy := p.ensureRuntimeStrategy()
	env := executionRuntime.BuildSkillEnvironment(req, modelRuntime, contextAssembly)

	messages, ctxLayers := p.BuildMessages(ctx, req)
	userMsg := extractUserMessage(req)
	tools := p.buildFunctionDefs(userMsg, req.TenantID, req.ChannelType, req.DisableDelegation, req.AllowedSkills, contextAssembly, delegationRuntime, skillRuntime)

	// Cogni MCP tool injection — additive tools contributed by the cognis that
	// activate this turn (their connected MCP servers). Gated the same way as the
	// cogni surface filter: skipped under disableDelegation (subagent isolation)
	// or an explicit user tool whitelist (AllowedSkills). cogniInvokers routes a
	// matching tool call back through the cogni's MCPManager during dispatch.
	var cogniInvokers map[string]CogniTool
	if !req.DisableDelegation && len(req.AllowedSkills) == 0 {
		tools, cogniInvokers = mergeCogniTools(tools, contextAssembly.CogniTools(ctx, userMsg, req.TenantID, req.ChannelType))
	}

	contextAssembly.EmitCogniTraceForRequest(req)

	var usedSkills []string
	var planSteps []PlanStep
	steps := 0
	lastRecoveryFailedCount := 0
	handoffHooks := delegationRuntime.HandoffHooks(p)
	resultState := func() PlanResultExecutionState {
		return executionRuntime.PlanResultStateForRequest(PlanResultStateRequest{
			Request:       req,
			UsedSkills:    usedSkills,
			Steps:         steps,
			PlanSteps:     planSteps,
			ContextLayers: ctxLayers,
		})
	}
	toolPostprocessState := func() ToolPostprocessExecutionState {
		return executionRuntime.ToolPostprocessStateForRequest(ToolPostprocessStateRequest{
			Request:         req,
			StepNumber:      steps,
			NextStepID:      len(planSteps) + 1,
			PlanSteps:       planSteps,
			LastFailedCount: lastRecoveryFailedCount,
			SkillRuntime:    skillRuntime,
		})
	}

	for steps < p.maxPlanSteps() {
		steps++

		// Check for mid-execution interrupts between steps
		if shouldStop, extraMsgs := p.checkInterrupt(req, messages); shouldStop {
			return executionRuntime.TaskStoppedPlanResultForRequest(TaskStoppedPlanResultRequest{
				State: resultState(),
				Reply: promptRuntime.TaskStoppedReply(),
			}), nil
		} else if len(extraMsgs) > 0 {
			messages = append(messages, extraMsgs...)
		}

		executionRuntime.EmitStepThinkingForRequest(req, steps)

		if steps == 1 {
			totalChars := 0
			for _, m := range messages {
				totalChars += len(m.Content)
			}
			slog.Info("planner: prompt stats", "msgs", len(messages), "total_chars", totalChars, "tools", len(tools), "tool_set_hash", toolSetHash(tools), "step", steps)
		}
		// chatWithToolsFallback walks the pool's tier fallback chain
		// (request tier → expert → smart → fast → local → primary) and
		// honors session ClientOverride and capability-aware selection.
		reply, toolCalls, lastReasoning, err := modelRuntime.ChatWithToolsFallbackForRequest(
			ctx, req, messages, tools, runtimeStrategy, p.modelReasoningEvents(req), p.modelFallbackEvents(req),
		)
		if err != nil {
			if len(planSteps) > 0 {
				return executionRuntime.PartialPlanResultForRequest(PartialPlanResultRequest{State: resultState(), RawError: err.Error()}), nil
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
					retry := executionRuntime.ApplyReflectRetryForRequest(ReflectRetryRequest{
						Request:          req,
						AssistantReply:   reply,
						ReasoningContent: lastReasoning,
						RetryPrompt:      promptRuntime.ReflectRetryPrompt(),
						EmitEvent:        true,
					})
					messages = append(messages, retry.Messages...)
					continue
				}
			}
			cleaned, nextMoves := extractNextMoves(cleaned)
			return executionRuntime.SuccessfulPlanResultForRequest(SuccessfulPlanResultRequest{
				Reply:       cleaned,
				State:       resultState(),
				Suggestions: nextMoves,
			}), nil
		}

		// Append assistant message with tool calls + reasoning (required by Kimi K2.5 etc.)
		messages = append(messages, executionRuntime.AssistantToolCallMessageForRequest(AssistantToolCallMessageRequest{
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
			timeout = delegationRuntime.HandoffTimeoutForTool(tc.Function.Name, timeout)
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

					// Cogni MCP tools take precedence: names are deduped at
					// injection (skills/handoff win), so a hit here is an
					// unambiguous MCP tool — route it through its Invoke instead
					// of the skill registry.
					if tool, ok := cogniInvokers[tc.Function.Name]; ok {
						slog.Info("planner: executing cogni mcp tool (fc/parallel)", "tool", tc.Function.Name, "step", steps)
						executionRuntime.EmitToolStartForRequest(ToolStartEventRequest{
							Request:   req,
							SkillName: tc.Function.Name,
							Args:      args,
						})
						out, cerr := tool.Invoke(toolCtx, args)
						if cerr != nil {
							resultsCh <- ToolExecutionResult{Index: idx, ToolCallID: tc.ID, SkillName: tc.Function.Name, Args: args, Err: cerr}
						} else {
							resultsCh <- ToolExecutionResult{Index: idx, ToolCallID: tc.ID, SkillName: tc.Function.Name, Args: args, Output: out}
						}
						return
					}

					if !req.DisableDelegation {
						handoff := delegationRuntime.ExecuteHandoffForRequest(
							toolCtx, req, tc.Function.Name, args, "fc", steps,
							handoffHooks,
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
					executionRuntime.EmitToolStartForRequest(ToolStartEventRequest{
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
		tcResults := executionRuntime.CollectToolResultsInOrder(resultsCh, len(toolCalls))
		for _, r := range tcResults {
			// Feed each tool outcome back to the active cognis so a Cogni can
			// self-tune its surface from real success/failure. Gated like the
			// cogni surface/tool injection; no-op without a cogni runtime.
			if !req.DisableDelegation && len(req.AllowedSkills) == 0 {
				contextAssembly.RecordCogniToolOutcome(userMsg, req.TenantID, req.ChannelType, r.SkillName, r.Err == nil)
			}
			applied := executionRuntime.ApplyToolResultPostprocessForState(ToolResultPostprocessApplicationRequest{
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
		recovery := executionRuntime.ApplyToolFailureRecoveryForState(toolPostprocessState())
		lastRecoveryFailedCount = recovery.LastFailedCount
		if recovery.Applied {
			messages = append(messages, executionRuntime.RecoveryPromptMessageForRequest(RecoveryPromptMessageRequest{Prompt: recovery.Prompt}))
		}

		// If the request context was cancelled (e.g. SSE disconnect) but tool
		// results are available, return them directly instead of calling the
		// LLM again (which would fail with "context canceled").
		if ctx.Err() != nil {
			if len(planSteps) > 0 {
				return executionRuntime.PartialPlanResultForRequest(PartialPlanResultRequest{State: resultState(), RawError: ctx.Err().Error()}), nil
			}
			return executionRuntime.TerminalPlanResultForRequest(TerminalPlanResultRequest{
				State:  resultState(),
				Reason: TerminalPlanResultContextCanceled,
			}), nil
		}
	}

	finalPrompt := executionRuntime.BuildFinalAnswerPromptForRequest(FinalAnswerPromptRequest{Request: req})
	if finalPrompt.HasMessage {
		messages = append(messages, finalPrompt.Message)
	}

	reply, _, err := modelRuntime.ChatWithToolsForRequest(ctx, req, messages, tools, 0.7)
	if err != nil {
		if len(planSteps) > 0 {
			return executionRuntime.PartialPlanResultForRequest(PartialPlanResultRequest{State: resultState(), RawError: err.Error()}), nil
		}
		return executionRuntime.TerminalPlanResultForRequest(TerminalPlanResultRequest{
			State:  resultState(),
			Reason: TerminalPlanResultFinalSynthesisFailed,
		}), nil
	}

	if len(usedSkills) > 0 {
		skillRuntime.RecordRecent(usedSkills)
	}

	return executionRuntime.SuccessfulPlanResultForRequest(SuccessfulPlanResultRequest{
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
func (p *Planner) buildFunctionDefs(userMessage, tenantID, channelType string, disableDelegation bool, allowedSkills []string, contextAssembly *ContextAssemblyService, delegationRuntime *DelegationRuntimeService, skillRuntime *SkillRuntimeService) []llm.FunctionDef {
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
	//
	// cogniAuthoritative records whether an activated cogni applied a non-identity
	// surface this turn. When it did, the cogni author has explicitly defined the
	// capability set, so the planner keeps that declared surface verbatim and
	// skips its own per-message intent ranking + tool cap below — turning the tool
	// block into a deterministic, prompt-cache-friendly prefix instead of a
	// per-message-varying (cache-busting) one. This is the seam that lets a Cogni
	// own tool orchestration above the flat skill/MCP layer: when it speaks, the
	// planner's heuristics step aside. The ambient path (no authoritative cogni)
	// keeps the original behaviour, so this is fully backward compatible.
	cogniAuthoritative := false
	if !disableDelegation && len(allowedSkills) == 0 {
		allSkills = contextAssembly.ApplyCogniSkillFilter(userMessage, tenantID, channelType, allSkills)
		cogniAuthoritative = contextAssembly.CogniSurfaceAuthoritative(userMessage, tenantID, channelType)
	}

	cats := p.registry.Categories()

	var catNames []string
	for _, c := range cats {
		catNames = append(catNames, fmt.Sprintf("%s(%d)", c.ID, len(c.SkillNames)))
	}

	// Delegation mode: planner only sees handoff tools + direct tools, exec agents handle the rest
	if !disableDelegation && delegationRuntime.HasHandoffAgents(4) {
		hdDefs := delegationRuntime.HandoffToolDefinitions()
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
				defs = append(defs, p.functionDefFor(sk))
			}
		}

		slog.Info("buildFunctionDefs", "mode", "delegation", "handoff_tools", len(defs), "total_skills", len(allSkills), "msg_prefix", truncate(userMessage, 50))
		sortFunctionDefsStable(defs)
		return defs
	}

	slog.Info("buildFunctionDefs", "total_skills", len(allSkills), "categories", len(cats), "cat_detail", strings.Join(catNames, ","), "msg_prefix", truncate(userMessage, 50))

	// Fallback: direct mode (no delegation agents or fewer than 4)
	// Strategy 1: Dynamic filtering by intent (threshold lowered from 25 to 10
	// so intent-based narrowing kicks in earlier, reducing tool noise for LLMs)
	if !cogniAuthoritative && userMessage != "" && len(allSkills) > 10 && len(cats) > 0 && len(allowedSkills) == 0 {
		skillScorer := skillRuntime.ScorerWithRecent()
		filtered := p.registry.FilterByIntentScored(userMessage, skillScorer)
		if len(filtered) < len(allSkills) && len(filtered) > 0 {
			filtered = p.rankSkillsByRecommendation(userMessage, filtered)
			filtered = p.capSkills(filtered)
			slog.Info("skill dynamic filter applied",
				"total", len(allSkills),
				"filtered", len(filtered),
				"message_prefix", truncate(userMessage, 50))
			defs := make([]llm.FunctionDef, 0, len(filtered))
			for _, s := range filtered {
				defs = append(defs, p.functionDefFor(s))
			}
			if !disableDelegation {
				for _, hd := range delegationRuntime.HandoffToolDefinitions() {
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
			sortFunctionDefsStable(defs)
			return defs
		}
	}

	// When a cogni surface is authoritative, its declared set (already narrowed by
	// the cogni ToolSurface, including surface.max_tools) is definitive: skip the
	// per-message recommendation ranking and the env tool cap so the prefix stays
	// deterministic and prompt-cache friendly. The ambient path keeps rank+cap.
	if !cogniAuthoritative {
		allSkills = p.rankSkillsByRecommendation(userMessage, allSkills)
		allSkills = p.capSkills(allSkills)
	} else {
		slog.Info("buildFunctionDefs: cogni surface authoritative; keeping declared surface as definitive tool set", "tools", len(allSkills))
	}

	defs := make([]llm.FunctionDef, 0, len(allSkills))
	for _, s := range allSkills {
		defs = append(defs, p.functionDefFor(s))
	}

	if !disableDelegation {
		for _, hd := range delegationRuntime.HandoffToolDefinitions() {
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

	sortFunctionDefsStable(defs)
	return defs
}

// functionDefFor returns the LLM FunctionDef for a skill, memoized per skill
// registry Version(). Skills rebuild their Parameters() schema map on every
// call; on a multi-step FC run this avoids reconstructing each tool's schema on
// every step. The cached Parameters map must be treated read-only.
func (p *Planner) functionDefFor(s skills.Skill) llm.FunctionDef {
	name := s.Name()
	ver := p.registry.Version()

	p.fnDefMu.RLock()
	if p.fnDefCacheVer == ver && p.fnDefCache != nil {
		if def, ok := p.fnDefCache[name]; ok {
			p.fnDefMu.RUnlock()
			return def
		}
	}
	p.fnDefMu.RUnlock()

	def := llm.FunctionDef{Name: name, Description: s.Description(), Parameters: s.Parameters()}

	p.fnDefMu.Lock()
	if p.fnDefCacheVer != ver || p.fnDefCache == nil {
		p.fnDefCache = make(map[string]llm.FunctionDef)
		p.fnDefCacheVer = ver
	}
	p.fnDefCache[name] = def
	p.fnDefMu.Unlock()
	return def
}

// maxFCTools returns the optional hard cap on how many skill tools are exposed
// to the model per turn (env PLANNER_MAX_FC_TOOLS; 0/unset = unlimited). Applied
// AFTER relevance ranking, so the most relevant tools are kept; it bounds the
// worst-case tool-schema payload on the fallback (no-narrowing) path.
func (p *Planner) maxFCTools() int {
	v := strings.TrimSpace(os.Getenv("PLANNER_MAX_FC_TOOLS"))
	if v == "" {
		return 0
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// capSkills truncates a relevance-ranked skill slice to maxFCTools (when set).
func (p *Planner) capSkills(list []skills.Skill) []skills.Skill {
	limit := p.maxFCTools()
	if limit > 0 && len(list) > limit {
		slog.Info("buildFunctionDefs: tool cap applied", "from", len(list), "limit", limit)
		return list[:limit]
	}
	return list
}

// sortFunctionDefsStable orders function defs deterministically by name. Tool
// SELECTION is decided upstream (filters + ranking + cap); a stable final order
// turns the tool block into a stable prefix so provider prompt caching can hit
// across turns/steps instead of re-billing a reordered block.
func sortFunctionDefsStable(defs []llm.FunctionDef) {
	sort.SliceStable(defs, func(i, j int) bool { return defs[i].Name < defs[j].Name })
}

// toolSetHash returns a stable, order-independent fingerprint of the tool set
// (names only). Logged in prompt stats so an operator/A-B harness can see
// whether the per-turn tool block stays constant across turns — the
// precondition for provider prompt-cache hits. A changing hash across otherwise
// similar turns means the tool prefix is being rebuilt and re-billed every turn
// (what the per-message intent filter causes, and what an authoritative cogni
// surface is meant to prevent).
func toolSetHash(defs []llm.FunctionDef) string {
	names := make([]string, len(defs))
	for i, d := range defs {
		names[i] = d.Name
	}
	sort.Strings(names)
	h := fnv.New32a()
	for _, n := range names {
		_, _ = h.Write([]byte(n))
		_, _ = h.Write([]byte{0})
	}
	return fmt.Sprintf("%08x", h.Sum32())
}

// mergeCogniTools appends the MCP-backed tools contributed by activated cognis
// to the skill/handoff tool list and re-sorts for prompt-cache stability.
//
// Orchestration rules (skill + MCP unified into one tool table):
//   - Local skills and handoff tools take precedence: a cogni tool whose name
//     collides with an existing tool is dropped, so every name the model sees
//     maps to exactly one binding and dispatch can route by name alone.
//   - Cogni tools are additive and intentional (declared by the activated
//     cogni), so they bypass PLANNER_MAX_FC_TOOLS — that cap bounds the broad
//     skill fallback, not a cogni's own scoped surface.
//   - The merged list is stable-sorted so the tool block stays a cache-friendly
//     prefix across steps.
//
// Returns the merged defs and a name→tool map the executor uses to route a
// matching tool call back through CallTool. Both the original defs and a nil
// invoker map are returned unchanged when there is nothing to inject.
func mergeCogniTools(defs []llm.FunctionDef, cogniTools []CogniTool) ([]llm.FunctionDef, map[string]CogniTool) {
	if len(cogniTools) == 0 {
		return defs, nil
	}
	existing := make(map[string]bool, len(defs))
	for _, d := range defs {
		existing[d.Name] = true
	}
	invokers := make(map[string]CogniTool, len(cogniTools))
	for _, t := range cogniTools {
		name := strings.TrimSpace(t.Name)
		if name == "" || t.Invoke == nil {
			continue
		}
		if existing[name] {
			slog.Warn("buildFunctionDefs: cogni mcp tool name collides with existing tool; skipping", "tool", name)
			continue
		}
		params := t.Parameters
		if params == nil {
			params = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		defs = append(defs, llm.FunctionDef{Name: name, Description: t.Description, Parameters: params})
		invokers[name] = t
		existing[name] = true
	}
	if len(invokers) == 0 {
		return defs, nil
	}
	slog.Info("buildFunctionDefs: cogni mcp tools injected", "added", len(invokers), "total_tools", len(defs))
	sortFunctionDefsStable(defs)
	return defs, invokers
}

func extractUserMessage(req PlanRequest) string {
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			return req.Messages[i].Content
		}
	}
	return ""
}
