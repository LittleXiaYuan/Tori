package opp

import (
	"errors"
	"testing"
)

func TestTransition_HappyPath(t *testing.T) {
	accept := NewAccept("agent", "caller", "s1", "t1")
	progress := NewProgress("agent", "caller", "s1", "t1", "build", 0.5, "compiling")
	result := NewResult("agent", "caller", "s1", "t1", "success", "done", nil)

	state := StatePending

	var err error
	state, err = Transition(state, accept)
	assertTransition(t, state, StateAccepted, err)

	state, err = Transition(state, progress)
	assertTransition(t, state, StateRunning, err)

	state, err = Transition(state, result)
	assertTransition(t, state, StateCompleted, err)
}

func TestTransition_QuestionAnswer(t *testing.T) {
	q := NewQuestion("agent", "caller", "s1", "t1", QuestionPayload{
		QuestionID: "q1", Text: "which env?", InputMode: map[string]any{"type": "select"},
	})
	a := NewAnswer("caller", "agent", "s1", "t1", "q1", "production")

	state, err := Transition(StateRunning, q)
	assertTransition(t, state, StateWaitingInput, err)

	state, err = Transition(state, a)
	assertTransition(t, state, StateRunning, err)
}

func TestTransition_ProblemDecide(t *testing.T) {
	p := NewProblem("agent", "caller", "s1", "t1", ProblemPayload{
		ProblemID: "p1", Severity: "error", Category: "port_conflict",
	})
	d := NewDecide("caller", "agent", "s1", "t1", "p1", "kill_old", "")

	state, err := Transition(StateRunning, p)
	assertTransition(t, state, StateBlocked, err)

	state, err = Transition(state, d)
	assertTransition(t, state, StateRunning, err)
}

func TestTransition_CancelFromAnyActive(t *testing.T) {
	cancel := newMsg("caller", "agent", "s1", MsgCancel, nil)
	actives := []TaskState{StatePending, StateAccepted, StateRunning, StateWaitingInput, StateBlocked}

	for _, s := range actives {
		next, err := Transition(s, cancel)
		if err != nil || next != StateCancelled {
			t.Errorf("cancel from %s: got %s, err %v", s, next, err)
		}
	}

	// Cannot cancel from terminal
	_, err := Transition(StateCompleted, cancel)
	if !errors.Is(err, ErrTransition) {
		t.Errorf("expected ErrTransition when cancelling completed task")
	}
}

func TestTransition_InvalidPath(t *testing.T) {
	answer := newMsg("caller", "agent", "s1", MsgAnswer, nil)
	_, err := Transition(StateRunning, answer)
	if !errors.Is(err, ErrTransition) {
		t.Errorf("expected ErrTransition for ANSWER in running state, got %v", err)
	}
}

func TestTransition_DelegateResume(t *testing.T) {
	delegate := NewDelegate("agent", "sub-agent", "s1", DelegatePayload{
		Intent: IntentEnvelope{Name: "ops.deploy", Version: "1.0"},
	})
	delegateResult := NewDelegateResult("sub-agent", "agent", "s1", "t1", DelegateResultPayload{
		DelegatedTo: "sub-agent",
		Result:      ResultPayload{Status: "success", Output: "deployed"},
	})

	state, err := Transition(StateRunning, delegate)
	assertTransition(t, state, StateWaitingInput, err)

	state, err = Transition(state, delegateResult)
	assertTransition(t, state, StateRunning, err)
}

func TestTransition_FeedbackOnCompleted(t *testing.T) {
	fb := NewFeedback("caller", "agent", "s1", "t1", FeedbackPayload{
		TaskID: "t1", Rating: 0.9,
	})

	state, err := Transition(StateCompleted, fb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != StateCompleted {
		t.Fatalf("state = %s, want completed (feedback doesn't change state)", state)
	}

	state, err = Transition(StateFailed, fb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != StateFailed {
		t.Fatalf("state = %s, want failed", state)
	}

	// Feedback on running is also allowed
	state, err = Transition(StateRunning, fb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != StateRunning {
		t.Fatalf("state = %s, want running", state)
	}
}

func TestTransition_FeedbackOnPending(t *testing.T) {
	fb := NewFeedback("caller", "agent", "s1", "t1", FeedbackPayload{
		TaskID: "t1", Rating: 0.5,
	})
	_, err := Transition(StatePending, fb)
	if !errors.Is(err, ErrTransition) {
		t.Errorf("expected ErrTransition for feedback on pending, got %v", err)
	}
}

func assertTransition(t *testing.T, got, want TaskState, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Fatalf("state = %s, want %s", got, want)
	}
}
