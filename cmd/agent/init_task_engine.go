package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"yunque-agent/internal/agentcore/adaptive"
	"yunque-agent/internal/agentcore/approval"
	"yunque-agent/internal/agentcore/audit"
	"yunque-agent/internal/agentcore/costtrack"
	"yunque-agent/internal/agentcore/cron"
	"yunque-agent/internal/agentcore/embeddings"
	"yunque-agent/internal/agentcore/guardrails"
	"yunque-agent/internal/agentcore/knowledge"
	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/memory"
	"yunque-agent/internal/agentcore/planner"
	"yunque-agent/internal/agentcore/rbac"
	agentrt "yunque-agent/internal/agentcore/runtime"
	"yunque-agent/internal/agentcore/selfheal"
	"yunque-agent/internal/agentcore/session"
	"yunque-agent/internal/agentcore/state"
	"yunque-agent/internal/agentcore/task"
	"yunque-agent/internal/agentcore/tools"
	"yunque-agent/internal/agentcore/trigger"
	"yunque-agent/internal/agentcore/workflow"
	"yunque-agent/internal/config"
	"yunque-agent/internal/controlplane/gateway"
	"yunque-agent/internal/execution/channel"
	"yunque-agent/internal/execution/sandbox"
	reflectpkg "yunque-agent/internal/experimental/reflect"
	iledger "yunque-agent/internal/ledger"
	"yunque-agent/internal/observe"
	"yunque-agent/pkg/skills"
	"yunque-agent/plugins/general"

	"github.com/LittleXiaYuan/ledger"
)

