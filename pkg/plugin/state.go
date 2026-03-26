package plugin

import "context"

// PluginEnv provides environment information to plugins during initialization.
type PluginEnv struct {
	DataDir    string                                                  // base data directory
	ConfigDir  string                                                  // config directory
	PluginsDir string                                                  // plugins directory
	LLMCall    func(ctx context.Context, system, user string) (string, error) // LLM helper
}

// PluginStateManager tracks plugin state persistence.
type PluginStateManager struct {
	stateDir string
}

// NewPluginStateManager creates a new state manager.
func NewPluginStateManager(stateDir string) *PluginStateManager {
	return &PluginStateManager{stateDir: stateDir}
}

// InitAll calls Init on plugins that implement the Initializable interface.
func (r *Registry) InitAll(ctx context.Context, stateMgr *PluginStateManager, env *PluginEnv) error {
	for _, e := range r.plugins {
		if !e.enabled {
			continue
		}
		if init, ok := e.plugin.(Initializable); ok {
			if err := init.Init(ctx, env); err != nil {
				return err
			}
		}
	}
	return nil
}

// Initializable is an optional interface for plugins that need initialization.
type Initializable interface {
	Init(ctx context.Context, env *PluginEnv) error
}
