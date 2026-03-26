//go:build windows

package plugin

import "log/slog"

// SOLoader is a no-op stub on Windows where Go's plugin build mode is not supported.
// On Linux/macOS, the real implementation in so_loader.go is used instead.
type SOLoader struct {
	dir      string
	registry *Registry
}

// NewSOLoader creates a (no-op) .so plugin loader on Windows.
func NewSOLoader(dir string, registry *Registry) *SOLoader {
	return &SOLoader{dir: dir, registry: registry}
}

// LoadAll is a no-op on Windows. Returns 0.
func (l *SOLoader) LoadAll() int {
	slog.Debug("so_loader: .so plugins not supported on Windows (use binary plugins instead)")
	return 0
}
