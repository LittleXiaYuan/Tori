package task

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"yunque-agent/pkg/skills"
)

// ──────────────────────────────────────────────
// Context-based cost tagging for task LLM calls
// ──────────────────────────────────────────────

type taskCostKey struct{}

// TaskCostContext carries cost attribution through context.
type TaskCostContext struct {
	TaskID    string
	StepID    string // "step-{N}" format
	SkillName string
}

// WithTaskCost injects cost attribution into context.
func WithTaskCost(ctx context.Context, c *TaskCostContext) context.Context {
	return context.WithValue(ctx, taskCostKey{}, c)
}

// TaskCostFromContext extracts cost attribution from context.
func TaskCostFromContext(ctx context.Context) *TaskCostContext {
	v, _ := ctx.Value(taskCostKey{}).(*TaskCostContext)
	return v
}

// ──────────────────────────────────────────────
// Runner — executes a Task by planning with LLM then running steps
//
// Flow: Task.Description → LLM plans steps → execute each step
// via skill registry → collect artifacts → mark complete/failed
//
// Enhancements:
//   - Step chaining: step N result auto-feeds step N+1 as input context
//   - Failure retry: each step auto-retries up to MaxRetries (default 2)
//   - Cancellation: Cancel(taskID) aborts a running task
// ──────────────────────────────────────────────

// LLMFunc calls the LLM with messages and returns the response.
type LLMFunc func(ctx context.Context, system, user string) (string, error)

// Runner executes tasks by planning and running steps.
type Runner struct {
	store     Store
	registry  *skills.Registry
	llmCall   LLMFunc
	env       *skills.Environment // base environment for skill execution
	gap       *GapAnalyzer        // optional: capability gap detection
	generator *SkillGenerator     // optional: capability auto-generation
	lifecycle *LifecycleManager   // unified lifecycle manager (new)

	mu        sync.Mutex
	running   map[string]context.CancelFunc // taskID → cancel (for in-flight tasks)
	paused    map[string]bool               // taskID → true if pause requested
	listeners []func(event, taskID, detail string)

	// CostTag is called before each LLM/skill invocation so callers can
	// associate the cost with a task_id and skill_name.
	// signature: func(taskID, skillName string)
	CostTag func(taskID, skillName string)

	// WorkMem manages per-task working memory for context compression.
	WorkMem *WorkingMemoryManager
}

// NewRunner creates a task runner.
func NewRunner(store Store, registry *skills.Registry, llmCall LLMFunc, env *skills.Environment) *Runner {
	r := &Runner{
		store:    store,
		registry: registry,
		llmCall:  llmCall,
		env:      env,
		running:  make(map[string]context.CancelFunc),
		paused:   make(map[string]bool),
	}
	// Initialize lifecycle manager
	r.lifecycle = NewLifecycleManager(store)
	return r
}

// SetGapAnalyzer attaches a capability gap analyzer.
func (r *Runner) SetGapAnalyzer(g *GapAnalyzer) {
	r.gap = g
}

// SetSkillGenerator attaches a capability generator for auto-growth.
func (r *Runner) SetSkillGenerator(g *SkillGenerator) {
	r.generator = g
}

// OnTaskEvent registers a callback for task lifecycle events.
func (r *Runner) OnTaskEvent(fn func(event, taskID, detail string)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.listeners = append(r.listeners, fn)
}

func (r *Runner) emit(event, taskID, detail string) {
	r.mu.Lock()
	ls := make([]func(string, string, string), len(r.listeners))
	copy(ls, r.listeners)
	r.mu.Unlock()
	for _, fn := range ls {
		fn(event, taskID, detail)
	}
}

// Cancel aborts a running task. Returns false if the task is not running.
func (r *Runner) Cancel(taskID string) bool {
	r.mu.Lock()
	cancel, ok := r.running[taskID]
	r.mu.Unlock()
	if !ok {
		return false
	}
	cancel()
	return true
}

// Pause requests a running task to pause after the current step completes.
// Returns false if the task is not running.
func (r *Runner) Pause(taskID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.running[taskID]; !ok {
		return false
	}
	r.paused[taskID] = true
	return true
}

