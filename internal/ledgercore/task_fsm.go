package ledger

import "fmt"

// validTransitions defines the set of legal state transitions.
// Key: from status, Value: set of allowed target statuses.
var validTransitions = map[TaskStatus]map[TaskStatus]bool{
	TaskCreated: {
		TaskReady:     true,
		TaskCancelled: true,
	},
	TaskReady: {
		TaskRunning:   true,
		TaskCancelled: true,
	},
	TaskRunning: {
		TaskCompleted:    true,
		TaskFailed:       true,
		TaskWaitingInput: true,
		TaskBlocked:      true,
		TaskRetrying:     true,
		TaskCancelled:    true,
	},
	TaskWaitingInput: {
		TaskRunning:   true,
		TaskCancelled: true,
	},
	TaskBlocked: {
		TaskReady:     true,
		TaskCancelled: true,
	},
	TaskRetrying: {
		TaskRunning: true,
		TaskFailed:  true,
	},
	// Terminal states can be reopened/restarted.
	TaskFailed: {
		TaskReady: true, // restart
	},
	TaskCancelled: {
		TaskReady: true, // reopen
	},
	// TaskCompleted has no outgoing transitions.
	TaskCompleted: {},
}

// ValidateTransition checks whether a state transition from → to is legal.
// Returns nil if valid, or a *TransitionError if not.
func ValidateTransition(taskID string, from, to TaskStatus) error {
	targets, ok := validTransitions[from]
	if !ok {
		return &TransitionError{TaskID: taskID, From: from, To: to}
	}
	if !targets[to] {
		return &TransitionError{TaskID: taskID, From: from, To: to}
	}
	return nil
}

// TransitionActor classifies who is allowed to trigger each transition.
type TransitionActor string

const (
	ActorRuntime TransitionActor = "runtime"
	ActorUser    TransitionActor = "user"
)

// transitionActors maps (from, to) → required actor.
var transitionActors = map[[2]TaskStatus]TransitionActor{
	// Runtime-driven
	{TaskCreated, TaskReady}:         ActorRuntime,
	{TaskReady, TaskRunning}:         ActorRuntime,
	{TaskRunning, TaskCompleted}:     ActorRuntime,
	{TaskRunning, TaskFailed}:        ActorRuntime,
	{TaskRunning, TaskWaitingInput}:  ActorRuntime,
	{TaskRunning, TaskBlocked}:       ActorRuntime,
	{TaskRunning, TaskRetrying}:      ActorRuntime,
	{TaskBlocked, TaskReady}:         ActorRuntime,
	{TaskRetrying, TaskRunning}:      ActorRuntime,
	{TaskRetrying, TaskFailed}:       ActorRuntime,

	// User-driven
	{TaskCreated, TaskCancelled}:     ActorUser,
	{TaskReady, TaskCancelled}:       ActorUser,
	{TaskRunning, TaskCancelled}:     ActorUser,
	{TaskWaitingInput, TaskRunning}:  ActorUser, // input received
	{TaskWaitingInput, TaskCancelled}: ActorUser,
	{TaskBlocked, TaskCancelled}:     ActorUser,
	{TaskFailed, TaskReady}:          ActorUser, // restart
	{TaskCancelled, TaskReady}:       ActorUser, // reopen
}

// TransitionActorFor returns the expected actor for a given transition.
func TransitionActorFor(from, to TaskStatus) TransitionActor {
	if actor, ok := transitionActors[[2]TaskStatus{from, to}]; ok {
		return actor
	}
	return ActorRuntime
}

// transitionEventKind maps transitions to their corresponding event kinds.
var transitionEventKind = map[[2]TaskStatus]EventKind{
	{TaskCreated, TaskReady}:         EventTaskReady,
	{TaskReady, TaskRunning}:         EventTaskStarted,
	{TaskRunning, TaskCompleted}:     EventTaskCompleted,
	{TaskRunning, TaskFailed}:        EventTaskFailed,
	{TaskRunning, TaskCancelled}:     EventTaskCancelled,
	{TaskRunning, TaskWaitingInput}:  EventTaskWaitingInput,
	{TaskRunning, TaskBlocked}:       EventTaskBlocked,
	{TaskRunning, TaskRetrying}:      EventTaskRetrying,
	{TaskWaitingInput, TaskRunning}:  EventTaskInputReceived,
	{TaskWaitingInput, TaskCancelled}: EventTaskCancelled,
	{TaskBlocked, TaskReady}:         EventTaskResumed,
	{TaskBlocked, TaskCancelled}:     EventTaskCancelled,
	{TaskRetrying, TaskRunning}:      EventTaskResumed,
	{TaskRetrying, TaskFailed}:       EventTaskFailed,
	{TaskFailed, TaskReady}:          EventTaskResumed,
	{TaskCancelled, TaskReady}:       EventTaskResumed,
	{TaskCreated, TaskCancelled}:     EventTaskCancelled,
	{TaskReady, TaskCancelled}:       EventTaskCancelled,
}

// EventKindForTransition returns the event kind that should be emitted
// for a given state transition.
func EventKindForTransition(from, to TaskStatus) (EventKind, error) {
	if kind, ok := transitionEventKind[[2]TaskStatus{from, to}]; ok {
		return kind, nil
	}
	return "", fmt.Errorf("ledger: no event kind defined for transition %s -> %s", from, to)
}
