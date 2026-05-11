package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/planner"
	agentrt "yunque-agent/internal/agentcore/runtime"
	builtinCogni "yunque-agent/internal/cognikernel/builtin"
	"yunque-agent/internal/controlplane/gateway"
	mcpkg "yunque-agent/internal/mcp"
	"yunque-agent/pkg/cogni"
	"yunque-agent/pkg/skills"
)

// cogniHookEnabled gates whether activated Cogni declarations actually
// influence the planner's system prompt. We default to ON so dropping a
// .json into data/cognis/ has visible effect, but the wiring stays
// per-module so it can be turned off without disabling the API.
const cogniHookEnabled = true

// cogniModule wires the hot-pluggable Cogni Registry into the runtime.
//
// Boot behaviour:
//  1. Init creates an empty cogni.Registry and attaches it to the App
//     (component key "cogni_registry") so other subsystems can look it up.
//  2. Init also pushes the registry into the Gateway via SetCogniRegistry
//     so the /v1/cognis/* admin endpoints become live.
//  3. Init builds a cogni.Hook backed by an in-memory TraceStore so every
//     turn produces a structured Trace consumable via /v1/cognis/traces.
//  4. Start performs a one-shot reload from `${DataDir}/cognis/` so any
//     declarations the user has dropped in are picked up.
//
// Hot-plug behaviour after boot:
//   - Drop a *.json file into data/cognis/ → POST /v1/cognis/reload picks it up.
//   - DELETE /v1/cognis/{id} or POST /v1/cognis/{id}/disable to remove/disable.
//   - POST /v1/cognis with a JSON body to add a declaration without a file.
//   - GET /v1/cognis/traces → recent turn traces (activations / context bytes / tool diff).
//
// The module is profile=lite because Cogni declarations themselves are cheap;
// gating happens at activation time (Evaluator) which is opt-in per turn.
type cogniModule struct {
	registry       *cogni.Registry
	dir            string
	store          cogni.TraceStore
	fileLog        *FileTraceStore
	sentinel       *cogni.Sentinel
	workflowEngine *cogni.WorkflowEngine
	experiences    map[string]*cogni.ExperienceStore // keyed by cogni ID
	hook           *cogni.Hook
	mcpMgr         *cogni.MCPManager
	bus            *cogni.CogniBus
	scheduler      *cogni.PerceptionScheduler
	costTracker    *cogni.CostTracker
	autoOrganizer  *cogni.AutoOrganizer
}

func (m *cogniModule) Name() string { return "cogni" }
func (m *cogniModule) Description() string {
	return "声明式智体注册中心，支持热插拔加载/启停"
}
func (m *cogniModule) Profile() string { return "lite" }

