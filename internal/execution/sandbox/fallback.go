package sandbox

import (
	"context"
	"fmt"
	"log/slog"
)

type FallbackRunner struct {
	primary  Runner
	fallback Runner
}

func NewFallbackRunner(primary, fallback Runner) *FallbackRunner {
	return &FallbackRunner{primary: primary, fallback: fallback}
}

func (r *FallbackRunner) Type() string {
	return fmt.Sprintf("%s+%s", r.primary.Type(), r.fallback.Type())
}

func (r *FallbackRunner) Run(ctx context.Context, req RunRequest) (*RunResult, error) {
	result, err := r.primary.Run(ctx, req)
	if err != nil {
		slog.Warn("sandbox: primary runner failed, falling back",
			"primary", r.primary.Type(), "fallback", r.fallback.Type(), "err", err)

		fallbackResult, fallbackErr := r.fallback.Run(ctx, req)
		if fallbackErr != nil {
			return nil, fmt.Errorf("primary (%s): %w; fallback (%s): %v",
				r.primary.Type(), err, r.fallback.Type(), fallbackErr)
		}
		notice := fmt.Sprintf("[⚠ 云端沙箱调用失败: %v — 已自动降级到本地执行]", err)
		if fallbackResult.Stderr != "" {
			fallbackResult.Stderr = notice + "\n" + fallbackResult.Stderr
		} else {
			fallbackResult.Stderr = notice
		}
		return fallbackResult, nil
	}
	return result, nil
}

func (r *FallbackRunner) Close() error {
	e1 := r.primary.Close()
	e2 := r.fallback.Close()
	if e1 != nil {
		return e1
	}
	return e2
}
