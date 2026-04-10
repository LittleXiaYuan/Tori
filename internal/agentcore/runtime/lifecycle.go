package runtime

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"yunque-agent/pkg/safego"
)

// startFunc is called during Start.
type startFunc func(ctx context.Context) error

// stopFunc is called during Stop.
type stopFunc func(ctx context.Context) error

// lifecycleEntry is a named start/stop pair.
type lifecycleEntry struct {
	name  string
	start startFunc
	stop  stopFunc
}

// Lifecycle manages ordered startup and shutdown of components.
type Lifecycle struct {
	mu      sync.Mutex
	entries []lifecycleEntry
}

// NewLifecycle creates a new lifecycle manager.
func NewLifecycle() *Lifecycle {
	return &Lifecycle{}
}

// RegisterFunc registers a named start/stop pair.
// Either start or stop may be nil.
func (lc *Lifecycle) RegisterFunc(name string, start startFunc, stop stopFunc) {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	lc.entries = append(lc.entries, lifecycleEntry{name: name, start: start, stop: stop})
}

// Start calls all registered start functions in order.
func (lc *Lifecycle) Start(ctx context.Context) error {
	lc.mu.Lock()
	entries := make([]lifecycleEntry, len(lc.entries))
	copy(entries, lc.entries)
	lc.mu.Unlock()

	for _, e := range entries {
		if e.start != nil {
			if err := e.start(ctx); err != nil {
				return err
			}
			slog.Info("lifecycle: started", "component", e.name)
		}
	}
	return nil
}

// Stop calls all registered stop functions in reverse order.
// Each component gets a limited time to shutdown (10s by default).
func (lc *Lifecycle) Stop(ctx context.Context) {
	lc.mu.Lock()
	entries := make([]lifecycleEntry, len(lc.entries))
	copy(entries, lc.entries)
	lc.mu.Unlock()

	const perComponentTimeout = 10 * time.Second
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		if e.stop != nil {
			stopCtx, cancel := context.WithTimeout(ctx, perComponentTimeout)
			done := make(chan error, 1)
			safego.Go("lifecycle-stop-"+e.name, func() { done <- e.stop(stopCtx) })
			select {
			case err := <-done:
				if err != nil {
					slog.Warn("lifecycle: stop error", "component", e.name, "err", err)
				} else {
					slog.Info("lifecycle: stopped", "component", e.name)
				}
			case <-stopCtx.Done():
				slog.Error("lifecycle: stop timeout, forcing next", "component", e.name, "timeout", perComponentTimeout)
			}
			cancel()
		}
	}
}
