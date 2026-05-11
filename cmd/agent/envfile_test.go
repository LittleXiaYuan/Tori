package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPickBestEnvFilePrefersRealConfigOverPlaceholder(t *testing.T) {
	root := t.TempDir()
	desktop := filepath.Join(root, "desktop")
	if err := os.MkdirAll(desktop, 0o755); err != nil {
		t.Fatal(err)
	}

	rootEnv := filepath.Join(root, ".env")
	desktopEnv := filepath.Join(desktop, ".env")

	if err := os.WriteFile(rootEnv, []byte("LLM_API_KEY=sk-real\nLLM_BASE_URL=https://api.example.com/v1\nLLM_MODEL=gpt-test\nJWT_SECRET=strong-secret-value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(desktopEnv, []byte("LLM_API_KEY=1\nLLM_BASE_URL=1\nLLM_MODEL=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got := pickBestEnvFile([]string{desktopEnv, rootEnv})
	if got != rootEnv {
		t.Fatalf("expected %s, got %s", rootEnv, got)
	}
}

func TestPickBestEnvFileAcceptsLocalProviderWithoutApiKey(t *testing.T) {
	dir := t.TempDir()
	env := filepath.Join(dir, ".env")
	if err := os.WriteFile(env, []byte("LLM_BASE_URL=http://localhost:11434/v1\nLLM_MODEL=qwen3.5:4b\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got := pickBestEnvFile([]string{env})
	if got != env {
		t.Fatalf("expected %s, got %s", env, got)
	}
}
