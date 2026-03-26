package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"yunque-agent/internal/agentcore/approval"
	"yunque-agent/internal/agentcore/adaptive"
	"yunque-agent/internal/agentcore/audit"
	"yunque-agent/internal/agentcore/bots"
	"yunque-agent/internal/agentcore/costtrack"
	"yunque-agent/internal/agentcore/cron"
	"yunque-agent/internal/agentcore/distill"
	"yunque-agent/internal/agentcore/rbac"
	"yunque-agent/internal/agentcore/review"
	"yunque-agent/internal/agentcore/skillgrow"
	"yunque-agent/internal/agentcore/tools"
	"yunque-agent/internal/agentcore/workflow"
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
	"yunque-agent/internal/execution/browser"
	"yunque-agent/internal/execution/channel"
	"yunque-agent/internal/execution/scheduler"
	"yunque-agent/internal/observe"
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
	passwordStore   *PasswordStore
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
	stickerMap       *emotion.StickerMap
	stickerCollector *emotion.StickerCollector
	channelReg       *channel.Registry
	emotionShift    *planner.EmotionShiftDetector // event-driven Reverie trigger
	factHook        *planner.FactEventHook        // event-driven Reverie trigger on high-value facts
	reverie         *planner.Reverie              // Reverie inner monologue system (for API access)
	taskStore       task.Store                    // task runtime persistence
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

	// Workflow Engine
	workflowStore  workflow.Store
	workflowEngine *workflow.Engine

	// RBAC
	rbacEnforcer   *rbac.Enforcer
	rbacMiddleware *rbac.Middleware

	// Approval (Human-in-the-Loop)
	approvalMgr *approval.Manager

	// SSE Event Stream
	sseBroker *SSEBroker

	// Execution event trail (unified AgentEvent audit)
	eventTrail *observe.AuditTrail

	// Last plan result cache for save_as_workflow
	lastPlanCache *sync.Map

	// Browser Engine
	browserEngine     *browser.Engine
	browserRecognizer *browser.Recognizer
	browserWorker     *browser.Worker
	browserNotifier   *browser.Notifier
	browserHeadless   bool
	browserDataDir    string

	mux             *http.ServeMux
	reqCount        atomic.Int64
	startTime       time.Time

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

// SetStickerCollector attaches a sticker collector for interactive sticker learning.
func (g *Gateway) SetStickerCollector(sc *emotion.StickerCollector) { g.stickerCollector = sc }

// WireStickerCommands connects the sticker command system to the channel registry.
// This creates a CommandInterceptor with sticker commands (/add, /add-all, /sticker,
// /sticker-del, /cancel) and attaches it to the channel registry so all IM channels
// get universal sticker command support.
// Call this after SetStickerMap, SetStickerCollector, and SetChannelRegistry.
func (g *Gateway) WireStickerCommands() {
	if g.channelReg == nil {
		return
	}

	ci := channel.NewCommandInterceptor()
	sc := &channel.StickerCommands{}

	if g.stickerCollector != nil {
		sc.StartCollect = func(channelType, userID, emotionStr string) string {
			em, ok := emotion.ParseStickerCommand("/sticker " + emotionStr)
			if !ok {
				em = emotion.EmotionHappy
			}
			return g.stickerCollector.StartSession(channelType, userID, em)
		}
		sc.StartBulkAdd = func(channelType, userID string) string {
			return g.stickerCollector.StartAddSession(channelType, userID)
		}
		sc.ListStickers = func(platform string) string {
			return g.stickerCollector.ListStickers(platform)
		}
		sc.CancelSession = func(channelType, userID string) bool {
			return g.stickerCollector.CancelSession(channelType, userID)
		}
	}

	if g.stickerMap != nil {
		sc.DeleteStickers = func(platform, emotionStr string) string {
			em := emotion.Emotion(emotionStr)
			g.stickerMap.Clear(platform, em)
			return fmt.Sprintf("✅ 已删除 %s 平台的「%s」情绪贴图", platform, emotionStr)
		}
	}

	// Fetch-and-learn for channels that support StickerSetFetcher
	if g.channelReg != nil && g.stickerCollector != nil {
		sc.FetchAndLearnSet = func(channelType, setName string) (string, error) {
			ch, ok := g.channelReg.Get(channelType)
			if !ok {
				return "", fmt.Errorf("channel %s not found", channelType)
			}
			fetcher, ok := ch.(channel.StickerSetFetcher)
			if !ok {
				return "", fmt.Errorf("channel %s does not support fetching sticker sets", channelType)
			}
			stickers, err := fetcher.FetchStickerSet(setName)
			if err != nil {
				return "", err
			}
			result := g.stickerCollector.LearnStickerSet(channelType, stickers)
			return result, nil
		}
	}

	ci.Register(sc.Handler())
	g.channelReg.SetCommandInterceptor(ci)
}

