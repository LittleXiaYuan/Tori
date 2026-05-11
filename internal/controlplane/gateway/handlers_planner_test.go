package gateway

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
	"time"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/planner"
	"yunque-agent/internal/agentcore/task"
	"yunque-agent/internal/observe"
	"yunque-agent/pkg/skills"
)

type plannerGatewayTestSkill struct {
	name   string
	execFn func(context.Context, map[string]any, *skills.Environment) (string, error)
}

func (s plannerGatewayTestSkill) Name() string        { return s.name }
func (s plannerGatewayTestSkill) Description() string { return "test " + s.name }
func (s plannerGatewayTestSkill) Parameters() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{}}
}
func (s plannerGatewayTestSkill) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	if s.execFn == nil {
		return "ok", nil
	}
	return s.execFn(ctx, args, env)
}

func TestPlannerCheckpointsRequiresAuth(t *testing.T) {
	gw, _ := newTestGateway()
	req := httptest.NewRequest(http.MethodGet, "/v1/planner/checkpoints", nil)
	w := httptest.NewRecorder()

	gw.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth, got %d", w.Code)
	}
}

func TestPlannerCheckpointsListCompactByDefault(t *testing.T) {
	gw, tm := newTestGateway()
	tenant := tm.Register("planner-checkpoints")
	store := planner.NewFileLongHorizonCheckpointStore(filepath.Join(t.TempDir(), "checkpoints.jsonl"))
	gw.planner.SetLongHorizonCheckpointStore(store)

	oldTime := time.Date(2026, 5, 10, 1, 0, 0, 0, time.UTC)
	newTime := time.Date(2026, 5, 10, 2, 0, 0, 0, time.UTC)
	if err := store.Save(context.Background(), planner.LongHorizonCheckpoint{
		PlanID:      "plan-old",
		TenantID:    tenant.ID,
		Status:      "running",
		Completed:   1,
		Total:       3,
		Recoverable: true,
		UpdatedAt:   oldTime,
		PlanSnapshot: []planner.PlanStep{{
			ID:     1,
			Action: "older step",
			Result: strings.Repeat("old", 200),
		}},
	}); err != nil {
		t.Fatalf("save old checkpoint: %v", err)
	}
	if err := store.Save(context.Background(), planner.LongHorizonCheckpoint{
		PlanID:      "plan-new",
		TenantID:    tenant.ID,
		TaskID:      "task-1",
		Goal:        "继续推进云雀 planner",
		Status:      "failed",
		CurrentStep: 2,
		Completed:   2,
		Total:       4,
		Error:       strings.Repeat("err", 120),
		Recoverable: true,
		ResumeHint:  "可继续恢复",
		UpdatedAt:   newTime,
		PlanSnapshot: []planner.PlanStep{{
			ID:     2,
			Action: "new step",
			Result: strings.Repeat("result", 200),
		}},
	}); err != nil {
		t.Fatalf("save new checkpoint: %v", err)
	}

	req := authedRequest(http.MethodGet, "/v1/planner/checkpoints?limit=1", "", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var resp plannerCheckpointListResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Count != 1 || len(resp.Checkpoints) != 1 {
		t.Fatalf("expected one checkpoint, got %+v", resp)
	}
	got := resp.Checkpoints[0]
	if got.PlanID != "plan-new" || got.TaskID != "task-1" || got.Goal != "继续推进云雀 planner" || got.Status != "failed" {
		t.Fatalf("unexpected checkpoint summary: %+v", got)
	}
	if len(got.PlanSnapshot) != 0 {
		t.Fatalf("expected compact default without plan_snapshot, got %+v", got.PlanSnapshot)
	}
	if len([]rune(got.Error)) > 243 {
		t.Fatalf("expected truncated error, got length %d", len([]rune(got.Error)))
	}
}

func TestPlannerCheckpointsAreScopedToTenant(t *testing.T) {
	gw, tm := newTestGateway()
	owner := tm.Register("planner-checkpoints-owner")
	other := tm.Register("planner-checkpoints-other")
	third := tm.Register("planner-checkpoints-third")
	store := planner.NewFileLongHorizonCheckpointStore(filepath.Join(t.TempDir(), "checkpoints.jsonl"))
	gw.planner.SetLongHorizonCheckpointStore(store)

	if err := store.Save(context.Background(), planner.LongHorizonCheckpoint{
		PlanID:      "plan-shared",
		TenantID:    owner.ID,
		TaskID:      "task-owner",
		Goal:        "owner checkpoint",
		Status:      "failed",
		Recoverable: true,
		UpdatedAt:   time.Date(2026, 5, 11, 1, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("save owner checkpoint: %v", err)
	}
	if err := store.Save(context.Background(), planner.LongHorizonCheckpoint{
		PlanID:      "plan-shared",
		TenantID:    other.ID,
		TaskID:      "task-other",
		Goal:        "other checkpoint",
		Status:      "failed",
		Recoverable: true,
		UpdatedAt:   time.Date(2026, 5, 11, 2, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("save other checkpoint: %v", err)
	}

	for _, tc := range []struct {
		name    string
		apiKey  string
		wantID  string
		wantCnt int
	}{
		{name: "owner", apiKey: owner.APIKey, wantID: "task-owner", wantCnt: 1},
		{name: "other", apiKey: other.APIKey, wantID: "task-other", wantCnt: 1},
		{name: "third", apiKey: third.APIKey, wantCnt: 0},
	} {
		req := authedRequest(http.MethodGet, "/v1/planner/checkpoints?plan_id=plan-shared&include_snapshot=1", "", tc.apiKey)
		w := httptest.NewRecorder()
		gw.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("%s: expected 200, got %d body=%s", tc.name, w.Code, w.Body.String())
		}
		var resp plannerCheckpointListResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("%s: decode checkpoint list: %v", tc.name, err)
		}
		if resp.Count != tc.wantCnt || len(resp.Checkpoints) != tc.wantCnt {
			t.Fatalf("%s: expected %d tenant-scoped checkpoints, got %+v", tc.name, tc.wantCnt, resp)
		}
		if tc.wantCnt == 1 && resp.Checkpoints[0].TaskID != tc.wantID {
			t.Fatalf("%s: expected checkpoint task %q, got %+v", tc.name, tc.wantID, resp.Checkpoints[0])
		}
	}
}

func TestPlannerCheckpointsCanIncludeSnapshot(t *testing.T) {
	gw, tm := newTestGateway()
	tenant := tm.Register("planner-checkpoint-detail")
	store := planner.NewFileLongHorizonCheckpointStore(filepath.Join(t.TempDir(), "checkpoints.jsonl"))
	gw.planner.SetLongHorizonCheckpointStore(store)

	if err := store.Save(context.Background(), planner.LongHorizonCheckpoint{
		PlanID:      "plan-detail",
		TenantID:    tenant.ID,
		Status:      "failed",
		Completed:   1,
		Total:       2,
		Recoverable: true,
		UpdatedAt:   time.Now().UTC(),
		PlanSnapshot: []planner.PlanStep{{
			ID:     1,
			Action: "read docs",
			Result: strings.Repeat("snapshot", 120),
		}},
	}); err != nil {
		t.Fatalf("save checkpoint: %v", err)
	}

	req := authedRequest(http.MethodGet, "/v1/planner/checkpoints?include_snapshot=1", "", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var resp plannerCheckpointListResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Checkpoints) != 1 || len(resp.Checkpoints[0].PlanSnapshot) != 1 {
		t.Fatalf("expected one checkpoint with one snapshot step, got %+v", resp)
	}
	if len([]rune(resp.Checkpoints[0].PlanSnapshot[0].Result)) > 243 {
		t.Fatalf("expected snapshot result to be truncated")
	}
}

func TestPlannerCheckpointsCanFilterByPlanID(t *testing.T) {
	gw, tm := newTestGateway()
	tenant := tm.Register("planner-checkpoint-filter")
	store := planner.NewFileLongHorizonCheckpointStore(filepath.Join(t.TempDir(), "checkpoints.jsonl"))
	gw.planner.SetLongHorizonCheckpointStore(store)

	for _, cp := range []planner.LongHorizonCheckpoint{
		{
			PlanID:      "plan-a",
			TenantID:    tenant.ID,
			Status:      "failed",
			Completed:   1,
			Total:       2,
			Recoverable: true,
			UpdatedAt:   time.Now().UTC().Add(-time.Minute),
			PlanSnapshot: []planner.PlanStep{{
				ID:     1,
				Action: "old step",
				Status: planner.StepDone,
			}},
		},
		{
			PlanID:      "plan-b",
			TenantID:    tenant.ID,
			Status:      "failed",
			Completed:   0,
			Total:       1,
			Recoverable: true,
			UpdatedAt:   time.Now().UTC(),
			PlanSnapshot: []planner.PlanStep{{
				ID:     2,
				Action: "target step",
				Status: planner.StepFailed,
			}},
		},
	} {
		if err := store.Save(context.Background(), cp); err != nil {
			t.Fatalf("save checkpoint %s: %v", cp.PlanID, err)
		}
	}

	req := authedRequest(http.MethodGet, "/v1/planner/checkpoints?plan_id=plan-b&include_snapshot=1&limit=1", "", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var resp plannerCheckpointListResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Count != 1 || len(resp.Checkpoints) != 1 {
		t.Fatalf("expected one filtered checkpoint, got %+v", resp)
	}
	if resp.Checkpoints[0].PlanID != "plan-b" || len(resp.Checkpoints[0].PlanSnapshot) != 1 {
		t.Fatalf("expected filtered checkpoint with snapshot, got %+v", resp.Checkpoints[0])
	}
	if resp.Checkpoints[0].PlanSnapshot[0].Action != "target step" {
		t.Fatalf("unexpected snapshot: %+v", resp.Checkpoints[0].PlanSnapshot)
	}
}

func TestPlannerCheckpointKnownErrorsAreDisplayedFriendly(t *testing.T) {
	gw, tm := newTestGateway()
	tenant := tm.Register("planner-checkpoint-friendly-error")
	store := planner.NewFileLongHorizonCheckpointStore(filepath.Join(t.TempDir(), "checkpoints.jsonl"))
	gw.planner.SetLongHorizonCheckpointStore(store)
	rawErr := `handoff agent "file_exec" execution failed: context deadline exceeded`

	if err := store.Save(context.Background(), planner.LongHorizonCheckpoint{
		PlanID:      "plan-friendly-error",
		TenantID:    tenant.ID,
		TaskID:      "task-friendly-error",
		Status:      "failed",
		Completed:   0,
		Total:       1,
		Error:       rawErr,
		Recoverable: true,
		UpdatedAt:   time.Now().UTC(),
		PlanSnapshot: []planner.PlanStep{
			{ID: 0, Action: "读取附件", Status: planner.StepFailed, Error: rawErr},
		},
	}); err != nil {
		t.Fatalf("save checkpoint: %v", err)
	}

	req := authedRequest(http.MethodGet, "/v1/planner/checkpoints?plan_id=plan-friendly-error&include_snapshot=1", "", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var listResp plannerCheckpointListResponse
	if err := json.NewDecoder(w.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listResp.Checkpoints) != 1 {
		t.Fatalf("expected checkpoint, got %+v", listResp)
	}
	if strings.Contains(listResp.Checkpoints[0].Error, "context deadline exceeded") || strings.Contains(listResp.Checkpoints[0].Error, "handoff agent") {
		t.Fatalf("expected friendly checkpoint error, got %q", listResp.Checkpoints[0].Error)
	}
	if len(listResp.Checkpoints[0].PlanSnapshot) != 1 {
		t.Fatalf("expected snapshot, got %+v", listResp.Checkpoints[0].PlanSnapshot)
	}
	if strings.Contains(listResp.Checkpoints[0].PlanSnapshot[0].Error, "context deadline exceeded") || strings.Contains(listResp.Checkpoints[0].PlanSnapshot[0].Error, "handoff agent") {
		t.Fatalf("expected friendly snapshot step error, got %q", listResp.Checkpoints[0].PlanSnapshot[0].Error)
	}

	req = authedRequest(http.MethodPost, "/v1/planner/checkpoints/recover", `{"plan_id":"plan-friendly-error","action":"continue"}`, tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected recover 200, got %d body=%s", w.Code, w.Body.String())
	}
	var recoverResp plannerCheckpointRecoverResponse
	if err := json.NewDecoder(w.Body).Decode(&recoverResp); err != nil {
		t.Fatalf("decode recover response: %v", err)
	}
	if strings.Contains(recoverResp.Prompt, "context deadline exceeded") || strings.Contains(recoverResp.Prompt, "handoff agent") {
		t.Fatalf("expected friendly recovery prompt error, got:\n%s", recoverResp.Prompt)
	}
	if !strings.Contains(recoverResp.Prompt, "已保留现场") {
		t.Fatalf("expected recovery prompt to keep actionable friendly context, got:\n%s", recoverResp.Prompt)
	}
}

func TestPlannerCheckpointKnownDiagnosticsInCompletedResultAreDisplayedFriendly(t *testing.T) {
	gw, tm := newTestGateway()
	tenant := tm.Register("planner-checkpoint-friendly-result")
	store := planner.NewFileLongHorizonCheckpointStore(filepath.Join(t.TempDir(), "checkpoints.jsonl"))
	gw.planner.SetLongHorizonCheckpointStore(store)
	rawResult := `子代理返回：handoff agent "file_exec" execution failed: context deadline exceeded EOF，但现场已保留。`

	if err := store.Save(context.Background(), planner.LongHorizonCheckpoint{
		PlanID:      "plan-friendly-result",
		TenantID:    tenant.ID,
		TaskID:      "task-friendly-result",
		Goal:        "根据申请表.docx 生成入驻申请材料",
		Status:      "failed",
		Completed:   1,
		Total:       2,
		Recoverable: true,
		PlanSnapshot: []planner.PlanStep{
			{ID: 0, Action: "读取附件", Status: planner.StepDone, Result: rawResult},
			{ID: 1, Action: "继续整理", Status: planner.StepFailed, DependsOn: []int{0}, Error: `context deadline exceeded EOF`},
		},
	}); err != nil {
		t.Fatalf("save checkpoint: %v", err)
	}

	req := authedRequest(http.MethodGet, "/v1/planner/checkpoints?plan_id=plan-friendly-result&include_snapshot=1", "", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var listResp plannerCheckpointListResponse
	if err := json.NewDecoder(w.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode checkpoint list: %v", err)
	}
	if len(listResp.Checkpoints) != 1 || len(listResp.Checkpoints[0].PlanSnapshot) != 2 {
		t.Fatalf("expected checkpoint snapshot, got %+v", listResp)
	}
	if got := listResp.Checkpoints[0].PlanSnapshot[0].Result; !strings.Contains(got, "已保留现场") {
		t.Fatalf("expected friendly completed result, got %q", got)
	}

	req = authedRequest(http.MethodPost, "/v1/planner/checkpoints/recover", `{"plan_id":"plan-friendly-result","action":"continue"}`, tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected recover 200, got %d body=%s", w.Code, w.Body.String())
	}
	var recoverResp plannerCheckpointRecoverResponse
	if err := json.NewDecoder(w.Body).Decode(&recoverResp); err != nil {
		t.Fatalf("decode recover response: %v", err)
	}
	if !strings.Contains(recoverResp.Prompt, "已保留现场") {
		t.Fatalf("expected friendly result in recovery prompt, got:\n%s", recoverResp.Prompt)
	}
	for _, banned := range []string{"handoff agent", "execution failed", "context deadline exceeded", "EOF"} {
		if strings.Contains(strings.ToLower(w.Body.String()), strings.ToLower(banned)) {
			t.Fatalf("checkpoint response should hide raw completed diagnostics %q: %s", banned, w.Body.String())
		}
	}
}

func TestPlannerKnownFriendlyErrorCoversToolExecutionFailures(t *testing.T) {
	cases := map[string][]string{
		"unknown skill: file_exec":          {"unknown skill"},
		"blocked by trust gate: need allow": {"blocked by trust gate", "trust gate"},
		"tool panic: nil pointer":           {"tool panic", "panic"},
		`planner fc step 1: all fallback LLM clients failed (FC): chat with tools: Post "https://api.moonshot.ai/v1/chat/completions": EOF`: {"planner fc", "all fallback", "moonshot", "EOF"},
		"当前：调用栈降级，正在级联唤醒备用引擎 [qwen3.5:4b]...":                                                                                               {"调用栈降级", "级联唤醒", "qwen3.5"},
	}
	for raw, banned := range cases {
		friendly := plannerKnownFriendlyError(raw)
		if friendly == "" {
			t.Fatalf("expected friendly mapping for %q", raw)
		}
		lower := strings.ToLower(friendly)
		for _, term := range banned {
			if strings.Contains(lower, strings.ToLower(term)) {
				t.Fatalf("friendly error for %q still exposes %q: %q", raw, term, friendly)
			}
		}
		if !(strings.Contains(friendly, "现场已保留") || strings.Contains(friendly, "已保留现场")) {
			t.Fatalf("friendly error should keep recovery wording for %q, got %q", raw, friendly)
		}
	}
}

func TestPlannerCheckpointRecoverBuildsBackendPrompt(t *testing.T) {
	gw, tm := newTestGateway()
	tenant := tm.Register("planner-checkpoint-recover")
	store := planner.NewFileLongHorizonCheckpointStore(filepath.Join(t.TempDir(), "checkpoints.jsonl"))
	gw.planner.SetLongHorizonCheckpointStore(store)

	if err := store.Save(context.Background(), planner.LongHorizonCheckpoint{
		PlanID:      "plan-recover",
		TenantID:    tenant.ID,
		TaskID:      "task-recover",
		Goal:        "读取文档并继续规划",
		Status:      "failed",
		Completed:   1,
		Total:       3,
		Error:       "子步骤没有完成",
		Recoverable: true,
		ResumeHint:  "可继续、重试失败步骤，或先返回已完成部分。",
		UpdatedAt:   time.Now().UTC(),
		PlanSnapshot: []planner.PlanStep{
			{ID: 0, Action: "读取技术蓝图", Status: "done"},
			{ID: 1, Action: "执行失败步骤", Skill: "file_exec", Status: "failed", Error: "timeout"},
		},
	}); err != nil {
		t.Fatalf("save checkpoint: %v", err)
	}

	req := authedRequest(http.MethodPost, "/v1/planner/checkpoints/recover", `{"plan_id":"plan-recover","action":"retry_failed"}`, tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var resp plannerCheckpointRecoverResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Action != "retry_failed" || resp.PlanID != "plan-recover" || resp.TaskID != "task-recover" {
		t.Fatalf("unexpected recovery response: %+v", resp)
	}
	for _, want := range []string{"请重试这个可恢复规划里的失败步骤", "Plan ID：plan-recover", "原始目标：读取文档并继续规划", "本次恢复范围：继续 1 个步骤", "失败原因：子步骤没有完成", "失败模式：模型或子任务响应不稳定", "推荐策略：先返回阶段结果或切为后台任务", "执行失败步骤"} {
		if !strings.Contains(resp.Prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, resp.Prompt)
		}
	}
	if len(resp.Checkpoint.PlanSnapshot) != 2 {
		t.Fatalf("expected snapshot in recovery response, got %+v", resp.Checkpoint.PlanSnapshot)
	}
	if resp.RecoveryPlan.Mode != "retry_failed" || !resp.RecoveryPlan.Executable {
		t.Fatalf("expected executable retry recovery plan, got %+v", resp.RecoveryPlan)
	}
	if len(resp.RecoveryPlan.Steps) != 2 || resp.RecoveryPlan.Steps[0].Selected || !resp.RecoveryPlan.Steps[1].Selected {
		t.Fatalf("expected only failed step selected, got %+v", resp.RecoveryPlan.Steps)
	}
	if resp.RecoveryPlan.Prompt != resp.Prompt {
		t.Fatalf("expected response prompt to match recovery plan prompt")
	}
}

func TestPlannerCheckpointRecoverValidatesRequest(t *testing.T) {
	gw, tm := newTestGateway()
	tenant := tm.Register("planner-checkpoint-recover-validation")
	store := planner.NewFileLongHorizonCheckpointStore(filepath.Join(t.TempDir(), "checkpoints.jsonl"))
	gw.planner.SetLongHorizonCheckpointStore(store)

	req := authedRequest(http.MethodPost, "/v1/planner/checkpoints/recover", `{"plan_id":"","action":"continue"}`, tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing plan_id, got %d", w.Code)
	}

	req = authedRequest(http.MethodPost, "/v1/planner/checkpoints/recover", `{"plan_id":"missing","action":"continue"}`, tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing checkpoint, got %d", w.Code)
	}

	req = authedRequest(http.MethodPost, "/v1/planner/checkpoints/recover", `{"plan_id":"missing","action":"unknown"}`, tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unsupported action, got %d", w.Code)
	}
}

func TestPlannerCheckpointResumeTaskCreatesSelectedTaskSteps(t *testing.T) {
	gw, tm := newTestGateway()
	tenant := tm.Register("planner-checkpoint-resume-task")
	store := planner.NewFileLongHorizonCheckpointStore(filepath.Join(t.TempDir(), "checkpoints.jsonl"))
	gw.planner.SetLongHorizonCheckpointStore(store)
	taskStore := task.NewJSONStore(filepath.Join(t.TempDir(), "tasks"))
	gw.SetTaskStore(taskStore)
	gw.SetTaskRunner(task.NewRunner(taskStore, skills.NewRegistry(), func(ctx context.Context, system, user string) (string, error) {
		return "ok", nil
	}, nil))

	if err := store.Save(context.Background(), planner.LongHorizonCheckpoint{
		PlanID:      "plan-task",
		TenantID:    tenant.ID,
		TaskID:      "origin-task",
		Goal:        "恢复长程规划",
		Status:      "failed",
		Completed:   1,
		Total:       3,
		Recoverable: true,
		UpdatedAt:   time.Now().UTC(),
		PlanSnapshot: []planner.PlanStep{
			{ID: 0, Action: "已完成", Status: planner.StepDone, Result: "[Parsed document: 申请表.docx]\n公司名称\t云鸢科技\n联系电话\t13864841667"},
			{ID: 1, Action: "读取文档", Skill: "file_exec", Status: planner.StepPending, DependsOn: []int{0}, Args: map[string]any{"path": "doc"}},
			{ID: 2, Action: "修复 planner", Skill: "code_exec", Status: planner.StepFailed, DependsOn: []int{1}},
		},
	}); err != nil {
		t.Fatalf("save checkpoint: %v", err)
	}

	req := authedRequest(http.MethodPost, "/v1/planner/checkpoints/resume", `{"plan_id":"plan-task","action":"continue","run":false}`, tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", w.Code, w.Body.String())
	}
	var resp plannerCheckpointResumeTaskResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.TaskID == "" || resp.Run {
		t.Fatalf("unexpected resume task response: %+v", resp)
	}
	if !resp.RecoveryPlan.Executable || resp.RecoveryPlan.Mode != "continue" {
		t.Fatalf("unexpected recovery plan: %+v", resp.RecoveryPlan)
	}
	created, ok := taskStore.Get(resp.TaskID)
	if !ok {
		t.Fatalf("created task not found: %s", resp.TaskID)
	}
	if created.TenantID != tenant.ID {
		t.Fatalf("tenant mismatch: got %q want %q", created.TenantID, tenant.ID)
	}
	if len(created.Steps) != 2 {
		t.Fatalf("expected two selected task steps, got %+v", created.Steps)
	}
	if created.Steps[0].Action != "读取文档" || created.Steps[0].SkillName != "file_exec" {
		t.Fatalf("unexpected first task step: %+v", created.Steps[0])
	}
	if created.Steps[1].Action != "修复 planner" || created.Steps[1].SkillName != "code_exec" {
		t.Fatalf("unexpected second task step: %+v", created.Steps[1])
	}
	if created.Steps[0].Metadata["planner_step_id"] != 1 || created.Steps[1].Metadata["planner_step_id"] != 2 {
		t.Fatalf("expected original planner step ids in metadata, got %+v / %+v", created.Steps[0].Metadata, created.Steps[1].Metadata)
	}
	if len(created.Steps[0].DependsOn) != 0 {
		t.Fatalf("first selected step should not depend on completed-only step, got %+v", created.Steps[0].DependsOn)
	}
	for _, want := range []string{"申请表.docx", "公司名称\t云鸢科技", "联系电话\t13864841667"} {
		if !strings.Contains(created.Steps[0].Input, want) {
			t.Fatalf("first selected step should carry completed dependency evidence %q, got:\n%s", want, created.Steps[0].Input)
		}
	}
	if len(created.Steps[1].DependsOn) != 1 || created.Steps[1].DependsOn[0] != 1 {
		t.Fatalf("expected selected dependency remapped to task step id 1, got %+v", created.Steps[1].DependsOn)
	}
	if created.Constraints == nil || created.Constraints.Extra["plan_id"] != "plan-task" {
		t.Fatalf("expected planner checkpoint metadata, got %+v", created.Constraints)
	}
	if !strings.Contains(created.Description, "本次恢复范围：继续 2 个步骤") {
		t.Fatalf("task description missing recovery scope:\n%s", created.Description)
	}
}

func TestPlannerCheckpointResumeTaskRejectsUnsafeDependencies(t *testing.T) {
	gw, tm := newTestGateway()
	tenant := tm.Register("planner-checkpoint-resume-block")
	store := planner.NewFileLongHorizonCheckpointStore(filepath.Join(t.TempDir(), "checkpoints.jsonl"))
	gw.planner.SetLongHorizonCheckpointStore(store)
	taskStore := task.NewJSONStore(filepath.Join(t.TempDir(), "tasks"))
	gw.SetTaskStore(taskStore)
	gw.SetTaskRunner(task.NewRunner(taskStore, skills.NewRegistry(), func(ctx context.Context, system, user string) (string, error) {
		return "ok", nil
	}, nil))

	if err := store.Save(context.Background(), planner.LongHorizonCheckpoint{
		PlanID:      "plan-blocked",
		TenantID:    tenant.ID,
		Status:      "failed",
		Recoverable: true,
		UpdatedAt:   time.Now().UTC(),
		PlanSnapshot: []planner.PlanStep{
			{ID: 0, Action: "前置步骤", Status: planner.StepPending},
			{ID: 1, Action: "失败步骤", Status: planner.StepFailed, DependsOn: []int{0}},
		},
	}); err != nil {
		t.Fatalf("save checkpoint: %v", err)
	}

	req := authedRequest(http.MethodPost, "/v1/planner/checkpoints/resume", `{"plan_id":"plan-blocked","action":"retry_failed","run":false}`, tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unsafe dependency, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "尚未完成") {
		t.Fatalf("expected dependency reason, got %s", w.Body.String())
	}
}

func TestPlannerCheckpointResumePlanRunsDAGWithoutRerunningCompleted(t *testing.T) {
	gw, tm := newTestGateway()
	tenant := tm.Register("planner-checkpoint-resume-plan")
	store := planner.NewFileLongHorizonCheckpointStore(filepath.Join(t.TempDir(), "checkpoints.jsonl"))
	gw.planner = planner.NewPlanner(nil, gw.registry, 8)
	gw.planner.SetLongHorizonCheckpointStore(store)
	calls := 0
	gw.registry.Register(plannerGatewayTestSkill{
		name: "resume_next",
		execFn: func(_ context.Context, args map[string]any, _ *skills.Environment) (string, error) {
			calls++
			if args["path"] != "doc" {
				return "", fmt.Errorf("unexpected args: %#v", args)
			}
			return "resume ok", nil
		},
	})
	if err := store.Save(context.Background(), planner.LongHorizonCheckpoint{
		PlanID:      "plan-direct",
		TenantID:    tenant.ID,
		TaskID:      "task-direct",
		Goal:        "直接续跑 planner",
		Status:      "failed",
		Completed:   1,
		Total:       2,
		StepsUsed:   1,
		Recoverable: true,
		UpdatedAt:   time.Now().UTC(),
		PlanSnapshot: []planner.PlanStep{
			{ID: 0, Action: "已完成", Skill: "already_done", Status: planner.StepDone, Result: "done output"},
			{ID: 1, Action: "继续执行", Skill: "resume_next", Args: map[string]any{"path": "doc"}, DependsOn: []int{0}, Status: planner.StepPending},
		},
	}); err != nil {
		t.Fatalf("save checkpoint: %v", err)
	}

	req := authedRequest(http.MethodPost, "/v1/planner/checkpoints/resume-plan", `{"plan_id":"plan-direct","action":"continue"}`, tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var resp plannerCheckpointResumePlanResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Status != "completed" || resp.Result == nil || len(resp.Result.Plan) != 2 {
		t.Fatalf("unexpected resume-plan response: %+v", resp)
	}
	if calls != 1 {
		t.Fatalf("expected only pending step to execute once, got %d calls", calls)
	}
	if resp.Result.Plan[0].Result != "done output" || resp.Result.Plan[1].Result != "resume ok" {
		t.Fatalf("expected completed output preserved and pending output written, got %+v", resp.Result.Plan)
	}
}

func TestPlannerCheckpointResumePlanPartialReturnsCompletedAttachmentEvidence(t *testing.T) {
	gw, tm := newTestGateway()
	tenant := tm.Register("planner-checkpoint-resume-plan-partial-attachment")
	store := planner.NewFileLongHorizonCheckpointStore(filepath.Join(t.TempDir(), "checkpoints.jsonl"))
	gw.planner = planner.NewPlanner(nil, gw.registry, 8)
	gw.planner.SetLongHorizonCheckpointStore(store)
	calls := 0
	gw.registry.Register(plannerGatewayTestSkill{
		name: "partial_should_not_run",
		execFn: func(context.Context, map[string]any, *skills.Environment) (string, error) {
			calls++
			return "unexpected", nil
		},
	})
	rawErr := `handoff agent "file_exec" execution failed: context deadline exceeded EOF`
	if err := store.Save(context.Background(), planner.LongHorizonCheckpoint{
		PlanID:      "plan-partial-attachment",
		TenantID:    tenant.ID,
		TaskID:      "task-partial-attachment",
		Goal:        "根据申请表.docx 生成入驻申请材料",
		Status:      "failed",
		Completed:   1,
		Total:       2,
		Recoverable: true,
		PlanSnapshot: []planner.PlanStep{
			{ID: 0, Action: "读取附件", Status: planner.StepDone, Result: "[Parsed document: 申请表.docx]\n公司名称\t云鸢科技\n联系电话\t13864841667"},
			{ID: 1, Action: "根据附件生成申请材料", Skill: "partial_should_not_run", Status: planner.StepFailed, DependsOn: []int{0}, Error: rawErr},
		},
	}); err != nil {
		t.Fatalf("save checkpoint: %v", err)
	}

	req := authedRequest(http.MethodPost, "/v1/planner/checkpoints/resume-plan", `{"plan_id":"plan-partial-attachment","action":"partial"}`, tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if calls != 0 {
		t.Fatalf("partial result must not execute remaining steps, got %d calls", calls)
	}
	var resp plannerCheckpointResumePlanResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Status != "completed" || resp.Action != "partial" || resp.Result == nil {
		t.Fatalf("unexpected partial response: %+v", resp)
	}
	if resp.RecoveryPlan.Mode != "partial" || resp.RecoveryPlan.Executable {
		t.Fatalf("partial recovery plan should be non-executable, got %+v", resp.RecoveryPlan)
	}
	for _, want := range []string{"阶段结果：根据申请表.docx", "读取附件（已保留证据）", "申请表.docx", "公司名称\t云鸢科技", "联系电话\t13864841667"} {
		if !strings.Contains(resp.Result.Reply, want) {
			t.Fatalf("partial reply missing %q:\n%s", want, resp.Result.Reply)
		}
	}
	for _, banned := range []string{"handoff agent", "execution failed", "context deadline exceeded", "EOF"} {
		if strings.Contains(strings.ToLower(w.Body.String()), strings.ToLower(banned)) {
			t.Fatalf("partial response should hide raw diagnostics %q: %s", banned, w.Body.String())
		}
	}
}

func TestPlannerCheckpointResumePlanPartialAsyncJobCompletesWithEvidence(t *testing.T) {
	gw, tm := newTestGateway()
	tenant := tm.Register("planner-checkpoint-resume-plan-partial-async")
	store := planner.NewFileLongHorizonCheckpointStore(filepath.Join(t.TempDir(), "checkpoints.jsonl"))
	gw.planner = planner.NewPlanner(nil, gw.registry, 8)
	gw.planner.SetLongHorizonCheckpointStore(store)
	calls := 0
	gw.registry.Register(plannerGatewayTestSkill{
		name: "partial_async_should_not_run",
		execFn: func(context.Context, map[string]any, *skills.Environment) (string, error) {
			calls++
			return "unexpected", nil
		},
	})
	if err := store.Save(context.Background(), planner.LongHorizonCheckpoint{
		PlanID:      "plan-partial-async",
		TenantID:    tenant.ID,
		TaskID:      "task-partial-async",
		Goal:        "根据申请表.docx 生成入驻申请材料",
		Status:      "failed",
		Completed:   1,
		Total:       2,
		Recoverable: true,
		PlanSnapshot: []planner.PlanStep{
			{ID: 0, Action: "读取附件", Status: planner.StepDone, Result: "[Parsed document: 申请表.docx]\n公司名称\t云鸢科技\n联系电话\t13864841667"},
			{ID: 1, Action: "根据附件生成申请材料", Skill: "partial_async_should_not_run", Status: planner.StepFailed, DependsOn: []int{0}, Error: `context deadline exceeded EOF`},
		},
	}); err != nil {
		t.Fatalf("save checkpoint: %v", err)
	}

	req := authedRequest(http.MethodPost, "/v1/planner/checkpoints/resume-plan", `{"plan_id":"plan-partial-async","action":"partial","async":true}`, tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", w.Code, w.Body.String())
	}
	var accepted plannerCheckpointResumePlanResponse
	if err := json.NewDecoder(w.Body).Decode(&accepted); err != nil {
		t.Fatalf("decode accepted response: %v", err)
	}

	var jobResp plannerCheckpointResumePlanJobResponse
	for i := 0; i < 20; i++ {
		req = authedRequest(http.MethodGet, "/v1/planner/checkpoints/resume-plan/jobs?id="+accepted.JobID, "", tenant.APIKey)
		w = httptest.NewRecorder()
		gw.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected job 200, got %d body=%s", w.Code, w.Body.String())
		}
		jobResp = plannerCheckpointResumePlanJobResponse{}
		if err := json.NewDecoder(w.Body).Decode(&jobResp); err != nil {
			t.Fatalf("decode job response: %v", err)
		}
		if jobResp.Job.Status == "completed" || jobResp.Job.Status == "failed" {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if calls != 0 {
		t.Fatalf("partial async result must not execute remaining steps, got %d calls", calls)
	}
	if jobResp.Job.Status != "completed" || jobResp.Job.Result == nil {
		t.Fatalf("expected completed partial async job, got %+v", jobResp.Job)
	}
	for _, want := range []string{"阶段结果：根据申请表.docx", "读取附件（已保留证据）", "公司名称\t云鸢科技"} {
		if !strings.Contains(jobResp.Job.Result.Reply, want) {
			t.Fatalf("partial async reply missing %q:\n%s", want, jobResp.Job.Result.Reply)
		}
	}
}

func TestPlannerCheckpointResumePlanSyncFailureReturnsFriendlyAdvice(t *testing.T) {
	gw, tm := newTestGateway()
	tenant := tm.Register("planner-checkpoint-resume-plan-sync-failed")
	store := planner.NewFileLongHorizonCheckpointStore(filepath.Join(t.TempDir(), "checkpoints.jsonl"))
	mock := mockLLMServer("[]")
	defer mock.Close()
	gw.planner = planner.NewPlanner(llm.NewClient(mock.URL, "test-key", "test-model"), gw.registry, 8)
	gw.planner.SetLongHorizonCheckpointStore(store)
	gw.registry.Register(plannerGatewayTestSkill{
		name: "resume_sync_timeout",
		execFn: func(context.Context, map[string]any, *skills.Environment) (string, error) {
			return "", context.DeadlineExceeded
		},
	})
	if err := store.Save(context.Background(), planner.LongHorizonCheckpoint{
		PlanID:      "plan-sync-failed",
		TenantID:    tenant.ID,
		TaskID:      "task-sync-failed",
		Goal:        "同步续跑失败也要可恢复",
		Status:      "failed",
		Completed:   0,
		Total:       1,
		Recoverable: true,
		UpdatedAt:   time.Now().UTC(),
		PlanSnapshot: []planner.PlanStep{
			{ID: 0, Action: "会超时的同步步骤", Skill: "resume_sync_timeout", Status: planner.StepPending},
		},
	}); err != nil {
		t.Fatalf("save checkpoint: %v", err)
	}

	req := authedRequest(http.MethodPost, "/v1/planner/checkpoints/resume-plan", `{"plan_id":"plan-sync-failed","action":"continue"}`, tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with failed result payload, got %d body=%s", w.Code, w.Body.String())
	}
	var resp plannerCheckpointResumePlanResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Status != "failed" || !resp.Recoverable || resp.NextAction != "retry_failed" {
		t.Fatalf("expected friendly failed response, got %+v", resp)
	}
	if resp.FriendlyError == "" || strings.Contains(resp.FriendlyError, "context deadline exceeded") {
		t.Fatalf("expected sanitized friendly error, got %q", resp.FriendlyError)
	}
	if resp.Result == nil || !plannerResumePlanResultFailed(resp.Result) {
		t.Fatalf("expected failed plan result retained, got %+v", resp.Result)
	}
	if strings.Contains(strings.ToLower(fmt.Sprint(resp.Result.Plan)), "context deadline exceeded") {
		t.Fatalf("expected response result plan errors to be friendly, got %+v", resp.Result.Plan)
	}
}

func TestPlannerCheckpointResumePlanAsyncJobCanBePolled(t *testing.T) {
	gw, tm := newTestGateway()
	tenant := tm.Register("planner-checkpoint-resume-plan-async")
	store := planner.NewFileLongHorizonCheckpointStore(filepath.Join(t.TempDir(), "checkpoints.jsonl"))
	gw.planner = planner.NewPlanner(nil, gw.registry, 8)
	gw.planner.SetLongHorizonCheckpointStore(store)
	gw.registry.Register(plannerGatewayTestSkill{name: "resume_async"})
	if err := store.Save(context.Background(), planner.LongHorizonCheckpoint{
		PlanID:      "plan-async",
		TenantID:    tenant.ID,
		TaskID:      "task-async",
		Goal:        "异步续跑 planner",
		Status:      "failed",
		Completed:   0,
		Total:       1,
		Recoverable: true,
		UpdatedAt:   time.Now().UTC(),
		PlanSnapshot: []planner.PlanStep{
			{ID: 0, Action: "异步执行", Skill: "resume_async", Status: planner.StepPending},
		},
	}); err != nil {
		t.Fatalf("save checkpoint: %v", err)
	}

	req := authedRequest(http.MethodPost, "/v1/planner/checkpoints/resume-plan", `{"plan_id":"plan-async","action":"continue","async":true}`, tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", w.Code, w.Body.String())
	}
	var accepted plannerCheckpointResumePlanResponse
	if err := json.NewDecoder(w.Body).Decode(&accepted); err != nil {
		t.Fatalf("decode accepted response: %v", err)
	}
	if accepted.JobID == "" || accepted.Status != "accepted" {
		t.Fatalf("expected accepted job response, got %+v", accepted)
	}

	var jobResp plannerCheckpointResumePlanJobResponse
	for i := 0; i < 20; i++ {
		req = authedRequest(http.MethodGet, "/v1/planner/checkpoints/resume-plan/jobs?id="+accepted.JobID, "", tenant.APIKey)
		w = httptest.NewRecorder()
		gw.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected job 200, got %d body=%s", w.Code, w.Body.String())
		}
		jobResp = plannerCheckpointResumePlanJobResponse{}
		if err := json.NewDecoder(w.Body).Decode(&jobResp); err != nil {
			t.Fatalf("decode job response: %v", err)
		}
		if jobResp.Job.Status == "completed" {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if jobResp.Job.Status != "completed" || jobResp.Job.Result == nil || len(jobResp.Job.Result.Plan) != 1 {
		t.Fatalf("expected completed async resume job, got %+v", jobResp.Job)
	}
	if len(jobResp.Job.Events) == 0 {
		t.Fatalf("expected async resume job events, got %+v", jobResp.Job)
	}
	if jobResp.Job.Events[0].Type == "" || jobResp.Job.Events[0].Summary == "" {
		t.Fatalf("expected UI-safe event type and summary, got %+v", jobResp.Job.Events[0])
	}
	lastEvent := jobResp.Job.Events[len(jobResp.Job.Events)-1]
	if lastEvent.Type != "planner.resume_plan_done" || !strings.Contains(lastEvent.Summary, "原规划续跑已完成") {
		t.Fatalf("expected terminal completion event, got %+v", lastEvent)
	}
}

func TestPlannerCheckpointResumePlanAsyncDeduplicatesRunningJob(t *testing.T) {
	gw, tm := newTestGateway()
	tenant := tm.Register("planner-checkpoint-resume-plan-async-dedup")
	store := planner.NewFileLongHorizonCheckpointStore(filepath.Join(t.TempDir(), "checkpoints.jsonl"))
	gw.planner = planner.NewPlanner(nil, gw.registry, 8)
	gw.planner.SetLongHorizonCheckpointStore(store)
	var calls atomic.Int32
	gw.registry.Register(plannerGatewayTestSkill{
		name: "resume_async_dedup",
		execFn: func(context.Context, map[string]any, *skills.Environment) (string, error) {
			calls.Add(1)
			time.Sleep(150 * time.Millisecond)
			return "dedup ok", nil
		},
	})
	if err := store.Save(context.Background(), planner.LongHorizonCheckpoint{
		PlanID:      "plan-async-dedup",
		TenantID:    tenant.ID,
		TaskID:      "task-async-dedup",
		Goal:        "异步续跑不要重复执行同一个恢复点",
		Status:      "failed",
		Completed:   0,
		Total:       1,
		Recoverable: true,
		UpdatedAt:   time.Now().UTC(),
		PlanSnapshot: []planner.PlanStep{
			{ID: 0, Action: "异步执行但只应该跑一次", Skill: "resume_async_dedup", Status: planner.StepPending},
		},
	}); err != nil {
		t.Fatalf("save checkpoint: %v", err)
	}

	body := `{"plan_id":"plan-async-dedup","action":"continue","async":true}`
	req := authedRequest(http.MethodPost, "/v1/planner/checkpoints/resume-plan", body, tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("first request expected 202, got %d body=%s", w.Code, w.Body.String())
	}
	var first plannerCheckpointResumePlanResponse
	if err := json.NewDecoder(w.Body).Decode(&first); err != nil {
		t.Fatalf("decode first accepted response: %v", err)
	}
	if first.JobID == "" {
		t.Fatalf("first response missing job id: %+v", first)
	}

	req = authedRequest(http.MethodPost, "/v1/planner/checkpoints/resume-plan", body, tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("second request expected 202, got %d body=%s", w.Code, w.Body.String())
	}
	var second plannerCheckpointResumePlanResponse
	if err := json.NewDecoder(w.Body).Decode(&second); err != nil {
		t.Fatalf("decode second accepted response: %v", err)
	}
	if second.JobID != first.JobID {
		t.Fatalf("expected duplicate async request to reuse running job %q, got %q", first.JobID, second.JobID)
	}

	var jobResp plannerCheckpointResumePlanJobResponse
	for i := 0; i < 30; i++ {
		req = authedRequest(http.MethodGet, "/v1/planner/checkpoints/resume-plan/jobs?id="+first.JobID, "", tenant.APIKey)
		w = httptest.NewRecorder()
		gw.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected job 200, got %d body=%s", w.Code, w.Body.String())
		}
		jobResp = plannerCheckpointResumePlanJobResponse{}
		if err := json.NewDecoder(w.Body).Decode(&jobResp); err != nil {
			t.Fatalf("decode job response: %v", err)
		}
		if jobResp.Job.Status == "completed" || jobResp.Job.Status == "failed" {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if jobResp.Job.Status != "completed" {
		t.Fatalf("expected completed deduplicated job, got %+v", jobResp.Job)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("expected running duplicate request to execute the resumed step once, got %d calls", got)
	}
}

func TestPlannerCheckpointResumePlanAsyncJobReturnsFriendlyFailureAdvice(t *testing.T) {
	gw, tm := newTestGateway()
	tenant := tm.Register("planner-checkpoint-resume-plan-async-failed")
	store := planner.NewFileLongHorizonCheckpointStore(filepath.Join(t.TempDir(), "checkpoints.jsonl"))
	mock := mockLLMServer("[]")
	defer mock.Close()
	gw.planner = planner.NewPlanner(llm.NewClient(mock.URL, "test-key", "test-model"), gw.registry, 8)
	gw.planner.SetLongHorizonCheckpointStore(store)
	gw.registry.Register(plannerGatewayTestSkill{
		name: "resume_timeout",
		execFn: func(context.Context, map[string]any, *skills.Environment) (string, error) {
			return "", context.DeadlineExceeded
		},
	})
	if err := store.Save(context.Background(), planner.LongHorizonCheckpoint{
		PlanID:      "plan-async-failed",
		TenantID:    tenant.ID,
		TaskID:      "task-async-failed",
		Goal:        "异步续跑失败也要可恢复",
		Status:      "failed",
		Completed:   0,
		Total:       1,
		Recoverable: true,
		UpdatedAt:   time.Now().UTC(),
		PlanSnapshot: []planner.PlanStep{
			{ID: 0, Action: "会超时的步骤", Skill: "resume_timeout", Status: planner.StepPending},
		},
	}); err != nil {
		t.Fatalf("save checkpoint: %v", err)
	}

	req := authedRequest(http.MethodPost, "/v1/planner/checkpoints/resume-plan", `{"plan_id":"plan-async-failed","action":"continue","async":true}`, tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", w.Code, w.Body.String())
	}
	var accepted plannerCheckpointResumePlanResponse
	if err := json.NewDecoder(w.Body).Decode(&accepted); err != nil {
		t.Fatalf("decode accepted response: %v", err)
	}

	var jobResp plannerCheckpointResumePlanJobResponse
	for i := 0; i < 20; i++ {
		req = authedRequest(http.MethodGet, "/v1/planner/checkpoints/resume-plan/jobs?id="+accepted.JobID, "", tenant.APIKey)
		w = httptest.NewRecorder()
		gw.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected job 200, got %d body=%s", w.Code, w.Body.String())
		}
		jobResp = plannerCheckpointResumePlanJobResponse{}
		if err := json.NewDecoder(w.Body).Decode(&jobResp); err != nil {
			t.Fatalf("decode job response: %v", err)
		}
		if jobResp.Job.Status == "failed" {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if jobResp.Job.Status != "failed" {
		t.Fatalf("expected failed async resume job, got %+v", jobResp.Job)
	}
	if !jobResp.Job.Recoverable || jobResp.Job.NextAction != "retry_failed" {
		t.Fatalf("expected retry advice, got %+v", jobResp.Job)
	}
	if jobResp.Job.FriendlyError == "" || strings.Contains(jobResp.Job.FriendlyError, "context deadline exceeded") {
		t.Fatalf("expected friendly sanitized error, got %q", jobResp.Job.FriendlyError)
	}
	if jobResp.Job.Error == "" || strings.Contains(strings.ToLower(jobResp.Job.Error), "context deadline exceeded") {
		t.Fatalf("expected endpoint job error to be friendly, got %q", jobResp.Job.Error)
	}
	if len(jobResp.Job.Events) == 0 {
		t.Fatalf("expected failed job to keep event history")
	}
	lastEvent := jobResp.Job.Events[len(jobResp.Job.Events)-1]
	if lastEvent.Type != "planner.resume_plan_done" {
		t.Fatalf("expected terminal failure event, got %+v", lastEvent)
	}
	for _, evt := range jobResp.Job.Events {
		if strings.Contains(evt.Summary, "context deadline exceeded") {
			t.Fatalf("expected event summary to hide raw timeout, got %+v", evt)
		}
	}
}

func TestPlannerResumePlanJobEndpointSanitizesStoredRawFailure(t *testing.T) {
	gw, tm := newTestGateway()
	tenant := tm.Register("planner-resume-job-sanitized")
	raw := `handoff agent "file_exec" execution failed: planner fc step 1: all fallback LLM clients failed (FC): chat with tools: Post "https://api.moonshot.ai/v1/chat/completions": EOF; context deadline exceeded`
	gw.savePlannerResumeJob(plannerCheckpointResumePlanJob{
		ID:        "resume-plan-raw",
		Status:    "failed",
		Action:    "continue",
		TenantID:  tenant.ID,
		PlanID:    "plan-raw",
		TaskID:    "task-raw",
		Error:     raw,
		StartedAt: "2026-05-11T01:00:00Z",
		Result: &planner.PlanResult{Plan: []planner.PlanStep{
			{ID: 0, Action: "解析文档", Skill: "file_exec", Status: planner.StepFailed, Error: raw},
		}},
		Events: []plannerCheckpointResumePlanJobEvent{
			{ID: "evt-raw", Type: "planner.tool_result", Summary: raw, Timestamp: "2026-05-11T01:00:01Z"},
		},
	})

	for _, path := range []string{
		"/v1/planner/checkpoints/resume-plan/jobs?id=resume-plan-raw",
		"/v1/planner/checkpoints/resume-plan/jobs?job_id=resume-plan-raw",
		"/v1/planner/checkpoints/resume-plan/jobs?plan_id=plan-raw",
	} {
		req := authedRequest(http.MethodGet, path, "", tenant.APIKey)
		w := httptest.NewRecorder()
		gw.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("%s: expected job 200, got %d body=%s", path, w.Code, w.Body.String())
		}
		body := strings.ToLower(w.Body.String())
		for _, banned := range []string{"handoff agent", "execution failed", "context deadline exceeded", "all fallback", "eof", "api.moonshot.ai"} {
			if strings.Contains(body, banned) {
				t.Fatalf("%s: job response should be friendly by default; found %q in %s", path, banned, w.Body.String())
			}
		}
		if !strings.Contains(w.Body.String(), "现场已保留") {
			t.Fatalf("%s: expected actionable friendly recovery wording, got %s", path, w.Body.String())
		}
	}
}

func TestPlannerResumePlanJobEndpointScopesJobsToTenant(t *testing.T) {
	gw, tm := newTestGateway()
	owner := tm.Register("planner-resume-job-owner")
	other := tm.Register("planner-resume-job-other")
	gw.savePlannerResumeJob(plannerCheckpointResumePlanJob{
		ID:        "resume-plan-tenant-owned",
		Status:    "completed",
		Action:    "continue",
		TenantID:  owner.ID,
		PlanID:    "plan-shared",
		StartedAt: "2026-05-11T04:00:00Z",
	})

	for _, path := range []string{
		"/v1/planner/checkpoints/resume-plan/jobs?id=resume-plan-tenant-owned",
		"/v1/planner/checkpoints/resume-plan/jobs?job_id=resume-plan-tenant-owned",
		"/v1/planner/checkpoints/resume-plan/jobs?plan_id=plan-shared",
	} {
		req := authedRequest(http.MethodGet, path, "", other.APIKey)
		w := httptest.NewRecorder()
		gw.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("%s: other tenant should not read owner resume job, got %d body=%s", path, w.Code, w.Body.String())
		}
	}

	req := authedRequest(http.MethodGet, "/v1/planner/checkpoints/resume-plan/jobs?job_id=resume-plan-tenant-owned", "", owner.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("owner should read resume job with job_id alias, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestPlannerResumePlanJobEventSummaryCoversToolExecutionFailures(t *testing.T) {
	cases := map[string][]string{
		"执行失败: unknown skill: missing_tool":        {"unknown skill"},
		"blocked by trust gate: need confirmation": {"blocked by trust gate", "trust gate"},
		"tool panic: nil pointer":                  {"tool panic", "panic"},
	}
	for raw, banned := range cases {
		friendly := plannerResumeJobEventDisplaySummary(raw)
		if friendly == "" || friendly == raw {
			t.Fatalf("expected friendly event summary for %q, got %q", raw, friendly)
		}
		lower := strings.ToLower(friendly)
		for _, term := range banned {
			if strings.Contains(lower, strings.ToLower(term)) {
				t.Fatalf("friendly event summary for %q still exposes %q: %q", raw, term, friendly)
			}
		}
		if !strings.Contains(friendly, "现场") {
			t.Fatalf("friendly event summary should keep recovery wording for %q, got %q", raw, friendly)
		}
	}
}

func TestPlannerCheckpointRecoverPlanSelectsContinueSteps(t *testing.T) {
	plan := buildPlannerCheckpointPlan(planner.LongHorizonCheckpoint{
		PlanID: "plan-continue",
		PlanSnapshot: []planner.PlanStep{
			{ID: 0, Action: "已完成", Status: planner.StepDone},
			{ID: 1, Action: "待执行", Status: planner.StepPending, DependsOn: []int{0}},
			{ID: 2, Action: "失败项", Status: planner.StepFailed, DependsOn: []int{1}},
			{ID: 3, Action: "已跳过", Status: planner.StepSkipped},
		},
	}, "continue", "prompt")

	if !plan.Executable || plan.Reason != "" {
		t.Fatalf("expected executable continue plan, got %+v", plan)
	}
	selected := map[int]bool{}
	deps := map[int][]int{}
	for _, step := range plan.Steps {
		selected[step.ID] = step.Selected
		deps[step.ID] = step.DependsOn
	}
	if selected[0] || !selected[1] || !selected[2] || selected[3] {
		t.Fatalf("unexpected continue step selection: %+v", plan.Steps)
	}
	if len(deps[1]) != 1 || deps[1][0] != 0 || len(deps[2]) != 1 || deps[2][0] != 1 {
		t.Fatalf("recovery plan should preserve planner dependencies, got %+v", plan.Steps)
	}
}

func TestPlannerCheckpointRecoverPlanPartialIsNotExecutable(t *testing.T) {
	plan := buildPlannerCheckpointPlan(planner.LongHorizonCheckpoint{
		PlanID: "plan-partial",
		PlanSnapshot: []planner.PlanStep{
			{ID: 0, Action: "已完成", Status: planner.StepDone},
			{ID: 1, Action: "失败项", Status: planner.StepFailed},
		},
	}, "partial", "prompt")

	if plan.Executable {
		t.Fatalf("expected partial recovery plan to be non-executable: %+v", plan)
	}
	for _, step := range plan.Steps {
		if step.Selected {
			t.Fatalf("partial should not select execution steps: %+v", plan.Steps)
		}
	}
	if !strings.Contains(plan.Reason, "已完成部分") {
		t.Fatalf("expected partial reason, got %q", plan.Reason)
	}
}

func TestPlannerCheckpointRecoverPlanBlocksUnmetDependency(t *testing.T) {
	plan := buildPlannerCheckpointPlan(planner.LongHorizonCheckpoint{
		PlanID: "plan-dep",
		PlanSnapshot: []planner.PlanStep{
			{ID: 0, Action: "前置步骤", Status: planner.StepPending},
			{ID: 1, Action: "失败项", Status: planner.StepFailed, DependsOn: []int{0}},
		},
	}, "retry_failed", "prompt")

	if plan.Executable {
		t.Fatalf("expected unmet dependency to block direct execution: %+v", plan)
	}
	if !strings.Contains(plan.Reason, "尚未完成") {
		t.Fatalf("expected dependency reason, got %q", plan.Reason)
	}
	if len(plan.Steps) != 2 || plan.Steps[0].Selected || !plan.Steps[1].Selected {
		t.Fatalf("retry_failed should select failed step only, got %+v", plan.Steps)
	}
}

func TestPlannerResumePlanJobPersistsAcrossGatewayInstances(t *testing.T) {
	path := filepath.Join(t.TempDir(), "resume_plan_jobs.jsonl")
	g1 := &Gateway{}
	g1.SetPlannerResumeJobStore(path)
	g1.savePlannerResumeJob(plannerCheckpointResumePlanJob{
		ID:         "resume-plan-persist-1",
		Status:     "completed",
		Action:     "continue",
		TenantID:   "tenant-persist",
		PlanID:     "plan-persist",
		StartedAt:  "2026-05-11T00:00:00Z",
		FinishedAt: "2026-05-11T00:00:03Z",
		Result:     &planner.PlanResult{Plan: []planner.PlanStep{{ID: 1, Action: "继续修复 Planner", Status: planner.StepDone}}},
		Events:     []plannerCheckpointResumePlanJobEvent{{ID: "evt-persist", Type: "planner.tool_result", Summary: "恢复完成", Timestamp: "2026-05-11T00:00:02Z"}},
	})

	g2 := &Gateway{}
	g2.SetPlannerResumeJobStore(path)
	job, ok := g2.getPlannerResumeJob("resume-plan-persist-1", "tenant-persist")
	if !ok {
		t.Fatal("expected persisted resume-plan job")
	}
	if job.Status != "completed" || job.PlanID != "plan-persist" {
		t.Fatalf("unexpected job: %+v", job)
	}
	if job.Result == nil || len(job.Result.Plan) != 1 || job.Result.Plan[0].Action != "继续修复 Planner" {
		t.Fatalf("persisted result not restored: %+v", job.Result)
	}
	if len(job.Events) != 1 || job.Events[0].Summary != "恢复完成" {
		t.Fatalf("persisted events not restored: %+v", job.Events)
	}
}

func TestPlannerResumePlanJobCanBeResolvedByPlanID(t *testing.T) {
	gw, tm := newTestGateway()
	tenant := tm.Register("planner-resume-job-by-plan")
	gw.savePlannerResumeJob(plannerCheckpointResumePlanJob{
		ID:        "resume-plan-old",
		Status:    "running",
		Action:    "continue",
		TenantID:  tenant.ID,
		PlanID:    "plan-latest",
		StartedAt: "2026-05-11T00:00:01Z",
	})
	gw.savePlannerResumeJob(plannerCheckpointResumePlanJob{
		ID:        "resume-plan-new",
		Status:    "completed",
		Action:    "continue",
		TenantID:  tenant.ID,
		PlanID:    "plan-latest",
		StartedAt: "2026-05-11T00:00:02Z",
	})

	req := authedRequest(http.MethodGet, "/v1/planner/checkpoints/resume-plan/jobs?plan_id=plan-latest", "", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected job 200, got %d body=%s", w.Code, w.Body.String())
	}
	var resp plannerCheckpointResumePlanJobResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode job response: %v", err)
	}
	if resp.Job.ID != "resume-plan-new" {
		t.Fatalf("expected latest job by plan, got %+v", resp.Job)
	}
}

func TestPlannerExecutionStateUnifiesCheckpointLatestJobAndFailureSummary(t *testing.T) {
	gw, tm := newTestGateway()
	tenant := tm.Register("planner-execution-state")
	store := planner.NewFileLongHorizonCheckpointStore(filepath.Join(t.TempDir(), "checkpoints.jsonl"))
	gw.planner = planner.NewPlanner(nil, gw.registry, 8)
	gw.planner.SetLongHorizonCheckpointStore(store)
	raw := `handoff agent "file_exec" execution failed: planner fc step 1: all fallback LLM clients failed (FC): chat with tools: Post "https://api.moonshot.ai/v1/chat/completions": EOF; context deadline exceeded`
	if err := store.Save(context.Background(), planner.LongHorizonCheckpoint{
		PlanID:      "plan-state",
		TenantID:    tenant.ID,
		TaskID:      "task-state",
		Goal:        "继续推进 Planner",
		Status:      "failed",
		Completed:   1,
		Total:       2,
		Error:       raw,
		Recoverable: true,
		UpdatedAt:   time.Date(2026, 5, 11, 1, 0, 0, 0, time.UTC),
		PlanSnapshot: []planner.PlanStep{
			{ID: 0, Action: "读取技术蓝图", Skill: "file_open", Status: planner.StepDone, Result: "已读取 doc"},
			{ID: 1, Action: "续跑失败步骤", Skill: "file_exec", Status: planner.StepFailed, Error: raw},
		},
	}); err != nil {
		t.Fatalf("append checkpoint: %v", err)
	}
	gw.savePlannerResumeJob(plannerCheckpointResumePlanJob{
		ID:            "resume-plan-state",
		Status:        "failed",
		Action:        "retry_failed",
		TenantID:      tenant.ID,
		PlanID:        "plan-state",
		TaskID:        "task-state",
		Error:         raw,
		FriendlyError: raw,
		Recoverable:   true,
		NextAction:    "retry_failed",
		StartedAt:     "2026-05-11T01:00:01Z",
		FinishedAt:    "2026-05-11T01:00:02Z",
		Events:        []plannerCheckpointResumePlanJobEvent{{ID: "evt-state", Type: "planner.tool_result", Summary: raw, Timestamp: "2026-05-11T01:00:02Z"}},
		Result: &planner.PlanResult{Plan: []planner.PlanStep{
			{ID: 0, Action: "读取技术蓝图", Skill: "file_open", Status: planner.StepDone, Result: "已读取 doc"},
			{ID: 1, Action: "续跑失败步骤", Skill: "file_exec", Status: planner.StepFailed, Error: raw},
		}},
	})

	req := authedRequest(http.MethodGet, "/v1/planner/execution-state?plan_id=plan-state", "", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected execution-state 200, got %d body=%s", w.Code, w.Body.String())
	}
	responseBody := w.Body.String()
	var resp plannerExecutionStateResponse
	if err := json.NewDecoder(strings.NewReader(responseBody)).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.PlanID != "plan-state" || resp.Status != "failed" || resp.Action != "retry_failed" || resp.NextAction != "retry_failed" {
		t.Fatalf("unexpected execution state header: %+v", resp)
	}
	if resp.Checkpoint == nil || resp.Checkpoint.Error == "" || strings.Contains(resp.Checkpoint.Error, "context deadline exceeded") {
		t.Fatalf("checkpoint should be present with friendly error, got %+v", resp.Checkpoint)
	}
	if resp.LatestJob == nil || resp.LatestJob.ID != "resume-plan-state" || len(resp.Events) != 1 {
		t.Fatalf("expected latest job and events, got job=%+v events=%+v", resp.LatestJob, resp.Events)
	}
	if resp.RecoveryPlan == nil || resp.RecoveryPlan.Mode != "retry_failed" || len(resp.RecoveryPlan.Steps) != 2 {
		t.Fatalf("expected retry recovery plan, got %+v", resp.RecoveryPlan)
	}
	if resp.FailureSummary == nil || resp.FailureSummary.FailedCount != 1 || resp.FailureSummary.CompletedCount != 1 {
		t.Fatalf("expected compact failure summary, got %+v", resp.FailureSummary)
	}
	encoded := strings.ToLower(responseBody)
	for _, banned := range []string{"handoff agent", "execution failed", "context deadline exceeded", "all fallback", "eof", "api.moonshot.ai"} {
		if strings.Contains(encoded, banned) {
			t.Fatalf("execution-state response should not expose raw diagnostic %q, got %s", banned, responseBody)
		}
	}
	if !strings.Contains(responseBody, "现场") {
		t.Fatalf("execution-state response should keep actionable friendly recovery wording, got %s", responseBody)
	}
}

func TestPlannerExecutionStateRecoveryPromptKeepsParsedAttachmentEvidence(t *testing.T) {
	gw, tm := newTestGateway()
	tenant := tm.Register("planner-execution-state-attachment")
	store := planner.NewFileLongHorizonCheckpointStore(filepath.Join(t.TempDir(), "checkpoints.jsonl"))
	gw.planner = planner.NewPlanner(nil, gw.registry, 8)
	gw.planner.SetLongHorizonCheckpointStore(store)
	rawErr := `handoff agent "file_exec" execution failed: context deadline exceeded`

	if err := store.Save(context.Background(), planner.LongHorizonCheckpoint{
		PlanID:      "plan-attachment-state",
		TenantID:    tenant.ID,
		TaskID:      "task-attachment-state",
		Goal:        "根据申请表.docx 生成入驻申请材料",
		Status:      "failed",
		Completed:   1,
		Total:       2,
		Error:       rawErr,
		Recoverable: true,
		UpdatedAt:   time.Date(2026, 5, 11, 2, 30, 0, 0, time.UTC),
		PlanSnapshot: []planner.PlanStep{
			{
				ID:     0,
				Action: "读取附件",
				Skill:  "file_open",
				Status: planner.StepDone,
				Result: "[Parsed document: 申请表.docx]\n公司名称\t云鸢科技\n联系电话\t13864841667",
			},
			{
				ID:        1,
				Action:    "根据附件生成申请材料",
				Skill:     "doc_writer",
				Status:    planner.StepFailed,
				DependsOn: []int{0},
				Error:     rawErr,
			},
		},
	}); err != nil {
		t.Fatalf("append checkpoint: %v", err)
	}

	req := authedRequest(http.MethodGet, "/v1/planner/execution-state?plan_id=plan-attachment-state", "", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected execution-state 200, got %d body=%s", w.Code, w.Body.String())
	}
	responseBody := w.Body.String()
	var resp plannerExecutionStateResponse
	if err := json.NewDecoder(strings.NewReader(responseBody)).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Action != "retry_failed" || resp.NextAction != "retry_failed" {
		t.Fatalf("expected retry_failed defaults, got action=%q next=%q", resp.Action, resp.NextAction)
	}
	if resp.RecoveryPlan == nil || resp.RecoveryPlan.Mode != "retry_failed" || !resp.RecoveryPlan.Executable {
		t.Fatalf("expected executable retry recovery plan, got %+v", resp.RecoveryPlan)
	}
	if resp.FailureSummary == nil || resp.FailureSummary.CompletedCount != 1 || resp.FailureSummary.FailedCount != 1 {
		t.Fatalf("expected completed attachment step and failed generation step, got %+v", resp.FailureSummary)
	}
	if resp.FailureSummary.FailurePattern != "模型或子任务响应不稳定" || !strings.Contains(resp.FailureSummary.Recommendation, "降低任务粒度") {
		t.Fatalf("expected failure analysis in execution state, got %+v", resp.FailureSummary)
	}
	for _, want := range []string{"申请表.docx", "公司名称\\t云鸢科技", "联系电话\\t13864841667"} {
		if !strings.Contains(responseBody, want) {
			t.Fatalf("execution-state should preserve parsed attachment evidence %q, got %s", want, responseBody)
		}
	}
	for _, want := range []string{"申请表.docx", "公司名称\t云鸢科技", "联系电话\t13864841667", "不要重复已完成步骤", "失败模式：模型或子任务响应不稳定", "推荐策略：先返回阶段结果或切为后台任务"} {
		if !strings.Contains(resp.RecoveryPlan.Prompt, want) {
			t.Fatalf("recovery prompt should preserve attachment evidence %q, got:\n%s", want, resp.RecoveryPlan.Prompt)
		}
	}
	encoded := strings.ToLower(responseBody)
	for _, banned := range []string{"handoff agent", "execution failed", "context deadline exceeded"} {
		if strings.Contains(encoded, banned) {
			t.Fatalf("execution-state response should not expose raw diagnostic %q, got %s", banned, responseBody)
		}
	}
}

func TestPlannerExecutionStateHidesRawCompletedResultDiagnostics(t *testing.T) {
	gw, tm := newTestGateway()
	tenant := tm.Register("planner-execution-state-friendly-result")
	store := planner.NewFileLongHorizonCheckpointStore(filepath.Join(t.TempDir(), "checkpoints.jsonl"))
	gw.planner = planner.NewPlanner(nil, gw.registry, 8)
	gw.planner.SetLongHorizonCheckpointStore(store)
	rawResult := `子代理返回：handoff agent "file_exec" execution failed: context deadline exceeded EOF，但现场已保留。`
	rawErr := `context deadline exceeded EOF`

	if err := store.Save(context.Background(), planner.LongHorizonCheckpoint{
		PlanID:      "plan-state-friendly-result",
		TenantID:    tenant.ID,
		TaskID:      "task-state-friendly-result",
		Goal:        "继续整理阶段结果",
		Status:      "failed",
		Completed:   1,
		Total:       2,
		Recoverable: true,
		UpdatedAt:   time.Date(2026, 5, 11, 3, 0, 0, 0, time.UTC),
		PlanSnapshot: []planner.PlanStep{
			{ID: 0, Action: "读取附件", Skill: "file_exec", Status: planner.StepDone, Result: rawResult},
			{ID: 1, Action: "继续整理", Skill: "doc_writer", Status: planner.StepFailed, DependsOn: []int{0}, Error: rawErr},
		},
	}); err != nil {
		t.Fatalf("append checkpoint: %v", err)
	}
	gw.savePlannerResumeJob(plannerCheckpointResumePlanJob{
		ID:         "resume-plan-friendly-result",
		Status:     "failed",
		Action:     "retry_failed",
		TenantID:   tenant.ID,
		PlanID:     "plan-state-friendly-result",
		TaskID:     "task-state-friendly-result",
		Error:      rawErr,
		StartedAt:  "2026-05-11T03:00:01Z",
		FinishedAt: "2026-05-11T03:00:02Z",
		Result: &planner.PlanResult{Plan: []planner.PlanStep{
			{ID: 0, Action: "读取附件", Skill: "file_exec", Status: planner.StepDone, Result: rawResult},
			{ID: 1, Action: "继续整理", Skill: "doc_writer", Status: planner.StepFailed, Error: rawErr},
		}},
	})

	req := authedRequest(http.MethodGet, "/v1/planner/execution-state?plan_id=plan-state-friendly-result", "", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected execution-state 200, got %d body=%s", w.Code, w.Body.String())
	}
	responseBody := w.Body.String()
	var resp plannerExecutionStateResponse
	if err := json.NewDecoder(strings.NewReader(responseBody)).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Checkpoint == nil || len(resp.Checkpoint.PlanSnapshot) != 2 {
		t.Fatalf("expected checkpoint snapshot, got %+v", resp.Checkpoint)
	}
	if got := resp.Checkpoint.PlanSnapshot[0].Result; !strings.Contains(got, "已保留现场") {
		t.Fatalf("expected friendly checkpoint completed result, got %q", got)
	}
	if resp.LatestJob == nil || resp.LatestJob.Result == nil || len(resp.LatestJob.Result.Plan) != 2 {
		t.Fatalf("expected sanitized latest job result, got %+v", resp.LatestJob)
	}
	if got := resp.LatestJob.Result.Plan[0].Result; !strings.Contains(got, "已保留现场") {
		t.Fatalf("expected friendly latest job completed result, got %q", got)
	}
	if resp.FailureSummary == nil || len(resp.FailureSummary.Tried) == 0 || !strings.Contains(strings.Join(resp.FailureSummary.Tried, "\n"), "已保留现场") {
		t.Fatalf("expected friendly completed result in failure summary tried list, got %+v", resp.FailureSummary)
	}
	encoded := strings.ToLower(responseBody)
	for _, banned := range []string{"handoff agent", "execution failed", "context deadline exceeded", "eof"} {
		if strings.Contains(encoded, banned) {
			t.Fatalf("execution-state response should hide raw completed result diagnostic %q, got %s", banned, responseBody)
		}
	}
}

func TestPlannerExecutionStateIncludesCogniSummaryFromEventTrail(t *testing.T) {
	gw, tm := newTestGateway()
	tenant := tm.Register("planner-execution-state-cogni")
	store := planner.NewFileLongHorizonCheckpointStore(filepath.Join(t.TempDir(), "checkpoints.jsonl"))
	gw.planner = planner.NewPlanner(nil, gw.registry, 8)
	gw.planner.SetLongHorizonCheckpointStore(store)
	trail := observe.NewAuditTrail(20)
	gw.SetEventTrail(trail)

	if err := store.Save(context.Background(), planner.LongHorizonCheckpoint{
		PlanID:      "plan-cogni-state",
		TenantID:    tenant.ID,
		TaskID:      "task-cogni-state",
		Goal:        "读取文档并续跑 Planner",
		Status:      "running",
		Completed:   0,
		Total:       1,
		Recoverable: true,
		UpdatedAt:   time.Date(2026, 5, 11, 2, 0, 0, 0, time.UTC),
		PlanSnapshot: []planner.PlanStep{
			{ID: 0, Action: "读取文档", Skill: "file_open", Status: planner.StepPending},
		},
	}); err != nil {
		t.Fatalf("append checkpoint: %v", err)
	}
	trail.Record(observe.AgentEvent{
		ID:        "evt-cogni-state",
		TraceID:   "trace-cogni-state",
		Timestamp: time.Date(2026, 5, 11, 2, 0, 1, 0, time.UTC),
		Domain:    observe.DomainPlanner,
		Type:      observe.EventPlan,
		Summary:   "Cogni 已激活：文档助手，工具面 12 → 3",
		Detail: planner.CogniTraceDetail{
			Activated:    []string{"文档助手"},
			ContextBytes: 256,
			ToolBefore:   12,
			ToolAfter:    3,
			Removed:      []string{"通用文件转储"},
		},
		Meta: observe.EventMeta{TaskID: "task-cogni-state"},
	})
	trail.Record(observe.AgentEvent{
		ID:        "evt-other-task",
		TraceID:   "trace-other",
		Timestamp: time.Date(2026, 5, 11, 2, 0, 2, 0, time.UTC),
		Domain:    observe.DomainPlanner,
		Type:      observe.EventPlan,
		Summary:   "Cogni 已激活：不应混入",
		Detail:    planner.CogniTraceDetail{Activated: []string{"不应混入"}},
		Meta:      observe.EventMeta{TaskID: "task-other"},
	})

	req := authedRequest(http.MethodGet, "/v1/planner/execution-state?plan_id=plan-cogni-state", "", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected execution-state 200, got %d body=%s", w.Code, w.Body.String())
	}
	var resp plannerExecutionStateResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Cogni == nil {
		t.Fatal("expected cogni summary")
	}
	if resp.Cogni.ContextBytes != 256 || resp.Cogni.ToolBefore != 12 || resp.Cogni.ToolAfter != 3 {
		t.Fatalf("unexpected cogni counters: %+v", resp.Cogni)
	}
	if len(resp.Cogni.Activated) != 1 || resp.Cogni.Activated[0] != "文档助手" {
		t.Fatalf("unexpected activated cogni list: %+v", resp.Cogni.Activated)
	}
	if len(resp.Cogni.Events) != 1 || resp.Cogni.Events[0].ID != "evt-cogni-state" {
		t.Fatalf("expected only matching cogni event, got %+v", resp.Cogni.Events)
	}
	if strings.Contains(w.Body.String(), "不应混入") {
		t.Fatalf("execution-state should not include cogni events from other tasks: %s", w.Body.String())
	}
}
