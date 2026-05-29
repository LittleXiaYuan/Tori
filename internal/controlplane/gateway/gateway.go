package gateway

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"yunque-agent/internal/agentcore/adaptive"
	"yunque-agent/internal/agentcore/approval"
	"yunque-agent/internal/agentcore/audit"
	"yunque-agent/internal/agentcore/bots"
	"yunque-agent/internal/agentcore/browserskill"
	"yunque-agent/internal/agentcore/costtrack"
	"yunque-agent/internal/agentcore/cron"
	"yunque-agent/internal/agentcore/rbac"
	"yunque-agent/internal/agentcore/review"
	"yunque-agent/internal/agentcore/tools"
	"yunque-agent/internal/agentcore/trust"
	"yunque-agent/internal/agentcore/workflow"
	"yunque-agent/internal/agentcore/skillgrowth/adapter"

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
	"yunque-agent/internal/agentcore/modes"
	"yunque-agent/internal/agentcore/notify"
	"yunque-agent/internal/agentcore/persona"
	"yunque-agent/internal/agentcore/planner"
	"yunque-agent/internal/agentcore/router"
	agentrt "yunque-agent/internal/agentcore/runtime"
	"yunque-agent/internal/agentcore/runtime/heartbeat"
	"yunque-agent/internal/agentcore/selfheal"
	"yunque-agent/internal/agentcore/session"
	"yunque-agent/internal/agentcore/skillgrowth"
	"yunque-agent/internal/agentcore/skillmarket"
	"yunque-agent/internal/agentcore/speech"
	"yunque-agent/internal/agentcore/state"
	"yunque-agent/internal/agentcore/subagent"
	"yunque-agent/internal/agentcore/task"
	"yunque-agent/internal/agentcore/trigger"
	"yunque-agent/internal/agentcore/websearch"
	"yunque-agent/internal/apperror"
	"yunque-agent/internal/cognikernel"
	"yunque-agent/internal/connectors"
	"yunque-agent/internal/controlplane/gateway/connectorapi"
	"yunque-agent/internal/controlplane/gateway/costapi"
	"yunque-agent/internal/controlplane/gateway/forkapi"
	"yunque-agent/internal/controlplane/gateway/gwshared"
	"yunque-agent/internal/controlplane/gateway/notifyapi"
	"yunque-agent/internal/controlplane/gateway/schedulerapi"
	"yunque-agent/internal/controlplane/gateway/workflowapi"
	"yunque-agent/internal/controlplane/tenant"
	"yunque-agent/internal/execution/channel"
	"yunque-agent/internal/execution/sandbox"
	"yunque-agent/internal/execution/scheduler"
	"yunque-agent/internal/agentcore/selfheal/iterate"
	reflectpkg "yunque-agent/internal/experimental/reflect"
	"yunque-agent/internal/integrations/mineru"
	mcpserver "yunque-agent/internal/mcp/server"
	"yunque-agent/internal/observe"
	"yunque-agent/internal/orchestrator"
	"yunque-agent/internal/tori"
	"yunque-agent/pkg/cogni"
	"yunque-agent/pkg/packruntime"
	"yunque-agent/pkg/plugin"
	"yunque-agent/pkg/skills"
)

type ctxKeyType string

const ctxKeyReqID ctxKeyType = "req_id"

type documentParser interface {
	Enabled() bool
	ParseFile(ctx context.Context, filePath string) (*mineru.ParseResult, error)
}

type healthChecker interface {
	HealthCheck(ctx context.Context) error
}

// RequestID extracts the request ID from context.
func RequestID(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyReqID).(string); ok {
		return v
	}
	return ""
}

