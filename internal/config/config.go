package config

import (
	"os"
	"strconv"
	"strings"

	"yunque-agent/internal/appdir"
)

// Config holds all agent configuration.
type Config struct {
	DataDir          string // root data directory (default: "data")
	Addr             string // HTTP listen address
	LLMBaseURL       string
	LLMAPIKey        string
	LLMModel         string
	LLMFastURL       string // optional: separate endpoint for fast/cheap model
	LLMFastKey       string
	LLMFastModel     string
	LLMExpertURL     string // optional: separate endpoint for expert/powerful model
	LLMExpertKey     string
	LLMExpertModel   string
	OllamaBaseURL    string // optional: Ollama local model endpoint
	OllamaModel      string
	VLLMBaseURL      string // optional: vLLM local model endpoint
	VLLMModel        string
	LocalModelTier   string // tier assignment for local models: fast/smart/expert
	HostReadPaths    string // comma-separated host paths for read-only access
	HostWritePaths   string // comma-separated host paths for writable access
	TelegramToken    string
	FeishuAppID      string
	FeishuAppSecret  string
	JWTSecret        string
	ToriAPIBaseURL   string
	MinerUEnabled    bool
	MinerUBackend    string
	MinerUCommand    string
	MinerUCLIArgs    string
	MinerUOutputDir  string
	MinerUTimeoutSec int
	// Self-iteration
	SelfIterateEnabled     bool
	SelfIterateTokenBudget int
	SelfIterateMaxRounds   int
	SelfIterateCooldownMin int
	SelfIterateAutoApprove bool
	ReActEnabled           bool   // enable Ledger-powered ReAct reasoning mode
	LongHorizonEnabled     bool   // enable DAG-based long-horizon planner for complex tasks
	ReflectMode            string // "strict", "learning", "off" (default: "learning")
	ReflectModel           string // LLM pool key for reflect evaluator (empty = primary)
	Profile                string // "lite", "standard", "full" (default: "standard")
	DisabledModules        string // comma-separated module names to disable

	// BuildFlavor selects industry-specific capabilities.
	// "standard" (default) — general-purpose agent
	// "legal"    — adds legal-ai engine (case retrieval, contract review)
	// "collect"  — adds collection scripts, knowledge embedding
	// Flavors do NOT change the core agent — they only control which
	// industry plugins are auto-registered in init_plugins.
	BuildFlavor string
}

// Load reads config from environment variables.
func Load() Config {
	iterBudget := 5000
	if v := getenv("SELF_ITERATE_TOKEN_BUDGET", ""); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			iterBudget = n
		}
	}
	iterRounds := 3
	if v := getenv("SELF_ITERATE_MAX_ROUNDS", ""); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			iterRounds = n
		}
	}
	iterCooldown := 60
	if v := getenv("SELF_ITERATE_COOLDOWN", ""); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			iterCooldown = n
		}
	}
	return Config{
		DataDir:        getenv("DATA_DIR", appdir.DataDir()),
		Addr:           getenv("AGENT_ADDR", ":9090"),
		LLMBaseURL:     getenv("LLM_BASE_URL", "https://api-ai.gitcode.com/v1"),
		LLMAPIKey:      getenv("LLM_API_KEY", ""),
		LLMModel:       getenv("LLM_MODEL", "zai-org/GLM-5"),
		LLMFastURL:     getenv("LLM_FAST_URL", ""),
		LLMFastKey:     getenv("LLM_FAST_KEY", ""),
		LLMFastModel:   getenv("LLM_FAST_MODEL", ""),
		LLMExpertURL:   getenv("LLM_EXPERT_URL", ""),
		LLMExpertKey:   getenv("LLM_EXPERT_KEY", ""),
		LLMExpertModel: getenv("LLM_EXPERT_MODEL", ""),
		OllamaBaseURL:  getenv("OLLAMA_BASE_URL", ""),
		OllamaModel:    getenv("OLLAMA_MODEL", ""),
		VLLMBaseURL:    getenv("VLLM_BASE_URL", ""),
		VLLMModel:      getenv("VLLM_MODEL", ""),
		LocalModelTier: getenv("LOCAL_MODEL_TIER", "fast"),

		HostReadPaths:          getenv("HOST_READ_PATHS", ""),
		HostWritePaths:         getenv("HOST_WRITE_PATHS", ""),
		TelegramToken:          getenv("TELEGRAM_BOT_TOKEN", ""),
		FeishuAppID:            getenv("FEISHU_APP_ID", ""),
		FeishuAppSecret:        getenv("FEISHU_APP_SECRET", ""),
		JWTSecret:              getenv("JWT_SECRET", ""),
		ToriAPIBaseURL:         getenv("TORI_API_BASE_URL", ""),
		MinerUEnabled:          getenv("MINERU_ENABLED", "") == "true",
		MinerUBackend:          getenv("MINERU_BACKEND", "cli"),
		MinerUCommand:          getenv("MINERU_COMMAND", "mineru"),
		MinerUCLIArgs:          getenv("MINERU_CLI_ARGS", "-p {input_file} -o {output_dir} -b pipeline"),
		MinerUOutputDir:        getenv("MINERU_OUTPUT_DIR", ""),
		MinerUTimeoutSec:       getenvInt("MINERU_TIMEOUT_SEC", 300),
		SelfIterateEnabled:     getenv("SELF_ITERATE_ENABLED", "") == "true",
		SelfIterateTokenBudget: iterBudget,
		SelfIterateMaxRounds:   iterRounds,
		SelfIterateCooldownMin: iterCooldown,
		SelfIterateAutoApprove: getenv("SELF_ITERATE_AUTO_APPROVE", "") == "true",
		ReActEnabled:           getenv("REACT_ENABLED", "") == "true",
		LongHorizonEnabled:     getenv("LONG_HORIZON_ENABLED", "") == "true",
		ReflectMode:            getenv("REFLECT_MODE", "learning"),
		ReflectModel:           getenv("REFLECT_MODEL", ""),
		Profile:                getenv("AGENT_PROFILE", "standard"),
		DisabledModules:        getenv("DISABLED_MODULES", ""),
		BuildFlavor:            getenv("BUILD_FLAVOR", "standard"),
	}
}

