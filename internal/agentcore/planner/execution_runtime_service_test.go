package planner

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/observe"
)

func TestExecutionRuntimeServiceBuildSkillEnvironment(t *testing.T) {
	var receivedModel string
	var receivedMessages []llm.Message
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		var payload struct {
			Model    string        `json:"model"`
			Messages []llm.Message `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		receivedModel = payload.Model
		receivedMessages = payload.Messages
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"role": "assistant", "content": "skill llm ok"}},
			},
		})
	}))
	defer srv.Close()

	contextAssembly := NewContextAssemblyService()
	contextAssembly.SetMemory(func(_ context.Context, tenantID, query string) string {
		return tenantID + ":" + query
	})

	env := NewExecutionRuntimeService(3).BuildSkillEnvironment(PlanRequest{
		ClassID:   "class-1",
		TeacherID: "teacher-1",
		StudentID: "student-1",
		TenantID:  "tenant-1",
	}, NewModelRuntimeService(llm.NewClient(srv.URL, "test-key", "test-model")), contextAssembly)

	if env.ClassID != "class-1" || env.TeacherID != "teacher-1" || env.StudentID != "student-1" || env.TenantID != "tenant-1" {
		t.Fatalf("request identity fields not copied into skill env: %#v", env)
	}
	reply, err := env.LLMCall(context.Background(), "system prompt", "user prompt")
	if err != nil {
		t.Fatalf("env LLMCall: %v", err)
	}
	if reply != "skill llm ok" {
		t.Fatalf("unexpected LLM reply %q", reply)
	}
	if receivedModel != "test-model" {
		t.Fatalf("expected model test-model, got %q", receivedModel)
	}
	if len(receivedMessages) != 2 || receivedMessages[0].Role != "system" || receivedMessages[0].Content != "system prompt" || receivedMessages[1].Role != "user" || receivedMessages[1].Content != "user prompt" {
		t.Fatalf("unexpected LLM messages: %#v", receivedMessages)
	}
	mem, err := env.MemorySearch(context.Background(), "tenant-2", "query", 5)
	if err != nil {
		t.Fatalf("env MemorySearch: %v", err)
	}
	if mem != "tenant-2:query" {
		t.Fatalf("unexpected memory result %q", mem)
	}
}

func TestExecutionRuntimeServiceBuildSkillEnvironmentHandlesMissingDependencies(t *testing.T) {
	env := NewExecutionRuntimeService(3).BuildSkillEnvironment(PlanRequest{}, NewModelRuntimeService(nil), nil)

	if _, err := env.LLMCall(context.Background(), "system", "user"); err == nil || !strings.Contains(err.Error(), "planner LLM client not configured") {
		t.Fatalf("expected missing LLM client error, got %v", err)
	}
	mem, err := env.MemorySearch(context.Background(), "tenant", "query", 5)
	if err != nil {
		t.Fatalf("nil context assembly memory search should not error: %v", err)
	}
	if mem != "" {
		t.Fatalf("nil context assembly should return empty memory, got %q", mem)
	}
}

func TestExecutionRuntimeServicePlanResultStateForRequest(t *testing.T) {
	state := NewExecutionRuntimeService(3).PlanResultStateForRequest(PlanResultStateRequest{
		Request:       PlanRequest{TraceID: "trace-state"},
		UsedSkills:    []string{"reader"},
		Steps:         0,
		PlanSteps:     []PlanStep{{ID: 1, Skill: "reader", Status: StepDone}},
		ContextLayers: []string{"memory"},
	})

	if state.Request.TraceID != "trace-state" || state.ResultSteps() != 1 || len(state.UsedSkills) != 1 || state.UsedSkills[0] != "reader" || len(state.PlanSteps) != 1 || len(state.ContextLayers) != 1 {
		t.Fatalf("unexpected plan result state: %#v", state)
	}

	explicit := NewExecutionRuntimeService(3).PlanResultStateForRequest(PlanResultStateRequest{
		Steps:     5,
		PlanSteps: []PlanStep{{ID: 1}},
	})
	if explicit.ResultSteps() != 5 {
		t.Fatalf("explicit step count should win, got %#v", explicit)
	}
}

func TestExecutionRuntimeServiceToolPostprocessRequestForState(t *testing.T) {
	runtime := NewExecutionRuntimeService(3)
	skillRuntime := NewSkillRuntimeService(nil)
	state := runtime.ToolPostprocessStateForRequest(ToolPostprocessStateRequest{
		Request:         PlanRequest{TraceID: "trace-tool-state"},
		StepNumber:      2,
		NextStepID:      3,
		PlanSteps:       []PlanStep{{ID: 1, Skill: "reader", Status: StepDone}},
		LastFailedCount: 1,
		SkillRuntime:    skillRuntime,
	})

	resultReq := runtime.ToolResultPostprocessRequestForState(state, ToolResultPostprocessInput{
		ToolCallID:            "call-1",
		SkillName:             "reader",
		Args:                  map[string]any{"path": "README.md"},
		Output:                "ok",
		IncludeToolMessage:    true,
		IncludeTextResultLine: true,
	})
	if resultReq.Request.TraceID != "trace-tool-state" || resultReq.StepNumber != 2 || resultReq.NextStepID != 3 || resultReq.ToolCallID != "call-1" || resultReq.SkillName != "reader" || resultReq.Args["path"] != "README.md" || resultReq.Output != "ok" || !resultReq.IncludeToolMessage || !resultReq.IncludeTextResultLine || resultReq.SkillRuntime != skillRuntime {
		t.Fatalf("unexpected tool result request: %#v", resultReq)
	}

	recoveryReq := runtime.ToolFailureRecoveryRequestForState(state)
	if recoveryReq.Request.TraceID != "trace-tool-state" || len(recoveryReq.PlanSteps) != 1 || recoveryReq.LastFailedCount != 1 {
		t.Fatalf("unexpected recovery request: %#v", recoveryReq)
	}
}

func TestExecutionRuntimeServiceApplyToolResultPostprocessForState(t *testing.T) {
	runtime := NewExecutionRuntimeService(3)
	applied := runtime.ApplyToolResultPostprocessForState(ToolResultPostprocessApplicationRequest{
		State: ToolPostprocessExecutionState{
			Request:    PlanRequest{TraceID: "trace-tool-apply"},
			StepNumber: 1,
			NextStepID: 2,
		},
		Input: ToolResultPostprocessInput{
			SkillName:             "reader",
			Args:                  map[string]any{"path": "README.md"},
			Output:                "ok",
			IncludeTextResultLine: true,
		},
		UsedSkills: []string{"search"},
		PlanSteps:  []PlanStep{{ID: 1, Skill: "search", Status: StepDone}},
	})

	if len(applied.UsedSkills) != 2 || applied.UsedSkills[0] != "search" || applied.UsedSkills[1] != "reader" {
		t.Fatalf("used skills not advanced: %#v", applied.UsedSkills)
	}
	if len(applied.PlanSteps) != 2 || applied.PlanSteps[1].ID != 2 || applied.PlanSteps[1].Skill != "reader" || applied.PlanSteps[1].Result != "ok" {
		t.Fatalf("plan steps not advanced: %#v", applied.PlanSteps)
	}
	if applied.Processed.ResultLine != "[reader] ok" {
		t.Fatalf("path-specific processed result not preserved: %#v", applied.Processed)
	}
}

func TestExecutionRuntimeServiceCollectToolResultsInOrder(t *testing.T) {
	runtime := NewExecutionRuntimeService(3)
	ch := make(chan ToolExecutionResult, 3)
	ch <- ToolExecutionResult{Index: 2, SkillName: "third", Output: "c"}
	ch <- ToolExecutionResult{Index: 0, SkillName: "first", Output: "a"}
	ch <- ToolExecutionResult{Index: 1, SkillName: "second", Output: "b"}

	ordered := runtime.CollectToolResultsInOrder(ch, 3)
	if len(ordered) != 3 || ordered[0].SkillName != "first" || ordered[1].SkillName != "second" || ordered[2].SkillName != "third" {
		t.Fatalf("results not restored in order: %#v", ordered)
	}

	if empty := runtime.CollectToolResultsInOrder(ch, 0); empty != nil {
		t.Fatalf("zero count should return nil, got %#v", empty)
	}
}

func TestExecutionRuntimeServiceTaskStoppedPlanResultForRequest(t *testing.T) {
	result := NewExecutionRuntimeService(3).TaskStoppedPlanResultForRequest(TaskStoppedPlanResultRequest{
		Reply: "Task stopped.",
		State: PlanResultExecutionState{
			UsedSkills:    []string{"reader"},
			PlanSteps:     []PlanStep{{ID: 1, Skill: "reader", Status: StepDone}},
			ContextLayers: []string{"memory"},
		},
	})

	if result.Reply != "Task stopped." || result.Steps != 1 || len(result.SkillsUsed) != 1 || result.SkillsUsed[0] != "reader" || len(result.Plan) != 1 || len(result.ContextLayers) != 1 {
		t.Fatalf("unexpected task stopped result: %#v", result)
	}
}

func TestExecutionRuntimeServiceSuccessfulPlanResultForRequest(t *testing.T) {
	result := NewExecutionRuntimeService(3).SuccessfulPlanResultForRequest(SuccessfulPlanResultRequest{
		Reply:            "final answer",
		ReasoningContent: "reasoning trace",
		State: PlanResultExecutionState{
			UsedSkills:    []string{"reader"},
			PlanSteps:     []PlanStep{{ID: 1, Skill: "reader", Status: StepDone}},
			ContextLayers: []string{"memory"},
		},
		Suggestions: []string{"next"},
	})

	if result.Reply != "final answer" || result.ReasoningContent != "reasoning trace" || result.Steps != 1 || len(result.SkillsUsed) != 1 || result.SkillsUsed[0] != "reader" || len(result.Plan) != 1 || len(result.ContextLayers) != 1 || len(result.Suggestions) != 1 {
		t.Fatalf("unexpected successful result: %#v", result)
	}
}

func TestExecutionRuntimeServiceEmitToolStartForRequest(t *testing.T) {
	var event observe.AgentEvent
	NewExecutionRuntimeService(3).EmitToolStartForRequest(ToolStartEventRequest{
		Request: PlanRequest{
			TraceID:  "trace-tool-start",
			TenantID: "tenant-tool-start",
			TaskID:   "task-tool-start",
			StepCallback: func(evt observe.AgentEvent) {
				event = evt
			},
		},
		SkillName: "reader",
		Args:      map[string]any{"path": "README.md"},
	})

	if event.Type != observe.EventToolStart || event.Meta.Skill != "reader" || event.Meta.TenantID != "tenant-tool-start" || event.Meta.TaskID != "task-tool-start" {
		t.Fatalf("unexpected tool start event: %#v", event)
	}
	if !strings.Contains(event.Summary, "reader") {
		t.Fatalf("tool start summary should mention skill, got %q", event.Summary)
	}
	detail, ok := event.Detail.(observe.ToolStartDetail)
	if !ok || detail.Skill != "reader" || detail.Args["path"] != "README.md" {
		t.Fatalf("unexpected tool start detail: %#v", event.Detail)
	}
}

func TestExecutionRuntimeServiceEmitToolStartForRequestNoCallback(t *testing.T) {
	NewExecutionRuntimeService(3).EmitToolStartForRequest(ToolStartEventRequest{
		Request:   PlanRequest{},
		SkillName: "reader",
		Args:      map[string]any{"path": "README.md"},
	})
}

func TestExecutionRuntimeServiceEmitStepThinkingForRequest(t *testing.T) {
	var event observe.AgentEvent
	NewExecutionRuntimeService(3).EmitStepThinkingForRequest(PlanRequest{
		TraceID:  "trace-thinking",
		TenantID: "tenant-thinking",
		TaskID:   "task-thinking",
		StepCallback: func(evt observe.AgentEvent) {
			event = evt
		},
	}, 2)

	if event.Type != observe.EventThinking || event.Meta.TenantID != "tenant-thinking" || event.Meta.TaskID != "task-thinking" {
		t.Fatalf("unexpected thinking event: %#v", event)
	}
	if !strings.Contains(event.Summary, "第 2 轮") {
		t.Fatalf("thinking summary should mention step, got %q", event.Summary)
	}
}

func TestExecutionRuntimeServiceReasoningDeltaCallbackForRequest(t *testing.T) {
	var events []observe.AgentEvent
	cb := NewExecutionRuntimeService(3).ReasoningDeltaCallbackForRequest(PlanRequest{
		TraceID:  "trace-reasoning-delta",
		TenantID: "tenant-reasoning-delta",
		TaskID:   "task-reasoning-delta",
		StepCallback: func(evt observe.AgentEvent) {
			events = append(events, evt)
		},
	})

	if cb == nil {
		t.Fatal("expected reasoning delta callback")
	}
	cb("content ignored", "")
	cb("", "思考片段")

	if len(events) != 1 {
		t.Fatalf("expected only non-empty reasoning delta to emit, got %#v", events)
	}
	event := events[0]
	if event.Type != observe.EventThinking || event.Summary != "思考片段" || event.Meta.TenantID != "tenant-reasoning-delta" || event.Meta.TaskID != "task-reasoning-delta" {
		t.Fatalf("unexpected reasoning delta event: %#v", event)
	}
	detail, ok := event.Detail.(map[string]string)
	if !ok || detail["stream_type"] != "thinking_delta" {
		t.Fatalf("unexpected reasoning delta detail: %#v", event.Detail)
	}

	if got := NewExecutionRuntimeService(3).ReasoningDeltaCallbackForRequest(PlanRequest{}); got != nil {
		t.Fatal("nil step callback should not create reasoning delta callback")
	}
}

func TestExecutionRuntimeServiceApplyReflectRetryForRequestEmitsEventAndMessages(t *testing.T) {
	var event observe.AgentEvent
	result := NewExecutionRuntimeService(3).ApplyReflectRetryForRequest(ReflectRetryRequest{
		Request: PlanRequest{
			TraceID:  "trace-reflect-retry",
			TenantID: "tenant-reflect-retry",
			TaskID:   "task-reflect-retry",
			StepCallback: func(evt observe.AgentEvent) {
				event = evt
			},
		},
		AssistantReply:   "draft answer",
		ReasoningContent: "draft reasoning",
		RetryPrompt:      "请重试",
		EmitEvent:        true,
	})

	if event.Type != observe.EventReflect || event.Meta.TenantID != "tenant-reflect-retry" || event.Meta.TaskID != "task-reflect-retry" {
		t.Fatalf("unexpected reflect retry event: %#v", event)
	}
	if !strings.Contains(event.Summary, "回答质量不够好") {
		t.Fatalf("unexpected reflect retry summary %q", event.Summary)
	}
	if len(result.Messages) != 2 {
		t.Fatalf("expected assistant+user retry messages, got %#v", result.Messages)
	}
	if result.Messages[0].Role != "assistant" || result.Messages[0].Content != "draft answer" || result.Messages[0].ReasoningContent != "draft reasoning" {
		t.Fatalf("unexpected assistant retry message: %#v", result.Messages[0])
	}
	if result.Messages[1].Role != "user" || result.Messages[1].Content != "请重试" {
		t.Fatalf("unexpected user retry message: %#v", result.Messages[1])
	}
}

func TestExecutionRuntimeServiceApplyReflectRetryForRequestDefaultPromptNoEvent(t *testing.T) {
	var events []observe.AgentEvent
	result := NewExecutionRuntimeService(3).ApplyReflectRetryForRequest(ReflectRetryRequest{
		Request: PlanRequest{StepCallback: func(evt observe.AgentEvent) {
			events = append(events, evt)
		}},
		AssistantReply: "draft answer",
	})

	if len(events) != 0 {
		t.Fatalf("reflect retry should not emit event unless requested, got %#v", events)
	}
	if len(result.Messages) != 2 || !strings.Contains(result.Messages[1].Content, "回答质量不够好") {
		t.Fatalf("expected default retry prompt, got %#v", result.Messages)
	}
}

func TestExecutionRuntimeServiceAssistantToolCallMessageForRequest(t *testing.T) {
	call := llm.ToolCall{ID: "call-1"}
	call.Function.Name = "reader"
	call.Function.Arguments = `{"path":"README.md"}`

	msg := NewExecutionRuntimeService(3).AssistantToolCallMessageForRequest(AssistantToolCallMessageRequest{
		AssistantReply:   "call tool",
		ToolCalls:        []llm.ToolCall{call},
		ReasoningContent: "need context",
	})

	if msg.Role != "assistant" || msg.Content != "call tool" || msg.ReasoningContent != "need context" {
		t.Fatalf("unexpected assistant tool-call message: %#v", msg)
	}
	if len(msg.ToolCalls) != 1 || msg.ToolCalls[0].ID != "call-1" || msg.ToolCalls[0].Function.Name != "reader" {
		t.Fatalf("tool calls not preserved: %#v", msg.ToolCalls)
	}
}

func TestExecutionRuntimeServiceRecoveryPromptMessageForRequest(t *testing.T) {
	msg := NewExecutionRuntimeService(3).RecoveryPromptMessageForRequest(RecoveryPromptMessageRequest{
		Prompt: "Planner 自愈提示：切换路径。",
	})

	if msg.Role != "user" || msg.Content != "Planner 自愈提示：切换路径。" {
		t.Fatalf("unexpected recovery prompt message: %#v", msg)
	}
}

func TestExecutionRuntimeServiceApplyToolResultForRequestSuccess(t *testing.T) {
	var events []observe.AgentEvent
	processed := NewExecutionRuntimeService(3).ApplyToolResultForRequest(ToolResultPostprocessRequest{
		Request: PlanRequest{
			TraceID:  "trace-success",
			TenantID: "tenant-success",
			TaskID:   "task-success",
			StepCallback: func(evt observe.AgentEvent) {
				events = append(events, evt)
			},
		},
		StepNumber:            2,
		NextStepID:            4,
		ToolCallID:            "call-1",
		SkillName:             "web_search",
		Args:                  map[string]any{"q": "云雀"},
		Output:                "搜索完成",
		IncludeToolMessage:    true,
		IncludeTextResultLine: true,
	})

	if !processed.Success || processed.UsedSkill != "web_search" {
		t.Fatalf("unexpected processed result: %#v", processed)
	}
	if processed.Step.ID != 4 || processed.Step.Status != StepDone || processed.Step.Result != "搜索完成" || processed.Step.Args["q"] != "云雀" {
		t.Fatalf("unexpected plan step: %#v", processed.Step)
	}
	if !processed.HasToolMessage || processed.ToolMessage.Role != "tool" || processed.ToolMessage.ToolCallID != "call-1" || processed.ToolMessage.Content != "搜索完成" {
		t.Fatalf("unexpected tool message: %#v", processed.ToolMessage)
	}
	if processed.ResultLine != "[web_search] 搜索完成" {
		t.Fatalf("unexpected text result line %q", processed.ResultLine)
	}
	if len(events) != 1 || events[0].Type != observe.EventToolResult || events[0].Meta.Skill != "web_search" || events[0].Meta.TenantID != "tenant-success" || events[0].Meta.TaskID != "task-success" {
		t.Fatalf("unexpected tool result event: %#v", events)
	}
	detail, ok := events[0].Detail.(observe.ToolResultDetail)
	if !ok || detail.Skill != "web_search" || detail.Result != "搜索完成" || detail.Error != "" {
		t.Fatalf("unexpected tool result detail: %#v", events[0].Detail)
	}
}

func TestExecutionRuntimeServiceApplyToolResultForRequestFailureUsesFriendlyText(t *testing.T) {
	var event observe.AgentEvent
	processed := NewExecutionRuntimeService(3).ApplyToolResultForRequest(ToolResultPostprocessRequest{
		Request: PlanRequest{
			TraceID:  "trace-failed",
			TenantID: "tenant-failed",
			StepCallback: func(evt observe.AgentEvent) {
				event = evt
			},
		},
		StepNumber:            1,
		NextStepID:            2,
		ToolCallID:            "call-failed",
		SkillName:             "timeout_tool",
		Args:                  map[string]any{"path": "slow"},
		Output:                "raw output ignored for model failure",
		Err:                   errors.New("context deadline exceeded"),
		IncludeToolMessage:    true,
		IncludeTextResultLine: true,
	})

	if processed.Success || processed.FriendlyError == "" || !strings.Contains(processed.FriendlyError, "现场已保留") {
		t.Fatalf("expected friendly failure result, got %#v", processed)
	}
	if processed.Step.Status != StepFailed || !strings.Contains(processed.Step.Error, "context deadline exceeded") || processed.Step.Result != "raw output ignored for model failure" {
		t.Fatalf("failed step should preserve raw evidence, got %#v", processed.Step)
	}
	if !processed.HasToolMessage || !strings.Contains(processed.ToolMessage.Content, "暂未完成：") || strings.Contains(processed.ToolMessage.Content, "context deadline exceeded") {
		t.Fatalf("model-facing tool message should use friendly failure text, got %#v", processed.ToolMessage)
	}
	if !strings.Contains(processed.ResultLine, "暂未完成：") || strings.Contains(processed.ResultLine, "context deadline exceeded") {
		t.Fatalf("text result line should use friendly failure text, got %q", processed.ResultLine)
	}
	if event.Type != observe.EventToolResult || !strings.Contains(event.Summary, "暂未完成") || strings.Contains(event.Summary, "context deadline exceeded") {
		t.Fatalf("unexpected failure event summary: %#v", event)
	}
	detail, ok := event.Detail.(observe.ToolResultDetail)
	if !ok || detail.Result != "" || !strings.Contains(detail.Error, "现场已保留") || strings.Contains(detail.Error, "context deadline exceeded") {
		t.Fatalf("unexpected failure detail: %#v", event.Detail)
	}
}

func TestExecutionRuntimeServiceApplyToolFailureRecoveryForRequestIgnoresSingleFailure(t *testing.T) {
	var events []observe.AgentEvent
	result := NewExecutionRuntimeService(3).ApplyToolFailureRecoveryForRequest(ToolFailureRecoveryRequest{
		Request: PlanRequest{StepCallback: func(evt observe.AgentEvent) {
			events = append(events, evt)
		}},
		PlanSteps: []PlanStep{
			{ID: 1, Skill: "missing_file", Status: StepFailed, Error: "file not found"},
		},
		LastFailedCount: 0,
	})

	if result.Applied || result.Prompt != "" || result.LastFailedCount != 0 {
		t.Fatalf("single failure should not trigger recovery, got %#v", result)
	}
	if len(events) != 0 {
		t.Fatalf("single failure should not emit recovery event, got %#v", events)
	}
}

func TestExecutionRuntimeServiceApplyToolFailureRecoveryForRequestEmitsPromptAndEvent(t *testing.T) {
	var event observe.AgentEvent
	result := NewExecutionRuntimeService(3).ApplyToolFailureRecoveryForRequest(ToolFailureRecoveryRequest{
		Request: PlanRequest{
			TraceID:  "trace-recovery",
			TenantID: "tenant-recovery",
			TaskID:   "task-recovery",
			StepCallback: func(evt observe.AgentEvent) {
				event = evt
			},
		},
		PlanSteps: []PlanStep{
			{ID: 1, Skill: "read_file", Status: StepDone, Result: "read README"},
			{ID: 2, Skill: "transfer_to_worker", Status: StepFailed, Error: "context deadline exceeded"},
			{ID: 3, Skill: "transfer_to_worker", Status: StepFailed, Error: "all fallback LLM clients failed"},
		},
		LastFailedCount: 0,
	})

	if !result.Applied || result.LastFailedCount != 2 || result.Summary.FailedCount != 2 {
		t.Fatalf("expected recovery to apply with updated failed count, got %#v", result)
	}
	if !strings.Contains(result.Prompt, "Planner 自愈提示") || !strings.Contains(result.Prompt, "不要继续重复同一路径") {
		t.Fatalf("unexpected recovery prompt: %q", result.Prompt)
	}
	if event.Type != observe.EventReflect || event.Meta.TenantID != "tenant-recovery" || event.Meta.TaskID != "task-recovery" {
		t.Fatalf("unexpected recovery event: %#v", event)
	}
	detail, ok := event.Detail.(PlannerFailureSummary)
	if !ok || detail.FailedCount != 2 || !detail.Recoverable {
		t.Fatalf("unexpected recovery detail: %#v", event.Detail)
	}
}

func TestExecutionRuntimeServiceApplyToolFailureRecoveryForRequestDoesNotRepeatForSameFailedCount(t *testing.T) {
	var events []observe.AgentEvent
	result := NewExecutionRuntimeService(3).ApplyToolFailureRecoveryForRequest(ToolFailureRecoveryRequest{
		Request: PlanRequest{StepCallback: func(evt observe.AgentEvent) {
			events = append(events, evt)
		}},
		PlanSteps: []PlanStep{
			{ID: 1, Skill: "transfer_to_worker", Status: StepFailed, Error: "context deadline exceeded"},
			{ID: 2, Skill: "transfer_to_worker", Status: StepFailed, Error: "all fallback LLM clients failed"},
		},
		LastFailedCount: 2,
	})

	if result.Applied || result.Prompt != "" || result.LastFailedCount != 2 {
		t.Fatalf("same failed count should not repeat recovery, got %#v", result)
	}
	if len(events) != 0 {
		t.Fatalf("same failed count should not emit recovery event, got %#v", events)
	}
}

func TestExecutionRuntimeServiceBuildTextReflectionPromptForRequest(t *testing.T) {
	result := NewExecutionRuntimeService(3).BuildTextReflectionPromptForRequest(TextReflectionPromptRequest{
		AssistantReply: "assistant plan",
		Results: []string{
			"[reader] 读到 A",
			"[search] 读到 B",
		},
		RecoveryPrompt: "Planner 自愈提示：不要重复失败路径。",
		ShouldContinue: true,
	})

	if !result.HasPrompt {
		t.Fatal("expected reflection prompt")
	}
	for _, want := range []string{
		"工具调用结果:",
		"[reader] 读到 A",
		"Planner 自愈提示",
		"请评估以上结果",
	} {
		if !strings.Contains(result.Prompt, want) {
			t.Fatalf("reflection prompt missing %q: %q", want, result.Prompt)
		}
	}
	if strings.Count(result.Prompt, "\n\n") < 2 {
		t.Fatalf("expected prompt sections to be separated, got %q", result.Prompt)
	}
	if len(result.Messages) != 2 || result.Messages[0].Role != "assistant" || result.Messages[0].Content != "assistant plan" || result.Messages[1].Role != "user" || result.Messages[1].Content != result.Prompt {
		t.Fatalf("unexpected text reflection messages: %#v", result.Messages)
	}
}

func TestExecutionRuntimeServiceBuildTextReflectionPromptForRequestNoContinue(t *testing.T) {
	result := NewExecutionRuntimeService(3).BuildTextReflectionPromptForRequest(TextReflectionPromptRequest{
		Results: []string{"[reader] ok"},
	})

	if !result.HasPrompt || !strings.Contains(result.Prompt, "工具调用结果:") {
		t.Fatalf("expected result-only reflection prompt, got %#v", result)
	}
	if strings.Contains(result.Prompt, "请评估以上结果") {
		t.Fatalf("did not expect continuation instruction, got %q", result.Prompt)
	}
	empty := NewExecutionRuntimeService(3).BuildTextReflectionPromptForRequest(TextReflectionPromptRequest{})
	if empty.HasPrompt || empty.Prompt != "" {
		t.Fatalf("empty reflection request should not produce prompt, got %#v", empty)
	}
	if len(empty.Messages) != 0 {
		t.Fatalf("empty reflection request should not produce messages, got %#v", empty.Messages)
	}
}

func TestExecutionRuntimeServiceBuildFinalAnswerPromptForRequest(t *testing.T) {
	result := NewExecutionRuntimeService(3).BuildFinalAnswerPromptForRequest(FinalAnswerPromptRequest{
		Request: PlanRequest{TraceID: "trace-final-answer"},
	})

	if !result.HasMessage {
		t.Fatal("expected final answer prompt message")
	}
	if result.Message.Role != "user" {
		t.Fatalf("expected user final answer prompt, got %#v", result.Message)
	}
	if !strings.Contains(result.Message.Content, "直接给出最终回答") {
		t.Fatalf("final answer prompt should ask for final answer, got %q", result.Message.Content)
	}
}

func TestExecutionRuntimeServiceTerminalPlanResultForRequest(t *testing.T) {
	svc := NewExecutionRuntimeService(3)
	canceled := svc.TerminalPlanResultForRequest(TerminalPlanResultRequest{
		State: PlanResultExecutionState{
			UsedSkills:    []string{"reader"},
			Steps:         2,
			ContextLayers: []string{"memory"},
		},
		Reason: TerminalPlanResultContextCanceled,
	})
	if canceled == nil || !strings.Contains(canceled.Reply, "连接暂时中断") || canceled.Steps != 2 {
		t.Fatalf("unexpected context-canceled terminal result: %#v", canceled)
	}
	if len(canceled.SkillsUsed) != 1 || canceled.SkillsUsed[0] != "reader" || len(canceled.ContextLayers) != 1 {
		t.Fatalf("terminal result should preserve execution state: %#v", canceled)
	}

	failed := svc.TerminalPlanResultForRequest(TerminalPlanResultRequest{
		State: PlanResultExecutionState{
			PlanSteps: []PlanStep{
				{ID: 1, Status: StepRunning},
				{ID: 2, Status: StepRunning},
			},
		},
		Reason: TerminalPlanResultFinalSynthesisFailed,
	})
	if failed == nil || failed.Steps != 2 || !strings.Contains(failed.Reply, "任务已执行 2 步") {
		t.Fatalf("unexpected final-synthesis terminal result: %#v", failed)
	}
}

func TestExecutionRuntimeServicePartialPlanResultForRequestFallsBackStepCount(t *testing.T) {
	result := NewExecutionRuntimeService(3).PartialPlanResultForRequest(PartialPlanResultRequest{
		State: PlanResultExecutionState{
			PlanSteps: []PlanStep{
				{ID: 1, Skill: "reader", Status: StepDone, Result: "阶段资料"},
				{ID: 2, Skill: "writer", Status: StepRunning},
			},
			UsedSkills:    []string{"reader"},
			ContextLayers: []string{"memory", "belief"},
		},
		RawError: "",
	})

	if result == nil || result.Steps != 2 || len(result.Plan) != 2 || len(result.SkillsUsed) != 1 || result.SkillsUsed[0] != "reader" {
		t.Fatalf("unexpected partial result shape: %#v", result)
	}
	if len(result.ContextLayers) != 2 || result.ContextLayers[0] != "memory" || result.ContextLayers[1] != "belief" {
		t.Fatalf("context layers should be preserved, got %#v", result.ContextLayers)
	}
	if !strings.Contains(result.Reply, "任务已部分执行") || !strings.Contains(result.Reply, "阶段资料") {
		t.Fatalf("unexpected partial reply %q", result.Reply)
	}
}

func TestExecutionRuntimeServicePartialPlanResultForRequestEmitsEvent(t *testing.T) {
	var event observe.AgentEvent
	result := NewExecutionRuntimeService(3).PartialPlanResultForRequest(PartialPlanResultRequest{
		State: PlanResultExecutionState{
			Request: PlanRequest{
				TraceID:  "trace-partial",
				TenantID: "tenant-partial",
				TaskID:   "task-partial",
				StepCallback: func(evt observe.AgentEvent) {
					event = evt
				},
			},
			PlanSteps: []PlanStep{
				{ID: 1, Skill: "reader", Status: StepDone, Result: "read README"},
				{ID: 2, Skill: "writer", Status: StepFailed, Error: "context deadline exceeded"},
			},
			UsedSkills: []string{"reader", "writer"},
			Steps:      7,
		},
		RawError: "all fallback LLM clients failed",
	})

	if result == nil || result.Steps != 7 {
		t.Fatalf("unexpected partial result: %#v", result)
	}
	if event.Type != observe.EventPartial || event.Meta.TenantID != "tenant-partial" || event.Meta.TaskID != "task-partial" {
		t.Fatalf("unexpected partial event: %#v", event)
	}
	detail, ok := event.Detail.(observe.PartialResultDetail)
	if !ok {
		t.Fatalf("expected partial detail, got %#v", event.Detail)
	}
	if !detail.Recoverable || detail.CompletedCount != 1 || detail.FailedCount != 1 || detail.NextStep == "" {
		t.Fatalf("unexpected partial detail: %#v", detail)
	}
	if !strings.Contains(detail.Reason, "现场已保留") || strings.Contains(detail.Reason, "fallback") || strings.Contains(detail.Steps[1].Error, "context deadline exceeded") {
		t.Fatalf("partial event should expose friendly diagnostics only, got %#v", detail)
	}
}

func TestExecutionRuntimeServicePartialPlanResultForRequestPreservesRawPlanEvidence(t *testing.T) {
	rawErr := "context deadline exceeded"
	result := NewExecutionRuntimeService(3).PartialPlanResultForRequest(PartialPlanResultRequest{
		State: PlanResultExecutionState{
			PlanSteps: []PlanStep{
				{ID: 1, Skill: "handoff", Status: StepFailed, Error: rawErr},
			},
		},
		RawError: rawErr,
	})

	if result == nil || len(result.Plan) != 1 || result.Plan[0].Error != rawErr {
		t.Fatalf("partial result should preserve raw plan evidence internally, got %#v", result)
	}
	if strings.Contains(result.Reply, rawErr) || !strings.Contains(result.Reply, "现场已保留") {
		t.Fatalf("partial reply should hide raw diagnostic but remain recoverable, got %q", result.Reply)
	}
}

func TestExecutionRuntimeServicePartialPlanResultForRequestNoStepsDoesNotEmitEvent(t *testing.T) {
	var events []observe.AgentEvent
	result := NewExecutionRuntimeService(3).PartialPlanResultForRequest(PartialPlanResultRequest{
		State: PlanResultExecutionState{
			Request: PlanRequest{StepCallback: func(evt observe.AgentEvent) {
				events = append(events, evt)
			}},
		},
		RawError: "context canceled",
	})

	if result == nil || result.Steps != 0 || len(result.Plan) != 0 {
		t.Fatalf("unexpected no-step partial result: %#v", result)
	}
	if !strings.Contains(result.Reply, "任务已部分执行") || strings.Contains(result.Reply, "context canceled") {
		t.Fatalf("no-step partial reply should be safe, got %q", result.Reply)
	}
	if len(events) != 0 {
		t.Fatalf("no-step partial result should not emit event, got %#v", events)
	}
}
