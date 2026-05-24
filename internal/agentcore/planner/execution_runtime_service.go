package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	ctxwindow "yunque-agent/internal/agentcore/context"
	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/observe"
	"yunque-agent/pkg/skills"
)

// ExecutionRuntimeService owns Planner execution knobs that are not part of
// the declarative Cogni surface: step budget, per-tool timeout, dynamic context
// budget, and user-visible acknowledgement behavior.
//
// Keeping these together prevents Planner from accumulating every operational
// switch as a top-level field while preserving the existing public setters.
type ExecutionRuntimeService struct {
	maxSteps         int
	toolTimeout      time.Duration
	dynContextBudget int
	ackEnabled       bool
}

// ToolStartEventRequest carries the start of one direct skill/tool execution
// into the execution runtime. The runtime owns the common EventToolStart shape
// used by native-FC and text executors.
type ToolStartEventRequest struct {
	Request   PlanRequest
	SkillName string
	Args      map[string]any
}

// ThinkingEventRequest carries one planner thinking/progress event into the
// execution runtime. Native-FC step progress and text-mode reasoning deltas
// share the same observe.EventThinking envelope.
type ThinkingEventRequest struct {
	Request PlanRequest
	Summary string
	Detail  any
}

// ReflectRetryRequest carries the shared retry post-processing after a
// reflection/evaluator hook rejects a model answer. The executor decides when
// the answer is unsatisfied; the execution runtime owns the retry event and
// follow-up model messages.
type ReflectRetryRequest struct {
	Request          PlanRequest
	AssistantReply   string
	ReasoningContent string
	RetryPrompt      string
	EmitEvent        bool
}

// ReflectRetryResult is the executor-facing set of messages to append before
// retrying the model answer.
type ReflectRetryResult struct {
	Messages []llm.Message
}

// AssistantToolCallMessageRequest carries the native-FC assistant turn that
// produced tool calls. The execution runtime owns the model-facing assistant
// message shape so executors do not rebuild role/tool-call/reasoning details.
type AssistantToolCallMessageRequest struct {
	AssistantReply   string
	ToolCalls        []llm.ToolCall
	ReasoningContent string
}

// RecoveryPromptMessageRequest carries a model-facing recovery prompt into the
// execution runtime for message construction.
type RecoveryPromptMessageRequest struct {
	Prompt string
}

// ToolResultPostprocessRequest carries the executor-local result of one tool
// call into the execution runtime. The runtime owns the shared shape for
// PlanStep construction, friendly failure text, tool_result event emission,
// and optional model-facing result messages.
type ToolResultPostprocessRequest struct {
	Request               PlanRequest
	StepNumber            int
	NextStepID            int
	ToolCallID            string
	SkillName             string
	Args                  map[string]any
	Output                string
	Err                   error
	IncludeToolMessage    bool
	IncludeTextResultLine bool
	SkillRuntime          *SkillRuntimeService
}

// ToolResultPostprocessResult is the normalized view executors need after the
// execution runtime has applied shared result semantics.
type ToolResultPostprocessResult struct {
	UsedSkill      string
	Step           PlanStep
	Output         string
	Success        bool
	FriendlyError  string
	ToolMessage    llm.Message
	HasToolMessage bool
	ResultLine     string
}

// ToolFailureRecoveryRequest carries the current request-level execution
// evidence into the execution runtime. Native-FC and text executors both need
// the same "stop repeating a failing path" decision, prompt, and event
// emission after tool results are appended.
type ToolFailureRecoveryRequest struct {
	Request         PlanRequest
	PlanSteps       []PlanStep
	LastFailedCount int
}

// ToolFailureRecoveryResult is the executor-facing recovery decision. Applied
// is true only when the failed-step count has grown beyond LastFailedCount and
// the failure summary is strong enough to produce a recovery prompt.
type ToolFailureRecoveryResult struct {
	LastFailedCount int
	Prompt          string
	Applied         bool
	Summary         PlannerFailureSummary
}

