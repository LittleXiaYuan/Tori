package gateway

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
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
	"yunque-agent/internal/agentcore/llm/distill"
	"yunque-agent/internal/agentcore/localbrain"
	"yunque-agent/internal/agentcore/memory"
	"yunque-agent/internal/agentcore/notify"
	"yunque-agent/internal/agentcore/persona"
	"yunque-agent/internal/agentcore/planner"
	"yunque-agent/internal/agentcore/rbac"
	"yunque-agent/internal/agentcore/review"
	"yunque-agent/internal/agentcore/router"
	agentrt "yunque-agent/internal/agentcore/runtime"
	"yunque-agent/internal/agentcore/runtime/heartbeat"
	"yunque-agent/internal/agentcore/selfheal"
	"yunque-agent/internal/agentcore/selfheal/iterate"
	"yunque-agent/internal/agentcore/session"
	"yunque-agent/internal/agentcore/skillgrowth"
	"yunque-agent/internal/agentcore/skillgrowth/adapter"
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
	"yunque-agent/internal/cognikernel"
	"yunque-agent/internal/connectors"
	"yunque-agent/internal/controlplane/models"
	"yunque-agent/internal/execution/channel"
	"yunque-agent/internal/execution/scheduler"
	reflectpkg "yunque-agent/internal/experimental/reflect"
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

// SetBaseContext sets the long-lived context used to launch background daemons
// (e.g. the orchestrator daemon) from request handlers. Passing a nil context
// falls back to context.Background(). Without this, handlers that need to start
// a daemon would have to use the request context, which is cancelled the moment
// the request returns — killing the daemon immediately.
func (g *Gateway) SetBaseContext(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	g.baseCtx = ctx
}

// BaseContext returns the long-lived context for background daemons, never nil.
func (g *Gateway) BaseContext() context.Context {
	if g.baseCtx != nil {
		return g.baseCtx
	}
	return context.Background()
}

// SetPackRegistry attaches the Pack Runtime registry used by /v1/packs and frontend sync.
func (g *Gateway) SetPackRegistry(r *packruntime.Registry) {
	g.packRegistry = r
	g.wireWasmPacks()
}

// SetPackTrustRoot installs the resolver used to verify signed .yqpack
// manifests at install time. Without it, signed packs fail closed.
func (g *Gateway) SetPackTrustRoot(tr packruntime.PublicKeyResolver) {
	g.packTrustRoot = tr
}

// PackRegistry exposes the Pack Runtime registry to host-side wiring (e.g. the
// wasm-plugin pack's remote-install executor, which installs verified cached
// .yqpacks via Registry.InstallFromYqpack). May be nil before init.
func (g *Gateway) PackRegistry() *packruntime.Registry { return g.packRegistry }

// PackTrustRoot exposes the signed-pack public-key resolver used to verify
// .yqpack signatures at install time. May be nil (signed packs fail closed).
func (g *Gateway) PackTrustRoot() packruntime.PublicKeyResolver { return g.packTrustRoot }

// SetPackCatalogSources attaches local pack manifest directories used by the
// read-only Pack Runtime catalog. Sources can point at directories containing
// pack.json files directly or nested pack folders.
func (g *Gateway) SetPackCatalogSources(sources []string) {
	g.packCatalogSources = g.packCatalogSources[:0]
	for _, source := range sources {
		source = strings.TrimSpace(source)
		if source != "" {
			g.packCatalogSources = append(g.packCatalogSources, source)
		}
	}
}

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

// RequireAuth exposes the host's normal tenant/JWT/API-key auth wrapper to
// packs that need custom auth composition (for example method-sensitive MCP
// dispatch where GET is a probe but POST must be authenticated).
func (g *Gateway) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return g.requireAuth(next)
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

// SetSkillGrowthPipeline attaches the canonical detect→generate→review→promote
// skill-growth pipeline. Detectors should feed capability gaps here instead of
// directly generating/installing skills.
func (g *Gateway) SetSkillGrowthPipeline(p skillgrowth.GapHandler) {
	g.skillGrowthPipe = p
}

// SetPersonaChain attaches a persona priority chain for session/conversation overrides.
func (g *Gateway) SetPersonaChain(pc *persona.PriorityChain) { g.personaChain = pc }

