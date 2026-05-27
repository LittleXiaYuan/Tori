package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"yunque-agent/internal/agentcore/audit"
	"yunque-agent/internal/agentcore/guardrails"
	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/llm/distill"
	"yunque-agent/internal/agentcore/review"
	agentrt "yunque-agent/internal/agentcore/runtime"
	"yunque-agent/internal/agentcore/selfheal/iterate"
	"yunque-agent/internal/agentcore/skillgrowth/adapter"
	"yunque-agent/internal/agentcore/speech"
	"yunque-agent/internal/agentcore/trust"
	"yunque-agent/internal/agentcore/websearch"
	"yunque-agent/internal/controlplane/gateway"
	"yunque-agent/internal/execution/channel"
	"yunque-agent/pkg/plugin"
)

// initExtensions is Phase 8: sets up trust tracker, review gate, iterate engine,
// distiller, skill growth detector, audit trail, and plugin extensions.
func initExtensions(app *agentrt.App) error {
	cfg := app.Config
	gw := app.MustGet(agentrt.CompGateway).(*gateway.Gateway)
	channelReg := app.MustGet(agentrt.CompChannelReg).(*channel.Registry)
	searchReg := app.MustGet(agentrt.CompSearchReg).(*websearch.Registry)
	speechReg, _ := app.Get("speech_reg")
	speechRegistry, _ := speechReg.(*speech.Registry)

	// ── Trust Tracker → Gateway ──
	// Created in initPlanner but not wired to gateway; wire it here.
	if rawTrust, ok := app.Get("trust_tracker"); ok {
		if tt, ok := rawTrust.(*trust.Tracker); ok {
			gw.SetTrustTracker(tt)
			slog.Info("trust tracker wired to gateway")
		}
	}

	// ── Review Gate ──
	reviewGate := review.NewGate()
	reviewGate.SetLLMReview(func(ctx context.Context, operation string) (bool, error) {
		system := "你是安全审查引擎。评估以下操作是否安全。\n" +
			"如果安全，回复 ALLOW。如果有风险，回复 DENY 并简述原因。\n" +
			"只回复 ALLOW 或 DENY:原因"
		reply, err := app.LLMClient.Chat(ctx, []llm.Message{
			{Role: "system", Content: system},
			{Role: "user", Content: operation},
		}, LowLLMTemperature)
		if err != nil {
			return true, err
		}
		if len(reply) >= 5 && reply[:5] == "ALLOW" {
			return true, nil
		}
		return false, nil
	})
	gw.SetReviewGate(reviewGate)
	slog.Info("review gate initialized")

	// ── Knowledge Distiller ──
	distiller := distill.New(llmChatFunc(app.LLMClient, LowLLMTemperature))
	if app.Orchestrator != nil {
		distiller.SetStore(func(ctx context.Context, key, value, category string) error {
			fact := fmt.Sprintf("[蒸馏规则/%s] %s: %s", category, key, value)
			return app.Orchestrator.Ingest(ctx, "system", fact, "distill_rule", "distiller")
		})
		distiller.SetSearch(func(ctx context.Context, query string) (string, bool) {
			ctx2 := app.Orchestrator.CompileContext(ctx, "system", "蒸馏规则 "+query)
			if ctx2 != "" {
				return ctx2, true
			}
			return "", false
		})
	}
	gw.SetDistiller(distiller)
	slog.Info("knowledge distiller initialized")

	// ── Skill Growth Detector ──
	growthThreshold := 3
	if v := os.Getenv("SKILLGROW_THRESHOLD"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			growthThreshold = n
		}
	}
	skillGrowDetector := adapter.NewDetector(growthThreshold)
	if app.Orchestrator != nil {
		skillGrowDetector.SetMemSearch(func(ctx context.Context, query string) (int, string) {
			compiled := app.Orchestrator.CompileContext(ctx, "system", query)
			if compiled == "" {
				return 0, ""
			}
			count := 1
			for i := range compiled {
				if compiled[i] == '\n' {
					count++
				}
			}
			return count, compiled
		})
	}
	skillGrowDetector.SetOnProposal(func(ctx context.Context, pattern, suggestion string) {
		slog.Info("skillgrow: proposal", "pattern", pattern, "suggestion", suggestion)
	})
	gw.SetSkillGrow(skillGrowDetector)
	slog.Info("skill growth detector initialized", "threshold", growthThreshold)

	// ── Audit Trail ──
	auditTrail := audit.NewTrail(cfg.DataPath("audit"))
	gw.SetAuditTrail(auditTrail)
	slog.Info("audit trail initialized", "dir", cfg.DataPath("audit"))

	// ── Iterate Engine ──
	// Prefer the engine constructed by initSoulLayer (it has the Discusser,
	// OnExecute handler for proposal application, and nighttime wiring). If
	// that one is missing (lean configurations), fall back to building a
	// minimal engine here so the gateway HTTP routes still function.
	var iterEngine *iterate.Engine
	if soulRaw, ok := app.Get("iterate_engine"); ok {
		if eng, ok := soulRaw.(*iterate.Engine); ok {
			iterEngine = eng
			slog.Info("iterate engine: reusing soul-layer engine for gateway")
		}
	}
	if iterEngine == nil {
		iterBudget := app.Config.SelfIterateTokenBudget
		if iterBudget <= 0 {
			iterBudget = 5000
		}
		iterEngine = iterate.NewEngine(iterate.Config{
			Enabled:     os.Getenv("SELF_ITERATE_ENABLED") == "true",
			TokenBudget: iterBudget,
			MaxRounds:   3,
			AutoApprove: os.Getenv("SELF_ITERATE_AUTO_APPROVE") == "true",
			DataDir:     cfg.DataPath("iterate"),
		})
		iterEngine.SetLLMCall(func(ctx context.Context, system, user string) (string, int, error) {
			msgs := []llm.Message{
				{Role: "system", Content: system},
				{Role: "user", Content: user},
			}
			reply, err := app.LLMClient.Chat(ctx, msgs, IterateLLMTemperature)
			estTokens := (len(system) + len(user) + len(reply)) / 4
			return reply, estTokens, err
		})
		if app.Orchestrator != nil {
			iterEngine.SetOnRecord(func(ctx context.Context, summary string) {
				_ = app.Orchestrator.Ingest(ctx, "system", summary, "iterate_insight", "iterate_engine")
			})
		}
		app.Set("iterate_engine", iterEngine)
		if iterEngine.Enabled() {
			slog.Info("iterate engine: minimal fallback constructed", "budget", iterBudget)
		}
	}
	gw.SetIterateEngine(iterEngine)
	app.Set(agentrt.CompIterateEngine, iterEngine)

	// ── Speech Registry (Edge TTS = free, no API key) ──
	if speechRegistry == nil {
		speechRegistry = speech.NewRegistry()
		speechRegistry.RegisterTTS(speech.NewEdgeTTS())
		slog.Info("speech: edge_tts registered (free)")
	}
	gw.SetSpeechRegistry(speechRegistry)
	slog.Info("speech registry wired to gateway", "tts", speechRegistry.ListTTS(), "stt", speechRegistry.ListSTT())

	// ── Plugin Extensions (SDK bridge) ──
	initPluginExtensions(app, gw, searchReg, speechRegistry, channelReg)

	return nil
}

