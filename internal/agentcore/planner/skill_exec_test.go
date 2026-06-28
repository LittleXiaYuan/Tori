package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"yunque-agent/internal/ledgercore"
	lsqlite "yunque-agent/internal/ledgercore/backend/sqlite"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/plan"
	"yunque-agent/internal/observe"
	"yunque-agent/pkg/skills"
)

func setupPlannerTestLedger(t *testing.T) *ledger.Ledger {
	t.Helper()
	backend, err := lsqlite.New(filepath.Join(t.TempDir(), "ledger.db"))
	if err != nil {
		t.Fatal(err)
	}
	ldg, err := ledger.Open(backend)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ldg.Close() })
	return ldg
}

func TestExecuteSkill_TrustGateUsesResolvedMetaTool(t *testing.T) {
	executed := false
	reg := skills.NewRegistry()
	reg.Register(&mockSkill{
		name: "shell_exec", desc: "Execute shell command",
		execFn: func(_ context.Context, _ map[string]any, _ *skills.Environment) (string, error) {
			executed = true
			return "should not run", nil
		},
	})

	p := NewPlanner(nil, reg, 8)
	checked := ""
	p.SetTrustCheck(func(skillName string) error {
		checked = skillName
		return fmt.Errorf("blocked")
	})

	exec := p.executeSkill(context.Background(), "use_shell", map[string]any{
		"action": "shell_exec",
		"args":   map[string]any{"cmd": "whoami"},
	}, nil)

	if exec.Err == nil {
		t.Fatal("expected trust gate error")
	}
	if executed {
		t.Fatal("blocked meta-tool target was executed")
	}
	if checked != "shell_exec" {
		t.Fatalf("expected trust check for resolved skill shell_exec, got %q", checked)
	}
	if exec.SkillName != "shell_exec" {
		t.Fatalf("expected resolved skill shell_exec, got %q", exec.SkillName)
	}
	if exec.Args["cmd"] != "whoami" {
		t.Fatalf("expected inner args to be used, got %+v", exec.Args)
	}
}

func TestExecuteSkill_TrimsModelEmittedSkillNames(t *testing.T) {
	reg := skills.NewRegistry()
	reg.Register(&mockSkill{
		name: "shell_exec", desc: "Execute shell command",
		execFn: func(_ context.Context, args map[string]any, _ *skills.Environment) (string, error) {
			if args["cmd"] != "whoami" {
				t.Fatalf("expected trimmed meta-tool inner args, got %+v", args)
			}
			return "ran", nil
		},
	})

	p := NewPlanner(nil, reg, 8)
	exec := p.executeSkill(context.Background(), " use_shell ", map[string]any{
		"action": " shell_exec ",
		"args":   map[string]any{"cmd": "whoami"},
	}, nil)

	if exec.Err != nil {
		t.Fatalf("expected whitespace-padded skill names to resolve, got %v", exec.Err)
	}
	if exec.RequestedName != "use_shell" || exec.SkillName != "shell_exec" {
		t.Fatalf("expected trimmed skill names, got requested=%q resolved=%q", exec.RequestedName, exec.SkillName)
	}
	if exec.Output != "ran" {
		t.Fatalf("expected tool output, got %q", exec.Output)
	}
}

