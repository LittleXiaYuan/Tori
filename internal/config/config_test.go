package config

import (
	"os"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	cfg := Load()
	if cfg.Addr != ":9090" {
		t.Fatalf("expected :9090, got %s", cfg.Addr)
	}
	if cfg.LLMModel != "zai-org/GLM-5" {
		t.Fatalf("expected default model, got %s", cfg.LLMModel)
	}
}

func TestLoadFromEnv(t *testing.T) {
	os.Setenv("AGENT_ADDR", ":8080")
	defer os.Unsetenv("AGENT_ADDR")
	cfg := Load()
	if cfg.Addr != ":8080" {
		t.Fatalf("expected :8080, got %s", cfg.Addr)
	}
}

func TestValidateMissingAPIKey(t *testing.T) {
	cfg := Config{LLMBaseURL: "http://x", LLMModel: "m"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for missing API key")
	}
}

func TestValidatePass(t *testing.T) {
	cfg := Config{LLMBaseURL: "http://x", LLMAPIKey: "k", LLMModel: "m"}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
