package gateway

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"yunque-agent/internal/agentcore/planner"
	"yunque-agent/internal/agentcore/task"
	"yunque-agent/internal/apperror"
	"yunque-agent/internal/observe"
	"yunque-agent/pkg/safego"
)

type plannerCheckpointListResponse struct {
	Checkpoints []plannerCheckpointSummary `json:"checkpoints"`
	Limit       int                        `json:"limit"`
	Count       int                        `json:"count"`
}

type plannerCheckpointSummary struct {
	PlanID       string             `json:"plan_id"`
	TaskID       string             `json:"task_id,omitempty"`
	Goal         string             `json:"goal,omitempty"`
	Status       string             `json:"status"`
	CurrentStep  int                `json:"current_step"`
	Completed    int                `json:"completed"`
	Total        int                `json:"total"`
	StepsUsed    int                `json:"steps_used"`
	Revisions    int                `json:"revisions"`
	Error        string             `json:"error,omitempty"`
	Recoverable  bool               `json:"recoverable"`
	ResumeHint   string             `json:"resume_hint,omitempty"`
	UpdatedAt    string             `json:"updated_at,omitempty"`
	PlanSnapshot []planner.PlanStep `json:"plan_snapshot,omitempty"`
}

type plannerCheckpointRecoverRequest struct {
	PlanID string `json:"plan_id"`
	Action string `json:"action"`
}

type plannerCheckpointResumeTaskRequest struct {
	PlanID string `json:"plan_id"`
	Action string `json:"action"`
	Run    *bool  `json:"run,omitempty"`
}

type plannerCheckpointResumePlanRequest struct {
	PlanID string `json:"plan_id"`
	Action string `json:"action"`
	Async  bool   `json:"async,omitempty"`
}

type plannerCheckpointRecoverResponse struct {
	Action       string                   `json:"action"`
	PlanID       string                   `json:"plan_id"`
	TaskID       string                   `json:"task_id,omitempty"`
	Prompt       string                   `json:"prompt"`
	RecoveryPlan plannerCheckpointPlan    `json:"recovery_plan"`
	Checkpoint   plannerCheckpointSummary `json:"checkpoint"`
}

type plannerCheckpointResumeTaskResponse struct {
	Status       string                   `json:"status"`
	TaskID       string                   `json:"task_id"`
	Run          bool                     `json:"run"`
	RecoveryPlan plannerCheckpointPlan    `json:"recovery_plan"`
	Checkpoint   plannerCheckpointSummary `json:"checkpoint"`
}

type plannerCheckpointResumePlanResponse struct {
	Status        string                   `json:"status"`
	Action        string                   `json:"action"`
	PlanID        string                   `json:"plan_id"`
	JobID         string                   `json:"job_id,omitempty"`
	FriendlyError string                   `json:"friendly_error,omitempty"`
	Recoverable   bool                     `json:"recoverable,omitempty"`
	NextAction    string                   `json:"next_action,omitempty"`
	Result        *planner.PlanResult      `json:"result"`
	RecoveryPlan  plannerCheckpointPlan    `json:"recovery_plan"`
	Checkpoint    plannerCheckpointSummary `json:"checkpoint"`
}

type plannerCheckpointResumePlanJob struct {
	ID            string                                `json:"id"`
	Status        string                                `json:"status"`
	Action        string                                `json:"action"`
	TenantID      string                                `json:"tenant_id,omitempty"`
	PlanID        string                                `json:"plan_id"`
	TaskID        string                                `json:"task_id,omitempty"`
	Error         string                                `json:"error,omitempty"`
	FriendlyError string                                `json:"friendly_error,omitempty"`
	Recoverable   bool                                  `json:"recoverable,omitempty"`
	NextAction    string                                `json:"next_action,omitempty"`
	Result        *planner.PlanResult                   `json:"result,omitempty"`
	Events        []plannerCheckpointResumePlanJobEvent `json:"events,omitempty"`
	StartedAt     string                                `json:"started_at"`
	FinishedAt    string                                `json:"finished_at,omitempty"`
}

type plannerCheckpointResumePlanJobEvent struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Summary   string `json:"summary"`
	Skill     string `json:"skill,omitempty"`
	Timestamp string `json:"timestamp"`
}

type plannerCheckpointResumePlanJobResponse struct {
	Job plannerCheckpointResumePlanJob `json:"job"`
}

type plannerExecutionStateResponse struct {
	PlanID         string                                `json:"plan_id"`
	Status         string                                `json:"status"`
	Action         string                                `json:"action"`
	NextAction     string                                `json:"next_action,omitempty"`
	UpdatedAt      string                                `json:"updated_at,omitempty"`
	Checkpoint     *plannerCheckpointSummary             `json:"checkpoint,omitempty"`
	LatestJob      *plannerCheckpointResumePlanJob       `json:"latest_job,omitempty"`
	RecoveryPlan   *plannerCheckpointPlan                `json:"recovery_plan,omitempty"`
	FailureSummary *plannerExecutionStateFailureSummary  `json:"failure_summary,omitempty"`
	Cogni          *plannerExecutionStateCogniSummary    `json:"cogni,omitempty"`
	Events         []plannerCheckpointResumePlanJobEvent `json:"events,omitempty"`
}

type plannerExecutionStateFailureSummary struct {
	FailedCount    int      `json:"failed_count"`
	CompletedCount int      `json:"completed_count"`
	FailedTools    []string `json:"failed_tools,omitempty"`
	Tried          []string `json:"tried,omitempty"`
	RuledOut       []string `json:"ruled_out,omitempty"`
	FailurePattern string   `json:"failure_pattern,omitempty"`
	Recommendation string   `json:"recommendation,omitempty"`
	NextStep       string   `json:"next_step,omitempty"`
}

type plannerExecutionStateCogniSummary struct {
	Activated    []string                              `json:"activated,omitempty"`
	ContextBytes int                                   `json:"context_bytes,omitempty"`
	ToolBefore   int                                   `json:"tool_before,omitempty"`
	ToolAfter    int                                   `json:"tool_after,omitempty"`
	Removed      []string                              `json:"removed,omitempty"`
	LastSummary  string                                `json:"last_summary,omitempty"`
	EventCount   int                                   `json:"event_count"`
	Events       []plannerCheckpointResumePlanJobEvent `json:"events,omitempty"`
}

type plannerCheckpointPlan struct {
	Mode       string                      `json:"mode"`
	Executable bool                        `json:"executable"`
	Reason     string                      `json:"reason,omitempty"`
	PlanID     string                      `json:"plan_id"`
	TaskID     string                      `json:"task_id,omitempty"`
	Steps      []plannerCheckpointPlanStep `json:"steps"`
	Prompt     string                      `json:"prompt"`
}

type plannerCheckpointPlanStep struct {
	ID        int    `json:"id"`
	Action    string `json:"action"`
	Skill     string `json:"skill,omitempty"`
	Status    string `json:"status"`
	DependsOn []int  `json:"depends_on,omitempty"`
	Selected  bool   `json:"selected"`
	Reason    string `json:"reason,omitempty"`
}

// handlePlannerCheckpoints lists recent long-horizon checkpoints.
//
// GET /v1/planner/checkpoints?limit=20
// GET /v1/planner/checkpoints?limit=20&include_snapshot=1
// GET /v1/planner/checkpoints?plan_id=...&include_snapshot=1
//
// By default the response is a compact, UI-safe list for "最近可恢复任务".
// Detailed plan snapshots are opt-in to avoid pushing large tool outputs into
// every page load.
func (g *Gateway) handlePlannerCheckpoints(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET only")
		return
	}
	if g.planner == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "planner not available")
		return
	}

	limit := parseCheckpointLimit(r.URL.Query().Get("limit"))
	includeSnapshot := parseBoolQuery(r.URL.Query().Get("include_snapshot"))
	planID := strings.TrimSpace(r.URL.Query().Get("plan_id"))
	if planID != "" {
		cp, ok, err := g.findPlannerCheckpoint(r, planID)
		if err != nil {
			apperror.WriteCode(w, apperror.CodeInternal, "failed to read planner checkpoints")
			return
		}
		out := make([]plannerCheckpointSummary, 0, 1)
		if ok {
			out = append(out, summarizePlannerCheckpoint(cp, includeSnapshot))
		}
		writeJSON(w, plannerCheckpointListResponse{
			Checkpoints: out,
			Limit:       1,
			Count:       len(out),
		})
		return
	}
	checkpoints, err := g.planner.RecentLongHorizonCheckpointsForTenant(r.Context(), tenantFromCtx(r.Context()), limit)
	if err != nil {
		apperror.WriteCode(w, apperror.CodeInternal, "failed to read planner checkpoints")
		return
	}

	out := make([]plannerCheckpointSummary, 0, len(checkpoints))
	for _, cp := range checkpoints {
		item := summarizePlannerCheckpoint(cp, includeSnapshot)
		out = append(out, item)
	}
	writeJSON(w, plannerCheckpointListResponse{
		Checkpoints: out,
		Limit:       limit,
		Count:       len(out),
	})
}

