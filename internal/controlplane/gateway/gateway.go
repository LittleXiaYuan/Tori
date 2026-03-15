package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"yunque-agent/internal/agentcore/adaptive"
	"yunque-agent/internal/agentcore/audit"
	"yunque-agent/internal/agentcore/bots"
	"yunque-agent/internal/agentcore/costtrack"
	"yunque-agent/internal/agentcore/cron"
	"yunque-agent/internal/agentcore/distill"
	"yunque-agent/internal/agentcore/review"
	"yunque-agent/internal/agentcore/skillgrow"
	"yunque-agent/internal/agentcore/tools"
	"yunque-agent/internal/agentcore/trust"

	// tools imported via handlers_tools.go
	"yunque-agent/internal/agentcore/embeddings"
	"yunque-agent/internal/agentcore/emotion"
	"yunque-agent/internal/agentcore/federation"
	"yunque-agent/internal/agentcore/guardrails"
	"yunque-agent/internal/agentcore/heartbeat"
	"yunque-agent/internal/agentcore/identity"
	"yunque-agent/internal/agentcore/inbox"
	"yunque-agent/internal/agentcore/iterate"
	"yunque-agent/internal/agentcore/knowledge"
	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/memory"
	"yunque-agent/internal/agentcore/persona"
	"yunque-agent/internal/agentcore/planner"
	reflectpkg "yunque-agent/internal/agentcore/reflect"
	"yunque-agent/internal/agentcore/router"
	agentrt "yunque-agent/internal/agentcore/runtime"
	"yunque-agent/internal/agentcore/selfheal"
	"yunque-agent/internal/agentcore/session"
	"yunque-agent/internal/agentcore/skillmarket"
	"yunque-agent/internal/agentcore/speech"
	"yunque-agent/internal/agentcore/state"
	"yunque-agent/internal/agentcore/subagent"
	"yunque-agent/internal/agentcore/task"
	"yunque-agent/internal/agentcore/trigger"
	"yunque-agent/internal/agentcore/websearch"
	"yunque-agent/internal/apperror"
	"yunque-agent/internal/controlplane/tenant"
	"yunque-agent/internal/execution/channel"
	"yunque-agent/internal/execution/scheduler"
	"yunque-agent/internal/observe"
	"yunque-agent/internal/version"
	"yunque-agent/pkg/plugin"
	"yunque-agent/pkg/skills"
)

type contextKey string

const ctxKeyReqID contextKey = "req_id"
const ctxKeyTenantID contextKey = "tenant_id"

// RequestID extracts the request ID from context.
func RequestID(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyReqID).(string); ok {
		return v
	}
	return ""
}

