package gateway

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"yunque-agent/internal/agentcore/adaptive"
	"yunque-agent/internal/agentcore/approval"
	"yunque-agent/internal/agentcore/audit"
	"yunque-agent/internal/agentcore/bots"
	"yunque-agent/internal/agentcore/costtrack"
	"yunque-agent/internal/agentcore/cron"
	"yunque-agent/internal/agentcore/embeddings"
	"yunque-agent/internal/agentcore/emotion"
	"yunque-agent/internal/agentcore/federation"
	"yunque-agent/internal/agentcore/guardrails"
	"yunque-agent/internal/agentcore/identity"
	"yunque-agent/internal/agentcore/inbox"
	"yunque-agent/internal/agentcore/instruction"
	"yunque-agent/internal/agentcore/knowledge"
	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/localbrain"
	"yunque-agent/internal/agentcore/memory"
	"yunque-agent/internal/agentcore/notify"
	"yunque-agent/internal/agentcore/persona"
	"yunque-agent/internal/agentcore/planner"
	"yunque-agent/internal/agentcore/rbac"
	"yunque-agent/internal/agentcore/review"
	"yunque-agent/internal/agentcore/router"
	agentrt "yunque-agent/internal/agentcore/runtime"
	"yunque-agent/internal/agentcore/selfheal"
	"yunque-agent/internal/agentcore/session"
	"yunque-agent/internal/agentcore/skillmarket"
	"yunque-agent/internal/agentcore/speech"
	"yunque-agent/internal/agentcore/state"
	"yunque-agent/internal/agentcore/subagent"
	"yunque-agent/internal/agentcore/task"
	"yunque-agent/internal/agentcore/tools"
	"yunque-agent/internal/agentcore/trigger"
	"yunque-agent/internal/agentcore/trust"
	"yunque-agent/internal/agentcore/websearch"
	"yunque-agent/internal/agentcore/workflow"
	"yunque-agent/internal/connectors"
	"yunque-agent/internal/execution/channel"
	"yunque-agent/internal/execution/scheduler"
	"yunque-agent/internal/experimental/distill"
	"yunque-agent/internal/experimental/heartbeat"
	"yunque-agent/internal/experimental/iterate"
	reflectpkg "yunque-agent/internal/experimental/reflect"
	"yunque-agent/internal/experimental/skillgrow"
	"yunque-agent/internal/integrations/mineru"
	"yunque-agent/internal/observe"
	"yunque-agent/internal/orchestrator"
	"yunque-agent/internal/tori"
	"yunque-agent/pkg/cogni"
	"yunque-agent/pkg/packruntime"
	"yunque-agent/pkg/plugin"
	"yunque-agent/pkg/safego"
)

// --- Component setters ---
// Each setter attaches an optional subsystem to the Gateway.
// They are called during init_tasks wiring phase and are kept in
// a dedicated file to reduce noise in gateway.go.

// SetPackRegistry attaches the Pack Runtime registry used by /v1/packs and frontend sync.
func (g *Gateway) SetPackRegistry(r *packruntime.Registry) { g.packRegistry = r }

// RegisterBackendPack mounts a backend capability pack module through the
// Pack Runtime route gate. It can be called after Gateway construction, which
// lets optional packages be installed or wired without adding fixed Gateway
// business routes.
func (g *Gateway) RegisterBackendPack(module packruntime.BackendModule) {
	if module == nil {
		return
	}
	g.backendPacks = append(g.backendPacks, module)
	g.registerBackendPack(module)
}

// SetHeartbeat attaches a heartbeat service.
func (g *Gateway) SetHeartbeat(hb *heartbeat.Service) { g.heartbeat = hb }

// SetInbox attaches an inbox store.
func (g *Gateway) SetInbox(ib *inbox.Store) { g.inbox = ib }

// SetBotManager attaches a multi-bot manager.
func (g *Gateway) SetBotManager(bm *bots.Manager) { g.botMgr = bm }

// SetSearchRegistry attaches a web search registry.
func (g *Gateway) SetSearchRegistry(sr *websearch.Registry) { g.searchReg = sr }

// SetSmartRouter attaches a smart model router.
func (g *Gateway) SetSmartRouter(r *router.Router) { g.smartRouter = r }