// ToolPostprocessExecutionState carries the shared executor-local state needed
// to build tool-result and failure-recovery post-processing requests. Executors
// still own tool execution and result collection; the execution runtime owns
// translating that mutable state into request DTOs.
type ToolPostprocessExecutionState struct {
	Request         PlanRequest
	StepNumber      int
	NextStepID      int
	PlanSteps       []PlanStep
	LastFailedCount int
	SkillRuntime    *SkillRuntimeService
}

// ToolPostprocessStateRequest carries the current tool post-processing state
// into the execution runtime before a result or recovery helper is selected.
type ToolPostprocessStateRequest struct {
	Request         PlanRequest
	StepNumber      int
	NextStepID      int
	PlanSteps       []PlanStep
	LastFailedCount int
	SkillRuntime    *SkillRuntimeService
}

// ToolResultPostprocessInput carries path-specific tool result fields. Native
// function-calling and text-mode executors differ only in the optional model
// feedback shape they request; the shared request fields come from
// ToolPostprocessExecutionState.
type ToolResultPostprocessInput struct {
	ToolCallID            string
	SkillName             string
	Args                  map[string]any
	Output                string
	Err                   error
	IncludeToolMessage    bool
	IncludeTextResultLine bool
}

// TextReflectionPromptRequest carries text-mode tool result lines and recovery
// state into the execution runtime. Text execution is the only path that asks
// the model to assess free-form tool results before continuing, but the prompt
// shape is still execution-runtime post-processing rather than executor logic.
type TextReflectionPromptRequest struct {
	AssistantReply string
	Results        []string
	RecoveryPrompt string
	ShouldContinue bool
}

// TextReflectionPromptResult returns the model-facing prompt and whether there
// is anything useful to append. Messages contains the optional assistant/user
// pair used by the text executor to ask the model to reflect on tool results.
type TextReflectionPromptResult struct {
	Prompt    string
	HasPrompt bool
	Messages  []llm.Message
}

// FinalAnswerPromptRequest carries max-step final synthesis state into the
// execution runtime. Native-FC is the path that asks the model for a final
// answer after exhausting the step budget; the model-facing instruction should
// still live with execution post-processing rather than in the executor loop.
type FinalAnswerPromptRequest struct {
	Request PlanRequest
}

// FinalAnswerPromptResult returns the final synthesis message to append.
type FinalAnswerPromptResult struct {
	Message    llm.Message
	HasMessage bool
}

// TerminalPlanResultReason identifies safe terminal replies that do not have
// enough useful tool evidence for PartialPlanResultForRequest. The executor
// detects the terminal condition; the execution runtime owns user-facing safe
// result wording and PlanResult shaping.
type TerminalPlanResultReason string

const (
	TerminalPlanResultContextCanceled      TerminalPlanResultReason = "context_canceled"
	TerminalPlanResultFinalSynthesisFailed TerminalPlanResultReason = "final_synthesis_failed"
)

// PlanResultExecutionState carries the executor-local state that every
// PlanResult-shaping helper needs. Executors own state mutation; the execution
// runtime owns translating the current state into result request shapes.
type PlanResultExecutionState struct {
	Request       PlanRequest
	UsedSkills    []string
	Steps         int
	PlanSteps     []PlanStep
	ContextLayers []string
}

func (state PlanResultExecutionState) ResultSteps() int {
	if state.Steps > 0 {
		return state.Steps
	}
	return len(state.PlanSteps)
}

// PlanResultStateRequest carries the mutable executor-local result state into
// the execution runtime before a terminal/success/partial helper is selected.
type PlanResultStateRequest struct {
	Request       PlanRequest
	UsedSkills    []string
	Steps         int
	PlanSteps     []PlanStep
	ContextLayers []string
}

// PlanResultStateForRequest normalizes the common state shared by all
// PlanResult-shaping helpers. Executors call this through a small closure so
// they do not repeat the same UsedSkills/Steps/PlanSteps/ContextLayers mapping
// at every return site.
func (s *ExecutionRuntimeService) PlanResultStateForRequest(req PlanResultStateRequest) PlanResultExecutionState {
	return PlanResultExecutionState{
		Request:       req.Request,
		UsedSkills:    req.UsedSkills,
		Steps:         req.Steps,
		PlanSteps:     req.PlanSteps,
		ContextLayers: req.ContextLayers,
	}
}