func (m *cogniModule) Init(ctx context.Context, app *agentrt.App) error {
	m.registry = cogni.NewRegistry()
	m.dir = filepath.Join(app.Config.DataDir, "cognis")

	// Prefer the file-backed trace store so decision history survives
	// restarts; fall back to pure in-memory if the file cannot be opened
	// (e.g. read-only data dir) — traces are diagnostic, never blocking.
	tracePath := filepath.Join(m.dir, "traces.jsonl")
	if fs, err := NewFileTraceStore(tracePath, 512, 10*1024*1024); err == nil {
		m.fileLog = fs
		m.store = fs
		slog.Info("cogni: trace log ready", "path", tracePath)
	} else {
		slog.Warn("cogni: using in-memory trace store (file log unavailable)", "err", err)
		m.store = cogni.NewInMemoryTraceStore(512)
	}

	app.Set(agentrt.CompCogniRegistry, m.registry)
	app.Set(agentrt.CompCogniTraces, m.store)

	m.sentinel = cogni.NewSentinel(m.store, m.registry, cogni.SentinelPolicy{
		Interval:              5 * time.Minute,
		AutoDisableOnCritical: cogniAutoDisableFromEnv(),
	})
	app.Set(agentrt.CompCogniSentinel, m.sentinel)

	// Evolution engine with LLM-powered bench & analyze
	evolutionEngine := cogni.NewEvolutionEngine(cogni.DefaultEvolutionConfig(), m.dir)
	evolutionEngine.SetRegistry(m.registry)
	if app.LLMPool != nil {
		if cl := app.LLMPool.GetOrFallback("smart"); cl != nil {
			evolutionEngine.SetBenchFunc(func(ctx context.Context, cogniID string) (*cogni.BenchResult, error) {
				decl, ok := m.registry.Get(cogniID)
				if !ok {
					return nil, fmt.Errorf("cogni %q not found", cogniID)
				}
				prompt := fmt.Sprintf("Evaluate this AI agent declaration and score its quality (0-100). Consider: activation rules clarity, context relevance, skill coverage, edge cases.\n\nDeclaration ID: %s\nDisplay Name: %s\nDescription: %s\n\nReturn ONLY a JSON: {\"score\": <0-100>, \"passed\": <count>, \"failed\": <count>, \"total\": <count>, \"failures\": [{\"task_id\": \"...\", \"expected\": \"...\", \"actual\": \"...\"}]}",
					decl.ID, decl.DisplayName, decl.Description)
				reply, err := cl.Chat(ctx, []llm.Message{{Role: "user", Content: prompt}}, 0.3)
				if err != nil {
					return nil, err
				}
				var br cogni.BenchResult
				if idx := strings.Index(reply, "{"); idx >= 0 {
					reply = reply[idx:]
				}
				if idx := strings.LastIndex(reply, "}"); idx >= 0 {
					reply = reply[:idx+1]
				}
				if err := json.Unmarshal([]byte(reply), &br); err != nil {
					return &cogni.BenchResult{Score: 50, Total: 1, Passed: 1}, nil
				}
				return &br, nil
			})
			evolutionEngine.SetAnalyzeFunc(func(ctx context.Context, failures []cogni.TaskFailure) ([]cogni.SkillMutation, error) {
				failJSON, _ := json.Marshal(failures)
				prompt := fmt.Sprintf("Analyze these AI agent benchmark failures and propose specific mutations to fix them.\n\nFailures:\n%s\n\nReturn ONLY a JSON array of mutations: [{\"skill_name\": \"...\", \"mutation_type\": \"prompt|parameter|timeout\", \"after\": {\"key\": \"value\"}, \"rationale\": \"...\"}]", string(failJSON))
				reply, err := cl.Chat(ctx, []llm.Message{{Role: "user", Content: prompt}}, 0.3)
				if err != nil {
					return nil, err
				}
				if idx := strings.Index(reply, "["); idx >= 0 {
					reply = reply[idx:]
				}
				if idx := strings.LastIndex(reply, "]"); idx >= 0 {
					reply = reply[:idx+1]
				}
				var mutations []cogni.SkillMutation
				if err := json.Unmarshal([]byte(reply), &mutations); err != nil {
					return nil, fmt.Errorf("parse mutations: %w", err)
				}
				return mutations, nil
			})
			slog.Info("cogni: evolution engine bench+analyze wired via LLM")
		}
	}

	// Federation
	selfID := "local"
	selfURL := "http://localhost" + app.Config.Addr
	federation := cogni.NewCogniFederation(selfID, selfURL, m.registry)

	// Self-genesis engine
	var genesis *cogni.Genesis
	if app.LLMPool != nil {
		if cl := app.LLMPool.GetOrFallback("smart"); cl != nil {
			genesis = cogni.NewGenesis(func(ctx context.Context, system, user string) (string, error) {
				msgs := []llm.Message{
					{Role: "system", Content: system},
					{Role: "user", Content: user},
				}
				return cl.Chat(ctx, msgs, 0.7)
			})
		}
	}

	m.experiences = make(map[string]*cogni.ExperienceStore)

	// MCPManager: per-cogni MCP server connections with lazy initialization.
	connector := cogni.NewStdioMCPConnector(func(ctx context.Context, def cogni.MCPServerDef) (cogni.MCPConnection, error) {
		switch def.Transport {
		case "streamable_http", "sse":
			headers := def.Headers
			if headers == nil {
				headers = make(map[string]string)
			}
			timeout := 30 * time.Second
			if def.Timeout > 0 {
				timeout = time.Duration(def.Timeout) * time.Second
			}
			provider := mcpkg.NewStreamableHTTPProvider(def.URL, headers, timeout)
			if err := provider.Start(ctx); err != nil {
				return nil, err
			}
			return &mcpProviderBridge{provider: provider}, nil
		default:
			env := cogni.ResolveEnv(def.Env)
			provider := mcpkg.NewStdioProvider(def.Command, def.Args, env)
			if err := provider.Start(ctx); err != nil {
				return nil, err
			}
			return &mcpProviderBridge{provider: provider}, nil
		}
	})
	m.mcpMgr = cogni.NewMCPManager(connector)

	// CogniBus: intent broadcast + bidding router.
	m.bus = cogni.NewCogniBus(cogni.NewEvaluator(), cogni.DefaultBusConfig())

	// Cost tracker for economics layer.
	m.costTracker = cogni.NewCostTracker()

	// AutoOrganizer: automatically create cogni declarations from installed skills.
	if app.SkillRegistry != nil {
		reg := app.SkillRegistry
		m.autoOrganizer = cogni.NewAutoOrganizer(m.registry, func() []cogni.SkillInfo {
			all := reg.All()
			out := make([]cogni.SkillInfo, len(all))
			for i, s := range all {
				out[i] = cogni.SkillInfo{
					Name:        s.Name(),
					Description: s.Description(),
					Category:    reg.CategoryOf(s.Name()),
				}
			}
			return out
		})
		if app.LLMPool != nil {
			if cl := app.LLMPool.GetOrFallback("smart"); cl != nil {
				m.autoOrganizer.SetLLM(func(ctx context.Context, system, user string) (string, error) {
					msgs := []llm.Message{
						{Role: "system", Content: system},
						{Role: "user", Content: user},
					}
					return cl.Chat(ctx, msgs, 0.7)
				})
			}
		}
	}

	// Wire WorkflowEngine with a SkillExecutor that delegates to the skill registry.
	if app.SkillRegistry != nil {
		reg := app.SkillRegistry
		m.workflowEngine = cogni.NewWorkflowEngine(func(ctx context.Context, skillName string, args map[string]any) (any, error) {
			sk, ok := reg.Get(skillName)
			if !ok {
				return nil, fmt.Errorf("skill %q not found", skillName)
			}
			env := &skills.Environment{}
			if app.LLMPool != nil {
				if cl := app.LLMPool.GetOrFallback("smart"); cl != nil {
					env.LLMCall = func(ctx context.Context, system, user string) (string, error) {
						msgs := []llm.Message{
							{Role: "system", Content: system},
							{Role: "user", Content: user},
						}
						return cl.Chat(ctx, msgs, 0.7)
					}
				}
			}
			result, err := sk.Execute(ctx, args, env)
			return result, err
		})
	}

	// NL Config translator: natural language → structured configuration.
	// Reuses the same LLM client as Genesis.
	var nlTranslator *cogni.NLConfigTranslator
	if app.LLMPool != nil {
		if cl := app.LLMPool.GetOrFallback("smart"); cl != nil {
			nlTranslator = cogni.NewNLConfigTranslator(func(ctx context.Context, system, user string) (string, error) {
				msgs := []llm.Message{
					{Role: "system", Content: system},
					{Role: "user", Content: user},
				}
				return cl.Chat(ctx, msgs, 0.3)
			})
			slog.Info("cogni: NL config translator wired")
		}
	}

	// Gateway wiring — placed after all cogni subsystems are initialized
	// so the gateway receives valid (non-nil) references.
	if gwRaw, ok := app.Get(agentrt.CompGateway); ok {
		if gw, ok := gwRaw.(*gateway.Gateway); ok {
			gw.SetCogniRegistry(m.registry, m.dir)
			gw.SetCogniTraceStore(m.store)
			gw.SetCogniSentinel(m.sentinel)
			gw.SetCogniWorkflowEngine(m.workflowEngine)
			gw.SetCogniExperiences(m.experiences)
			gw.SetCogniGenesis(genesis)
			gw.SetCogniEvolution(evolutionEngine)
			gw.SetCogniFederation(federation)
			gw.SetCogniCostTracker(m.costTracker)
			gw.SetNLConfigTranslator(nlTranslator)
		}
	}

	if cogniHookEnabled && app.Planner != nil {
		hook := cogni.NewHook(m.registry)
		m.hook = hook
		hook.SetTraceStore(m.store)
		if app.Orchestrator != nil {
			orch := app.Orchestrator
			hook.SetMemorySearch(func(ctx context.Context, tenantID, query string) string {
				return orch.CompileContext(ctx, tenantID, query)
			})
		}
		// Until a real pkg/capsule.Registry is wired into the runtime, use
		// skill category as the pseudo-capsule id so Declaration.Surface.
		// FromCapsules can actually narrow by "owner group" (e.g.
		// `from_capsules: ["browser"]`).
		if app.SkillRegistry != nil {
			reg := app.SkillRegistry
			hook.SetSkillOwner(func(name string) string {
				return reg.CategoryOf(name)
			})
		}
		hook.SetExperienceProvider(func(cogniID string) *cogni.ExperienceStore {
			return m.experiences[cogniID]
		})
		app.Planner.SetCogniContext(func(_ context.Context, message, tenantID, channel string) string {
			return hook.BuildContext(cogni.ContextRequest{
				Message:  message,
				TenantID: tenantID,
				Channel:  channel,
			})
		})
		app.Planner.SetCogniSkillFilter(func(message, tenantID, channel string, in []skills.Skill) []skills.Skill {
			return hook.FilterSkills(cogni.ContextRequest{
				Message:  message,
				TenantID: tenantID,
				Channel:  channel,
			}, in)
		})
		app.Planner.SetCogniTrace(func(message, tenantID, channel string) (planner.CogniTraceDetail, bool) {
			trace, ok := hook.TraceSnapshot(cogni.ContextRequest{
				Message:  message,
				TenantID: tenantID,
				Channel:  channel,
			})
			if !ok {
				return planner.CogniTraceDetail{}, false
			}
			detail := planner.CogniTraceDetail{
				ContextBytes:      trace.Context.Bytes,
				TemplateFallbacks: trace.Context.TemplateFallbacks,
				MessageHash:       trace.MessageHash,
				DurationMs:        trace.DurationMs,
			}
			for _, a := range trace.Activations {
				if a.Activated {
					if a.DisplayName != "" {
						detail.Activated = append(detail.Activated, a.DisplayName)
					} else {
						detail.Activated = append(detail.Activated, a.ID)
					}
				}
			}
			if tf := trace.ToolFilter; tf != nil {
				detail.ToolBefore = tf.Before
				detail.ToolAfter = tf.After
				detail.Removed = append([]string(nil), tf.Removed...)
				detail.FellBackToInput = tf.FellBackToInput
			}
			return detail, true
		})
		// Wire cost tracking + bus routing on activation
		{
			tracker := m.costTracker
			bus := m.bus
			hook.SetOnActivation(func(cogniID string, score float64) {
				if tracker != nil && score > 0 {
					tracker.Record(cogni.CostEntry{
						CogniID:   cogniID,
						Cost:      score * 0.01,
						Operation: "activation",
					})
				}
				if bus != nil {
					slog.Debug("cogni_bus: activation", "cogni", cogniID, "score", score)
				}
			})
		}
		slog.Info("cogni: planner context injection + surface filter + bus + cost + trace wired")
	}

	// Adapt existing Plugins as Cogni declarations so they participate
	// in the unified evaluation pipeline.
	if app.PluginReg != nil {
		cogni.RegisterPlugins(m.registry, app.PluginReg.All())
		slog.Info("cogni: existing plugins adapted as declarations",
			"count", len(app.PluginReg.All()))
	}

	// Register built-in Cogni declarations (office/creative/data-analyst).
	builtinDecls, err := builtinCogni.LoadAll()
	if err != nil {
		slog.Warn("cogni: failed to load built-in declarations", "err", err)
	} else {
		for _, d := range builtinDecls {
			if addErr := m.registry.Add(d, "builtin"); addErr != nil {
				slog.Warn("cogni: failed to add built-in declaration", "id", d.ID, "err", addErr)
			}
		}
		if len(builtinDecls) > 0 {
			slog.Info("cogni: built-in declarations registered", "count", len(builtinDecls))
		}
	}

	slog.Info("cogni registry initialized", "dir", m.dir)
	return nil
}

