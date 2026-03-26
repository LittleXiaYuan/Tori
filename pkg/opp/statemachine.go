package opp

import "fmt"

// Transition returns the next TaskState given current state and an incoming message.
// Pure function, no side effects. Returns ErrTransition on illegal moves.
func Transition(current TaskState, msg *Message) (TaskState, error) {
	if msg.Type == MsgCancel {
		if IsTerminal(current) {
			return current, fmt.Errorf("%w: cannot cancel %s", ErrTransition, current)
		}
		return StateCancelled, nil
	}

	if msg.Type == MsgResult {
		if IsTerminal(current) {
			return current, fmt.Errorf("%w: result from %s", ErrTransition, current)
		}
		p, err := msg.DecodeResult()
		if err != nil {
			return current, fmt.Errorf("opp: decode result: %w", err)
		}
		if p.Status == "failed" {
			return StateFailed, nil
		}
		return StateCompleted, nil
	}

	switch current {
	case StatePending:
		switch msg.Type {
		case MsgAccept:
			return StateAccepted, nil
		case MsgReject:
			return StateFailed, nil
		}

	case StateAccepted:
		switch msg.Type {
		case MsgProgress, MsgHeartbeat:
			return StateRunning, nil
		case MsgQuestion:
			return StateWaitingInput, nil
		case MsgProblem:
			return StateBlocked, nil
		}

	case StateRunning:
		switch msg.Type {
		case MsgProgress, MsgHeartbeat, MsgObservation, MsgActionTaken:
			return StateRunning, nil
		case MsgQuestion:
			return StateWaitingInput, nil
		case MsgProblem:
			return StateBlocked, nil
		}

	case StateWaitingInput:
		if msg.Type == MsgAnswer {
			return StateRunning, nil
		}

	case StateBlocked:
		if msg.Type == MsgDecide {
			return StateRunning, nil
		}
	}

	return current, fmt.Errorf("%w: %s via %s", ErrTransition, current, msg.Type)
}

func IsTerminal(s TaskState) bool {
	switch s {
	case StateCompleted, StateFailed, StateCancelled, StateTimedOut:
		return true
	}
	return false
}

func IsActive(s TaskState) bool { return !IsTerminal(s) }