// Gateway is the HTTP API server for the agent.
//
// Field groupings mirror the route registrations in routes.go. When this
// struct grows beyond ~15 fields per group, consider extracting the group
// into its own sub-struct (embed it to keep handler code unchanged).
type Gateway struct {

	// ── Chat & Conversation ────────────────────
	planner       *planner.Planner
	convStore     *session.Store
	forkTree      *session.ForkTree
	forkPersister *session.ForkPersister
	subagentMgr   *subagent.Manager
	handoffReg    *subagent.HandoffRegistry
	preAckEmojis  []string

	// ── Memory & Knowledge ─────────────────────
	memory         *memory.Manager
	orchestrator   *memory.Orchestrator
	pipeline       *memory.Pipeline
	knowledgeStore *knowledge.Store
	wikiStore      *knowledge.WikiStore
	knowledgeDir   string
	embedResolver  *embeddings.Resolver
	identityRes    *identity.Resolver

	// ── Pack Runtime / Plugins / Skills ────────
	packRegistry          *packruntime.Registry
	packCatalogSources    []string
	backendPacks          []packruntime.BackendModule
	backendPackRoutes     map[string]string
	backendPackRouteInfos map[string]packruntime.BackendRouteInfo
	registry              *skills.Registry
	pluginReg             *plugin.Registry
	pluginLoader          *plugin.Loader
	skillMarket           *skillmarket.Market
	skillInstaller        *skillmarket.Installer
	skillPolicy           *skillmarket.SecurityPolicy
	clawHub               *skillmarket.ClawHubProvider
	toriHub               *skillmarket.ToriHubProvider
	skillFileLoader       *skillmarket.SkillFileLoader
	skillGrow             *adapter.Detector
	skillSuggester        *memory.SkillSuggester
	suggestCounter        atomic.Int64
	pendingSuggestions    map[string][]memory.SkillSuggestion
	pendingSuggestionsMu  sync.Mutex
	plannerResumeJobs     map[string]plannerCheckpointResumePlanJob
	plannerResumeJobsMu   sync.Mutex
	plannerResumeJobsPath string

	// ── Persona & Emotion ──────────────────────
	persona          *persona.Persona
	personaChain     *persona.PriorityChain
	emotionAnalyzer  *emotion.Analyzer
	emotionHistory   *emotion.History
	stickerMap       *emotion.StickerMap
	stickerCollector *emotion.StickerCollector
	emotionShift     *planner.EmotionShiftDetector
	factHook         *planner.FactEventHook
	modeManager      *modes.ModeManager
	reverie          *planner.Reverie

	// ── Security & Auth ────────────────────────
	tenants       *tenant.Manager
	jwtCfg        *JWTConfig
	passwordStore *PasswordStore
	limiter       *RateLimiter
	zhGuard       *guardrails.Pipeline
	toolGuard     *guardrails.ToolGuard
	egressGuard   *guardrails.EgressGuard
	sanitizer     *guardrails.Sanitizer
	trustTracker  *trust.Tracker
	reviewGate    *review.Gate
	auditChain    *audit.Chain
	auditTrail    *audit.Trail

	// ── LLM Providers & Routing ────────────────
	providerReg *llm.ProviderRegistry
	modelMgr    *modelManager
	smartRouter *router.Router
	costTracker *costtrack.Tracker
	llmCall     workflow.LLMCallFunc

	// ── Tasks & Scheduling ─────────────────────
	scheduler     *scheduler.Scheduler
	cronMgr       *cron.Manager
	taskStore     task.Store
	taskRunner    *task.Runner
	gapAnalyzer   *task.GapAnalyzer
	stateKernel   *state.Kernel
	templateStore *task.TemplateStore
	workMemMgr    *task.WorkingMemoryManager
	threadMgr     *task.ThreadManager
	triggerRT     *trigger.Runtime
	triggerMgr    *trigger.Manager

	// ── Tools & Execution ──────────────────────
	toolsMgr    *tools.ProcessManager
	shellPolicy *tools.ShellExecPolicy

	// ── Channels & Integrations ────────────────
	feishuAPI  *channel.FeishuAPI
	channelReg *channel.Registry
	botMgr     *bots.Manager
	searchReg  *websearch.Registry
	searchOn   atomic.Bool
	inbox      *inbox.Store
	speechReg  *speech.Registry

	// ── Federation ─────────────────────────────
	fedHub       *federation.Hub
	fedBridge    *federation.OPPBridge
	fedTransport *federation.Transport

	// ── Observability & Self-Heal ──────────────
	metrics         *observe.Metrics
	usage           *UsageTracker
	ledgerHealth    healthChecker
	heartbeat       *heartbeat.Service
	healer          *selfheal.Healer
	lifecycle       *selfheal.Lifecycle
	skillGrowthPipe skillgrowth.GapHandler
	adaptiveLoop    *adaptive.Loop
	learning        *reflectpkg.LearningLoop
	iterateEngine   *iterate.Engine
	distiller       *distill.Distiller
	experienceStore *reflectpkg.ExperienceStore
	reflectiveLoop  *cognikernel.ReflectiveLoop
	runtimePool     *agentrt.Pool
	bindingRouter   *agentrt.Router

	allowedOrigins []string

	// Workflow Engine
	workflowStore      workflow.Store
	workflowEngine     *workflow.Engine
	workflowAPIHandler *workflowapi.Handler

	// RBAC
	rbacEnforcer   *rbac.Enforcer
	rbacMiddleware *rbac.Middleware

	// Approval (Human-in-the-Loop)
	approvalMgr *approval.Manager

	// Session Queue Manager
	queueMgr *session.QueueManager

	// SSE Event Stream
	sseBroker *SSEBroker

	// Execution event trail (unified AgentEvent audit)
	eventTrail *observe.AuditTrail

	// Last plan result cache for save_as_workflow
	lastPlanCache *sync.Map

	// Browser Extension Hub (replaces headless browser engine)
	browserHub      *BrowserHub
	browserSessions *BrowserSessionStore

	// Connector Registry (GitHub, Gmail, Calendar, etc.)
	connectorReg   *connectors.Registry
	documentParser documentParser

	notifier *notify.Notifier

	// MCP Dispatch Server (work orchestration for external workers).
	// mcpDispatchCtx is retained so late-bound dependencies (e.g. taskStore
	// injected via SetTaskStore *after* gateway.New) can be pushed into the
	// dispatch tools without recreating them — see SetTaskStore.
	mcpDispatchServer *mcpserver.Server
	workerRegistry    *mcpserver.WorkerRegistry
	mcpDispatchCtx    *mcpserver.DispatchContext

	// Project management (orchestrator)
	projectStore *orchestrator.ProjectStore
	orchDaemon   *orchestrator.Daemon
	orchLauncher *orchestrator.Launcher

	// User-defined instructions (per-tenant)
	instructionStore *instruction.Store

	// Output directory for agent-generated files (file_write, code_exec outputs)
	outputDir string

	// Update checker callback (set externally)
	updateChecker func() (tagName, htmlURL string, hasNew bool)

	// Tori OAuth2 token store (set externally)
	toriTokenStore *tori.TokenStore

	// Cloud sandbox runner (E2B Desktop)
	cloudRunner    *sandbox.CloudRunner
	desktopSandbox *sandbox.DesktopSandbox
	desktopMu      sync.Mutex

	// OAuth login state (Tori PKCE)
	oauthPending map[string]*oauthPendingState
	oauthStateMu sync.Mutex

	// Exec provider override (cognitive/execution separation)
	execProvider   string
	execProviderMu sync.RWMutex

	mux       *http.ServeMux
	reqCount  atomic.Int64
	startTime time.Time
	routesMu  sync.RWMutex

	// dynamicRoutes holds runtime-mounted pack routes (wasm-backed), keyed by
	// path and guarded by routesMu. Unlike mux, entries can be removed.
	dynamicRoutes       map[string]*dynRoute
	wasmWired           bool
	wasmSandboxOnce     sync.Once
	wasmSandboxInstance *sandbox.WasmSandbox
	// packTrustRoot resolves publisher keys when installing signed .yqpacks.
	// nil means signed packs fail closed (only unsigned dev packs install).
	packTrustRoot packruntime.PublicKeyResolver

	replyHooks   []ReplyHook
	replyHooksMu sync.RWMutex

	modules *agentrt.ModuleRegistry
	profile string

	// Cogni hot-pluggable registry (declarative AI-cognition shells).
	// Populated by the cogni runtime module after Gateway construction.
	cogniKernelRuntimeState http.HandlerFunc
	cogniRegistry           *cogni.Registry
	cogniDir                string
	cogniTraces             cogni.TraceStore
	cogniSentinel           *cogni.Sentinel
	cogniWorkflowEngine     *cogni.WorkflowEngine
	cogniExperiences        map[string]*cogni.ExperienceStore
	cogniGenesis            *cogni.Genesis
	cogniEvolution          *cogni.EvolutionEngine
	cogniFederation         *cogni.CogniFederation
	cogniCostTracker        *cogni.CostTracker

	// NL Config — natural-language → structured-config translator
	nlConfigTranslator *cogni.NLConfigTranslator

	// LoRA training & evolution (optional — wired from app in init_tasks)
	loraScheduler        *localbrain.LoRAScheduler
	trainingMetrics      *localbrain.TrainingMetrics
	evolutionCoordinator *localbrain.EvolutionCoordinator
}