// ToolPostprocessStateForRequest normalizes the common state shared by tool
// result and failure-recovery request constructors.
func (s *ExecutionRuntimeService) ToolPostprocessStateForRequest(req ToolPostprocessStateRequest) ToolPostprocessExecutionState {
	return ToolPostprocessExecutionState{
		Request:         req.Request,
		StepNumber:      req.StepNumber,
		NextStepID:      req.NextStepID,
		PlanSteps:       req.PlanSteps,
		LastFailedCount: req.LastFailedCount,
		SkillRuntime:    req.SkillRuntime,
	}
}

// TerminalPlanResultRequest carries no-evidence terminal execution state into
// the execution runtime.
type TerminalPlanResultRequest struct {
	State  PlanResultExecutionState
	Reason TerminalPlanResultReason
}

// TaskStoppedPlanResultRequest carries interrupted execution state into the
// execution runtime. PromptRuntimeService owns the localized stop reply; the
// execution runtime owns the PlanResult shape shared by native-FC and text
// executors.
type TaskStoppedPlanResultRequest struct {
	State PlanResultExecutionState
	Reply string
}

// SuccessfulPlanResultRequest carries executor success state into the
// execution runtime. Executors still own reply cleaning, next-move extraction,
// and reasoning content; the runtime owns the shared PlanResult shape.
type SuccessfulPlanResultRequest struct {
	Reply            string
	ReasoningContent string
	State            PlanResultExecutionState
	Suggestions      []string
}

// PartialPlanResultRequest carries recoverable partial execution state into the
// execution runtime. Native-FC and text executors both need the same safe
// partial reply, PlanResult shape, and observe.EventPartial emission when model
// synthesis fails after useful steps already completed.
type PartialPlanResultRequest struct {
	State    PlanResultExecutionState
	RawError string
}

func NewExecutionRuntimeService(maxSteps int) *ExecutionRuntimeService {
	if maxSteps <= 0 {
		maxSteps = 15
	}
	return &ExecutionRuntimeService{
		maxSteps:    maxSteps,
		toolTimeout: 60 * time.Second,
	}
}

func (s *ExecutionRuntimeService) MaxSteps() int {
	if s == nil || s.maxSteps <= 0 {
		return 15
	}
	return s.maxSteps
}

func (s *ExecutionRuntimeService) SetMaxSteps(maxSteps int) {
	if s == nil {
		return
	}
	if maxSteps <= 0 {
		maxSteps = 15
	}
	s.maxSteps = maxSteps
}

func (s *ExecutionRuntimeService) ToolTimeout() time.Duration {
	if s == nil || s.toolTimeout <= 0 {
		return 60 * time.Second
	}
	return s.toolTimeout
}

func (s *ExecutionRuntimeService) SetToolTimeout(timeout time.Duration) {
	if s == nil {
		return
	}
	s.toolTimeout = timeout
}

func (s *ExecutionRuntimeService) DynContextBudget() int {
	if s == nil {
		return DynContextBudgetDefault
	}
	return s.dynContextBudget
}

func (s *ExecutionRuntimeService) SetDynContextBudget(tokens int) {
	if s != nil {
		s.dynContextBudget = tokens
	}
}

func (s *ExecutionRuntimeService) SetAckEnabled(enabled bool) {
	if s != nil {
		s.ackEnabled = enabled
	}
}

func (s *ExecutionRuntimeService) AckEnabled() bool {
	return s != nil && s.ackEnabled
}

