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
	env := p.ensureExecutionRuntime().BuildSkillEnvironment(req, p.ensureModelRuntime(), p.contextAssembly)

	messages, ctxLayers := p.BuildMessages(ctx, req)
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

		reply, err := p.ensureModelRuntime().ChatFallbackForRequest(ctx, req, messages, p.runtimeStrategy, p.modelFallbackEvents(req))
		if err != nil {
			if len(planSteps) > 0 {
				return p.ensureExecutionRuntime().PartialPlanResultForRequest(PartialPlanResultRequest{State: resultState(), RawError: err.Error()}), nil
			}
			return nil, fmt.Errorf("planner step %d: %w", steps, err)
		}

		calls := p.parseSkillCalls(reply)
		if len(calls) == 0 {
			cleaned := p.cleanReply(reply)

			if p.reflect != nil && steps < p.maxPlanSteps() {
				userIntent := ""
				if len(req.Messages) > 0 {
					userIntent = req.Messages[len(req.Messages)-1].Content
				}
				if !p.reflect(ctx, userIntent, cleaned) {
					slog.Info("planner: reflect unsatisfied, retrying", "step", steps)
					retry := p.ensureExecutionRuntime().ApplyReflectRetryForRequest(ReflectRetryRequest{
						Request:        req,
						AssistantReply: reply,
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

		// Execute tool calls in parallel
		ch := make(chan ToolExecutionResult, len(calls))
		for i, call := range calls {
			idx, c := i, call // capture loop vars
			timeout := p.perToolTimeout()
			timeout = p.ensureDelegationRuntime().HandoffTimeoutForTool(c.Name, timeout)
			safeToolGo(ctx, timeout, func(toolCtx context.Context) {
				// Check for handoff (transfer_to_*) calls first
				handoff := p.ensureDelegationRuntime().ExecuteHandoffForRequest(
					toolCtx, req, c.Name, c.Args, "text", steps,
					HandoffExecutionHooks{
						Metrics:                p.skillMetrics,
						RecordExecutionFailure: p.ensureProactiveCognition().RecordExecutionFailure,
					},
				)
				if handoff.Handled {
					if handoff.Err != nil {
						ch <- ToolExecutionResult{Index: idx, SkillName: c.Name, Err: handoff.Err}
					} else {
						ch <- ToolExecutionResult{Index: idx, SkillName: c.Name, Output: handoff.Reply}
					}
					return
				}

				slog.Info("planner: executing skill", "skill", c.Name, "step", steps, "parallel", len(calls) > 1)
				p.ensureExecutionRuntime().EmitToolStartForRequest(ToolStartEventRequest{
					Request:   req,
					SkillName: c.Name,
					Args:      c.Args,
				})
				exec := p.executeSkill(toolCtx, c.Name, c.Args, env)
				if exec.Err != nil {
					ch <- ToolExecutionResult{Index: idx, SkillName: exec.SkillName, Err: exec.Err}
				} else {
					ch <- ToolExecutionResult{Index: idx, SkillName: exec.SkillName, Output: exec.Output}
				}
			})
		}

		// Collect results preserving order
		ordered := p.ensureExecutionRuntime().CollectToolResultsInOrder(ch, len(calls))

		var results []string
		for i, r := range ordered {
			processed := p.ensureExecutionRuntime().ApplyToolResultForRequest(
				p.ensureExecutionRuntime().ToolResultPostprocessRequestForState(toolPostprocessState(), ToolResultPostprocessInput{
					SkillName:             r.SkillName,
					Args:                  calls[i].Args,
					Output:                r.Output,
					Err:                   r.Err,
					IncludeTextResultLine: true,
				}),
			)
			usedSkills = append(usedSkills, processed.UsedSkill)
			planSteps = append(planSteps, processed.Step)
			results = append(results, processed.ResultLine)
		}

		recovery := p.ensureExecutionRuntime().ApplyToolFailureRecoveryForRequest(
			p.ensureExecutionRuntime().ToolFailureRecoveryRequestForState(toolPostprocessState()),
		)
		lastRecoveryFailedCount = recovery.LastFailedCount
		reflection := p.ensureExecutionRuntime().BuildTextReflectionPromptForRequest(TextReflectionPromptRequest{
			AssistantReply: reply,
			Results:        results,
			RecoveryPrompt: recovery.Prompt,
			ShouldContinue: steps < p.maxPlanSteps()-1 && len(planSteps) > 0,
		})
		messages = append(messages, reflection.Messages...)
	}

	streamCB := p.ensureExecutionRuntime().ReasoningDeltaCallbackForRequest(req)
	finalResult, err := p.ensureModelRuntime().ChatFallbackFullForRequest(ctx, req, messages, p.runtimeStrategy, p.modelFallbackEvents(req), streamCB)
	if err != nil {
		if len(planSteps) > 0 {
			return p.ensureExecutionRuntime().PartialPlanResultForRequest(PartialPlanResultRequest{State: resultState(), RawError: err.Error()}), nil
		}
		return nil, fmt.Errorf("planner final: %w", err)
	}
	return p.ensureExecutionRuntime().SuccessfulPlanResultForRequest(SuccessfulPlanResultRequest{
		Reply:            p.cleanReply(finalResult.Content),
		ReasoningContent: finalResult.ReasoningContent,
		State:            resultState(),
	}), nil
}
