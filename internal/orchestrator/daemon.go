package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"yunque-agent/internal/agentcore/task"
	mcpserver "yunque-agent/internal/mcp/server"
)

// DaemonPolicy controls what the daemon is allowed to do automatically.
type DaemonPolicy struct {
	AllowAutoLaunch  bool `json:"allow_auto_launch"`  // auto-start IDEs for pending tasks
	AllowAutoReview  bool `json:"allow_auto_review"`   // auto-review completed results
	AllowAutoRequeue bool `json:"allow_auto_requeue"`  // auto-requeue timed-out tasks
	RequireApproval  bool `json:"require_approval"`    // require human approval before marking done
}

// DefaultPolicy returns a fully autonomous policy.
func DefaultPolicy() DaemonPolicy {
	return DaemonPolicy{
		AllowAutoLaunch:  true,
		AllowAutoReview:  true,
		AllowAutoRequeue: true,
		RequireApproval:  false,
	}
}

// NotifyFunc is called when the daemon needs to inform the user about events.
type NotifyFunc func(level, title, detail string)

type Daemon struct {
	taskStore  task.Store
	dispatcher *task.Dispatcher
	workers    *mcpserver.WorkerRegistry
	launcher   *Launcher
	reviewer   *ReviewAgent
	projects   *ProjectStore
	interval   time.Duration
	policy     DaemonPolicy
	notify     NotifyFunc
	eventLog   *EventLog

	mu               sync.Mutex
	running          bool
	cancel           context.CancelFunc
	consecutiveFails int
	circuitOpen      bool
	circuitOpenAt    time.Time
}

type DaemonConfig struct {
	TaskStore  task.Store
	Dispatcher *task.Dispatcher
	Workers    *mcpserver.WorkerRegistry
	Launcher   *Launcher
	Reviewer   *ReviewAgent
	Projects   *ProjectStore
	Interval   time.Duration
	Policy     *DaemonPolicy
	Notify     NotifyFunc
}

const (
	circuitBreakerThreshold = 5               // consecutive failures to trigger circuit breaker
	circuitBreakerCooldown  = 2 * time.Minute // wait before auto-recovery
)

func NewDaemon(cfg DaemonConfig) *Daemon {
	if cfg.Interval == 0 {
		cfg.Interval = 5 * time.Second
	}
	policy := DefaultPolicy()
	if cfg.Policy != nil {
		policy = *cfg.Policy
	}
	return &Daemon{
		taskStore:  cfg.TaskStore,
		dispatcher: cfg.Dispatcher,
		workers:    cfg.Workers,
		launcher:   cfg.Launcher,
		reviewer:   cfg.Reviewer,
		projects:   cfg.Projects,
		interval:   cfg.Interval,
		policy:     policy,
		notify:     cfg.Notify,
		eventLog:   NewEventLog(2000),
	}
}

// SetNotify sets the notification callback.
func (d *Daemon) SetNotify(fn NotifyFunc) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.notify = fn
}

// SetPolicy updates the daemon's automation policy at runtime.
func (d *Daemon) SetPolicy(p DaemonPolicy) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.policy = p
	slog.Info("orchestrator: policy updated",
		"auto_launch", p.AllowAutoLaunch, "auto_review", p.AllowAutoReview,
		"auto_requeue", p.AllowAutoRequeue, "require_approval", p.RequireApproval)
}

// Policy returns the current policy.
func (d *Daemon) Policy() DaemonPolicy {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.policy
}

// Events returns the event log for querying orchestration history.
func (d *Daemon) Events() *EventLog {
	return d.eventLog
}

func (d *Daemon) emitEvent(typ EventType, taskID, message string, meta map[string]any) {
	d.eventLog.Append(Event{
		Type:    typ,
		TaskID:  taskID,
		Message: message,
		Meta:    meta,
	})
}

func (d *Daemon) Start(ctx context.Context) {
	d.mu.Lock()
	if d.running {
		d.mu.Unlock()
		return
	}
	childCtx, cancel := context.WithCancel(ctx)
	d.running = true
	d.cancel = cancel
	d.mu.Unlock()

	slog.Info("orchestrator: daemon started", "interval", d.interval)
	d.emitEvent(EventDaemonStarted, "", "daemon started", nil)

	go d.loop(childCtx)
}

