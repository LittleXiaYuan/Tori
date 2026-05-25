package planner

import (
	"context"
	"log/slog"
	"time"
)

// safeToolGo runs fn in a goroutine with panic recovery and a timeout derived from ctx.
// If fn panics, it sends an error result on resultsCh. If the context deadline is exceeded,
// the tool's context is cancelled (the tool must respect ctx.Done()).
func safeToolGo(ctx context.Context, timeout time.Duration, fn func(ctx context.Context)) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("planner: tool goroutine panic", "panic", r)
			}
		}()
		if timeout <= 0 {
			fn(ctx)
			return
		}
		toolCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		fn(toolCtx)
	}()
}
