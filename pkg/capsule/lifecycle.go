package capsule

import (
	"fmt"
	"sync"
	"time"
)

// State represents a Capsule's lifecycle state. States flow forward (install →
// enable → activate) and backward (suspend → disable → uninstall). Failure is
// a terminal branch reachable from any running state.
//
//	┌──────────────┐
//	│  Registered  │                  (known to the registry, no artifacts yet)
//	└──────┬───────┘
//	       │ Install()
//	       ▼
//	┌──────────────┐
//	│  Installing  │
//	└──────┬───────┘
//	       │ (artifacts ready)
//	       ▼
//	┌──────────────┐     Disable()/Uninstall()
//	│  Installed   │◄──────────────────────────┐
//	└──────┬───────┘                            │
//	       │ Enable()                           │
//	       ▼                                    │
//	┌──────────────┐                            │
//	│   Enabled    │──Disable()─────────────────┤
//	└──────┬───────┘                            │
//	       │ Activate()  (Runtime.Start)        │
//	       ▼                                    │
//	┌──────────────┐   Suspend()  ┌─────────────┴┐
//	│  Activated   │─────────────►│  Suspended   │
//	└──────┬───────┘◄─ Resume()──┴─────┬────────┘
//	       │                            │
//	       └── on error ──► Failed ◄────┘
type State string

const (
	StateRegistered   State = "registered"
	StateInstalling   State = "installing"
	StateInstalled    State = "installed"
	StateEnabled      State = "enabled"
	StateActivated    State = "activated"
	StateSuspended    State = "suspended"
	StateFailed       State = "failed"
	StateUninstalling State = "uninstalling"
)

// validTransitions declares the legal forward/backward moves between states.
// A transition not listed here is rejected by Transition().
var validTransitions = map[State][]State{
	StateRegistered:   {StateInstalling, StateFailed},
	StateInstalling:   {StateInstalled, StateFailed, StateRegistered},
	StateInstalled:    {StateEnabled, StateUninstalling, StateFailed},
	StateEnabled:      {StateActivated, StateInstalled, StateFailed},
	StateActivated:    {StateSuspended, StateFailed, StateEnabled},
	StateSuspended:    {StateActivated, StateEnabled, StateFailed},
	StateFailed:       {StateInstalled, StateEnabled, StateUninstalling},
	StateUninstalling: {StateRegistered, StateFailed},
}

// IsTerminal returns true if no forward transitions are possible.
func (s State) IsTerminal() bool {
	return len(validTransitions[s]) == 0
}

// IsRunning returns true if the capsule is currently executing work.
func (s State) IsRunning() bool {
	return s == StateActivated
}

// Observer is notified of state changes. It must not block.
type Observer func(old, new State, reason string)

// Lifecycle manages a capsule's state machine, maintains timestamps, and
// notifies observers. It is safe for concurrent use.
type Lifecycle struct {
	mu        sync.RWMutex
	state     State
	lastErr   error
	lastChg   time.Time
	observers []Observer
}

// NewLifecycle creates a Lifecycle in the Registered state.
func NewLifecycle() *Lifecycle {
	return &Lifecycle{
		state:   StateRegistered,
		lastChg: time.Now(),
	}
}

// State returns the current lifecycle state.
func (l *Lifecycle) State() State {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.state
}

// LastChanged returns the timestamp of the most recent state transition.
func (l *Lifecycle) LastChanged() time.Time {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.lastChg
}

// LastError returns the error associated with the most recent Failed transition,
// or nil.
func (l *Lifecycle) LastError() error {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.lastErr
}

// OnChange subscribes an observer to state changes.
// Observers are invoked synchronously; keep them fast.
func (l *Lifecycle) OnChange(fn Observer) {
	l.mu.Lock()
	l.observers = append(l.observers, fn)
	l.mu.Unlock()
}

// Transition advances the state machine. Returns an error if the transition
// is not permitted by validTransitions.
//
// `reason` is a free-form string surfaced to observers and the UI.
func (l *Lifecycle) Transition(to State, reason string) error {
	l.mu.Lock()
	from := l.state
	if from == to {
		l.mu.Unlock()
		return nil
	}
	allowed := validTransitions[from]
	ok := false
	for _, c := range allowed {
		if c == to {
			ok = true
			break
		}
	}
	if !ok {
		l.mu.Unlock()
		return fmt.Errorf("invalid transition %s → %s", from, to)
	}
	l.state = to
	l.lastChg = time.Now()
	if to != StateFailed {
		l.lastErr = nil
	}
	observers := append([]Observer(nil), l.observers...)
	l.mu.Unlock()

	for _, fn := range observers {
		fn(from, to, reason)
	}
	return nil
}

// Fail is a shortcut for Transition(StateFailed, err.Error()) that also
// stores the error for later retrieval.
func (l *Lifecycle) Fail(err error) error {
	if err == nil {
		err = fmt.Errorf("unknown failure")
	}
	l.mu.Lock()
	from := l.state
	allowed := validTransitions[from]
	ok := false
	for _, c := range allowed {
		if c == StateFailed {
			ok = true
			break
		}
	}
	if !ok {
		l.mu.Unlock()
		return fmt.Errorf("cannot fail from state %s", from)
	}
	l.state = StateFailed
	l.lastErr = err
	l.lastChg = time.Now()
	observers := append([]Observer(nil), l.observers...)
	l.mu.Unlock()

	for _, fn := range observers {
		fn(from, StateFailed, err.Error())
	}
	return nil
}

// Reset forces the lifecycle back to Registered without going through
// the normal transition graph. Use only during registry teardown / tests.
func (l *Lifecycle) Reset() {
	l.mu.Lock()
	l.state = StateRegistered
	l.lastErr = nil
	l.lastChg = time.Now()
	l.mu.Unlock()
}
