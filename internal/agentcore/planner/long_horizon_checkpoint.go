package planner

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/plan"
	"yunque-agent/internal/observe"
)

// LongHorizonCheckpoint is the recoverable, UI-safe snapshot emitted while a
// DAG plan is running. It intentionally mirrors PlanResult.Plan so a dropped
// SSE connection or failed step can still leave enough state for the frontend
// to show what completed, what failed, and which step can be retried later.
type LongHorizonCheckpoint struct {
	PlanID       string     `json:"plan_id"`
	TenantID     string     `json:"tenant_id,omitempty"`
	TaskID       string     `json:"task_id,omitempty"`
	Goal         string     `json:"goal,omitempty"`
	Status       string     `json:"status"`
	CurrentStep  int        `json:"current_step"`
	Completed    int        `json:"completed"`
	Total        int        `json:"total"`
	StepsUsed    int        `json:"steps_used"`
	Revisions    int        `json:"revisions"`
	Error        string     `json:"error,omitempty"`
	Recoverable  bool       `json:"recoverable"`
	ResumeHint   string     `json:"resume_hint,omitempty"`
	PlanSnapshot []PlanStep `json:"plan_snapshot,omitempty"`
	UpdatedAt    time.Time  `json:"updated_at,omitempty"`
}

func buildLongHorizonCheckpoint(req PlanRequest, pl *plan.Plan, errText string) LongHorizonCheckpoint {
	if pl == nil {
		return LongHorizonCheckpoint{TaskID: req.TaskID, Error: errText, UpdatedAt: time.Now().UTC()}
	}
	completed, total := pl.Progress()
	cp := LongHorizonCheckpoint{
		PlanID:       pl.ID,
		TenantID:     req.TenantID,
		TaskID:       req.TaskID,
		Goal:         truncate(extractGoal(req), 500),
		Status:       string(pl.Status),
		CurrentStep:  pl.CurrentStep(),
		Completed:    completed,
		Total:        total,
		StepsUsed:    pl.Budget.StepsUsed,
		Revisions:    pl.Revisions,
		Error:        errText,
		Recoverable:  errText != "" || pl.Status == plan.PlanFailed || pl.Status == plan.PlanAborted,
		PlanSnapshot: planStepsFromDAG(pl, 600),
	}
	if cp.Recoverable {
		cp.ResumeHint = "可根据 plan_snapshot 继续、重试失败步骤，或先返回已完成部分。"
	}
	return cp
}

func friendlyLongHorizonCheckpoint(cp LongHorizonCheckpoint) LongHorizonCheckpoint {
	if cp.Error != "" {
		cp.Error = plannerFriendlyFailureText(cp.Error)
	}
	if len(cp.PlanSnapshot) > 0 {
		cp.PlanSnapshot = append([]PlanStep(nil), cp.PlanSnapshot...)
		for i := range cp.PlanSnapshot {
			if cp.PlanSnapshot[i].Result != "" {
				cp.PlanSnapshot[i].Result = plannerFriendlyOutputForModel(cp.PlanSnapshot[i].Result)
			}
			if cp.PlanSnapshot[i].Error != "" {
				cp.PlanSnapshot[i].Error = plannerFriendlyFailureText(cp.PlanSnapshot[i].Error)
			}
		}
	}
	return cp
}

func planStepsFromDAG(pl *plan.Plan, outputLimit int) []PlanStep {
	if pl == nil {
		return nil
	}
	out := make([]PlanStep, 0, len(pl.Steps))
	for _, s := range pl.Steps {
		out = append(out, PlanStep{
			ID:        s.Index,
			Action:    s.Description,
			Skill:     s.Skill,
			Args:      s.Args,
			DependsOn: append([]int(nil), s.DependsOn...),
			Status:    convertStatus(s.Status),
			Result:    truncate(s.Output, outputLimit),
			Error:     truncate(s.Error, outputLimit),
		})
	}
	return out
}

