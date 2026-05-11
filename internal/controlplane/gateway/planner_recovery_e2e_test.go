package gateway

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"yunque-agent/internal/agentcore/planner"
)

func TestE2E_PlannerCheckpointRecoveryAcrossRestart(t *testing.T) {
	mock := mockLLMServer("ok")
	defer mock.Close()

	storePath := filepath.Join(t_tempDir(), "planner-checkpoints.jsonl")
	store := planner.NewFileLongHorizonCheckpointStore(storePath)
	checkpoint := planner.LongHorizonCheckpoint{
		PlanID:      "plan-recovery-e2e",
		TaskID:      "task-recovery-e2e",
		Goal:        "继续修复 planner 并保留恢复现场",
		Status:      "failed",
		CurrentStep: 1,
		Completed:   1,
		Total:       2,
		StepsUsed:   4,
		Revisions:   1,
		Error:       "context deadline exceeded",
		Recoverable: true,
		ResumeHint:  "可根据 plan_snapshot 继续、重试失败步骤，或先返回已完成部分。",
		PlanSnapshot: []planner.PlanStep{
			{
				ID:     0,
				Action: "读取技术蓝图",
				Skill:  "file_open",
				Status: planner.StepDone,
				Result: "[Parsed document: 技术蓝图.md]\n- planner\n- recovery\n",
			},
			{
				ID:        1,
				Action:    "继续修复 Planner",
				Skill:     "code_edit",
				Status:    planner.StepFailed,
				DependsOn: []int{0},
				Error:     "unknown skill: file_exec",
			},
		},
		UpdatedAt: time.Now().UTC(),
	}
	if err := store.Save(context.Background(), checkpoint); err != nil {
		t.Fatalf("save checkpoint: %v", err)
	}

	gw, tm := newE2EGatewayFull(mock.URL)
	gw.planner.SetLongHorizonCheckpointStore(store)
	tenant := tm.Register("planner-recovery-e2e")

	req := authedRequest("GET", "/v1/planner/checkpoints?plan_id=plan-recovery-e2e&include_snapshot=1", "", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("checkpoint list: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var listResp plannerCheckpointListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("decode checkpoint list: %v", err)
	}
	if listResp.Count != 1 || len(listResp.Checkpoints) != 1 {
		t.Fatalf("expected one recovered checkpoint, got %+v", listResp)
	}
	if !listResp.Checkpoints[0].Recoverable || len(listResp.Checkpoints[0].PlanSnapshot) != 2 {
		t.Fatalf("unexpected checkpoint summary: %+v", listResp.Checkpoints[0])
	}

	req = authedRequest("GET", "/v1/planner/execution-state?plan_id=plan-recovery-e2e", "", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("execution-state: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var state plannerExecutionStateResponse
	if err := json.Unmarshal(w.Body.Bytes(), &state); err != nil {
		t.Fatalf("decode execution state: %v", err)
	}
	if state.Checkpoint == nil || state.Checkpoint.PlanID != "plan-recovery-e2e" {
		t.Fatalf("expected checkpoint in execution state, got %+v", state)
	}
	if state.RecoveryPlan == nil || state.RecoveryPlan.Prompt == "" {
		t.Fatalf("expected recovery plan in execution state, got %+v", state)
	}
	if !strings.Contains(state.RecoveryPlan.Prompt, "Plan ID：plan-recovery-e2e") || !strings.Contains(state.RecoveryPlan.Prompt, "本次恢复范围") {
		t.Fatalf("execution state should surface recovery prompt, got:\n%s", state.RecoveryPlan.Prompt)
	}

	req = authedRequest("POST", "/v1/planner/checkpoints/recover", `{"plan_id":"plan-recovery-e2e","action":"retry_failed"}`, tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("checkpoint recover: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var recoverResp plannerCheckpointRecoverResponse
	if err := json.Unmarshal(w.Body.Bytes(), &recoverResp); err != nil {
		t.Fatalf("decode recover response: %v", err)
	}
	if recoverResp.RecoveryPlan.Mode != "retry_failed" || !recoverResp.RecoveryPlan.Executable {
		t.Fatalf("unexpected recovery plan: %+v", recoverResp.RecoveryPlan)
	}
	if len(recoverResp.RecoveryPlan.Steps) != 2 || recoverResp.RecoveryPlan.Steps[1].Selected != true || recoverResp.RecoveryPlan.Steps[0].Selected != false {
		t.Fatalf("expected only failed step selected, got %+v", recoverResp.RecoveryPlan.Steps)
	}
	if !strings.Contains(recoverResp.Prompt, "本次恢复范围：继续 1 个步骤") || !strings.Contains(recoverResp.Prompt, "Plan ID：plan-recovery-e2e") {
		t.Fatalf("recover prompt should reflect persisted checkpoint, got:\n%s", recoverResp.Prompt)
	}

	req = authedRequest("POST", "/v1/planner/checkpoints/resume-plan", `{"plan_id":"plan-recovery-e2e","action":"partial"}`, tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("partial resume-plan: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resumeResp plannerCheckpointResumePlanResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resumeResp); err != nil {
		t.Fatalf("decode resume-plan response: %v", err)
	}
	if resumeResp.Status != "completed" || resumeResp.Result == nil {
		t.Fatalf("unexpected partial resume response: %+v", resumeResp)
	}
	if !strings.Contains(resumeResp.Result.Reply, "阶段结果") || !strings.Contains(resumeResp.Result.Reply, "[Parsed document: 技术蓝图.md]") {
		t.Fatalf("expected partial resume to return preserved stage result, got:\n%s", resumeResp.Result.Reply)
	}
}