func TestLongHorizonStepExecutor_TrustGateBlocksSkill(t *testing.T) {
	executed := false
	reg := skills.NewRegistry()
	reg.Register(&mockSkill{
		name: "shell_exec", desc: "Execute shell command",
		execFn: func(_ context.Context, _ map[string]any, _ *skills.Environment) (string, error) {
			executed = true
			return "should not run", nil
		},
	})

	p := NewPlanner(nil, reg, 8)
	p.SetTrustCheck(func(skillName string) error {
		if skillName != "shell_exec" {
			t.Fatalf("unexpected skill checked: %s", skillName)
		}
		return fmt.Errorf("blocked")
	})

	execStep := p.buildStepExecutor(PlanRequest{TenantID: "test"})
	pl := &plan.Plan{Steps: []plan.PlanStep{{
		Index:       0,
		Description: "run command",
		Skill:       "shell_exec",
		Args:        map[string]any{"cmd": "whoami"},
	}}}

	out, tools, err := execStep(context.Background(), pl, 0)
	if err == nil {
		t.Fatal("expected trust gate error")
	}
	if out != "" {
		t.Fatalf("expected empty output, got %q", out)
	}
	if executed {
		t.Fatal("blocked long-horizon skill was executed")
	}
	if len(tools) != 1 || tools[0] != "shell_exec" {
		t.Fatalf("expected tools [shell_exec], got %v", tools)
	}
	if !contains(err.Error(), "blocked by trust gate") {
		t.Fatalf("expected trust gate error, got %v", err)
	}
}

func TestRunNativeFC_TrustGateBlocksSkill(t *testing.T) {
	var llmCalls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if atomic.AddInt32(&llmCalls, 1) == 1 {
			json.NewEncoder(w).Encode(map[string]any{
				"choices": []map[string]any{{
					"message": map[string]any{
						"role":    "assistant",
						"content": "",
						"tool_calls": []map[string]any{{
							"id":   "call-1",
							"type": "function",
							"function": map[string]any{
								"name":      "shell_exec",
								"arguments": `{"cmd":"whoami"}`,
							},
						}},
					},
					"finish_reason": "tool_calls",
				}},
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{
				"message":       map[string]any{"role": "assistant", "content": "blocked safely"},
				"finish_reason": "stop",
			}},
		})
	}))
	defer srv.Close()

	executed := false
	reg := skills.NewRegistry()
	reg.Register(&mockSkill{
		name: "shell_exec", desc: "Execute shell command",
		execFn: func(_ context.Context, _ map[string]any, _ *skills.Environment) (string, error) {
			executed = true
			return "should not run", nil
		},
	})

	p := NewPlanner(llm.NewClient(srv.URL, "test-key", "test-model"), reg, 3)
	p.SetNativeFC(true)
	p.SetTrustCheck(func(skillName string) error {
		if skillName != "shell_exec" {
			t.Fatalf("unexpected skill checked: %s", skillName)
		}
		return fmt.Errorf("blocked")
	})

	result, err := p.Run(context.Background(), PlanRequest{
		Messages: []llm.Message{{Role: "user", Content: "run whoami"}},
		TenantID: "test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if executed {
		t.Fatal("blocked native-FC skill was executed")
	}
	if len(result.Plan) == 0 || result.Plan[0].Status != StepFailed {
		t.Fatalf("expected failed plan step, got %+v", result.Plan)
	}
	if !contains(result.Plan[0].Error, "blocked by trust gate") {
		t.Fatalf("expected trust gate error, got %q", result.Plan[0].Error)
	}
}