// SetCostTracker attaches a cost tracking module. /v1/cost/* is mounted by the
// cost pack once the task engine registers it with the wired tracker.
func (g *Gateway) SetCostTracker(ct *costtrack.Tracker) {
	g.costTracker = ct
}

// SetForkTree attaches a conversation fork tree. /v1/fork* is mounted by the
// forks pack and resolves the current tree lazily.
func (g *Gateway) SetForkTree(ft *session.ForkTree) {
	g.forkTree = ft
}

// SetForkPersister attaches a fork tree persister for saving state to disk.
func (g *Gateway) SetForkPersister(fp *session.ForkPersister) {
	g.forkPersister = fp
}

// ForkTree returns the conversation fork tree.
func (g *Gateway) ForkTree() *session.ForkTree { return g.forkTree }

// ForkPersister returns the conversation fork persister.
func (g *Gateway) ForkPersister() *session.ForkPersister { return g.forkPersister }

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

// OrchDaemon exposes the orchestration daemon to the orchestrator pack.
func (g *Gateway) OrchDaemon() *orchestrator.Daemon { return g.orchDaemon }

// OrchLauncher exposes the worker launcher to the orchestrator pack.
func (g *Gateway) OrchLauncher() *orchestrator.Launcher { return g.orchLauncher }

// SetZhGuard attaches the Chinese guardrail pipeline.
func (g *Gateway) SetZhGuard(p *guardrails.Pipeline) { g.zhGuard = p }

// SetAdaptiveLoop attaches the adaptive behavior loop.
func (g *Gateway) SetAdaptiveLoop(al *adaptive.Loop) { g.adaptiveLoop = al }

// SetAuditChain attaches the Merkle audit chain.
func (g *Gateway) SetAuditChain(ac *audit.Chain) { g.auditChain = ac }

// SetSkillMarket attaches the skill marketplace.
func (g *Gateway) SetSkillMarket(sm *skillmarket.Market) { g.skillMarket = sm }

// SkillMarket exposes the local skill marketplace to the market pack.
func (g *Gateway) SkillMarket() *skillmarket.Market { return g.skillMarket }

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

// CronManager exposes the cron manager to the cron pack (internal/packs/cron),
// which owns /v1/cron/* natively. May be nil if not configured.
func (g *Gateway) CronManager() *cron.Manager { return g.cronMgr }

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

// SkillInstaller exposes the SkillHub installer to the SkillHub pack.
func (g *Gateway) SkillInstaller() *skillmarket.Installer { return g.skillInstaller }

// SetSkillPolicy attaches the security policy for SkillHub enforcement.
func (g *Gateway) SetSkillPolicy(sp *skillmarket.SecurityPolicy) { g.skillPolicy = sp }

// SkillPolicy exposes the SkillHub security policy to the SkillHub pack.
func (g *Gateway) SkillPolicy() *skillmarket.SecurityPolicy { return g.skillPolicy }

// SetClawHubProvider attaches the ClawHub remote skill provider.
func (g *Gateway) SetClawHubProvider(ch *skillmarket.ClawHubProvider) { g.clawHub = ch }

// ClawHubProvider exposes the ClawHub remote source to the SkillHub pack.
func (g *Gateway) ClawHubProvider() *skillmarket.ClawHubProvider { return g.clawHub }

// SetToriHubProvider attaches the ToriHub remote skill provider.
func (g *Gateway) SetToriHubProvider(th *skillmarket.ToriHubProvider) { g.toriHub = th }

// ToriHubProvider exposes the ToriHub remote source to the SkillHub pack.
func (g *Gateway) ToriHubProvider() *skillmarket.ToriHubProvider { return g.toriHub }

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
func (g *Gateway) SetSkillGrow(sg *adapter.Detector) {
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
		sg.SetOnGap(func(ctx context.Context, gap skillgrowth.Gap) {
			if g.skillGrowthPipe == nil {
				return
			}
			safego.Go("skillgrowth-pipeline", func() {
				pipeCtx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
				defer cancel()
				if _, _, err := g.skillGrowthPipe.HandleGap(pipeCtx, gap); err != nil {
					slog.Warn("skillgrowth: pipeline gap handling failed", "capability", gap.CapabilityID, "err", err)
				}
			})
		})
	}
}

// SetAuditTrail attaches the task audit trail.
func (g *Gateway) SetAuditTrail(at *audit.Trail) { g.auditTrail = at }

