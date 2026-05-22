package reflect

import (
	"context"

	"yunque-agent/internal/cognikernel"
)

// AsReflectEvalFunc adapts the legacy Engine evaluator to the canonical
// cognikernel ReflectiveLoop hook shape.
//
// Deprecated: wire new reflection flows directly through
// cognikernel.ReflectiveLoop. This adapter exists only to migrate existing
// calls off the legacy package without changing runtime behavior.
func (e *Engine) AsReflectEvalFunc() cognikernel.ReflectEvalFunc {
	return func(ctx context.Context, intent, reply string, skillResults []string) (*cognikernel.ReflectEvalResult, error) {
		eval, err := e.Evaluate(ctx, intent, reply, skillResults)
		if err != nil {
			return nil, err
		}
		return toKernelReflectEvalResult(eval), nil
	}
}

func toKernelReflectEvalResult(eval *Evaluation) *cognikernel.ReflectEvalResult {
	if eval == nil {
		return nil
	}
	memoryUpdates := make([]cognikernel.MemUpdateReq, 0, len(eval.MemoryUpdates))
	for _, update := range eval.MemoryUpdates {
		memoryUpdates = append(memoryUpdates, cognikernel.MemUpdateReq{
			Action: update.Action,
			Key:    update.Key,
			Value:  update.Value,
		})
	}
	return &cognikernel.ReflectEvalResult{
		Satisfied:     eval.Satisfied,
		Quality:       eval.Quality,
		Issues:        append([]string(nil), eval.Issues...),
		Suggestions:   append([]string(nil), eval.Suggestions...),
		MemoryUpdates: memoryUpdates,
	}
}
