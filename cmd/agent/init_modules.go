package main

import (
	"context"
	"log/slog"
	"strings"

	agentrt "yunque-agent/internal/agentcore/runtime"
)

func registerModules(app *agentrt.App) {
	cfg := app.Config

	app.Modules.Register(&cogniModule{})
	app.Modules.Register(&heartbeatModule{})
	app.Modules.Register(&federationModule{})

	disabled := make(map[string]bool)
	if cfg.DisabledModules != "" {
		for _, name := range strings.Split(cfg.DisabledModules, ",") {
			name = strings.TrimSpace(name)
			if name != "" {
				disabled[name] = true
			}
		}
	}

	app.Modules.InitAll(context.Background(), app, cfg.Profile, disabled)

	modules := app.Modules.List()
	running := 0
	for _, m := range modules {
		if m.Running {
			running++
		}
	}
	slog.Info("modules initialized", "total", len(modules), "running", running, "profile", cfg.Profile)
}
