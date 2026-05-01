package cognikernel

import (
	"context"
	"time"
)

// ActiveLoop wraps the Planner-based main cognitive loop.
// It adds kernel-level hooks: publishing ConversationEnded events after each
// interaction so the reflective loop can pick up automatically.
//
// The actual planning logic stays in internal/agentcore/planner — this is
// purely a thin orchestration layer.
type ActiveLoop struct {
	kernel  *CogniKernel
	planFn  PlanFunc
	guardFn GuardFunc
}

// PlanFunc is the planner's entry point. It mirrors Planner.Run but accepts
// the kernel's ConversationRequest, allowing the kernel to inject hooks.
type PlanFunc func(ctx context.Context, req ConversationRequest) (*ConversationResult, error)

// GuardFunc is called before each conversation to run immune system checks.
// Returns non-nil error to block the conversation.
type GuardFunc func(ctx context.Context, input string) error

// ConversationRequest is the input to the active loop.
type ConversationRequest struct {
	TenantID  string `json:"tenant_id"`
	SessionID string `json:"session_id"`
	UserInput string `json:"user_input"`
	TaskID    string `json:"task_id,omitempty"`
	ModelTier string `json:"model_tier,omitempty"`
}

// ConversationResult is the output of the active loop.
type ConversationResult struct {
	Reply      string   `json:"reply"`
	SkillsUsed []string `json:"skills_used"`
	Steps      int      `json:"steps"`
	ModelTier  string   `json:"model_tier"`
}

// NewActiveLoop creates the main cognitive loop wrapper.
func NewActiveLoop(kernel *CogniKernel) *ActiveLoop {
	return &ActiveLoop{kernel: kernel}
}

// SetPlanFunc sets the planning function (typically Planner.Run adapter).
func (al *ActiveLoop) SetPlanFunc(fn PlanFunc) { al.planFn = fn }

// SetGuardFunc sets the pre-conversation immune gate.
func (al *ActiveLoop) SetGuardFunc(fn GuardFunc) { al.guardFn = fn }

// Handle processes one conversation turn through the main loop.
// After completion, it publishes a ConversationEnded event to the kernel bus
// so the reflective loop can auto-trigger.
func (al *ActiveLoop) Handle(ctx context.Context, req ConversationRequest) (*ConversationResult, error) {
	t0 := time.Now()

	// Immune gate: check input before processing
	if al.guardFn != nil {
		if err := al.guardFn(ctx, req.UserInput); err != nil {
			return nil, err
		}
	}

	if al.planFn == nil {
		return &ConversationResult{Reply: "Planner not configured"}, nil
	}

	result, err := al.planFn(ctx, req)
	if err != nil {
		return nil, err
	}

	// Publish conversation-ended event to trigger reflective loop
	al.kernel.OnConversationEnd(ConversationEndData{
		TenantID:   req.TenantID,
		SessionID:  req.SessionID,
		UserIntent: req.UserInput,
		AgentReply: result.Reply,
		SkillsUsed: result.SkillsUsed,
		ModelTier:  result.ModelTier,
		TaskID:     req.TaskID,
		Duration:   time.Since(t0),
	})

	return result, nil
}
