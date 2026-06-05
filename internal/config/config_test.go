package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	// Keep the defaults check hermetic: a developer shell (or CI) may export
	// AGENT_ADDR / LLM_MODEL, which would otherwise override the values under
	// test. getenv treats an empty value as unset and returns the built-in
	// default, so clearing them here exercises the real defaults.
	t.Setenv("AGENT_ADDR", "")
	t.Setenv("LLM_MODEL", "")
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

func TestPackCatalogSourceDirsDefaults(t *testing.T) {
	t.Setenv("PACK_CATALOG_SOURCES", "")
	cfg := Load()
	want := []string{filepath.Join("packs", "official"), filepath.Join("packs", "templates")}
	if got := cfg.PackCatalogSourceDirs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("expected default pack catalog sources %v, got %v", want, got)
	}
}

func TestPackCatalogSourceDirsFromEnv(t *testing.T) {
	t.Setenv("PACK_CATALOG_SOURCES", " packs/internal , C:\\packs\\private\\pack.json, ,packs/vendor ")
	cfg := Load()
	want := []string{"packs/internal", "C:\\packs\\private\\pack.json", "packs/vendor"}
	if got := cfg.PackCatalogSourceDirs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("expected env pack catalog sources %v, got %v", want, got)
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