func initTaskEngine(
	app *agentrt.App,
	gw *gateway.Gateway,
	costTracker *costtrack.Tracker,
	knowledgeStore *knowledge.Store,
	convStore *session.Store,
	auditChain *audit.Chain,
) error {
	cfg := app.Config
	p := app.Planner
	channelReg := app.MustGet(agentrt.CompChannelReg).(*channel.Registry)
	learningLoop := app.MustGet("learning_loop").(*reflectpkg.LearningLoop)
	emotionShiftDetector := app.MustGet("emotion_shift_detector").(*planner.EmotionShiftDetector)

	var typedLdg *ledger.Ledger
	if ldgRaw, ok := app.Get("github.com/LittleXiaYuan/ledger"); ok {
		typedLdg, _ = ldgRaw.(*ledger.Ledger)
	}

	// ── Tools ──
	toolsMgr := tools.NewProcessManager()
	gw.SetToolsManager(toolsMgr)

	// ── Task Runtime ──
	taskStore := iledger.NewLedgerStore(typedLdg, cfg.DataPath("tasks"))
	app.Set(agentrt.CompTaskStore, taskStore)

	costAwareLLM := func(ctx context.Context, system, user string) (string, error) {
		msgs := []llm.Message{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		}
		start := time.Now()
		reply, err := app.LLMClient.Chat(ctx, msgs, DefaultLLMTemperature)
		elapsed := time.Since(start)
		if tag := task.TaskCostFromContext(ctx); tag != nil {
			estIn := len(system+user)/4 + 50
			estOut := len(reply)/4 + 50
			costTracker.RecordExt(costtrack.RecordOpts{
				Model: cfg.LLMModel, TaskID: tag.TaskID, StepID: tag.StepID,
				SkillName: tag.SkillName, RunnerType: "task",
				TokensIn: estIn, TokensOut: estOut, Latency: elapsed,
			})
			app.Metrics.RecordLLMCall(cfg.LLMModel, elapsed, int64(estIn), int64(estOut), err)
		}
		return reply, err
	}

	taskRunner := task.NewRunner(taskStore, app.SkillRegistry, costAwareLLM, &skills.Environment{
		LLMCall: costAwareLLM,
	})
	gw.SetTaskStore(taskStore)
	gw.SetTaskRunner(taskRunner)
	app.Set(agentrt.CompTaskRunner, taskRunner)

	// ── Deep Soul Layer ──
	skillLifecycleRaw, _ := app.Get("skill_lifecycle")
	initSoulLayer(soulDeps{
		app:            app,
		costAwareLLM:   costAwareLLM,
		typedLdg:       typedLdg,
		skillLifecycle: skillLifecycleRaw.(*selfheal.Lifecycle),
	})

	if recovered := taskRunner.RecoverAll(); recovered > 0 {
		slog.Warn("task recovery: marked interrupted tasks", "count", recovered)
	}

	// ── Cron ──
	cronMgr := cron.NewManager(cfg.DataDir, func(ctx context.Context, job *cron.Job) (string, error) {
		if job.Payload.Kind == cron.PayloadAgentTurn {
			return app.LLMClient.Chat(ctx, []llm.Message{{Role: "user", Content: job.Payload.Message}}, DefaultLLMTemperature)
		}
		return "event: " + job.Payload.Message, nil
	})
	cronMgr.SetSessionFactory(func(job *cron.Job, runID string) string {
		return fmt.Sprintf("cron_%s_%s", job.ID[:8], runID[:8])
	})
	if err := cronMgr.Start(); err != nil {
		slog.Warn("cron manager start failed", "err", err)
	}
	gw.SetCronManager(cronMgr)
	app.Set(agentrt.CompCronMgr, cronMgr)

	// ── Triggers ──
	var wfActionHandler *trigger.WorkflowActionHandler

	triggerRT := trigger.NewRuntime(
		func(ctx context.Context, t *trigger.Trigger, event *trigger.EventPayload) error {
			if t.Action.Type == trigger.ActionAgentTurn && t.Action.Message != "" {
				_, err := app.LLMClient.Chat(ctx, []llm.Message{
					{Role: "user", Content: t.Action.Message},
				}, DefaultLLMTemperature)
				return err
			}
			if t.Action.Type == trigger.ActionRunWorkflow && wfActionHandler != nil {
				return wfActionHandler.Handle(ctx, t, event)
			}
			return nil
		},
		nil,
	)
	triggerRT.Start()
	gw.SetTriggerRuntime(triggerRT)

	triggerStore := trigger.NewStore(cfg.DataPath("triggers"))
	if ldgRaw, ok := app.Get("github.com/LittleXiaYuan/ledger"); ok {
		if ldg, ok := ldgRaw.(*ledger.Ledger); ok {
			tmigrator := iledger.NewKVMigrator(ldg)
			_ = tmigrator.MigrateFile("trigger", "data", cfg.DataPath("triggers/triggers.json"))
			triggerStore.SetKVStore(iledger.NewKVConfigStore(ldg, "trigger"))
			slog.Info("trigger store wired to Ledger KV")
		}
	}
	triggerExecutor := trigger.NewExecutor(triggerStore)
	triggerMgr := trigger.NewManager(triggerStore, triggerExecutor, cronMgr)
	gw.SetTriggerManager(triggerMgr)
	app.Set(agentrt.CompTriggerMgr, triggerMgr)

	// ── Workflow Engine ──
	wfStore := workflow.NewJSONStore(cfg.DataPath("workflows"))
	if ldgRaw, ok := app.Get("github.com/LittleXiaYuan/ledger"); ok {
		if ldg, ok := ldgRaw.(*ledger.Ledger); ok {
			wfStore.SetKVStore(iledger.NewKVConfigStore(ldg, "workflow"))
			slog.Info("workflow store wired to Ledger KV")
		}
	}
	wfEngine := workflow.NewEngine(wfStore, app.SkillRegistry,
		func(ctx context.Context, name string, args map[string]any) (string, error) {
			sk, ok := app.SkillRegistry.Get(name)
			if !ok {
				return "", fmt.Errorf("skill %q not found", name)
			}
			return sk.Execute(ctx, args, &skills.Environment{LLMCall: costAwareLLM})
		},
		costAwareLLM,
	)
	gw.SetWorkflowStore(wfStore)
	gw.SetWorkflowEngine(wfEngine)
	slog.Info("workflow engine initialized", "dir", cfg.DataPath("workflows"))

	wireWorkflowExecutors(gw, wfEngine, knowledgeStore, cfg)
	wireWorkflowSkills(app, gw, wfStore, wfEngine, triggerRT, costAwareLLM)

	wfActionHandler = trigger.NewWorkflowActionHandler(
		func(ctx context.Context, defID, tenantID string, vars map[string]any) (string, error) {
			inst, err := wfStore.CreateInstance(defID, tenantID, vars)
			if err != nil {
				return "", err
			}
			go wfEngine.Run(context.Background(), inst.ID)
			return inst.ID, nil
		},
	)

	// ── RBAC ──
	rbacEnforcer := rbac.NewEnforcer()
	gw.SetRBACEnforcer(rbacEnforcer)
	slog.Info("rbac enforcer initialized", "roles", len(rbacEnforcer.ListRoles()))

	// ── Approval + SSE ──
	approvalMgr := approval.NewManager(approval.DefaultPolicy())
	if typedLdg != nil {
		amigrator := iledger.NewKVMigrator(typedLdg)
		_ = amigrator.MigrateFile("approval", "rules", cfg.DataPath("approval_rules.json"))
		approvalMgr.Rules().SetKVStore(iledger.NewKVConfigStore(typedLdg, "approval"))
		slog.Info("approval rules wired to Ledger KV")
	}
	gw.SetApprovalManager(approvalMgr)

	shellPolicy := tools.NewShellExecPolicy(approvalMgr, toolsMgr)
	gw.SetShellPolicy(shellPolicy)
	slog.Info("shell exec policy wired to approval manager")

	sseBroker := gateway.NewSSEBroker()
	gw.SetSSEBroker(sseBroker)

	approvalMgr.OnRequest(func(req *approval.Request) {
		sseBroker.Broadcast(gateway.SSEEvent{
			Type: "approval.request",
			Data: req,
		})
	})

	eventTrail := observe.NewAuditTrail(10000)
	gw.SetEventTrail(eventTrail)

	slog.Info("approval + SSE initialized")

	wfEngine.OnEvent(func(evt observe.AgentEvent) {
		sseBroker.Broadcast(gateway.SSEEvent{
			Type: evt.QualifiedType(),
			Data: evt,
		})
	})

	// ── Security Guards ──
	toolGuard := guardrails.NewToolGuard(guardrails.DefaultToolGuardConfig())
	egressGuard := guardrails.NewEgressGuard(guardrails.DefaultEgressGuardConfig())
	if auditChain != nil {
		toolGuard.SetAudit(auditChain)
		egressGuard.SetAudit(auditChain)
	}
	gw.SetToolGuard(toolGuard)
	gw.SetEgressGuard(egressGuard)

	// ── Ledger State Engine ──
	if typedLdg != nil {
		initLedgerStateEngine(app, typedLdg, taskStore, taskRunner)
	} else {
		slog.Warn("ledger state engine skipped (no ledger)")
	}

	// ── Gap Analyzer + Skill Generator ──
	gapAnalyzer := task.NewGapAnalyzer(llmChatFunc(app.LLMClient, DefaultLLMTemperature))
	if typedLdg != nil {
		gapAnalyzer.SetPersist(func(ctx context.Context, rec task.GapRecord) error {
			payload, _ := json.Marshal(rec)
			return typedLdg.Events.Append(ctx, &ledger.Event{
				TaskID:    rec.TaskID,
				Kind:      "gap.detected",
				Actor:     "gap_analyzer",
				Payload:   payload,
				CreatedAt: rec.OccurredAt,
			})
		})
		_ = gapAnalyzer.LoadRecords(context.Background(), func(ctx context.Context) ([]task.GapRecord, error) {
			events, err := typedLdg.Events.Query(ctx, ledger.EventQuery{
				Kinds: []ledger.EventKind{"gap.detected"},
				Limit: 500,
			})
			if err != nil {
				return nil, err
			}
			var records []task.GapRecord
			for _, e := range events {
				var rec task.GapRecord
				if err := json.Unmarshal(e.Payload, &rec); err == nil {
					records = append(records, rec)
				}
			}
			return records, nil
		})
	}
	taskRunner.SetGapAnalyzer(gapAnalyzer)
	gw.SetGapAnalyzer(gapAnalyzer)
	app.Set(agentrt.CompGapAnalyzer, gapAnalyzer)

	skillGenerator := task.NewSkillGenerator(
		llmChatFunc(app.LLMClient, DefaultLLMTemperature),
		app.SkillRegistry,
		&skills.Environment{LLMCall: llmChatFunc(app.LLMClient, DefaultLLMTemperature)},
	)
	taskRunner.SetSkillGenerator(skillGenerator)

	// ── State Kernel ──
	stateKernel := state.NewKernel(cfg.DataDir)
	if ldgRaw, ok := app.Get("github.com/LittleXiaYuan/ledger"); ok {
		if ldg, ok := ldgRaw.(*ledger.Ledger); ok {
			stateKernel.SetKVStore(iledger.NewKVConfigStore(ldg, "state_kernel"))
		}
	}
	stateKernel.UpdateCapabilities(state.CapSnapshot{TotalSkills: len(app.SkillRegistry.All())})
	gw.SetStateKernel(stateKernel)
	p.SetStateContext(stateKernel.CompileForLLM)
	app.Set(agentrt.CompStateKernel, stateKernel)

	// Wire task events → State Kernel + SSE
	taskRunner.OnTaskEvent(func(event, taskID, detail string) {
		stateKernel.RecordAction(state.ActionRecord{
			Action: event + ": " + taskID, Result: detail,
			Success: event == "task_completed" || event == "step_completed",
		})
		gapCount := 0
		if gapAnalyzer != nil {
			gapCount = len(gapAnalyzer.Records("", true))
		}
		stateKernel.UpdateCapabilities(state.CapSnapshot{
			TotalSkills:    len(app.SkillRegistry.All()),
			UnresolvedGaps: gapCount,
		})
		_ = stateKernel.Save()
	})

	taskRunner.OnTaskEvent(func(event, taskID, detail string) {
		sseBroker.Broadcast(gateway.SSEEvent{
			Type: "task." + event,
			Data: map[string]string{
				"task_id": taskID,
				"event":   event,
				"detail":  detail,
			},
		})
	})

	// ── Templates / Working Memory / Threads ──
	templateStore := task.NewTemplateStore(cfg.DataPath("templates"))
	if ldgRaw, ok := app.Get("github.com/LittleXiaYuan/ledger"); ok {
		if ldg, ok := ldgRaw.(*ledger.Ledger); ok {
			templateStore.SetKVStore(iledger.NewKVConfigStore(ldg, "task_templates"))
		}
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

	// ── Reverie Action Callbacks ──
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

	// ── Reflection Loop ──
	experienceStore := reflectpkg.NewExperienceStore(cfg.DataPath("experience.json"))
	taskReflector := reflectpkg.NewTaskReflector(app.LLMClient, experienceStore)
	gw.SetExperienceStore(experienceStore)
	p.SetStrategyContext(func() string {
		return experienceStore.CompileStrategies(10)
	})

	if ldgRaw, ok := app.Get("github.com/LittleXiaYuan/ledger"); ok {
		if ldg, ok := ldgRaw.(*ledger.Ledger); ok {
			migrator := iledger.NewKVMigrator(ldg)
			_ = migrator.MigrateFile("experience", "data", cfg.DataPath("experience.json"))
			experienceStore.SetKVStore(iledger.NewKVConfigStore(ldg, "experience"))
			slog.Info("experience store wired to Ledger KV")
		}
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

	// ── Trigger Callbacks ──
	taskRunner.OnTaskEvent(func(event, taskID, detail string) {
		if event == "task_completed" || event == "task_failed" {
			evName := trigger.EventTaskCompleted
			if event == "task_failed" {
				evName = trigger.EventTaskFailed
			}
			payload := trigger.EventPayload{
				Event: evName, Text: detail,
				Data: map[string]any{"task_id": taskID}, TaskID: taskID, Timestamp: time.Now(),
			}
			triggerRT.Emit(context.Background(), payload)
			triggerMgr.Emit(context.Background(), payload)
		}
		triggerMgr.Emit(context.Background(), trigger.EventPayload{
			Event: trigger.EventTaskStatusChanged, Text: detail,
			Data: map[string]any{"task_id": taskID, "event": event}, TaskID: taskID, Timestamp: time.Now(),
		})
	})

	wireTriggerExecutor(triggerExecutor, taskStore, taskRunner, threadMgr, channelReg, app, costAwareLLM)

	triggerMgr.SetConditionEvaluator(trigger.NewConditionEvaluator(&trigger.DataSource{
		GetTaskStatus: func(taskID string) (string, error) {
			t, ok := taskStore.Get(taskID)
			if !ok {
				return "", fmt.Errorf("task not found: %s", taskID)
			}
			return string(t.Status), nil
		},
		GetTodayCost: costTracker.TodayCost,
		GetMonthCost: costTracker.MonthCost,
		GetMemoryCount: func(tenantID string) int {
			stats := app.Orchestrator.Stats(tenantID)
			return stats.ShortCount + stats.MidCount + stats.LongCount
		},
	}))
	triggerMgr.Start()

	// ── Cognitive Triggers ──
	app.Reverie.SetOnThought(func(thought planner.Thought) {
		app.Metrics.Cognitive().ReverieThink.Add(1)
		if thought.Significance >= ReverieMinSignificance {
			triggerMgr.EmitCognitive(context.Background(), "reverie_insight", map[string]any{
				"thought_id": thought.ID, "category": thought.Category,
				"significance": thought.Significance, "content": thought.Content,
				"trigger": thought.Trigger,
			})
		}
	})
	if emotionShiftDetector != nil {
		emotionShiftDetector.SetOnShift(func(from, to string, confidence float64) {
			triggerMgr.EmitCognitive(context.Background(), "emotion_shift", map[string]any{
				"from": from, "to": to, "confidence": confidence,
			})
		})
	}

	// ── Final Gateway Wiring ──
	gw.SetOrchestrator(app.Orchestrator)
	guardPipeline := app.MustGet(agentrt.CompGuardPipeline).(*guardrails.Pipeline)
	gw.SetZhGuard(guardPipeline)
	adaptiveRaw, _ := app.Get(agentrt.CompAdaptiveLoop)
	if adaptiveRaw != nil {
		gw.SetAdaptiveLoop(adaptiveRaw.(*adaptive.Loop))
	}
	gw.SetProviderRegistry(app.Providers)

	if app.Ledger != nil {
		modelKV := iledger.NewKVConfigStore(app.Ledger, "models")
		gw.SetModelKVStore(modelKV)
		slog.Info("models: using Ledger KV for persistence")

		usageKV := iledger.NewKVConfigStore(app.Ledger, "usage")
		gw.SetUsageKVStore(usageKV)
		slog.Info("usage: using Ledger KV for persistence")
	}

	gw.SetOutputDir(filepath.Join(cfg.DataDir, "output"))

	// Store remaining refs
	app.Set("sched", app.MustGet(agentrt.CompScheduler))
	app.Set("conv_store", convStore)
	app.Set("trigger_rt", triggerRT)
	app.Set("trigger_mgr", triggerMgr)
	app.Set("cron_mgr", cronMgr)
	app.Set("skill_optimizer", app.SkillOptimizer)

	return nil
}

func initLedgerStateEngine(app *agentrt.App, typedLdg *ledger.Ledger, taskStore task.Store, taskRunner *task.Runner) {
	embedResRaw, _ := app.Get("embed_resolver")

	ledgerSync := iledger.NewLedgerSync(typedLdg, taskStore)
	taskRunner.OnTaskEvent(ledgerSync.OnEvent)
	app.Set("ledger_sync", ledgerSync)

	memBridge := iledger.NewMemoryBridge(typedLdg, taskStore)
	taskRunner.OnTaskEvent(memBridge.OnEvent)
	app.Set("ledger_memory_bridge", memBridge)

	if embedResRaw != nil {
		if embedRes, ok := embedResRaw.(*embeddings.Resolver); ok {
			if emb, ok := embedRes.Primary(); ok {
				typedLdg.Vector.SetEmbedFunc(func(ctx context.Context, text string) ([]float32, error) {
					return emb.Embed(ctx, text)
				})
				slog.Info("ledger vector index: embed function attached")
			}
		}
	}

	typedLdg.Recall.SetGraph(typedLdg.Graph)

	app.Lifecycle.RegisterFunc("ledger_lifecycle", func(ctx context.Context) error {
		go func() {
			ticker := time.NewTicker(6 * time.Hour)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					typedLdg.Lifecycle.RunDecay(ctx, "default")
					typedLdg.Lifecycle.RunGC(ctx, "default")
					typedLdg.Lifecycle.RunConsolidate(ctx, "default")
				}
			}
		}()
		return nil
	}, nil)

	slog.Info("ledger state engine initialized",
		"sync", true, "memory_bridge", true,
		"vector", typedLdg.Vector.Enabled(), "graph", true, "lifecycle", true)
}