func usedSkillsFromDAG(pl *plan.Plan) []string {
	if pl == nil {
		return nil
	}
	var out []string
	for _, s := range pl.Steps {
		out = append(out, s.ToolsUsed...)
	}
	return out
}

// LongHorizonCheckpointStore persists recoverable long-horizon state so a
// dropped SSE connection or process restart does not erase the last known plan.
type LongHorizonCheckpointStore interface {
	Save(ctx context.Context, cp LongHorizonCheckpoint) error
	Recent(ctx context.Context, limit int) ([]LongHorizonCheckpoint, error)
}

type tenantScopedLongHorizonCheckpointStore interface {
	RecentForTenant(ctx context.Context, tenantID string, limit int) ([]LongHorizonCheckpoint, error)
}

// FileLongHorizonCheckpointStore appends checkpoints as JSONL under the local
// data directory. It intentionally stays append-only: the newest line is the
// source of truth, while older lines remain useful for debugging/replay.
type FileLongHorizonCheckpointStore struct {
	path string
	mu   sync.Mutex
}

func NewFileLongHorizonCheckpointStore(path string) *FileLongHorizonCheckpointStore {
	return &FileLongHorizonCheckpointStore{path: path}
}

func (s *FileLongHorizonCheckpointStore) Save(ctx context.Context, cp LongHorizonCheckpoint) error {
	if s == nil || s.path == "" || cp.PlanID == "" {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if cp.UpdatedAt.IsZero() {
		cp.UpdatedAt = time.Now().UTC()
	}
	data, err := json.Marshal(cp)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(s.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(data, '\n')); err != nil {
		return err
	}
	return nil
}

func (s *FileLongHorizonCheckpointStore) Recent(ctx context.Context, limit int) ([]LongHorizonCheckpoint, error) {
	return s.recent(ctx, "", limit, false)
}

func (s *FileLongHorizonCheckpointStore) RecentForTenant(ctx context.Context, tenantID string, limit int) ([]LongHorizonCheckpoint, error) {
	return s.recent(ctx, tenantID, limit, true)
}

func (s *FileLongHorizonCheckpointStore) recent(ctx context.Context, tenantID string, limit int, filterTenant bool) ([]LongHorizonCheckpoint, error) {
	if s == nil || s.path == "" {
		return nil, nil
	}
	tenantID = strings.TrimSpace(tenantID)
	if limit <= 0 {
		limit = 20
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := os.Open(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	var all []LongHorizonCheckpoint
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		var cp LongHorizonCheckpoint
		if err := json.Unmarshal(scanner.Bytes(), &cp); err == nil && cp.PlanID != "" {
			if filterTenant && strings.TrimSpace(cp.TenantID) != tenantID {
				continue
			}
			all = append(all, cp)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	// newest first for UI/API consumers, with append-only updates collapsed so
	// "最近可恢复任务" shows one latest row per plan instead of every step event.
	capacity := limit
	if len(all) < capacity {
		capacity = len(all)
	}
	recent := make([]LongHorizonCheckpoint, 0, capacity)
	seen := make(map[string]bool, len(all))
	for i := len(all) - 1; i >= 0 && len(recent) < limit; i-- {
		cp := all[i]
		key := longHorizonCheckpointDedupKey(cp)
		if seen[key] {
			continue
		}
		seen[key] = true
		recent = append(recent, cp)
	}
	return recent, nil
}

func longHorizonCheckpointDedupKey(cp LongHorizonCheckpoint) string {
	return strings.TrimSpace(cp.TenantID) + "\x00" + strings.TrimSpace(cp.PlanID)
}

func (p *Planner) SetLongHorizonCheckpointStore(store LongHorizonCheckpointStore) {
	p.longHorizonCheckpoints = store
}

// RecentLongHorizonCheckpoints returns newest-first recoverable DAG snapshots
// for UI/API consumers. It is intentionally read-only and best-effort: callers
// should treat an empty slice as "nothing to recover" rather than a fatal
// planner state.
func (p *Planner) RecentLongHorizonCheckpoints(ctx context.Context, limit int) ([]LongHorizonCheckpoint, error) {
	if p == nil || p.longHorizonCheckpoints == nil {
		return nil, nil
	}
	return p.longHorizonCheckpoints.Recent(ctx, limit)
}

func (p *Planner) RecentLongHorizonCheckpointsForTenant(ctx context.Context, tenantID string, limit int) ([]LongHorizonCheckpoint, error) {
	if p == nil || p.longHorizonCheckpoints == nil {
		return nil, nil
	}
	tenantID = strings.TrimSpace(tenantID)
	if scoped, ok := p.longHorizonCheckpoints.(tenantScopedLongHorizonCheckpointStore); ok {
		return scoped.RecentForTenant(ctx, tenantID, limit)
	}
	scanLimit := limit
	if scanLimit < 100 {
		scanLimit = 100
	}
	checkpoints, err := p.longHorizonCheckpoints.Recent(ctx, scanLimit)
	if err != nil {
		return nil, err
	}
	out := make([]LongHorizonCheckpoint, 0, len(checkpoints))
	for _, cp := range checkpoints {
		if strings.TrimSpace(cp.TenantID) == tenantID {
			out = append(out, cp)
			if limit > 0 && len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

func (p *Planner) persistLongHorizonCheckpoint(req PlanRequest, cp LongHorizonCheckpoint) {
	if p.longHorizonCheckpoints == nil || cp.PlanID == "" {
		return
	}
	if err := p.longHorizonCheckpoints.Save(context.Background(), cp); err != nil {
		// Persistence is best-effort; never fail the planner because the local
		// recovery log is temporarily unavailable.
	}
}

func (p *Planner) emitLongHorizonCheckpoint(req PlanRequest, pl *plan.Plan, errText string) {
	if req.StepCallback == nil || pl == nil {
		return
	}
	cp := buildLongHorizonCheckpoint(req, pl, errText)
	p.persistLongHorizonCheckpoint(req, cp)
	summary := fmt.Sprintf("长程规划进度：%d/%d", cp.Completed, cp.Total)
	if errText != "" {
		summary = "长程规划已保存失败现场，可继续恢复"
	}
	evt := observe.NewEvent(req.TraceID, observe.DomainPlanner, observe.EventPlan, summary)
	evt.Meta.TenantID = req.TenantID
	evt.Meta.TaskID = req.TaskID
	evt.Detail = friendlyLongHorizonCheckpoint(cp)
	req.StepCallback(evt)
}

func normalizeCheckpointResumeAction(raw string) string {
	switch raw {
	case "", "continue", "resume", "resume_plan":
		return "continue"
	case "retry", "retry_failed", "retry_failed_step":
		return "retry_failed"
	case "partial", "return_partial", "return_partial_result":
		return "partial"
	default:
		return ""
	}
}

func rebuildCheckpointDAGSteps(cp LongHorizonCheckpoint, action string) ([]plan.PlanStep, error) {
	action = normalizeCheckpointResumeAction(action)
	if action == "" {
		return nil, fmt.Errorf("unsupported checkpoint resume action")
	}
	if len(cp.PlanSnapshot) == 0 {
		return nil, fmt.Errorf("checkpoint has no plan snapshot")
	}
	byID := make(map[int]PlanStep, len(cp.PlanSnapshot))
	idToIndex := make(map[int]int, len(cp.PlanSnapshot))
	for i, step := range cp.PlanSnapshot {
		byID[step.ID] = step
		idToIndex[step.ID] = i
	}
	selected := make(map[int]bool, len(cp.PlanSnapshot))
	for _, step := range cp.PlanSnapshot {
		if checkpointStepSelectedForResume(step, action) {
			selected[step.ID] = true
		}
	}
	for _, step := range cp.PlanSnapshot {
		if !selected[step.ID] {
			continue
		}
		for _, depID := range step.DependsOn {
			dep, ok := byID[depID]
			if !ok {
				return nil, fmt.Errorf("checkpoint step %d depends on missing step %d", step.ID, depID)
			}
			if selected[depID] || dep.Status == StepDone || dep.Status == StepSkipped {
				continue
			}
			return nil, fmt.Errorf("checkpoint step %d waits for unfinished dependency %d", step.ID, depID)
		}
	}
	out := make([]plan.PlanStep, 0, len(cp.PlanSnapshot))
	for i, step := range cp.PlanSnapshot {
		status := planStatusFromCheckpoint(step.Status)
		output := step.Result
		errText := step.Error
		if selected[step.ID] {
			status = plan.StepPending
			output = ""
			errText = ""
		} else if status != plan.StepCompleted && status != plan.StepSkipped {
			status = plan.StepSkipped
			output = "not selected in checkpoint resume"
			errText = ""
		}
		deps := make([]int, 0, len(step.DependsOn))
		for _, depID := range step.DependsOn {
			if depIndex, ok := idToIndex[depID]; ok {
				deps = append(deps, depIndex)
			} else {
				deps = append(deps, -1)
			}
		}
		out = append(out, plan.PlanStep{
			Index:       i,
			Description: step.Action,
			Skill:       step.Skill,
			Args:        cloneArgs(step.Args),
			DependsOn:   deps,
			Status:      status,
			Output:      output,
			Error:       errText,
		})
	}
	return out, nil
}

func checkpointStepSelectedForResume(step PlanStep, action string) bool {
	switch action {
	case "continue":
		return step.Status == StepPending || step.Status == StepRunning || step.Status == StepFailed
	case "retry_failed":
		return step.Status == StepFailed
	default:
		return false
	}
}

func planStatusFromCheckpoint(status StepStatus) plan.StepStatus {
	switch status {
	case StepDone:
		return plan.StepCompleted
	case StepSkipped:
		return plan.StepSkipped
	case StepRunning:
		return plan.StepInProgress
	case StepFailed:
		return plan.StepFailed
	default:
		return plan.StepPending
	}
}

func cloneDAGSteps(in []plan.PlanStep) []plan.PlanStep {
	out := make([]plan.PlanStep, len(in))
	for i, step := range in {
		out[i] = step
		out[i].Args = cloneArgs(step.Args)
		out[i].DependsOn = append([]int(nil), step.DependsOn...)
		out[i].ToolsUsed = append([]string(nil), step.ToolsUsed...)
	}
	return out
}

func cloneArgs(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func requestWithCheckpointGoal(req PlanRequest, cp LongHorizonCheckpoint) PlanRequest {
	if extractGoal(req) != "" || cp.Goal == "" {
		return req
	}
	req.Messages = append([]llm.Message(nil), req.Messages...)
	req.Messages = append(req.Messages, llm.Message{Role: "user", Content: cp.Goal})
	return req
}

func checkpointResumeGoal(req PlanRequest, cp LongHorizonCheckpoint) string {
	if goal := extractGoal(req); goal != "" {
		return goal
	}
	if cp.Goal != "" {
		return cp.Goal
	}
	if cp.PlanID != "" {
		return "恢复规划 " + cp.PlanID
	}
	return "恢复规划"
}

func partialCheckpointResult(cp LongHorizonCheckpoint) *PlanResult {
	var b strings.Builder
	if cp.Goal != "" {
		b.WriteString("阶段结果：")
		b.WriteString(cp.Goal)
		b.WriteString("\n\n")
	} else {
		b.WriteString("阶段结果：\n\n")
	}
	for _, step := range cp.PlanSnapshot {
		if step.Status != StepDone && step.Status != StepSkipped {
			continue
		}
		b.WriteString("- ")
		if step.Action != "" {
			b.WriteString(step.Action)
		} else {
			b.WriteString(step.Skill)
		}
		if step.Result != "" {
			b.WriteString("（已保留证据）：")
			b.WriteString(truncate(step.Result, 300))
		}
		b.WriteString("\n")
	}
	return &PlanResult{
		Reply:      strings.TrimSpace(b.String()),
		SkillsUsed: nil,
		Steps:      cp.StepsUsed,
		Plan:       append([]PlanStep(nil), cp.PlanSnapshot...),
	}
}
