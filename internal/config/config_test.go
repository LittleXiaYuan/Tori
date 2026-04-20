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

// Validate() is intentionally permissive now: the setup flow moved from
// the CLI wizard to the web UI, so a missing API key is surfaced via
// NeedsSetup() + the /v1/setup/* endpoints rather than blocking boot.
// The two tests below lock in that contract so nobody accidentally
// regresses Validate() back to erroring on empty LLM config.

func TestValidateAlwaysPassesPostWebSetup(t *testing.T) {
	cfg := Config{} // nothing set at all
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate must not fail on empty config (web setup flow); got %v", err)
	}
}

func TestNeedsSetupReportsMissingLLM(t *testing.T) {
	missingKey := Config{LLMBaseURL: "http://x", LLMModel: "m"}
	if !missingKey.NeedsSetup() {
		t.Fatal("NeedsSetup must be true when LLMAPIKey is empty")
	}
	missingURL := Config{LLMAPIKey: "k", LLMModel: "m"}
	if !missingURL.NeedsSetup() {
		t.Fatal("NeedsSetup must be true when LLMBaseURL is empty")
	}
	missingModel := Config{LLMBaseURL: "http://x", LLMAPIKey: "k"}
	if !missingModel.NeedsSetup() {
		t.Fatal("NeedsSetup must be true when LLMModel is empty")
	}

	allSet := Config{LLMBaseURL: "http://x", LLMAPIKey: "k", LLMModel: "m"}
	if allSet.NeedsSetup() {
		t.Fatal("NeedsSetup must be false once all three core LLM fields are present")
	}
}