// Gateway is the HTTP API server for the agent.
type Gateway struct {
	planner         *planner.Planner
	tenants         *tenant.Manager
	memory          *memory.Manager
	registry        *skills.Registry
	scheduler       *scheduler.Scheduler
	convStore       *session.Store
	pluginReg       *plugin.Registry
	feishuAPI       *channel.FeishuAPI
	learning        *reflectpkg.LearningLoop
	limiter         *RateLimiter
	jwtCfg          *JWTConfig
	usage           *UsageTracker
	metrics         *observe.Metrics
	pipeline        *memory.Pipeline
	persona         *persona.Persona
	personaChain    *persona.PriorityChain
	heartbeat       *heartbeat.Service
	inbox           *inbox.Store
	botMgr          *bots.Manager
	searchReg       *websearch.Registry
	smartRouter     *router.Router
	identityRes     *identity.Resolver
	healer          *selfheal.Healer
	lifecycle       *selfheal.Lifecycle
	costTracker     *costtrack.Tracker
	forkTree        *session.ForkTree
	forkPersister   *session.ForkPersister
	embedResolver   *embeddings.Resolver
	subagentMgr     *subagent.Manager
	handoffReg      *subagent.HandoffRegistry
	orchestrator    *memory.Orchestrator
	zhGuard         *guardrails.Pipeline
	adaptiveLoop    *adaptive.Loop
	auditChain      *audit.Chain
	skillMarket     *skillmarket.Market
	fedHub          *federation.Hub
	knowledgeStore  *knowledge.Store
	cronMgr         *cron.Manager
	toolsMgr        *tools.ProcessManager
	runtimePool     *agentrt.Pool
	bindingRouter   *agentrt.Router
	toolGuard       *guardrails.ToolGuard
	egressGuard     *guardrails.EgressGuard
	skillInstaller  *skillmarket.Installer
	skillPolicy     *skillmarket.SecurityPolicy
	clawHub         *skillmarket.ClawHubProvider
	toriHub         *skillmarket.ToriHubProvider
	pluginLoader    *plugin.Loader
	iterateEngine   *iterate.Engine
	trustTracker    *trust.Tracker
	distiller       *distill.Distiller
	reviewGate      *review.Gate
	skillGrow       *skillgrow.Detector
	auditTrail      *audit.Trail
	providerReg     *llm.ProviderRegistry
	speechReg       *speech.Registry
	emotionAnalyzer *emotion.Analyzer
	emotionHistory  *emotion.History
	stickerMap      *emotion.StickerMap
	channelReg      *channel.Registry
	emotionShift    *planner.EmotionShiftDetector // event-driven Reverie trigger
	factHook        *planner.FactEventHook        // event-driven Reverie trigger on high-value facts
	reverie         *planner.Reverie              // Reverie inner monologue system (for API access)
	taskStore       *task.Store                   // task runtime persistence
	taskRunner      *task.Runner                  // task execution engine
	gapAnalyzer     *task.GapAnalyzer             // capability gap detection
	stateKernel     *state.Kernel                 // structured state kernel
	experienceStore *reflectpkg.ExperienceStore   // reflection experience store
	templateStore   *task.TemplateStore           // task template store
	workMemMgr      *task.WorkingMemoryManager    // task working memory
	threadMgr       *task.ThreadManager           // task thread manager
	triggerRT       *trigger.Runtime              // trigger runtime (legacy)
	triggerMgr      *trigger.Manager              // unified trigger manager
	preAckEmojis    []string                      // emoji list for pre-ack reactions (e.g., ["👍","🤔","💡"])
	allowedOrigins  []string
	mux             *http.ServeMux
	reqCount        atomic.Int64
	startTime       time.Time
}

// New creates a new Gateway.
func New(p *planner.Planner, t *tenant.Manager, m *memory.Manager, r *skills.Registry, s *scheduler.Scheduler, cs *session.Store, pr *plugin.Registry, fa *channel.FeishuAPI, ll *reflectpkg.LearningLoop, jwtCfg *JWTConfig, met *observe.Metrics, pipeline *memory.Pipeline, per *persona.Persona) *Gateway {
	if met == nil {
		met = observe.New()
	}
	g := &Gateway{planner: p, tenants: t, memory: m, registry: r, scheduler: s, convStore: cs, pluginReg: pr, feishuAPI: fa, learning: ll, jwtCfg: jwtCfg, limiter: NewRateLimiter(30, time.Minute), usage: NewUsageTracker(), metrics: met, pipeline: pipeline, persona: per, mux: http.NewServeMux(), startTime: time.Now()}
	g.routes()
	return g
}