// WireStickerEnricher sets up automatic sticker sending based on detected emotion.
// After the planner replies, if the incoming message carries a recognized emotion
// above the confidence threshold, a sticker is appended to the reply.
// Call this after SetStickerMap, SetEmotionAnalyzer, and SetChannelRegistry.
func (g *Gateway) WireStickerEnricher() {
	if g.channelReg == nil || g.stickerMap == nil || g.emotionAnalyzer == nil {
		return
	}

	enricher := &channel.StickerEnricher{
		MinConfidence: 0.5,
		AnalyzeEmotion: func(text string) (string, float64) {
			if !g.emotionAnalyzer.Enabled() {
				return "", 0
			}
			featureOK := g.personaChain == nil || g.personaChain.FeatureEnabled(persona.FeatureEmotion)
			if !featureOK {
				return "", 0
			}
			res, err := g.emotionAnalyzer.AnalyzeText(context.Background(), text)
			if err != nil || res == nil {
				return "", 0
			}
			return string(res.Emotion), res.Confidence
		},
		SuggestSticker: func(emo, platform string) *channel.StickerComponent {
			stickerFeatureOK := g.personaChain == nil || g.personaChain.FeatureEnabled(persona.FeatureSticker)
			if !stickerFeatureOK {
				return nil
			}
			s := g.stickerMap.Suggest(emotion.Emotion(emo), platform)
			if s == nil {
				return nil
			}
			sc := channel.NewSticker(s.PackageID, s.StickerID)
			sc.Platform = s.Platform
			sc.FileID = s.FileID
			sc.SetName = s.SetName
			sc.Emoji = s.Emoji
			if s.CDNURL != "" {
				sc.URL = s.CDNURL
			}
			return sc
		},
		SendProbability: func() float64 {
			freq := 2.0
			if g.personaChain != nil {
				freq = g.personaChain.FloatFeature(persona.FeatureStickerFrequency, 2)
			}
			return stickerSendProb(freq)
		},
	}

	// Override MinConfidence from persona if available
	if g.personaChain != nil {
		minConf := g.personaChain.FloatFeature(persona.FeatureEmotionMinConfidence, 0.5)
		enricher.MinConfidence = minConf
	}

	g.channelReg.SetStickerEnricher(enricher)
}

// SetEmotionShiftDetector attaches the emotion shift detector for Reverie event-driven triggers.
func (g *Gateway) SetEmotionShiftDetector(d *planner.EmotionShiftDetector) { g.emotionShift = d }

// SetFactEventHook attaches the fact event hook for Reverie event-driven triggers.
func (g *Gateway) SetFactEventHook(h *planner.FactEventHook) { g.factHook = h }

// SetReverie attaches the Reverie system for API access.
func (g *Gateway) SetReverie(r *planner.Reverie) { g.reverie = r }

