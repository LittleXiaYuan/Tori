package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/plan"
	"yunque-agent/internal/agentcore/subagent"
	"yunque-agent/internal/observe"
	"yunque-agent/pkg/skills"
)

type dummyPlannerSkill string

func (s dummyPlannerSkill) Name() string        { return string(s) }
func (s dummyPlannerSkill) Description() string { return "dummy " + string(s) }
func (s dummyPlannerSkill) Parameters() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{}}
}
func (s dummyPlannerSkill) Execute(context.Context, map[string]any, *skills.Environment) (string, error) {
	return "ok", nil
}

type countingParamsSkill struct {
	name  string
	calls *int
}

func (s countingParamsSkill) Name() string        { return s.name }
func (s countingParamsSkill) Description() string { return "desc" }
func (s countingParamsSkill) Parameters() map[string]any {
	*s.calls++
	return map[string]any{"type": "object"}
}
func (s countingParamsSkill) Execute(context.Context, map[string]any, *skills.Environment) (string, error) {
	return "ok", nil
}

func TestFunctionDefForCachesByRegistryVersion(t *testing.T) {
	reg := skills.NewRegistry()
	calls := 0
	reg.Register(countingParamsSkill{name: "alpha", calls: &calls})
	p := NewPlanner(nil, reg, 5)

	sk, ok := reg.Get("alpha")
	if !ok {
		t.Fatal("skill not registered")
	}
	if d := p.functionDefFor(sk); d.Name != "alpha" {
		t.Fatalf("unexpected def name %q", d.Name)
	}
	p.functionDefFor(sk)
	if calls != 1 {
		t.Fatalf("Parameters() built %d times across two calls, want 1 (cache hit)", calls)
	}

	// A registry mutation bumps Version() and must invalidate the cache.
	reg.Register(countingParamsSkill{name: "beta", calls: new(int)})
	p.functionDefFor(sk)
	if calls != 2 {
		t.Fatalf("Parameters() built %d times after registry version bump, want 2 (cache invalidated)", calls)
	}
}

func TestMergeCogniTools(t *testing.T) {
	base := []llm.FunctionDef{{Name: "file_read", Description: "read"}}
	invoked := false
	cogniTools := []CogniTool{
		{
			Name:        "github_create_issue",
			Description: "create issue",
			Parameters:  map[string]any{"type": "object"},
			Invoke: func(_ context.Context, _ map[string]any) (string, error) {
				invoked = true
				return "ok", nil
			},
		},
		{Name: "file_read", Description: "dup"}, // collides with skill — dropped
		{Name: "", Invoke: func(context.Context, map[string]any) (string, error) { return "", nil }},
	}
	merged, invokers := mergeCogniTools(base, cogniTools)
	if len(merged) != 2 {
		t.Fatalf("merged len = %d, want 2", len(merged))
	}
	if merged[0].Name != "file_read" || merged[1].Name != "github_create_issue" {
		t.Fatalf("unexpected stable order: %#v", merged)
	}
	if len(invokers) != 1 {
		t.Fatalf("invokers len = %d, want 1", len(invokers))
	}
	tool, ok := invokers["github_create_issue"]
	if !ok {
		t.Fatal("missing invoker for github_create_issue")
	}
	out, err := tool.Invoke(context.Background(), nil)
	if err != nil || out != "ok" || !invoked {
		t.Fatalf("invoke = %q err=%v invoked=%v", out, err, invoked)
	}

	unchanged, nilMap := mergeCogniTools(base, nil)
	if len(unchanged) != 1 || nilMap != nil {
		t.Fatalf("nil cogni tools should be no-op: %#v %v", unchanged, nilMap)
	}
}

// surfaceAuthorityStub is a CogniRuntime that reports a configurable
// authoritative flag and an identity skill filter, isolating the authoritative
// tool-set branch in buildFunctionDefs.
type surfaceAuthorityStub struct {
	authoritative bool
}

func (s surfaceAuthorityStub) BuildContext(context.Context, string, string, string) string {
	return ""
}
func (s surfaceAuthorityStub) FilterSkills(_ string, _ string, _ string, in []skills.Skill) []skills.Skill {
	return in
}
func (s surfaceAuthorityStub) Trace(string, string, string) (CogniTraceDetail, bool) {
	return CogniTraceDetail{}, false
}
func (s surfaceAuthorityStub) Tools(context.Context, string, string, string) []CogniTool { return nil }
func (s surfaceAuthorityStub) SurfaceAuthoritative(string, string, string) bool {
	return s.authoritative
}
func (s surfaceAuthorityStub) RecordToolOutcome(string, string, string, string, bool) {}

// TestBuildFunctionDefsCogniSurfaceAuthoritativeBypassesCap proves the P1
// paradigm change: when a cogni surface is authoritative the planner keeps the
// declared set verbatim (skips the env tool cap + per-message ranking), giving a
// deterministic, cache-stable prefix; the ambient (non-authoritative) path still
// applies the cap.
func TestBuildFunctionDefsCogniSurfaceAuthoritativeBypassesCap(t *testing.T) {
	t.Setenv("PLANNER_MAX_FC_TOOLS", "3")
	reg := skills.NewRegistry()
	for _, n := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
		reg.Register(&mockSkill{name: n, desc: n})
	}

	authP := NewPlanner(nil, reg, 5)
	authP.SetCogniRuntime(surfaceAuthorityStub{authoritative: true})
	authDefs := authP.buildFunctionDefs("hello", "t", "web", false, nil,
		authP.ensureContextAssembly(), authP.ensureDelegationRuntime(), authP.ensureSkillRuntime())
	if len(authDefs) != 8 {
		t.Fatalf("authoritative surface should bypass cap and keep all 8 tools, got %d", len(authDefs))
	}

	ambientP := NewPlanner(nil, reg, 5)
	ambientP.SetCogniRuntime(surfaceAuthorityStub{authoritative: false})
	ambientDefs := ambientP.buildFunctionDefs("hello", "t", "web", false, nil,
		ambientP.ensureContextAssembly(), ambientP.ensureDelegationRuntime(), ambientP.ensureSkillRuntime())
	if len(ambientDefs) != 3 {
		t.Fatalf("ambient path should apply env cap of 3, got %d", len(ambientDefs))
	}

	// Authoritative output is deterministic across different messages (stable
	// prefix) — the prompt-cache precondition.
	authDefs2 := authP.buildFunctionDefs("totally different message", "t", "web", false, nil,
		authP.ensureContextAssembly(), authP.ensureDelegationRuntime(), authP.ensureSkillRuntime())
	if toolSetHash(authDefs) != toolSetHash(authDefs2) {
		t.Fatalf("authoritative tool set should be deterministic across messages: %s vs %s", toolSetHash(authDefs), toolSetHash(authDefs2))
	}
}

func TestCleanReplyRemovesToolCalls(t *testing.T) {
	p := &Planner{}
	input := `这是回答内容。{"tool_calls": [{"name": "test", "arguments": {}}]}后续文字`
	cleaned := p.cleanReply(input)
	if cleaned != "这是回答内容。后续文字" {
		t.Fatalf("unexpected: %q", cleaned)
	}
}

func TestCleanReplyRemovesThinkBlock(t *testing.T) {
	p := &Planner{}
	input := `<think>这是思考过程</think>这是真正的回答`
	cleaned := p.cleanReply(input)
	if cleaned != "这是真正的回答" {
		t.Fatalf("unexpected: %q", cleaned)
	}
}

func TestCleanReplyRemovesCodeBlock(t *testing.T) {
	p := &Planner{}
	input := "前文```json\n{\"a\":1}\n```后文"
	cleaned := p.cleanReply(input)
	if cleaned != "前文后文" {
		t.Fatalf("unexpected: %q", cleaned)
	}
}

func TestParseSkillCalls(t *testing.T) {
	p := &Planner{}
	input := `我来帮你查询。{"tool_calls": [{"name": "web_search", "arguments": {"query": "天气"}}]}`
	calls := p.parseSkillCalls(input)
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Name != "web_search" {
		t.Fatalf("expected web_search, got %s", calls[0].Name)
	}
}

func TestParseSkillCallsAcceptsStringArguments(t *testing.T) {
	p := &Planner{}
	input := `{"tool_calls":[{"name":"web_search","arguments":"{\"query\":\"天气\"}"}]}`
	calls := p.parseSkillCalls(input)
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Name != "web_search" || calls[0].Args["query"] != "天气" {
		t.Fatalf("unexpected parsed call: %#v", calls[0])
	}
}

func TestParseSkillCallsAcceptsOpenAIFunctionWrapper(t *testing.T) {
	p := &Planner{}
	input := `模型决定调用工具：{"tool_calls":[{"id":"call-1","type":"function","function":{"name":"file_open","arguments":"{\"path\":\"doc/README.md\"}"}}]}`
	calls := p.parseSkillCalls(input)
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Name != "file_open" || calls[0].Args["path"] != "doc/README.md" {
		t.Fatalf("unexpected parsed call: %#v", calls[0])
	}
}

func TestParseSkillCallsAcceptsSingleFunctionCallWrapper(t *testing.T) {
	p := &Planner{}
	input := `模型决定调用工具：{"function_call":{"name":"file_open","arguments":"{\"path\":\"doc/README.md\"}"}}`
	calls := p.parseSkillCalls(input)
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Name != "file_open" || calls[0].Args["path"] != "doc/README.md" {
		t.Fatalf("unexpected parsed call: %#v", calls[0])
	}
}

func TestParseSkillCallsAcceptsSingleToolCallWrapper(t *testing.T) {
	p := &Planner{}
	input := `{"tool_call":{"type":"function","function":{"name":"web_search","arguments":"{\"query\":\"planner\"}"}}}`
	calls := p.parseSkillCalls(input)
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Name != "web_search" || calls[0].Args["query"] != "planner" {
		t.Fatalf("unexpected parsed call: %#v", calls[0])
	}
}

func TestParseFunctionCallsTagAcceptsOpenAIFunctionWrapper(t *testing.T) {
	p := &Planner{}
	input := `<function_calls>
{"id":"call-1","type":"function","function":{"name":"file_open","arguments":"{\"path\":\"doc/README.md\"}"}}
</function_calls>`
	calls := p.parseSkillCalls(input)
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Name != "file_open" || calls[0].Args["path"] != "doc/README.md" {
		t.Fatalf("unexpected parsed call: %#v", calls[0])
	}
}

func TestParseSkillCallsNone(t *testing.T) {
	p := &Planner{}
	calls := p.parseSkillCalls("这是普通回复，没有技能调用。")
	if len(calls) != 0 {
		t.Fatalf("expected 0 calls, got %d", len(calls))
	}
}

func TestParseDAGStepsSkipsLeadingNonPlanArrays(t *testing.T) {
	reply := `说明：[1]
也可能先给一个参考数组：[{"note":"not a plan"}]
` + "```json" + `
[
  {"description":"读取 doc 技术蓝图","skill":"file_exec","args":{"path":"doc"},"depends_on":[]},
  {"description":"修复 planner","skill":"code_exec","args":{},"depends_on":[0]}
]
` + "```"

	steps, err := parseDAGSteps(reply)
	if err != nil {
		t.Fatalf("parse DAG steps: %v", err)
	}
	if len(steps) != 2 || steps[0].Description != "读取 doc 技术蓝图" || steps[1].DependsOn[0] != 0 {
		t.Fatalf("unexpected steps: %#v", steps)
	}
}

func TestParseDAGStepsHandlesBracketsInsideStrings(t *testing.T) {
	reply := `[{"description":"分析 [doc] 目录里的蓝图","skill":"","args":{"query":"planner [resume]"},"depends_on":[]}]`
	steps, err := parseDAGSteps(reply)
	if err != nil {
		t.Fatalf("parse DAG steps with brackets in strings: %v", err)
	}
	if len(steps) != 1 || steps[0].Description != "分析 [doc] 目录里的蓝图" || steps[0].Args["query"] != "planner [resume]" {
		t.Fatalf("unexpected steps: %#v", steps)
	}
}

func TestParseDAGStepsNormalizesUnsafeDependencies(t *testing.T) {
	reply := `[
  {"description":"第一步","skill":"","args":{},"depends_on":[0,1,-1,99]},
  {"description":"第二步","skill":"","args":{},"depends_on":[0,0,1,2]}
]`
	steps, err := parseDAGSteps(reply)
	if err != nil {
		t.Fatalf("parse DAG steps with unsafe dependencies: %v", err)
	}
	if len(steps[0].DependsOn) != 0 {
		t.Fatalf("first step should not keep self/future/out-of-range deps: %#v", steps[0].DependsOn)
	}
	if len(steps[1].DependsOn) != 1 || steps[1].DependsOn[0] != 0 {
		t.Fatalf("second step should keep only unique previous deps: %#v", steps[1].DependsOn)
	}
}

