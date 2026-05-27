package approval

// ──────────────────────────────────────────────
// Evaluator — determines if an operation requires approval
//
// Called before skill execution by the Task Runner.
// Returns nil if no approval needed, or an ApprovalRequest if needed.
// ──────────────────────────────────────────────

// Evaluator checks operations against risk policy.
type Evaluator struct {
	policy  Policy
	riskMap map[string]RiskLevel
}

// NewEvaluator creates an evaluator with the given policy.
func NewEvaluator(policy Policy) *Evaluator {
	return &Evaluator{
		policy:  policy,
		riskMap: SkillRisk,
	}
}

// SetRisk overrides the risk level for a specific skill.
func (e *Evaluator) SetRisk(skillName string, level RiskLevel) {
	if e.riskMap == nil {
		e.riskMap = make(map[string]RiskLevel)
	}
	e.riskMap[skillName] = level
}

// EvalInput describes the operation to evaluate.
type EvalInput struct {
	SkillName string
	TaskID    string
	StepIndex int
	Params    map[string]any
	TenantID  string
}

// Evaluate checks if the operation needs human approval.
// Returns nil if auto-approved, or a pre-filled Request if approval is needed.
func (e *Evaluator) Evaluate(input EvalInput) *Request {
	risk, ok := e.riskMap[input.SkillName]
	if !ok {
		risk = RiskLow // unknown skills default to low risk
	}

	// Determine category from skill name
	cat := categorize(input.SkillName)

	// Check if this risk level requires approval
	if risk == RiskLow {
		return nil
	}

	// Check always-require categories
	alwaysRequired := false
	for _, c := range e.policy.AlwaysRequire {
		if c == cat {
			alwaysRequired = true
			break
		}
	}

	// Below threshold and not always-required
	if !risk.AtLeast(e.policy.MinRiskLevel) && !alwaysRequired {
		return nil
	}

	// Build approval request
	return &Request{
		TaskID:    input.TaskID,
		StepIndex: input.StepIndex,
		Category:  cat,
		RiskLevel: risk,
		Summary:   buildSummary(input.SkillName, input.Params),
		Details: map[string]any{
			"skill_name": input.SkillName,
			"params":     input.Params,
		},
		Requester: "task_runner",
		TenantID:  input.TenantID,
	}
}

// categorize maps a skill name to a risk category.
func categorize(skillName string) Category {
	switch {
	case contains(skillName, "email", "message", "notify", "send"):
		return CatCommunication
	case contains(skillName, "exec", "run", "shell", "python", "code"):
		return CatCodeExec
	case contains(skillName, "http", "api", "request", "fetch"):
		return CatExternalAPI
	case contains(skillName, "pay", "charge", "bill", "cost"):
		return CatFinancial
	case contains(skillName, "delete", "remove", "drop", "write", "update", "create"):
		return CatDataMutation
	case contains(skillName, "config", "setting", "install", "deploy"):
		return CatSystemConfig
	default:
		return CatDataMutation
	}
}

func contains(s string, subs ...string) bool {
	for _, sub := range subs {
		if len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}

// buildSummary creates a human-readable description.
func buildSummary(skillName string, params map[string]any) string {
	summary := "执行操作: " + skillName
	if cmd, ok := params["command"]; ok {
		summary += " — " + truncStr(toString(cmd), 80)
	}
	if path, ok := params["path"]; ok {
		summary += " — " + truncStr(toString(path), 80)
	}
	if to, ok := params["to"]; ok {
		summary += " → " + truncStr(toString(to), 40)
	}
	return summary
}

func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func truncStr(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "..."
}