// SetIdentityResolver attaches a cross-channel identity resolver.
func (g *Gateway) SetIdentityResolver(ir *identity.Resolver) { g.identityRes = ir }

// SetHealer attaches a self-healing plugin generator.
func (g *Gateway) SetHealer(h *selfheal.Healer) { g.healer = h }

// SetLifecycle attaches a skill lifecycle manager.
func (g *Gateway) SetLifecycle(lc *selfheal.Lifecycle) { g.lifecycle = lc }

// SetPersonaChain attaches a persona priority chain for session/conversation overrides.
func (g *Gateway) SetPersonaChain(pc *persona.PriorityChain) { g.personaChain = pc }

// SetCostTracker attaches a cost tracking module.
func (g *Gateway) SetCostTracker(ct *costtrack.Tracker) { g.costTracker = ct }

// SetForkTree attaches a conversation fork tree.
func (g *Gateway) SetForkTree(ft *session.ForkTree) { g.forkTree = ft }

// SetForkPersister attaches a fork tree persister for saving state to disk.
func (g *Gateway) SetForkPersister(fp *session.ForkPersister) { g.forkPersister = fp }

// SetEmbeddings attaches an embeddings resolver.
func (g *Gateway) SetEmbeddings(er *embeddings.Resolver) { g.embedResolver = er }

// SetSubagentManager attaches a subagent manager.
func (g *Gateway) SetSubagentManager(sm *subagent.Manager) { g.subagentMgr = sm }

// SetHandoffRegistry attaches a handoff registry for subagent delegation.
func (g *Gateway) SetHandoffRegistry(hr *subagent.HandoffRegistry) { g.handoffReg = hr }

// SetOrchestrator attaches the five-layer memory orchestrator.
func (g *Gateway) SetOrchestrator(o *memory.Orchestrator) { g.orchestrator = o }

// SetOrchDaemon attaches the orchestration daemon for IDE work dispatch.
func (g *Gateway) SetOrchDaemon(d *orchestrator.Daemon) { g.orchDaemon = d }

// SetOrchLauncher attaches the worker launcher for adapter management.
func (g *Gateway) SetOrchLauncher(l *orchestrator.Launcher) { g.orchLauncher = l }

// SetZhGuard attaches the Chinese guardrail pipeline.
func (g *Gateway) SetZhGuard(p *guardrails.Pipeline) { g.zhGuard = p }

// SetAdaptiveLoop attaches the adaptive behavior loop.
func (g *Gateway) SetAdaptiveLoop(al *adaptive.Loop) { g.adaptiveLoop = al }

// SetAuditChain attaches the Merkle audit chain.
func (g *Gateway) SetAuditChain(ac *audit.Chain) { g.auditChain = ac }

// SetSkillMarket attaches the skill marketplace.
func (g *Gateway) SetSkillMarket(sm *skillmarket.Market) { g.skillMarket = sm }

// SetFederationHub attaches the federation hub.
func (g *Gateway) SetFederationHub(h *federation.Hub) { g.fedHub = h }

// SetFederationBridge attaches the OPP v3 bridge for model-aware federation.
func (g *Gateway) SetFederationBridge(b *federation.OPPBridge) { g.fedBridge = b }

// SetFederationTransport attaches the federation HTTP transport.
func (g *Gateway) SetFederationTransport(t *federation.Transport) { g.fedTransport = t }

// SetKnowledgeStore attaches the knowledge base.
func (g *Gateway) SetKnowledgeStore(ks *knowledge.Store) { g.knowledgeStore = ks }

// SetWikiStore attaches the LLM Wiki structured knowledge store.
func (g *Gateway) SetWikiStore(ws *knowledge.WikiStore) { g.wikiStore = ws }

// SetKnowledgeDir sets the directory for persisting extracted conversation facts.
func (g *Gateway) SetKnowledgeDir(dir string) { g.knowledgeDir = dir }

// SetCronManager attaches the cron job manager.
func (g *Gateway) SetCronManager(cm *cron.Manager) { g.cronMgr = cm }

// SetToolsManager attaches the process/tools manager.
func (g *Gateway) SetToolsManager(tm *tools.ProcessManager) { g.toolsMgr = tm }

// SetShellPolicy attaches the shell execution policy (approval + guard).
func (g *Gateway) SetShellPolicy(sp *tools.ShellExecPolicy) { g.shellPolicy = sp }