func wireWorkflowExecutors(gw *gateway.Gateway, wfEngine *workflow.Engine, knowledgeStore *knowledge.Store, cfg *config.Config) {
	wfEngine.SetBrowserExecutor(func(ctx context.Context, action string, args map[string]any) (string, error) {
		hub := gw.BrowserHub()
		if hub == nil || !hub.Connected() {
			return "", fmt.Errorf("browser extension not connected; install and connect the Yunque Browser Connector extension")
		}
		browserAction := map[string]any{"type": "browser_" + action}
		if target, ok := args["target"].(string); ok {
			if action == "navigate" {
				browserAction["url"] = target
			} else {
				browserAction["target"] = map[string]any{"strategy": "bySelector", "selector": target}
			}
		}
		if text, ok := args["text"].(string); ok {
			browserAction["text"] = text
		}
		actionData, _ := json.Marshal(browserAction)
		resultData, err := hub.SendActionRaw(ctx, actionData)
		if err != nil {
			return "", err
		}
		return string(resultData), nil
	})

	sandboxRunner, sandboxErr := sandbox.NewRunner(sandbox.SandboxConfig{
		BaseDir: filepath.Join(cfg.DataDir, "sandbox"),
		Policy:  sandbox.DefaultPolicy(),
	})
	if sandboxErr != nil {
		slog.Warn("sandbox runner init failed, code nodes will be unavailable", "err", sandboxErr)
	} else {
		wfEngine.SetCodeExecutor(func(ctx context.Context, language, code string) (string, error) {
			res, err := sandboxRunner.Run(ctx, sandbox.RunRequest{
				Language: language,
				Code:     code,
				Timeout:  30 * time.Second,
			})
			if err != nil {
				return "", err
			}
			if res.ExitCode != 0 {
				return "", fmt.Errorf("exit %d: %s", res.ExitCode, res.Stderr)
			}
			return res.Stdout, nil
		})
		slog.Info("workflow code executor wired", "backend", sandboxRunner.Type())
	}

	wfEngine.SetKnowledgeExecutor(func(ctx context.Context, query string, topK int) (string, error) {
		scored := knowledgeStore.HybridSearchReranked(ctx, query, topK)
		if len(scored) == 0 {
			return "未找到匹配的知识条目", nil
		}
		var buf strings.Builder
		for i, sc := range scored {
			fmt.Fprintf(&buf, "[%d] (score %.2f) %s\n", i+1, sc.Score, sc.Chunk.Content)
		}
		return buf.String(), nil
	})
	slog.Info("workflow executors wired", "browser", "lazy", "knowledge", "ready")
}