// handlePlannerCheckpointRecover turns an explicit recovery action into a
// backend-owned prompt. The frontend should call this instead of hand-crafting
// prompts so recovery semantics stay versioned with the planner.
//
// POST /v1/planner/checkpoints/recover
// { "plan_id": "...", "action": "continue|retry_failed|partial" }
func (g *Gateway) handlePlannerCheckpointRecover(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	if g.planner == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "planner not available")
		return
	}
	var req plannerCheckpointRecoverRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON")
		return
	}
	req.PlanID = strings.TrimSpace(req.PlanID)
	req.Action = normalizeCheckpointAction(req.Action)
	if req.PlanID == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "plan_id is required")
		return
	}
	if req.Action == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "unsupported recovery action")
		return
	}

	cp, ok, err := g.findPlannerCheckpoint(r, req.PlanID)
	if err != nil {
		apperror.WriteCode(w, apperror.CodeInternal, "failed to read planner checkpoints")
		return
	}
	if !ok {
		apperror.WriteCode(w, apperror.CodeNotFound, "planner checkpoint not found")
		return
	}
	summary := summarizePlannerCheckpoint(cp, true)
	prompt := buildPlannerRecoveryPrompt(cp, req.Action)
	recoveryPlan := buildPlannerCheckpointPlan(cp, req.Action, prompt)
	writeJSON(w, plannerCheckpointRecoverResponse{
		Action:       req.Action,
		PlanID:       cp.PlanID,
		TaskID:       cp.TaskID,
		Prompt:       prompt,
		RecoveryPlan: recoveryPlan,
		Checkpoint:   summary,
	})
}

// handlePlannerCheckpointResumeTask creates a first-class task from a
// checkpoint recovery plan. Unlike the recover endpoint, this is task-state
// based: only selected pending/failed steps become task steps, completed steps
// remain reference context inside the task description.
//
// POST /v1/planner/checkpoints/resume
// { "plan_id": "...", "action": "continue|retry_failed|partial", "run": true }
func (g *Gateway) handlePlannerCheckpointResumeTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	if g.planner == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "planner not available")
		return
	}
	if g.taskStore == nil || g.taskRunner == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "task runtime not available")
		return
	}
	var req plannerCheckpointResumeTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON")
		return
	}
	req.PlanID = strings.TrimSpace(req.PlanID)
	req.Action = normalizeCheckpointAction(req.Action)
	if req.PlanID == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "plan_id is required")
		return
	}
	if req.Action == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "unsupported recovery action")
		return
	}

	cp, ok, err := g.findPlannerCheckpoint(r, req.PlanID)
	if err != nil {
		apperror.WriteCode(w, apperror.CodeInternal, "failed to read planner checkpoints")
		return
	}
	if !ok {
		apperror.WriteCode(w, apperror.CodeNotFound, "planner checkpoint not found")
		return
	}
	prompt := buildPlannerRecoveryPrompt(cp, req.Action)
	recoveryPlan := buildPlannerCheckpointPlan(cp, req.Action, prompt)
	if req.Action != "partial" && !recoveryPlan.Executable {
		apperror.WriteCode(w, apperror.CodeBadRequest, recoveryPlan.Reason)
		return
	}

	shouldRun := true
	if req.Run != nil {
		shouldRun = *req.Run
	}
	taskTitle := fmt.Sprintf("恢复规划 %s", cp.PlanID)
	if cp.Goal != "" {
		taskTitle = "恢复：" + truncateStr(cp.Goal, 42)
	}
	t, err := g.taskStore.Create(task.CreateRequest{
		Title:       taskTitle,
		Description: prompt,
		TenantID:    tenantFromCtx(r.Context()),
		Constraints: &task.TaskConstraints{
			MaxSteps:        maxInt(1, len(recoveryPlan.Steps)),
			TimeoutSec:      300,
			SuccessCriteria: "不重复已完成步骤，完成本次恢复范围内的步骤并返回可验证阶段结果。",
			Priority:        "high",
			RiskLevel:       task.RiskMedium,
			AutoApprove:     true,
			Tags:            []string{"planner-recovery", req.Action},
			Extra: map[string]any{
				"source":        "planner_checkpoint",
				"plan_id":       cp.PlanID,
				"task_id":       cp.TaskID,
				"action":        req.Action,
				"recovery_plan": recoveryPlan,
			},
		},
	})
	if err != nil {
		apperror.WriteCode(w, apperror.CodeInternal, "failed to create recovery task")
		return
	}
	t.Steps = checkpointTaskSteps(cp, recoveryPlan)
	if err := g.taskStore.Update(t); err != nil {
		apperror.WriteCode(w, apperror.CodeInternal, "failed to save recovery task")
		return
	}
	if shouldRun {
		taskID := t.ID
		safego.Go("planner-checkpoint-resume-"+taskID, func() {
			if err := g.taskRunner.Run(context.Background(), taskID); err != nil {
				slog.Warn("planner checkpoint resume task failed", "task", taskID, "plan", cp.PlanID, "err", err)
			}
		})
		w.WriteHeader(http.StatusAccepted)
	} else {
		w.WriteHeader(http.StatusCreated)
	}
	writeJSON(w, plannerCheckpointResumeTaskResponse{
		Status:       mapBool(shouldRun, "accepted", "created"),
		TaskID:       t.ID,
		Run:          shouldRun,
		RecoveryPlan: recoveryPlan,
		Checkpoint:   summarizePlannerCheckpoint(cp, true),
	})
}