func (d *Daemon) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.running {
		return
	}
	d.cancel()
	d.running = false
	slog.Info("orchestrator: daemon stopped")
	d.emitEvent(EventDaemonStopped, "", "daemon stopped", nil)
}

func (d *Daemon) IsRunning() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.running
}

func (d *Daemon) loop(ctx context.Context) {
	defer func() {
		d.mu.Lock()
		d.running = false
		d.mu.Unlock()
	}()

	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.tick(ctx)
		}
	}
}

func (d *Daemon) tick(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("orchestrator: panic in tick", "recover", r)
			d.recordFailure("panic in tick")
		}
	}()

	d.mu.Lock()
	if d.circuitOpen {
		if time.Since(d.circuitOpenAt) > circuitBreakerCooldown {
			slog.Info("orchestrator: circuit breaker cooldown expired, half-open")
			d.circuitOpen = false
			d.consecutiveFails = 0
			d.eventLog.Append(Event{Type: EventCircuitClosed, Message: "circuit breaker recovered"})
		} else {
			d.mu.Unlock()
			return
		}
	}
	d.mu.Unlock()

	d.checkPendingTasks(ctx)
	d.checkTimedOutTasks(ctx)
	d.checkCompletedResults(ctx)
}

func (d *Daemon) recordFailure(reason string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.consecutiveFails++
	if d.consecutiveFails >= circuitBreakerThreshold {
		d.circuitOpen = true
		d.circuitOpenAt = time.Now()
		slog.Error("orchestrator: circuit breaker OPEN — too many consecutive failures",
			"count", d.consecutiveFails, "cooldown", circuitBreakerCooldown)
		d.emitEvent(EventCircuitOpen, "", "circuit breaker triggered",
			map[string]any{"consecutive_fails": d.consecutiveFails, "reason": reason})
		d.sendNotify("error", "编排熔断",
			fmt.Sprintf("连续 %d 次失败，守护进程已暂停 %v。原因: %s", d.consecutiveFails, circuitBreakerCooldown, reason))
	}
}

func (d *Daemon) recordSuccess() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.consecutiveFails = 0
}

func (d *Daemon) sendNotify(level, title, detail string) {
	if d.notify != nil {
		d.notify(level, title, detail)
	}
}

func (d *Daemon) checkPendingTasks(ctx context.Context) {
	if d.dispatcher == nil {
		return
	}

	pending := d.dispatcher.Pending()
	if len(pending) == 0 {
		return
	}

	for _, entry := range pending {
		if ctx.Err() != nil {
			return
		}

		if d.workers != nil && len(entry.RequiredCaps) > 0 {
			candidates := d.workers.FindByCapability(entry.RequiredCaps[0])
			if len(candidates) > 0 {
				continue
			}
		}

		if !d.policy.AllowAutoLaunch {
			slog.Debug("orchestrator: auto-launch disabled by policy", "task", entry.TaskID)
			continue
		}

		d.tryAutoLaunch(ctx, entry)
	}
}

func (d *Daemon) tryAutoLaunch(ctx context.Context, entry task.DispatchEntry) {
	if d.launcher == nil {
		return
	}

	available := d.launcher.AvailableAdapters()
	if len(available) == 0 {
		slog.Debug("orchestrator: no adapters available for auto-launch")
		return
	}

	t, ok := d.taskStore.Get(entry.TaskID)
	if !ok {
		return
	}

	workDir := d.resolveWorkDir(t)
	projectID := d.resolveProjectID(t)

	// Try to reuse an existing persistent session for the same project
	if existing := d.launcher.FindSessionForProject(projectID, workDir); existing != nil {
		slog.Info("orchestrator: reusing persistent session for project",
			"task", entry.TaskID, "session", existing.SessionID, "adapter", existing.AdapterName)
		d.emitEvent(EventWorkerLaunched, entry.TaskID,
			"reusing existing session "+existing.SessionID,
			map[string]any{"adapter": existing.AdapterName, "reused": true})
		return
	}

	lt := LaunchTask{
		TaskID:      entry.TaskID,
		ProjectID:   projectID,
		Description: t.Description,
		WorkDir:     workDir,
		MCPEndpoint: "http://localhost:8765/mcp/v1",
	}

	adapterName := available[0]
	result, err := d.launcher.Launch(ctx, adapterName, lt)
	if err != nil {
		slog.Warn("orchestrator: auto-launch failed",
			"task", entry.TaskID, "adapter", adapterName, "err", err)
		d.recordFailure("auto-launch: " + err.Error())
		d.emitEvent(EventWorkerFailed, entry.TaskID, "auto-launch failed: "+err.Error(),
			map[string]any{"adapter": adapterName})
		return
	}

	d.recordSuccess()
	slog.Info("orchestrator: auto-launched worker",
		"task", entry.TaskID, "adapter", adapterName, "session", result.SessionID)
	d.emitEvent(EventWorkerLaunched, entry.TaskID, "worker launched via "+adapterName,
		map[string]any{"adapter": adapterName, "session_id": result.SessionID})
	d.sendNotify("info", "Worker 已启动",
		fmt.Sprintf("任务 %s 已通过 %s 启动", entry.TaskID, adapterName))
}

