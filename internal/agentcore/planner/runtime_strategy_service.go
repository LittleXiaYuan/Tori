package planner

import (
	"context"
	"log/slog"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/localbrain"
)

// RuntimeStrategyService owns Planner runtime strategy switches and model
// routing helpers. It groups LocalBrain, AgenticThinking, ReAct, long-horizon,
// and capability-aware provider routing so Planner can stay an orchestrator
// instead of accumulating every execution-mode field directly.
type RuntimeStrategyService struct {
	reactMode       bool
	longHorizonMode bool
	localBrain      *localbrain.LocalBrain
	agenticThinking *localbrain.AgenticThinking
	providerReg     *llm.ProviderRegistry
}

func NewRuntimeStrategyService() *RuntimeStrategyService {
	return &RuntimeStrategyService{}
}

func (s *RuntimeStrategyService) SetReActMode(enabled bool) {
	if s == nil {
		return
	}
	s.reactMode = enabled
}

func (s *RuntimeStrategyService) ReActMode() bool {
	return s != nil && s.reactMode
}

func (s *RuntimeStrategyService) SetLongHorizonMode(enabled bool) {
	if s == nil {
		return
	}
	s.longHorizonMode = enabled
}

func (s *RuntimeStrategyService) LongHorizonMode() bool {
	return s != nil && s.longHorizonMode
}

func (s *RuntimeStrategyService) SetLocalBrain(brain *localbrain.LocalBrain) {
	if s == nil {
		return
	}
	s.localBrain = brain
}

func (s *RuntimeStrategyService) LocalBrain() *localbrain.LocalBrain {
	if s == nil {
		return nil
	}
	return s.localBrain
}

func (s *RuntimeStrategyService) SetAgenticThinking(thinking *localbrain.AgenticThinking) {
	if s == nil {
		return
	}
	s.agenticThinking = thinking
}

func (s *RuntimeStrategyService) AgenticThinking() *localbrain.AgenticThinking {
	if s == nil {
		return nil
	}
	return s.agenticThinking
}

func (s *RuntimeStrategyService) SetProviderRegistry(reg *llm.ProviderRegistry) {
	if s == nil {
		return
	}
	s.providerReg = reg
}

func (s *RuntimeStrategyService) SelectProviderByCapability(required ...llm.Capability) *llm.ProviderInstance {
	if s == nil || s.providerReg == nil || len(required) == 0 {
		return nil
	}
	return s.providerReg.SelectByCapability(required...)
}

func (s *RuntimeStrategyService) Classify(ctx context.Context, query, tenantID string) (*localbrain.Decision, error) {
	if s == nil || s.localBrain == nil {
		return nil, nil
	}
	return s.localBrain.Classify(ctx, query, tenantID)
}

func (s *RuntimeStrategyService) Think(ctx context.Context, req localbrain.ThinkRequest) (*localbrain.ThinkResult, error) {
	if s == nil || s.agenticThinking == nil {
		return nil, nil
	}
	return s.agenticThinking.Think(ctx, req)
}

func (s *RuntimeStrategyService) SelectTierFromThinking(ctx context.Context, req localbrain.ThinkRequest) (tier string, stop bool, result *localbrain.ThinkResult) {
	result, err := s.Think(ctx, req)
	if err != nil || result == nil {
		if err != nil {
			slog.Debug("runtime strategy: agentic thinking skipped", "err", err)
		}
		return "", false, nil
	}
	if result.ShouldStop {
		return "", true, result
	}
	switch result.Level {
	case localbrain.ThinkQuick:
		return "fast", false, result
	case localbrain.ThinkDeep:
		return "expert", false, result
	default:
		return "smart", false, result
	}
}