func TestParseDAGStepsAcceptsCommonAliasFields(t *testing.T) {
	reply := `[
  {"task":"读取技术蓝图","tool":"file_open","arguments":{"path":"doc/技术蓝图.md"},"depends":[]},
  {"step":"整理 planner 风险","tool_name":"file_search","params":{"query":"planner"},"dependencies":[0]},
  {"name":"形成修复计划","skill_name":"code_execute","args":{"command":"go test ./internal/agentcore/planner"},"dependsOn":[0,1,1]}
]`
	steps, err := parseDAGSteps(reply)
	if err != nil {
		t.Fatalf("parse DAG steps with aliases: %v", err)
	}
	if len(steps) != 3 {
		t.Fatalf("expected 3 steps, got %#v", steps)
	}
	if steps[0].Description != "读取技术蓝图" || steps[0].Skill != "file_open" || steps[0].Args["path"] != "doc/技术蓝图.md" {
		t.Fatalf("unexpected first step: %#v", steps[0])
	}
	if steps[1].Description != "整理 planner 风险" || steps[1].Skill != "file_search" || steps[1].DependsOn[0] != 0 {
		t.Fatalf("unexpected second step: %#v", steps[1])
	}
	if steps[2].Description != "形成修复计划" || steps[2].Skill != "code_execute" {
		t.Fatalf("unexpected third step identity: %#v", steps[2])
	}
	if len(steps[2].DependsOn) != 2 || steps[2].DependsOn[0] != 0 || steps[2].DependsOn[1] != 1 {
		t.Fatalf("third step should normalize duplicate deps: %#v", steps[2].DependsOn)
	}
}

func TestParseDAGStepsMapsStringInputToArgs(t *testing.T) {
	reply := `[{"action":"总结执行结果","tool":"file_search","input":"planner resume","depends_on":[]}]`
	steps, err := parseDAGSteps(reply)
	if err != nil {
		t.Fatalf("parse DAG steps with string input: %v", err)
	}
	if len(steps) != 1 || steps[0].Args["input"] != "planner resume" {
		t.Fatalf("expected string input to be preserved as args.input: %#v", steps)
	}
	if steps[0].Description != "总结执行结果" || steps[0].Skill != "file_search" {
		t.Fatalf("unexpected action/tool mapping: %#v", steps[0])
	}
}

func TestParseDAGStepsDoesNotUseActionAsSkillWhenToolProvided(t *testing.T) {
	reply := `[{"action":"读取 doc 技术蓝图","tool":"file_open","args":{"path":"doc"},"depends_on":[]}]`
	steps, err := parseDAGSteps(reply)
	if err != nil {
		t.Fatalf("parse DAG steps with action and tool: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %#v", steps)
	}
	if steps[0].Description != "读取 doc 技术蓝图" || steps[0].Skill != "file_open" {
		t.Fatalf("action should stay description and tool should stay skill: %#v", steps[0])
	}
}

func TestParseDAGStepsAcceptsStringAndOneBasedDependencies(t *testing.T) {
	reply := `[
  {"description":"读取蓝图","skill":"file_open","args":{"path":"doc"},"depends_on":[]},
  {"description":"分析 Planner","skill":"file_search","args":{"query":"planner"},"depends_on":["1"]},
  {"description":"整理结论","skill":"writer","args":{},"depends_on":["step 1","step 2","#2",2,99]}
]`
	steps, err := parseDAGSteps(reply)
	if err != nil {
		t.Fatalf("parse DAG steps with string/one-based deps: %v", err)
	}
	if len(steps) != 3 {
		t.Fatalf("expected 3 steps, got %#v", steps)
	}
	if len(steps[1].DependsOn) != 1 || steps[1].DependsOn[0] != 0 {
		t.Fatalf("second step should convert one-based string dep to previous step 0: %#v", steps[1].DependsOn)
	}
	if len(steps[2].DependsOn) != 2 || steps[2].DependsOn[0] != 0 || steps[2].DependsOn[1] != 1 {
		t.Fatalf("third step should normalize string/one-based deps and dedupe: %#v", steps[2].DependsOn)
	}
}

func TestParseDAGStepsAcceptsFunctionWrappedStepAndChineseDependencies(t *testing.T) {
	reply := `[
  {"description":"读取蓝图","function":{"name":"file_open","arguments":"{\"path\":\"doc\"}"},"depends_on":[]},
  {"description":"分析 Planner","function_call":{"name":"file_search","arguments":{"query":"planner"}},"depends_on":["第1步"]},
  {"description":"整理结论","tool_call":{"function":{"name":"writer","arguments":"{\"format\":\"markdown\"}"}},"depends_on":["步骤 1","前置步骤 2",2.0,"#2"]}
]`
	steps, err := parseDAGSteps(reply)
	if err != nil {
		t.Fatalf("parse DAG steps with wrapped function calls: %v", err)
	}
	if len(steps) != 3 {
		t.Fatalf("expected 3 steps, got %#v", steps)
	}
	if steps[0].Skill != "file_open" || steps[0].Args["path"] != "doc" {
		t.Fatalf("first function wrapper should become skill args, got %#v", steps[0])
	}
	if steps[1].Skill != "file_search" || steps[1].Args["query"] != "planner" {
		t.Fatalf("function_call wrapper should become skill args, got %#v", steps[1])
	}
	if len(steps[1].DependsOn) != 1 || steps[1].DependsOn[0] != 0 {
		t.Fatalf("Chinese one-based dependency should map to step 0, got %#v", steps[1].DependsOn)
	}
	if steps[2].Skill != "writer" || steps[2].Args["format"] != "markdown" {
		t.Fatalf("tool_call.function wrapper should become skill args, got %#v", steps[2])
	}
	if len(steps[2].DependsOn) != 2 || steps[2].DependsOn[0] != 0 || steps[2].DependsOn[1] != 1 {
		t.Fatalf("mixed Chinese/string/float deps should normalize and dedupe, got %#v", steps[2].DependsOn)
	}
}

func TestParseDAGStepsAcceptsFencedWrapperAndNestedArgsArrays(t *testing.T) {
	reply := `我会先给一个检查清单：["不是计划", "只是一段说明"]

` + "```json" + `
{
  "summary": "把 Office 附件解析结果交给 Planner 继续拆解",
  "steps": [
    {
      "description": "读取申请表字段",
      "tool": "file_parse",
      "args": {"path": "申请表.docx", "fields": ["公司名称", "联系电话"]},
      "depends_on": []
    },
    {
      "description": "生成阶段结果",
      "tool": "writer",
      "args": {"sections": ["已保留证据", "下一步"]},
      "depends_on": [1]
    }
  ]
}
` + "```" + `

后续 UI 可显示 ["阶段结果"]。`

	steps, err := parseDAGSteps(reply)
	if err != nil {
		t.Fatalf("parse DAG steps from fenced wrapper: %v", err)
	}
	if len(steps) != 2 {
		t.Fatalf("expected 2 steps, got %#v", steps)
	}
	fields, ok := steps[0].Args["fields"].([]any)
	if !ok || len(fields) != 2 || fields[0] != "公司名称" || fields[1] != "联系电话" {
		t.Fatalf("nested args array should be preserved, got %#v", steps[0].Args["fields"])
	}
	if len(steps[1].DependsOn) != 1 || steps[1].DependsOn[0] != 0 {
		t.Fatalf("one-based dependency in wrapper should map to first step, got %#v", steps[1].DependsOn)
	}
}

func TestParseDAGStepsAcceptsObjectDependencies(t *testing.T) {
	reply := `[
  {"description":"读取蓝图","tool":"file_open","args":{"path":"doc"},"depends_on":[]},
  {"description":"分析 Planner","tool":"file_search","args":{"query":"planner"},"depends_on":{"step":"第1步"}},
  {"description":"整理执行计划","tool":"writer","args":{},"depends_on":{"steps":["步骤 1", {"id": 2}, {"depends_on": ["#2"]}]}}
]`

	steps, err := parseDAGSteps(reply)
	if err != nil {
		t.Fatalf("parse DAG steps with object dependencies: %v", err)
	}
	if len(steps) != 3 {
		t.Fatalf("expected 3 steps, got %#v", steps)
	}
	if len(steps[1].DependsOn) != 1 || steps[1].DependsOn[0] != 0 {
		t.Fatalf("object step dependency should map to first step, got %#v", steps[1].DependsOn)
	}
	if len(steps[2].DependsOn) != 2 || steps[2].DependsOn[0] != 0 || steps[2].DependsOn[1] != 1 {
		t.Fatalf("object steps dependencies should flatten and dedupe, got %#v", steps[2].DependsOn)
	}
}

func TestParseDAGStepsCapsOverlongModelPlanToBlueprintLimit(t *testing.T) {
	reply := `[
  {"description":"步骤 1","tool":"writer","args":{"n":1},"depends_on":[]},
  {"description":"步骤 2","tool":"writer","args":{"n":2},"depends_on":[1]},
  {"description":"步骤 3","tool":"writer","args":{"n":3},"depends_on":[2]},
  {"description":"步骤 4","tool":"writer","args":{"n":4},"depends_on":[3]},
  {"description":"步骤 5","tool":"writer","args":{"n":5},"depends_on":[4]},
  {"description":"步骤 6","tool":"writer","args":{"n":6},"depends_on":[5]},
  {"description":"步骤 7","tool":"writer","args":{"n":7},"depends_on":[6]},
  {"description":"步骤 8","tool":"writer","args":{"n":8},"depends_on":[7]},
  {"description":"步骤 9","tool":"writer","args":{"n":9},"depends_on":[8]},
  {"description":"步骤 10","tool":"writer","args":{"n":10},"depends_on":[9]}
]`

	steps, err := parseDAGSteps(reply)
	if err != nil {
		t.Fatalf("parse overlong DAG steps: %v", err)
	}
	if len(steps) != 8 {
		t.Fatalf("blueprint long-horizon DAG should cap at 8 steps, got %d: %#v", len(steps), steps)
	}
	last := steps[len(steps)-1]
	if last.Description != "步骤 8" {
		t.Fatalf("expected first 8 model steps to be preserved, got last=%#v", last)
	}
	for i, step := range steps {
		for _, dep := range step.DependsOn {
			if dep < 0 || dep >= i {
				t.Fatalf("dependency should remain valid after cap, step=%d deps=%#v", i, step.DependsOn)
			}
		}
	}
}

func TestInitialDAGDecomposePadsTooShortModelPlanToBlueprintMinimum(t *testing.T) {
	reply := `[
  {"description":"读取技术蓝图","tool":"file_open","args":{"path":"doc"},"depends_on":[]},
  {"description":"修复 planner","tool":"code_execute","args":{"command":"go test ./internal/agentcore/planner"},"depends_on":[1]}
]`

	client := mockLLMServer(t, func(_ []llm.Message) string {
		return reply
	})
	p := NewPlanner(client, skills.NewRegistry(), 4)
	steps, err := p.buildDecomposeDAG(PlanRequest{})(context.Background(), "推进 planner")
	if err != nil {
		t.Fatalf("decompose short DAG steps: %v", err)
	}
	if len(steps) != 3 {
		t.Fatalf("blueprint long-horizon DAG should pad to 3 steps, got %d: %#v", len(steps), steps)
	}
	if steps[0].Description != "读取技术蓝图" || steps[1].Description != "修复 planner" {
		t.Fatalf("model steps should be preserved, got %#v", steps)
	}
	last := steps[2]
	if last.Description == "" || last.Skill != "" {
		t.Fatalf("synthetic final step should be a reasoning summary step, got %#v", last)
	}
	if len(last.DependsOn) != 2 || last.DependsOn[0] != 0 || last.DependsOn[1] != 1 {
		t.Fatalf("synthetic final step should depend on all prior steps, got %#v", last.DependsOn)
	}
}

func TestModelRuntimeServiceDecomposeLongHorizonDAGPadsMinimum(t *testing.T) {
	reply := `[{"description":"读取技术蓝图","tool":"file_open","args":{"path":"doc"},"depends_on":[]}]`
	client := mockLLMServer(t, func(messages []llm.Message) string {
		if len(messages) != 2 || !strings.Contains(messages[1].Content, "可用工具") || !strings.Contains(messages[1].Content, "推进 planner") {
			t.Fatalf("unexpected decompose messages %#v", messages)
		}
		return reply
	})

	steps, err := NewModelRuntimeService(client).DecomposeLongHorizonDAG(context.Background(), PlanRequest{}, "- file_open: read\n", "推进 planner")
	if err != nil {
		t.Fatalf("decompose via model runtime: %v", err)
	}
	if len(steps) != minLongHorizonDAGSteps {
		t.Fatalf("expected model runtime decompose to pad minimum steps, got %#v", steps)
	}
	if steps[0].Description != "读取技术蓝图" || steps[0].Skill != "file_open" {
		t.Fatalf("unexpected first step %#v", steps[0])
	}
}

func TestModelRuntimeServiceReviseLongHorizonDAG(t *testing.T) {
	reply := `[{"description":"改用阶段总结","tool":"","args":{},"depends_on":[]}]`
	client := mockLLMServer(t, func(messages []llm.Message) string {
		if len(messages) != 2 || !strings.Contains(messages[1].Content, "步骤 1 失败") || !strings.Contains(messages[1].Content, "状态:") {
			t.Fatalf("unexpected revise messages %#v", messages)
		}
		return reply
	})

	steps, err := NewModelRuntimeService(client).ReviseLongHorizonDAG(context.Background(), PlanRequest{}, "推进 planner", "[0] failed", 1)
	if err != nil {
		t.Fatalf("revise via model runtime: %v", err)
	}
	if len(steps) != 1 || steps[0].Description != "改用阶段总结" {
		t.Fatalf("unexpected revised steps %#v", steps)
	}
}