// SetPluginLoader attaches a plugin directory loader for CRUD operations.
func (g *Gateway) SetPluginLoader(l *plugin.Loader) { g.pluginLoader = l }

// SetSkillFileLoader attaches a file-based skill loader for data/skills/ auto-scanning.
func (g *Gateway) SetSkillFileLoader(l *skillmarket.SkillFileLoader) { g.skillFileLoader = l }

// SetRuntimePool attaches the multi-agent runtime pool.
func (g *Gateway) SetRuntimePool(p *agentrt.Pool) { g.runtimePool = p }

// SetBindingRouter attaches the binding-based agent router.
func (g *Gateway) SetBindingRouter(r *agentrt.Router) { g.bindingRouter = r }

// SetToolGuard attaches the tool call parameter guard.
func (g *Gateway) SetToolGuard(tg *guardrails.ToolGuard) { g.toolGuard = tg }

// SetEgressGuard attaches the output sanitization guard.
func (g *Gateway) SetEgressGuard(eg *guardrails.EgressGuard) { g.egressGuard = eg }

// SetSanitizer attaches the unified input sanitizer middleware.
func (g *Gateway) SetSanitizer(s *guardrails.Sanitizer) { g.sanitizer = s }

// SetSkillInstaller attaches the skill installer for SkillHub API.
func (g *Gateway) SetSkillInstaller(si *skillmarket.Installer) { g.skillInstaller = si }

// SetSkillPolicy attaches the security policy for SkillHub enforcement.
func (g *Gateway) SetSkillPolicy(sp *skillmarket.SecurityPolicy) { g.skillPolicy = sp }

// SetClawHubProvider attaches the ClawHub remote skill provider.
func (g *Gateway) SetClawHubProvider(ch *skillmarket.ClawHubProvider) { g.clawHub = ch }

// SetToriHubProvider attaches the ToriHub remote skill provider.
func (g *Gateway) SetToriHubProvider(th *skillmarket.ToriHubProvider) { g.toriHub = th }

// SetIterateEngine attaches the self-iteration engine.
func (g *Gateway) SetIterateEngine(ie *iterate.Engine) { g.iterateEngine = ie }

// SetTrustTracker attaches the progressive trust tracker.
func (g *Gateway) SetTrustTracker(tt *trust.Tracker) { g.trustTracker = tt }

// SetDistiller attaches the knowledge distiller.
func (g *Gateway) SetDistiller(d *distill.Distiller) { g.distiller = d }

// SetReviewGate attaches the risk-graded review gate.
func (g *Gateway) SetReviewGate(rg *review.Gate) { g.reviewGate = rg }

// SetSkillGrow attaches the skill growth detector and wires its proposal
// callback to auto-save skills into RAG and surface notifications in the frontend.
func (g *Gateway) SetSkillGrow(sg *skillgrow.Detector) {
	g.skillGrow = sg
	if sg != nil {
		sg.SetOnProposal(func(_ context.Context, pattern, suggestion string) {
			slog.Info("skillgrow: auto-saving skill", "pattern", pattern)

			g.storePendingSuggestions("default", []memory.SkillSuggestion{{
				Name:        "自动化: " + truncateStr(pattern, 30),
				Description: suggestion,
				Trigger:     pattern,
				Confidence:  9,
			}})

			// Write structured knowledge entry to RAG
			if g.knowledgeStore != nil {
				safego.Go("skillgrow-rag-write", func() {
					name := "自动化: " + truncateStr(pattern, 30)
					trigger := "当用户进行类似操作时"
					_, err := g.knowledgeStore.IngestStructured(name, trigger, suggestion)
					if err != nil {
						slog.Warn("skillgrow: RAG write failed", "err", err)
						return
					}
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					_ = g.knowledgeStore.BuildIndex(ctx)
					slog.Info("skillgrow: RAG write completed", "pattern", pattern)
				})
			}

			// Also write to orchestrator memory for short-term recall
			if g.orchestrator != nil {
				safego.Go("skillgrow-mem-write", func() {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					fact := "用户频繁使用的操作模式：" + pattern + "。" + suggestion
					_ = g.orchestrator.Ingest(ctx, "default", fact, "skill_pattern", "skillgrow")
				})
			}
		})
	}
}

