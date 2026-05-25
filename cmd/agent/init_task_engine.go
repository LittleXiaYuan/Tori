package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"yunque-agent/internal/agentcore/adaptive"
	"yunque-agent/internal/agentcore/approval"
	"yunque-agent/internal/agentcore/audit"
	"yunque-agent/internal/agentcore/costtrack"
	"yunque-agent/internal/agentcore/cron"
	"yunque-agent/internal/agentcore/guardrails"
	"yunque-agent/internal/agentcore/knowledge"
	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/planner"
	"yunque-agent/internal/agentcore/rbac"
	agentrt "yunque-agent/internal/agentcore/runtime"
	"yunque-agent/internal/agentcore/selfheal"
	"yunque-agent/internal/agentcore/session"
	"yunque-agent/internal/agentcore/state"
	"yunque-agent/internal/agentcore/task"
	"yunque-agent/internal/agentcore/tasksched/rlsched"
	"yunque-agent/internal/agentcore/tools"
	"yunque-agent/internal/agentcore/trigger"
	"yunque-agent/internal/agentcore/workflow"
	"yunque-agent/internal/controlplane/gateway"
	"yunque-agent/internal/execution/channel"
	reflectpkg "yunque-agent/internal/experimental/reflect"
	iledger "yunque-agent/internal/ledger"
	"yunque-agent/internal/observe"
	"yunque-agent/pkg/skills"

	"yunque-agent/internal/ledgercore"
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

	taskEngineCtx, taskEngineCancel := context.WithCancel(context.Background())
	app.Lifecycle.RegisterFunc("task_engine_ctx", nil, func(_ context.Context) error {
		taskEngineCancel()
		return nil
	})
	channelReg := app.MustGet(agentrt.CompChannelReg).(*channel.Registry)
	learningLoop := app.MustGet("learning_loop").(*reflectpkg.LearningLoop)
	emotionShiftDetector := app.MustGet("emotion_shift_detector").(*planner.EmotionShiftDetector)

	var typedLdg *ledger.Ledger
	if ldgRaw, ok := app.Get(agentrt.CompLedger); ok {
		if typed, castOk := ldgRaw.(*ledger.Ledger); castOk {
			typedLdg = typed
		} else {
			slog.Warn("ledger component present but wrong type", "type", fmt.Sprintf("%T", ldgRaw))
		}
	}

	// ── Tools ──
	toolsMgr := tools.NewProcessManager()
	gw.SetToolsManager(toolsMgr)

	// ── Task Runtime ──
	baseTaskStore := iledger.NewLedgerStore(typedLdg, cfg.DataPath("tasks"))
	qlScheduler := ensureTaskQLearner(app)
	taskStore := rlsched.NewPolicyStore(baseTaskStore, qlScheduler)
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
	taskFeedback := rlsched.NewTaskFeedback(qlScheduler, taskStore)
	taskRunner.OnTaskEvent(taskFeedback.OnTaskEvent)
	slog.Info("Q-Learning task feedback wired", "actions", len([]string{"priority_high", "priority_normal", "priority_low", "defer"}))
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
	if ldgRaw, ok := app.Get(agentrt.CompLedger); ok {
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
	if ldgRaw, ok := app.Get(agentrt.CompLedger); ok {
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
	gw.SetLLMCall(workflow.LLMCallFunc(costAwareLLM))
	slog.Info("workflow engine initialized", "dir", cfg.DataPath("workflows"))

	wireWorkflowExecutors(gw, wfEngine, knowledgeStore, cfg)
	wireWorkflowSkills(app, gw, wfStore, wfEngine, triggerRT, costAwareLLM, taskEngineCtx)

	wfActionHandler = trigger.NewWorkflowActionHandler(
		func(ctx context.Context, defID, tenantID string, vars map[string]any) (string, error) {
			inst, err := wfStore.CreateInstance(defID, tenantID, vars)
			if err != nil {
				return "", err
			}
			go wfEngine.Run(taskEngineCtx, inst.ID)
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
	toolGuard := guardrails.NewToolGuard(guardrails.LoadToolGuardConfig("data/tool-guard.yaml"))
	egressGuard := guardrails.NewEgressGuard(guardrails.DefaultEgressGuardConfig())
	sanitizer := guardrails.NewSanitizer(guardrails.DefaultSanitizerConfig())
	if auditChain != nil {
		toolGuard.SetAudit(auditChain)
		egressGuard.SetAudit(auditChain)
		sanitizer.SetAudit(auditChain)
	}
	gw.SetToolGuard(toolGuard)
	gw.SetEgressGuard(egressGuard)
	gw.SetSanitizer(sanitizer)

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
	if ldgRaw, ok := app.Get(agentrt.CompLedger); ok {
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

	// ── Cognition Layer (templates, working memory, threads, reverie, reflection) ──
	threadMgr := initCognitionWiring(app, gw, cfg, convStore, taskStore, taskRunner, channelReg, learningLoop, typedLdg, costAwareLLM)

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

	wireTriggerExecutor(triggerExecutor, taskStore, taskRunner, threadMgr, channelReg, app, costAwareLLM, taskEngineCtx)

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
	if adaptiveRaw, ok := app.Get(agentrt.CompAdaptiveLoop); ok {
		if loop, castOk := adaptiveRaw.(*adaptive.Loop); castOk {
			gw.SetAdaptiveLoop(loop)
		} else {
			slog.Warn("adaptive loop component present but wrong type", "type", fmt.Sprintf("%T", adaptiveRaw))
		}
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

func ensureTaskQLearner(app *agentrt.App) *rlsched.QLearner {
	if raw, ok := app.Get("ql_scheduler"); ok {
		if ql, ok := raw.(*rlsched.QLearner); ok {
			return ql
		}
		slog.Warn("ql_scheduler component present but wrong type", "type", fmt.Sprintf("%T", raw))
	}
	qlActions := []string{"priority_high", "priority_normal", "priority_low", "defer"}
	ql := rlsched.NewQLearner(rlsched.DefaultQLearnerConfig(qlActions))
	app.Set("ql_scheduler", ql)
	return ql
}
