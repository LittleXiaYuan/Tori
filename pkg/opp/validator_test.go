package opp

import (
	"errors"
	"testing"
)

func TestMessageValidate_BadVersion(t *testing.T) {
	msg := NewAccept("a", "b", "s", "t")
	msg.V = 99
	if err := msg.Validate(); !errors.Is(err, ErrValidation) {
		t.Errorf("expected ErrValidation for bad version")
	}
}

func TestProblemPayload_Validate(t *testing.T) {
	bad := ProblemPayload{Severity: "error", Category: "x"}
	if err := bad.Validate(); !errors.Is(err, ErrValidation) {
		t.Errorf("expected ErrValidation for missing problem_id")
	}
}

func TestResultPayload_Validate(t *testing.T) {
	failedNoErr := ResultPayload{Status: "failed"}
	if err := failedNoErr.Validate(); !errors.Is(err, ErrValidation) {
		t.Errorf("expected ErrValidation for failed without error")
	}

	failedWithErr := ResultPayload{Status: "failed", Error: &OPPError{Code: ErrCodeInternalError, Message: "boom"}}
	if err := failedWithErr.Validate(); err != nil {
		t.Errorf("expected valid, got %v", err)
	}
}