// SetAuditTrail attaches the task audit trail.
func (g *Gateway) SetAuditTrail(at *audit.Trail) { g.auditTrail = at }

// SetProviderRegistry attaches the LLM provider registry.
func (g *Gateway) SetProviderRegistry(pr *llm.ProviderRegistry) { g.providerReg = pr }

// SetSpeechRegistry attaches the TTS/STT speech registry.
func (g *Gateway) SetSpeechRegistry(sr *speech.Registry) { g.speechReg = sr }

// SetEmotionAnalyzer attaches the emotion analyzer for text/audio emotion detection.
func (g *Gateway) SetEmotionAnalyzer(ea *emotion.Analyzer) { g.emotionAnalyzer = ea }

// SetEmotionHistory attaches the emotion history store.
func (g *Gateway) SetEmotionHistory(h *emotion.History) { g.emotionHistory = h }

// SetInstructionStore attaches the user instruction store.
func (g *Gateway) SetInstructionStore(s *instruction.Store) { g.instructionStore = s }

// SetStickerMap attaches a sticker suggestion map.
func (g *Gateway) SetStickerMap(sm *emotion.StickerMap) { g.stickerMap = sm }

// SetStickerCollector attaches a sticker collector for interactive sticker learning.
func (g *Gateway) SetStickerCollector(sc *emotion.StickerCollector) { g.stickerCollector = sc }

// SetEmotionShiftDetector attaches the emotion shift detector for Reverie event-driven triggers.
func (g *Gateway) SetEmotionShiftDetector(d *planner.EmotionShiftDetector) { g.emotionShift = d }

// SetFactEventHook attaches the fact event hook for Reverie event-driven triggers.
func (g *Gateway) SetFactEventHook(h *planner.FactEventHook) { g.factHook = h }

// SetSkillSuggester attaches the skill suggestion analyzer.
func (g *Gateway) SetSkillSuggester(s *memory.SkillSuggester) { g.skillSuggester = s }

// SetReverie attaches the Reverie system for API access.
func (g *Gateway) SetReverie(r *planner.Reverie) { g.reverie = r }

// SetTaskStore attaches the task persistence store. The MCP dispatch context
// is also updated so get_pending_tasks / claim_task / report_progress /
// submit_result / get_task_context see the same store without needing the
// dispatch server to be rebuilt (registerMCPDispatchRoutes runs during
// gateway.New, before this setter is invoked in cmd/agent/init_tasks.go).
func (g *Gateway) SetTaskStore(s task.Store) {
	g.taskStore = s
	if g.mcpDispatchCtx != nil {
		g.mcpDispatchCtx.TaskStore = s
	}
}

// SetTaskRunner attaches the task execution engine.
func (g *Gateway) SetTaskRunner(r *task.Runner) { g.taskRunner = r }

// SetGapAnalyzer attaches the capability gap analyzer.
func (g *Gateway) SetGapAnalyzer(a *task.GapAnalyzer) { g.gapAnalyzer = a }

// SetStateKernel attaches the structured state kernel.
func (g *Gateway) SetStateKernel(sk *state.Kernel) { g.stateKernel = sk }

// SetExperienceStore attaches the reflection experience store.
func (g *Gateway) SetExperienceStore(es *reflectpkg.ExperienceStore) { g.experienceStore = es }

// SetTemplateStore attaches the task template store.
func (g *Gateway) SetTemplateStore(ts *task.TemplateStore) { g.templateStore = ts }

// SetWorkingMemoryManager attaches the task working memory manager.
func (g *Gateway) SetWorkingMemoryManager(wm *task.WorkingMemoryManager) { g.workMemMgr = wm }

// SetThreadManager attaches the task thread manager.
func (g *Gateway) SetThreadManager(tm *task.ThreadManager) { g.threadMgr = tm }

// SetTriggerRuntime attaches the trigger runtime.
func (g *Gateway) SetTriggerRuntime(rt *trigger.Runtime) { g.triggerRT = rt }

// SetTriggerManager attaches the unified trigger manager.
func (g *Gateway) SetTriggerManager(m *trigger.Manager) { g.triggerMgr = m }

// SetChannelRegistry attaches the channel registry for react/sticker operations.
func (g *Gateway) SetChannelRegistry(cr *channel.Registry) { g.channelReg = cr }