// Validate checks configuration fields. Missing LLM settings are no longer
// fatal — the web UI will prompt users to complete setup.
func (c Config) Validate() error {
	return nil
}

// Warnings returns non-fatal configuration issues for startup logging.
func (c Config) Warnings() []string {
	var w []string
	if c.NeedsSetup() {
		w = append(w, "LLM not configured — setup wizard will launch on first visit")
	}
	if c.JWTSecret == "" {
		w = append(w, "JWT_SECRET not set — auth tokens will use an insecure default")
	}
	if c.JWTSecret != "" && len(c.JWTSecret) < 16 {
		w = append(w, "JWT_SECRET is shorter than 16 chars — consider a stronger secret")
	}
	if c.Profile != ProfileLite && c.Profile != ProfileStandard && c.Profile != ProfileFull {
		w = append(w, "AGENT_PROFILE unrecognized ("+c.Profile+") — falling back to standard")
	}
	if c.LLMExpertURL != "" && c.LLMExpertModel == "" {
		w = append(w, "LLM_EXPERT_URL set but LLM_EXPERT_MODEL missing — expert tier will be unused")
	}
	if c.LLMFastURL != "" && c.LLMFastModel == "" {
		w = append(w, "LLM_FAST_URL set but LLM_FAST_MODEL missing — fast tier will be unused")
	}
	return w
}

// Summary returns a human-readable one-liner of key config state for startup logs.
func (c Config) Summary() string {
	llm := c.LLMModel
	if llm == "" {
		llm = "(not set)"
	}
	return "profile=" + c.Profile + " addr=" + c.Addr + " model=" + llm + " data=" + c.DataDir
}

// NeedsSetup returns true when essential LLM configuration is missing.
func (c Config) NeedsSetup() bool {
	return c.LLMBaseURL == "" || c.LLMAPIKey == "" || c.LLMModel == ""
}

// DataPath joins sub-paths to the configured data directory root.
// Usage: cfg.DataPath("memory", "daily") → "data/memory/daily"
func (c Config) DataPath(elem ...string) string {
	parts := append([]string{c.DataDir}, elem...)
	return strings.Join(parts, "/")
}

// Profile constants — controls which subsystems are initialized.
const (
	ProfileLite     = "lite"     // chat-only: no workflow/triggers/federation/heartbeat
	ProfileStandard = "standard" // full features minus experimental
	ProfileFull     = "full"     // everything including experimental modules
)

// ProfileAtLeast returns true when the configured profile is at or above the given level.
func (c Config) ProfileAtLeast(level string) bool {
	order := map[string]int{ProfileLite: 0, ProfileStandard: 1, ProfileFull: 2}
	cur, ok := order[c.Profile]
	if !ok {
		cur = 1 // default to standard
	}
	req, ok := order[level]
	if !ok {
		return true
	}
	return cur >= req
}

// IsModuleDisabled returns true when the given module name appears in DISABLED_MODULES.
func (c Config) IsModuleDisabled(name string) bool {
	if c.DisabledModules == "" {
		return false
	}
	for _, m := range strings.Split(c.DisabledModules, ",") {
		if strings.TrimSpace(m) == name {
			return true
		}
	}
	return false
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

// ParsePaths splits a comma-separated path list, trims whitespace, and drops empties.
func ParsePaths(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
