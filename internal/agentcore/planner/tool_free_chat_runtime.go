package planner

import (
	"context"
	"fmt"
)

func (p *Planner) runToolFreeChat(ctx context.Context, req PlanRequest, errPrefix string, steps int) (*PlanResult, error) {
	modelRuntime := p.ensureModelRuntime()
	runtimeStrategy := p.ensureRuntimeStrategy()
	messages, layers := p.BuildMessages(ctx, req)
	if p.contextAssembly != nil {
		p.contextAssembly.EmitCogniTraceForRequest(req)
	}
	reply, err := modelRuntime.ChatFallbackForRequest(ctx, req, messages, runtimeStrategy, p.modelFallbackEvents(req))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errPrefix, err)
	}
	cleaned := p.cleanReply(reply)
	cleaned, nextMoves := extractNextMoves(cleaned)
	return &PlanResult{Reply: cleaned, Steps: steps, ContextLayers: layers, Suggestions: nextMoves}, nil
}
