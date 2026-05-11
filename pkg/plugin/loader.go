package plugin

import (
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Loader watches a plugins directory and hot-reloads script plugins.
type Loader struct {
	dir      string
	registry *Registry
	mu       sync.Mutex
	stopCh   chan struct{}
	onChange func() // callback when plugins change
}

// NewLoader creates a plugin loader for the given directory.
func NewLoader(pluginsDir string, registry *Registry, onChange func()) *Loader {
	if pluginsDir == "" {
		pluginsDir = "data/plugins"
	}
	os.MkdirAll(pluginsDir, 0755)
	return &Loader{
		dir:      pluginsDir,
		registry: registry,
		stopCh:   make(chan struct{}),
		onChange: onChange,
	}
}

// LoadAll scans the plugins directory and loads all valid plugins.
func (l *Loader) LoadAll() int {
	l.mu.Lock()
	defer l.mu.Unlock()

	entries, err := os.ReadDir(l.dir)
	if err != nil {
		slog.Error("read plugins dir", "err", err, "dir", l.dir)
		return 0
	}

	loaded := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pluginDir := filepath.Join(l.dir, e.Name())

		// Check for manifest
		hasManifest := false
		for _, name := range []string{"plugin.yaml", "plugin.json"} {
			if _, err := os.Stat(filepath.Join(pluginDir, name)); err == nil {
				hasManifest = true
				break
			}
		}
		if !hasManifest {
			continue
		}

		sp, err := LoadScriptPlugin(pluginDir)
		if err != nil {
			slog.Error("load plugin", "dir", pluginDir, "err", err)
			continue
		}

		// Preserve enabled state if plugin was already registered
		wasEnabled := l.registry.IsEnabled(sp.Name())
		_, existed := l.registry.Get(sp.Name())

		slot := sp.Manifest().Slot
		if slot != "" {
			if err := l.registry.RegisterWithSlot(sp, slot); err != nil {
				slog.Warn("plugin slot conflict, skipping", "plugin", sp.Name(), "slot", slot, "err", err)
				continue
			}
		} else {
			l.registry.Register(sp)
		}
		if existed && !wasEnabled {
			l.registry.SetEnabled(sp.Name(), false)
		}
		loaded++
	}

	slog.Info("plugins loaded", "count", loaded, "dir", l.dir)
	return loaded
}

// Watch starts a background goroutine that periodically checks for plugin changes.
func (l *Loader) Watch(interval time.Duration) {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		lastState := l.snapshot()
		for {
			select {
			case <-l.stopCh:
				return
			case <-ticker.C:
				current := l.snapshot()
				if !stateEqual(lastState, current) {
					slog.Info("plugin changes detected, reloading")
					l.LoadAll()
					lastState = current
					if l.onChange != nil {
						l.onChange()
					}
				}
			}
		}
	}()
}

// Stop stops the background watcher.
func (l *Loader) Stop() {
	select {
	case l.stopCh <- struct{}{}:
	default:
	}
}

// Dir returns the plugins directory path.
func (l *Loader) Dir() string { return l.dir }

// pluginState captures the modification state of a plugin directory.
type pluginState struct {
	name    string
	modTime time.Time
}

func (l *Loader) snapshot() []pluginState {
	entries, err := os.ReadDir(l.dir)
	if err != nil {
		return nil
	}
	var states []pluginState
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		for _, mf := range []string{"plugin.yaml", "plugin.json"} {
			info, err := os.Stat(filepath.Join(l.dir, e.Name(), mf))
			if err == nil {
				states = append(states, pluginState{name: e.Name(), modTime: info.ModTime()})
				break
			}
		}
	}
	return states
}

func stateEqual(a, b []pluginState) bool {
	if len(a) != len(b) {
		return false
	}
	m := make(map[string]time.Time, len(a))
	for _, s := range a {
		m[s.name] = s.modTime
	}
	for _, s := range b {
		if t, ok := m[s.name]; !ok || !t.Equal(s.modTime) {
			return false
		}
	}
	return true
}
