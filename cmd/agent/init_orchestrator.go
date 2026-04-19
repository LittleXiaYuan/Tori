package main

import (
	"log/slog"

	agentrt "yunque-agent/internal/agentcore/runtime"
	"yunque-agent/internal/agentcore/task"
	"yunque-agent/internal/controlplane/gateway"
	"yunque-agent/internal/orchestrator"
)

func initWorkOrchestrator(app *agentrt.App, gw *gateway.Gateway) {
	taskStore, ok := app.Get(agentrt.CompTaskStore)
	if !ok {
		slog.Info("orchestrator: task store not available, skipping")
		return
	}
	ts, _ := taskStore.(task.Store)
	if ts == nil {
		slog.Info("orchestrator: task store type mismatch, skipping")
		return
	}

	dispRaw, ok := app.Get("task_dispatcher")
	var dispatcher *task.Dispatcher
	if ok {
		dispatcher, _ = dispRaw.(*task.Dispatcher)
	}
	if dispatcher == nil {
		dispatcher = task.NewDispatcher(ts)
		app.Set("task_dispatcher", dispatcher)
		slog.Info("orchestrator: created new task dispatcher")
	}

	projectStore := orchestrator.NewProjectStore(app.Config.DataPath("projects"))

	launcher := orchestrator.NewLauncher()
	launcher.RegisterAdapter(&orchestrator.ClaudeCodeAdapter{})
	launcher.RegisterAdapter(orchestrator.NewCursorAdapter())
	launcher.RegisterAdapter(orchestrator.NewWindsurfAdapter())
	launcher.RegisterAdapter(orchestrator.NewTraeAdapter())
	orchestrator.AutoRegisterAdapters(launcher)
	slog.Info("orchestrator: adapters registered", "available", launcher.AvailableAdapters())

	daemon := orchestrator.NewDaemon(orchestrator.DaemonConfig{
		TaskStore:  ts,
		Dispatcher: dispatcher,
		Launcher:   launcher,
		Projects:   projectStore,
	})

	gw.SetOrchDaemon(daemon)
	gw.SetOrchLauncher(launcher)

	orchSkill := orchestrator.NewOrchestrateSkill(ts, dispatcher, projectStore)
	if app.SkillRegistry != nil {
		app.SkillRegistry.Register(orchSkill)
		slog.Info("orchestrator: registered orchestrate_task skill")
	}

	slog.Info("orchestrator: work orchestration initialized",
		"adapters", len(launcher.AvailableAdapters()),
		"daemon", "ready (not started)")
}