// SetPreAckEmojis configures the emoji list for pre-ack reactions on incoming messages.
func (g *Gateway) SetPreAckEmojis(emojis []string) { g.preAckEmojis = emojis }

// SetWorkflowStore attaches the workflow definition/instance store.
func (g *Gateway) SetWorkflowStore(s workflow.Store) { g.workflowStore = s }

// SetWorkflowEngine attaches the workflow execution engine.
func (g *Gateway) SetWorkflowEngine(e *workflow.Engine) { g.workflowEngine = e }

// SetRBACEnforcer attaches the RBAC permission enforcer.
func (g *Gateway) SetRBACEnforcer(e *rbac.Enforcer) { g.rbacEnforcer = e }

// SetRBACMiddleware attaches the RBAC HTTP middleware.
func (g *Gateway) SetRBACMiddleware(m *rbac.Middleware) { g.rbacMiddleware = m }

// SetApprovalManager attaches the human-in-the-loop approval manager.
func (g *Gateway) SetApprovalManager(m *approval.Manager) { g.approvalMgr = m }

// SetQueueManager attaches the session task queue manager.
func (g *Gateway) SetQueueManager(qm *session.QueueManager) { g.queueMgr = qm }

// SetSSEBroker attaches the SSE event stream broker.
func (g *Gateway) SetSSEBroker(b *SSEBroker) { g.sseBroker = b }

// SetEventTrail attaches the unified event audit trail.
func (g *Gateway) SetEventTrail(t *observe.AuditTrail) { g.eventTrail = t }

// SetLastPlanCache sets the plan result cache for save_as_workflow.
func (g *Gateway) SetLastPlanCache(c *sync.Map) { g.lastPlanCache = c }

// SetScheduler replaces the scheduler reference (used in init wiring).
func (g *Gateway) SetScheduler(s *scheduler.Scheduler) { g.scheduler = s }

// SetUpdateChecker sets the callback for checking available updates.
func (g *Gateway) SetUpdateChecker(fn func() (tagName, htmlURL string, hasNew bool)) {
	g.updateChecker = fn
}

// SetToriTokenStore sets the Tori OAuth2 token store for bind/unbind endpoints.
func (g *Gateway) SetToriTokenStore(ts *tori.TokenStore) { g.toriTokenStore = ts }

// SetConnectorRegistry sets the connector registry.
func (g *Gateway) SetConnectorRegistry(reg *connectors.Registry) { g.connectorReg = reg }

// ConnectorRegistry returns the connector registry.
func (g *Gateway) ConnectorRegistry() *connectors.Registry { return g.connectorReg }

// SetMinerUClient sets the MinerU document parser backend.
func (g *Gateway) SetMinerUClient(client *mineru.Client) { g.documentParser = client }

// SetNotifier sets the notification dispatcher.
func (g *Gateway) SetNotifier(n *notify.Notifier) { g.notifier = n }

// Notifier returns the notification dispatcher.
func (g *Gateway) Notifier() *notify.Notifier { return g.notifier }

// SetExecProvider sets the LLM provider used by all exec agents.
func (g *Gateway) SetExecProvider(id string) {
	g.execProviderMu.Lock()
	defer g.execProviderMu.Unlock()
	g.execProvider = id
}

// ExecProvider returns the current exec provider (empty means "smart" default).
func (g *Gateway) ExecProvider() string {
	g.execProviderMu.RLock()
	defer g.execProviderMu.RUnlock()
	return g.execProvider
}

// ProviderClient returns an enabled provider client by id.
func (g *Gateway) ProviderClient(id string) (*llm.Client, bool) {
	if g.providerReg == nil || id == "" || id == "smart" {
		return nil, false
	}
	p := g.providerReg.Get(id)
	if p == nil {
		for _, status := range g.providerReg.List() {
			if status.ID+"-"+status.Model == id {
				p = g.providerReg.Get(status.ID)
				break
			}
		}
	}
	if p == nil || !p.Config.Enabled || p.Client == nil {
		return nil, false
	}
	return p.Client, true
}

// SetModelKVStore injects a Ledger KV store into the model manager for persistence.
func (g *Gateway) SetModelKVStore(kvs modelKVStore) {
	if g.modelMgr != nil {
		g.modelMgr.SetKVStore(kvs)
	}
}

