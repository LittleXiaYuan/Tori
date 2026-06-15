package main

import (
	"context"
	"fmt"
	"log/slog"

	"yunque-agent/internal/agentcore/memory"
	agentrt "yunque-agent/internal/agentcore/runtime"
	"yunque-agent/internal/agentcore/session"
	"yunque-agent/internal/agentcore/task"
	"yunque-agent/internal/config"
	"yunque-agent/internal/controlplane/gateway"
	"yunque-agent/internal/execution/channel"
	reflectpkg "yunque-agent/internal/experimental/reflect"
	iledger "yunque-agent/internal/ledger"
	"yunque-agent/pkg/cognisdk"

	"yunque-agent/internal/ledgercore"
)

func initCognitionWiring(
	app *agentrt.App,
	gw *gateway.Gateway,
	cfg *config.Config,
	convStore *session.Store,
	taskStore task.Store,
	taskRunner *task.Runner,
	channelReg *channel.Registry,
	learningLoop *reflectpkg.LearningLoop,
	typedLdg *ledger.Ledger,
	costAwareLLM func(ctx context.Context, system, user string) (string, error),
) *task.ThreadManager {
	p := app.Planner

	templateStore := task.NewTemplateStore(cfg.DataPath("templates"))
	if typedLdg != nil {
		templateStore.SetKVStore(iledger.NewKVConfigStore(typedLdg, "task_templates"))
	}
	gw.SetTemplateStore(templateStore)

	workMemMgr := task.NewWorkingMemoryManagerWithPersistence(
		llmChatFunc(app.LLMClient, LowLLMTemperature), cfg.DataDir,
	)
	taskRunner.WorkMem = workMemMgr
	gw.SetWorkingMemoryManager(workMemMgr)

	threadMgr := task.NewThreadManager(convStore, cfg.DataDir)
	threadMgr.SetChannelSend(func(ctx context.Context, channelType, target, content string) error {
		ch, ok := channelReg.Get(channelType)
		if !ok {
			return fmt.Errorf("channel %s not registered", channelType)
		}
		return ch.Send(ctx, target, channel.Reply{Content: content, Format: "text"})
	})
	gw.SetThreadManager(threadMgr)

	if typedLdg != nil {
		migrator := iledger.NewKVMigrator(typedLdg)
		_ = migrator.MigrateFile("thread", "threads", cfg.DataPath("threads.json"))
		_ = migrator.MigrateFile("working_memory", "data", cfg.DataPath("working_memory.json"))
		threadMgr.SetKVStore(iledger.NewKVConfigStore(typedLdg, "thread"))
		workMemMgr.SetKVStore(iledger.NewKVConfigStore(typedLdg, "working_memory"))
		slog.Info("thread/working_memory wired to Ledger KV")
	}

	taskRunner.OnTaskEvent(func(event, taskID, detail string) {
		if !threadMgr.HasThread(taskID) {
			return
		}
		switch event {
		case "step_completed":
			threadMgr.PostStepResult(taskID, "system", 0, "", detail)
		case "step_failed":
			threadMgr.PostStepFailed(taskID, "system", 0, "", detail)
		case "task_completed":
			threadMgr.PostTaskCompleted(taskID, "system", detail)
		case "task_failed":
			threadMgr.PostTaskFailed(taskID, "system", detail)
		}
	})

	app.Reverie.SetWriteMemory(func(ctx context.Context, fact string) error {
		return app.MemManager.AddMid(ctx, "system", memory.Item{Key: "reverie_insight", Value: fact, Source: "reverie"})
	})
	app.Reverie.SetCreateTask(func(ctx context.Context, title, desc string) error {
		_, err := taskStore.Create(task.CreateRequest{Title: title, Description: desc, TenantID: "system"})
		return err
	})
	app.Reverie.SetUpdateProfile(func(ctx context.Context, key, value string) error {
		app.EditableMem.AddBlock(key, value, 2000)
		return nil
	})

	experienceStore := reflectpkg.NewExperienceStore(cfg.DataPath("experience.json"))
	taskReflector := reflectpkg.NewTaskReflector(app.LLMClient, experienceStore)
	gw.SetExperienceStore(experienceStore)
	// Expose the single experience store app-wide so the CogniKernel offline
	// dream loop sinks into the SAME store the planner reads for strategy
	// injection (anti-fragmentation rule — one truth source, not a new one).
	app.Set(agentrt.CompExperienceStore, experienceStore)
	p.SetStrategyContext(func() string {
		return experienceStore.CompileStrategies(10)
	})
	p.SetStrategyContextFor(func(query string) string {
		if scoped := experienceStore.CompileStrategiesForQuery(query, 10); scoped != "" {
			return scoped
		}
		return experienceStore.CompileStrategies(10)
	})
	// Tier 0 microkernel: let ENABLED capability packs inject context into the
	// prompt, so a Pack's enablement actually flows into the agent's reasoning
	// (not just its HTTP routes). Packs opt in by implementing ContextProvider.
	p.SetPackContext(gw.PackContext)

	if typedLdg != nil {
		migrator := iledger.NewKVMigrator(typedLdg)
		_ = migrator.MigrateFile("workload_feedback", "data", cfg.DataPath("experience.json"))
		experienceStore.SetKVStore(iledger.NewKVConfigStore(typedLdg, "workload_feedback"))
		slog.Info("experience store wired to Ledger KV", "namespace", "workload_feedback")
	}

	packDir := cfg.DataPath("cognisdk")
	localPacks, packErrs, err := cognisdk.LoadPacksFromDir(packDir)
	if err != nil {
		slog.Warn("cognisdk: failed to load local packs", "dir", packDir, "err", err)
	} else if len(packErrs) > 0 {
		for _, perr := range packErrs {
			slog.Warn("cognisdk: local pack skipped", "path", perr.Path, "err", perr.Err)
		}
	}
	sdkPacks := append([]cognisdk.PackManifest{}, cognisdk.BuiltinPacks()...)
	sdkPacks = append(sdkPacks, localPacks...)
	beliefSDK := cognisdk.NewHostAdapter(cognisdk.Config{Packs: sdkPacks})
	if p != nil {
		p.SetBeliefContext(beliefSDK.BuildContext)
		slog.Info("cognisdk: planner belief context wired", "builtin_packs", len(cognisdk.BuiltinPacks()), "local_packs", len(localPacks))
	}

	learningLoop.SetOnLesson(func(category, outcome, lesson, ctx string, tags []string) {
		experienceStore.Add(reflectpkg.Experience{
			Source:   "interaction",
			Category: category,
			Outcome:  outcome,
			Lesson:   lesson,
			Context:  ctx,
			Tags:     tags,
		})
	})

	taskRunner.OnTaskEvent(func(event, taskID, detail string) {
		if event != "task_completed" && event != "task_failed" {
			return
		}
		t, ok := taskStore.Get(taskID)
		if !ok || t == nil {
			return
		}
		trace := reflectpkg.TaskTrace{
			TaskID: t.ID, Title: t.Title, Description: t.Description, Outcome: string(t.Status),
		}
		if t.StartedAt != nil && t.FinishedAt != nil {
			trace.Duration = t.FinishedAt.Sub(*t.StartedAt)
		}
		for _, s := range t.Steps {
			trace.Steps = append(trace.Steps, reflectpkg.StepTrace{
				Action: s.Action, SkillName: s.SkillName, Status: string(s.Status),
				Error: s.Error, Retries: s.RetryCount, GapType: s.GapType,
			})
		}
		taskReflector.AfterTask(context.Background(), trace)
	})

	return threadMgr
}