func TestModelRuntimeServiceExecuteLongHorizonReasoningStepUsesTier(t *testing.T) {
	client := mockLLMServer(t, func(messages []llm.Message) string {
		t.Fatalf("fast tier should not be used for explicit expert reasoning, messages=%#v", messages)
		return ""
	})
	expert := mockLLMServer(t, func(messages []llm.Message) string {
		if len(messages) != 2 || !strings.Contains(messages[1].Content, "分析依赖") {
			t.Fatalf("unexpected reasoning messages %#v", messages)
		}
		return "专家推理完成"
	})
	pool := llm.NewPool()
	pool.Register("fast", client)
	pool.Register("expert", expert)
	pool.SetPrimary("fast")
	service := NewModelRuntimeService(client)
	service.SetPool(pool)

	reply, err := service.ExecuteLongHorizonReasoningStep(context.Background(), PlanRequest{}, "expert", "分析依赖")
	if err != nil {
		t.Fatalf("reasoning via model runtime: %v", err)
	}
	if reply != "专家推理完成" {
		t.Fatalf("expected expert tier reasoning reply, got %q", reply)
	}
}

func TestModelRuntimeServiceSynthesizeLongHorizonResult(t *testing.T) {
	client := mockLLMServer(t, func(messages []llm.Message) string {
		if len(messages) != 2 || !strings.Contains(messages[1].Content, "目标: 推进 planner") || !strings.Contains(messages[1].Content, "步骤结果") {
			t.Fatalf("unexpected synthesize messages %#v", messages)
		}
		return "最终总结"
	})

	reply, err := NewModelRuntimeService(client).SynthesizeLongHorizonResult(context.Background(), PlanRequest{}, "推进 planner", "- 步骤结果")
	if err != nil {
		t.Fatalf("synthesize via model runtime: %v", err)
	}
	if reply != "最终总结" {
		t.Fatalf("unexpected synthesized reply %q", reply)
	}
}

func TestBuildSystemPrompt(t *testing.T) {
	reg := skills.NewRegistry()
	p := NewPlanner(nil, reg, 8)
	prompt := p.buildSystemPrompt()
	if prompt == "" {
		t.Fatal("expected non-empty system prompt")
	}
	if len(prompt) < 50 {
		t.Fatal("system prompt too short")
	}
}

func TestBuildFunctionDefsSubagentHonorsAllowedSkills(t *testing.T) {
	reg := skills.NewRegistry()
	for _, name := range []string{"file_open", "file_search", "browser_search", "code_execute"} {
		reg.Register(dummyPlannerSkill(name))
	}
	p := NewPlanner(nil, reg, 8)
	p.SetCogniRuntime(stubCogniRuntime{
		context: "cogni",
		trace:   CogniTraceDetail{Activated: []string{"demo"}},
	})
	p.SetHandoffRegistry(subagent.NewHandoffRegistry(subagent.NewManager()))

	defs := p.buildFunctionDefs("读取 docx", "", "", true, []string{" file_open ", "file_search", ""}, p.ensureContextAssembly(), p.ensureDelegationRuntime(), p.ensureSkillRuntime())
	got := map[string]bool{}
	for _, d := range defs {
		got[d.Name] = true
		if strings.HasPrefix(d.Name, "transfer_to_") {
			t.Fatalf("subagent direct mode must not expose handoff tool %s", d.Name)
		}
	}
	if !got["file_open"] || !got["file_search"] {
		t.Fatalf("expected file tools, got %#v", got)
	}
	if got["browser_search"] || got["code_execute"] {
		t.Fatalf("subagent allowed skills leaked unrelated tools: %#v", got)
	}
}

func TestReActToolsDescriptionHonorsAllowedSkills(t *testing.T) {
	reg := skills.NewRegistry()
	for _, name := range []string{"file_open", "file_search", "browser_search", "code_execute"} {
		reg.Register(dummyPlannerSkill(name))
	}
	p := NewPlanner(nil, reg, 8)

	desc := p.buildToolsDescription([]string{" file_open ", "file_search", ""})
	if !strings.Contains(desc, "file_open") || !strings.Contains(desc, "file_search") {
		t.Fatalf("expected allowed tools in ReAct description: %q", desc)
	}
	if strings.Contains(desc, "browser_search") || strings.Contains(desc, "code_execute") {
		t.Fatalf("ReAct description leaked disallowed tools: %q", desc)
	}
}

func TestAssessCognitiveLoadRoutesRoadmapPlannerWork(t *testing.T) {
	req := PlanRequest{Messages: []llm.Message{{Role: "user", Content: "请根据 doc 技术蓝图和当前代码问题，继续推进云雀，拆解路线图，把 planner 做成功，完善后端、前端、子代理和测试，并一直执行下去。"}}}
	load := assessCognitiveLoad(req)
	if !load.NeedsLongHorizon() {
		t.Fatalf("expected high cognitive load, got level=%s score=%d signals=%v domains=%v", load.Level, load.Score, load.Signals, load.Domains)
	}
	if len(load.Domains) < 3 {
		t.Fatalf("expected multi-domain assessment, got %#v", load.Domains)
	}
}

func TestAssessCognitiveLoadKeepsGreetingLight(t *testing.T) {
	req := PlanRequest{Messages: []llm.Message{{Role: "user", Content: "你好呀"}}}
	load := assessCognitiveLoad(req)
	if load.NeedsLongHorizon() || load.Level != CognitiveLoadLow {
		t.Fatalf("expected low load greeting, got level=%s score=%d", load.Level, load.Score)
	}
}

func TestPlannerShouldUseLongHorizonFromCognitiveLoad(t *testing.T) {
	p := NewPlanner(nil, skills.NewRegistry(), 8)
	p.SetLongHorizonMode(true)
	req := PlanRequest{Messages: []llm.Message{{Role: "user", Content: "继续扫描代码和 doc 技术蓝图，拆解路线图，修复 planner、子代理、文件上传和测试问题，并验证全部核心链路。"}}}
	if !p.shouldUseLongHorizon(req) {
		t.Fatal("expected cognitive load to route into long horizon mode")
	}
}

func TestEmitCognitiveLoadEventCarriesDetail(t *testing.T) {
	p := NewPlanner(nil, skills.NewRegistry(), 8)
	var got observe.AgentEvent
	req := PlanRequest{
		TraceID:  "trace-load",
		TenantID: "tenant-a",
		TaskID:   "task-a",
		StepCallback: func(evt observe.AgentEvent) {
			got = evt
		},
	}
	load := CognitiveLoadAssessment{Level: CognitiveLoadHigh, Score: 6, Signals: []string{"multi_action_request"}}
	p.emitCognitiveLoadEvent(req, load)
	if got.Type != observe.EventPlan || got.Meta.TaskID != "task-a" {
		t.Fatalf("unexpected event: %#v", got)
	}
	detail, ok := got.Detail.(CognitiveLoadAssessment)
	if !ok || detail.Score != 6 {
		t.Fatalf("expected cognitive load detail, got %#v", got.Detail)
	}
}

func TestEmitCogniTraceEventCarriesSurfaceDetail(t *testing.T) {
	p := NewPlanner(nil, skills.NewRegistry(), 8)
	p.SetCogniRuntime(stubCogniRuntime{
		trace: CogniTraceDetail{
			Activated:       []string{"文档助手"},
			ContextBytes:    128,
			ToolBefore:      12,
			ToolAfter:       3,
			Removed:         []string{"browser_search"},
			MessageHash:     "abc123",
			FellBackToInput: false,
		},
	})
	var got observe.AgentEvent
	req := PlanRequest{
		TraceID:     "trace-cogni",
		TenantID:    "tenant-a",
		TaskID:      "task-a",
		ChannelType: "web",
		Messages:    []llm.Message{{Role: "user", Content: "需要读取文档"}},
		StepCallback: func(evt observe.AgentEvent) {
			got = evt
		},
	}
	p.contextAssembly.EmitCogniTraceForRequest(req)
	if got.Type != observe.EventPlan || got.Meta.TaskID != "task-a" {
		t.Fatalf("unexpected event: %#v", got)
	}
	if !strings.Contains(got.Summary, "文档助手") || !strings.Contains(got.Summary, "12 → 3") {
		t.Fatalf("unexpected summary: %q", got.Summary)
	}
	detail, ok := got.Detail.(CogniTraceDetail)
	if !ok || detail.ContextBytes != 128 || detail.ToolAfter != 3 || len(detail.Removed) != 1 {
		t.Fatalf("expected CogniTraceDetail, got %#v", got.Detail)
	}
}

func TestEmitCogniTraceEventSkipsNoVisibleEffect(t *testing.T) {
	p := NewPlanner(nil, skills.NewRegistry(), 8)
	p.SetCogniRuntime(stubCogniRuntime{})
	called := false
	req := PlanRequest{
		Messages: []llm.Message{{Role: "user", Content: "你好"}},
		StepCallback: func(observe.AgentEvent) {
			called = true
		},
	}
	p.contextAssembly.EmitCogniTraceForRequest(req)
	if called {
		t.Fatal("empty Cogni trace should not emit a user-visible event")
	}
}

func TestModelFallbackSummaryAvoidsEngineeringTerms(t *testing.T) {
	summary := modelFallbackSummary("qwen3.5:4b")
	for _, banned := range []string{"调用栈", "级联", "唤醒", "降级", "fallback"} {
		if strings.Contains(strings.ToLower(summary), strings.ToLower(banned)) {
			t.Fatalf("fallback summary should be natural, got %q", summary)
		}
	}
	if !strings.Contains(summary, "正在换用 qwen3.5:4b 继续") {
		t.Fatalf("summary should mention model naturally, got %q", summary)
	}
}

func TestEmitModelFallbackEventCarriesNaturalDetail(t *testing.T) {
	p := NewPlanner(nil, skills.NewRegistry(), 8)
	var got observe.AgentEvent
	p.emitModelFallbackEvent(PlanRequest{
		TraceID:  "trace-fallback",
		TenantID: "tenant-a",
		TaskID:   "task-a",
		StepCallback: func(evt observe.AgentEvent) {
			got = evt
		},
	}, "backup-model", 2, fmt.Errorf("EOF"))
	if got.Summary != "模型暂时没有回应，正在换用 backup-model 继续。" {
		t.Fatalf("unexpected summary: %q", got.Summary)
	}
	detail, ok := got.Detail.(ModelFallbackDetail)
	if !ok || detail.Attempt != 2 || detail.Model != "backup-model" || detail.Reason == "" {
		t.Fatalf("unexpected detail: %#v", got.Detail)
	}
	assertPlannerTextHasNoRawDiagnostics(t, detail.Reason)
	if !strings.Contains(detail.Reason, "现场已保留") {
		t.Fatalf("expected recoverable wording in reason, got %q", detail.Reason)
	}
	if got.Meta.TaskID != "task-a" || got.Meta.TenantID != "tenant-a" {
		t.Fatalf("missing meta: %#v", got.Meta)
	}
}

func TestFileLongHorizonCheckpointStorePersistsAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "planner", "checkpoints.jsonl")
	store := NewFileLongHorizonCheckpointStore(path)
	cp := LongHorizonCheckpoint{
		PlanID:       "plan-a",
		TaskID:       "task-a",
		Status:       "failed",
		Completed:    1,
		Total:        2,
		Recoverable:  true,
		Error:        "boom",
		PlanSnapshot: []PlanStep{{ID: 1, Action: "失败步骤", Skill: "bad_skill", Status: StepFailed, Error: "boom"}},
	}
	if err := store.Save(context.Background(), cp); err != nil {
		t.Fatalf("save checkpoint: %v", err)
	}
	reopened := NewFileLongHorizonCheckpointStore(path)
	recent, err := reopened.Recent(context.Background(), 10)
	if err != nil {
		t.Fatalf("load recent checkpoints: %v", err)
	}
	if len(recent) != 1 || recent[0].PlanID != "plan-a" || !recent[0].Recoverable || len(recent[0].PlanSnapshot) != 1 {
		t.Fatalf("unexpected checkpoints: %#v", recent)
	}
	if recent[0].UpdatedAt.IsZero() {
		t.Fatalf("expected updated_at to be populated: %#v", recent[0])
	}
}

func TestFileLongHorizonCheckpointStoreRecentDeduplicatesPlanUpdates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "planner", "checkpoints.jsonl")
	store := NewFileLongHorizonCheckpointStore(path)
	for _, cp := range []LongHorizonCheckpoint{
		{PlanID: "plan-a", TaskID: "task-a", Completed: 0, Total: 3, Recoverable: true},
		{PlanID: "plan-b", TaskID: "task-b", Completed: 1, Total: 2, Recoverable: true},
		{PlanID: "plan-a", TaskID: "task-a", Completed: 2, Total: 3, Recoverable: true, Error: "latest"},
	} {
		if err := store.Save(context.Background(), cp); err != nil {
			t.Fatalf("save checkpoint %s: %v", cp.PlanID, err)
		}
	}
	recent, err := store.Recent(context.Background(), 10)
	if err != nil {
		t.Fatalf("load recent checkpoints: %v", err)
	}
	if len(recent) != 2 {
		t.Fatalf("expected 2 unique plans, got %#v", recent)
	}
	if recent[0].PlanID != "plan-a" || recent[0].Completed != 2 || recent[0].Error != "latest" {
		t.Fatalf("expected newest plan-a update first, got %#v", recent[0])
	}
	if recent[1].PlanID != "plan-b" {
		t.Fatalf("expected older unique plan-b second, got %#v", recent[1])
	}
	limited, err := store.Recent(context.Background(), 1)
	if err != nil {
		t.Fatalf("load limited checkpoints: %v", err)
	}
	if len(limited) != 1 || limited[0].PlanID != "plan-a" || limited[0].Completed != 2 {
		t.Fatalf("limit should count unique newest plans, got %#v", limited)
	}
}