// ReplyHook interceptor for outgoing messages.
type ReplyHook func(ctx context.Context, msg channel.Message, reply channel.Reply)

// AddReplyHook registers a global interceptor for outgoing channel replies.
func (g *Gateway) AddReplyHook(h ReplyHook) {
	g.replyHooksMu.Lock()
	defer g.replyHooksMu.Unlock()
	g.replyHooks = append(g.replyHooks, h)
}

// InvokeReplyHooks triggers all registered hooks asynchronously with a
// 10-second timeout to prevent goroutine leaks from blocking hooks.
func (g *Gateway) InvokeReplyHooks(ctx context.Context, msg channel.Message, reply channel.Reply) {
	g.replyHooksMu.RLock()
	hooks := make([]ReplyHook, len(g.replyHooks))
	copy(hooks, g.replyHooks)
	g.replyHooksMu.RUnlock()

	for _, h := range hooks {
		h := h
		go func() {
			hookCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			h(hookCtx, msg, reply)
		}()
	}
}

// GatewayConfig holds the required dependencies for creating a Gateway.
type GatewayConfig struct {
	Planner   *planner.Planner
	Tenants   *tenant.Manager
	Memory    *memory.Manager
	Skills    *skills.Registry
	Scheduler *scheduler.Scheduler
	ConvStore *session.Store
	Plugins   *plugin.Registry
	FeishuAPI *channel.FeishuAPI
	Learning  *reflectpkg.LearningLoop
	JWTConfig *JWTConfig
	Metrics   *observe.Metrics
	Pipeline  *memory.Pipeline
	Persona   *persona.Persona
	Packs     *packruntime.Registry

	// BackendPacks are optional capability-pack HTTP modules mounted by the
	// Pack Runtime host. If omitted, Gateway mounts the built-in pack modules.
	BackendPacks []packruntime.BackendModule
}