// SetProviderRegistry attaches the LLM provider registry.
func (g *Gateway) SetProviderRegistry(pr *llm.ProviderRegistry) { g.providerReg = pr }

// SetLLMCall attaches the shared LLM caller used by extracted route groups.
func (g *Gateway) SetLLMCall(fn workflow.LLMCallFunc) {
	g.llmCall = fn
	if g.workflowAPIHandler != nil {
		g.workflowAPIHandler.SetLLMCall(fn)
	}
}

// SetSpeechRegistry attaches the TTS/STT speech registry.
func (g *Gateway) SetSpeechRegistry(sr *speech.Registry) { g.speechReg = sr }

// SetEmotionAnalyzer attaches the emotion analyzer for text/audio emotion detection.
func (g *Gateway) SetEmotionAnalyzer(ea *emotion.Analyzer) { g.emotionAnalyzer = ea }

// SetEmotionHistory attaches the emotion history store.
func (g *Gateway) SetEmotionHistory(h *emotion.History) { g.emotionHistory = h }

// EmotionHistory exposes the emotion history store to the emotion pack
// (internal/packs/emotion), which owns /v1/emotion/history natively. May be nil.
func (g *Gateway) EmotionHistory() *emotion.History { return g.emotionHistory }

// SetInstructionStore attaches the user instruction store.
func (g *Gateway) SetInstructionStore(s *instruction.Store) { g.instructionStore = s }

// InstructionStore exposes the user instruction store to the instructions pack
// (internal/packs/instructions), which owns /v1/instructions* natively. It is
// the narrow host accessor that replaces the pack reaching into the gateway
// struct. May be nil if not configured.
func (g *Gateway) InstructionStore() *instruction.Store { return g.instructionStore }

// SetStickerMap attaches a sticker suggestion map.
func (g *Gateway) SetStickerMap(sm *emotion.StickerMap) { g.stickerMap = sm }

// StickerMap exposes the sticker suggestion map to the emotion pack
// (internal/packs/emotion), which owns /v1/emotion/stickers natively. May be nil.
func (g *Gateway) StickerMap() *emotion.StickerMap { return g.stickerMap }

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

// SetTaskStore attaches the task persistence store.
func (g *Gateway) SetTaskStore(s task.Store) {
	g.taskStore = s
}

// SetTaskRunner attaches the task execution engine.
func (g *Gateway) SetTaskRunner(r *task.Runner) { g.taskRunner = r }

// SetGapAnalyzer attaches the capability gap analyzer.
func (g *Gateway) SetGapAnalyzer(a *task.GapAnalyzer) { g.gapAnalyzer = a }

// SetStateKernel attaches the structured state kernel.
func (g *Gateway) SetStateKernel(sk *state.Kernel) { g.stateKernel = sk }

// StateKernel exposes the structured state kernel to the state pack, which owns
// /v1/state* natively. May be nil until the task engine finishes wiring.
func (g *Gateway) StateKernel() *state.Kernel { return g.stateKernel }

// SetExperienceStore attaches the reflection experience store.
func (g *Gateway) SetExperienceStore(es *reflectpkg.ExperienceStore) { g.experienceStore = es }

// ExperienceStore exposes the reflection experience store to the reflection pack.
func (g *Gateway) ExperienceStore() *reflectpkg.ExperienceStore { return g.experienceStore }

// SetReflectiveLoop attaches the canonical reflection loop used for explicit
// feedback ingestion and post-turn learning.
func (g *Gateway) SetReflectiveLoop(rl *cognikernel.ReflectiveLoop) { g.reflectiveLoop = rl }

// ReflectiveLoop returns the gateway-attached reflection loop, or nil if none.
// Callers can use it to inject additional hooks (e.g. ledger event emission)
// after WireReflectionLoop has run.
func (g *Gateway) ReflectiveLoop() *cognikernel.ReflectiveLoop { return g.reflectiveLoop }

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

// TriggerManager / TriggerRuntime expose the trigger subsystems to the triggers
// pack (internal/packs/triggers), which owns /v1/triggers* natively. May be nil.
func (g *Gateway) TriggerManager() *trigger.Manager { return g.triggerMgr }
func (g *Gateway) TriggerRuntime() *trigger.Runtime { return g.triggerRT }