// Resume restarts a paused, interrupted, or failed task from where it left off.
// Completed steps are skipped. Returns an error if the task cannot be resumed.
func (r *Runner) Resume(ctx context.Context, taskID string) error {
	t, ok := r.store.Get(taskID)
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}
	if !t.IsResumable() {
		return fmt.Errorf("task %s in state %s is not resumable", taskID, t.Status)
	}
	// Reset failed steps to pending for retry
	for i := range t.Steps {
		if t.Steps[i].Status == StepFailed {
			t.Steps[i].Status = StepPending
			t.Steps[i].Error = ""
			t.Steps[i].RetryCount = 0
		}
	}
	t.Status = StatusPending
	t.Error = ""
	t.FinishedAt = nil
	r.store.Update(t)
	r.emit("task_resumed", taskID, fmt.Sprintf("resumed from %s", t.Status))

	return r.Run(ctx, taskID)
}

// Restart resets a terminal task completely and runs it again.
// Unlike Resume, this re-plans from scratch.
func (r *Runner) Restart(ctx context.Context, taskID string) error {
	t, ok := r.store.Get(taskID)
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}
	if !t.IsTerminal() && !t.IsResumable() {
		return fmt.Errorf("task %s in state %s cannot be restarted", taskID, t.Status)
	}
	// Full reset
	t.Steps = nil
	t.Status = StatusPending
	t.Error = ""
	t.StartedAt = nil
	t.FinishedAt = nil
	t.Artifacts = nil
	r.store.Update(t)
	r.emit("task_restarted", taskID, "full restart")

	return r.Run(ctx, taskID)
}

// RecoverAll marks interrupted tasks on startup. Call once after creating Runner.
func (r *Runner) RecoverAll() int {
	return r.store.RecoverInterrupted()
}

// IsRunning returns true if the task is currently being executed.
func (r *Runner) IsRunning(taskID string) bool {
	r.mu.Lock()
	_, ok := r.running[taskID]
	r.mu.Unlock()
	return ok
}