func (d *Daemon) resolveProjectID(t *task.Task) string {
	if t.Constraints != nil && t.Constraints.Extra != nil {
		if id, ok := t.Constraints.Extra["project_id"].(string); ok {
			return id
		}
	}
	return ""
}

func (d *Daemon) resolveWorkDir(t *task.Task) string {
	if d.projects != nil && t.Constraints != nil && t.Constraints.Extra != nil {
		if projID, ok := t.Constraints.Extra["project_id"].(string); ok {
			if p, found := d.projects.Get(projID); found {
				return p.RepoPath
			}
		}
	}
	return "."
}

func (d *Daemon) checkTimedOutTasks(_ context.Context) {
	if d.dispatcher == nil {
		return
	}
	timedOut := d.dispatcher.CheckTimeouts()
	for _, taskID := range timedOut {
		if !d.policy.AllowAutoRequeue {
			slog.Warn("orchestrator: task timed out but auto-requeue disabled", "task", taskID)
			continue
		}
		slog.Warn("orchestrator: task timed out, requeueing", "task", taskID)
		if err := d.dispatcher.Requeue(taskID); err != nil {
			slog.Error("orchestrator: requeue failed", "task", taskID, "err", err)
		}
	}
}

func (d *Daemon) checkCompletedResults(ctx context.Context) {
	if d.taskStore == nil {
		return
	}

	tasks := d.taskStore.List("", 50)
	for _, t := range tasks {
		if ctx.Err() != nil {
			return
		}
		if !d.needsReview(t) {
			continue
		}

		workerResult := extractWorkerResult(t)
		if workerResult == "" {
			continue
		}

		if d.taskAutoApproved(t) {
			slog.Info("orchestrator: task auto-approved (skip review)", "task", t.ID)
			d.markReviewed(t, &ReviewResult{Approved: true, Score: 8, Suggestions: []string{"auto-approved by policy"}})
			d.recordSuccess()
			continue
		}

		risk := d.taskRiskLevel(t)

		// Low risk: approve immediately, queue async review in background
		if risk == task.RiskLow {
			slog.Info("orchestrator: low-risk task, async review", "task", t.ID)
			d.markReviewed(t, &ReviewResult{Approved: true, Score: 7, Suggestions: []string{"low-risk: async review pending"}})
			d.recordSuccess()
			if d.reviewer != nil && d.policy.AllowAutoReview {
				go d.asyncReview(context.Background(), t, workerResult)
			}
			continue
		}

		if !d.policy.AllowAutoReview {
			slog.Debug("orchestrator: auto-review disabled by policy", "task", t.ID)
			continue
		}

		if d.reviewer == nil {
			d.markReviewed(t, &ReviewResult{Approved: true, Score: 7})
			continue
		}

		result, err := d.reviewer.Review(ctx, t, workerResult)
		if err != nil {
			slog.Warn("orchestrator: review failed", "task", t.ID, "err", err)
			continue
		}

		requireApproval := d.policy.RequireApproval || risk == task.RiskHigh

		if result.Approved {
			d.recordSuccess()
			meta := map[string]any{"score": result.Score, "risk": string(risk)}
			if result.TestPassed != nil {
				meta["test_passed"] = *result.TestPassed
			}
			if requireApproval {
				slog.Info("orchestrator: review passed but human approval required", "task", t.ID, "score", result.Score, "risk", risk)
				d.markPendingApproval(t, result)
				d.emitEvent(EventPendingApproval, t.ID, "review passed, awaiting human approval", meta)
				d.sendNotify("info", "待人工审批",
					fmt.Sprintf("任务 %s 审查通过(评分 %d, 风险 %s)，等待人工确认", t.ID, result.Score, risk))
			} else {
				slog.Info("orchestrator: review approved", "task", t.ID, "score", result.Score)
				d.markReviewed(t, result)
				d.emitEvent(EventReviewApproved, t.ID, fmt.Sprintf("approved with score %d", result.Score), meta)
				d.sendNotify("info", "任务完成",
					fmt.Sprintf("任务 %s 审查通过，评分 %d/10", t.ID, result.Score))
			}
		} else {
			d.recordFailure("review rejected: " + t.ID)
			slog.Info("orchestrator: review rejected, requeueing", "task", t.ID, "issues", result.Issues)
			d.rejectAndRequeue(t, result)
			d.emitEvent(EventReviewRejected, t.ID, fmt.Sprintf("rejected with score %d", result.Score),
				map[string]any{"score": result.Score, "issues": result.Issues})
			d.sendNotify("warn", "任务被打回",
				fmt.Sprintf("任务 %s 审查未通过(评分 %d)，已重新排队。问题: %v", t.ID, result.Score, result.Issues))
		}
	}
}