func TestRunNativeFCRepeatedFailuresEmitRecoverableStrategyEvent(t *testing.T) {
	var llmCalls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		call := atomic.AddInt32(&llmCalls, 1)
		if call == 1 {
			json.NewEncoder(w).Encode(map[string]any{
				"choices": []map[string]any{{
					"message": map[string]any{
						"role":    "assistant",
						"content": "",
						"tool_calls": []map[string]any{
							{
								"id":   "call-1",
								"type": "function",
								"function": map[string]any{
									"name":      "fail_timeout",
									"arguments": `{}`,
								},
							},
							{
								"id":   "call-2",
								"type": "function",
								"function": map[string]any{
									"name":      "fail_model",
									"arguments": `{}`,
								},
							},
						},
					},
					"finish_reason": "tool_calls",
				}},
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{
				"message":       map[string]any{"role": "assistant", "content": "已切换策略，先返回阶段结果。"},
				"finish_reason": "stop",
			}},
		})
	}))
	defer srv.Close()

	reg := skills.NewRegistry()
	reg.Register(&mockSkill{
		name: "fail_timeout", desc: "timeout",
		execFn: func(context.Context, map[string]any, *skills.Environment) (string, error) {
			return "", context.DeadlineExceeded
		},
	})
	reg.Register(&mockSkill{
		name: "fail_model", desc: "model",
		execFn: func(context.Context, map[string]any, *skills.Environment) (string, error) {
			return "", fmt.Errorf("all fallback LLM clients failed (FC): EOF")
		},
	})

	p := NewPlanner(llm.NewClient(srv.URL, "test-key", "test-model"), reg, 3)
	p.SetNativeFC(true)

	var recovery observe.AgentEvent
	result, err := p.Run(context.Background(), PlanRequest{
		Messages: []llm.Message{{Role: "user", Content: "run failing tools"}},
		TenantID: "test",
		TraceID:  "trace-failure-recovery",
		StepCallback: func(evt observe.AgentEvent) {
			if detail, ok := evt.Detail.(PlannerFailureSummary); ok {
				recovery = evt
				if !detail.Recoverable || detail.NextStep == "" {
					t.Fatalf("expected recoverable strategy detail, got %#v", detail)
				}
				if detail.PrimaryTarget == nil || detail.PrimaryTarget.Href != "/settings/providers?tab=providers" {
					t.Fatalf("expected provider recovery target, got %#v", detail.PrimaryTarget)
				}
				for _, raw := range []string{"context deadline exceeded", "all fallback", "EOF"} {
					if strings.Contains(strings.ToLower(strings.Join(detail.RuledOut, "\n")), strings.ToLower(raw)) {
						t.Fatalf("recovery ruled-out detail should hide raw %q, got %#v", raw, detail.RuledOut)
					}
				}
			}
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.Reply == "" {
		t.Fatalf("expected final result after strategy prompt, got %#v", result)
	}
	if recovery.Type != observe.EventReflect || !strings.Contains(recovery.Summary, "切换执行策略") {
		t.Fatalf("expected failure recovery event, got %#v", recovery)
	}
}

func TestRunNativeFCReturnsPartialAfterToolThenModelFailure(t *testing.T) {
	var llmCalls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := atomic.AddInt32(&llmCalls, 1)
		w.Header().Set("Content-Type", "application/json")
		if call == 1 {
			json.NewEncoder(w).Encode(map[string]any{
				"choices": []map[string]any{{
					"message": map[string]any{
						"role":    "assistant",
						"content": "",
						"tool_calls": []map[string]any{{
							"id":   "call-stage",
							"type": "function",
							"function": map[string]any{
								"name":      "stage_tool",
								"arguments": `{}`,
							},
						}},
					},
					"finish_reason": "tool_calls",
				}},
			})
			return
		}
		http.Error(w, "context deadline exceeded EOF all fallback LLM clients failed", http.StatusInternalServerError)
	}))
	defer srv.Close()

	reg := skills.NewRegistry()
	reg.Register(&mockSkill{
		name: "stage_tool", desc: "produce stage result",
		execFn: func(context.Context, map[string]any, *skills.Environment) (string, error) {
			return "阶段资料：Planner 已经完成工具执行，等待后续总结。", nil
		},
	})

	p := NewPlanner(llm.NewClient(srv.URL, "test-key", "test-model"), reg, 3)
	p.SetNativeFC(true)

	var partialEvt observe.AgentEvent
	result, err := p.Run(context.Background(), PlanRequest{
		Messages: []llm.Message{{Role: "user", Content: "推进云雀 planner"}},
		TenantID: "test",
		TraceID:  "trace-partial-fc",
		StepCallback: func(evt observe.AgentEvent) {
			if detail, ok := evt.Detail.(observe.PartialResultDetail); ok {
				partialEvt = evt
				if !detail.Recoverable || detail.CompletedCount != 1 || detail.NextStep == "" {
					t.Fatalf("expected recoverable partial detail, got %#v", detail)
				}
				if len(detail.Steps) != 1 || detail.Steps[0].Status != string(StepDone) {
					t.Fatalf("expected one completed partial step, got %#v", detail.Steps)
				}
				assertPlannerTextHasNoRawDiagnostics(t, detail.Reason)
			}
		},
	})
	if err != nil {
		t.Fatalf("expected partial result instead of error, got %v", err)
	}
	if result == nil || len(result.Plan) != 1 || result.Plan[0].Status != StepDone {
		t.Fatalf("expected one completed stage step, got %#v", result)
	}
	if !strings.Contains(result.Reply, "任务已部分执行") || !strings.Contains(result.Reply, "现场已保留") || !strings.Contains(result.Reply, "阶段资料") {
		t.Fatalf("expected friendly partial reply with stage result, got %q", result.Reply)
	}
	assertPlannerTextHasNoRawDiagnostics(t, result.Reply)
	if partialEvt.Type != observe.EventPartial || !strings.Contains(partialEvt.Summary, "阶段结果") {
		t.Fatalf("expected partial result event, got %#v", partialEvt)
	}
}

