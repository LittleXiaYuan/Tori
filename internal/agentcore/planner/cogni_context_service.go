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
	runtime CogniRuntime
}

func NewCogniContextService() *CogniContextService {
	return &CogniContextService{}
}

func (s *CogniContextService) SetRuntime(runtime CogniRuntime) {
	s.runtime = runtime
}

func (s *CogniContextService) Context(ctx context.Context, message, tenantID, channel, scope string) string {
	if s == nil || s.runtime == nil {
		return ""
	}
	return s.runtime.BuildContext(ctx, message, tenantID, channel, scope)
}

func (s *CogniContextService) FilterSkills(message, tenantID, channel string, in []skills.Skill) []skills.Skill {
	if s == nil || s.runtime == nil {
		return in
	}
	return s.runtime.FilterSkills(message, tenantID, channel, in)
}

func (s *CogniContextService) Trace(message, tenantID, channel string) (CogniTraceDetail, bool) {
	if s == nil || s.runtime == nil {
		return CogniTraceDetail{}, false
	}
	return s.runtime.Trace(message, tenantID, channel)
}

func (s *CogniContextService) Tools(ctx context.Context, message, tenantID, channel string) []CogniTool {
	if s == nil || s.runtime == nil {
		return nil
	}
	return s.runtime.Tools(ctx, message, tenantID, channel)
}

func (s *CogniContextService) SurfaceAuthoritative(message, tenantID, channel string) bool {
	if s == nil || s.runtime == nil {
		return false
	}
	return s.runtime.SurfaceAuthoritative(message, tenantID, channel)
}

func (s *CogniContextService) RecordToolOutcome(message, tenantID, channel, tool string, success bool) {
	if s == nil || s.runtime == nil {
		return
	}
	s.runtime.RecordToolOutcome(message, tenantID, channel, tool, success)
}

func (s *CogniContextService) HasSkillFilter() bool {
	return s != nil && s.runtime != nil
}

func (s *CogniContextService) HasTrace() bool {
	return s != nil && s.runtime != nil
}