// New creates a new Gateway.
//
// Deprecated: Use NewFromConfig for new code. This function is kept for
// backward compatibility and delegates to NewFromConfig internally.
func New(p *planner.Planner, t *tenant.Manager, m *memory.Manager, r *skills.Registry, s *scheduler.Scheduler, cs *session.Store, pr *plugin.Registry, fa *channel.FeishuAPI, ll *reflectpkg.LearningLoop, jwtCfg *JWTConfig, met *observe.Metrics, pipeline *memory.Pipeline, per *persona.Persona) *Gateway {
	return NewFromConfig(GatewayConfig{
		Planner: p, Tenants: t, Memory: m, Skills: r, Scheduler: s,
		ConvStore: cs, Plugins: pr, FeishuAPI: fa, Learning: ll,
		JWTConfig: jwtCfg, Metrics: met, Pipeline: pipeline, Persona: per,
	})
}

// NewFromConfig creates a new Gateway from a config struct.
func NewFromConfig(cfg GatewayConfig) *Gateway {
	met := cfg.Metrics
	if met == nil {
		met = observe.New()
	}
	g := &Gateway{
		planner:               cfg.Planner,
		tenants:               cfg.Tenants,
		memory:                cfg.Memory,
		registry:              cfg.Skills,
		scheduler:             cfg.Scheduler,
		convStore:             cfg.ConvStore,
		pluginReg:             cfg.Plugins,
		feishuAPI:             cfg.FeishuAPI,
		learning:              cfg.Learning,
		jwtCfg:                cfg.JWTConfig,
		metrics:               met,
		pipeline:              cfg.Pipeline,
		persona:               cfg.Persona,
		packRegistry:          cfg.Packs,
		backendPacks:          cfg.BackendPacks,
		backendPackRoutes:     make(map[string]string),
		backendPackRouteInfos: make(map[string]packruntime.BackendRouteInfo),
		limiter:               NewRateLimiter(30, time.Minute),
		usage:                 NewUsageTracker(),
		mux:                   http.NewServeMux(),
		startTime:             time.Now(),
		browserSessions:       NewBrowserSessionStore(),
		modelMgr:              newModelManager(),
		oauthPending:          make(map[string]*oauthPendingState),
	}
	g.searchOn.Store(true)
	g.routes()
	g.wireWasmPacks() // no-op unless cfg.Packs was provided (e.g. tests)
	return g
}

