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

func (s *CogniContextService) Context(ctx context.Context, message, tenantID, channel string) string {
	if s == nil || s.context == nil {
		return ""
	}
	return s.context(ctx, message, tenantID, channel)
}

func (s *CogniContextService) FilterSkills(message, tenantID, channel string, in []skills.Skill) []skills.Skill {
	if s == nil || s.skillFilter == nil {
		return in
	}
	return s.skillFilter(message, tenantID, channel, in)
}

func (s *CogniContextService) Trace(message, tenantID, channel string) (CogniTraceDetail, bool) {
	if s == nil || s.trace == nil {
		return CogniTraceDetail{}, false
	}
	return s.trace(message, tenantID, channel)
}

func (s *CogniContextService) HasSkillFilter() bool {
	return s != nil && s.skillFilter != nil
}

func (s *CogniContextService) HasTrace() bool {
	return s != nil && s.trace != nil
}