// SetRateLimit reconfigures the rate limiter.
func (g *Gateway) SetRateLimit(maxRequests int, window time.Duration) {
	g.limiter = NewRateLimiter(maxRequests, window)
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

// SetKnowledgeStore attaches the knowledge base.
func (g *Gateway) SetKnowledgeStore(ks *knowledge.Store) { g.knowledgeStore = ks }

// SetCronManager attaches the cron job manager.
func (g *Gateway) SetCronManager(cm *cron.Manager) { g.cronMgr = cm }

// SetToolsManager attaches the process/tools manager.
func (g *Gateway) SetToolsManager(tm *tools.ProcessManager) { g.toolsMgr = tm }

// SetPluginLoader attaches a plugin directory loader for CRUD operations.
func (g *Gateway) SetPluginLoader(l *plugin.Loader) { g.pluginLoader = l }

// SetRuntimePool attaches the multi-agent runtime pool.
func (g *Gateway) SetRuntimePool(p *agentrt.Pool) { g.runtimePool = p }

// SetBindingRouter attaches the binding-based agent router.
func (g *Gateway) SetBindingRouter(r *agentrt.Router) { g.bindingRouter = r }

// SetToolGuard attaches the tool call parameter guard.
func (g *Gateway) SetToolGuard(tg *guardrails.ToolGuard) { g.toolGuard = tg }

// SetEgressGuard attaches the output sanitization guard.
func (g *Gateway) SetEgressGuard(eg *guardrails.EgressGuard) { g.egressGuard = eg }

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

// SetSkillGrow attaches the skill growth detector.
func (g *Gateway) SetSkillGrow(sg *skillgrow.Detector) { g.skillGrow = sg }

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

// SetStickerMap attaches a sticker suggestion map.
func (g *Gateway) SetStickerMap(sm *emotion.StickerMap) { g.stickerMap = sm }

// SetEmotionShiftDetector attaches the emotion shift detector for Reverie event-driven triggers.
func (g *Gateway) SetEmotionShiftDetector(d *planner.EmotionShiftDetector) { g.emotionShift = d }

// SetFactEventHook attaches the fact event hook for Reverie event-driven triggers.
func (g *Gateway) SetFactEventHook(h *planner.FactEventHook) { g.factHook = h }

// SetReverie attaches the Reverie system for API access.
func (g *Gateway) SetReverie(r *planner.Reverie) { g.reverie = r }

// SetTaskStore attaches the task persistence store.
func (g *Gateway) SetTaskStore(s *task.Store) { g.taskStore = s }

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

// MountPluginRoutes discovers all UIPlugin HTTP handlers from the plugin registry
// and mounts them on the mux under /v1/ext/{plugin-key}/{path}.
// Call this after all plugins are registered and before ListenAndServe.
func (g *Gateway) MountPluginRoutes() {
	handlers := g.pluginReg.AllHTTPHandlers()
	for path, handler := range handlers {
		slog.Info("mounted plugin route", "path", path)
		g.mux.HandleFunc(path, g.requireAuth(handler))
	}
	if len(handlers) > 0 {
		slog.Info("plugin routes mounted", "count", len(handlers))
	}
}

// SetAllowedOrigins configures CORS allowed origins. Use "*" for wildcard (dev only).
func (g *Gateway) SetAllowedOrigins(origins []string) {
	g.allowedOrigins = origins
}

func (g *Gateway) corsOrigin(origin string) string {
	if len(g.allowedOrigins) == 0 {
		return "*"
	}
	for _, o := range g.allowedOrigins {
		if o == "*" {
			return "*"
		}
		if o == origin {
			return origin
		}
	}
	if origin == "" {
		return g.allowedOrigins[0]
	}
	return ""
}

// statusWriter wraps http.ResponseWriter to capture status code.
type statusWriter struct {
	http.ResponseWriter
	code int
}

func (sw *statusWriter) WriteHeader(code int) {
	sw.code = code
	sw.ResponseWriter.WriteHeader(code)
}

// Flush implements http.Flusher so SSE streaming works through the wrapper.
func (sw *statusWriter) Flush() {
	if f, ok := sw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// ServeHTTP implements http.Handler with request logging.
func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	count := g.reqCount.Add(1)
	reqID := fmt.Sprintf("%d-%d", start.UnixMilli(), count)

	origin := r.Header.Get("Origin")
	allowed := g.corsOrigin(origin)
	w.Header().Set("Access-Control-Allow-Origin", allowed)
	w.Header().Set("Access-Control-Allow-Methods", "GET,POST,DELETE,OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization,Content-Type,X-API-Key")
	w.Header().Set("Access-Control-Max-Age", "86400")
	w.Header().Set("X-Request-ID", reqID)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	if r.Method == "OPTIONS" {
		w.WriteHeader(204)
		return
	}
	// Limit request body size: 32MB for uploads, 1MB for everything else
	if r.Body != nil {
		maxBody := int64(1 << 20) // 1MB
		if strings.HasPrefix(r.URL.Path, "/v1/upload") {
			maxBody = 32 << 20 // 32MB
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	}
	sw := &statusWriter{ResponseWriter: w, code: 200}
	ctx := context.WithValue(r.Context(), ctxKeyReqID, reqID)
	g.mux.ServeHTTP(sw, r.WithContext(ctx))
	slog.Info("http", "method", r.Method, "path", r.URL.Path, "status", sw.code, "duration_ms", time.Since(start).Milliseconds(), "req_id", reqID)
	if g.auditChain != nil {
		g.auditChain.Append(audit.EventSystem, tenantFromCtx(ctx), r.Method+" "+r.URL.Path, fmt.Sprintf("status=%d dur=%dms", sw.code, time.Since(start).Milliseconds()))
	}
}

func (g *Gateway) routes() {
	// Root "/" serves the embedded Next.js static UI (SPA).
	// Falls back to pure HTML dashboard if no frontend build is available.
	// Specific API routes below take priority over this catch-all.
	g.mux.HandleFunc("/", g.serveWebUI)

	g.mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		breaker := g.planner.LLMBreaker()
		health := map[string]any{
			"status":        "ok",
			"version":       version.Version,
			"breaker_state": breaker.State(),
			"uptime_sec":    int(time.Since(g.startTime).Seconds()),
		}
		if breaker.State() == "open" {
			health["status"] = "degraded"
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(health)
	})
	g.mux.HandleFunc("/v1/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(version.Get())
	})

	g.mux.HandleFunc("/v1/tenants", g.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			g.handleCreateTenant(w, r)
		case http.MethodGet:
			g.handleListTenants(w, r)
		default:
			apperror.WriteCode(w, apperror.CodeMethodNotAllow, "method not allowed")
		}
	}))

	g.mux.HandleFunc("/v1/chat", g.requireAuth(g.limiter.Middleware(g.handleChat)))
	g.mux.HandleFunc("/v1/chat/stream", g.requireAuth(g.limiter.Middleware(g.handleStreamChat)))
	g.mux.HandleFunc("/v1/skills", g.requireAuth(g.handleSkills))
	g.mux.HandleFunc("/v1/memory/stats", g.requireAuth(g.handleMemoryStats))
	g.mux.HandleFunc("/v1/memory/search", g.requireAuth(g.handleMemorySearch))
	g.mux.HandleFunc("/v1/memory/add", g.requireAuth(g.handleMemoryAdd))
	g.mux.HandleFunc("/v1/sandbox/exec", g.requireAuth(g.handleSandboxExec))
	g.mux.HandleFunc("/v1/system/info", g.requireAuth(g.handleSystemInfo))
	g.mux.HandleFunc("/v1/metrics", g.requireAuth(g.handleMetrics))
	g.mux.HandleFunc("/v1/metrics/prometheus", g.handleMetricsPrometheus)
	g.mux.HandleFunc("/v1/scheduler/jobs", g.requireAuth(g.handleSchedulerJobs))
	g.mux.HandleFunc("/v1/scheduler/add", g.requireAuth(g.handleSchedulerAdd))
	g.mux.HandleFunc("/v1/scheduler/remove", g.requireAuth(g.handleSchedulerRemove))
	g.mux.HandleFunc("/v1/conversations", g.requireAuth(g.handleConversations))
	g.mux.HandleFunc("/v1/conversations/messages", g.requireAuth(g.handleConversationMessages))
	g.mux.HandleFunc("/v1/conversations/manage", g.requireAuth(g.handleConversationManage))
	g.mux.HandleFunc("/webhook/feishu", g.handleFeishuWebhook)
	g.mux.HandleFunc("/v1/plugins", g.requireAuth(g.handlePlugins))
	g.mux.HandleFunc("/v1/plugins/toggle", g.requireAuth(g.handlePluginToggle))
	g.mux.HandleFunc("/v1/plugins/create", g.requireAuth(g.handlePluginCreate))
	g.mux.HandleFunc("/v1/plugins/delete", g.requireAuth(g.handlePluginDelete))
	g.mux.HandleFunc("/v1/plugins/files", g.requireAuth(g.handlePluginFiles))
	g.mux.HandleFunc("/v1/plugins/ui", g.requireAuth(g.handlePluginUI))
	g.mux.HandleFunc("/v1/upload", g.requireAuth(g.handleFileUpload))
	g.mux.HandleFunc("/v1/memory/compact", g.requireAuth(g.handleMemoryCompact))
	g.mux.HandleFunc("/v1/persona", g.requireAuth(g.handlePersona))
	g.mux.HandleFunc("/v1/persona/skills", g.requireAuth(g.handlePersonaSkills))
	g.mux.HandleFunc("/v1/persona/presets", g.requireAuth(g.handlePresets))
	g.mux.HandleFunc("/v1/persona/presets/custom", g.requireAuth(g.handleCustomPreset))
	g.mux.HandleFunc("/v1/persona/presets/features", g.requireAuth(g.handlePresetFeatures))
	g.mux.HandleFunc("/v1/emotion/stickers", g.requireAuth(g.handleStickers))
	g.mux.HandleFunc("/v1/emotion/history", g.requireAuth(g.handleEmotionHistory))
	g.mux.HandleFunc("/v1/heartbeat", g.requireAuth(g.handleHeartbeat))
	g.mux.HandleFunc("/v1/heartbeat/trigger", g.requireAuth(g.handleHeartbeatTrigger))
	g.mux.HandleFunc("/v1/heartbeat/logs", g.requireAuth(g.handleHeartbeatLogs))
	g.mux.HandleFunc("/v1/inbox", g.requireAuth(g.handleInbox))
	g.mux.HandleFunc("/v1/inbox/read", g.requireAuth(g.handleInboxRead))
	g.mux.HandleFunc("/v1/bots", g.requireAuth(g.handleBots))
	g.mux.HandleFunc("/v1/bots/detail", g.requireAuth(g.handleBotDetail))
	g.mux.HandleFunc("/v1/search", g.requireAuth(g.handleSearch))
	g.mux.HandleFunc("/v1/search/providers", g.requireAuth(g.handleSearchProviders))
	g.mux.HandleFunc("/v1/graph/entities", g.requireAuth(g.handleGraphEntities))
	g.mux.HandleFunc("/v1/graph/relations", g.requireAuth(g.handleGraphRelations))
	g.mux.HandleFunc("/v1/graph/context", g.requireAuth(g.handleGraphContext))
	g.mux.HandleFunc("/v1/graph/stats", g.requireAuth(g.handleGraphStats))
	g.mux.HandleFunc("/v1/router/stats", g.requireAuth(g.handleRouterStats))
	g.mux.HandleFunc("/v1/identity/resolve", g.requireAuth(g.handleIdentityResolve))
	g.mux.HandleFunc("/v1/identity/profiles", g.requireAuth(g.handleIdentityProfiles))
	g.mux.HandleFunc("/v1/cost/summary", g.requireAuth(g.handleCostSummary))
	g.mux.HandleFunc("/v1/cost/budget", g.requireAuth(g.handleCostBudget))
	g.mux.HandleFunc("/v1/cost/task", g.requireAuth(g.handleCostByTask))
	g.mux.HandleFunc("/v1/cost/task/timeline", g.requireAuth(g.handleCostTaskTimeline))
	g.mux.HandleFunc("/v1/cost/breakdown", g.requireAuth(g.handleCostBreakdown))
	g.mux.HandleFunc("/v1/cost/history", g.requireAuth(g.handleCostHistory))
	g.mux.HandleFunc("/v1/cost/alerts", g.requireAuth(g.handleCostAlerts))
	g.mux.HandleFunc("/v1/fork", g.requireAuth(g.handleFork))
	g.mux.HandleFunc("/v1/fork/branch", g.requireAuth(g.handleForkBranch))
	g.mux.HandleFunc("/v1/fork/list", g.requireAuth(g.handleForkList))
	g.mux.HandleFunc("/v1/embeddings", g.requireAuth(g.handleEmbeddings))
	g.mux.HandleFunc("/v1/subagent", g.requireAuth(g.handleSubagent))
	g.mux.HandleFunc("/v1/subagent/message", g.requireAuth(g.handleSubagentMessage))
	g.mux.HandleFunc("/v1/system/stats", g.requireAuth(g.handleSystemStats))
	g.mux.HandleFunc("/v1/cache/stats", g.requireAuth(g.handleCacheStats))
	g.mux.HandleFunc("/v1/ws", g.requireAuth(g.handleWebSocket))
	g.mux.HandleFunc("/v1/token", g.handleTokenGenerate)
	g.mux.HandleFunc("/v1/usage", g.requireAuth(g.handleUsage))
	g.mux.HandleFunc("/v1/quota", g.requireAuth(g.handleSetQuota))
	g.mux.HandleFunc("/v1/audit/tail", g.requireAuth(g.handleAuditTail))
	g.mux.HandleFunc("/v1/audit/verify", g.requireAuth(g.handleAuditVerify))
	g.mux.HandleFunc("/v1/audit/stats", g.requireAuth(g.handleAuditStats))
	g.mux.HandleFunc("/v1/market/search", g.requireAuth(g.handleMarketSearch))
	g.mux.HandleFunc("/v1/market/top", g.requireAuth(g.handleMarketTop))
	g.mux.HandleFunc("/v1/market/stats", g.requireAuth(g.handleMarketStats))
	g.mux.HandleFunc("/v1/federation/peers", g.requireAuth(g.handleFedPeers))
	g.mux.HandleFunc("/v1/federation/stats", g.requireAuth(g.handleFedStats))
	g.mux.HandleFunc("/v1/knowledge/search", g.requireAuth(g.handleKBSearch))
	g.mux.HandleFunc("/v1/knowledge/sources", g.requireAuth(g.handleKBSources))
	g.mux.HandleFunc("/v1/knowledge/stats", g.requireAuth(g.handleKBStats))
	g.mux.HandleFunc("/v1/knowledge/upload", g.requireAuth(g.handleKBUpload))
	g.mux.HandleFunc("/v1/knowledge/ingest", g.requireAuth(g.handleKBIngest))
	g.mux.HandleFunc("/v1/knowledge/import-url", g.requireAuth(g.handleKBImportURL))
	g.mux.HandleFunc("/v1/knowledge/import-repo", g.requireAuth(g.handleKBImportRepo))
	g.mux.HandleFunc("/v1/knowledge/source", g.requireAuth(g.handleKBDelete))
	g.mux.HandleFunc("/v1/cron/list", g.requireAuth(g.handleCronList))
	g.mux.HandleFunc("/v1/cron/add", g.requireAuth(g.handleCronAdd))
	g.mux.HandleFunc("/v1/cron/remove", g.requireAuth(g.handleCronRemove))
	g.mux.HandleFunc("/v1/cron/run", g.requireAuth(g.handleCronRun))
	g.mux.HandleFunc("/v1/triggers", g.requireAuth(g.handleTriggers))
	g.mux.HandleFunc("/v1/triggers/emit", g.requireAuth(g.handleTriggerEmit))
	g.mux.HandleFunc("/v1/triggers/v2", g.requireAuth(g.handleTriggersV2))
	g.mux.HandleFunc("/v1/triggers/v2/emit", g.requireAuth(g.handleTriggersV2Emit))
	g.mux.HandleFunc("/v1/triggers/v2/runs", g.requireAuth(g.handleTriggersV2Runs))
	g.mux.HandleFunc("/v1/triggers/v2/events", g.requireAuth(g.handleTriggersV2Events))
	g.mux.HandleFunc("/v1/tools/exec", g.requireAuth(g.handleToolExec))
	g.mux.HandleFunc("/v1/tools/list", g.requireAuth(g.handleToolList))
	g.mux.HandleFunc("/v1/tools/poll", g.requireAuth(g.handleToolPoll))
	g.mux.HandleFunc("/v1/tools/kill", g.requireAuth(g.handleToolKill))

	// React & Sticker API
	g.mux.HandleFunc("/v1/react", g.requireAuth(g.handleReact))
	g.mux.HandleFunc("/v1/sticker/send", g.requireAuth(g.handleSendSticker))

	// Channel Groups API
	g.mux.HandleFunc("/v1/channels/groups", g.requireAuth(g.handleChannelGroups))

	// Reverie API (inner monologue visualization & operations)
	g.mux.HandleFunc("/v1/reverie/journal", g.requireAuth(g.handleReverieJournal))
	g.mux.HandleFunc("/v1/reverie/stats", g.requireAuth(g.handleReverieStats))
	g.mux.HandleFunc("/v1/reverie/config", g.requireAuth(g.handleReverieConfig))
	g.mux.HandleFunc("/v1/reverie/think", g.requireAuth(g.handleReverieThink))
	g.mux.HandleFunc("/v1/reverie/thought", g.requireAuth(g.handleReverieDeleteThought))
	g.mux.HandleFunc("/v1/reverie/targets", g.requireAuth(g.handleReverieTargets))
	g.mux.HandleFunc("/v1/reverie/actions", g.requireAuth(g.handleReverieActions))

	// Task Runtime API
	g.mux.HandleFunc("/v1/tasks", g.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			g.handleTaskList(w, r)
		case http.MethodPost:
			g.handleTaskCreate(w, r)
		case http.MethodDelete:
			g.handleTaskDelete(w, r)
		default:
			apperror.WriteCode(w, apperror.CodeBadRequest, "method not allowed")
		}
	}))
	g.mux.HandleFunc("/v1/tasks/run", g.requireAuth(g.handleTaskRun))
	g.mux.HandleFunc("/v1/tasks/cancel", g.requireAuth(g.handleTaskCancel))
	g.mux.HandleFunc("/v1/tasks/pause", g.requireAuth(g.handleTaskPause))
	g.mux.HandleFunc("/v1/tasks/resume", g.requireAuth(g.handleTaskResume))
	g.mux.HandleFunc("/v1/tasks/restart", g.requireAuth(g.handleTaskRestart))
	g.mux.HandleFunc("/v1/tasks/gaps", g.requireAuth(g.handleGaps))
	g.mux.HandleFunc("/v1/tasks/gaps/resolve", g.requireAuth(g.handleGapResolve))
	g.mux.HandleFunc("/v1/tasks/memory", g.requireAuth(g.handleTaskWorkingMemory))
	g.mux.HandleFunc("/v1/tasks/threads", g.requireAuth(g.handleTaskThread))
	g.mux.HandleFunc("/v1/tasks/templates", g.requireAuth(g.handleTemplates))
	g.mux.HandleFunc("/v1/tasks/templates/instantiate", g.requireAuth(g.handleTemplateInstantiate))

	// Document Generation API
	g.mux.HandleFunc("/v1/documents/generate", g.requireAuth(g.handleDocGenerate))

	// State Kernel API
	g.mux.HandleFunc("/v1/state", g.requireAuth(g.handleStateSnapshot))
	g.mux.HandleFunc("/v1/state/goals", g.requireAuth(g.handleStateGoals))
	g.mux.HandleFunc("/v1/state/focus", g.requireAuth(g.handleStateFocus))
	g.mux.HandleFunc("/v1/state/resources", g.requireAuth(g.handleStateResources))

	// Reflection / Experience
	g.mux.HandleFunc("/v1/reflect/experiences", g.requireAuth(g.handleExperiences))
	g.mux.HandleFunc("/v1/reflect/strategies", g.requireAuth(g.handleStrategies))

	// SkillHub API
	g.mux.HandleFunc("/api/skillhub/search", g.requireAuth(g.handleSkillHubSearch))
	g.mux.HandleFunc("/api/skillhub/install", g.requireAuth(g.handleSkillHubInstall))
	g.mux.HandleFunc("/api/skillhub/installed", g.requireAuth(g.handleSkillHubInstalled))
	g.mux.HandleFunc("/api/skillhub/uninstall", g.requireAuth(g.handleSkillHubUninstall))
	g.mux.HandleFunc("/api/skillhub/trending", g.requireAuth(g.handleSkillHubTrending))
	g.mux.HandleFunc("/api/skillhub/detail", g.requireAuth(g.handleSkillHubDetail))
	g.mux.HandleFunc("/api/skillhub/check-updates", g.requireAuth(g.handleSkillHubCheckUpdates))
	g.mux.HandleFunc("/api/skillhub/update", g.requireAuth(g.handleSkillHubUpdate))
	g.mux.HandleFunc("/api/skillhub/rollback", g.requireAuth(g.handleSkillHubRollback))
	g.mux.HandleFunc("/api/skillhub/versions", g.requireAuth(g.handleSkillHubVersions))
	g.mux.HandleFunc("/api/skillhub/policy", g.requireAuth(g.handleSkillHubPolicy))
	g.mux.HandleFunc("/api/skillhub/policy/check", g.requireAuth(g.handleSkillHubPolicyCheck))
	g.mux.HandleFunc("/api/skillhub/analytics", g.requireAuth(g.handleSkillHubAnalytics))

	// Iterate (self-improvement) API
	g.mux.HandleFunc("/api/iterate/proposals", g.requireAuth(g.handleIterateProposals))
	g.mux.HandleFunc("/api/iterate/approve", g.requireAuth(g.handleIterateApprove))
	g.mux.HandleFunc("/api/iterate/reject", g.requireAuth(g.handleIterateReject))
	g.mux.HandleFunc("/api/iterate/trigger", g.requireAuth(g.handleIterateTrigger))
	g.mux.HandleFunc("/api/iterate/status", g.requireAuth(g.handleIterateStatus))

	// Innovation API (Phase I)
	g.mux.HandleFunc("/api/trust/scores", g.requireAuth(g.handleTrustScores))
	g.mux.HandleFunc("/api/trust/reset", g.requireAuth(g.handleTrustReset))
	g.mux.HandleFunc("/api/audit/trail", g.requireAuth(g.handleAuditTrail))
	g.mux.HandleFunc("/api/skillgrow/patterns", g.requireAuth(g.handleSkillGrowPatterns))
	g.mux.HandleFunc("/api/review/status", g.requireAuth(g.handleReviewStatus))

	// Settings API (env config management + setup check)
	g.mux.HandleFunc("/api/settings/schema", g.requireAuth(g.handleSettingsSchema))
	g.mux.HandleFunc("/api/settings/config", g.requireAuth(g.handleSettingsConfig))
	g.mux.HandleFunc("/api/settings/check", g.handleSettingsCheck) // no auth — needed for first-run setup

	// Provider API (LLM provider management)
	g.mux.HandleFunc("/api/providers", g.requireAuth(g.handleProviderList))
	g.mux.HandleFunc("/api/providers/test", g.requireAuth(g.handleProviderTest))
	g.mux.HandleFunc("/api/providers/enable", g.requireAuth(g.handleProviderEnable))
	g.mux.HandleFunc("/api/providers/disable", g.requireAuth(g.handleProviderDisable))
	g.mux.HandleFunc("/api/providers/switch-model", g.requireAuth(g.handleProviderSwitchModel))
	g.mux.HandleFunc("/api/providers/session", g.requireAuth(g.handleProviderSessionOverride))
	g.mux.HandleFunc("/api/providers/local/discover", g.requireAuth(g.handleLocalDiscover))
	g.mux.HandleFunc("/api/providers/local/register", g.requireAuth(g.handleLocalRegister))

	// Backup & Restore
	g.mux.HandleFunc("/v1/backup/export", g.requireAuth(g.handleBackupExport))
	g.mux.HandleFunc("/v1/backup/import", g.requireAuth(g.handleBackupImport))
	g.mux.HandleFunc("/v1/backup/info", g.requireAuth(g.handleBackupInfo))

	// Speech (TTS / STT)
	g.mux.HandleFunc("/v1/speech/tts", g.requireAuth(g.handleTTS))
	g.mux.HandleFunc("/v1/speech/stt", g.requireAuth(g.handleSTT))
	g.mux.HandleFunc("/v1/speech/voices", g.requireAuth(g.handleVoices))

	// WebChat widget (public — no auth, to allow embedding)
	g.mux.HandleFunc("/v1/webchat/widget.js", g.handleWebChatWidget)
}

