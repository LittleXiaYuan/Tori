package rlsched

import (
	"time"

	"yunque-agent/internal/agentcore/task"
)

const (
	MetaPolicyState       = "ql_policy_state"
	MetaPolicyAction      = "ql_policy_action"
	MetaPolicyExploratory = "ql_policy_exploratory"
	MetaPolicyApplied     = "ql_policy_applied"
)

// PolicyStore wraps a task.Store and lets the learned Q policy influence
// newly-created task priority when the caller did not explicitly set one.
type PolicyStore struct {
	Base    task.Store
	Learner *QLearner
	Now     func() time.Time
}

func NewPolicyStore(base task.Store, learner *QLearner) *PolicyStore {
	return &PolicyStore{Base: base, Learner: learner, Now: time.Now}
}

func (s *PolicyStore) Create(req task.CreateRequest) (*task.Task, error) {
	if s == nil || s.Base == nil {
		return nil, errNilPolicyStore
	}
	req = s.apply(req)
	return s.Base.Create(req)
}

func (s *PolicyStore) Get(id string) (*task.Task, bool) {
	return s.Base.Get(id)
}

func (s *PolicyStore) List(tenantID string, limit int) []*task.Task {
	return s.Base.List(tenantID, limit)
}

func (s *PolicyStore) Update(t *task.Task) error {
	return s.Base.Update(t)
}

func (s *PolicyStore) Delete(id string) bool {
	return s.Base.Delete(id)
}

func (s *PolicyStore) ArtifactDir(taskID string) (string, error) {
	return s.Base.ArtifactDir(taskID)
}

func (s *PolicyStore) RecoverInterrupted() int {
	return s.Base.RecoverInterrupted()
}

func (s *PolicyStore) apply(req task.CreateRequest) task.CreateRequest {
	if s.Learner == nil {
		return req
	}
	now := time.Now()
	if s.Now != nil {
		now = s.Now()
	}
	constraints := cloneConstraints(req.Constraints)
	if constraints == nil {
		constraints = &task.TaskConstraints{}
	}
	state := EncodeTaskState(s.Base, &task.Task{
		TenantID:    req.TenantID,
		Constraints: constraints,
	}, now)
	action, exploratory := s.Learner.SelectAction(state)
	if action == "" {
		return req
	}
	if constraints.Extra == nil {
		constraints.Extra = map[string]any{}
	}
	constraints.Extra[MetaPolicyState] = state
	constraints.Extra[MetaPolicyAction] = action
	constraints.Extra[MetaPolicyExploratory] = exploratory

	if constraints.Priority == "" {
		constraints.Priority = priorityForAction(action)
		constraints.Extra[MetaPolicyApplied] = true
	} else {
		constraints.Extra[MetaPolicyApplied] = false
	}
	req.Constraints = constraints
	return req
}

func priorityForAction(action string) string {
	switch action {
	case "priority_high":
		return "high"
	case "priority_low", "defer":
		return "low"
	default:
		return "medium"
	}
}

func learnedSchedulingAction(t *task.Task) (string, bool) {
	if t == nil || t.Constraints == nil || t.Constraints.Extra == nil {
		return "", false
	}
	if raw, ok := t.Constraints.Extra[MetaPolicyAction]; ok {
		if action, ok := raw.(string); ok && action != "" {
			return action, true
		}
	}
	return "", false
}

func cloneConstraints(in *task.TaskConstraints) *task.TaskConstraints {
	if in == nil {
		return nil
	}
	out := *in
	if in.Tags != nil {
		out.Tags = append([]string(nil), in.Tags...)
	}
	if in.Extra != nil {
		out.Extra = make(map[string]any, len(in.Extra))
		for k, v := range in.Extra {
			out.Extra[k] = v
		}
	}
	return &out
}