// SetChannelRegistry attaches the channel registry for react/sticker operations.
func (g *Gateway) SetChannelRegistry(cr *channel.Registry) { g.channelReg = cr }

// ChannelRegistry exposes the messaging channel registry to the channels pack.
func (g *Gateway) ChannelRegistry() *channel.Registry { return g.channelReg }

// SetPreAckEmojis configures the emoji list for pre-ack reactions on incoming messages.
func (g *Gateway) SetPreAckEmojis(emojis []string) { g.preAckEmojis = emojis }

// SetWorkflowStore attaches the workflow definition/instance store.
func (g *Gateway) SetWorkflowStore(s workflow.Store) {
	g.workflowStore = s
	if g.workflowAPIHandler != nil {
		g.workflowAPIHandler.SetStore(s)
	}
}

// SetWorkflowEngine attaches the workflow execution engine.
func (g *Gateway) SetWorkflowEngine(e *workflow.Engine) {
	g.workflowEngine = e
	if g.workflowAPIHandler != nil {
		g.workflowAPIHandler.SetEngine(e)
	}
}

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

// Scheduler returns the execution scheduler.
func (g *Gateway) Scheduler() *scheduler.Scheduler { return g.scheduler }

// SetUpdateChecker sets the callback for checking available updates.
func (g *Gateway) SetUpdateChecker(fn func() (tagName, htmlURL string, hasNew bool)) {
	g.updateChecker = fn
}

// SetToriTokenStore sets the Tori OAuth2 token store for bind/unbind endpoints.
func (g *Gateway) SetToriTokenStore(ts *tori.TokenStore) { g.toriTokenStore = ts }

// SetConnectorRegistry sets the connector registry.
func (g *Gateway) SetConnectorRegistry(reg *connectors.Registry) {
	g.connectorReg = reg
}

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
func (g *Gateway) SetModelKVStore(kvs models.KVStore) {
	if g.modelMgr != nil {
		g.modelMgr.SetKVStore(kvs)
	}
}

func (g *Gateway) ModelManager() *models.Manager {
	return g.modelMgr
}

func (g *Gateway) ProviderModels() []models.ProviderModel {
	if g.providerReg == nil {
		return nil
	}
	providers := g.providerReg.List()
	out := make([]models.ProviderModel, 0, len(providers))
	for _, p := range providers {
		out = append(out, models.ProviderModel{
			ID:      p.ID,
			Model:   p.Model,
			Type:    string(p.Type),
			BaseURL: p.BaseURL,
		})
	}
	return out
}

func (g *Gateway) DeleteProviderModel(id string) bool {
	if g.providerReg == nil {
		return false
	}
	for _, p := range g.providerReg.List() {
		syntheticID := p.ID + "-" + p.Model
		if syntheticID == id || p.ID == id {
			_ = g.providerReg.Delete(p.ID)
			return true
		}
	}
	return false
}

// SetUsageKVStore injects a Ledger KV store into the usage tracker for persistence.
func (g *Gateway) SetUsageKVStore(kvs usageKVStore) {
	if g.usage != nil {
		g.usage.SetKVStore(kvs)
	}
}

// SetLedgerHealthChecker attaches the persistent state health check used by probes.
func (g *Gateway) SetLedgerHealthChecker(h healthChecker) { g.ledgerHealth = h }

// SetOnboardingKVStore injects a Ledger KV store for persisting first-run
// onboarding completion (replaces fragile browser localStorage).
func (g *Gateway) SetOnboardingKVStore(kvs onboardingKVStore) { g.onboardingKV = kvs }

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

// SetCogniKernel attaches the CogniKernel so the offline dream loop can be
// surfaced read-only via /v1/reverie/dream/status.
func (g *Gateway) SetCogniKernel(k *cognikernel.CogniKernel) { g.cogniKernel = k }

// SetCogniFederation attaches the federation manager.
func (g *Gateway) SetCogniFederation(cf *cogni.CogniFederation) { g.cogniFederation = cf }

// SetCogniCostTracker attaches the economics cost tracker.
func (g *Gateway) SetCogniCostTracker(ct *cogni.CostTracker) { g.cogniCostTracker = ct }

// SetCogniBus attaches the bidding router for POST /v1/cognis/route.
func (g *Gateway) SetCogniBus(b *cogni.CogniBus) { g.cogniBus = b }

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