// --- Auth middleware ---

type ctxKey string

const ctxTenantKey ctxKey = "tenant_id"

func (g *Gateway) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract token from X-API-Key or Authorization header
		token := r.Header.Get("X-API-Key")
		if token == "" {
			auth := r.Header.Get("Authorization")
			if strings.HasPrefix(auth, "Bearer ") {
				token = strings.TrimPrefix(auth, "Bearer ")
			}
		}

		// Localhost bypass: auto-authenticate with default tenant for local
		// desktop access (browser same machine). No key required.
		if token == "" {
			if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
				if host == "127.0.0.1" || host == "::1" {
					tenants := g.tenants.List()
					if len(tenants) > 0 {
						ctx := contextWithTenant(r.Context(), tenants[0].ID)
						next.ServeHTTP(w, r.WithContext(ctx))
						return
					}
				}
			}
		}

		if token == "" {
			apperror.WriteCode(w, apperror.CodeUnauthorized, "missing credentials")
			return
		}

		// Try API Key first
		if t := g.tenants.ByAPIKey(token); t != nil {
			ctx := contextWithTenant(r.Context(), t.ID)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// Try JWT
		if g.jwtCfg != nil {
			claims, err := ValidateJWT(*g.jwtCfg, token)
			if err == nil {
				ctx := contextWithTenant(r.Context(), claims.TenantID)
				ctx = context.WithValue(ctx, ctxRoleKey, claims.Role)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}

		apperror.WriteCode(w, apperror.CodeUnauthorized, "invalid credentials")
	}
}

func contextWithTenant(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxTenantKey, id)
}

func tenantFromCtx(ctx context.Context) string {
	v, _ := ctx.Value(ctxTenantKey).(string)
	return v
}
