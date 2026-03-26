package plugin

import "context"

// Shutdownable is an optional interface for plugins that need cleanup.
type Shutdownable interface {
	Shutdown() error
}

// ShutdownAll calls Shutdown on any plugins that implement Shutdownable.
func (r *Registry) ShutdownAll(ctx ...context.Context) {
	for _, e := range r.plugins {
		if s, ok := e.plugin.(Shutdownable); ok {
			_ = s.Shutdown()
		}
	}
}