// Run plans and executes a task. Blocks until completion, failure, or cancellation.
func (r *Runner) Run(ctx context.Context, taskID string) error {
	t, ok := r.store.Get(taskID)
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}
	if t.IsTerminal() {
		return fmt.Errorf("task %s already in terminal state: %s", taskID, t.Status)
	}

	// Create cancellable context and register it
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Apply task-level timeout if specified
	if t.Constraints != nil && t.Constraints.TimeoutSec > 0 {
		var timeoutCancel context.CancelFunc
		ctx, timeoutCancel = context.WithTimeout(ctx, time.Duration(t.Constraints.TimeoutSec)*time.Second)
		defer timeoutCancel()
	}

	r.mu.Lock()
	r.running[taskID] = cancel
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		delete(r.running, taskID)
		r.mu.Unlock()
	}()

	// Phase 1: Plan (if no steps yet)
	if len(t.Steps) == 0 {
		// Use lifecycle for state transition
		if err := r.lifecycle.TransitionTo(ctx, taskID, StatusPlanning); err != nil {
			return err
		}

		steps, err := r.plan(ctx, t)
		if err != nil {
			if ctx.Err() != nil {
				return r.markCancelled(t)
			}
			// Use lifecycle for state transition
			r.lifecycle.TransitionTo(ctx, taskID, StatusFailed)
			// Update error message (lifecycle doesn't handle this)
			t, _ = r.store.Get(taskID)
			t.Error = fmt.Sprintf("planning failed: %v", err)
			r.store.Update(t)
			return err
		}
		// Reload task after lifecycle update
		t, _ = r.store.Get(taskID)
		t.Steps = steps
		r.store.Update(t)
	}

	// Phase 2: Execute steps — parallel groups run concurrently
	// Use lifecycle for state transition
	if err := r.lifecycle.TransitionTo(ctx, taskID, StatusRunning); err != nil {
		return err
	}

	// Reload task after lifecycle update to pick up the new status
	t, _ = r.store.Get(taskID)

	// Initialize working memory for this task
	if r.WorkMem != nil {
		r.WorkMem.Init(t)
	}

	var prevResult string // chain: previous group's merged result

	groups := groupSteps(t.Steps)
	for _, grp := range groups {
		// Check cancellation before each group
		if ctx.Err() != nil {
			return r.markCancelled(t)
		}

		// Check pause before each group
		r.mu.Lock()
		shouldPause := r.paused[taskID]
		if shouldPause {
			delete(r.paused, taskID)
		}
		r.mu.Unlock()
		if shouldPause {
			return r.markPaused(t)
		}

		if len(grp) == 1 {
			// Single step — execute directly (original sequential logic)
			idx := grp[0]
			// Reload task to get latest state
			t, _ = r.store.Get(taskID)
			step := &t.Steps[idx]
			if step.Status == StepDone || step.Status == StepSkipped {
				prevResult = step.Result
				continue
			}
			if missing := unmetTaskStepDependencies(t, step); len(missing) > 0 {
				return r.markDependencyBlocked(t, step, missing)
			}
			if prevResult != "" {
				step.Input = prevResult
				r.store.Update(t)
			}
			if step.MaxRetries == 0 {
				step.MaxRetries = DefaultMaxRetries
			}

			slog.Info("task: executing step", "task", taskID, "step", step.ID, "action", step.Action)

			// Use lifecycle for step start
			if err := r.lifecycle.OnStepStart(ctx, taskID, idx); err != nil {
				return err
			}

			result, err := r.executeStepWithRetry(ctx, t, step)
			if err != nil {
				if ctx.Err() != nil {
					return r.markCancelled(t)
				}
				// Use lifecycle for step failure
				r.lifecycle.OnStepFailed(ctx, taskID, idx, err)

				if failErr := r.handleStepFailure(ctx, t, step, err); failErr != nil {
					return failErr
				}
				// If handleStepFailure resolved it (growth loop), continue
				// Reload task to check status
				t, _ = r.store.Get(taskID)
				if t.Steps[idx].Status == StepDone {
					prevResult = t.Steps[idx].Result
					continue
				}
				return err
			}

			// Use lifecycle for step complete
			if err := r.lifecycle.OnStepComplete(ctx, taskID, idx, result); err != nil {
				return err
			}

			r.emit("step_completed", taskID, fmt.Sprintf("step %d: %s", step.ID, step.Action))
			if r.WorkMem != nil {
				// Reload task after lifecycle update
				t, _ = r.store.Get(taskID)
				r.WorkMem.UpdateAfterStep(t, &t.Steps[idx])
			}
			prevResult = result

		} else {
			// Parallel group — execute concurrently
			// Worker goroutines only call executeStep (read-only on task state).
			// All step state mutations happen on the main goroutine after collection.
			type stepResult struct {
				idx    int
				result string
				err    error
			}

			// Use a cancellable sub-context so first-failure cancels remaining parallel steps
			grpCtx, grpCancel := context.WithCancel(ctx)
			defer grpCancel()

			resultCh := make(chan stepResult, len(grp))
			for _, idx := range grp {
				step := &t.Steps[idx]
				if step.Status == StepDone || step.Status == StepSkipped {
					continue
				}
				if missing := unmetTaskStepDependencies(t, step); len(missing) > 0 {
					return r.markDependencyBlocked(t, step, missing)
				}
			}
			for _, idx := range grp {
				step := &t.Steps[idx]
				if step.Status == StepDone || step.Status == StepSkipped {
					resultCh <- stepResult{idx: idx, result: step.Result}
					continue
				}
				if prevResult != "" {
					step.Input = prevResult
				}
				if step.MaxRetries == 0 {
					step.MaxRetries = DefaultMaxRetries
				}

				go func(stepIdx int) {
					s := &t.Steps[stepIdx]
					slog.Info("task: executing parallel step", "task", taskID, "step", s.ID, "action", s.Action)
					res, err := r.executeParallelStep(grpCtx, t, s)
					resultCh <- stepResult{idx: stepIdx, result: res, err: err}
				}(idx)
			}

			// Collect results — all step state writes on main goroutine
			var mergedResults []string
			var firstErr error
			for range grp {
				sr := <-resultCh
				step := &t.Steps[sr.idx]
				if sr.err != nil {
					if firstErr == nil {
						firstErr = sr.err
					}
					stepDone := time.Now()
					step.DoneAt = &stepDone
					step.Status = StepFailed
					step.Error = sr.err.Error()
					// Gap analysis
					if r.gap != nil {
						gapRec := r.gap.Analyze(ctx, t, step)
						step.GapType = string(gapRec.GapType)
					}
				} else {
					step.Status = StepDone
					step.Result = sr.result
					stepDone := time.Now()
					step.DoneAt = &stepDone
					r.emit("step_completed", taskID, fmt.Sprintf("step %d: %s", step.ID, step.Action))
					if r.WorkMem != nil {
						r.WorkMem.UpdateAfterStep(t, step)
					}
					mergedResults = append(mergedResults, sr.result)
				}
			}
			r.store.Update(t)

			if firstErr != nil {
				t.Status = StatusFailed
				t.Error = fmt.Sprintf("parallel group failed: %v", firstErr)
				errNow := time.Now()
				t.FinishedAt = &errNow
				r.store.Update(t)
				r.emit("task_failed", taskID, t.Error)
				return firstErr
			}

			prevResult = strings.Join(mergedResults, "\n---\n")
		}
	}

	// Phase 3: Complete
	// Use lifecycle for state transition
	if err := r.lifecycle.TransitionTo(ctx, taskID, StatusCompleted); err != nil {
		return err
	}

	r.emit("task_completed", taskID, fmt.Sprintf("%d steps", len(t.Steps)))
	slog.Info("task: completed", "task", taskID, "steps", len(t.Steps))
	return nil
}

