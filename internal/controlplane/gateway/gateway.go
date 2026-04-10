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
	"yunque-agent/internal/agentcore/browserskill"
	"yunque-agent/internal/agentcore/approval"
	"yunque-agent/internal/agentcore/audit"
	"yunque-agent/internal/agentcore/bots"
	"yunque-agent/internal/agentcore/costtrack"
	"yunque-agent/internal/agentcore/cron"
	"yunque-agent/internal/agentcore/distill"
	"yunque-agent/internal/agentcore/rbac"
	"yunque-agent/internal/agentcore/review"
	"yunque-agent/internal/agentcore/skillgrow"
	"yunque-agent/internal/agentcore/tools"
	"yunque-agent/internal/agentcore/trust"
	"yunque-agent/internal/agentcore/workflow"

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
	"yunque-agent/internal/agentcore/modes"
	"yunque-agent/internal/agentcore/notify"
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
	"yunque-agent/internal/connectors"
	"yunque-agent/internal/controlplane/tenant"
	"yunque-agent/internal/execution/channel"
	"yunque-agent/internal/execution/scheduler"
	"yunque-agent/internal/integrations/mineru"
	"yunque-agent/internal/observe"
	"yunque-agent/internal/tori"
	"yunque-agent/pkg/plugin"
	"yunque-agent/pkg/skills"
)

type ctxKeyType string

const ctxKeyReqID ctxKeyType = "req_id"

type documentParser interface {
	Enabled() bool
	ParseFile(ctx context.Context, filePath string) (*mineru.ParseResult, error)
}

// RequestID extracts the request ID from context.
func RequestID(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyReqID).(string); ok {
		return v
	}
	return ""
}

// Gateway is the HTTP API server for the agent.
type Gateway struct {
	planner              *planner.Planner
	tenants              *tenant.Manager
	memory               *memory.Manager
	registry             *skills.Registry
	scheduler            *scheduler.Scheduler
	convStore            *session.Store
	pluginReg            *plugin.Registry
	feishuAPI            *channel.FeishuAPI
	learning             *reflectpkg.LearningLoop
	limiter              *RateLimiter
	jwtCfg               *JWTConfig
	passwordStore        *PasswordStore
	usage                *UsageTracker
	metrics              *observe.Metrics
	pipeline             *memory.Pipeline
	persona              *persona.Persona
	personaChain         *persona.PriorityChain
	heartbeat            *heartbeat.Service
	inbox                *inbox.Store
	botMgr               *bots.Manager
	searchReg            *websearch.Registry
	smartRouter          *router.Router
	identityRes          *identity.Resolver
	healer               *selfheal.Healer
	lifecycle            *selfheal.Lifecycle
	costTracker          *costtrack.Tracker
	forkTree             *session.ForkTree
	forkPersister        *session.ForkPersister
	embedResolver        *embeddings.Resolver
	subagentMgr          *subagent.Manager
	handoffReg           *subagent.HandoffRegistry
	orchestrator         *memory.Orchestrator
	zhGuard              *guardrails.Pipeline
	adaptiveLoop         *adaptive.Loop
	auditChain           *audit.Chain
	skillMarket          *skillmarket.Market
	fedHub               *federation.Hub
	fedBridge            *federation.OPPBridge // OPP v3 bridge (model-aware federation)
	fedTransport         *federation.Transport // federation HTTP transport
	knowledgeStore       *knowledge.Store
	cronMgr              *cron.Manager
	toolsMgr             *tools.ProcessManager
	shellPolicy          *tools.ShellExecPolicy
	runtimePool          *agentrt.Pool
	bindingRouter        *agentrt.Router
	toolGuard            *guardrails.ToolGuard
	egressGuard          *guardrails.EgressGuard
	skillInstaller       *skillmarket.Installer
	skillPolicy          *skillmarket.SecurityPolicy
	clawHub              *skillmarket.ClawHubProvider
	toriHub              *skillmarket.ToriHubProvider
	pluginLoader         *plugin.Loader
	iterateEngine        *iterate.Engine
	trustTracker         *trust.Tracker
	distiller            *distill.Distiller
	reviewGate           *review.Gate
	skillGrow            *skillgrow.Detector
	auditTrail           *audit.Trail
	providerReg          *llm.ProviderRegistry
	speechReg            *speech.Registry
	emotionAnalyzer      *emotion.Analyzer
	emotionHistory       *emotion.History
	stickerMap           *emotion.StickerMap
	stickerCollector     *emotion.StickerCollector
	channelReg           *channel.Registry
	emotionShift         *planner.EmotionShiftDetector // event-driven Reverie trigger
	factHook             *planner.FactEventHook        // event-driven Reverie trigger on high-value facts
	skillSuggester       *memory.SkillSuggester        // auto skill suggestion from conversations
	pendingSuggestions   map[string][]memory.SkillSuggestion
	pendingSuggestionsMu sync.Mutex
	modeManager          *modes.ModeManager          // persona mode management
	reverie              *planner.Reverie            // Reverie inner monologue system (for API access)
	taskStore            task.Store                  // task runtime persistence
	taskRunner           *task.Runner                // task execution engine
	gapAnalyzer          *task.GapAnalyzer           // capability gap detection
	stateKernel          *state.Kernel               // structured state kernel
	experienceStore      *reflectpkg.ExperienceStore // reflection experience store
	templateStore        *task.TemplateStore         // task template store
	workMemMgr           *task.WorkingMemoryManager  // task working memory
	threadMgr            *task.ThreadManager         // task thread manager
	triggerRT            *trigger.Runtime            // trigger runtime (legacy)
	triggerMgr           *trigger.Manager            // unified trigger manager
	preAckEmojis         []string                    // emoji list for pre-ack reactions (e.g., ["👍","🤔","💡"])
	allowedOrigins       []string

	// Workflow Engine
	workflowStore  workflow.Store
	workflowEngine *workflow.Engine

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

	// Output directory for agent-generated files (file_write, code_exec outputs)
	outputDir string

	// Update checker callback (set externally)
	updateChecker func() (tagName, htmlURL string, hasNew bool)

	// Tori OAuth2 token store (set externally)
	toriTokenStore *tori.TokenStore

	mux       *http.ServeMux
	reqCount  atomic.Int64
	startTime time.Time

	replyHooks   []ReplyHook
	replyHooksMu sync.RWMutex
}