func (m *cogniModule) Start(_ context.Context) error {
	summary, err := m.registry.ReloadFromDir(m.dir)
	if err != nil {
		slog.Warn("cogni: initial reload failed", "dir", m.dir, "err", err)
		return nil
	}
	if summary.Added > 0 || summary.Updated > 0 {
		slog.Info("cogni: declarations loaded",
			"dir", m.dir,
			"added", summary.Added,
			"updated", summary.Updated,
			"errors", len(summary.Errors))
	}
	for _, e := range summary.Errors {
		slog.Warn("cogni: declaration load error", "path", e.Path, "err", e.Err)
	}

	// Auto-organize: create cognis from installed skills
	if m.autoOrganizer != nil {
		result := m.autoOrganizer.Sync(context.Background())
		if result.Created > 0 || result.Updated > 0 {
			slog.Info("cogni: auto-organized skills into cognis",
				"created", result.Created,
				"updated", result.Updated,
				"removed", result.Removed)
		}
	}

	m.initExperienceStores()

	// Re-initialize experience stores when cognis are added/updated/removed.
	m.registry.OnChange(func(event, id string) {
		m.initExperienceStores()
	})

	if m.sentinel != nil {
		m.sentinel.Start(context.Background())
	}

	// Start perception scheduler for cron-based activation + webhook registration.
	if m.hook != nil {
		m.scheduler = cogni.NewPerceptionScheduler(m.registry, m.hook, func(ctx context.Context, cogniID string, signal *cogni.PerceptionSignal) {
			slog.Info("cogni: perception event", "cogni", cogniID, "schedule", signal.ScheduleTriggered, "webhook", signal.WebhookTriggered)
		})
		m.scheduler.Start()
		paths := m.scheduler.WebhookPaths()
		if len(paths) > 0 {
			slog.Info("cogni: perception webhook paths registered", "paths", paths)
		}
	}

	return nil
}