// initPluginExtensions sets up the ExtensionRegistry, Plugin API handler,
// and token management. This is the bridge between plugins and agent subsystems.
func initPluginExtensions(
	app *agentrt.App,
	gw *gateway.Gateway,
	searchReg *websearch.Registry,
	speechReg *speech.Registry,
	channelReg *channel.Registry,
) {
	extRegistry := plugin.NewExtensionRegistry()
	app.Set("ext_registry", extRegistry)

	extRegistry.OnRegisterProvider(func(cfg plugin.ProviderRegistration) error {
		return app.Providers.Register(llm.ProviderConfig{
			ID: cfg.ID, DisplayName: cfg.DisplayName,
			Type: llm.ProviderType(cfg.Type), BaseURL: cfg.BaseURL,
			APIKeys: cfg.APIKeys, Model: cfg.Model,
			Enabled: true, Tier: cfg.Tier, Priority: cfg.Priority,
		})
	})

	extRegistry.OnRegisterSearch(func(cfg plugin.SearchRegistration) error {
		searchReg.Register(websearch.NewGenericHTTP(cfg.Name, cfg.BaseURL, cfg.APIKey, cfg.SearchPath))
		return nil
	})

	if guardRaw, ok := app.Get(agentrt.CompGuardPipeline); ok {
		if guardPipeline, ok := guardRaw.(*guardrails.Pipeline); ok {
			extRegistry.OnRegisterGuardrail(func(cfg plugin.GuardrailRegistration) error {
				guardPipeline.Add(guardrails.NewKeywordGuardFromList(cfg.Name, cfg.Keywords))
				return nil
			})
		}
	}

	extRegistry.OnRegisterSpeech(func(cfg plugin.SpeechRegistration) error {
		if cfg.Type == "tts" {
			speechReg.RegisterTTS(speech.NewOpenAICompatTTS(cfg.Name, cfg.BaseURL, cfg.APIKey, cfg.Model, cfg.Voice))
		}
		return nil
	})

	slog.Info("extension registry initialized")

	// Plugin API handler (SDK bridge)
	tokenMgr := gateway.NewPluginTokenManager()
	memMgr := plugin.NewPluginMemoryManager(app.Config.DataPath("plugin_memory"))

	apiHandler := gateway.NewPluginAPIHandler(gateway.PluginAPIConfig{
		LLMClient:    app.LLMClient,
		LLMBreaker:   app.LLMBreaker.Call,
		MemManager:   app.MemManager,
		Orchestrator: app.Orchestrator,
		MemoryMgr:    memMgr,
		SearchFunc: func(ctx context.Context, query string, limit int) ([]gateway.SearchResult, error) {
			results, err := searchReg.Search(ctx, query, limit)
			if err != nil {
				return nil, err
			}
			out := make([]gateway.SearchResult, len(results))
			for i, r := range results {
				out[i] = gateway.SearchResult{Title: r.Title, URL: r.URL, Snippet: r.Snippet}
			}
			return out, nil
		},
		SendFunc: func(channelType, target, content, format string) error {
			ch, ok := channelReg.Get(channelType)
			if !ok {
				return fmt.Errorf("channel %q not found", channelType)
			}
			return ch.Send(context.Background(), target, channel.Reply{Content: content, Format: format})
		},
		ExtRegistry: extRegistry,
	}, tokenMgr)

	gw.MountPluginAPIRoutes(apiHandler)
	app.Set("plugin_token_mgr", tokenMgr)
	app.Set("plugin_mem_mgr", memMgr)

	// Issue tokens for existing Script Plugins
	for _, p := range app.PluginReg.All() {
		sp, ok := p.(*plugin.ScriptPlugin)
		if !ok {
			continue
		}
		perms := []string{"llm", "memory", "search"}
		for _, h := range sp.Manifest().Hooks {
			if h == "memory.extract" {
				perms = append(perms, "memory.read", "memory.write")
			}
		}
		sp.SetAPIToken(tokenMgr.Issue(sp.Name(), perms))
	}

	slog.Info("plugin API handler initialized")
}
