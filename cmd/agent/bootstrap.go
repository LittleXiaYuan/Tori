package main

import (
	"log/slog"
	"net"
	"os"
	"strings"

	"github.com/joho/godotenv"
	agentrt "yunque-agent/internal/agentcore/runtime"
	"yunque-agent/internal/config"
)

// loadConfig loads the global configuration from environment / .env file.
//
// Note: initial setup is done entirely via the web UI (/setup). The agent
// starts even when .env is missing — the gateway exposes /v1/setup/* and
// /v1/tori/* for the frontend to drive configuration. This avoids the
// long-standing UX issue where the CLI wizard would block a GUI desktop
// launch on stdin.
func loadConfig() *config.Config {
	_ = godotenv.Load()

	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		slog.Warn("configuration issue", "err", err)
	}
	slog.Info("config loaded", "summary", cfg.Summary())
	for _, w := range cfg.Warnings() {
		slog.Warn("config", "warning", w)
	}
	warnWeakSecrets(&cfg)
	return &cfg
}

// Known placeholder secrets that must never reach production.
var weakJWTSecrets = map[string]bool{
	"":                                     true,
	"your-jwt-secret-change-in-production": true,
	"change-me-in-production":              true,
	"changeme":                             true,
	"default":                              true,
	"secret":                               true,
}

// warnWeakSecrets emits warnings for development and refuses to start under
// production-like deployments when security-sensitive config values are still
// set to their example/default placeholders.
//
// Production is inferred from YUNQUE_ENV=production or when the agent binds to
// a non-loopback address without YUNQUE_ALLOW_WEAK_SECRETS=true. Boot-time
// fail-closed prevents accidental deployment with the default JWT secret from
// the example compose file, which would allow trivial JWT forgery across all
// tenants.
func warnWeakSecrets(cfg *config.Config) {
	production := isProductionLike(cfg)

	if weakJWTSecrets[cfg.JWTSecret] {
		if production {
			slog.Error("JWT_SECRET is weak or set to a known placeholder — refusing to start. " +
				"Set JWT_SECRET to a strong random value (e.g. `openssl rand -hex 32`) " +
				"or set YUNQUE_ALLOW_WEAK_SECRETS=true for explicit development override.")
			os.Exit(1)
		}
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

// isProductionLike returns true when the process is likely running in
// production, so fail-closed checks should engage.
func isProductionLike(cfg *config.Config) bool {
	if strings.EqualFold(os.Getenv("YUNQUE_ALLOW_WEAK_SECRETS"), "true") {
		return false
	}
	if strings.EqualFold(os.Getenv("YUNQUE_ENV"), "production") {
		return true
	}
	// Binding to a non-loopback interface implies the instance is reachable
	// off-host, which is a reasonable stand-in for "production-like".
	return !isListeningLocalhost(cfg)
}

// isListeningLocalhost returns true only when the agent is bound exclusively
// to a loopback address. Go's `net.Listen("tcp", ":9090")` listens on all
// interfaces, so a bare ":port" must be treated as non-loopback.
func isListeningLocalhost(cfg *config.Config) bool {
	addr := strings.TrimSpace(cfg.Addr)
	if addr == "" {
		return false
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	host = strings.Trim(host, "[]")
	if host == "" || host == "0.0.0.0" || host == "::" {
		return false
	}
	if host == "localhost" {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
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

	// Phase 10: Hot-pluggable modules (gated by AGENT_PROFILE)
	registerModules(app)

	return app, nil
}