// EmitToolStartForRequest normalizes the direct tool-start event for planner
// executors. Handoff start/done events stay behind DelegationRuntimeService.
func (s *ExecutionRuntimeService) EmitToolStartForRequest(req ToolStartEventRequest) {
	if req.Request.StepCallback == nil {
		return
	}
	evt := observe.NewEvent(req.Request.TraceID, observe.DomainPlanner, observe.EventToolStart,
		fmt.Sprintf("🔧 正在调用 [%s]...", req.SkillName))
	evt.Meta.Skill = req.SkillName
	evt.Meta.TenantID = req.Request.TenantID
	evt.Meta.TaskID = req.Request.TaskID
	evt.Detail = observe.ToolStartDetail{Skill: req.SkillName, Args: req.Args}
	req.Request.StepCallback(evt)
}

// EmitThinkingForRequest normalizes planner thinking/progress event emission.
func (s *ExecutionRuntimeService) EmitThinkingForRequest(req ThinkingEventRequest) {
	if req.Request.StepCallback == nil || req.Summary == "" {
		return
	}
	evt := observe.NewEvent(req.Request.TraceID, observe.DomainPlanner, observe.EventThinking, req.Summary)
	evt.Meta.TenantID = req.Request.TenantID
	evt.Meta.TaskID = req.Request.TaskID
	evt.Detail = req.Detail
	req.Request.StepCallback(evt)
}

func (s *ExecutionRuntimeService) EmitStepThinkingForRequest(req PlanRequest, step int) {
	if step <= 0 {
		step = 1
	}
	s.EmitThinkingForRequest(ThinkingEventRequest{
		Request: req,
		Summary: fmt.Sprintf("正在思考 (第 %d 轮)...", step),
	})
}

func (s *ExecutionRuntimeService) ReasoningDeltaCallbackForRequest(req PlanRequest) llm.StreamDeltaFunc {
	if req.StepCallback == nil {
		return nil
	}
	return func(_, reasoningDelta string) {
		if reasoningDelta == "" {
			return
		}
		s.EmitThinkingForRequest(ThinkingEventRequest{
			Request: req,
			Summary: reasoningDelta,
			Detail:  map[string]string{"stream_type": "thinking_delta"},
		})
	}
}

// ApplyReflectRetryForRequest normalizes the shared post-processing after
// answer-quality reflection asks the executor to retry. Native-FC preserves
// reasoning content in the assistant retry message; text mode can leave it
// empty while still using the same retry-message shape.
func (s *ExecutionRuntimeService) ApplyReflectRetryForRequest(req ReflectRetryRequest) ReflectRetryResult {
	if req.EmitEvent {
		emitReflectRetryEvent(req.Request)
	}
	retryPrompt := req.RetryPrompt
	if retryPrompt == "" {
		retryPrompt = "你的回答质量不够好，请重新组织更完善的回答。"
	}
	return ReflectRetryResult{
		Messages: []llm.Message{
			{Role: "assistant", Content: req.AssistantReply, ReasoningContent: req.ReasoningContent},
			{Role: "user", Content: retryPrompt},
		},
	}
}

// AssistantToolCallMessageForRequest centralizes the native-FC assistant
// message that carries tool calls and optional reasoning content.
func (s *ExecutionRuntimeService) AssistantToolCallMessageForRequest(req AssistantToolCallMessageRequest) llm.Message {
	return llm.Message{
		Role:             "assistant",
		Content:          req.AssistantReply,
		ToolCalls:        req.ToolCalls,
		ReasoningContent: req.ReasoningContent,
	}
}

// RecoveryPromptMessageForRequest returns the model-facing user message for a
// recovery prompt. Empty prompts intentionally return an empty message; callers
// should only append it after recovery has applied.
func (s *ExecutionRuntimeService) RecoveryPromptMessageForRequest(req RecoveryPromptMessageRequest) llm.Message {
	return llm.Message{Role: "user", Content: req.Prompt}
}

// ToolResultPostprocessRequestForState builds the shared tool-result
// post-processing request from normalized executor state plus one tool result.
func (s *ExecutionRuntimeService) ToolResultPostprocessRequestForState(state ToolPostprocessExecutionState, input ToolResultPostprocessInput) ToolResultPostprocessRequest {
	return ToolResultPostprocessRequest{
		Request:               state.Request,
		StepNumber:            state.StepNumber,
		NextStepID:            state.NextStepID,
		ToolCallID:            input.ToolCallID,
		SkillName:             input.SkillName,
		Args:                  input.Args,
		Output:                input.Output,
		Err:                   input.Err,
		IncludeToolMessage:    input.IncludeToolMessage,
		IncludeTextResultLine: input.IncludeTextResultLine,
		SkillRuntime:          state.SkillRuntime,
	}
}

