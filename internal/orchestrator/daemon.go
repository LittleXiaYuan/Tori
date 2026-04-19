package orchestrator

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"yunque-agent/internal/agentcore/task"
	mcpserver "yunque-agent/internal/mcp/server"
)

type Daemon struct {
	taskStore  task.Store
	dispatcher *task.Dispatcher
	workers    *mcpserver.WorkerRegistry
	launcher   *Launcher
	reviewer   *ReviewAgent
	projects   *ProjectStore
	interval   time.Duration

	mu      sync.Mutex
	running bool
	cancel  context.CancelFunc
}

type DaemonConfig struct {
	TaskStore  task.Store
	Dispatcher *task.Dispatcher
	Workers    *mcpserver.WorkerRegistry
	Launcher   *Launcher
	Reviewer   *ReviewAgent
	Projects   *ProjectStore
	Interval   time.Duration
}

func NewDaemon(cfg DaemonConfig) *Daemon {
	if cfg.Interval == 0 {
		cfg.Interval = 5 * time.Second
	}
	return &Daemon{
		taskStore:  cfg.TaskStore,
		dispatcher: cfg.Dispatcher,
		workers:    cfg.Workers,
		launcher:   cfg.Launcher,
		reviewer:   cfg.Reviewer,
		projects:   cfg.Projects,
		interval:   cfg.Interval,
	}
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
		}
	}()

	d.checkPendingTasks(ctx)
	d.checkTimedOutTasks(ctx)
	d.checkCompletedResults(ctx)
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

		if d.workers != nil {
			candidates := d.workers.FindByCapability(entry.RequiredCaps[0])
			if len(candidates) > 0 {
				continue
			}
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

	lt := LaunchTask{
		TaskID:      entry.TaskID,
		Description: t.Description,
		WorkDir:     workDir,
		MCPEndpoint: "http://localhost:8765/mcp/v1",
	}

	adapterName := available[0]
	result, err := d.launcher.Launch(ctx, adapterName, lt)
	if err != nil {
		slog.Warn("orchestrator: auto-launch failed",
			"task", entry.TaskID, "adapter", adapterName, "err", err)
		return
	}

	slog.Info("orchestrator: auto-launched worker",
		"task", entry.TaskID, "adapter", adapterName, "session", result.SessionID)
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
		slog.Warn("orchestrator: task timed out, requeueing", "task", taskID)
		if err := d.dispatcher.Requeue(taskID); err != nil {
			slog.Error("orchestrator: requeue failed", "task", taskID, "err", err)
		}
	}
}

func (d *Daemon) checkCompletedResults(ctx context.Context) {
	if d.reviewer == nil || d.taskStore == nil {
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

		result, err := d.reviewer.Review(ctx, t, workerResult)
		if err != nil {
			slog.Warn("orchestrator: review failed", "task", t.ID, "err", err)
			continue
		}

		if result.Approved {
			slog.Info("orchestrator: review approved", "task", t.ID, "score", result.Score)
			d.markReviewed(t, result)
		} else {
			slog.Info("orchestrator: review rejected, requeueing", "task", t.ID, "issues", result.Issues)
			d.rejectAndRequeue(t, result)
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
