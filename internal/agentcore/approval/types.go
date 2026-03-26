package approval

import (
	"time"
)

// ──────────────────────────────────────────────
// Approval — Human-in-the-Loop decision gate
//
// When the Agent performs a high-risk operation (send email, delete data,
// execute code, spend money), it creates an ApprovalRequest and pauses
// execution until a human approves or denies.
//
// Integration points:
//   - Task Runner: pauses step execution pending approval
//   - Workflow Engine: approval node type
//   - Guardrails: ToolGuard can trigger approval instead of block
//   - SSE: pushes approval requests to frontend in real-time
// ──────────────────────────────────────────────

// Status of an approval request.
type Status string

const (
	StatusPending  Status = "pending"
	StatusApproved Status = "approved"
	StatusDenied   Status = "denied"
	StatusExpired  Status = "expired"
	StatusAutoApproved Status = "auto_approved" // trust level high enough
)

// RiskLevel classifies the risk of an operation.
type RiskLevel string

const (
	RiskLow      RiskLevel = "low"      // informational, auto-approved
	RiskMedium   RiskLevel = "medium"   // logged, may auto-approve with high trust
	RiskHigh     RiskLevel = "high"     // requires human approval
	RiskCritical RiskLevel = "critical" // always requires human approval, even at max trust
)

// Category classifies the type of operation.
type Category string

const (
	CatDataMutation Category = "data_mutation"   // create/update/delete data
	CatExternalAPI  Category = "external_api"    // call external services
	CatCodeExec     Category = "code_execution"  // run code/commands
	CatFinancial    Category = "financial"       // spend money / billing
	CatCommunication Category = "communication" // send email/message
	CatSystemConfig Category = "system_config"  // change system settings
)

// Request is a pending approval from the Agent runtime.
type Request struct {
	ID          string            `json:"id"`
	TaskID      string            `json:"task_id,omitempty"`
	WorkflowID  string            `json:"workflow_id,omitempty"`
	StepIndex   int               `json:"step_index,omitempty"`
	Category    Category          `json:"category"`
	RiskLevel   RiskLevel         `json:"risk_level"`
	Summary     string            `json:"summary"`       // human-readable description
	Details     map[string]any    `json:"details"`       // structured details (skill name, params, etc.)
	Status      Status            `json:"status"`
	Requester   string            `json:"requester"`     // agent role / component that triggered this
	Approver    string            `json:"approver,omitempty"`  // human who approved/denied
	Reason      string            `json:"reason,omitempty"`    // reason for denial
	TenantID    string            `json:"tenant_id"`
	CreatedAt   time.Time         `json:"created_at"`
	ResolvedAt  *time.Time        `json:"resolved_at,omitempty"`
	ExpiresAt   time.Time         `json:"expires_at"`
}

// IsResolved returns true if the request has been decided.
func (r *Request) IsResolved() bool {
	return r.Status != StatusPending
}

// Policy defines when approvals are required.
type Policy struct {
	// MinRiskLevel: operations at or above this level require approval.
	MinRiskLevel RiskLevel `json:"min_risk_level"`

	// TrustAutoApprove: if user trust score >= this, auto-approve medium risk.
	TrustAutoApprove float64 `json:"trust_auto_approve"`

	// DefaultTimeout: how long to wait for approval before expiring.
	DefaultTimeout time.Duration `json:"default_timeout"`

	// AlwaysRequire: categories that always require approval regardless of trust.
	AlwaysRequire []Category `json:"always_require"`
}

// DefaultPolicy returns a reasonable default approval policy.
func DefaultPolicy() Policy {
	return Policy{
		MinRiskLevel:     RiskHigh,
		TrustAutoApprove: 0.9, // 90% trust = auto-approve medium
		DefaultTimeout:   30 * time.Minute,
		AlwaysRequire:    []Category{CatFinancial, CatCommunication},
	}
}

// ── Skill Risk Classification ──

// SkillRisk maps skill names to their risk classification.
// Used by the Evaluator to determine if an operation needs approval.
var SkillRisk = map[string]RiskLevel{
	"exec_command":   RiskHigh,
	"run_python":     RiskHigh,
	"run_code":       RiskHigh,
	"send_email":     RiskCritical,
	"send_message":   RiskMedium,
	"http_request":   RiskMedium,
	"file_write":     RiskMedium,
	"file_delete":    RiskHigh,
	"db_query":       RiskMedium,
	"db_mutation":    RiskHigh,
	"deploy":         RiskCritical,
	"install_plugin": RiskHigh,
}
