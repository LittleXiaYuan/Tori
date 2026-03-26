package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all agent configuration.
type Config struct {
	Addr            string // HTTP listen address
	LLMBaseURL      string
	LLMAPIKey       string
	LLMModel        string
	LLMFastURL      string // optional: separate endpoint for fast/cheap model
	LLMFastKey      string
	LLMFastModel    string
	LLMExpertURL    string // optional: separate endpoint for expert/powerful model
	LLMExpertKey    string
	LLMExpertModel  string
	OllamaBaseURL   string // optional: Ollama local model endpoint
	OllamaModel     string
	VLLMBaseURL     string // optional: vLLM local model endpoint
	VLLMModel       string
	LocalModelTier  string // tier assignment for local models: fast/smart/expert
	HostReadPaths   string // comma-separated host paths for read-only access
	TelegramToken   string
	FeishuAppID     string
	FeishuAppSecret string
	JWTSecret       string
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
		Addr:                   getenv("AGENT_ADDR", ":9090"),
		LLMBaseURL:             getenv("LLM_BASE_URL", "https://api-ai.gitcode.com/v1"),
		LLMAPIKey:              getenv("LLM_API_KEY", ""),
		LLMModel:               getenv("LLM_MODEL", "zai-org/GLM-5"),
		LLMFastURL:             getenv("LLM_FAST_URL", ""),
		LLMFastKey:             getenv("LLM_FAST_KEY", ""),
		LLMFastModel:           getenv("LLM_FAST_MODEL", ""),
		LLMExpertURL:           getenv("LLM_EXPERT_URL", ""),
		LLMExpertKey:           getenv("LLM_EXPERT_KEY", ""),
		LLMExpertModel:         getenv("LLM_EXPERT_MODEL", ""),
		OllamaBaseURL:          getenv("OLLAMA_BASE_URL", ""),
		OllamaModel:            getenv("OLLAMA_MODEL", ""),
		VLLMBaseURL:            getenv("VLLM_BASE_URL", ""),
		VLLMModel:              getenv("VLLM_MODEL", ""),
		LocalModelTier:         getenv("LOCAL_MODEL_TIER", "fast"),

		HostReadPaths:          getenv("HOST_READ_PATHS", ""),
		TelegramToken:          getenv("TELEGRAM_BOT_TOKEN", ""),
		FeishuAppID:            getenv("FEISHU_APP_ID", ""),
		FeishuAppSecret:        getenv("FEISHU_APP_SECRET", ""),
		JWTSecret:              getenv("JWT_SECRET", ""),
		SelfIterateEnabled:     getenv("SELF_ITERATE_ENABLED", "") == "true",
		SelfIterateTokenBudget: iterBudget,
		SelfIterateMaxRounds:   iterRounds,
		SelfIterateCooldownMin: iterCooldown,
		SelfIterateAutoApprove: getenv("SELF_ITERATE_AUTO_APPROVE", "") == "true",
		ReActEnabled:           getenv("REACT_ENABLED", "") == "true",
		LongHorizonEnabled:     getenv("LONG_HORIZON_ENABLED", "") == "true",
		ReflectMode:            getenv("REFLECT_MODE", "learning"),
		ReflectModel:           getenv("REFLECT_MODEL", ""),
	}
}

// Validate checks required configuration fields.
func (c Config) Validate() error {
	if c.LLMBaseURL == "" {
		return fmt.Errorf("LLM_BASE_URL is required")
	}
	if c.LLMAPIKey == "" {
		return fmt.Errorf("LLM_API_KEY is required")
	}
	if c.LLMModel == "" {
		return fmt.Errorf("LLM_MODEL is required")
	}
	return nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