// ReplyHook interceptor for outgoing messages.
type ReplyHook func(ctx context.Context, msg channel.Message, reply channel.Reply)

// AddReplyHook registers a global interceptor for outgoing channel replies.
func (g *Gateway) AddReplyHook(h ReplyHook) {
	g.replyHooksMu.Lock()
	defer g.replyHooksMu.Unlock()
	g.replyHooks = append(g.replyHooks, h)
}

// InvokeReplyHooks triggers all registered hooks asynchronously.
func (g *Gateway) InvokeReplyHooks(ctx context.Context, msg channel.Message, reply channel.Reply) {
	g.replyHooksMu.RLock()
	hooks := make([]ReplyHook, len(g.replyHooks))
	copy(hooks, g.replyHooks)
	g.replyHooksMu.RUnlock()

	for _, h := range hooks {
		go h(ctx, msg, reply)
	}
}

// New creates a new Gateway.
func New(p *planner.Planner, t *tenant.Manager, m *memory.Manager, r *skills.Registry, s *scheduler.Scheduler, cs *session.Store, pr *plugin.Registry, fa *channel.FeishuAPI, ll *reflectpkg.LearningLoop, jwtCfg *JWTConfig, met *observe.Metrics, pipeline *memory.Pipeline, per *persona.Persona) *Gateway {
	if met == nil {
		met = observe.New()
	}
	g := &Gateway{planner: p, tenants: t, memory: m, registry: r, scheduler: s, convStore: cs, pluginReg: pr, feishuAPI: fa, learning: ll, jwtCfg: jwtCfg, limiter: NewRateLimiter(30, time.Minute), usage: NewUsageTracker(), metrics: met, pipeline: pipeline, persona: per, mux: http.NewServeMux(), startTime: time.Now(), browserSessions: NewBrowserSessionStore()}
	g.routes()
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

// Hijack implements http.Hijacker so WebSocket upgrades work through the wrapper.
func (sw *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := sw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("response does not implement http.Hijacker")
	}
	return h.Hijack()
}