func TestFileLongHorizonCheckpointStoreRecentForTenantScopesAndDeduplicates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "planner", "checkpoints.jsonl")
	store := NewFileLongHorizonCheckpointStore(path)
	for _, cp := range []LongHorizonCheckpoint{
		{PlanID: "plan-shared", TenantID: "tenant-a", TaskID: "task-a-old", Completed: 0, Total: 2, Recoverable: true},
		{PlanID: "plan-shared", TenantID: "tenant-b", TaskID: "task-b", Completed: 1, Total: 2, Recoverable: true},
		{PlanID: "plan-shared", TenantID: "tenant-a", TaskID: "task-a-new", Completed: 2, Total: 2, Recoverable: true, Error: "latest-a"},
	} {
		if err := store.Save(context.Background(), cp); err != nil {
			t.Fatalf("save checkpoint %s/%s: %v", cp.TenantID, cp.PlanID, err)
		}
	}
	recentA, err := store.RecentForTenant(context.Background(), "tenant-a", 10)
	if err != nil {
		t.Fatalf("load tenant-a checkpoints: %v", err)
	}
	if len(recentA) != 1 || recentA[0].TaskID != "task-a-new" || recentA[0].Error != "latest-a" {
		t.Fatalf("expected latest tenant-a checkpoint only, got %#v", recentA)
	}
	recentB, err := store.RecentForTenant(context.Background(), "tenant-b", 10)
	if err != nil {
		t.Fatalf("load tenant-b checkpoints: %v", err)
	}
	if len(recentB) != 1 || recentB[0].TaskID != "task-b" {
		t.Fatalf("expected tenant-b checkpoint only, got %#v", recentB)
	}
}

func TestEmitLongHorizonCheckpointPersistsSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "checkpoints.jsonl")
	store := NewFileLongHorizonCheckpointStore(path)
	p := NewPlanner(nil, skills.NewRegistry(), 4)
	p.SetLongHorizonCheckpointStore(store)
	mgr := plan.NewManager(nil, nil)
	pl := mgr.CreateFromSteps("persist me", []string{"读取文档"})
	pl.Steps[0].Skill = "file_open"

	p.emitLongHorizonCheckpoint(PlanRequest{TraceID: "trace", TaskID: "task-persist", TenantID: "tenant-a", StepCallback: func(observe.AgentEvent) {}}, pl, "boom")

	recent, err := store.Recent(context.Background(), 1)
	if err != nil {
		t.Fatalf("load recent checkpoints: %v", err)
	}
	if len(recent) != 1 || recent[0].TaskID != "task-persist" || recent[0].Error != "boom" || !recent[0].Recoverable {
		t.Fatalf("checkpoint was not persisted with recoverable failure: %#v", recent)
	}
}

func TestEmitLongHorizonCheckpointFriendlyEventKeepsRawPersisted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "checkpoints.jsonl")
	store := NewFileLongHorizonCheckpointStore(path)
	p := NewPlanner(nil, skills.NewRegistry(), 4)
	p.SetLongHorizonCheckpointStore(store)
	pl := &plan.Plan{
		ID:     "plan-friendly-event",
		Task:   "推进 planner",
		Status: plan.PlanFailed,
		Steps: []plan.PlanStep{
			{Index: 0, Description: "失败步骤", Skill: "file_exec", Status: plan.StepFailed, Error: "handoff agent execution failed: context deadline exceeded EOF"},
		},
	}

	var evtCP LongHorizonCheckpoint
	p.emitLongHorizonCheckpoint(PlanRequest{
		TraceID:  "trace-friendly-event",
		TaskID:   "task-friendly-event",
		TenantID: "tenant-a",
		StepCallback: func(evt observe.AgentEvent) {
			if cp, ok := evt.Detail.(LongHorizonCheckpoint); ok {
				evtCP = cp
			}
		},
	}, pl, "handoff agent execution failed: context deadline exceeded EOF")

	if evtCP.Error == "" || len(evtCP.PlanSnapshot) != 1 || evtCP.PlanSnapshot[0].Error == "" {
		t.Fatalf("expected checkpoint detail in event, got %#v", evtCP)
	}
	assertPlannerTextHasNoRawDiagnostics(t, evtCP.Error)
	assertPlannerTextHasNoRawDiagnostics(t, evtCP.PlanSnapshot[0].Error)
	recent, err := store.Recent(context.Background(), 1)
	if err != nil {
		t.Fatalf("load persisted checkpoints: %v", err)
	}
	if len(recent) != 1 || !strings.Contains(strings.ToLower(recent[0].Error), "context deadline exceeded") {
		t.Fatalf("persisted checkpoint should keep raw diagnostic evidence, got %#v", recent)
	}
	if len(recent[0].PlanSnapshot) != 1 || !strings.Contains(strings.ToLower(recent[0].PlanSnapshot[0].Error), "handoff agent") {
		t.Fatalf("persisted snapshot should keep raw step error, got %#v", recent[0].PlanSnapshot)
	}
}

func TestHandoffFailureEventKeepsRawErrorOutOfUserText(t *testing.T) {
	err := fmt.Errorf("handoff agent \"file_exec\" execution failed: %w", context.DeadlineExceeded)
	summary := handoffFailureSummary("file_exec", err)
	if !strings.Contains(summary, "响应超时") {
		t.Fatalf("expected timeout wording, got %q", summary)
	}
	assertPlannerTextHasNoRawDiagnostics(t, summary)
	detail := buildHandoffFailureDetail("file_exec", 1500*time.Millisecond, err)
	if !detail.Recoverable || detail.NextStep == "" || detail.Error == "" {
		t.Fatalf("expected recoverable detail with friendly error, got %#v", detail)
	}
	assertPlannerTextHasNoRawDiagnostics(t, detail.Error)
	if !strings.Contains(detail.Error, "现场已保留") {
		t.Fatalf("expected recoverable wording in detail error, got %q", detail.Error)
	}
}

func TestTextHandoffFailureEmitsRecoverableEvent(t *testing.T) {
	callCount := 0
	client := mockLLMServer(t, func(_ []llm.Message) string {
		callCount++
		if callCount == 1 {
			return `{"tool_calls":[{"name":"transfer_to_file_exec","arguments":{"input":"读取附件并总结"}}]}`
		}
		return "已切换策略，先返回可恢复结论。"
	})
	p := NewPlanner(client, skills.NewRegistry(), 2)
	hr := subagent.NewHandoffRegistry(subagent.NewManager())
	if err := hr.Register(subagent.HandoffConfig{Name: "file_exec", Description: "文件解析"}); err != nil {
		t.Fatalf("register handoff: %v", err)
	}
	hr.SetRunFunc(func(context.Context, string, string, string) (string, error) {
		return "", context.DeadlineExceeded
	})
	p.SetHandoffRegistry(hr)

	var done observe.AgentEvent
	_, err := p.runTextBased(context.Background(), PlanRequest{
		TraceID:  "trace-handoff",
		TenantID: "tenant-a",
		Messages: []llm.Message{{Role: "user", Content: "请读取附件"}},
		StepCallback: func(evt observe.AgentEvent) {
			if evt.Type == observe.EventHandoffDone {
				done = evt
			}
		},
	})
	if err != nil {
		t.Fatalf("runTextBased failed: %v", err)
	}
	if done.Type != observe.EventHandoffDone {
		t.Fatalf("expected handoff_done event, got %#v", done)
	}
	if !strings.Contains(done.Summary, "响应超时") {
		t.Fatalf("unexpected summary: %q", done.Summary)
	}
	assertPlannerTextHasNoRawDiagnostics(t, done.Summary)
	detail, ok := done.Detail.(observe.HandoffDetail)
	if !ok || !detail.Recoverable || detail.NextStep == "" || detail.Error == "" {
		t.Fatalf("expected recoverable handoff detail, got %#v", done.Detail)
	}
	assertPlannerTextHasNoRawDiagnostics(t, detail.Error)
}

func assertPlannerTextHasNoRawDiagnostics(t *testing.T, text string) {
	t.Helper()
	rawTerms := []string{
		"调用栈降级",
		"级联唤醒",
		"fallback",
		"execution failed",
		"context canceled",
		"context cancelled",
		"context deadline exceeded",
		"deadline exceeded",
		"EOF",
		"handoff agent",
		"unknown skill",
		"tool panic",
		"trust gate",
	}
	lower := strings.ToLower(text)
	for _, term := range rawTerms {
		if strings.Contains(lower, strings.ToLower(term)) {
			t.Fatalf("text should hide raw diagnostic %q, got %q", term, text)
		}
	}
}

func TestLongHorizonSkillListHonorsAllowedSkills(t *testing.T) {
	reg := skills.NewRegistry()
	reg.Register(dummyPlannerSkill("file_open"))
	reg.Register(dummyPlannerSkill("browser_search"))
	p := NewPlanner(nil, reg, 8)
	list := p.buildSkillListForDecomposeWithAllow(allowedSkillSet([]string{"file_open"}))
	if !strings.Contains(list, "file_open") {
		t.Fatalf("expected allowed skill in list: %q", list)
	}
	if strings.Contains(list, "browser_search") {
		t.Fatalf("unexpected disallowed skill in list: %q", list)
	}
}