// ToolFailureRecoveryRequestForState builds the shared repeated-failure
// recovery request from normalized executor state.
func (s *ExecutionRuntimeService) ToolFailureRecoveryRequestForState(state ToolPostprocessExecutionState) ToolFailureRecoveryRequest {
	return ToolFailureRecoveryRequest{
		Request:         state.Request,
		PlanSteps:       state.PlanSteps,
		LastFailedCount: state.LastFailedCount,
	}
}

// ApplyToolResultForRequest normalizes a single tool execution result for
// planner executors. Native-FC and text executors differ in how they feed
// results back to the model, but they share PlanStep construction, friendly
// failure wording, tool_result event shape, and recommendation feedback.
func (s *ExecutionRuntimeService) ApplyToolResultForRequest(req ToolResultPostprocessRequest) ToolResultPostprocessResult {
	stepID := req.NextStepID
	if stepID <= 0 {
		stepID = 1
	}
	out := ToolResultPostprocessResult{
		UsedSkill: req.SkillName,
		Output:    req.Output,
		Success:   req.Err == nil,
		Step: PlanStep{
			ID:     stepID,
			Action: fmt.Sprintf("调用 %s", req.SkillName),
			Skill:  req.SkillName,
			Args:   req.Args,
			Status: StepDone,
			Result: req.Output,
		},
	}

	if req.Err != nil {
		out.FriendlyError = plannerFriendlyFailureText(req.Err.Error())
		out.Output = "暂未完成：" + out.FriendlyError
		out.Step.Status = StepFailed
		out.Step.Error = req.Err.Error()
	}

	if req.SkillRuntime != nil {
		req.SkillRuntime.RecordRecommendationOutcome(req.SkillName, out.Success)
	}

	emitToolResultEvent(req.Request, req.SkillName, out.Output, out.FriendlyError)

	if req.IncludeToolMessage {
		pruned := pruneToolResult(out.Output, req.StepNumber)
		out.ToolMessage = buildToolResultMsg(req.ToolCallID, pruned)
		out.HasToolMessage = true
	}
	if req.IncludeTextResultLine {
		if out.FriendlyError != "" {
			out.ResultLine = fmt.Sprintf("[%s] 暂未完成：%s", req.SkillName, out.FriendlyError)
		} else {
			out.ResultLine = fmt.Sprintf("[%s] %s", req.SkillName, req.Output)
		}
	}
	return out
}

// ApplyToolFailureRecoveryForRequest centralizes the shared recovery
// post-processing used by planner executors after tool results are appended.
// It owns repeated-failure detection, failed-count state advancement, recovery
// event emission, and model-facing recovery prompt construction.
func (s *ExecutionRuntimeService) ApplyToolFailureRecoveryForRequest(req ToolFailureRecoveryRequest) ToolFailureRecoveryResult {
	out := ToolFailureRecoveryResult{LastFailedCount: req.LastFailedCount}
	summary, ok := buildPlannerFailureSummary(req.PlanSteps)
	if !ok || summary.FailedCount <= req.LastFailedCount {
		return out
	}
	out.LastFailedCount = summary.FailedCount
	out.Prompt = formatFailureRecoveryPrompt(summary)
	out.Applied = true
	out.Summary = summary
	emitFailureRecoveryEvent(req.Request, summary)
	return out
}