// ServeHTTP implements http.Handler with request logging.
func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	count := g.reqCount.Add(1)
	reqID := fmt.Sprintf("%d-%d", start.UnixMilli(), count)

	origin := r.Header.Get("Origin")
	allowed := g.corsOrigin(origin)
	if allowed != "" {
		w.Header().Set("Access-Control-Allow-Origin", allowed)
		if allowed != "*" {
			w.Header().Set("Vary", "Origin")
		}
	}
	w.Header().Set("Access-Control-Allow-Methods", "GET,POST,DELETE,OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization,Content-Type,X-API-Key")
	w.Header().Set("Access-Control-Max-Age", "86400")
	w.Header().Set("X-Request-ID", reqID)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
	w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; connect-src 'self' ws: wss:; font-src 'self' data:; frame-ancestors 'none'")
	if r.Header.Get("X-Forwarded-Proto") == "https" || r.TLS != nil {
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
	}
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

	// Block all authenticated API access until admin password is set (first-run enforcement).
	// Exempt paths: health, version, auth endpoints, setup, static assets.
	if g.passwordStore != nil && !g.passwordStore.IsSetup() {
		p := r.URL.Path
		exempt := p == "/healthz" || p == "/v1/version" || p == "/" ||
			strings.HasPrefix(p, "/v1/auth/") ||
			strings.HasPrefix(p, "/v1/setup/") ||
			strings.HasPrefix(p, "/api/settings/check") ||
			strings.HasPrefix(p, "/_next/") ||
			!strings.HasPrefix(p, "/v1/") && !strings.HasPrefix(p, "/api/")
		if !exempt {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"error":"password_required","message":"Admin password must be set before using the API. POST /v1/auth/set-password"}`))
			return
		}
	}

	// Global rate limiting for mutating requests (POST, DELETE, PUT, PATCH)
	// GET/OPTIONS/HEAD and health endpoints are exempt
	if r.Method != "GET" && r.Method != "OPTIONS" && r.Method != "HEAD" &&
		r.URL.Path != "/healthz" && r.URL.Path != "/v1/version" {
		key := tenantFromCtx(ctx)
		if key == "" {
			// For unauthenticated requests, use IP-based limiting
			key = "ip:" + r.RemoteAddr
		}
		if !g.limiter.Allow(key) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"rate limit exceeded","retry_after":60}`))
			slog.Warn("rate limited", "path", r.URL.Path, "key", key, "req_id", reqID)
			return
		}
	}

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

	// Domain-specific route groups
	g.registerSystemRoutes()     // healthz, version, tenants, metrics, settings, backup, speech, heartbeat, federation
	g.registerChatRoutes()       // chat, ws, conversations, persona, emotion, bots, inbox, webhooks, webchat
	g.registerMemoryRoutes()     // memory, graph, identity, embeddings, search
	g.registerKnowledgeRoutes()  // knowledge base (RAG)
	g.registerTaskRoutes()       // tasks, state kernel, reflection, documents
	g.registerTriggerRoutes()    // triggers, cron, scheduler, tools, sandbox
	g.registerPluginRoutes()     // plugins, skills, skill market, skillhub
	g.registerGovernanceRoutes() // audit, trust, iterate, review, cost, usage
	g.registerProviderRoutes()   // LLM providers, router stats
	g.registerReverieRoutes()    // reverie inner monologue
	g.registerWorkflowRoutes()   // workflow engine (DAG)
	g.registerRBACRoutes()       // role-based access control
	g.registerApprovalRoutes()   // human-in-the-loop approval
	g.registerSetupRoutes()      // setup, onboarding, templates
	g.registerQueueRoutes()      // session task queues
	g.registerSSERoutes()        // SSE event stream
	g.registerTraceRoutes()      // execution trace / audit API
	g.registerBrowserRoutes()    // browser engine management
	g.registerConnectorRoutes()  // connectors (GitHub, Gmail, Calendar, etc.)
	g.registerNotifyRoutes()     // notification channels (webhook, DingTalk, Feishu, etc.)
	g.registerIDERoutes()        // IDE supervisor plugin (review, status)
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
	return context.WithValue(ctx, ctxTenantKey, id)
}

func tenantFromCtx(ctx context.Context) string {
	v, _ := ctx.Value(ctxTenantKey).(string)
	return v
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
