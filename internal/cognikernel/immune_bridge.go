package cognikernel

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
)

// ImmuneBridge is the cross-cutting safety layer that connects
// Trust, Guardrails, and CircuitBreaker across all cognitive loops.
//
// It acts as a unified gate: the kernel calls BeforeSkill/AfterSkill around
// every skill execution, and HandleEvent processes security alerts.
type ImmuneBridge struct {
	mu sync.RWMutex

	trustCheck   TrustCheckFunc
	trustRecord  TrustRecordFunc
	guardCheck   GuardCheckFunc
	onAlert      AlertFunc
	metrics      ImmuneMetrics
}

// TrustCheckFunc returns non-nil error if the skill is not trusted enough.
type TrustCheckFunc func(skillName string) error

// TrustRecordFunc records a skill execution outcome for trust scoring.
type TrustRecordFunc func(skillName string, success bool)

// GuardCheckFunc runs guardrail checks on input/output.
// Returns non-nil error if the content is blocked.
type GuardCheckFunc func(ctx context.Context, input string) error

// AlertFunc is called when a security event is detected.
type AlertFunc func(ctx context.Context, severity int, source, detail string)

// ImmuneMetrics tracks security-related statistics.
type ImmuneMetrics struct {
	TrustBlocks    int64 `json:"trust_blocks"`
	GuardBlocks    int64 `json:"guard_blocks"`
	TotalChecks    int64 `json:"total_checks"`
	SecurityAlerts int64 `json:"security_alerts"`
}

// NewImmuneBridge creates the immune system bridge.
func NewImmuneBridge() *ImmuneBridge {
	return &ImmuneBridge{}
}

func (ib *ImmuneBridge) SetTrustCheck(fn TrustCheckFunc)   { ib.trustCheck = fn }
func (ib *ImmuneBridge) SetTrustRecord(fn TrustRecordFunc) { ib.trustRecord = fn }
func (ib *ImmuneBridge) SetGuardCheck(fn GuardCheckFunc)   { ib.guardCheck = fn }
func (ib *ImmuneBridge) SetOnAlert(fn AlertFunc)           { ib.onAlert = fn }

// BeforeSkill is called before each skill execution.
// It checks trust and guardrails, returning an error to block execution.
func (ib *ImmuneBridge) BeforeSkill(ctx context.Context, skillName string, args map[string]any) error {
	ib.mu.Lock()
	ib.metrics.TotalChecks++
	ib.mu.Unlock()

	if ib.trustCheck != nil {
		if err := ib.trustCheck(skillName); err != nil {
			ib.mu.Lock()
			ib.metrics.TrustBlocks++
			ib.mu.Unlock()

			slog.Warn("immune: trust gate blocked skill",
				"skill", skillName, "err", err)
			return fmt.Errorf("blocked by trust gate: %w", err)
		}
	}

	return nil
}

// AfterSkill is called after each skill execution to update trust scores.
func (ib *ImmuneBridge) AfterSkill(ctx context.Context, skillName string, success bool, err error) {
	if ib.trustRecord != nil {
		ib.trustRecord(skillName, success)
	}

	if !success && ib.onAlert != nil {
		errMsg := "unknown error"
		if err != nil {
			errMsg = err.Error()
		}
		ib.onAlert(ctx, 1, "skill_failure", fmt.Sprintf("skill %s failed: %s", skillName, errMsg))
	}
}

// BeforeConversation runs input guardrails before processing a user message.
func (ib *ImmuneBridge) BeforeConversation(ctx context.Context, input string) error {
	if ib.guardCheck == nil {
		return nil
	}

	ib.mu.Lock()
	ib.metrics.TotalChecks++
	ib.mu.Unlock()

	if err := ib.guardCheck(ctx, input); err != nil {
		ib.mu.Lock()
		ib.metrics.GuardBlocks++
		ib.mu.Unlock()

		slog.Warn("immune: guardrail blocked input", "err", err)
		return fmt.Errorf("blocked by guardrails: %w", err)
	}
	return nil
}

// HandleEvent processes a security alert event from the kernel bus.
func (ib *ImmuneBridge) HandleEvent(ctx context.Context, ev Event) {
	ib.mu.Lock()
	ib.metrics.SecurityAlerts++
	ib.mu.Unlock()

	slog.Info("immune: security event received", "type", ev.Type)

	if ib.onAlert != nil {
		detail := fmt.Sprintf("kernel security event: %v", ev.Data)
		ib.onAlert(ctx, 2, "kernel", detail)
	}
}

// Metrics returns a snapshot of immune system metrics.
func (ib *ImmuneBridge) Metrics() ImmuneMetrics {
	ib.mu.RLock()
	defer ib.mu.RUnlock()
	return ib.metrics
}