// handlePlannerCheckpointResumePlan executes a checkpoint recovery through the
// planner DAG runner. It is intentionally separate from /resume, which creates
// a first-class Task; this endpoint is for callers that explicitly want
// plan-level execution without translating steps into task steps first. For UI
// calls, async=true avoids holding the HTTP request open during a long resume.
//
// POST /v1/planner/checkpoints/resume-plan
// { "plan_id": "...", "action": "continue|retry_failed|partial", "async": true }
func (g *Gateway) handlePlannerCheckpointResumePlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	if g.planner == nil {
		apperror.WriteCode(w, apperror.CodeLLMUnavailable, "planner unavailable")
		return
	}
	var req plannerCheckpointResumePlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON")
		return
	}
	req.PlanID = strings.TrimSpace(req.PlanID)
	req.Action = normalizeCheckpointAction(req.Action)
	if req.PlanID == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "plan_id required")
		return
	}
	if req.Action == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "unsupported recovery action")
		return
	}
	cp, ok, err := g.findPlannerCheckpoint(r, req.PlanID)
	if err != nil {
		apperror.WriteCode(w, apperror.CodeInternal, "failed to read planner checkpoints")
		return
	}
	if !ok {
		apperror.WriteCode(w, apperror.CodeNotFound, "planner checkpoint not found")
		return
	}
	prompt := buildPlannerRecoveryPrompt(cp, req.Action)
	recoveryPlan := buildPlannerCheckpointPlan(cp, req.Action, prompt)
	if !recoveryPlan.Executable && req.Action != "partial" {
		apperror.WriteCode(w, apperror.CodeBadRequest, recoveryPlan.Reason)
		return
	}
	if req.Async {
		tenantID := tenantFromCtx(r.Context())
		job, reused := g.reservePlannerResumeJob(cp.PlanID, cp.TaskID, tenantID, req.Action)
		if !reused {
			safego.Go("planner-resume-plan-"+job.ID, func() {
				defer func() {
					if rec := recover(); rec != nil {
						job.Status = "failed"
						job.Error = fmt.Sprintf("planner resume interrupted: %v", rec)
						applyPlannerResumeJobFailureAdvice(&job, recoveryPlan)
						job.FinishedAt = time.Now().UTC().Format(time.RFC3339)
						g.appendAndBroadcastPlannerResumeJobEvent(&job, plannerResumeJobTerminalEvent(job))
					}
				}()
				result, err := g.planner.ResumeLongHorizonCheckpoint(context.Background(), planner.PlanRequest{
					TenantID: tenantID,
					TaskID:   cp.TaskID,
					TraceID:  job.ID,
					StepCallback: func(evt observe.AgentEvent) {
						g.appendAndBroadcastPlannerResumeJobEvent(&job, plannerResumeJobEventFromAgent(evt))
					},
				}, cp, req.Action)
				finished := time.Now().UTC().Format(time.RFC3339)
				if err != nil {
					job.Status = "failed"
					job.Error = err.Error()
					applyPlannerResumeJobFailureAdvice(&job, recoveryPlan)
				} else if req.Action != "partial" && plannerResumePlanResultFailed(result) {
					job.Status = "failed"
					job.Result = result
					job.Error = plannerResumePlanResultError(result)
					applyPlannerResumeJobFailureAdvice(&job, recoveryPlan)
				} else {
					job.Status = "completed"
					job.Result = result
				}
				job.FinishedAt = finished
				g.appendAndBroadcastPlannerResumeJobEvent(&job, plannerResumeJobTerminalEvent(job))
			})
		}
		w.WriteHeader(http.StatusAccepted)
		writeJSON(w, plannerCheckpointResumePlanResponse{
			Status:       "accepted",
			Action:       req.Action,
			PlanID:       cp.PlanID,
			JobID:        job.ID,
			RecoveryPlan: recoveryPlan,
			Checkpoint:   summarizePlannerCheckpoint(cp, true),
		})
		return
	}
	result, err := g.planner.ResumeLongHorizonCheckpoint(r.Context(), planner.PlanRequest{
		TenantID: tenantFromCtx(r.Context()),
		TaskID:   cp.TaskID,
	}, cp, req.Action)
	if err != nil {
		friendly, _ := plannerResumeFailureAdvice(err.Error(), recoveryPlan)
		apperror.WriteCode(w, apperror.CodeBadRequest, friendly)
		return
	}
	status := "completed"
	friendlyError := ""
	nextAction := ""
	recoverable := false
	if req.Action != "partial" && plannerResumePlanResultFailed(result) {
		status = "failed"
		friendlyError, nextAction = plannerResumeFailureAdvice(plannerResumePlanResultError(result), recoveryPlan)
		recoverable = true
	}
	writeJSON(w, plannerCheckpointResumePlanResponse{
		Status:        status,
		Action:        req.Action,
		PlanID:        cp.PlanID,
		FriendlyError: friendlyError,
		Recoverable:   recoverable,
		NextAction:    nextAction,
		Result:        sanitizePlannerPlanResult(result),
		RecoveryPlan:  recoveryPlan,
		Checkpoint:    summarizePlannerCheckpoint(cp, true),
	})
}

func (g *Gateway) handlePlannerCheckpointResumePlanJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET only")
		return
	}
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		id = strings.TrimSpace(r.URL.Query().Get("job_id"))
	}
	planID := strings.TrimSpace(r.URL.Query().Get("plan_id"))
	tenantID := tenantFromCtx(r.Context())
	var job plannerCheckpointResumePlanJob
	var ok bool
	if id != "" {
		job, ok = g.getPlannerResumeJob(id, tenantID)
	} else if planID != "" {
		job, ok = g.getLatestPlannerResumeJobForPlan(planID, tenantID)
	} else {
		apperror.WriteCode(w, apperror.CodeBadRequest, "id or plan_id required")
		return
	}
	if !ok {
		apperror.WriteCode(w, apperror.CodeNotFound, "planner resume job not found")
		return
	}
	writeJSON(w, plannerCheckpointResumePlanJobResponse{Job: sanitizePlannerResumeJobForResponse(job)})
}

// handlePlannerExecutionState provides one normalized view over the currently
// recoverable planner checkpoint, the latest direct-resume job, the recovery
// plan, and a compact failure summary. It is intentionally read-only so UI
// surfaces can hydrate the full "execution scene" after refresh or reconnect
// without stitching multiple endpoints together.
//
// GET /v1/planner/execution-state?plan_id=...&action=continue|retry_failed|partial
func (g *Gateway) handlePlannerExecutionState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET only")
		return
	}
	if g.planner == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "planner not available")
		return
	}
	planID := strings.TrimSpace(r.URL.Query().Get("plan_id"))
	if planID == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "plan_id required")
		return
	}
	cp, ok, err := g.findPlannerCheckpoint(r, planID)
	if err != nil {
		apperror.WriteCode(w, apperror.CodeInternal, "failed to read planner checkpoints")
		return
	}
	if !ok {
		apperror.WriteCode(w, apperror.CodeNotFound, "planner checkpoint not found")
		return
	}
	summary := summarizePlannerCheckpoint(cp, true)
	action := ""
	if rawAction := strings.TrimSpace(r.URL.Query().Get("action")); rawAction != "" {
		action = normalizeCheckpointAction(rawAction)
	}
	var latestJob *plannerCheckpointResumePlanJob
	if job, ok := g.getLatestPlannerResumeJobForPlan(planID, tenantFromCtx(r.Context())); ok {
		job = sanitizePlannerResumeJobForExecutionState(job)
		latestJob = &job
		if action == "" {
			action = normalizeCheckpointAction(job.Action)
		}
	}
	if action == "" {
		action = defaultPlannerExecutionStateAction(summary, latestJob)
	}
	recoveryPlan := buildPlannerCheckpointPlan(cp, action, buildPlannerRecoveryPrompt(cp, action))
	failureSummary := buildPlannerExecutionStateFailureSummary(summary.PlanSnapshot, latestJob)
	status := summary.Status
	nextAction := ""
	updatedAt := summary.UpdatedAt
	events := []plannerCheckpointResumePlanJobEvent(nil)
	if latestJob != nil {
		status = latestJob.Status
		nextAction = latestJob.NextAction
		if latestJob.FinishedAt != "" {
			updatedAt = latestJob.FinishedAt
		} else if latestJob.StartedAt != "" {
			updatedAt = latestJob.StartedAt
		}
		events = latestJob.Events
	}
	if nextAction == "" && failureSummary != nil && failureSummary.FailedCount > 0 {
		nextAction = "retry_failed"
	}
	cogniSummary := g.buildPlannerExecutionStateCogniSummary(cp, latestJob)
	writeJSON(w, plannerExecutionStateResponse{
		PlanID:         planID,
		Status:         status,
		Action:         action,
		NextAction:     nextAction,
		UpdatedAt:      updatedAt,
		Checkpoint:     &summary,
		LatestJob:      latestJob,
		RecoveryPlan:   &recoveryPlan,
		FailureSummary: failureSummary,
		Cogni:          cogniSummary,
		Events:         events,
	})
}

func (g *Gateway) buildPlannerExecutionStateCogniSummary(cp planner.LongHorizonCheckpoint, latestJob *plannerCheckpointResumePlanJob) *plannerExecutionStateCogniSummary {
	if g == nil || g.eventTrail == nil {
		return nil
	}
	taskID := strings.TrimSpace(cp.TaskID)
	if taskID == "" && latestJob != nil {
		taskID = strings.TrimSpace(latestJob.TaskID)
	}
	if taskID == "" {
		return nil
	}
	return summarizePlannerCogniEvents(g.eventTrail.QueryByTaskID(taskID))
}

