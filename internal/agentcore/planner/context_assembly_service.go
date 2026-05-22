package planner

import (
	"context"

	"yunque-agent/pkg/skills"
)

// ContextAssemblyService owns Planner's dynamic context assembly callbacks.
//
// It groups retrieval, strategy, Cogni, CognitivePlugin, and belief context
// sources behind one boundary. Planner remains responsible for orchestration;
// PromptBuilder consumes a snapshot of this service instead of many scattered
// Planner fields.
type ContextAssemblyService struct {
	memory             MemorySearchFunc
	graphContext       func(query string) string
	codeContext        func(query string) string
	stateContext       func() string
	strategyContext    func() string
	strategyContextFor func(query string) string
	cognitiveContext   CognitiveContextFunc
	beliefContext      BeliefContextFunc
	cogniService       *CogniContextService
}

func NewContextAssemblyService() *ContextAssemblyService {
	return &ContextAssemblyService{}
}

func (s *ContextAssemblyService) SetMemory(fn MemorySearchFunc) {
	if s != nil {
		s.memory = fn
	}
}

func (s *ContextAssemblyService) Memory(ctx context.Context, tenantID, query string) string {
	if s == nil || s.memory == nil {
		return ""
	}
	return s.memory(ctx, tenantID, query)
}

func (s *ContextAssemblyService) SetGraphContext(fn func(query string) string) {
	if s != nil {
		s.graphContext = fn
	}
}

func (s *ContextAssemblyService) GraphContext() func(query string) string {
	if s == nil {
		return nil
	}
	return s.graphContext
}

func (s *ContextAssemblyService) SetCodeContext(fn func(query string) string) {
	if s != nil {
		s.codeContext = fn
	}
}

func (s *ContextAssemblyService) SetStateContext(fn func() string) {
	if s != nil {
		s.stateContext = fn
	}
}

func (s *ContextAssemblyService) SetStrategyContext(fn func() string) {
	if s != nil {
		s.strategyContext = fn
	}
}

func (s *ContextAssemblyService) SetStrategyContextFor(fn func(query string) string) {
	if s != nil {
		s.strategyContextFor = fn
	}
}

func (s *ContextAssemblyService) SetCognitiveContext(fn CognitiveContextFunc) {
	if s != nil {
		s.cognitiveContext = fn
	}
}

func (s *ContextAssemblyService) SetBeliefContext(fn BeliefContextFunc) {
	if s != nil {
		s.beliefContext = fn
	}
}

func (s *ContextAssemblyService) ensureCogniService() *CogniContextService {
	if s.cogniService == nil {
		s.cogniService = NewCogniContextService()
	}
	return s.cogniService
}

func (s *ContextAssemblyService) SetCogniContext(fn CogniContextFunc) {
	if s != nil {
		s.ensureCogniService().SetContext(fn)
	}
}

func (s *ContextAssemblyService) SetCogniSkillFilter(fn CogniSkillFilterFunc) {
	if s != nil {
		s.ensureCogniService().SetSkillFilter(fn)
	}
}

func (s *ContextAssemblyService) SetCogniTrace(fn CogniTraceFunc) {
	if s != nil {
		s.ensureCogniService().SetTrace(fn)
	}
}

func (s *ContextAssemblyService) HasCogniTrace() bool {
	return s != nil && s.cogniService != nil && s.cogniService.HasTrace()
}

func (s *ContextAssemblyService) CogniTrace(message, tenantID, channel string) (CogniTraceDetail, bool) {
	if s == nil || s.cogniService == nil {
		return CogniTraceDetail{}, false
	}
	return s.cogniService.Trace(message, tenantID, channel)
}

func (s *ContextAssemblyService) HasCogniSkillFilter() bool {
	return s != nil && s.cogniService != nil && s.cogniService.HasSkillFilter()
}

func (s *ContextAssemblyService) FilterCogniSkills(message, tenantID, channel string, in []skills.Skill) []skills.Skill {
	if s == nil || s.cogniService == nil {
		return in
	}
	return s.cogniService.FilterSkills(message, tenantID, channel, in)
}