// initExperienceStores creates/updates per-cogni ExperienceStore instances
// for every declaration that has Experience.Enabled == true.
func (m *cogniModule) initExperienceStores() {
	active := m.registry.Active()
	seen := make(map[string]bool, len(active))
	for _, d := range active {
		seen[d.ID] = true

		// Experience stores
		if d.Experience.Enabled {
			if _, exists := m.experiences[d.ID]; !exists {
				cfg := d.Experience
				if cfg.StoreDir == "" {
					cfg.StoreDir = filepath.Join(m.dir, "experience", d.ID)
				}
				m.experiences[d.ID] = cogni.NewExperienceStore(d.ID, cfg)
				slog.Info("cogni: experience store initialized", "cogni", d.ID)
			}
		}

		// MCP connections (lazy — registered but not connected until first use)
		if len(d.MCP.Servers) > 0 && m.mcpMgr != nil {
			m.mcpMgr.Register(d.ID, d.MCP)
		}

		// Bus registration
		if m.bus != nil {
			m.bus.Register(d)
		}

		// Economics
		if m.costTracker != nil && (d.Economics.BudgetPerRun > 0 || d.Economics.DailyBudget > 0) {
			m.costTracker.SetConfig(d.ID, d.Economics)
		}
	}
	for id := range m.experiences {
		if !seen[id] {
			delete(m.experiences, id)
		}
	}
}