func (d *Daemon) needsReview(t *task.Task) bool {
	if t.Constraints == nil || t.Constraints.Extra == nil {
		return false
	}
	status, _ := t.Constraints.Extra["dispatch_status"].(string)
	return status == "submitted"
}

func extractWorkerResult(t *task.Task) string {
	if t.Constraints == nil || t.Constraints.Extra == nil {
		return ""
	}
	r, _ := t.Constraints.Extra["worker_result"].(string)
	return r
}

func (d *Daemon) markReviewed(t *task.Task, result *ReviewResult) {
	if t.Constraints == nil {
		t.Constraints = &task.TaskConstraints{}
	}
	if t.Constraints.Extra == nil {
		t.Constraints.Extra = make(map[string]any)
	}
	t.Constraints.Extra["dispatch_status"] = "reviewed"
	t.Constraints.Extra["review_score"] = result.Score
	t.Constraints.Extra["review_approved"] = true
	d.taskStore.Update(t)
}

// taskAutoApproved checks the per-task AutoApprove flag.
func (d *Daemon) taskAutoApproved(t *task.Task) bool {
	return t.Constraints != nil && t.Constraints.AutoApprove
}

func (d *Daemon) taskRiskLevel(t *task.Task) task.RiskLevel {
	if t.Constraints != nil && t.Constraints.RiskLevel != "" {
		return t.Constraints.RiskLevel
	}
	return task.RiskMedium
}

// asyncReview runs a review in the background for low-risk tasks.
// If the review finds issues, it downgrades the task status.
func (d *Daemon) asyncReview(ctx context.Context, t *task.Task, workerResult string) {
	result, err := d.reviewer.Review(ctx, t, workerResult)
	if err != nil {
		slog.Warn("orchestrator: async review failed", "task", t.ID, "err", err)
		return
	}
	if !result.Approved {
		slog.Warn("orchestrator: async review found issues in low-risk task", "task", t.ID, "issues", result.Issues)
		d.sendNotify("warn", "低风险任务审查异常",
			fmt.Sprintf("任务 %s 事后审查发现问题(评分 %d): %v", t.ID, result.Score, result.Issues))
	}
}

func (d *Daemon) markPendingApproval(t *task.Task, result *ReviewResult) {
	if t.Constraints == nil {
		t.Constraints = &task.TaskConstraints{}
	}
	if t.Constraints.Extra == nil {
		t.Constraints.Extra = make(map[string]any)
	}
	t.Constraints.Extra["dispatch_status"] = "pending_approval"
	t.Constraints.Extra["review_score"] = result.Score
	t.Constraints.Extra["review_approved"] = true
	d.taskStore.Update(t)
}

func (d *Daemon) rejectAndRequeue(t *task.Task, result *ReviewResult) {
	if t.Constraints == nil {
		t.Constraints = &task.TaskConstraints{}
	}
	if t.Constraints.Extra == nil {
		t.Constraints.Extra = make(map[string]any)
	}
	t.Constraints.Extra["dispatch_status"] = "rejected"
	t.Constraints.Extra["review_issues"] = result.Issues
	t.Constraints.Extra["review_suggestions"] = result.Suggestions
	d.taskStore.Update(t)

	if d.dispatcher != nil {
		d.dispatcher.Requeue(t.ID)
	}
}