func summarizePlannerCogniEvents(events []observe.AgentEvent) *plannerExecutionStateCogniSummary {
	if len(events) == 0 {
		return nil
	}
	summary := plannerExecutionStateCogniSummary{}
	activatedSet := map[string]bool{}
	removedSet := map[string]bool{}
	for _, evt := range events {
		detail, ok := plannerCogniTraceDetailFromEvent(evt)
		if !ok {
			continue
		}
		for _, name := range detail.Activated {
			name = strings.TrimSpace(name)
			if name != "" && !activatedSet[name] {
				activatedSet[name] = true
				summary.Activated = append(summary.Activated, name)
			}
		}
		for _, name := range detail.Removed {
			name = strings.TrimSpace(name)
			if name != "" && !removedSet[name] {
				removedSet[name] = true
				summary.Removed = append(summary.Removed, name)
			}
		}
		if detail.ContextBytes > summary.ContextBytes {
			summary.ContextBytes = detail.ContextBytes
		}
		if detail.ToolBefore > 0 || detail.ToolAfter > 0 {
			summary.ToolBefore = detail.ToolBefore
			summary.ToolAfter = detail.ToolAfter
		}
		eventSummary := truncateStr(evt.Summary, 240)
		if friendly := plannerResumeJobEventFriendlySummary(eventSummary); friendly != "" {
			eventSummary = friendly
		}
		if eventSummary == "" {
			eventSummary = "Cogni 已参与本轮规划。"
		}
		summary.LastSummary = eventSummary
		summary.Events = append(summary.Events, plannerCheckpointResumePlanJobEvent{
			ID:        evt.ID,
			Type:      evt.QualifiedType(),
			Summary:   eventSummary,
			Skill:     evt.Meta.Skill,
			Timestamp: evt.Timestamp.UTC().Format(time.RFC3339),
		})
	}
	if len(summary.Events) == 0 {
		return nil
	}
	if len(summary.Events) > 8 {
		summary.Events = append([]plannerCheckpointResumePlanJobEvent(nil), summary.Events[len(summary.Events)-8:]...)
	}
	summary.EventCount = len(summary.Events)
	return &summary
}

func plannerCogniTraceDetailFromEvent(evt observe.AgentEvent) (planner.CogniTraceDetail, bool) {
	if evt.Domain != observe.DomainPlanner {
		return planner.CogniTraceDetail{}, false
	}
	if detail, ok := plannerCogniTraceDetailFromAny(evt.Detail); ok {
		return detail, true
	}
	if strings.Contains(evt.Summary, "Cogni 已激活") {
		return planner.CogniTraceDetail{Activated: []string{"已激活"}}, true
	}
	return planner.CogniTraceDetail{}, false
}

func plannerCogniTraceDetailFromAny(v any) (planner.CogniTraceDetail, bool) {
	switch detail := v.(type) {
	case planner.CogniTraceDetail:
		if detail.ContextBytes > 0 || detail.ToolBefore > 0 || detail.ToolAfter > 0 || len(detail.Activated) > 0 || len(detail.Removed) > 0 {
			return detail, true
		}
	case *planner.CogniTraceDetail:
		if detail != nil {
			return plannerCogniTraceDetailFromAny(*detail)
		}
	case map[string]any:
		out := planner.CogniTraceDetail{
			Activated:    plannerStringSliceFromAny(detail["activated"]),
			ContextBytes: plannerIntFromAny(detail["context_bytes"]),
			ToolBefore:   plannerIntFromAny(detail["tool_before"]),
			ToolAfter:    plannerIntFromAny(detail["tool_after"]),
			Removed:      plannerStringSliceFromAny(detail["removed"]),
		}
		if out.ContextBytes > 0 || out.ToolBefore > 0 || out.ToolAfter > 0 || len(out.Activated) > 0 || len(out.Removed) > 0 {
			return out, true
		}
	}
	return planner.CogniTraceDetail{}, false
}