// SetUsageKVStore injects a Ledger KV store into the usage tracker for persistence.
func (g *Gateway) SetUsageKVStore(kvs usageKVStore) {
	if g.usage != nil {
		g.usage.SetKVStore(kvs)
	}
}

// SetLedgerHealthChecker attaches the persistent state health check used by probes.
func (g *Gateway) SetLedgerHealthChecker(h healthChecker) { g.ledgerHealth = h }

// FlushUsageKV persists usage data before shutdown.
func (g *Gateway) FlushUsageKV() {
	if g.usage != nil {
		g.usage.FlushToKV()
	}
}

// SetModuleRegistry attaches the hot-pluggable module registry.
func (g *Gateway) SetModuleRegistry(r *agentrt.ModuleRegistry, profile string) {
	g.modules = r
	g.profile = profile
}

// SetCogniRegistry attaches the hot-pluggable Cogni registry plus the directory
// it watches for declarative *.json files. Called by cmd/agent/module_cogni.go
// after the runtime module starts.
func (g *Gateway) SetCogniRegistry(r *cogni.Registry, dir string) {
	g.cogniRegistry = r
	g.cogniDir = dir
}

// SetCogniKernelRuntimeStateHandler attaches a read-only Pack Runtime state
// reporter owned by the Cogni Kernel module. It is intentionally separate from
// the broad /v1/cognis/ bridge route so Pack Runtime can keep a method-aware
// gate for /v1/cognis/runtime/pack-state while the legacy sub-resource bridge
// continues to cover declaration operations.
func (g *Gateway) SetCogniKernelRuntimeStateHandler(handler http.HandlerFunc) {
	g.cogniKernelRuntimeState = handler
}

// SetCogniTraceStore attaches the trace store for /v1/cognis/traces and
// /v1/cognis/{id}/trace endpoints.
func (g *Gateway) SetCogniTraceStore(s cogni.TraceStore) { g.cogniTraces = s }

// SetCogniSentinel attaches the sentinel (alert + auto-disable engine).
func (g *Gateway) SetCogniSentinel(s *cogni.Sentinel) { g.cogniSentinel = s }

// SetCogniWorkflowEngine attaches the workflow engine for /v1/cognis/{id}/workflows/*.
func (g *Gateway) SetCogniWorkflowEngine(we *cogni.WorkflowEngine) { g.cogniWorkflowEngine = we }

// SetCogniExperiences attaches the per-cogni experience stores.
func (g *Gateway) SetCogniExperiences(m map[string]*cogni.ExperienceStore) { g.cogniExperiences = m }

// SetCogniGenesis attaches the self-genesis engine for /v1/cognis/generate.
func (g *Gateway) SetCogniGenesis(gen *cogni.Genesis) { g.cogniGenesis = gen }

// SetCogniEvolution attaches the evolution engine for /v1/cognis/{id}/evolve.
func (g *Gateway) SetCogniEvolution(ee *cogni.EvolutionEngine) { g.cogniEvolution = ee }

// SetCogniFederation attaches the federation manager.
func (g *Gateway) SetCogniFederation(cf *cogni.CogniFederation) { g.cogniFederation = cf }

// SetCogniCostTracker attaches the economics cost tracker.
func (g *Gateway) SetCogniCostTracker(ct *cogni.CostTracker) { g.cogniCostTracker = ct }

// SetNLConfigTranslator attaches the natural-language config translator.
func (g *Gateway) SetNLConfigTranslator(t *cogni.NLConfigTranslator) { g.nlConfigTranslator = t }

// SetLoRAScheduler attaches the LoRA training lifecycle scheduler.
func (g *Gateway) SetLoRAScheduler(s *localbrain.LoRAScheduler) { g.loraScheduler = s }

// SetTrainingMetrics attaches training history / aggregate metrics storage.
func (g *Gateway) SetTrainingMetrics(m *localbrain.TrainingMetrics) { g.trainingMetrics = m }

// SetEvolutionCoordinator attaches the multi-layer evolution coordinator.
func (g *Gateway) SetEvolutionCoordinator(ec *localbrain.EvolutionCoordinator) {
	g.evolutionCoordinator = ec
}

func truncateStr(s string, maxRunes int) string {
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes]) + "..."
}