func TestRunLongHorizonFailureReturnsRecoverablePlan(t *testing.T) {
	callCount := 0
	client := mockLLMServer(t, func(_ []llm.Message) string {
		callCount++
		if callCount == 1 {
			return `[{"description":"执行会失败的步骤","skill":"bad_skill","args":{},"depends_on":[]}]`
		}
		return `[]`
	})
	reg := skills.NewRegistry()
	reg.Register(&mockSkill{
		name: "bad_skill", desc: "fails",
		execFn: func(_ context.Context, _ map[string]any, _ *skills.Environment) (string, error) {
			return "", fmt.Errorf("boom")
		},
	})
	p := NewPlanner(client, reg, 4)

	var checkpoints []LongHorizonCheckpoint
	result, err := p.runLongHorizon(context.Background(), PlanRequest{
		TraceID:  "trace-long",
		TaskID:   "task-long",
		TenantID: "tenant-a",
		Messages: []llm.Message{{Role: "user", Content: "请先做失败步骤，然后继续"}},
		StepCallback: func(evt observe.AgentEvent) {
			if cp, ok := evt.Detail.(LongHorizonCheckpoint); ok {
				checkpoints = append(checkpoints, cp)
			}
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || len(result.Plan) != 3 {
		t.Fatalf("expected partial plan in result, got %#v", result)
	}
	if result.Plan[0].Status != StepFailed || !strings.Contains(result.Plan[0].Error, "boom") {
		t.Fatalf("expected failed step retained, got %+v", result.Plan[0])
	}
	if result.Plan[1].Status != StepPending || result.Plan[2].Status != StepPending {
		t.Fatalf("expected synthetic summary steps to remain pending after first failure, got %+v", result.Plan)
	}
	if len(checkpoints) == 0 {
		t.Fatal("expected long-horizon checkpoints to be emitted")
	}
	last := checkpoints[len(checkpoints)-1]
	if !last.Recoverable || last.TaskID != "task-long" || last.Error == "" || len(last.PlanSnapshot) != 3 {
		t.Fatalf("expected recoverable final checkpoint, got %+v", last)
	}
	if last.PlanSnapshot[0].Status != StepFailed {
		t.Fatalf("expected failed step in checkpoint snapshot, got %+v", last.PlanSnapshot[0])
	}
}

func TestRunLongHorizonFailureCallbackHidesRawError(t *testing.T) {
	callCount := 0
	client := mockLLMServer(t, func(_ []llm.Message) string {
		callCount++
		if callCount == 1 {
			return `[{"description":"执行会超时的步骤","skill":"timeout_skill","args":{},"depends_on":[]}]`
		}
		return `[]`
	})
	reg := skills.NewRegistry()
	reg.Register(&mockSkill{
		name: "timeout_skill", desc: "timeout",
		execFn: func(_ context.Context, _ map[string]any, _ *skills.Environment) (string, error) {
			return "", context.DeadlineExceeded
		},
	})
	p := NewPlanner(client, reg, 4)

	var failedSummary string
	result, err := p.runLongHorizon(context.Background(), PlanRequest{
		TraceID:  "trace-friendly",
		TaskID:   "task-friendly",
		TenantID: "tenant-a",
		Messages: []llm.Message{{Role: "user", Content: "请执行一个会超时的长程任务"}},
		StepCallback: func(evt observe.AgentEvent) {
			if evt.Type == observe.EventToolResult && strings.Contains(evt.Summary, "暂停") {
				failedSummary = evt.Summary
			}
		},
	})
	if err != nil {
		t.Fatalf("run long horizon: %v", err)
	}
	if failedSummary == "" {
		t.Fatal("expected failed step callback summary")
	}
	for _, raw := range []string{"context deadline exceeded", "execution failed", "fallback", "EOF"} {
		if strings.Contains(strings.ToLower(failedSummary), strings.ToLower(raw)) {
			t.Fatalf("callback summary should hide raw error %q, got %q", raw, failedSummary)
		}
		if result != nil && strings.Contains(strings.ToLower(result.Reply), strings.ToLower(raw)) {
			t.Fatalf("partial reply should hide raw error %q, got %q", raw, result.Reply)
		}
	}
	if !strings.Contains(failedSummary, "等待时间过长") || !strings.Contains(failedSummary, "现场已保留") {
		t.Fatalf("expected friendly recoverable wording, got %q", failedSummary)
	}
}

func TestLongHorizonFriendlyFailureTextCoversToolExecutionFailures(t *testing.T) {
	cases := map[string][]string{
		"unknown skill: file_exec":          {"unknown skill"},
		"blocked by trust gate: need allow": {"blocked by trust gate", "trust gate"},
		"tool panic: nil pointer":           {"tool panic", "panic"},
	}
	for raw, banned := range cases {
		friendly := plannerFriendlyFailureText(raw)
		if friendly == "" {
			t.Fatalf("expected friendly mapping for %q", raw)
		}
		lower := strings.ToLower(friendly)
		for _, term := range banned {
			if strings.Contains(lower, strings.ToLower(term)) {
				t.Fatalf("friendly text for %q still exposes %q: %q", raw, term, friendly)
			}
		}
		if !strings.Contains(friendly, "现场已保留") {
			t.Fatalf("friendly text should keep recovery wording for %q, got %q", raw, friendly)
		}
	}
}

func TestSynthesizePlanResultHidesRawDiagnosticsFromSummaryPrompt(t *testing.T) {
	var summaryPrompt string
	client := mockLLMServer(t, func(msgs []llm.Message) string {
		for _, msg := range msgs {
			if msg.Role == "user" && strings.Contains(msg.Content, "目标:") {
				summaryPrompt = msg.Content
			}
		}
		return "最终总结"
	})
	p := NewPlanner(client, skills.NewRegistry(), 4)
	pl := &plan.Plan{
		ID:     "plan-summary-friendly",
		Task:   "汇总阶段结果",
		Status: plan.PlanCompleted,
		Steps: []plan.PlanStep{
			{
				Index:       0,
				Description: "调用子代理读取附件",
				Status:      plan.StepCompleted,
				Output:      "子代理返回：handoff agent execution failed: context deadline exceeded EOF，但现场已保留。",
			},
		},
	}

	reply := p.synthesizePlanResult(context.Background(), PlanRequest{}, pl)
	if reply != "最终总结" {
		t.Fatalf("unexpected reply: %q", reply)
	}
	if summaryPrompt == "" {
		t.Fatal("expected summary prompt to be captured")
	}
	assertPlannerTextHasNoRawDiagnostics(t, summaryPrompt)
	if !strings.Contains(summaryPrompt, "现场已保留") {
		t.Fatalf("expected friendly recoverable wording in summary prompt, got %q", summaryPrompt)
	}
}

func TestLongHorizonDependencyPromptsHideRawCompletedDiagnostics(t *testing.T) {
	var prompts []string
	client := mockLLMServer(t, func(msgs []llm.Message) string {
		for _, msg := range msgs {
			if msg.Role == "user" {
				prompts = append(prompts, msg.Content)
			}
		}
		if len(msgs) > 0 && strings.Contains(msgs[len(msgs)-1].Content, "汇总阶段证据") {
			return "已汇总阶段证据"
		}
		return `[{"description":"继续执行","skill":"capture_evidence","args":{},"depends_on":[]}]`
	})
	reg := skills.NewRegistry()
	var skillArgs map[string]any
	reg.Register(&mockSkill{
		name: "capture_evidence", desc: "captures dependency evidence",
		execFn: func(_ context.Context, args map[string]any, _ *skills.Environment) (string, error) {
			skillArgs = args
			return "已接收证据", nil
		},
	})
	p := NewPlanner(client, reg, 4)
	rawOutput := "子代理返回：handoff agent execution failed: context deadline exceeded EOF，但现场已保留。"
	pl := &plan.Plan{Steps: []plan.PlanStep{
		{Index: 0, Description: "读取附件", Status: plan.StepCompleted, Output: rawOutput},
		{Index: 1, Description: "汇总阶段证据", Status: plan.StepPending, DependsOn: []int{0}},
		{Index: 2, Description: "调用工具继续", Skill: "capture_evidence", Status: plan.StepPending, DependsOn: []int{0}},
	}}

	if got := friendlyPlanStepSummary(pl); strings.Contains(got, "现场已保留") {
		assertPlannerTextHasNoRawDiagnostics(t, got)
	} else {
		t.Fatalf("expected friendly evidence in plan summary, got %q", got)
	}
	if _, _, err := p.executeReasoningStep(context.Background(), PlanRequest{}, p.ensureModelRuntime(), p.ensureRuntimeStrategy(), pl, 1); err != nil {
		t.Fatalf("reasoning step: %v", err)
	}
	if _, _, err := p.buildStepExecutor(PlanRequest{})(context.Background(), pl, 2); err != nil {
		t.Fatalf("skill step: %v", err)
	}
	if len(prompts) == 0 {
		t.Fatal("expected reasoning prompt")
	}
	reasoningPrompt := prompts[len(prompts)-1]
	if !strings.Contains(reasoningPrompt, "现场已保留") {
		t.Fatalf("expected friendly recovery wording in reasoning prompt, got %q", reasoningPrompt)
	}
	assertPlannerTextHasNoRawDiagnostics(t, reasoningPrompt)
	evidence, _ := skillArgs["dependency_results"].(string)
	if !strings.Contains(evidence, "现场已保留") {
		t.Fatalf("expected friendly dependency evidence, got %#v", skillArgs)
	}
	assertPlannerTextHasNoRawDiagnostics(t, evidence)
}

func TestFriendlyLongHorizonCheckpointHidesRawCompletedOutputInEventOnly(t *testing.T) {
	rawOutput := "子代理返回：handoff agent execution failed: context deadline exceeded EOF，但现场已保留。"
	cp := LongHorizonCheckpoint{
		PlanID:      "plan-friendly-output",
		TaskID:      "task-friendly-output",
		Status:      "running",
		Recoverable: true,
		PlanSnapshot: []PlanStep{
			{ID: 0, Action: "读取附件", Status: StepDone, Result: rawOutput},
		},
	}

	friendly := friendlyLongHorizonCheckpoint(cp)
	if got := friendly.PlanSnapshot[0].Result; !strings.Contains(got, "现场已保留") {
		t.Fatalf("expected friendly output in event checkpoint, got %q", got)
	}
	assertPlannerTextHasNoRawDiagnostics(t, friendly.PlanSnapshot[0].Result)
	if !strings.Contains(cp.PlanSnapshot[0].Result, "context deadline exceeded") {
		t.Fatalf("original checkpoint should keep raw persisted evidence, got %#v", cp.PlanSnapshot[0])
	}
}

func TestBuildReviseFuncHidesRawStepErrorInPrompt(t *testing.T) {
	var revisePrompt string
	client := mockLLMServer(t, func(msgs []llm.Message) string {
		if len(msgs) > 0 {
			revisePrompt = msgs[len(msgs)-1].Content
		}
		return `[{"description":"改用直接工具继续","skill":"file_open","args":{"path":"doc"},"depends_on":[]}]`
	})
	p := NewPlanner(client, skills.NewRegistry(), 4)
	pl := &plan.Plan{
		Task: "推进云雀 planner",
		Steps: []plan.PlanStep{
			{Index: 0, Description: "读取蓝图", Skill: "file_open", Status: plan.StepCompleted, Output: "已读取"},
			{Index: 1, Description: "委派解析", Skill: "transfer_to_file_exec", Status: plan.StepFailed, Error: "handoff agent execution failed: context deadline exceeded EOF"},
		},
	}

	steps, err := p.buildReviseFunc(PlanRequest{TenantID: "test"})(context.Background(), pl.Task, pl, 1)
	if err != nil {
		t.Fatalf("revise func: %v", err)
	}
	if len(steps) != 1 || steps[0].Skill != "file_open" {
		t.Fatalf("unexpected revised steps: %#v", steps)
	}
	if !strings.Contains(revisePrompt, "现场已保留") {
		t.Fatalf("expected friendly recovery wording in revise prompt, got %q", revisePrompt)
	}
	assertPlannerTextHasNoRawDiagnostics(t, revisePrompt)
	if !strings.Contains(pl.StepSummary(), "context deadline exceeded") {
		t.Fatalf("raw plan summary should remain available for internal diagnostics")
	}
}

func TestResumeLongHorizonCheckpointContinuesWithoutRerunningCompleted(t *testing.T) {
	client := mockLLMServer(t, func(_ []llm.Message) string {
		return "恢复完成总结"
	})
	var rerunDone atomic.Int32
	var ranNext atomic.Int32
	reg := skills.NewRegistry()
	reg.Register(&mockSkill{
		name: "already_done", desc: "must not rerun",
		execFn: func(_ context.Context, _ map[string]any, _ *skills.Environment) (string, error) {
			rerunDone.Add(1)
			return "should not run", nil
		},
	})
	reg.Register(&mockSkill{
		name: "next_step", desc: "next",
		execFn: func(_ context.Context, args map[string]any, _ *skills.Environment) (string, error) {
			ranNext.Add(1)
			if args["path"] != "doc" {
				t.Fatalf("expected checkpoint args to be preserved, got %#v", args)
			}
			return "next ok", nil
		},
	})
	p := NewPlanner(client, reg, 4)
	cp := LongHorizonCheckpoint{
		PlanID:      "plan-resume",
		TaskID:      "task-resume",
		Goal:        "继续推进 planner",
		Status:      "failed",
		Completed:   1,
		Total:       2,
		StepsUsed:   1,
		Recoverable: true,
		PlanSnapshot: []PlanStep{
			{ID: 0, Action: "已完成读取蓝图", Skill: "already_done", Status: StepDone, Result: "docs read"},
			{ID: 1, Action: "继续修复 planner", Skill: "next_step", Args: map[string]any{"path": "doc"}, DependsOn: []int{0}, Status: StepPending},
		},
	}

	result, err := p.ResumeLongHorizonCheckpoint(context.Background(), PlanRequest{TenantID: "tenant-a", TaskID: "task-resume"}, cp, "continue")
	if err != nil {
		t.Fatalf("resume checkpoint: %v", err)
	}
	if rerunDone.Load() != 0 {
		t.Fatalf("completed checkpoint step was re-run %d times", rerunDone.Load())
	}
	if ranNext.Load() != 1 {
		t.Fatalf("expected next step to run once, got %d", ranNext.Load())
	}
	if result == nil || len(result.Plan) != 2 {
		t.Fatalf("expected resumed plan result, got %#v", result)
	}
	if result.Plan[0].Status != StepDone || result.Plan[0].Result != "docs read" {
		t.Fatalf("completed output should be preserved, got %+v", result.Plan[0])
	}
	if result.Plan[1].Status != StepDone || result.Plan[1].Result != "next ok" {
		t.Fatalf("pending step should be executed, got %+v", result.Plan[1])
	}
}

func TestNormalizeCheckpointResumeActionAcceptsUiAliases(t *testing.T) {
	cases := map[string]string{
		"":                      "continue",
		" Resume_Plan ":         "continue",
		"继续":                    "continue",
		"retry-failed-step":     "retry_failed",
		" 重试失败 ":                "retry_failed",
		"return-partial-result": "partial",
		"返回阶段结果":                "partial",
		"unknown":               "",
	}
	for input, want := range cases {
		if got := NormalizeCheckpointResumeAction(input); got != want {
			t.Fatalf("NormalizeCheckpointResumeAction(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestResumeLongHorizonCheckpointPersistsUnderOriginalPlanID(t *testing.T) {
	client := mockLLMServer(t, func(_ []llm.Message) string {
		return "恢复完成总结"
	})
	reg := skills.NewRegistry()
	reg.Register(&mockSkill{
		name: "next_step", desc: "next",
		execFn: func(_ context.Context, _ map[string]any, _ *skills.Environment) (string, error) {
			return "next ok", nil
		},
	})
	store := NewFileLongHorizonCheckpointStore(filepath.Join(t.TempDir(), "checkpoints.jsonl"))
	p := NewPlanner(client, reg, 4)
	p.SetLongHorizonCheckpointStore(store)
	cp := LongHorizonCheckpoint{
		PlanID:      "plan-original-resume",
		TaskID:      "task-original-resume",
		Goal:        "继续推进 planner",
		Status:      "failed",
		Completed:   1,
		Total:       2,
		StepsUsed:   1,
		Recoverable: true,
		PlanSnapshot: []PlanStep{
			{ID: 0, Action: "读取蓝图", Skill: "already_done", Status: StepDone, Result: "docs read"},
			{ID: 1, Action: "继续修复 planner", Skill: "next_step", DependsOn: []int{0}, Status: StepPending},
		},
	}

	if _, err := p.ResumeLongHorizonCheckpoint(context.Background(), PlanRequest{TenantID: "tenant-a", TaskID: "task-original-resume"}, cp, "continue"); err != nil {
		t.Fatalf("resume checkpoint: %v", err)
	}
	recent, err := store.Recent(context.Background(), 10)
	if err != nil {
		t.Fatalf("load recent checkpoints: %v", err)
	}
	if len(recent) != 1 {
		t.Fatalf("resume should update original checkpoint instead of creating a second plan, got %#v", recent)
	}
	if recent[0].PlanID != cp.PlanID || recent[0].TaskID != cp.TaskID {
		t.Fatalf("resumed checkpoint should keep original ids, got %#v", recent[0])
	}
	if recent[0].Completed != 2 || recent[0].Total != 2 || recent[0].Status != "completed" {
		t.Fatalf("expected original checkpoint to reflect completed resume, got %#v", recent[0])
	}
}

func TestResumeLongHorizonCheckpointReasoningStepSeesCompletedAttachmentEvidence(t *testing.T) {
	var reasoningPrompt string
	client := mockLLMServer(t, func(msgs []llm.Message) string {
		for _, msg := range msgs {
			if msg.Role == "user" && strings.Contains(msg.Content, "根据附件生成申请材料") && strings.Contains(msg.Content, "[步骤0结果]") {
				reasoningPrompt = msg.Content
			}
		}
		return "已根据申请表生成材料草稿"
	})
	p := NewPlanner(client, skills.NewRegistry(), 4)
	cp := LongHorizonCheckpoint{
		PlanID:      "plan-resume-attachment",
		TaskID:      "task-resume-attachment",
		Goal:        "根据申请表.docx 生成入驻申请材料",
		Status:      "failed",
		Completed:   1,
		Total:       2,
		StepsUsed:   1,
		Recoverable: true,
		PlanSnapshot: []PlanStep{
			{
				ID:     0,
				Action: "读取附件",
				Status: StepDone,
				Result: "[Parsed document: 申请表.docx]\n公司名称\t云鸢科技\n联系电话\t13864841667",
			},
			{
				ID:        1,
				Action:    "根据附件生成申请材料",
				Status:    StepFailed,
				DependsOn: []int{0},
				Error:     "上次生成未完成",
			},
		},
	}

	result, err := p.ResumeLongHorizonCheckpoint(context.Background(), PlanRequest{TenantID: "tenant-a", TaskID: "task-resume-attachment"}, cp, "retry_failed")
	if err != nil {
		t.Fatalf("resume checkpoint: %v", err)
	}
	if result == nil || len(result.Plan) != 2 {
		t.Fatalf("expected resumed plan result, got %#v", result)
	}
	if result.Plan[0].Status != StepDone || !strings.Contains(result.Plan[0].Result, "云鸢科技") {
		t.Fatalf("completed attachment evidence should be preserved, got %+v", result.Plan[0])
	}
	if result.Plan[1].Status != StepDone || !strings.Contains(result.Plan[1].Result, "材料草稿") {
		t.Fatalf("failed reasoning step should be retried and completed, got %+v", result.Plan[1])
	}
	for _, want := range []string{"根据附件生成申请材料", "[步骤0结果]", "申请表.docx", "公司名称\t云鸢科技", "联系电话\t13864841667"} {
		if !strings.Contains(reasoningPrompt, want) {
			t.Fatalf("reasoning retry prompt missing %q:\n%s", want, reasoningPrompt)
		}
	}
}

func TestResumeLongHorizonCheckpointSkillStepReceivesCompletedDependencyEvidence(t *testing.T) {
	client := mockLLMServer(t, func(_ []llm.Message) string {
		return "恢复完成总结"
	})
	var receivedArgs map[string]any
	reg := skills.NewRegistry()
	reg.Register(&mockSkill{
		name: "write_application", desc: "write application from evidence",
		execFn: func(_ context.Context, args map[string]any, _ *skills.Environment) (string, error) {
			receivedArgs = args
			return "已根据附件证据生成申请材料", nil
		},
	})
	p := NewPlanner(client, reg, 4)
	cp := LongHorizonCheckpoint{
		PlanID:      "plan-resume-skill-evidence",
		TaskID:      "task-resume-skill-evidence",
		Goal:        "根据申请表.docx 生成入驻申请材料",
		Status:      "failed",
		Completed:   1,
		Total:       2,
		StepsUsed:   1,
		Recoverable: true,
		PlanSnapshot: []PlanStep{
			{
				ID:     0,
				Action: "读取附件",
				Status: StepDone,
				Result: "[Parsed document: 申请表.docx]\n公司名称\t云鸢科技\n联系电话\t13864841667",
			},
			{
				ID:        1,
				Action:    "根据附件生成申请材料",
				Skill:     "write_application",
				Args:      map[string]any{"format": "markdown"},
				Status:    StepFailed,
				DependsOn: []int{0},
				Error:     "上次生成未完成",
			},
		},
	}

	result, err := p.ResumeLongHorizonCheckpoint(context.Background(), PlanRequest{TenantID: "tenant-a", TaskID: "task-resume-skill-evidence"}, cp, "retry_failed")
	if err != nil {
		t.Fatalf("resume checkpoint: %v", err)
	}
	if result == nil || len(result.Plan) != 2 || result.Plan[1].Status != StepDone {
		t.Fatalf("expected skill resume to complete, got %#v", result)
	}
	evidence, ok := receivedArgs["dependency_results"].(string)
	if !ok || evidence == "" {
		t.Fatalf("expected dependency evidence in skill args, got %#v", receivedArgs)
	}
	for _, want := range []string{"读取附件", "申请表.docx", "公司名称\t云鸢科技", "联系电话\t13864841667"} {
		if !strings.Contains(evidence, want) {
			t.Fatalf("dependency evidence missing %q:\n%s", want, evidence)
		}
	}
	if receivedArgs["format"] != "markdown" {
		t.Fatalf("original args should be preserved, got %#v", receivedArgs)
	}
}

func TestResumeLongHorizonCheckpointPartialKeepsCompletedAttachmentEvidence(t *testing.T) {
	var ranPending atomic.Int32
	reg := skills.NewRegistry()
	reg.Register(&mockSkill{
		name: "should_not_run_in_partial", desc: "partial must only report current evidence",
		execFn: func(_ context.Context, _ map[string]any, _ *skills.Environment) (string, error) {
			ranPending.Add(1)
			return "unexpected", nil
		},
	})
	p := NewPlanner(nil, reg, 4)
	rawErr := `handoff agent "file_exec" execution failed: context deadline exceeded EOF`
	cp := LongHorizonCheckpoint{
		PlanID:      "plan-partial-attachment",
		TaskID:      "task-partial-attachment",
		Goal:        "根据申请表.docx 生成入驻申请材料",
		Status:      "failed",
		Completed:   1,
		Total:       2,
		StepsUsed:   1,
		Recoverable: true,
		PlanSnapshot: []PlanStep{
			{
				ID:     0,
				Action: "读取附件",
				Status: StepDone,
				Result: "[Parsed document: 申请表.docx]\n公司名称\t云鸢科技\n联系电话\t13864841667",
			},
			{
				ID:        1,
				Action:    "根据附件生成申请材料",
				Skill:     "should_not_run_in_partial",
				Status:    StepFailed,
				DependsOn: []int{0},
				Error:     rawErr,
			},
		},
	}

	result, err := p.ResumeLongHorizonCheckpoint(context.Background(), PlanRequest{TenantID: "tenant-a", TaskID: "task-partial-attachment"}, cp, "partial")
	if err != nil {
		t.Fatalf("partial checkpoint: %v", err)
	}
	if ranPending.Load() != 0 {
		t.Fatalf("partial result must not continue execution, pending ran %d times", ranPending.Load())
	}
	if result == nil || len(result.Plan) != 2 {
		t.Fatalf("expected partial result with original snapshot, got %#v", result)
	}
	if result.Plan[0].Status != StepDone || !strings.Contains(result.Plan[0].Result, "云鸢科技") {
		t.Fatalf("completed attachment evidence should stay in plan snapshot, got %+v", result.Plan[0])
	}
	for _, want := range []string{"阶段结果：根据申请表.docx", "读取附件（已保留证据）", "申请表.docx", "公司名称\t云鸢科技", "联系电话\t13864841667"} {
		if !strings.Contains(result.Reply, want) {
			t.Fatalf("partial reply missing %q:\n%s", want, result.Reply)
		}
	}
	assertPlannerTextHasNoRawDiagnostics(t, result.Reply)
}

func TestResumeLongHorizonCheckpointFailureCallbackHidesRawError(t *testing.T) {
	client := mockLLMServer(t, func(_ []llm.Message) string {
		return `[]`
	})
	reg := skills.NewRegistry()
	reg.Register(&mockSkill{
		name: "resume_timeout", desc: "timeout",
		execFn: func(_ context.Context, _ map[string]any, _ *skills.Environment) (string, error) {
			return "", context.DeadlineExceeded
		},
	})
	p := NewPlanner(client, reg, 4)
	cp := LongHorizonCheckpoint{
		PlanID:      "plan-resume-timeout",
		TaskID:      "task-resume-timeout",
		Goal:        "继续恢复超时步骤",
		Status:      "failed",
		Recoverable: true,
		PlanSnapshot: []PlanStep{
			{ID: 0, Action: "恢复时会超时", Skill: "resume_timeout", Status: StepFailed},
		},
	}

	var failedSummary string
	result, err := p.ResumeLongHorizonCheckpoint(context.Background(), PlanRequest{
		TraceID:  "trace-resume-friendly",
		TenantID: "tenant-a",
		TaskID:   "task-resume-timeout",
		StepCallback: func(evt observe.AgentEvent) {
			if evt.Type == observe.EventToolResult && strings.Contains(evt.Summary, "暂停") {
				failedSummary = evt.Summary
			}
		},
	}, cp, "retry_failed")
	if err != nil {
		t.Fatalf("resume checkpoint: %v", err)
	}
	if failedSummary == "" {
		t.Fatal("expected failed resume step callback summary")
	}
	for _, raw := range []string{"context deadline exceeded", "execution failed", "fallback", "EOF"} {
		if strings.Contains(strings.ToLower(failedSummary), strings.ToLower(raw)) {
			t.Fatalf("resume callback summary should hide raw error %q, got %q", raw, failedSummary)
		}
		if result != nil && strings.Contains(strings.ToLower(result.Reply), strings.ToLower(raw)) {
			t.Fatalf("resume partial reply should hide raw error %q, got %q", raw, result.Reply)
		}
	}
	if !strings.Contains(failedSummary, "等待时间过长") || !strings.Contains(failedSummary, "现场已保留") {
		t.Fatalf("expected friendly recoverable wording, got %q", failedSummary)
	}
}

func TestResumeLongHorizonCheckpointRetryRejectsUnfinishedDependency(t *testing.T) {
	p := NewPlanner(nil, skills.NewRegistry(), 4)
	cp := LongHorizonCheckpoint{
		PlanID:      "plan-blocked",
		Recoverable: true,
		PlanSnapshot: []PlanStep{
			{ID: 0, Action: "前置还没完成", Status: StepPending},
			{ID: 1, Action: "失败步骤", Status: StepFailed, DependsOn: []int{0}},
		},
	}

	_, err := p.ResumeLongHorizonCheckpoint(context.Background(), PlanRequest{}, cp, "retry_failed")
	if err == nil || !strings.Contains(err.Error(), "unfinished dependency") {
		t.Fatalf("expected unfinished dependency rejection, got %v", err)
	}
}

func TestBuildPlannerFailureSummaryAfterRepeatedFailures(t *testing.T) {
	summary, ok := buildPlannerFailureSummary([]PlanStep{
		{ID: 1, Skill: "file_open", Status: StepDone, Result: "read README"},
		{ID: 2, Skill: "transfer_to_file_exec", Status: StepFailed, Error: "context deadline exceeded"},
		{ID: 3, Skill: "transfer_to_file_exec", Status: StepFailed, Error: "all fallback LLM clients failed"},
	})
	if !ok {
		t.Fatal("expected repeated failures to trigger recovery")
	}
	if summary.FailedCount != 2 || len(summary.RuledOut) != 2 {
		t.Fatalf("unexpected summary: %#v", summary)
	}
	if summary.CompletedCount != 1 {
		t.Fatalf("expected completed count to be tracked, got %#v", summary)
	}
	if !summary.Recoverable || summary.NextStep == "" {
		t.Fatalf("expected recoverable failure summary with next step, got %#v", summary)
	}
	if summary.FailurePattern != "模型或子任务响应不稳定" {
		t.Fatalf("expected timeout/connection failure pattern, got %#v", summary)
	}
	if !strings.Contains(summary.Recommendation, "阶段结果") || !strings.Contains(summary.Recommendation, "降低任务粒度") {
		t.Fatalf("expected actionable recommendation, got %#v", summary)
	}
	for _, text := range append(summary.RuledOut, summary.Tried...) {
		assertPlannerTextHasNoRawDiagnostics(t, text)
	}
	prompt := formatFailureRecoveryPrompt(summary)
	if !strings.Contains(prompt, "已失败/暂时排除") || !strings.Contains(prompt, "不要继续重复同一路径") {
		t.Fatalf("recovery prompt missing guidance: %q", prompt)
	}
	if !strings.Contains(prompt, "失败模式：模型或子任务响应不稳定") || !strings.Contains(prompt, "推荐策略：") {
		t.Fatalf("recovery prompt missing failure analysis: %q", prompt)
	}
	assertPlannerTextHasNoRawDiagnostics(t, prompt)
}

func TestBuildPlannerFailureSummaryClassifiesToolSurfaceFailure(t *testing.T) {
	summary, ok := buildPlannerFailureSummary([]PlanStep{
		{ID: 1, Skill: "missing_tool", Status: StepFailed, Error: "unknown skill: missing_tool"},
		{ID: 2, Skill: "blocked_tool", Status: StepFailed, Error: "blocked by allowed tool surface"},
	})
	if !ok {
		t.Fatal("expected repeated tool-surface failures to trigger recovery")
	}
	if summary.FailurePattern != "所需工具不可用或不在当前工具范围" {
		t.Fatalf("unexpected failure pattern: %#v", summary)
	}
	if !strings.Contains(summary.Recommendation, "当前可用工具") {
		t.Fatalf("expected tool-surface recommendation, got %#v", summary)
	}
	prompt := formatFailureRecoveryPrompt(summary)
	if !strings.Contains(prompt, "失败模式：所需工具不可用或不在当前工具范围") {
		t.Fatalf("expected prompt to include tool-surface analysis, got %q", prompt)
	}
	assertPlannerTextHasNoRawDiagnostics(t, prompt)
}

func TestBuildPlannerFailureSummaryIgnoresSingleFailure(t *testing.T) {
	_, ok := buildPlannerFailureSummary([]PlanStep{
		{ID: 1, Skill: "file_open", Status: StepFailed, Error: "missing file"},
	})
	if ok {
		t.Fatal("single failure should not trigger recovery")
	}
}

func TestSetNativeFC(t *testing.T) {
	p := NewPlanner(nil, nil, 8)
	if p.promptRuntime != nil && p.promptRuntime.NativeFC() {
		t.Fatal("should default to false")
	}
	p.SetNativeFC(true)
	if p.promptRuntime == nil || !p.promptRuntime.NativeFC() {
		t.Fatal("should be true after set")
	}
}

func TestFindClosingBrace(t *testing.T) {
	tests := []struct {
		input string
		start int
		want  int
	}{
		{`{"a": 1}`, 0, 7},
		{`{"a": {"b": 2}}`, 0, 14},
		{`xx{"a": 1}yy`, 2, 9},
		{`{"text":"} not closed","nested":{"ok":true}} tail`, 0, 43},
		{`{"text":"escaped \" } still string","ok":true} tail`, 0, 45},
		{`{unclosed`, 0, -1},
	}
	for _, tt := range tests {
		got := findClosingBrace(tt.input, tt.start)
		if got != tt.want {
			t.Errorf("findClosingBrace(%q, %d) = %d, want %d", tt.input, tt.start, got, tt.want)
		}
	}
}

func TestCleanReplyMultipleToolCalls(t *testing.T) {
	p := &Planner{}
	input := `先搜索一下。{"tool_calls": [{"name": "web_search", "arguments": {"query": "a"}}]}然后再查。{"tool_calls": [{"name": "file_list", "arguments": {"path": "."}}]}最终结果。`
	cleaned := p.cleanReply(input)
	if cleaned != "先搜索一下。然后再查。最终结果。" {
		t.Fatalf("unexpected: %q", cleaned)
	}
}

func TestCleanReplyTrailingCallDescription(t *testing.T) {
	p := &Planner{}
	// After JSON is stripped, the trailing "让我先调用..." should be cleaned
	input := "关于Chirp技能，让我先调用use_skill来加载详细说明："
	cleaned := p.cleanReply(input)
	if cleaned != "关于Chirp技能，" && cleaned != "关于Chirp技能" {
		// Accept both with and without trailing comma/punctuation
		if len(cleaned) > len("关于Chirp技能，") {
			t.Fatalf("expected trailing call description removed, got: %q", cleaned)
		}
	}
}

func TestCleanReplyTrailingCallDescriptionPreservesNormal(t *testing.T) {
	p := &Planner{}
	input := "这是一个正常的回答，没有工具调用描述。"
	cleaned := p.cleanReply(input)
	if cleaned != input {
		t.Fatalf("should not modify normal text, got: %q", cleaned)
	}
}

func TestCleanReplyRemovesACTTags(t *testing.T) {
	p := &Planner{}
	input := `<|ACT {"emotion":{"name":"happy","intensity":1}}|>
嗨！你好呀！

<|ACT {"emotion":{"name":"curious","intensity":1}}|>
今天有什么需要帮忙的吗？`
	cleaned := p.cleanReply(input)
	expected := "嗨！你好呀！\n\n今天有什么需要帮忙的吗？"
	if cleaned != expected {
		t.Fatalf("ACT tags not properly stripped.\nGot:      %q\nExpected: %q", cleaned, expected)
	}
}

func TestCleanReplyACTTagsOnlyLine(t *testing.T) {
	p := &Planner{}
	input := `<|ACT {"emotion":{"name":"neutral","intensity":1}}|>
Hello!`
	cleaned := p.cleanReply(input)
	if cleaned != "Hello!" {
		t.Fatalf("single ACT tag not stripped, got: %q", cleaned)
	}
}

func TestExecutionSummaryEmpty(t *testing.T) {
	result := &PlanResult{Reply: "hello", Plan: nil}
	if result.ExecutionSummary() != "" {
		t.Fatal("expected empty summary for no plan steps")
	}
}

func TestExecutionSummaryWithSteps(t *testing.T) {
	result := &PlanResult{
		Reply: "搜索结果如下...",
		Plan: []PlanStep{
			{Skill: "web_search", Status: StepDone, Result: "找到3个结果"},
			{Skill: "translate", Status: StepDone, Result: "翻译完成"},
		},
	}
	summary := result.ExecutionSummary()
	if summary == "" {
		t.Fatal("expected non-empty summary")
	}
	if !contains(summary, "web_search") || !contains(summary, "translate") {
		t.Fatalf("expected skill names in summary, got: %s", summary)
	}
	if !contains(summary, "✓") {
		t.Fatal("expected success markers")
	}
}

func TestExecutionSummaryWithFailure(t *testing.T) {
	result := &PlanResult{
		Reply: "sorry",
		Plan: []PlanStep{
			{Skill: "use_skill", Status: StepFailed, Error: "skill \"Chirp\" is not installed"},
		},
	}
	summary := result.ExecutionSummary()
	if !contains(summary, "失败") {
		t.Fatalf("expected failure indicator, got: %s", summary)
	}
	if !contains(summary, "use_skill") {
		t.Fatalf("expected skill name, got: %s", summary)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ── Integration-level tests (mock LLM server) ──

func mockLLMServer(t *testing.T, responseFunc func(msgs []llm.Message) string) *llm.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Messages []llm.Message `json:"messages"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		reply := responseFunc(req.Messages)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"role": "assistant", "content": reply}},
			},
		})
	}))
	t.Cleanup(srv.Close)
	return llm.NewClient(srv.URL, "test-key", "test-model")
}

func mockLLMServerFC(t *testing.T, responseFunc func(msgs []llm.Message) map[string]any) *llm.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Messages []llm.Message `json:"messages"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		msg := responseFunc(req.Messages)
		if _, ok := msg["role"]; !ok {
			msg["role"] = "assistant"
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": msg},
			},
		})
	}))
	t.Cleanup(srv.Close)
	return llm.NewClient(srv.URL, "test-key", "test-model")
}

type mockSkill struct {
	name   string
	desc   string
	execFn func(ctx context.Context, args map[string]any, env *skills.Environment) (string, error)
}

func (s *mockSkill) Name() string        { return s.name }
func (s *mockSkill) Description() string { return s.desc }
func (s *mockSkill) Parameters() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{}}
}
func (s *mockSkill) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	return s.execFn(ctx, args, env)
}