func plannerStringSliceFromAny(v any) []string {
	switch raw := v.(type) {
	case []string:
		return append([]string(nil), raw...)
	case []any:
		out := make([]string, 0, len(raw))
		for _, item := range raw {
			if s := strings.TrimSpace(fmt.Sprint(item)); s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func plannerIntFromAny(v any) int {
	switch raw := v.(type) {
	case int:
		return raw
	case int64:
		return int(raw)
	case float64:
		return int(raw)
	case json.Number:
		n, _ := raw.Int64()
		return int(n)
	default:
		return 0
	}
}

func plannerResumeJobEventFromAgent(evt observe.AgentEvent) plannerCheckpointResumePlanJobEvent {
	return plannerCheckpointResumePlanJobEvent{
		ID:        evt.ID,
		Type:      evt.QualifiedType(),
		Summary:   plannerResumeJobEventDisplaySummary(evt.Summary),
		Skill:     evt.Meta.Skill,
		Timestamp: evt.Timestamp.UTC().Format(time.RFC3339),
	}
}

func plannerResumeJobTerminalEvent(job plannerCheckpointResumePlanJob) plannerCheckpointResumePlanJobEvent {
	timestamp := strings.TrimSpace(job.FinishedAt)
	if timestamp == "" {
		timestamp = time.Now().UTC().Format(time.RFC3339)
	}
	summary := "原规划续跑已完成。"
	if job.Status == "failed" {
		summary = strings.TrimSpace(job.FriendlyError)
		if summary == "" {
			summary = plannerResumeJobEventDisplaySummary(job.Error)
		}
		if summary == "" {
			summary = "续跑没有顺利完成，现场已保留，可重试或切换策略继续。"
		}
	}
	return plannerCheckpointResumePlanJobEvent{
		ID:        fmt.Sprintf("%s-terminal", job.ID),
		Type:      "planner.resume_plan_done",
		Summary:   plannerResumeJobEventDisplaySummary(summary),
		Timestamp: timestamp,
	}
}

func (g *Gateway) appendAndBroadcastPlannerResumeJobEvent(job *plannerCheckpointResumePlanJob, evt plannerCheckpointResumePlanJobEvent) {
	if job == nil {
		return
	}
	job.Events = appendPlannerResumeJobEvent(job.Events, evt)
	g.savePlannerResumeJob(*job)
	if g.sseBroker == nil || len(job.Events) == 0 {
		return
	}
	g.sseBroker.Broadcast(SSEEvent{
		Type:     "planner.resume_plan_event",
		TenantID: job.TenantID,
		Data:     map[string]any{"job_id": job.ID, "event": job.Events[len(job.Events)-1]},
	})
}

func plannerResumeJobEventDisplaySummary(summary string) string {
	summary = truncateStr(strings.TrimSpace(summary), 240)
	if friendly := plannerResumeJobEventFriendlySummary(summary); friendly != "" {
		return friendly
	}
	return summary
}

func plannerResumeJobEventFriendlySummary(summary string) string {
	msg := strings.TrimSpace(summary)
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "unknown skill") || strings.Contains(msg, "未知工具") || strings.Contains(msg, "未找到工具"):
		return "这一步需要的工具暂时不可用，现场已保留，可换用可用工具或调整步骤继续。"
	case strings.Contains(lower, "blocked by trust gate") || strings.Contains(lower, "trust gate") || strings.Contains(msg, "信任"):
		return "这一步需要更高信任或确认，现场已保留，可确认后继续。"
	case strings.Contains(lower, "tool panic") || strings.Contains(lower, "panic"):
		return "这一步的工具运行时遇到异常，现场已保留，可重试或切换策略继续。"
	case strings.Contains(lower, "context deadline exceeded") || strings.Contains(lower, "timeout") || strings.Contains(msg, "响应超时"):
		return "这一步等待时间过长，现场已保留，可稍后重试或先返回阶段结果。"
	case strings.Contains(lower, "execution failed") || strings.Contains(lower, "handoff agent") || strings.Contains(lower, "fallback") || strings.Contains(lower, "all fallback llm clients failed") || strings.Contains(lower, "eof"):
		return "这一步暂时没有顺利完成，现场已保留，可切换策略继续。"
	default:
		return ""
	}
}

func appendPlannerResumeJobEvent(events []plannerCheckpointResumePlanJobEvent, evt plannerCheckpointResumePlanJobEvent) []plannerCheckpointResumePlanJobEvent {
	if strings.TrimSpace(evt.ID) == "" && strings.TrimSpace(evt.Summary) == "" {
		return events
	}
	events = append(events, evt)
	const maxResumePlanJobEvents = 80
	if len(events) > maxResumePlanJobEvents {
		events = append([]plannerCheckpointResumePlanJobEvent(nil), events[len(events)-maxResumePlanJobEvents:]...)
	}
	return events
}

func applyPlannerResumeJobFailureAdvice(job *plannerCheckpointResumePlanJob, recoveryPlan plannerCheckpointPlan) {
	if job == nil {
		return
	}
	friendly, nextAction := plannerResumeFailureAdvice(job.Error, recoveryPlan)
	job.FriendlyError = friendly
	job.NextAction = nextAction
	job.Recoverable = true
}

func plannerResumePlanResultFailed(result *planner.PlanResult) bool {
	if result == nil {
		return false
	}
	for _, step := range result.Plan {
		if step.Status == planner.StepFailed || strings.TrimSpace(step.Error) != "" {
			return true
		}
	}
	return false
}

func plannerResumePlanResultError(result *planner.PlanResult) string {
	if result == nil {
		return ""
	}
	for _, step := range result.Plan {
		if strings.TrimSpace(step.Error) != "" {
			return step.Error
		}
	}
	if strings.TrimSpace(result.Reply) != "" {
		return result.Reply
	}
	return "planner resume did not complete all steps"
}

func plannerResumeFailureAdvice(raw string, recoveryPlan plannerCheckpointPlan) (friendly, nextAction string) {
	message := strings.TrimSpace(raw)
	lower := strings.ToLower(message)
	switch {
	case strings.Contains(message, "尚未完成") || strings.Contains(message, "依赖") || strings.Contains(lower, "dependency"):
		return "前置步骤还没有完全确认，建议先查看依赖关系，或转为后台恢复任务继续推进。", "inspect_dependencies"
	case strings.Contains(lower, "context deadline exceeded") || strings.Contains(message, "响应超时") || strings.Contains(lower, "timeout"):
		return "这次续跑等待时间过长，现场已经保留；建议重试失败步骤，或先返回阶段结果。", "retry_failed"
	case strings.Contains(lower, "execution failed") || strings.Contains(lower, "handoff agent") || strings.Contains(lower, "fallback") || strings.Contains(lower, "all fallback llm clients failed") || strings.Contains(lower, "eof"):
		return "这次续跑没有顺利完成，现场已经保留；建议转为后台恢复任务，或先拿到阶段结果。", "create_task"
	}
	if !recoveryPlan.Executable && strings.TrimSpace(recoveryPlan.Reason) != "" {
		return "当前规划需要先确认恢复范围，建议查看依赖关系或先返回阶段结果。", "inspect_dependencies"
	}
	return "这次续跑没有顺利完成，现场已经保留；可以重试失败步骤、转为后台任务，或先返回阶段结果。", "retry_failed"
}

func defaultPlannerExecutionStateAction(cp plannerCheckpointSummary, latestJob *plannerCheckpointResumePlanJob) string {
	if latestJob != nil {
		if action := normalizeCheckpointAction(latestJob.Action); action != "" {
			return action
		}
	}
	for _, step := range cp.PlanSnapshot {
		if step.Status == planner.StepFailed {
			return "retry_failed"
		}
	}
	if cp.Status == "failed" || cp.Error != "" {
		return "retry_failed"
	}
	return "continue"
}

func buildPlannerExecutionStateFailureSummary(snapshot []planner.PlanStep, latestJob *plannerCheckpointResumePlanJob) *plannerExecutionStateFailureSummary {
	steps := snapshot
	if latestJob != nil && latestJob.Result != nil && len(latestJob.Result.Plan) > 0 {
		steps = latestJob.Result.Plan
	}
	if len(steps) == 0 {
		return nil
	}
	summary := &plannerExecutionStateFailureSummary{}
	seenFailedTools := map[string]bool{}
	failureErrors := []string{}
	for _, step := range steps {
		label := strings.TrimSpace(step.Skill)
		if label == "" {
			label = strings.TrimSpace(step.Action)
		}
		if label == "" {
			label = fmt.Sprintf("step-%d", step.ID)
		}
		switch step.Status {
		case planner.StepDone, planner.StepSkipped:
			summary.CompletedCount++
			if step.Result != "" {
				summary.Tried = append(summary.Tried, fmt.Sprintf("%s: %s", label, truncateStr(step.Result, 120)))
			} else {
				summary.Tried = append(summary.Tried, fmt.Sprintf("%s: 已完成", label))
			}
		case planner.StepFailed:
			summary.FailedCount++
			failureErrors = append(failureErrors, step.Error)
			if !seenFailedTools[label] {
				summary.FailedTools = append(summary.FailedTools, label)
				seenFailedTools[label] = true
			}
			errText := plannerCheckpointDisplayError(step.Error)
			if errText == "" {
				errText = "这一步没有顺利完成，现场已保留。"
			}
			summary.RuledOut = append(summary.RuledOut, fmt.Sprintf("%s: %s", label, truncateStr(errText, 140)))
		}
	}
	if summary.FailedCount == 0 && summary.CompletedCount == 0 {
		return nil
	}
	if summary.FailedCount > 0 {
		summary.FailurePattern, summary.Recommendation = plannerCheckpointFailureAnalysis(failureErrors, summary.FailedTools)
	}
	if latestJob != nil && latestJob.NextAction != "" {
		switch latestJob.NextAction {
		case "inspect_dependencies":
			summary.NextStep = "先检查依赖关系，再选择重试失败步骤或转为后台任务。"
		case "create_task":
			summary.NextStep = "转为后台恢复任务，保留现场并继续推进。"
		default:
			summary.NextStep = "重试失败步骤，或先返回阶段结果。"
		}
	} else if summary.FailedCount > 0 {
		summary.NextStep = summary.Recommendation
		if summary.NextStep == "" {
			summary.NextStep = "停止重复失败路径，重试失败步骤、转为后台任务，或先返回阶段结果。"
		}
	} else {
		summary.NextStep = "当前没有失败步骤，可按原规划继续或整理阶段结果。"
	}
	return summary
}

func plannerCheckpointFailureAnalysis(errors []string, failedTools []string) (pattern, recommendation string) {
	buckets := map[string]int{}
	for _, raw := range errors {
		buckets[plannerCheckpointFailureBucket(raw)]++
	}
	if len(buckets) == 0 {
		return "", ""
	}
	dominant := ""
	dominantCount := -1
	for bucket, count := range buckets {
		if count > dominantCount || (count == dominantCount && plannerCheckpointFailureBucketPriority(bucket) < plannerCheckpointFailureBucketPriority(dominant)) {
			dominant = bucket
			dominantCount = count
		}
	}
	toolHint := ""
	if len(failedTools) > 0 {
		toolHint = "，暂不重复使用 " + strings.Join(failedTools, "、")
	}
	switch dominant {
	case "tool":
		return "所需工具不可用或不在当前工具范围", "改用当前可用工具，或先请求开放/替换工具后再继续。"
	case "trust":
		return "需要用户确认或更高信任", "暂停自动推进，向用户说明需要确认的动作，确认后再继续。"
	case "dependency":
		return "规划依赖未满足", "回到最早未完成的前置步骤，先补齐依赖，再执行后续步骤。"
	case "runtime":
		return "工具运行异常", "降低输入规模或切换等价工具；如果已有证据足够，先返回阶段结果。"
	case "timeout":
		return "模型或子任务响应不稳定", "先返回阶段结果或切为后台任务；继续时降低任务粒度" + toolHint + "。"
	default:
		return "重复失败路径", "停止重复失败路径，换一个工具、降低任务粒度，或先汇总已获得证据再继续。"
	}
}

func plannerCheckpointFailureBucketPriority(bucket string) int {
	switch bucket {
	case "timeout":
		return 0
	case "tool":
		return 1
	case "trust":
		return 2
	case "dependency":
		return 3
	case "runtime":
		return 4
	default:
		return 9
	}
}

func plannerCheckpointFailureBucket(raw string) string {
	lower := strings.ToLower(strings.TrimSpace(raw))
	switch {
	case strings.Contains(lower, "unknown skill"), strings.Contains(lower, "allowed tool surface"):
		return "tool"
	case strings.Contains(lower, "blocked by trust gate"), strings.Contains(lower, "trust gate"):
		return "trust"
	case strings.Contains(lower, "dependency"), strings.Contains(lower, "depend"), strings.Contains(lower, "no ready steps"), strings.Contains(raw, "依赖"):
		return "dependency"
	case strings.Contains(lower, "tool panic"), strings.Contains(lower, "panic"):
		return "runtime"
	case strings.Contains(lower, "context deadline exceeded"),
		strings.Contains(lower, "deadline exceeded"),
		strings.Contains(lower, "context canceled"),
		strings.Contains(lower, "context cancelled"),
		strings.Contains(lower, "timeout"),
		strings.Contains(lower, "timed out"),
		strings.Contains(lower, "handoff agent"),
		strings.Contains(lower, "execution failed"),
		strings.Contains(lower, "all fallback"),
		strings.Contains(lower, "fallback llm"),
		strings.Contains(lower, "eof"),
		strings.Contains(raw, "响应超时"),
		strings.Contains(raw, "超时"):
		return "timeout"
	default:
		return "repeated"
	}
}

func sanitizePlannerResumeJobForResponse(job plannerCheckpointResumePlanJob) plannerCheckpointResumePlanJob {
	job.TenantID = ""
	job.Error = plannerCheckpointDisplayError(job.Error)
	job.FriendlyError = plannerCheckpointDisplayError(job.FriendlyError)
	if job.Result != nil {
		job.Result = sanitizePlannerPlanResult(job.Result)
	}
	for i := range job.Events {
		job.Events[i].Summary = plannerResumeJobEventDisplaySummary(job.Events[i].Summary)
	}
	return job
}

func sanitizePlannerResumeJobForExecutionState(job plannerCheckpointResumePlanJob) plannerCheckpointResumePlanJob {
	return sanitizePlannerResumeJobForResponse(job)
}

func sanitizePlannerPlanResult(result *planner.PlanResult) *planner.PlanResult {
	if result == nil {
		return nil
	}
	clone := *result
	if len(result.Plan) > 0 {
		clone.Plan = summarizePlanSnapshot(result.Plan)
	}
	return &clone
}

// SetPlannerResumeJobStore persists async resume-plan job snapshots so the
// detail page can still show the last known outcome after a sidecar restart.
func (g *Gateway) SetPlannerResumeJobStore(path string) {
	g.plannerResumeJobsMu.Lock()
	g.plannerResumeJobsPath = strings.TrimSpace(path)
	g.plannerResumeJobsMu.Unlock()
	if err := g.loadPlannerResumeJobs(); err != nil {
		slog.Warn("planner resume job store load failed", "err", err)
	}
}

func (g *Gateway) savePlannerResumeJob(job plannerCheckpointResumePlanJob) {
	g.plannerResumeJobsMu.Lock()
	if g.plannerResumeJobs == nil {
		g.plannerResumeJobs = make(map[string]plannerCheckpointResumePlanJob)
	}
	g.plannerResumeJobs[job.ID] = job
	path := g.plannerResumeJobsPath
	g.plannerResumeJobsMu.Unlock()
	if path != "" {
		if err := appendPlannerResumeJob(path, job); err != nil {
			slog.Warn("planner resume job store append failed", "job", job.ID, "err", err)
		}
	}
}

func (g *Gateway) reservePlannerResumeJob(planID, taskID, tenantID, action string) (plannerCheckpointResumePlanJob, bool) {
	planID = strings.TrimSpace(planID)
	tenantID = strings.TrimSpace(tenantID)
	action = normalizeCheckpointAction(action)
	if err := g.loadPlannerResumeJobs(); err != nil {
		slog.Warn("planner resume job store reload failed", "err", err)
	}
	now := time.Now().UTC()
	job := plannerCheckpointResumePlanJob{
		ID:        fmt.Sprintf("resume-plan-%d", now.UnixNano()),
		Status:    "running",
		Action:    action,
		TenantID:  tenantID,
		PlanID:    planID,
		TaskID:    strings.TrimSpace(taskID),
		StartedAt: now.Format(time.RFC3339),
	}
	g.plannerResumeJobsMu.Lock()
	if g.plannerResumeJobs == nil {
		g.plannerResumeJobs = make(map[string]plannerCheckpointResumePlanJob)
	}
	for _, existing := range g.plannerResumeJobs {
		if plannerResumeJobTenantMatches(existing, tenantID) && existing.PlanID == planID && normalizeCheckpointAction(existing.Action) == action && existing.Status == "running" {
			g.plannerResumeJobsMu.Unlock()
			return existing, true
		}
	}
	g.plannerResumeJobs[job.ID] = job
	path := g.plannerResumeJobsPath
	g.plannerResumeJobsMu.Unlock()
	if path != "" {
		if err := appendPlannerResumeJob(path, job); err != nil {
			slog.Warn("planner resume job store append failed", "job", job.ID, "err", err)
		}
	}
	return job, false
}

func (g *Gateway) getPlannerResumeJob(id, tenantID string) (plannerCheckpointResumePlanJob, bool) {
	tenantID = strings.TrimSpace(tenantID)
	g.plannerResumeJobsMu.Lock()
	if g.plannerResumeJobs != nil {
		job, ok := g.plannerResumeJobs[id]
		g.plannerResumeJobsMu.Unlock()
		return job, ok && plannerResumeJobTenantMatches(job, tenantID)
	}
	g.plannerResumeJobsMu.Unlock()
	if err := g.loadPlannerResumeJobs(); err != nil {
		slog.Warn("planner resume job store reload failed", "err", err)
	}
	g.plannerResumeJobsMu.Lock()
	defer g.plannerResumeJobsMu.Unlock()
	if g.plannerResumeJobs == nil {
		return plannerCheckpointResumePlanJob{}, false
	}
	job, ok := g.plannerResumeJobs[id]
	return job, ok && plannerResumeJobTenantMatches(job, tenantID)
}

func (g *Gateway) getLatestPlannerResumeJobForPlan(planID, tenantID string) (plannerCheckpointResumePlanJob, bool) {
	planID = strings.TrimSpace(planID)
	tenantID = strings.TrimSpace(tenantID)
	if planID == "" {
		return plannerCheckpointResumePlanJob{}, false
	}
	if err := g.loadPlannerResumeJobs(); err != nil {
		slog.Warn("planner resume job store reload failed", "err", err)
	}
	g.plannerResumeJobsMu.Lock()
	defer g.plannerResumeJobsMu.Unlock()
	var latest plannerCheckpointResumePlanJob
	ok := false
	for _, job := range g.plannerResumeJobs {
		if job.PlanID != planID || !plannerResumeJobTenantMatches(job, tenantID) {
			continue
		}
		if !ok || job.StartedAt > latest.StartedAt || (job.StartedAt == latest.StartedAt && job.ID > latest.ID) {
			latest = job
			ok = true
		}
	}
	return latest, ok
}

func plannerResumeJobTenantMatches(job plannerCheckpointResumePlanJob, tenantID string) bool {
	return strings.TrimSpace(job.TenantID) == strings.TrimSpace(tenantID)
}

func (g *Gateway) loadPlannerResumeJobs() error {
	g.plannerResumeJobsMu.Lock()
	path := g.plannerResumeJobsPath
	g.plannerResumeJobsMu.Unlock()
	if path == "" {
		return nil
	}
	jobs, err := readPlannerResumeJobs(path)
	if err != nil {
		return err
	}
	g.plannerResumeJobsMu.Lock()
	defer g.plannerResumeJobsMu.Unlock()
	if g.plannerResumeJobs == nil {
		g.plannerResumeJobs = make(map[string]plannerCheckpointResumePlanJob, len(jobs))
	}
	for id, job := range jobs {
		g.plannerResumeJobs[id] = job
	}
	return nil
}

func appendPlannerResumeJob(path string, job plannerCheckpointResumePlanJob) error {
	if strings.TrimSpace(path) == "" || strings.TrimSpace(job.ID) == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(job)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(data, '\n'))
	return err
}