func wireWorkflowSkills(
	app *agentrt.App,
	gw *gateway.Gateway,
	wfStore *workflow.JSONStore,
	wfEngine *workflow.Engine,
	triggerRT *trigger.Runtime,
	costAwareLLM func(ctx context.Context, system, user string) (string, error),
) {
	// Inject workflow store into GeneralPlugin
	for _, pl := range app.PluginReg.All() {
		if gp, ok := pl.(*general.GeneralPlugin); ok {
			gp.SetWorkflowStore(wfStore)
			break
		}
	}
	app.SkillRegistry = skills.NewRegistry()
	for _, s := range app.PluginReg.AllSkills() {
		app.SkillRegistry.Register(s)
	}
	slog.Info("generate_workflow skill: shared workflow store injected via plugin")

	var lastPlanCache sync.Map
	gw.SetLastPlanCache(&lastPlanCache)

	saveWFSkill := workflow.NewSaveWorkflowSkill(wfStore, func(tenantID string) *planner.PlanResult {
		if v, ok := lastPlanCache.Load(tenantID); ok {
			return v.(*planner.PlanResult)
		}
		return nil
	})
	saveWFSkill.SetTriggerBinder(func(wfID, triggerExpr, tenantID string) (string, error) {
		tType, tValue := trigger.ParseTriggerExpr(triggerExpr)
		tID := triggerRT.Register(trigger.Trigger{
			Name:   "auto:" + wfID,
			Kind:   tType,
			Event:  trigger.EventName(tValue),
			Action: trigger.Action{Type: trigger.ActionRunWorkflow, Data: map[string]any{"workflow_id": wfID}},
		})
		return tID, nil
	})
	app.SkillRegistry.Register(saveWFSkill)

	defaultTID := os.Getenv("DEFAULT_TENANT_ID")
	if defaultTID == "" {
		defaultTID = "default"
	}

	runWFSkill := workflow.NewRunWorkflowSkill(wfStore, func(ctx context.Context, instanceID string) error {
		go wfEngine.Run(context.Background(), instanceID)
		return nil
	})
	runWFSkill.SetTenantID(defaultTID)
	app.SkillRegistry.Register(runWFSkill)

	listWFSkill := workflow.NewListWorkflowsSkill(wfStore)
	listWFSkill.SetTenantID(defaultTID)
	app.SkillRegistry.Register(listWFSkill)
}

