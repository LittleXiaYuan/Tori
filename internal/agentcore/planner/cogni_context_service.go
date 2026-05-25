package planner

import (
	"context"

	"yunque-agent/pkg/skills"
)

// CogniContextService groups all declarative Cogni callbacks that used to live
// as separate Planner fields. Keeping them behind one boundary is the first
// step toward making Cogni the activation/context owner and Planner only the
// execution runtime.
type CogniContextService struct {
	context     CogniContextFunc
	skillFilter CogniSkillFilterFunc
	trace       CogniTraceFunc
	runtime     CogniRuntime
}

func NewCogniContextService() *CogniContextService {
	return &CogniContextService{}
}

func (s *CogniContextService) SetContext(fn CogniContextFunc) {
	s.context = fn
}

func (s *CogniContextService) SetSkillFilter(fn CogniSkillFilterFunc) {
	s.skillFilter = fn
}

func (s *CogniContextService) SetTrace(fn CogniTraceFunc) {
	s.trace = fn
}

func (s *CogniContextService) SetRuntime(runtime CogniRuntime) {
	s.runtime = runtime
}

func (s *CogniContextService) Context(ctx context.Context, message, tenantID, channel string) string {
	if s == nil || s.context == nil {
		if s != nil && s.runtime != nil {
			return s.runtime.BuildContext(ctx, message, tenantID, channel)
		}
		return ""
	}
	return s.context(ctx, message, tenantID, channel)
}

func (s *CogniContextService) FilterSkills(message, tenantID, channel string, in []skills.Skill) []skills.Skill {
	if s == nil || s.skillFilter == nil {
		if s != nil && s.runtime != nil {
			return s.runtime.FilterSkills(message, tenantID, channel, in)
		}
		return in
	}
	return s.skillFilter(message, tenantID, channel, in)
}

func (s *CogniContextService) Trace(message, tenantID, channel string) (CogniTraceDetail, bool) {
	if s == nil || s.trace == nil {
		if s != nil && s.runtime != nil {
			return s.runtime.Trace(message, tenantID, channel)
		}
		return CogniTraceDetail{}, false
	}
	return s.trace(message, tenantID, channel)
}

func (s *CogniContextService) HasSkillFilter() bool {
	return s != nil && (s.skillFilter != nil || s.runtime != nil)
}

func (s *CogniContextService) HasTrace() bool {
	return s != nil && (s.trace != nil || s.runtime != nil)
}