// SetRateLimit reconfigures the rate limiter.
func (g *Gateway) SetRateLimit(maxRequests int, window time.Duration) {
	g.limiter = NewRateLimiter(maxRequests, window)
}

// MountPluginAPIRoutes registers all /v1/plugin-api/* routes from a PluginAPIHandler.
// These routes are used by the Go/Python SDKs for plugin ↔ agent communication.
// SetPasswordStore attaches the admin password store.
func (g *Gateway) SetPasswordStore(ps *PasswordStore) {
	g.passwordStore = ps
	// Register auth routes (no auth required for these)
	g.mux.HandleFunc("/v1/auth/login", g.handleAuthLogin)
	g.mux.HandleFunc("/v1/auth/status", g.handleAuthStatus)
	g.mux.HandleFunc("/v1/auth/set-password", g.handleAuthSetPassword)
	g.mux.HandleFunc("/v1/auth/oauth/tori", g.handleOAuthToriStart)
	g.mux.HandleFunc("/v1/auth/oauth/tori/callback", g.handleOAuthToriCallback)
}

func (g *Gateway) MountPluginAPIRoutes(handler *PluginAPIHandler) {
	handler.RegisterRoutes(g.mux)
	slog.Info("plugin API routes mounted", "prefix", "/v1/plugin-api/")
}

// MountPluginRoutes discovers all UIPlugin HTTP handlers from the plugin registry
// and mounts them on the mux under /v1/ext/{plugin-key}/{path}.
// Call this after all plugins are registered and before ListenAndServe.
func (g *Gateway) MountPluginRoutes() {
	handlers := g.pluginReg.AllHTTPHandlers()
	for path, handler := range handlers {
		slog.Info("mounted plugin route", "path", path)
		if strings.HasPrefix(path, "/v1/ext/airi/") {
			// Airi acts as an external OpenAI client and does not use Yunque JWTs
			g.mux.HandleFunc(path, handler)
		} else {
			g.mux.HandleFunc(path, g.requireAuth(handler))
		}
	}
	if len(handlers) > 0 {
		slog.Info("plugin routes mounted", "count", len(handlers))
	}
}

