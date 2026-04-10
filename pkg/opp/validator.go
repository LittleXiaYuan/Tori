package opp

import "fmt"

func (m *Message) Validate() error {
	if m.V != Version {
		return fmt.Errorf("%w: version %d unsupported", ErrValidation, m.V)
	}
	if m.ID == "" {
		return fmt.Errorf("%w: missing id", ErrValidation)
	}
	if m.SessionID == "" {
		return fmt.Errorf("%w: missing session_id", ErrValidation)
	}
	if m.Source == "" {
		return fmt.Errorf("%w: missing source", ErrValidation)
	}
	if m.Target == "" {
		return fmt.Errorf("%w: missing target", ErrValidation)
	}
	if m.Type == "" {
		return fmt.Errorf("%w: missing type", ErrValidation)
	}
	return nil
}

func (p *IntentPayload) Validate() error {
	if p.Intent.Name == "" {
		return fmt.Errorf("%w: intent.name required", ErrValidation)
	}
	if p.Intent.Version == "" {
		return fmt.Errorf("%w: intent.version required", ErrValidation)
	}
	return nil
}

func (p *ResultPayload) Validate() error {
	if p.Status == "" {
		return fmt.Errorf("%w: status required", ErrValidation)
	}
	if p.Status == "failed" && p.Error == nil {
		return fmt.Errorf("%w: failed result must include error", ErrValidation)
	}
	return nil
}

func (p *ProblemPayload) Validate() error {
	if p.ProblemID == "" {
		return fmt.Errorf("%w: problem_id required", ErrValidation)
	}
	if p.Severity == "" {
		return fmt.Errorf("%w: severity required", ErrValidation)
	}
	if p.Category == "" {
		return fmt.Errorf("%w: category required", ErrValidation)
	}
	return nil
}

func (p *QuestionPayload) Validate() error {
	if p.QuestionID == "" {
		return fmt.Errorf("%w: question_id required", ErrValidation)
	}
	if p.Text == "" {
		return fmt.Errorf("%w: text required", ErrValidation)
	}
	if len(p.InputMode) == 0 {
		return fmt.Errorf("%w: input_mode required", ErrValidation)
	}
	return nil
}

func (p *CapabilitiesPayload) Validate() error {
	if p.AgentID == "" {
		return fmt.Errorf("%w: agent_id required", ErrValidation)
	}
	return nil
}

func (p *DelegatePayload) Validate() error {
	if p.Intent.Name == "" {
		return fmt.Errorf("%w: intent.name required", ErrValidation)
	}
	return nil
}

func (p *FeedbackPayload) Validate() error {
	if p.TaskID == "" {
		return fmt.Errorf("%w: task_id required", ErrValidation)
	}
	if p.Rating < 0 || p.Rating > 1 {
		return fmt.Errorf("%w: rating must be 0.0-1.0", ErrValidation)
	}
	return nil
}
