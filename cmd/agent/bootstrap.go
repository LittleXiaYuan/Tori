package main

import (
	"log/slog"
	"os"
	"strings"

	"github.com/joho/godotenv"
	agentrt "yunque-agent/internal/agentcore/runtime"
	"yunque-agent/internal/config"
)

// loadConfig loads the global configuration from environment / .env file.
func loadConfig() *config.Config {
	_ = godotenv.Load()

	// Run setup wizard if no .env file present and stdin is a terminal
	if _, err := os.Stat(".env"); os.IsNotExist(err) && isTerminal() {
		if !runSetupWizard() {
			slog.Error("setup wizard cancelled")
			os.Exit(1)
		}
	}
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		slog.Error("invalid configuration", "err", err)
		os.Exit(1)
	}
	warnWeakSecrets(&cfg)
	return &cfg
}

// warnWeakSecrets emits loud warnings when security-sensitive config values
// are still set to their example/default placeholders.
func warnWeakSecrets(cfg *config.Config) {
	weakJWT := map[string]bool{
		"":                                    true,
		"your-jwt-secret-change-in-production": true,
		"change-me-in-production":              true,
	}
	if weakJWT[cfg.JWTSecret] {
		slog.Warn("⚠️  JWT_SECRET is weak or unset — generate a strong random value for production (e.g. openssl rand -hex 32)")
	}

	apiKey := os.Getenv("DEFAULT_API_KEY")
	if apiKey == "" || apiKey == "ya_your-default-api-key" {
		slog.Warn("⚠️  DEFAULT_API_KEY is weak or unset — anyone can access all /v1/* endpoints")
	}

	origins := os.Getenv("ALLOWED_ORIGINS")
	if origins == "*" {
		slog.Warn("⚠️  ALLOWED_ORIGINS=* allows any domain — restrict to your actual origins in production")
	} else if origins == "" && !isListeningLocalhost(cfg) {
		slog.Warn("⚠️  ALLOWED_ORIGINS not set and binding to non-localhost — CORS will default to same-origin only")
	}
}

// isListeningLocalhost returns true if the agent is bound to a localhost address.
func isListeningLocalhost(cfg *config.Config) bool {
	addr := cfg.Addr
	return addr == "" || addr == ":8080" || addr == ":9090" ||
		strings.HasPrefix(addr, "127.0.0.1:") || strings.HasPrefix(addr, "localhost:") ||
		strings.HasPrefix(addr, "[::1]:")
}

// newApp creates and initializes the App container, running all init phases
// in dependency order.
func newApp(cfg *config.Config) (*agentrt.App, error) {
	app := agentrt.NewApp(cfg)

	// Phase 1: Storage (DB connection)
	if err := initStorage(app); err != nil {
		return nil, err
	}

	// Phase 2: LLM clients & pool
	if err := initLLM(app); err != nil {
		return nil, err
	}

	// Phase 3: Memory, persona, adaptive systems
	if err := initMemory(app); err != nil {
		return nil, err
	}

	// Phase 4: Plugins & skill registry
	if err := initPlugins(app); err != nil {
		return nil, err
	}

	// Phase 5: Channels
	if err := initChannels(app); err != nil {
		return nil, err
	}

	// Phase 6: Planner
	if err := initPlanner(app); err != nil {
		return nil, err
	}

	// Phase 6.5: Browser (optional, non-fatal)
	if err := initBrowser(app); err != nil {
		return nil, err
	}

	wireDocxVerifier(app)

	// Phase 6.8: Intelligence layer (LocalBrain, AgenticThinking, MetaCog, WorldModel, Causal)
	if err := initIntelligence(app); err != nil {
		return nil, err
	}

	// Phase 7: Tasks, extensions, and wiring
	if err := initTasks(app); err != nil {
		return nil, err
	}

	// Phase 8: Extensions (trust tracker, review gate, iterate engine)
	if err := initExtensions(app); err != nil {
		return nil, err
	}

	// Phase 9: Training data pipeline (DataCollector + NightScheduler)
	initTrainingPipeline(app)

	return app, nil
}
