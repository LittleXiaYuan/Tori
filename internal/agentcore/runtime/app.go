package runtime

import (
	"fmt"
	"sync"

	"github.com/LittleXiaYuan/ledger"

	ctxwindow "yunque-agent/internal/agentcore/context"
	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/memory"
	"yunque-agent/internal/agentcore/planner"
	"yunque-agent/internal/config"
	"yunque-agent/internal/experimental/circuit"
	agreact "yunque-agent/internal/experimental/react"
	"yunque-agent/internal/observe"
	"yunque-agent/pkg/plugin"
	"yunque-agent/pkg/skills"
)

// App is the root dependency container for yunque-agent. It holds
// direct references to heavily-used subsystems plus a generic
// component registry for everything else.
type App struct {
	// ── Core ──
	Config    *config.Config
	Lifecycle *Lifecycle

	// ── LLM subsystem ──
	LLMClient  *llm.Client
	LLMPool    *llm.Pool
	LLMBreaker *circuit.Breaker
	Providers  *llm.ProviderRegistry

	// ── Memory & orchestration ──
	ShortMem     *memory.ShortTerm
	MidMem       *memory.MidTerm
	LongMem      *memory.LongTerm
	KnGraph      *memory.Graph
	EditableMem  *memory.EditableMemory
	MemManager   *memory.Manager
	MemPipeline  *memory.Pipeline
	Orchestrator *memory.Orchestrator

	// ── Context window management ──
	CtxManager *ctxwindow.Manager

	// ── Ledger (state infrastructure) ──
	Ledger      *ledger.Ledger
	ReActRunner *agreact.Runner

	// ── Planner ──
	Planner        *planner.Planner
	Reverie        *planner.Reverie
	SkillOptimizer *planner.SkillOptimizer

	// ── Runtime pool (multi-agent) ──
	RuntimePool *Pool

	// ── Plugin & skill registries ──
	PluginReg     *plugin.Registry
	SkillRegistry *skills.Registry

	// ── Observability ──
	Metrics *observe.Metrics

	// ── Module registry (hot-pluggable subsystems) ──
	Modules *ModuleRegistry

	// ── Generic component registry ──
	mu         sync.RWMutex
	components map[string]any
}

// NewApp creates a new, empty App with initialized internals.
func NewApp(cfg *config.Config) *App {
	return &App{
		Config:     cfg,
		Lifecycle:  NewLifecycle(),
		Metrics:    observe.New(),
		Modules:    NewModuleRegistry(),
		components: make(map[string]any),
	}
}

// Set stores a component in the registry under the given key.
func (a *App) Set(key string, value any) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.components[key] = value
}

// Get retrieves a component by key. Returns (nil, false) if not found.
func (a *App) Get(key string) (any, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	v, ok := a.components[key]
	return v, ok
}

// MustGet retrieves a component by key or panics.
//
// Panic-on-missing is intentional and follows Go's Must* idiom: this
// function is only called from cmd/agent/init_*.go during process boot,
// where a missing component means an earlier init phase forgot to
// app.Set() the matching key — i.e. a programmer error, not a runtime
// condition. Failing loudly here surfaces the mis-wiring immediately
// instead of letting a nil pointer propagate into the first real
// request. Do NOT convert this to error-returning in an attempt to
// clean up "production panics"; the contract is that callers of
// MustGet have already proven the key must be registered.
//
// For business-path lookups where a missing component is a recoverable
// condition, use Get(key) (any, bool) instead.
func (a *App) MustGet(key string) any {
	v, ok := a.Get(key)
	if !ok {
		panic(fmt.Sprintf("agentrt: component %q not registered", key))
	}
	return v
}

// ── Component key constants ──

const (
	CompChannelReg          = "channel_reg"
	CompConvStore           = "conv_store"
	CompScheduler           = "scheduler"
	CompPersona             = "persona"
	CompPersonaChain        = "persona_chain"
	CompGuardPipeline       = "guard_pipeline"
	CompToolGuard           = "tool_guard"
	CompEgressGuard         = "egress_guard"
	CompAuditChain          = "audit_chain"
	CompGateway             = "gateway"
	CompTenantMgr           = "tenant_mgr"
	CompFeishuAPI           = "feishu_api"
	CompSearchReg           = "search_reg"
	CompSmartRouter         = "smart_router"
	CompIdentityRes         = "identity_resolver"
	CompHealer              = "healer"
	CompSelfhealLife        = "selfheal_lifecycle"
	CompCostTracker         = "cost_tracker"
	CompForkTree            = "fork_tree"
	CompForkPersist         = "fork_persister"
	CompEmbedResolver       = "embed_resolver"
	CompSubagentMgr         = "subagent_mgr"
	CompHandoffReg          = "handoff_reg"
	CompMemPersister        = "mem_persister"
	CompOrchPersist         = "orch_persist"
	CompAdaptiveLoop        = "adaptive_loop"
	CompBotMgr              = "bot_mgr"
	CompSkillMarket         = "skill_market"
	CompFedHub              = "fed_hub"
	CompKnowledgeStore      = "knowledge_store"
	CompReflectEngine       = "reflect_engine"
	CompInbox               = "inbox"
	CompCronMgr             = "cron_mgr"
	CompTriggerMgr          = "trigger_mgr"
	CompHTTPServer          = "http_server"
	CompTrustTracker        = "trust_tracker"
	CompIterateEngine       = "iterate_engine"
	CompPresetMgr           = "preset_mgr"
	CompPluginStateMgr      = "plugin_state_mgr"
	CompPluginLoader        = "plugin_loader"
	CompMCPGateway          = "mcp_gateway"
	CompSessionStore        = "session_store"
	CompBotManager          = "bot_manager"
	CompHeartbeat           = "heartbeat"
	CompModelCatalog        = "model_catalog"
	CompFederationHub       = "federation_hub"
	CompLedger              = "ledger"
	CompReActRunner         = "react_runner"
	CompTaskStore           = "task_store"
	CompTaskRunner          = "task_runner"
	CompGapAnalyzer         = "gap_analyzer"
	CompStateKernel         = "state_kernel"
	CompExperienceStore     = "experience_store"
	CompTemplateStore       = "template_store"
	CompWorkMemMgr          = "work_mem_mgr"
	CompThreadMgr           = "thread_mgr"
	CompCogniRegistry       = "cogni_registry"
	CompCogniTraces         = "cogni_traces"
	CompCogniSentinel       = "cogni_sentinel"
	CompPackRuntimeRegistry = "pack_runtime_registry"
)
