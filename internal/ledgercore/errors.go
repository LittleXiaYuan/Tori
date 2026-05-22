package ledger

import "errors"

// Sentinel errors for the Ledger domain.
var (
	// Task errors
	ErrTaskNotFound      = errors.New("ledger: task not found")
	ErrInvalidTransition = errors.New("ledger: invalid state transition")
	ErrVersionConflict   = errors.New("ledger: version conflict (optimistic lock)")
	ErrTaskTerminal      = errors.New("ledger: task is in a terminal state")

	// Event errors
	ErrEventSeqConflict = errors.New("ledger: event sequence conflict")

	// Checkpoint errors
	ErrCheckpointNotFound = errors.New("ledger: checkpoint not found")

	// Memory errors
	ErrMemoryNotFound = errors.New("ledger: memory entry not found")

	// KV errors
	ErrKVNotFound = errors.New("ledger: kv entry not found")

	// Artifact errors
	ErrArtifactNotFound = errors.New("ledger: artifact not found")

	// Backend errors
	ErrBackendClosed   = errors.New("ledger: backend is closed")
	ErrMigrationFailed = errors.New("ledger: migration failed")
)

// TransitionError provides details about a rejected state transition.
type TransitionError struct {
	TaskID string
	From   TaskStatus
	To     TaskStatus
}

func (e *TransitionError) Error() string {
	return "ledger: invalid transition from " + string(e.From) + " to " + string(e.To) + " for task " + e.TaskID
}

func (e *TransitionError) Unwrap() error {
	return ErrInvalidTransition
}