func readPlannerResumeJobs(path string) (map[string]plannerCheckpointResumePlanJob, error) {
	out := make(map[string]plannerCheckpointResumePlanJob)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var job plannerCheckpointResumePlanJob
		if err := json.Unmarshal(scanner.Bytes(), &job); err == nil && strings.TrimSpace(job.ID) != "" {
			out[job.ID] = job
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (g *Gateway) findPlannerCheckpoint(r *http.Request, planID string) (planner.LongHorizonCheckpoint, bool, error) {
	checkpoints, err := g.planner.RecentLongHorizonCheckpointsForTenant(r.Context(), tenantFromCtx(r.Context()), 100)
	if err != nil {
		return planner.LongHorizonCheckpoint{}, false, err
	}
	for _, cp := range checkpoints {
		if cp.PlanID == planID {
			return cp, true, nil
		}
	}
	return planner.LongHorizonCheckpoint{}, false, nil
}

func parseCheckpointLimit(raw string) int {
	if raw == "" {
		return 20
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 20
	}
	if n > 100 {
		return 100
	}
	return n
}

func parseBoolQuery(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func normalizeCheckpointAction(raw string) string {
	action := strings.ToLower(strings.TrimSpace(raw))
	action = strings.ReplaceAll(action, "-", "_")
	switch action {
	case "", "continue", "resume", "resume_plan", "继续":
		return "continue"
	case "retry", "retry_failed", "retry_failed_step", "重试", "重试失败":
		return "retry_failed"
	case "partial", "return_partial", "return_partial_result", "阶段结果", "返回阶段结果":
		return "partial"
	default:
		return ""
	}
}

func summarizePlannerCheckpoint(cp planner.LongHorizonCheckpoint, includeSnapshot bool) plannerCheckpointSummary {
	item := plannerCheckpointSummary{
		PlanID:      cp.PlanID,
		TaskID:      cp.TaskID,
		Goal:        truncateStr(cp.Goal, 240),
		Status:      cp.Status,
		CurrentStep: cp.CurrentStep,
		Completed:   cp.Completed,
		Total:       cp.Total,
		StepsUsed:   cp.StepsUsed,
		Revisions:   cp.Revisions,
		Error:       plannerCheckpointDisplayError(cp.Error),
		Recoverable: cp.Recoverable,
		ResumeHint:  cp.ResumeHint,
	}
	if !cp.UpdatedAt.IsZero() {
		item.UpdatedAt = cp.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")
	}
	if includeSnapshot {
		item.PlanSnapshot = summarizePlanSnapshot(cp.PlanSnapshot)
	}
	return item
}

func plannerCheckpointDisplayError(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if friendly := plannerKnownFriendlyError(raw); friendly != "" {
		return friendly
	}
	return truncateStr(raw, 240)
}

func plannerKnownFriendlyError(raw string) string {
	message := strings.TrimSpace(raw)
	lower := strings.ToLower(message)
	switch {
	case strings.Contains(message, "尚未完成") || strings.Contains(message, "依赖") || strings.Contains(lower, "dependency"):
		return "前置步骤还没有完全确认，已保留现场，可查看依赖关系后继续。"
	case strings.Contains(lower, "unknown skill") || strings.Contains(message, "未知工具") || strings.Contains(message, "未找到工具"):
		return "所需工具暂时不可用，已保留现场，可换用可用工具或调整步骤继续。"
	case strings.Contains(lower, "blocked by trust gate") || strings.Contains(message, "信任") || strings.Contains(lower, "trust gate"):
		return "这一步需要更高信任或确认，已保留现场，可确认后继续。"
	case strings.Contains(lower, "tool panic") || strings.Contains(lower, "panic"):
		return "工具运行时遇到异常，已保留现场，可重试或切换策略继续。"
	case strings.Contains(lower, "context deadline exceeded") || strings.Contains(message, "响应超时") || strings.Contains(lower, "timeout"):
		return "响应暂时超时，已保留现场，可稍后重试或先返回阶段结果。"
	case strings.Contains(message, "当前模型响应失败") || strings.Contains(message, "备用模型") || strings.Contains(message, "调用栈降级") || strings.Contains(message, "级联唤醒") || strings.Contains(message, "备用引擎"):
		return "模型暂时没有回应，已保留现场，正在换用可用模型继续。"
	case strings.Contains(lower, "execution failed") || strings.Contains(lower, "handoff agent") || strings.Contains(lower, "fallback") || strings.Contains(lower, "all fallback llm clients failed") || strings.Contains(lower, "eof"):
		return "任务暂时没有顺利完成，已保留现场，可切换策略或稍后继续。"
	default:
		return ""
	}
}

func summarizePlanSnapshot(in []planner.PlanStep) []planner.PlanStep {
	if len(in) == 0 {
		return nil
	}
	out := make([]planner.PlanStep, 0, len(in))
	for _, step := range in {
		step.Result = plannerCheckpointDisplayResult(step.Result)
		step.Error = plannerCheckpointDisplayError(step.Error)
		out = append(out, step)
	}
	return out
}

func plannerCheckpointDisplayResult(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if friendly := plannerKnownFriendlyError(raw); friendly != "" {
		return friendly
	}
	return truncateStr(raw, 240)
}

func buildPlannerCheckpointPlan(cp planner.LongHorizonCheckpoint, action, prompt string) plannerCheckpointPlan {
	steps := make([]plannerCheckpointPlanStep, 0, len(cp.PlanSnapshot))
	selected := selectPlannerRecoverySteps(cp.PlanSnapshot, action)
	executable := true
	reason := ""
	if len(cp.PlanSnapshot) == 0 {
		executable = false
		reason = "没有可恢复的步骤快照，只能生成继续提示。"
	}
	if action == "partial" {
		executable = false
		reason = "当前操作只返回已完成部分，不会继续执行步骤。"
	}
	if action != "partial" && len(selected) == 0 {
		executable = false
		reason = "没有找到需要继续或重试的步骤。"
	}

	byID := make(map[int]planner.PlanStep, len(cp.PlanSnapshot))
	for _, step := range cp.PlanSnapshot {
		byID[step.ID] = step
	}
	if action != "partial" {
		if missing := firstUnmetPlannerDependency(cp.PlanSnapshot, selected, byID); missing != "" {
			executable = false
			reason = missing
		}
	}

	for _, step := range cp.PlanSnapshot {
		id := step.ID
		isSelected := selected[id]
		stepReason := plannerRecoveryStepReason(step, action, isSelected)
		steps = append(steps, plannerCheckpointPlanStep{
			ID:        id,
			Action:    plannerRecoveryStepAction(step),
			Skill:     step.Skill,
			Status:    string(step.Status),
			DependsOn: append([]int(nil), step.DependsOn...),
			Selected:  isSelected,
			Reason:    stepReason,
		})
	}
	return plannerCheckpointPlan{
		Mode:       action,
		Executable: executable,
		Reason:     reason,
		PlanID:     cp.PlanID,
		TaskID:     cp.TaskID,
		Steps:      steps,
		Prompt:     prompt,
	}
}

func checkpointTaskSteps(cp planner.LongHorizonCheckpoint, recoveryPlan plannerCheckpointPlan) []task.Step {
	if recoveryPlan.Mode == "partial" {
		return []task.Step{{
			ID:         1,
			Action:     "整理并返回这个可恢复规划已经完成的部分，说明剩余步骤和下一步最小动作。",
			Status:     task.StepPending,
			MaxRetries: task.DefaultMaxRetries,
		}}
	}
	selected := make(map[int]bool, len(recoveryPlan.Steps))
	for _, step := range recoveryPlan.Steps {
		if step.Selected {
			selected[step.ID] = true
		}
	}
	selectedSteps := make([]planner.PlanStep, 0, len(selected))
	for _, step := range cp.PlanSnapshot {
		if !selected[step.ID] {
			continue
		}
		selectedSteps = append(selectedSteps, step)
	}
	taskIDByPlannerID := make(map[int]int, len(selectedSteps))
	for i, step := range selectedSteps {
		taskIDByPlannerID[step.ID] = i + 1
	}
	out := make([]task.Step, 0, len(selectedSteps))
	for _, step := range selectedSteps {
		deps := make([]int, 0, len(step.DependsOn))
		for _, depID := range step.DependsOn {
			if taskStepID, ok := taskIDByPlannerID[depID]; ok {
				deps = append(deps, taskStepID)
			}
		}
		out = append(out, task.Step{
			ID:         len(out) + 1,
			Action:     plannerRecoveryStepAction(step),
			SkillName:  step.Skill,
			Args:       step.Args,
			Input:      checkpointCompletedDependencyContext(cp, step),
			Status:     task.StepPending,
			MaxRetries: task.DefaultMaxRetries,
			DependsOn:  deps,
			Metadata: map[string]any{
				"source":             "planner_checkpoint",
				"plan_id":            cp.PlanID,
				"origin_task_id":     cp.TaskID,
				"planner_step_id":    step.ID,
				"planner_depends_on": step.DependsOn,
				"planner_status":     string(step.Status),
			},
		})
	}
	if len(out) == 0 {
		out = append(out, task.Step{
			ID:         1,
			Action:     "根据恢复计划返回阶段结果，并说明没有找到需要继续执行的步骤。",
			Status:     task.StepPending,
			MaxRetries: task.DefaultMaxRetries,
		})
	}
	return out
}

func checkpointCompletedDependencyContext(cp planner.LongHorizonCheckpoint, step planner.PlanStep) string {
	if len(step.DependsOn) == 0 || len(cp.PlanSnapshot) == 0 {
		return ""
	}
	byID := make(map[int]planner.PlanStep, len(cp.PlanSnapshot))
	for _, candidate := range cp.PlanSnapshot {
		byID[candidate.ID] = candidate
	}
	var b strings.Builder
	for _, depID := range step.DependsOn {
		dep, ok := byID[depID]
		if !ok || (dep.Status != planner.StepDone && dep.Status != planner.StepSkipped) {
			continue
		}
		result := plannerCheckpointDisplayResult(dep.Result)
		if result == "" {
			continue
		}
		label := plannerRecoveryStepAction(dep)
		b.WriteString(fmt.Sprintf("[%s]: %s\n", label, truncateStr(result, 1200)))
	}
	return strings.TrimSpace(b.String())
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func mapBool(ok bool, yes, no string) string {
	if ok {
		return yes
	}
	return no
}

func selectPlannerRecoverySteps(steps []planner.PlanStep, action string) map[int]bool {
	selected := make(map[int]bool)
	switch action {
	case "partial":
		return selected
	case "retry_failed":
		for _, step := range steps {
			if step.Status == planner.StepFailed {
				selected[step.ID] = true
			}
		}
		if len(selected) == 0 {
			for _, step := range steps {
				if step.Status != planner.StepDone && step.Status != planner.StepSkipped {
					selected[step.ID] = true
					break
				}
			}
		}
	default:
		for _, step := range steps {
			switch step.Status {
			case planner.StepPending, planner.StepRunning, planner.StepFailed:
				selected[step.ID] = true
			}
		}
	}
	return selected
}

func firstUnmetPlannerDependency(steps []planner.PlanStep, selected map[int]bool, byID map[int]planner.PlanStep) string {
	for _, step := range steps {
		if !selected[step.ID] {
			continue
		}
		for _, depID := range step.DependsOn {
			dep, ok := byID[depID]
			if !ok {
				return fmt.Sprintf("步骤 %d 依赖的步骤 %d 不在快照中，暂不能安全直接恢复。", step.ID, depID)
			}
			if selected[depID] || dep.Status == planner.StepDone || dep.Status == planner.StepSkipped {
				continue
			}
			return fmt.Sprintf("步骤 %d 依赖的步骤 %d 尚未完成，暂不能安全直接恢复。", step.ID, depID)
		}
	}
	return ""
}

func plannerRecoveryStepAction(step planner.PlanStep) string {
	if strings.TrimSpace(step.Action) != "" {
		return step.Action
	}
	if strings.TrimSpace(step.Skill) != "" {
		return step.Skill
	}
	return fmt.Sprintf("步骤 %d", step.ID)
}

func plannerRecoveryStepReason(step planner.PlanStep, action string, selected bool) string {
	if action == "partial" {
		if step.Status == planner.StepDone || step.Status == planner.StepSkipped {
			return "会纳入阶段结果"
		}
		return "暂不继续执行"
	}
	if selected {
		switch step.Status {
		case planner.StepFailed:
			return "需要重试"
		case planner.StepRunning:
			return "上次中断时正在执行，恢复时会重新确认"
		case planner.StepPending:
			return "尚未执行"
		default:
			return "已选入恢复范围"
		}
	}
	if step.Status == planner.StepDone || step.Status == planner.StepSkipped {
		return "已完成，不重复执行"
	}
	return "暂不在本次恢复范围"
}

func buildPlannerRecoveryPrompt(cp planner.LongHorizonCheckpoint, action string) string {
	var b strings.Builder
	switch action {
	case "retry_failed":
		b.WriteString("请重试这个可恢复规划里的失败步骤。\n")
		b.WriteString("不要重复已完成步骤，先缩小输入和工具面；如果同一路径再次失败，请换策略并返回阶段结果。\n\n")
	case "partial":
		b.WriteString("请基于这个可恢复规划，先整理并返回已经完成的部分。\n")
		b.WriteString("明确说明哪些步骤已完成、哪些失败、下一步最小可执行动作是什么。\n\n")
	default:
		b.WriteString("请继续这个可恢复规划。\n")
		b.WriteString("不要从头重跑，优先复用已完成步骤，只处理 pending/failed 部分；必要时调整工具或降低粒度。\n\n")
	}
	b.WriteString(fmt.Sprintf("Plan ID：%s\n", cp.PlanID))
	if cp.TaskID != "" {
		b.WriteString(fmt.Sprintf("Task ID：%s\n", cp.TaskID))
	} else {
		b.WriteString("Task ID：未知\n")
	}
	if cp.Goal != "" {
		b.WriteString("原始目标：")
		b.WriteString(truncateStr(cp.Goal, 240))
		b.WriteString("\n")
	}
	b.WriteString(fmt.Sprintf("状态：%s\n", cp.Status))
	b.WriteString(fmt.Sprintf("进度：%d/%d\n", cp.Completed, cp.Total))
	if cp.Error != "" {
		b.WriteString("失败原因：")
		b.WriteString(plannerCheckpointDisplayError(cp.Error))
		b.WriteString("\n")
	}
	if cp.ResumeHint != "" {
		b.WriteString("恢复提示：")
		b.WriteString(cp.ResumeHint)
		b.WriteString("\n")
	}
	if pattern, recommendation := plannerCheckpointRecoveryFailureAnalysis(cp); pattern != "" {
		b.WriteString("失败模式：")
		b.WriteString(pattern)
		b.WriteString("\n")
		if recommendation != "" {
			b.WriteString("推荐策略：")
			b.WriteString(recommendation)
			b.WriteString("\n")
		}
	}
	appendPlannerRecoveryScope(&b, cp, action)
	if len(cp.PlanSnapshot) > 0 {
		b.WriteString("\n步骤快照：\n")
		for i, step := range cp.PlanSnapshot {
			if i >= 12 {
				b.WriteString(fmt.Sprintf("- ...还有 %d 个步骤未列出\n", len(cp.PlanSnapshot)-i))
				break
			}
			b.WriteString(formatPlannerRecoveryStep(step))
			b.WriteString("\n")
		}
	}
	return strings.TrimSpace(b.String())
}

func plannerCheckpointRecoveryFailureAnalysis(cp planner.LongHorizonCheckpoint) (string, string) {
	errors := []string{}
	failedTools := []string{}
	seenTools := map[string]bool{}
	if cp.Error != "" {
		errors = append(errors, cp.Error)
	}
	for _, step := range cp.PlanSnapshot {
		if step.Status != planner.StepFailed {
			continue
		}
		if step.Error != "" {
			errors = append(errors, step.Error)
		}
		label := strings.TrimSpace(step.Skill)
		if label == "" {
			label = strings.TrimSpace(step.Action)
		}
		if label != "" && !seenTools[label] {
			failedTools = append(failedTools, label)
			seenTools[label] = true
		}
	}
	return plannerCheckpointFailureAnalysis(errors, failedTools)
}

func appendPlannerRecoveryScope(b *strings.Builder, cp planner.LongHorizonCheckpoint, action string) {
	if len(cp.PlanSnapshot) == 0 {
		return
	}
	if action == "partial" {
		b.WriteString("\n本次恢复范围：只整理已完成/已跳过步骤，不继续执行。\n")
		return
	}
	selected := selectPlannerRecoverySteps(cp.PlanSnapshot, action)
	if len(selected) == 0 {
		b.WriteString("\n本次恢复范围：未找到需要继续的步骤，请先返回阶段结果。\n")
		return
	}
	b.WriteString(fmt.Sprintf("\n本次恢复范围：继续 %d 个步骤。\n", len(selected)))
	listed := 0
	for _, step := range cp.PlanSnapshot {
		if !selected[step.ID] {
			continue
		}
		if listed >= 12 {
			b.WriteString("- ...还有更多步骤未列出\n")
			break
		}
		b.WriteString(formatPlannerRecoveryStep(step))
		b.WriteString("\n")
		listed++
	}
}

func formatPlannerRecoveryStep(step planner.PlanStep) string {
	action := step.Action
	if action == "" {
		action = step.Skill
	}
	if action == "" {
		action = fmt.Sprintf("步骤 %d", step.ID)
	}
	line := fmt.Sprintf("- %s · %s", step.Status, action)
	if step.Skill != "" {
		line += " · skill=" + step.Skill
	}
	if step.Error != "" {
		line += " · error=" + truncateStr(plannerCheckpointDisplayError(step.Error), 180)
	}
	if step.Result != "" {
		line += " · result=" + truncateStr(strings.ReplaceAll(plannerCheckpointDisplayResult(step.Result), "\n", " / "), 240)
	}
	return line
}