// WireReverieActions connects Reverie's action callbacks to the actual subsystems:
//   - write_memory → Memory Orchestrator (ingest as "reverie_insight")
//   - create_task → Task Runner (create a new task)
//   - update_profile → Persona identity (update user profile key-value)
//
// This enables the "reverie → memory → reflection" feedback loop:
// Reverie thinks → writes to memory → memory informs future conversations
// → reflection learns from outcomes → strategies guide Reverie.
// Call this after SetReverie, SetOrchestrator, SetTaskRunner.
func (g *Gateway) WireReverieActions() {
	if g.reverie == nil {
		return
	}

	// write_memory → orchestrator
	if g.orchestrator != nil {
		g.reverie.SetWriteMemory(func(ctx context.Context, fact string) error {
			return g.orchestrator.Ingest(ctx, "default", fact, "reverie_insight", "reverie")
		})
	}

	// create_task → task store
	if g.taskStore != nil {
		g.reverie.SetCreateTask(func(ctx context.Context, title, desc string) error {
			_, err := g.taskStore.Create(task.CreateRequest{
				Title:       title,
				Description: desc,
				TenantID:    "default",
			})
			return err
		})
	}

	// update_profile → memory as persistent profile fact
	if g.orchestrator != nil {
		g.reverie.SetUpdateProfile(func(ctx context.Context, key, value string) error {
			fact := fmt.Sprintf("[用户画像] %s: %s", key, value)
			return g.orchestrator.Ingest(ctx, "default", fact, "profile_update", "reverie")
		})
	}
}

// WireReflectionLoop connects the reflection experience store to the planner
// so that compiled strategies are injected into conversation context.
// This closes the feedback loop: tasks run → experiences recorded → strategies compiled
// → strategies guide future conversations → better outcomes → better experiences.
func (g *Gateway) WireReflectionLoop() {
	if g.experienceStore == nil || g.planner == nil {
		return
	}
	g.planner.SetStrategyContext(func() string {
		return g.experienceStore.CompileStrategies(20)
	})
}

// SetTaskStore attaches the task persistence store.
func (g *Gateway) SetTaskStore(s task.Store) { g.taskStore = s }

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

// SetSSEBroker attaches the SSE event stream broker.
func (g *Gateway) SetSSEBroker(b *SSEBroker) { g.sseBroker = b }

// SetEventTrail attaches the unified event audit trail.
func (g *Gateway) SetEventTrail(t *observe.AuditTrail) { g.eventTrail = t }

// SetLastPlanCache sets the plan result cache for save_as_workflow.
func (g *Gateway) SetLastPlanCache(c *sync.Map) { g.lastPlanCache = c }

// SetBrowserEngine attaches the browser engine for runtime management.
func (g *Gateway) SetBrowserEngine(e *browser.Engine, headless bool, dataDir string) {
	g.browserEngine = e
	g.browserHeadless = headless
	g.browserDataDir = dataDir
}

// SetBrowserRecognizer attaches the OCR recognizer.
func (g *Gateway) SetBrowserRecognizer(r *browser.Recognizer) { g.browserRecognizer = r }

// SetBrowserWorker attaches the browser sub-agent worker.
func (g *Gateway) SetBrowserWorker(w *browser.Worker) { g.browserWorker = w }

// SetBrowserNotifier attaches the browser event notifier.
func (g *Gateway) SetBrowserNotifier(n *browser.Notifier) { g.browserNotifier = n }

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
	g.registerSystemRoutes()    // healthz, version, tenants, metrics, settings, backup, speech, heartbeat, federation
	g.registerChatRoutes()      // chat, ws, conversations, persona, emotion, bots, inbox, webhooks, webchat
	g.registerMemoryRoutes()    // memory, graph, identity, embeddings, search
	g.registerKnowledgeRoutes() // knowledge base (RAG)
	g.registerTaskRoutes()      // tasks, state kernel, reflection, documents
	g.registerTriggerRoutes()   // triggers, cron, scheduler, tools, sandbox
	g.registerPluginRoutes()    // plugins, skills, skill market, skillhub
	g.registerGovernanceRoutes() // audit, trust, iterate, review, cost, usage
	g.registerProviderRoutes()  // LLM providers, router stats
	g.registerReverieRoutes()   // reverie inner monologue
	g.registerWorkflowRoutes()  // workflow engine (DAG)
	g.registerRBACRoutes()      // role-based access control
	g.registerApprovalRoutes()  // human-in-the-loop approval
	g.registerSSERoutes()       // SSE event stream
	g.registerTraceRoutes()     // execution trace / audit API
	g.registerBrowserRoutes()   // browser engine management
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
