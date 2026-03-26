//go:build linux || darwin

package plugin

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	goplugin "plugin"
	"strings"
)

// SOLoader discovers and loads Go shared library plugins (.so files) at runtime.
// Only available on Linux and macOS where Go's plugin build mode is supported.
//
// Usage:
//
//	loader := NewSOLoader("data/plugins", registry)
//	loaded := loader.LoadAll()
//
// Plugin .so files must export a variable named "Plugin" that implements
// the Plugin interface (or CognitivePlugin, UIPlugin, etc.):
//
//	// In the plugin's main.go:
//	package main
//
//	var Plugin = &MyPlugin{}
type SOLoader struct {
	dir      string
	registry *Registry
}

// NewSOLoader creates a .so plugin loader for the given directory.
func NewSOLoader(dir string, registry *Registry) *SOLoader {
	return &SOLoader{dir: dir, registry: registry}
}

// LoadAll scans the plugin directory for .so files and loads them.
// Returns the number of successfully loaded plugins.
func (l *SOLoader) LoadAll() int {
	entries, err := os.ReadDir(l.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0
		}
		slog.Warn("so_loader: read dir failed", "dir", l.dir, "err", err)
		return 0
	}

	loaded := 0
	for _, entry := range entries {
		if entry.IsDir() {
			// Check for .so files inside plugin subdirectories
			subDir := filepath.Join(l.dir, entry.Name())
			subEntries, _ := os.ReadDir(subDir)
			for _, se := range subEntries {
				if !se.IsDir() && strings.HasSuffix(se.Name(), ".so") {
					soPath := filepath.Join(subDir, se.Name())
					if err := l.loadOne(soPath); err != nil {
						slog.Warn("so_loader: load failed", "path", soPath, "err", err)
					} else {
						loaded++
					}
				}
			}
			continue
		}
		if strings.HasSuffix(entry.Name(), ".so") {
			soPath := filepath.Join(l.dir, entry.Name())
			if err := l.loadOne(soPath); err != nil {
				slog.Warn("so_loader: load failed", "path", soPath, "err", err)
			} else {
				loaded++
			}
		}
	}

	if loaded > 0 {
		slog.Info("so_loader: loaded Go shared library plugins", "count", loaded, "dir", l.dir)
	}
	return loaded
}

func (l *SOLoader) loadOne(path string) error {
	p, err := goplugin.Open(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}

	// Look for exported "Plugin" symbol
	sym, err := p.Lookup("Plugin")
	if err != nil {
		return fmt.Errorf("lookup Plugin in %s: %w", path, err)
	}

	// Try different interface levels (most specific first)
	switch v := sym.(type) {
	case *CognitivePlugin:
		l.registry.Register(*v)
		slog.Info("so_loader: registered CognitivePlugin", "name", (*v).Name(), "path", path)
	case *Plugin:
		l.registry.Register(*v)
		slog.Info("so_loader: registered Plugin", "name", (*v).Name(), "path", path)
	default:
		return fmt.Errorf("%s: Plugin symbol has unsupported type %T", path, sym)
	}

	return nil
}
