package runtime

import (
	"context"
	"log/slog"
	"sync"
)

// Module is a self-contained subsystem that can be started, stopped, and
// queried at runtime. Modules are the building blocks of the hot-pluggable
// architecture described in COGNI-DESIGN.md (Profile layering).
type Module interface {
	Name() string
	Description() string
	Profile() string // minimum profile required: "lite", "standard", "full"
	Init(ctx context.Context, app *App) error
	Start(ctx context.Context) error
	Stop() error
	Status() ModuleStatus
}

// ModuleStatus reports the runtime state of a module.
type ModuleStatus struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Profile     string `json:"profile"`
	Enabled     bool   `json:"enabled"`
	Running     bool   `json:"running"`
	Error       string `json:"error,omitempty"`
}

// ModuleRegistry manages the lifecycle of optional modules.
type ModuleRegistry struct {
	mu      sync.RWMutex
	entries []*moduleEntry
}

type moduleEntry struct {
	mod     Module
	enabled bool
	running bool
	err     string
}

// NewModuleRegistry creates an empty registry.
func NewModuleRegistry() *ModuleRegistry {
	return &ModuleRegistry{}
}

// Register adds a module. It is NOT started until Enable+Start is called.
func (r *ModuleRegistry) Register(m Module) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, &moduleEntry{mod: m})
}

// InitAll initializes and starts modules whose profile <= the app profile.
// Modules listed in disabledNames are skipped.
func (r *ModuleRegistry) InitAll(ctx context.Context, app *App, profile string, disabledNames map[string]bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	order := map[string]int{"lite": 0, "standard": 1, "full": 2}
	profileLevel, ok := order[profile]
	if !ok {
		profileLevel = 1
	}

	for _, e := range r.entries {
		reqLevel, ok := order[e.mod.Profile()]
		if !ok {
			reqLevel = 1
		}
		if reqLevel > profileLevel {
			slog.Info("module skipped (profile)", "module", e.mod.Name(), "requires", e.mod.Profile(), "current", profile)
			continue
		}
		if disabledNames[e.mod.Name()] {
			slog.Info("module skipped (disabled)", "module", e.mod.Name())
			continue
		}
		if err := e.mod.Init(ctx, app); err != nil {
			e.err = err.Error()
			slog.Warn("module init failed", "module", e.mod.Name(), "err", err)
			continue
		}
		if err := e.mod.Start(ctx); err != nil {
			e.err = err.Error()
			slog.Warn("module start failed", "module", e.mod.Name(), "err", err)
			continue
		}
		e.enabled = true
		e.running = true
		slog.Info("module started", "module", e.mod.Name(), "profile", e.mod.Profile())
	}
}

// StopAll gracefully stops all running modules in reverse order.
func (r *ModuleRegistry) StopAll() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := len(r.entries) - 1; i >= 0; i-- {
		e := r.entries[i]
		if !e.running {
			continue
		}
		if err := e.mod.Stop(); err != nil {
			slog.Warn("module stop failed", "module", e.mod.Name(), "err", err)
		}
		e.running = false
	}
}

// List returns the status of all registered modules.
func (r *ModuleRegistry) List() []ModuleStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]ModuleStatus, len(r.entries))
	for i, e := range r.entries {
		out[i] = ModuleStatus{
			Name:        e.mod.Name(),
			Description: e.mod.Description(),
			Profile:     e.mod.Profile(),
			Enabled:     e.enabled,
			Running:     e.running,
			Error:       e.err,
		}
	}
	return out
}

// IsEnabled returns whether a module is currently enabled and running.
func (r *ModuleRegistry) IsEnabled(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, e := range r.entries {
		if e.mod.Name() == name {
			return e.running
		}
	}
	return false
}
