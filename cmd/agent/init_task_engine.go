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
	"yunque-agent/internal/cognicore/causal"
	"yunque-agent/internal/cognicore/curiosity"
	"yunque-agent/internal/cognicore/eval"
	"yunque-agent/internal/cognicore/microagent"
	"yunque-agent/internal/cognicore/recommend"
	"yunque-agent/internal/cognicore/trait"
	"yunque-agent/internal/cognicore/world"
	"yunque-agent/internal/controlplane/gateway"
	"yunque-agent/internal/execution/channel"
	reflectpkg "yunque-agent/internal/experimental/reflect"
	iledger "yunque-agent/internal/ledger"
	"yunque-agent/internal/observe"
	connectorspack "yunque-agent/internal/packs/connectors"
	controlplanepack "yunque-agent/internal/packs/controlplane"
	costpack "yunque-agent/internal/packs/cost"
	cronpack "yunque-agent/internal/packs/cron"
	documentspack "yunque-agent/internal/packs/documents"
	emotionpack "yunque-agent/internal/packs/emotion"
	experiencepack "yunque-agent/internal/packs/experience"
	filespack "yunque-agent/internal/packs/files"
	graphpack "yunque-agent/internal/packs/graph"
	idepack "yunque-agent/internal/packs/ide"
	innerlifepack "yunque-agent/internal/packs/innerlife"
	instructionspack "yunque-agent/internal/packs/instructions"
	knowledgepack "yunque-agent/internal/packs/knowledge"
	memorypack "yunque-agent/internal/packs/memory"
	microagentpack "yunque-agent/internal/packs/microagent"
	missionspack "yunque-agent/internal/packs/missions"
	modespack "yunque-agent/internal/packs/modes"
	nightschoolpack "yunque-agent/internal/packs/nightschool"
	notificationspack "yunque-agent/internal/packs/notifications"
	reveriepack "yunque-agent/internal/packs/reverie"
	skillspack "yunque-agent/internal/packs/skills"
	statepack "yunque-agent/internal/packs/state"
	triggerspack "yunque-agent/internal/packs/triggers"
	workpack "yunque-agent/internal/packs/work"
	worldmodelpack "yunque-agent/internal/packs/worldmodel"
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

	// Knowledge (RAG) pack — the read + import surface is served natively by the
	// pack via the knowledge store.
	_ = gw.RegisterModule(knowledgepack.NewHandlerWithStore(gw, gw.KnowledgeStore()))

	// Memory pack — native stats/search/add/compact via the shared NewWired path
	// (de-shelled from the gateway).
	_ = gw.RegisterModule(memorypack.NewWired(gw.MemoryManager(), gw.MemoryPipeline(), gw.MemoryOrchestrator, gw.TenantOf))

	// Skills pack — fully de-shelled: listing / scan / dynamic / approve / reject
	// all served natively via the registry + metrics + file scanner. SkillHub /
	// market keep their own gateway routes.
	skillsPack := skillspack.NewHandlerWithService(gw.SkillsRegistry(), gw.Metrics())
	// De-shelled approve/reject persist via the same path the gateway used.
	skillsPack.SetDynamicSave(func() error {
		return task.SaveDynamicSkills(gw.SkillsRegistry(), "data/dynamic_skills.json")
	})
	// De-shelled /v1/skills/scan rescans data/skills via the gateway file loader.
	skillsPack.SetScan(gw.ScanSkills)
	_ = gw.RegisterModule(skillsPack)

	// Work pack — owns the task (/v1/tasks/*) + project (/v1/projects/*)
	// surfaces natively. Workflows remain in the workflowapi sub-package.
	_ = gw.RegisterModule(workpack.NewHandler(gw))

	// Control-plane pack — owns the audit / trust / iterate / review / skillgrow /
	// usage surfaces natively. Shipped default-enabled (an always-on core
	// governance surface) so audit/trust stay available out of box.
	_ = gw.RegisterModule(controlplanepack.NewHandler(gw))
	// Cost pack — owns /v1/cost/* natively, turning cost governance into a
	// Pack-gated capability rather than a direct gateway sub-package.
	_ = gw.RegisterModule(costpack.NewProvider(func() *costtrack.Tracker { return costTracker }))
	// Connectors pack — owns /api/connectors* natively. The registry is wired
	// later by initSkillRegistration, so the pack resolves it lazily per request.
	_ = gw.RegisterModule(connectorspack.NewProvider(gw.ConnectorRegistry))
	// Notifications pack — owns /api/notify/* natively. The notifier is wired
	// by initTasks, so the pack resolves it lazily per request.
	_ = gw.RegisterModule(notificationspack.NewProvider(gw.Notifier))

	// Persona-modes pack — owns /v1/persona/mode* natively (de-shelled from the
	// gateway monolith). Resolves the mode manager lazily, so order is moot.
	_ = gw.RegisterModule(modespack.New(gw))
	// Reverie pack — owns /v1/reverie/* natively (de-shelled from the monolith).
	_ = gw.RegisterModule(reveriepack.New(gw))
	// IDE pack — owns /v1/ide/* natively (de-shelled from the monolith).
	_ = gw.RegisterModule(idepack.New(gw))
	// Cron pack — owns /v1/cron/* natively (split out of the triggers grab-bag).
	_ = gw.RegisterModule(cronpack.New(gw))
	// Triggers pack — owns /v1/triggers* natively (split out of the grab-bag).
	_ = gw.RegisterModule(triggerspack.New(gw))
	// Documents pack — owns /v1/documents* natively (split out of the grab-bag).
	_ = gw.RegisterModule(documentspack.New(gw))
	// Missions pack — owns /v1/missions* natively (split out of the grab-bag).
	_ = gw.RegisterModule(missionspack.New(gw))
	// Files pack — owns /api/files* (list/preview/download) natively, the last
	// non-admin slice split out of the triggers grab-bag. Reads the output dir
	// via the narrow OutputDir() host accessor.
	_ = gw.RegisterModule(filespack.New(gw))
	// Instructions pack — owns /v1/instructions* natively (de-shelled from the
	// chat-routes grab-bag). Reads the store via the narrow InstructionStore()
	// host accessor; tenant is resolved from request context.
	_ = gw.RegisterModule(instructionspack.New(gw))
	// Emotion pack — owns /v1/emotion/{stickers,history} natively (de-shelled
	// from the chat-routes grab-bag). Reads the sticker map + history via narrow
	// host accessors.
	_ = gw.RegisterModule(emotionpack.New(gw))
	// Graph pack — owns /v1/graph/{entities,relations,context,stats} natively
	// (de-shelled from the memory-routes grab-bag). Reads the knowledge graph via
	// the existing MemoryPipeline() host accessor.
	_ = gw.RegisterModule(graphpack.New(gw))

	// Inner Life pack — exposes the soul-layer outputs (curiosity / reflection /
	// dreaming) over a read-only HTTP surface. Registered here because the
	// curiosity module is built by initSoulLayer.
	if app.Ledger != nil {
		var curiosityMod *curiosity.Module
		if cm, ok := app.Get("curiosity"); ok {
			curiosityMod, _ = cm.(*curiosity.Module)
		}
		// Inner Life is migrated to the v2 Module lifecycle (Tier 0 microkernel):
		// RegisterModule runs Init(host) + Start, and wires enable/disable →
		// Start/Stop through the pack registry. Falls back gracefully if Init/Start
		// fail (routes still mounted, retryable by toggling the pack).
		_ = gw.RegisterModule(innerlifepack.New(innerlifepack.Config{
			Ledger:    app.Ledger,
			Curiosity: curiosityMod,
		}))
	}

	// Night School pack — exposes nightly learning: dreaming sessions, distilled
	// task experience and learned user traits. Registered after the soul layer
	// because the trait store / task distiller are built there.
	if app.Ledger != nil {
		var traitStore *trait.Store
		if ts, ok := app.Get("trait_store"); ok {
			traitStore, _ = ts.(*trait.Store)
		}
		// Night School migrated to the v2 Module lifecycle (Tier 0 microkernel).
		_ = gw.RegisterModule(nightschoolpack.New(nightschoolpack.Config{
			Ledger:     app.Ledger,
			TraitStore: traitStore,
		}))
	}

	// Experience pack — exposes the recommendation engine and task self-evals.
	// Recommend engine is wired in initIntelligence; evaluator in initSoulLayer.
	if app.Ledger != nil {
		var recEngine *recommend.Engine
		if re, ok := app.Get("recommend_engine"); ok {
			recEngine, _ = re.(*recommend.Engine)
		}
		var evaluator *eval.Evaluator
		if ev, ok := app.Get("evaluator"); ok {
			evaluator, _ = ev.(*eval.Evaluator)
		}
		// Experience migrated to the v2 Module lifecycle (Tier 0 microkernel).
		_ = gw.RegisterModule(experiencepack.New(experiencepack.Config{
			Ledger:    app.Ledger,
			Recommend: recEngine,
			Evaluator: evaluator,
		}))
	}

	// World Model pack — exposes the agent's understanding of external state
	// (world model) plus causal-engine views on failed tasks.
	if app.Ledger != nil {
		var worldModel *world.Model
		if wm, ok := app.Get("world_model"); ok {
			worldModel, _ = wm.(*world.Model)
		}
		var causalEngine *causal.CausalEngine
		if ce, ok := app.Get("causal_engine"); ok {
			causalEngine, _ = ce.(*causal.CausalEngine)
		}
		// World Model migrated to the v2 Module lifecycle (Tier 0 microkernel).
		_ = gw.RegisterModule(worldmodelpack.New(worldmodelpack.Config{
			WorldModel: worldModel,
			Causal:     causalEngine,
		}))
	}

	// Micro-Agent pack — exposes the microagent registry and ReAct reasoning
	// trace replay. Microagent registry is wired in initIntelligence; ReAct
	// reasoning is recorded into the ledger by the react runner from initSoul.
	{
		var maRegistry *microagent.Registry
		if mr, ok := app.Get("microagent_registry"); ok {
			maRegistry, _ = mr.(*microagent.Registry)
		}
		// Micro-Agent migrated to the v2 Module lifecycle (Tier 0 microkernel).
		_ = gw.RegisterModule(microagentpack.New(microagentpack.Config{
			Registry: maRegistry,
			Ledger:   app.Ledger,
		}))
	}

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
	// State pack — owns /v1/state* natively via the state-kernel accessor.
	_ = gw.RegisterModule(statepack.New(gw))

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

		gw.SetOnboardingKVStore(iledger.NewKVConfigStore(app.Ledger, "onboarding"))
		slog.Info("onboarding: using Ledger KV for persistence")

		// Backs the pack-scoped ledger_get/ledger_set WASM host functions
		// (permission-gated; keys are namespaced per pack + tenant).
		gw.SetWasmPackKVStore(iledger.NewKVConfigStore(app.Ledger, "pack_kv"))
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
