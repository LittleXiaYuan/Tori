package cognisdk

import (
	"context"
)

// HostAdapter bridges host requests into the experimental cognition SDK.
type HostAdapter struct {
	engine *Engine
}

// NewHostAdapter creates an adapter from a direct config.
func NewHostAdapter(config Config) *HostAdapter {
	return &HostAdapter{engine: NewEngine(config)}
}

// NewHostAdapterFromDir loads local packs from a directory and builds an adapter.
func NewHostAdapterFromDir(dir string) (*HostAdapter, []PackLoadError, error) {
	pm, errs, err := NewPackManagerFromDir(dir)
	if err != nil {
		return nil, errs, err
	}
	return &HostAdapter{engine: &Engine{manager: pm}}, errs, nil
}

// Engine exposes the underlying engine for inspection.
func (a *HostAdapter) Engine() *Engine {
	if a == nil {
		return nil
	}
	return a.engine
}

// BuildContext evaluates one turn and returns planner-ready markdown.
func (a *HostAdapter) BuildContext(ctx context.Context, message, tenantID, channel string) string {
	if a == nil || a.engine == nil {
		return ""
	}
	result := a.engine.Evaluate(ctx, Input{
		Message: message,
		UserID:  tenantID,
		Channel: channel,
	})
	return RenderMarkdown(result)
}

// Evaluate returns the structured cognition result for a turn.
func (a *HostAdapter) Evaluate(ctx context.Context, input Input) Result {
	if a == nil || a.engine == nil {
		return Result{}
	}
	return a.engine.Evaluate(ctx, input)
}

// PackManager exposes the runtime pack manager.
func (a *HostAdapter) PackManager() *PackManager {
	if a == nil || a.engine == nil {
		return nil
	}
	return a.engine.PackManager()
}