// BuildTextReflectionPromptForRequest centralizes text-mode reflection prompt
// shaping after tool calls finish. The text executor owns tool execution order;
// the execution runtime owns the model-facing "assess results / continue"
// envelope and recovery prompt insertion.
func (s *ExecutionRuntimeService) BuildTextReflectionPromptForRequest(req TextReflectionPromptRequest) TextReflectionPromptResult {
	var sections []string
	if len(req.Results) > 0 {
		sections = append(sections, "工具调用结果:\n"+strings.Join(req.Results, "\n\n"))
	}
	if req.RecoveryPrompt != "" {
		sections = append(sections, req.RecoveryPrompt)
	}
	if req.ShouldContinue {
		sections = append(sections, "请评估以上结果：如果信息充足，直接给出最终回答；如果还需要更多信息，继续调用工具。")
	}
	prompt := strings.Join(sections, "\n\n")
	out := TextReflectionPromptResult{Prompt: prompt, HasPrompt: prompt != ""}
	if out.HasPrompt {
		out.Messages = []llm.Message{
			{Role: "assistant", Content: req.AssistantReply},
			{Role: "user", Content: prompt},
		}
	}
	return out
}

// BuildFinalAnswerPromptForRequest centralizes the native-FC max-step final
// synthesis instruction. The executor decides that the step budget is
// exhausted; the execution runtime owns the message shape and wording.
func (s *ExecutionRuntimeService) BuildFinalAnswerPromptForRequest(_ FinalAnswerPromptRequest) FinalAnswerPromptResult {
	return FinalAnswerPromptResult{
		Message: llm.Message{
			Role:    "user",
			Content: "你已执行了足够多的步骤。请根据以上所有工具结果，直接给出最终回答。",
		},
		HasMessage: true,
	}
}

// TerminalPlanResultForRequest centralizes safe terminal replies when there is
// not enough useful step evidence to build a partial result. Native-FC uses this
// for context-cancel and final-synthesis failure fallbacks after proving there
// are no completed steps worth summarizing.
func (s *ExecutionRuntimeService) TerminalPlanResultForRequest(req TerminalPlanResultRequest) *PlanResult {
	steps := req.State.ResultSteps()
	reply := fmt.Sprintf("任务已执行 %d 步，现场已保留。", steps)
	if req.Reason == TerminalPlanResultContextCanceled {
		reply = "连接暂时中断，现场已保留；如果任务已经推进，可以从最近可恢复任务继续。"
	}
	return &PlanResult{
		Reply:         reply,
		SkillsUsed:    req.State.UsedSkills,
		Steps:         steps,
		Plan:          req.State.PlanSteps,
		ContextLayers: req.State.ContextLayers,
	}
}

// TaskStoppedPlanResultForRequest centralizes the interrupted-execution result
// shape shared by native-FC and text executors. The caller supplies the
// localized reply from PromptRuntimeService.
func (s *ExecutionRuntimeService) TaskStoppedPlanResultForRequest(req TaskStoppedPlanResultRequest) *PlanResult {
	steps := req.State.ResultSteps()
	return &PlanResult{
		Reply:         req.Reply,
		SkillsUsed:    req.State.UsedSkills,
		Steps:         steps,
		Plan:          req.State.PlanSteps,
		ContextLayers: req.State.ContextLayers,
	}
}

// SuccessfulPlanResultForRequest centralizes the successful execution result
// shape shared by native-FC and text executors.
func (s *ExecutionRuntimeService) SuccessfulPlanResultForRequest(req SuccessfulPlanResultRequest) *PlanResult {
	steps := req.State.ResultSteps()
	return &PlanResult{
		Reply:            req.Reply,
		ReasoningContent: req.ReasoningContent,
		SkillsUsed:       req.State.UsedSkills,
		Steps:            steps,
		Plan:             req.State.PlanSteps,
		ContextLayers:    req.State.ContextLayers,
		Suggestions:      req.Suggestions,
	}
}

// PartialPlanResultForRequest centralizes recoverable partial-result
// post-processing for planner executors. It owns the user-visible safe reply,
// step-count fallback, context-layer propagation, and partial-result event
// emission so the native-FC and text paths do not keep Planner wrapper methods.
func (s *ExecutionRuntimeService) PartialPlanResultForRequest(req PartialPlanResultRequest) *PlanResult {
	steps := req.State.ResultSteps()
	emitPartialResultEvent(req.State.Request, req.State.PlanSteps, req.RawError)
	return &PlanResult{
		Reply:         buildPartialPlanReply(req.State.PlanSteps, req.RawError),
		SkillsUsed:    req.State.UsedSkills,
		Steps:         steps,
		Plan:          req.State.PlanSteps,
		ContextLayers: req.State.ContextLayers,
	}
}