func (m *cogniModule) Stop() error {
	if m.scheduler != nil {
		m.scheduler.Stop()
	}
	if m.sentinel != nil {
		m.sentinel.Stop()
	}
	if m.mcpMgr != nil {
		m.mcpMgr.CloseAll()
	}
	if m.fileLog != nil {
		_ = m.fileLog.Close()
	}
	return nil
}

// mcpProviderBridge adapts internal/mcp.Provider to cogni.MCPConnection.
type mcpProviderBridge struct {
	provider mcpkg.Provider
	stopper  interface{ Stop() }
}

func (b *mcpProviderBridge) ListTools(ctx context.Context) ([]cogni.MCPToolInfo, error) {
	tools, err := b.provider.ListTools(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]cogni.MCPToolInfo, len(tools))
	for i, t := range tools {
		out[i] = cogni.MCPToolInfo{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		}
	}
	return out, nil
}

func (b *mcpProviderBridge) CallTool(ctx context.Context, name string, args map[string]any) (any, error) {
	result, err := b.provider.CallTool(ctx, name, args)
	if err != nil {
		return nil, err
	}
	if result.IsError && len(result.Content) > 0 {
		return nil, fmt.Errorf("%s", result.Content[0].Text)
	}
	if len(result.Content) > 0 {
		return result.Content[0].Text, nil
	}
	return "", nil
}

func (b *mcpProviderBridge) Close() error {
	if b.stopper != nil {
		b.stopper.Stop()
	}
	return nil
}

// cogniAutoDisableFromEnv reads COGNI_AUTO_DISABLE (default false). When
// true, the sentinel disables cognis whose score stays critical.
func cogniAutoDisableFromEnv() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("COGNI_AUTO_DISABLE")))
	return v == "true" || v == "1" || v == "yes" || v == "on"
}

func (m *cogniModule) Status() agentrt.ModuleStatus {
	return agentrt.ModuleStatus{
		Name:        m.Name(),
		Description: m.Description(),
		Profile:     m.Profile(),
		Enabled:     m.registry != nil,
		Running:     m.registry != nil,
	}
}
