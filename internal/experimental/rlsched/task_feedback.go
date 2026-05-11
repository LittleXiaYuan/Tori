package rlsched

import (
	"strings"
	"time"

	"yunque-agent/internal/agentcore/task"
)

const (
	EventTaskCompleted = "task_completed"
	EventTaskFailed    = "task_failed"
)

// TaskFeedback converts task lifecycle events into Q-Learning transitions.
// It closes the runtime loop described by the roadmap:
// task state + chosen scheduling action + outcome reward → QLearner.Update.
type TaskFeedback struct {
	Learner *QLearner
	Store   task.Store
	Now     func() time.Time
}

func NewTaskFeedback(learner *QLearner, store task.Store) *TaskFeedback {
	return &TaskFeedback{
		Learner: learner,
		Store:   store,
		Now:     time.Now,
	}
}

func (tf *TaskFeedback) OnTaskEvent(event, taskID, _ string) {
	_ = tf.Record(event, taskID)
}

func (tf *TaskFeedback) Record(event, taskID string) bool {
	if tf == nil || tf.Learner == nil || tf.Store == nil {
		return false
	}
	if event != EventTaskCompleted && event != EventTaskFailed {
		return false
	}
	t, ok := tf.Store.Get(taskID)
	if !ok || t == nil {
		return false
	}

	now := time.Now()
	if tf.Now != nil {
		now = tf.Now()
	}
	state := EncodeTaskState(tf.Store, t, now)
	action := TaskSchedulingAction(t)
	reward := TaskOutcomeReward(event, t, now)
	nextState := EncodeTaskState(tf.Store, t, now.Add(time.Second))
	tf.Learner.Update(state, action, reward, nextState)
	return true
}

func EncodeTaskState(store task.Store, t *task.Task, now time.Time) string {
	if t == nil {
		return "queue=unknown|priority=normal|steps=unknown|hour=unknown"
	}
	return strings.Join([]string{
		"queue=" + queueBucket(store, t.TenantID),
		"priority=" + priorityBucket(t),
		"steps=" + stepBucket(len(t.Steps)),
		"hour=" + hourBucket(now.Hour()),
	}, "|")
}

func TaskSchedulingAction(t *task.Task) string {
	switch priorityBucket(t) {
	case "high":
		return "priority_high"
	case "low":
		return "priority_low"
	default:
		return "priority_normal"
	}
}

func TaskOutcomeReward(event string, t *task.Task, now time.Time) float64 {
	if event == EventTaskFailed {
		return -0.2
	}
	reward := 0.75
	retries := 0
	failedSteps := 0
	for _, step := range t.Steps {
		retries += step.RetryCount
		if step.Status == task.StepFailed {
			failedSteps++
		}
	}
	if retries == 0 && failedSteps == 0 {
		reward += 0.15
	}
	if t.StartedAt != nil {
		elapsed := now.Sub(*t.StartedAt)
		switch {
		case elapsed <= 5*time.Minute:
			reward += 0.10
		case elapsed > time.Hour:
			reward -= 0.20
		case elapsed > 30*time.Minute:
			reward -= 0.10
		}
	}
	reward -= float64(retries) * 0.05
	if reward > 1 {
		return 1
	}
	if reward < -1 {
		return -1
	}
	return reward
}

func queueBucket(store task.Store, tenantID string) string {
	if store == nil {
		return "unknown"
	}
	pending := 0
	for _, t := range store.List(tenantID, 200) {
		if t != nil && !t.IsTerminal() {
			pending++
		}
	}
	switch {
	case pending == 0:
		return "empty"
	case pending <= 3:
		return "small"
	case pending <= 10:
		return "medium"
	default:
		return "large"
	}
}

func priorityBucket(t *task.Task) string {
	if t == nil || t.Constraints == nil {
		return "normal"
	}
	switch strings.ToLower(strings.TrimSpace(t.Constraints.Priority)) {
	case "high", "urgent", "p0", "p1":
		return "high"
	case "low", "defer", "p3", "p4":
		return "low"
	default:
		return "normal"
	}
}

func stepBucket(n int) string {
	switch {
	case n <= 0:
		return "unknown"
	case n <= 3:
		return "small"
	case n <= 8:
		return "medium"
	default:
		return "large"
	}
}

func hourBucket(hour int) string {
	switch {
	case hour >= 9 && hour < 18:
		return "workday"
	case hour >= 18 && hour < 23:
		return "evening"
	default:
		return "night"
	}
}
