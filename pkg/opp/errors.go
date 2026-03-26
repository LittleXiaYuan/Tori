package opp

import "errors"

var (
	ErrValidation = errors.New("opp: validation failed")
	ErrTransition = errors.New("opp: invalid state transition")
)

// OPPError is a protocol-level error carried in ERROR or failed RESULT messages.
type OPPError struct {
	Code      ErrorCode      `json:"code"`
	Message   string         `json:"message"`
	Retryable bool           `json:"retryable"`
	Details   map[string]any `json:"details,omitempty"`
}

func (e *OPPError) Error() string {
	return string(e.Code) + ": " + e.Message
}