// SetBrowserHub attaches the BrowserHub for browser extension WebSocket
// and registers browser skills (navigate, click, input, etc.) into the
// planner's skill registry so the LLM can invoke them via function calling.
func (g *Gateway) SetBrowserHub(hub *BrowserHub) {
	g.browserHub = hub
	g.mux.HandleFunc("/ws/browser", g.handleBrowserWS)
	if g.registry != nil {
		browserskill.RegisterSkills(g.registry, &browserHubAdapter{hub: hub})
		browserSkillNames := []string{
			"browser_navigate", "browser_click", "browser_input",
			"browser_screenshot", "browser_scroll", "browser_get_content",
			"browser_press_key", "browser_mark_elements", "browser_unmark_elements",
			"browser_get_elements", "browser_list_tabs", "browser_switch_tab",
			"browser_new_tab", "browser_close_tab", "browser_takeover",
		}
		g.registry.DefineCategory(skills.SkillCategory{
			ID:          "browser",
			Name:        "浏览器",
			Description: "Control the user's real browser via the Yunque Browser Connector extension. Pass 'action' with the browser skill name (e.g. 'browser_navigate') and 'args' with its parameters. Use these tools when the user asks to open a website, search the web, click elements, fill forms, or take screenshots.",
			SkillNames:  browserSkillNames,
		})
	}
	slog.Info("browser extension WebSocket endpoint registered", "path", "/ws/browser")
	slog.Info("browser extension hub initialized")
}

// BrowserHub returns the BrowserHub instance.
func (g *Gateway) BrowserHub() *BrowserHub {
	return g.browserHub
}

type browserHubAdapter struct{ hub *BrowserHub }

func (a *browserHubAdapter) Connected() bool { return a.hub.Connected() }

func (a *browserHubAdapter) SendAction(ctx context.Context, action any) (any, error) {
	raw, err := json.Marshal(action)
	if err != nil {
		return nil, err
	}
	result, err := a.hub.SendActionRaw(ctx, raw)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	_ = json.Unmarshal(result, &out)
	return out, nil
}

// SetAllowedOrigins configures CORS allowed origins. Use "*" for wildcard (dev only).
func (g *Gateway) SetAllowedOrigins(origins []string) {
	g.allowedOrigins = origins
}

func (g *Gateway) SetOutputDir(dir string) {
	g.outputDir = dir
}

// checkWSOrigin validates WebSocket upgrade origins against allowedOrigins
// and localhost addresses. Used by all WS endpoints except browser extension
// which has its own origin checker (allowBrowserWSOrigin).
func (g *Gateway) checkWSOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		slog.Debug("ws: empty Origin header, allowing (non-browser client)", "path", r.URL.Path)
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	host := u.Hostname()
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return true
	}
	if u.Scheme == "chrome-extension" {
		return true
	}
	for _, o := range g.allowedOrigins {
		if o == "*" {
			return true
		}
		if o == origin {
			return true
		}
	}
	return false
}

// corsOrigin returns the value to echo in Access-Control-Allow-Origin for the
// supplied request. An empty allowed-origins list used to default to "*",
// which is a footgun as soon as the operator adds Cookie-based auth. We now
// fall back to refusing cross-origin requests (empty string → no CORS header)
// when no origins are configured, matching "same-origin only" semantics.
// Operators who explicitly want the old permissive behaviour can set
// ALLOWED_ORIGINS=* to opt in.
func (g *Gateway) corsOrigin(origin string) string {
	if len(g.allowedOrigins) == 0 {
		return ""
	}
	for _, o := range g.allowedOrigins {
		if o == "*" {
			return "*"
		}
		if o == origin {
			return origin
		}
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

// Hijack implements http.Hijacker so WebSocket upgrades work through the wrapper.
func (sw *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := sw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("response does not implement http.Hijacker")
	}
	return h.Hijack()
}

// ServeHTTP implements http.Handler with a composable middleware chain.
// Individual middleware are defined in middleware.go.
func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.buildMiddlewareChain(g.dynamicDispatch(g.mux)).ServeHTTP(w, r)
}

