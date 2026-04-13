package main

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"time"

	"yunque-agent/internal/experimental/circuit"
	"yunque-agent/internal/agentcore/llm"
	agentrt "yunque-agent/internal/agentcore/runtime"
	"yunque-agent/internal/appdir"
	iledger "yunque-agent/internal/ledger"
)

// initLLM initializes the LLM client pool, provider registry, local model
// auto-detection, and circuit breaker.
// Extracted from main.go lines 131-291.
func initLLM(app *agentrt.App) error {
	cfg := app.Config

	// Primary LLM client (smart tier)
	app.LLMClient = llm.NewClient(cfg.LLMBaseURL, cfg.LLMAPIKey, cfg.LLMModel)

	// Multi-model LLM pool: register fast/smart/expert tiers
	app.LLMPool = llm.NewPool()
	app.LLMPool.Register("smart", app.LLMClient)
	app.LLMPool.SetPrimary("smart")

	if cfg.LLMFastModel != "" {
		fastURL := cfg.LLMFastURL
		if fastURL == "" {
			fastURL = cfg.LLMBaseURL
		}
		fastKey := cfg.LLMFastKey
		if fastKey == "" {
			fastKey = cfg.LLMAPIKey
		}
		app.LLMPool.Register("fast", llm.NewClient(fastURL, fastKey, cfg.LLMFastModel))
		slog.Info("llm pool: fast tier configured", "model", cfg.LLMFastModel)
	}
	if cfg.LLMExpertModel != "" {
		expertURL := cfg.LLMExpertURL
		if expertURL == "" {
			expertURL = cfg.LLMBaseURL
		}
		expertKey := cfg.LLMExpertKey
		if expertKey == "" {
			expertKey = cfg.LLMAPIKey
		}
		app.LLMPool.Register("expert", llm.NewClient(expertURL, expertKey, cfg.LLMExpertModel))
		slog.Info("llm pool: expert tier configured", "model", cfg.LLMExpertModel)
	}
	slog.Info("llm pool initialized", "tiers", app.LLMPool.Keys())

	// Provider registry
	app.Providers = llm.NewProviderRegistry(app.LLMPool)
	if app.Ledger != nil {
		kvStore := iledger.NewKVConfigStore(app.Ledger, "providers")
		app.Providers.SetPersistStore(kvStore)
		migrator := iledger.NewKVMigrator(app.Ledger)
		_ = migrator.MigrateFile("providers", "all", appdir.File("providers.json"))
		slog.Info("provider: using Ledger KV for persistence")

		// Load persisted providers BEFORE registering primary —
		// Register(primary) calls persist() which would overwrite saved data with [].
		if count, err := app.Providers.LoadFromStore(); err != nil {
			slog.Warn("provider: load from Ledger KV error", "err", err)
		} else if count > 0 {
			slog.Info("provider: loaded from Ledger KV", "count", count)
		}
	} else {
		app.Providers.SetPersistPath(appdir.File("providers.json"))
		slog.Info("provider: using JSON file for persistence (Ledger unavailable)")
	}
	_ = app.Providers.Register(llm.ProviderConfig{
		ID:           "primary",
		DisplayName:  "Primary (" + cfg.LLMModel + ")",
		Type:         llm.ProviderTypeChat,
		BaseURL:      cfg.LLMBaseURL,
		APIKeys:      []string{cfg.LLMAPIKey},
		Model:        cfg.LLMModel,
		Enabled:      true,
		Tier:         "smart",
		Capabilities: []llm.Capability{llm.CapChat, llm.CapTools},
	})

	// Load extra providers from env + file in parallel (these won't overwrite Ledger ones since Register is idempotent by ID)
	type providerLoadResult struct {
		source    string
		providers []llm.ProviderConfig
		err       error
	}
	providerLoads := make(chan providerLoadResult, 2)
	go func() {
		providers, err := llm.LoadProvidersFromEnv()
		providerLoads <- providerLoadResult{source: "env", providers: providers, err: err}
	}()
	go func() {
		providers, err := llm.LoadProvidersFromFile(appdir.File("providers.json"))
		providerLoads <- providerLoadResult{source: "file", providers: providers, err: err}
	}()
	for i := 0; i < 2; i++ {
		res := <-providerLoads
		if res.err != nil {
			if res.source == "env" {
				slog.Warn("LLM_PROVIDERS parse error", "err", res.err)
			} else {
				slog.Warn("data/providers.json parse error", "err", res.err)
			}
			continue
		}
		for _, pc := range res.providers {
			if err := app.Providers.Register(pc); err != nil {
				slog.Warn("provider registration failed", "id", pc.ID, "err", err)
			}
		}
		if len(res.providers) > 0 {
			slog.Info("extra LLM providers loaded", "source", res.source, "count", len(res.providers))
		}
	}
	slog.Info("provider registry initialized", "providers", len(app.Providers.List()))

	// Local model auto-detection (Ollama / vLLM)
	type localProbe struct {
		name string
		cfg  llm.LocalAutoConfig
	}
	localProbes := make([]localProbe, 0, 2)
	if cfg.OllamaBaseURL != "" {
		localProbes = append(localProbes, localProbe{
			name: "ollama",
			cfg: llm.LocalAutoConfig{
				BaseURL: cfg.OllamaBaseURL,
				Model:   cfg.OllamaModel,
				Tier:    cfg.LocalModelTier,
				Backend: llm.BackendOllama,
			},
		})
	}
	if cfg.VLLMBaseURL != "" {
		localProbes = append(localProbes, localProbe{
			name: "vllm",
			cfg: llm.LocalAutoConfig{
				BaseURL: cfg.VLLMBaseURL,
				Model:   cfg.VLLMModel,
				Tier:    cfg.LocalModelTier,
				Backend: llm.BackendVLLM,
			},
		})
	}
	if len(localProbes) > 0 {
		var localWg sync.WaitGroup
		for _, probe := range localProbes {
			probe := probe
			localWg.Add(1)
			go func() {
				defer localWg.Done()
				probeCtx, probeCancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer probeCancel()
				if pid, err := llm.AutoRegisterLocal(probeCtx, app.Providers, probe.cfg); err != nil {
					slog.Warn(probe.name+" auto-register failed", "err", err)
				} else {
					slog.Info(probe.name+" local provider registered", "id", pid)
				}
			}()
		}
		localWg.Wait()
	}

	// Circuit breaker for LLM resilience
	app.LLMBreaker = circuit.New(func(ctx context.Context, system, user string) (string, error) {
		msgs := []llm.Message{{Role: "system", Content: system}, {Role: "user", Content: user}}
		return app.LLMClient.Chat(ctx, msgs, 0.7)
	}, circuit.Config{FailureThreshold: 6, RecoveryTime: 15 * time.Second, HalfOpenMax: 2})
	slog.Info("circuit breaker initialized for LLM calls")

	// Optional LLM fallback
	if fallbackURL := os.Getenv("LLM_FALLBACK_URL"); fallbackURL != "" {
		fallbackKey := os.Getenv("LLM_FALLBACK_KEY")
		if fallbackKey == "" {
			fallbackKey = cfg.LLMAPIKey
		}
		fallbackModel := os.Getenv("LLM_FALLBACK_MODEL")
		if fallbackModel == "" {
			fallbackModel = cfg.LLMModel
		}
		fallbackClient := llm.NewClient(fallbackURL, fallbackKey, fallbackModel)
		app.LLMBreaker.AddFallback(fallbackModel, func(ctx context.Context, system, user string) (string, error) {
			msgs := []llm.Message{{Role: "system", Content: system}, {Role: "user", Content: user}}
			return fallbackClient.Chat(ctx, msgs, 0.7)
		})
		slog.Info("LLM fallback registered", "model", fallbackModel)
	}

	return nil
}