func TestPlannerDefaults(t *testing.T) {
	client := mockLLMServer(t, func(_ []llm.Message) string { return "hi" })
	p := NewPlanner(client, skills.NewRegistry(), 0)
	if got := p.maxPlanSteps(); got != 15 {
		t.Errorf("expected default maxSteps=15, got %d", got)
	}
	if got := p.perToolTimeout(); got != 60*time.Second {
		t.Errorf("expected default toolTimeout=60s, got %v", got)
	}
}

func TestRunTextBased_SimpleReply(t *testing.T) {
	client := mockLLMServer(t, func(_ []llm.Message) string {
		return "Hello! How can I help you?"
	})
	p := NewPlanner(client, skills.NewRegistry(), 8)
	result, err := p.Run(context.Background(), PlanRequest{
		Messages: []llm.Message{{Role: "user", Content: "hello"}},
		TenantID: "test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Reply == "" {
		t.Error("expected non-empty reply")
	}
	if result.Steps != 1 {
		t.Errorf("expected 1 step, got %d", result.Steps)
	}
}

func TestRunTextBased_SkillCall(t *testing.T) {
	callCount := 0
	client := mockLLMServer(t, func(_ []llm.Message) string {
		callCount++
		if callCount == 1 {
			return `I need to search. {"tool_calls": [{"name": "web_search", "arguments": {"query": "golang testing"}}]}`
		}
		return "Go testing uses the testing package."
	})

	reg := skills.NewRegistry()
	reg.Register(&mockSkill{
		name: "web_search", desc: "Search the web",
		execFn: func(_ context.Context, args map[string]any, _ *skills.Environment) (string, error) {
			q, _ := args["query"].(string)
			return fmt.Sprintf("Results for '%s': Go testing is built-in.", q), nil
		},
	})

	p := NewPlanner(client, reg, 8)
	result, err := p.Run(context.Background(), PlanRequest{
		Messages: []llm.Message{{Role: "user", Content: "how does go testing work?"}},
		TenantID: "test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.SkillsUsed) != 1 || result.SkillsUsed[0] != "web_search" {
		t.Errorf("expected [web_search], got %v", result.SkillsUsed)
	}
}

func TestRunTextBasedReturnsPartialAfterToolThenModelFailure(t *testing.T) {
	var llmCalls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := atomic.AddInt32(&llmCalls, 1)
		w.Header().Set("Content-Type", "application/json")
		if call == 1 {
			json.NewEncoder(w).Encode(map[string]any{
				"choices": []map[string]any{{
					"message": map[string]any{
						"role":    "assistant",
						"content": `{"tool_calls":[{"name":"stage_tool","arguments":{}}]}`,
					},
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
			return "阶段资料：已经读取技术蓝图并完成初步拆解。", nil
		},
	})

	p := NewPlanner(llm.NewClient(srv.URL, "test-key", "test-model"), reg, 3)
	var partialEvt observe.AgentEvent
	result, err := p.Run(context.Background(), PlanRequest{
		Messages: []llm.Message{{Role: "user", Content: "推进云雀 planner"}},
		TenantID: "test",
		TraceID:  "trace-partial-text",
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

func TestRunTextBasedReflectionPromptHidesRawToolError(t *testing.T) {
	var prompts []string
	var toolStartEvent observe.AgentEvent
	var toolResultEvent observe.AgentEvent
	client := mockLLMServer(t, func(messages []llm.Message) string {
		if len(messages) > 0 {
			prompts = append(prompts, messages[len(messages)-1].Content)
		}
		if len(prompts) == 1 {
			return `{"tool_calls":[{"name":"timeout_tool","arguments":{}}]}`
		}
		return "现场已保留，可以稍后继续。"
	})

	reg := skills.NewRegistry()
	reg.Register(&mockSkill{
		name: "timeout_tool", desc: "times out",
		execFn: func(context.Context, map[string]any, *skills.Environment) (string, error) {
			return "", context.DeadlineExceeded
		},
	})

	p := NewPlanner(client, reg, 3)
	result, err := p.Run(context.Background(), PlanRequest{
		Messages: []llm.Message{{Role: "user", Content: "执行会超时的工具"}},
		TenantID: "test",
		StepCallback: func(evt observe.AgentEvent) {
			switch evt.Type {
			case observe.EventToolStart:
				toolStartEvent = evt
			case observe.EventToolResult:
				toolResultEvent = evt
			}
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || len(result.Plan) != 1 || result.Plan[0].Status != StepFailed {
		t.Fatalf("expected failed step to remain in plan, got %#v", result)
	}
	if len(prompts) < 2 {
		t.Fatalf("expected reflection prompt after tool execution, got %#v", prompts)
	}
	reflectPrompt := prompts[1]
	if !strings.Contains(reflectPrompt, "工具调用结果") || !strings.Contains(reflectPrompt, "现场已保留") {
		t.Fatalf("expected friendly recovery wording in reflection prompt, got %q", reflectPrompt)
	}
	assertPlannerTextHasNoRawDiagnostics(t, reflectPrompt)
	if toolStartEvent.Type != observe.EventToolStart || toolStartEvent.Meta.Skill != "timeout_tool" {
		t.Fatalf("expected text fallback tool_start event, got %#v", toolStartEvent)
	}
	startDetail, ok := toolStartEvent.Detail.(observe.ToolStartDetail)
	if !ok || startDetail.Skill != "timeout_tool" {
		t.Fatalf("expected text fallback tool_start detail, got %#v", toolStartEvent.Detail)
	}
	if toolResultEvent.Type != observe.EventToolResult || toolResultEvent.Meta.Skill != "timeout_tool" {
		t.Fatalf("expected text fallback tool_result event, got %#v", toolResultEvent)
	}
	resultDetail, ok := toolResultEvent.Detail.(observe.ToolResultDetail)
	if !ok || resultDetail.Error == "" || resultDetail.Result != "" {
		t.Fatalf("expected failed text fallback tool_result detail, got %#v", toolResultEvent.Detail)
	}
	assertPlannerTextHasNoRawDiagnostics(t, toolResultEvent.Summary)
	assertPlannerTextHasNoRawDiagnostics(t, resultDetail.Error)
	if !strings.Contains(toolResultEvent.Summary, "暂未完成") || !strings.Contains(toolResultEvent.Summary, "现场已保留") {
		t.Fatalf("expected text fallback tool_result summary to use recoverable wording, got %q", toolResultEvent.Summary)
	}
	if !strings.Contains(result.Plan[0].Error, "context deadline exceeded") {
		t.Fatalf("internal text fallback plan step should keep raw diagnostic evidence, got %q", result.Plan[0].Error)
	}
}

func TestRunNativeFCToolResultMessageHidesRawToolError(t *testing.T) {
	var toolMessage string
	var toolResultEvent observe.AgentEvent
	callCount := 0
	client := mockLLMServerFC(t, func(messages []llm.Message) map[string]any {
		callCount++
		for _, msg := range messages {
			if msg.Role == "tool" {
				toolMessage = msg.Content
			}
		}
		if callCount == 1 {
			return map[string]any{
				"content": "",
				"tool_calls": []map[string]any{{
					"id":   "call-timeout",
					"type": "function",
					"function": map[string]any{
						"name":      "timeout_tool",
						"arguments": `{}`,
					},
				}},
			}
		}
		return map[string]any{"content": "现场已保留，可以稍后继续。"}
	})

	reg := skills.NewRegistry()
	reg.Register(&mockSkill{
		name: "timeout_tool", desc: "times out",
		execFn: func(context.Context, map[string]any, *skills.Environment) (string, error) {
			return "", context.DeadlineExceeded
		},
	})

	p := NewPlanner(client, reg, 3)
	p.SetNativeFC(true)
	result, err := p.Run(context.Background(), PlanRequest{
		Messages: []llm.Message{{Role: "user", Content: "执行会超时的原生工具调用"}},
		TenantID: "test",
		StepCallback: func(evt observe.AgentEvent) {
			if evt.Type == observe.EventToolResult {
				toolResultEvent = evt
			}
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || len(result.Plan) != 1 || result.Plan[0].Status != StepFailed {
		t.Fatalf("expected failed native FC step, got %#v", result)
	}
	if toolMessage == "" || !strings.Contains(toolMessage, "现场已保留") {
		t.Fatalf("expected friendly tool message to be sent back to model, got %q", toolMessage)
	}
	assertPlannerTextHasNoRawDiagnostics(t, toolMessage)
	detail, ok := toolResultEvent.Detail.(observe.ToolResultDetail)
	if !ok || detail.Error == "" {
		t.Fatalf("expected failed native FC tool_result detail, got %#v", toolResultEvent.Detail)
	}
	assertPlannerTextHasNoRawDiagnostics(t, toolResultEvent.Summary)
	assertPlannerTextHasNoRawDiagnostics(t, detail.Error)
	if !strings.Contains(detail.Error, "现场已保留") {
		t.Fatalf("expected friendly recovery wording in native FC tool_result detail, got %q", detail.Error)
	}
	if !strings.Contains(toolResultEvent.Summary, "暂未完成") || !strings.Contains(toolResultEvent.Summary, "现场已保留") {
		t.Fatalf("expected native FC tool_result summary to use recoverable wording, got %q", toolResultEvent.Summary)
	}
	if strings.Contains(toolResultEvent.Summary, "执行失败") {
		t.Fatalf("native FC tool_result summary should avoid hard failure wording, got %q", toolResultEvent.Summary)
	}
	if !strings.Contains(result.Plan[0].Error, "context deadline exceeded") {
		t.Fatalf("internal plan step should keep raw diagnostic evidence, got %q", result.Plan[0].Error)
	}
}

func TestRunNativeFCReturnsPartialEventAfterToolThenContextCancel(t *testing.T) {
	client := mockLLMServerFC(t, func(messages []llm.Message) map[string]any {
		for _, msg := range messages {
			if msg.Role == "tool" {
				t.Fatalf("should not call model again after context cancellation, got tool message %q", msg.Content)
			}
		}
		return map[string]any{
			"content": "",
			"tool_calls": []map[string]any{{
				"id":   "call-stage",
				"type": "function",
				"function": map[string]any{
					"name":      "stage_tool",
					"arguments": `{}`,
				},
			}},
		}
	})

	var cancel context.CancelFunc
	reg := skills.NewRegistry()
	reg.Register(&mockSkill{
		name: "stage_tool", desc: "produce a stage result before disconnect",
		execFn: func(context.Context, map[string]any, *skills.Environment) (string, error) {
			cancel()
			return "阶段资料：已读取技术蓝图，并完成 planner 恢复点拆解。", nil
		},
	})

	p := NewPlanner(client, reg, 3)
	p.SetNativeFC(true)
	ctx, cancelFn := context.WithCancel(context.Background())
	cancel = cancelFn
	defer cancel()

	var partialEvt observe.AgentEvent
	result, err := p.Run(ctx, PlanRequest{
		Messages: []llm.Message{{Role: "user", Content: "推进云雀 planner"}},
		TenantID: "test",
		TaskID:   "task-native-partial",
		TraceID:  "trace-native-partial",
		StepCallback: func(evt observe.AgentEvent) {
			if evt.Type == observe.EventPartial {
				partialEvt = evt
			}
		},
	})
	if err != nil {
		t.Fatalf("expected partial result instead of context error, got %v", err)
	}
	if result == nil || len(result.Plan) != 1 || result.Plan[0].Status != StepDone {
		t.Fatalf("expected one completed native FC step, got %#v", result)
	}
	if !strings.Contains(result.Reply, "任务已部分执行") || !strings.Contains(result.Reply, "现场已保留") || !strings.Contains(result.Reply, "阶段资料") {
		t.Fatalf("expected friendly partial reply with stage output, got %q", result.Reply)
	}
	assertPlannerTextHasNoRawDiagnostics(t, result.Reply)
	if partialEvt.Type != observe.EventPartial {
		t.Fatalf("expected partial event after native FC context cancellation, got %#v", partialEvt)
	}
	detail, ok := partialEvt.Detail.(observe.PartialResultDetail)
	if !ok {
		t.Fatalf("expected partial detail, got %#v", partialEvt.Detail)
	}
	if !detail.Recoverable || detail.CompletedCount != 1 || detail.FailedCount != 0 || detail.NextStep == "" {
		t.Fatalf("expected recoverable partial detail with one completed step, got %#v", detail)
	}
	if !strings.Contains(detail.Reason, "现场已保留") {
		t.Fatalf("expected friendly interruption reason, got %q", detail.Reason)
	}
	assertPlannerTextHasNoRawDiagnostics(t, partialEvt.Summary)
	assertPlannerTextHasNoRawDiagnostics(t, detail.Reason)
}

func TestRunTextBased_TrustGateBlocksSkill(t *testing.T) {
	callCount := 0
	client := mockLLMServer(t, func(_ []llm.Message) string {
		callCount++
		if callCount == 1 {
			return `{"tool_calls": [{"name": "shell_exec", "arguments": {"cmd": "whoami"}}]}`
		}
		return "blocked safely"
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

	p := NewPlanner(client, reg, 8)
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
		t.Fatal("blocked skill was executed")
	}
	if len(result.Plan) == 0 || result.Plan[0].Status != StepFailed {
		t.Fatalf("expected failed plan step, got %+v", result.Plan)
	}
	if !contains(result.Plan[0].Error, "blocked by trust gate") {
		t.Fatalf("expected trust gate error, got %q", result.Plan[0].Error)
	}
}

func TestRunTextBased_ParallelSkillCalls(t *testing.T) {
	callCount := 0
	client := mockLLMServer(t, func(_ []llm.Message) string {
		callCount++
		if callCount == 1 {
			return `{"tool_calls": [
				{"name": "skill_a", "arguments": {"id": "1"}},
				{"name": "skill_b", "arguments": {"id": "2"}}
			]}`
		}
		return "Combined results."
	})

	var executed int32
	reg := skills.NewRegistry()
	reg.Register(&mockSkill{
		name: "skill_a", desc: "A",
		execFn: func(_ context.Context, _ map[string]any, _ *skills.Environment) (string, error) {
			time.Sleep(50 * time.Millisecond)
			atomic.AddInt32(&executed, 1)
			return "result_a", nil
		},
	})
	reg.Register(&mockSkill{
		name: "skill_b", desc: "B",
		execFn: func(_ context.Context, _ map[string]any, _ *skills.Environment) (string, error) {
			time.Sleep(50 * time.Millisecond)
			atomic.AddInt32(&executed, 1)
			return "result_b", nil
		},
	})

	p := NewPlanner(client, reg, 8)
	result, err := p.Run(context.Background(), PlanRequest{
		Messages: []llm.Message{{Role: "user", Content: "do both"}},
		TenantID: "test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.SkillsUsed) != 2 {
		t.Errorf("expected 2 skills, got %v", result.SkillsUsed)
	}
	if atomic.LoadInt32(&executed) != 2 {
		t.Errorf("expected both skills executed, got %d", executed)
	}
}

func TestRunTextBased_ContextCancellation(t *testing.T) {
	client := mockLLMServer(t, func(_ []llm.Message) string {
		time.Sleep(2 * time.Second)
		return "should not reach"
	})
	p := NewPlanner(client, skills.NewRegistry(), 8)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_, err := p.Run(ctx, PlanRequest{
		Messages: []llm.Message{{Role: "user", Content: "hello"}},
		TenantID: "test",
	})
	if err == nil {
		t.Error("expected context cancellation error")
	}
}

func TestRunTextBased_ReflectRetry(t *testing.T) {
	callCount := 0
	client := mockLLMServer(t, func(_ []llm.Message) string {
		callCount++
		if callCount == 1 {
			return "bad answer"
		}
		return "improved answer after reflection"
	})
	p := NewPlanner(client, skills.NewRegistry(), 8)

	reflectCount := 0
	p.SetReflect(func(_ context.Context, _, _ string) bool {
		reflectCount++
		return reflectCount > 1 // reject first, accept second
	})

	result, err := p.Run(context.Background(), PlanRequest{
		Messages: []llm.Message{{Role: "user", Content: "what is 2+2?"}},
		TenantID: "test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Steps != 2 {
		t.Errorf("expected 2 steps (reflect retry), got %d", result.Steps)
	}
}

func TestSafeToolGo_PanicRecovery(t *testing.T) {
	done := make(chan bool, 1)
	safeToolGo(context.Background(), 5*time.Second, func(_ context.Context) {
		defer func() { done <- true }()
		panic("test panic")
	})
	select {
	case <-done:
		// recovered successfully
	case <-time.After(2 * time.Second):
		t.Error("safeToolGo did not recover in time")
	}
}

func TestSafeToolGo_Timeout(t *testing.T) {
	started := make(chan bool, 1)
	safeToolGo(context.Background(), 100*time.Millisecond, func(ctx context.Context) {
		started <- true
		<-ctx.Done()
	})
	select {
	case <-started:
		// timeout will cancel the goroutine
	case <-time.After(2 * time.Second):
		t.Error("goroutine did not start")
	}
}

func TestSetToolTimeout(t *testing.T) {
	client := mockLLMServer(t, func(_ []llm.Message) string { return "ok" })
	p := NewPlanner(client, skills.NewRegistry(), 8)
	p.SetToolTimeout(30 * time.Second)
	if got := p.perToolTimeout(); got != 30*time.Second {
		t.Errorf("expected 30s, got %v", got)
	}
}

func TestContextLayers_NoCrossContamination(t *testing.T) {
	client := mockLLMServer(t, func(_ []llm.Message) string { return "reply" })
	reg := skills.NewRegistry()
	p := NewPlanner(client, reg, 8)

	p.SetMemory(func(_ context.Context, _, query string) string {
		time.Sleep(10 * time.Millisecond)
		if contains(query, "memory-a") {
			return "fact-a"
		}
		return ""
	})

	const N = 20
	results := make([]*PlanResult, N)
	errs := make([]error, N)
	var wg sync.WaitGroup
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func(idx int) {
			defer wg.Done()
			msg := fmt.Sprintf("request-%d", idx)
			if idx%2 == 0 {
				msg = "memory-a " + msg
			}
			results[idx], errs[idx] = p.Run(context.Background(), PlanRequest{
				Messages: []llm.Message{{Role: "user", Content: msg}},
				TenantID: fmt.Sprintf("tenant-%d", idx),
			})
		}(i)
	}
	wg.Wait()

	for i := 0; i < N; i++ {
		if errs[i] != nil {
			t.Fatalf("request %d failed: %v", i, errs[i])
		}
		if results[i] == nil {
			t.Fatalf("request %d: nil result", i)
		}
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hello..."},
		{"你好世界", 2, "你好..."},
		{"", 5, ""},
	}
	for _, tt := range tests {
		got := truncate(tt.input, tt.maxLen)
		if got != tt.expected {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.expected)
		}
	}
}