func TestRunReAct_TrustGateBlocksSkill(t *testing.T) {
	callCount := 0
	client := mockLLMServer(t, func(_ []llm.Message) string {
		callCount++
		if callCount == 1 {
			return `{"thought":"need shell","action":"shell_exec","args":{"cmd":"whoami"},"confidence":0.9}`
		}
		return `{"thought":"blocked","answer":"blocked safely","confidence":0.9}`
	})

	executed := false
	reg := skills.NewRegistry()
	reg.Register(&mockSkill{
		name: "shell_exec", desc: "Execute shell command",
		execFn: func(_ context.Context, _ map[string]any, _ *skills.Environment) (string, error) {
			executed = true
			return "should not run", nil
		},
	})

	p := NewPlanner(client, reg, 3)
	p.SetLedger(setupPlannerTestLedger(t))
	p.SetReActMode(true)
	p.SetTrustCheck(func(skillName string) error {
		if skillName != "shell_exec" {
			t.Fatalf("unexpected skill checked: %s", skillName)
		}
		return fmt.Errorf("blocked")
	})

	result, err := p.Run(context.Background(), PlanRequest{
		Messages: []llm.Message{{Role: "user", Content: "run whoami"}},
		TenantID: "test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if executed {
		t.Fatal("blocked ReAct skill was executed")
	}
	if len(result.Plan) == 0 || result.Plan[0].Status != StepFailed {
		t.Fatalf("expected failed plan step, got %+v", result.Plan)
	}
	if !contains(result.Plan[0].Error, "blocked by trust gate") {
		t.Fatalf("expected trust gate error, got %q", result.Plan[0].Error)
	}
}

func TestRunReAct_TrimsActionBeforeAllowedCheck(t *testing.T) {
	callCount := 0
	client := mockLLMServer(t, func(_ []llm.Message) string {
		callCount++
		if callCount == 1 {
			return `{"thought":"need file","action":" file_open ","args":{"path":"doc/README.md"},"confidence":0.9}`
		}
		return `{"thought":"done","answer":"已读取文档","confidence":0.9}`
	})

	executed := false
	reg := skills.NewRegistry()
	reg.Register(&mockSkill{
		name: "file_open", desc: "Open file",
		execFn: func(_ context.Context, args map[string]any, _ *skills.Environment) (string, error) {
			executed = true
			if args["path"] != "doc/README.md" {
				t.Fatalf("unexpected args: %+v", args)
			}
			return "README content", nil
		},
	})

	p := NewPlanner(client, reg, 3)
	p.SetLedger(setupPlannerTestLedger(t))
	p.SetReActMode(true)

	result, err := p.Run(context.Background(), PlanRequest{
		Messages:      []llm.Message{{Role: "user", Content: "read doc"}},
		TenantID:      "test",
		AllowedSkills: []string{" file_open "},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !executed {
		t.Fatal("expected whitespace-padded ReAct action to execute after trim")
	}
	if len(result.Plan) == 0 || result.Plan[0].Skill != "file_open" || result.Plan[0].Status != StepDone {
		t.Fatalf("expected successful file_open step, got %+v", result.Plan)
	}
}

func TestRunReAct_ToolFailureEmitsFailedEventOnly(t *testing.T) {
	callCount := 0
	var historyPrompt string
	client := mockLLMServer(t, func(msgs []llm.Message) string {
		callCount++
		for _, msg := range msgs {
			if strings.Contains(msg.Content, "Previous reasoning steps:") {
				historyPrompt = msg.Content
			}
		}
		if callCount == 1 {
			return `{"thought":"need failing tool","action":"fail_tool","args":{},"confidence":0.9}`
		}
		return `{"thought":"recovered","answer":"已保留现场","confidence":0.9}`
	})

	reg := skills.NewRegistry()
	reg.Register(&mockSkill{
		name: "fail_tool", desc: "Fails",
		execFn: func(_ context.Context, _ map[string]any, _ *skills.Environment) (string, error) {
			return "", context.DeadlineExceeded
		},
	})

	p := NewPlanner(client, reg, 3)
	p.SetLedger(setupPlannerTestLedger(t))
	p.SetReActMode(true)
	var toolEvents []observe.AgentEvent

	result, err := p.Run(context.Background(), PlanRequest{
		Messages: []llm.Message{{Role: "user", Content: "run failing tool"}},
		TenantID: "test",
		StepCallback: func(evt observe.AgentEvent) {
			if evt.Type == observe.EventToolResult {
				toolEvents = append(toolEvents, evt)
			}
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Plan) == 0 || result.Plan[0].Status != StepFailed {
		t.Fatalf("expected failed ReAct plan step, got %+v", result.Plan)
	}
	if len(toolEvents) != 1 {
		t.Fatalf("expected exactly one tool result event, got %d: %+v", len(toolEvents), toolEvents)
	}
	evt := toolEvents[0]
	if strings.Contains(evt.Summary, "完成") || !strings.Contains(evt.Summary, "暂停") {
		t.Fatalf("failed ReAct tool should not emit completion summary, got %q", evt.Summary)
	}
	detail, ok := evt.Detail.(observe.ToolResultDetail)
	if !ok || detail.Error == "" || detail.Result != "" {
		t.Fatalf("expected failed tool detail with error only, got %#v", evt.Detail)
	}
	assertPlannerTextHasNoRawDiagnostics(t, evt.Summary)
	assertPlannerTextHasNoRawDiagnostics(t, detail.Error)
	if historyPrompt == "" || !strings.Contains(historyPrompt, "现场已保留") {
		t.Fatalf("expected friendly ReAct history prompt, got %q", historyPrompt)
	}
	assertPlannerTextHasNoRawDiagnostics(t, historyPrompt)
}

func TestParseReActResponseHandlesBracesInsideStrings(t *testing.T) {
	p := NewPlanner(nil, skills.NewRegistry(), 3)
	reply := `前置说明 {"thought":"字段里有 } 符号","action":" file_open ","args":{"path":"doc/{blueprint}.md"},"confidence":0.9} 后置说明`
	got, err := p.parseReActResponse(reply)
	if err != nil {
		t.Fatalf("parse ReAct response: %v", err)
	}
	if got.Action == nil {
		t.Fatalf("expected action, got %#v", got)
	}
	if got.Action.Name != "file_open" || got.Action.Args["path"] != "doc/{blueprint}.md" {
		t.Fatalf("unexpected action: %#v", got.Action)
	}
}

// mcpToolRuntime is a CogniRuntime that contributes exactly one MCP-backed
// CogniTool. BuildContext/FilterSkills/Trace are inert so the test isolates the
// FC executor's cogni-tool injection + dispatch routing path.
type mcpToolRuntime struct {
	tool CogniTool
}

func (m mcpToolRuntime) BuildContext(context.Context, string, string, string, string) string { return "" }
func (m mcpToolRuntime) FilterSkills(_ string, _ string, _ string, in []skills.Skill) []skills.Skill {
	return in
}
func (m mcpToolRuntime) Trace(string, string, string) (CogniTraceDetail, bool) {
	return CogniTraceDetail{}, false
}
func (m mcpToolRuntime) Tools(context.Context, string, string, string) []CogniTool {
	return []CogniTool{m.tool}
}
func (m mcpToolRuntime) SurfaceAuthoritative(string, string, string) bool       { return false }
func (m mcpToolRuntime) RecordToolOutcome(string, string, string, string, bool) {}

// TestRunNativeFCInvokesCogniMCPTool is the end-to-end proof that a Cogni's MCP
// tool reaches the live FC loop: it is injected into the model's tool list,
// the model calls it, the planner routes the call back through the tool's Invoke
// (not the skill registry), and the result flows into the final answer.
func TestRunNativeFCInvokesCogniMCPTool(t *testing.T) {
	var llmCalls int32
	var sawMCPTool int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The cogni MCP tool must appear in the tools advertised to the model.
		var payload struct {
			Tools []struct {
				Function struct {
					Name string `json:"name"`
				} `json:"function"`
			} `json:"tools"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		for _, tl := range payload.Tools {
			if tl.Function.Name == "github_create_issue" {
				atomic.StoreInt32(&sawMCPTool, 1)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		if atomic.AddInt32(&llmCalls, 1) == 1 {
			json.NewEncoder(w).Encode(map[string]any{
				"choices": []map[string]any{{
					"message": map[string]any{
						"role":    "assistant",
						"content": "",
						"tool_calls": []map[string]any{{
							"id":   "call-1",
							"type": "function",
							"function": map[string]any{
								"name":      "github_create_issue",
								"arguments": `{"title":"bug report"}`,
							},
						}},
					},
					"finish_reason": "tool_calls",
				}},
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{
				"message":       map[string]any{"role": "assistant", "content": "已创建 issue #42"},
				"finish_reason": "stop",
			}},
		})
	}))
	defer srv.Close()

	// A real skill coexists with the MCP tool to prove the unified tool table.
	reg := skills.NewRegistry()
	reg.Register(&mockSkill{
		name: "web_search", desc: "search the web",
		execFn: func(context.Context, map[string]any, *skills.Environment) (string, error) {
			return "unused", nil
		},
	})

	var invokeCount int32
	var invokedArgs map[string]any
	runtime := mcpToolRuntime{tool: CogniTool{
		Name:        "github_create_issue",
		Description: "Create a GitHub issue",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{"title": map[string]any{"type": "string"}},
		},
		Invoke: func(_ context.Context, args map[string]any) (string, error) {
			atomic.AddInt32(&invokeCount, 1)
			invokedArgs = args
			return "issue #42 created", nil
		},
	}}

	p := NewPlanner(llm.NewClient(srv.URL, "test-key", "test-model"), reg, 3)
	p.SetNativeFC(true)
	p.SetCogniRuntime(runtime)

	result, err := p.Run(context.Background(), PlanRequest{
		Messages: []llm.Message{{Role: "user", Content: "create a github issue"}},
		TenantID: "test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if atomic.LoadInt32(&sawMCPTool) != 1 {
		t.Fatal("cogni MCP tool was not injected into the LLM tool list")
	}
	if got := atomic.LoadInt32(&invokeCount); got != 1 {
		t.Fatalf("cogni MCP tool Invoke called %d times, want 1 (dispatch routing)", got)
	}
	if invokedArgs["title"] != "bug report" {
		t.Fatalf("MCP tool invoked with unexpected args: %#v", invokedArgs)
	}
	if !strings.Contains(result.Reply, "已创建 issue") {
		t.Fatalf("unexpected final reply: %q", result.Reply)
	}
	if len(result.Plan) == 0 || result.Plan[0].Skill != "github_create_issue" || result.Plan[0].Status != StepDone {
		t.Fatalf("expected completed MCP tool step routed by name, got %#v", result.Plan)
	}
}