func (g *Gateway) routes() {
	// Root "/" serves the embedded Next.js static UI (SPA).
	// Falls back to pure HTML dashboard if no frontend build is available.
	// Specific API routes below take priority over this catch-all.
	g.mux.HandleFunc("/", g.serveWebUI)

	// Domain-specific route groups
	g.registerSystemRoutes()       // healthz, version, tenants, metrics, settings, backup, speech, heartbeat, federation
	g.registerChatRoutes()         // chat, ws, conversations, persona, emotion, bots, inbox, webhooks, webchat
	g.registerMemoryRoutes()       // memory, graph, identity, embeddings, search
	g.registerKnowledgeRoutes()    // knowledge base (RAG)
	g.registerTaskRoutes()         // tasks, state kernel, reflection, documents
	g.registerTriggerRoutes()      // triggers, cron, scheduler, tools, sandbox
	g.registerPackRoutes()         // packs registry, frontend sync, enable/disable/rollback
	g.registerPluginRoutes()       // plugins, skills, skill market, skillhub
	g.registerGovernanceRoutes()   // audit, trust, iterate, review, cost, usage
	g.registerProviderRoutes()     // LLM providers, router stats
	g.registerReverieRoutes()      // reverie inner monologue
	g.registerRBACRoutes()         // role-based access control
	g.registerApprovalRoutes()     // human-in-the-loop approval
	g.registerSetupRoutes()        // setup, onboarding, templates
	g.registerQueueRoutes()        // session task queues
	g.registerSSERoutes()          // SSE event stream
	g.registerTraceRoutes()        // execution trace / audit API
	g.registerBrowserRoutes()      // browser engine management
	g.registerIDERoutes()          // IDE supervisor plugin (review, status)
	g.registerModesRoutes()        // persona mode management (/v1/persona/modes, mode, mode/current)
	g.registerMCPDispatchRoutes()  // MCP dispatch server for external workers
	g.registerProjectRoutes()      // project management (orchestrator)
	g.registerOrchestratorRoutes() // orchestrator daemon control

	// Extracted handler groups (sub-packages)
	(&costapi.Handler{
		Tracker: g.costTracker,
	}).RegisterRoutes(g.mux, g.requireAuth)

	(&connectorapi.Handler{
		Registry: g.connectorReg,
	}).RegisterRoutes(g.mux, g.requireAuth)

	(&notifyapi.Handler{
		NotifierFunc: func() *notify.Notifier {
			return g.notifier
		},
	}).RegisterRoutes(g.mux, g.requireAuth)

	g.workflowAPIHandler = &workflowapi.Handler{
		Store:   g.workflowStore,
		Engine:  g.workflowEngine,
		LLMCall: g.llmCall,
	}
	g.workflowAPIHandler.RegisterRoutes(g.mux, g.requireAuth)

	(&schedulerapi.Handler{
		Scheduler: g.scheduler,
	}).RegisterRoutes(g.mux, g.requireAuth)

	(&forkapi.Handler{
		ForkTree:  g.forkTree,
		Persister: g.forkPersister,
	}).RegisterRoutes(g.mux, g.requireAuth)
}

// --- Auth middleware ---

const ctxTenantKey ctxKeyType = "tenant_id"

func (g *Gateway) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := authTokenFromHeaders(r)

		// Localhost bypass is disabled — all access requires authentication.
		// Set LOCALHOST_BYPASS=true in .env to re-enable for development.

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
	return gwshared.ContextWithTenant(ctx, id)
}

func tenantFromCtx(ctx context.Context) string {
	return gwshared.TenantFromCtx(ctx)
}

// requireSetupOrAuth allows unauthenticated access only during true first-run
// setup: when both the admin password is NOT yet set AND the LLM configuration
// is incomplete. Once the admin password has been set, all setup endpoints
// require authentication — even if the LLM config is still missing.
func (g *Gateway) requireSetupOrAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		passwordSet := g.passwordStore != nil && g.passwordStore.IsSetup()
		values := readEnvFile()
		llmConfigured := values["LLM_BASE_URL"] != "" && values["LLM_MODEL"] != ""

		if !passwordSet && !llmConfigured {
			ctx := contextWithTenant(r.Context(), "setup")
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}
		g.requireAuth(next).ServeHTTP(w, r)
	}
}

// requireAdmin wraps a handler to ensure the caller has the "admin" role.
func (g *Gateway) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		role := roleFromCtx(r.Context())
		if role != "admin" {
			apperror.WriteCode(w, apperror.CodeForbidden, "admin role required")
			return
		}
		next.ServeHTTP(w, r)
	}
}