func emitToolResultEvent(req PlanRequest, skillName, output, friendlyError string) {
	if req.StepCallback == nil {
		return
	}
	trSummary := fmt.Sprintf("✅ [%s] 完成", skillName)
	detail := observe.ToolResultDetail{Skill: skillName, Result: truncate(output, 200)}
	if friendlyError != "" {
		trSummary = fmt.Sprintf("⏸️ [%s] 暂未完成：%s", skillName, friendlyError)
		detail = observe.ToolResultDetail{Skill: skillName, Error: friendlyError}
	}
	trEvt := observe.NewEvent(req.TraceID, observe.DomainPlanner, observe.EventToolResult, trSummary)
	trEvt.Meta.Skill = skillName
	trEvt.Meta.TenantID = req.TenantID
	trEvt.Meta.TaskID = req.TaskID
	trEvt.Detail = detail
	req.StepCallback(trEvt)
}

func emitFailureRecoveryEvent(req PlanRequest, summary PlannerFailureSummary) {
	if req.StepCallback == nil {
		return
	}
	evt := observe.NewEvent(req.TraceID, observe.DomainPlanner, observe.EventReflect,
		"检测到连续失败，正在切换执行策略")
	evt.Meta.TenantID = req.TenantID
	evt.Meta.TaskID = req.TaskID
	evt.Detail = summary
	req.StepCallback(evt)
}

func emitPartialResultEvent(req PlanRequest, planSteps []PlanStep, rawErr string) {
	if req.StepCallback == nil || len(planSteps) == 0 {
		return
	}
	evt := observe.NewEvent(req.TraceID, observe.DomainPlanner, observe.EventPartial,
		"已返回阶段结果，现场已保留，可继续恢复")
	evt.Meta.TenantID = req.TenantID
	evt.Meta.TaskID = req.TaskID
	evt.Detail = buildPartialResultDetail(planSteps, rawErr)
	req.StepCallback(evt)
}

func emitReflectRetryEvent(req PlanRequest) {
	if req.StepCallback == nil {
		return
	}
	evt := observe.NewEvent(req.TraceID, observe.DomainPlanner, observe.EventReflect,
		"🔄 回答质量不够好，正在重新思考...")
	evt.Meta.TenantID = req.TenantID
	evt.Meta.TaskID = req.TaskID
	req.StepCallback(evt)
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

// BuildSkillEnvironment constructs the shared skill execution environment for
// one planner request.
//
// LLMCall honors the request's session-level client/model override through
// ModelRuntimeService so skills invoked during planning use the same provider
// selected for the conversation. MemorySearch stays behind
// ContextAssemblyService, keeping retrieval callbacks out of Planner's main
// orchestration surface.
func (s *ExecutionRuntimeService) BuildSkillEnvironment(req PlanRequest, modelRuntime *ModelRuntimeService, contextAssembly *ContextAssemblyService) *skills.Environment {
	return &skills.Environment{
		ClassID:   req.ClassID,
		TeacherID: req.TeacherID,
		StudentID: req.StudentID,
		TenantID:  req.TenantID,
		LLMCall: func(ctx context.Context, system, user string) (string, error) {
			msgs := []llm.Message{
				{Role: "system", Content: system},
				{Role: "user", Content: user},
			}
			client := modelRuntime.ClientForRequest(req)
			if client == nil {
				return "", fmt.Errorf("planner LLM client not configured")
			}
			return client.Chat(ctx, msgs, 0.7)
		},
		MemorySearch: func(ctx context.Context, tenantID, query string, topK int) (string, error) {
			if contextAssembly == nil {
				return "", nil
			}
			return contextAssembly.Memory(ctx, tenantID, query), nil
		},
	}
}