func wireTriggerExecutor(
	exec *trigger.Executor,
	taskStore task.Store,
	taskRunner *task.Runner,
	threadMgr *task.ThreadManager,
	channelReg *channel.Registry,
	app *agentrt.App,
	costAwareLLM func(ctx context.Context, system, user string) (string, error),
) {
	exec.SetCreateTask(func(ctx context.Context, tenantID, title, desc string) (string, error) {
		t, err := taskStore.Create(task.CreateRequest{Title: title, Description: desc, TenantID: tenantID})
		if err != nil {
			return "", err
		}
		go taskRunner.Run(context.Background(), t.ID)
		return t.ID, nil
	})
	exec.SetContinueTask(func(ctx context.Context, taskID, message string) error {
		if threadMgr != nil {
			threadMgr.Post(taskID, "", "trigger", message)
		}
		return taskRunner.Resume(ctx, taskID)
	})
	exec.SetSendMessage(func(ctx context.Context, channelID, threadID, message string) (string, error) {
		for _, ch := range channelReg.All() {
			if ch.Type() == channelID {
				target := threadID
				if target == "" {
					target = channelID
				}
				if err := ch.Send(ctx, target, channel.Reply{Content: message}); err != nil {
					return "", err
				}
				return "sent", nil
			}
		}
		return "", fmt.Errorf("channel not found: %s", channelID)
	})
	exec.SetCallSkill(func(ctx context.Context, skillName string, args map[string]any) (string, float64, error) {
		sk, ok := app.SkillRegistry.Get(skillName)
		if !ok {
			return "", 0, fmt.Errorf("skill not found: %s", skillName)
		}
		env := &skills.Environment{LLMCall: costAwareLLM}
		result, err := sk.Execute(ctx, args, env)
		return result, 0, err
	})
	exec.SetWriteMemory(func(ctx context.Context, tenantID, content string) error {
		return app.Orchestrator.Ingest(ctx, tenantID, content, "trigger", "trigger_action")
	})
	exec.SetUpdateProfile(func(ctx context.Context, tenantID, key, value string) error {
		if app.EditableMem != nil {
			app.EditableMem.AddBlock(key, value, 2000)
			return nil
		}
		return fmt.Errorf("editable memory not available")
	})
}