func unmetTaskStepDependencies(t *Task, step *Step) []int {
	if step == nil || len(step.DependsOn) == 0 {
		return nil
	}
	byID := make(map[int]StepStatus, len(t.Steps))
	for _, s := range t.Steps {
		byID[s.ID] = s.Status
	}
	var missing []int
	for _, depID := range step.DependsOn {
		status, ok := byID[depID]
		if !ok || (status != StepDone && status != StepSkipped) {
			missing = append(missing, depID)
		}
	}
	return missing
}

func (r *Runner) markDependencyBlocked(t *Task, step *Step, missing []int) error {
	if t == nil || step == nil {
		return fmt.Errorf("task dependency blocked")
	}
	t.Status = StatusInterrupted
	t.Error = fmt.Sprintf("步骤 %d 等待依赖步骤完成：%s", step.ID, formatStepIDs(missing))
	step.Status = StepPending
	r.store.Update(t)
	r.emit("task_interrupted", t.ID, t.Error)
	slog.Info("task: dependency blocked", "task", t.ID, "step", step.ID, "missing", missing)
	return fmt.Errorf("%s", t.Error)
}

func formatStepIDs(ids []int) string {
	if len(ids) == 0 {
		return ""
	}
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, fmt.Sprintf("%d", id))
	}
	return strings.Join(parts, ", ")
}

// markCancelled sets task status to cancelled.
func (r *Runner) markCancelled(t *Task) error {
	now := time.Now()
	t.Status = StatusCancelled
	t.Error = "cancelled by user"
	t.FinishedAt = &now
	// Mark any running/pending steps as skipped
	for i := range t.Steps {
		if t.Steps[i].Status == StepRunning || t.Steps[i].Status == StepPending || t.Steps[i].Status == StepRetrying {
			t.Steps[i].Status = StepSkipped
		}
	}
	r.store.Update(t)
	slog.Info("task: cancelled", "task", t.ID)
	return fmt.Errorf("task %s cancelled", t.ID)
}

// markPaused sets task status to paused, preserving step progress.
func (r *Runner) markPaused(t *Task) error {
	t.Status = StatusPaused
	t.Error = ""
	// Mark any running steps back to pending
	for i := range t.Steps {
		if t.Steps[i].Status == StepRunning || t.Steps[i].Status == StepRetrying {
			t.Steps[i].Status = StepPending
		}
	}
	r.store.Update(t)
	r.emit("task_paused", t.ID, "paused by user")
	slog.Info("task: paused", "task", t.ID)
	return fmt.Errorf("task %s paused", t.ID)
}
