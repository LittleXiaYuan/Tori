package main

import (
	"testing"
	"time"
)

func TestGraphRAGConfigFromEnv(t *testing.T) {
	t.Setenv("GRAPHRAG_ENABLED", "false")
	t.Setenv("GRAPHRAG_MAX_COMMUNITY_SIZE", "24")
	t.Setenv("GRAPHRAG_REBUILD_INTERVAL", "30m")

	cfg := graphRAGConfigFromEnv()
	if cfg.Enabled {
		t.Fatal("expected GraphRAG disabled")
	}
	if cfg.MaxCommunity != 24 {
		t.Fatalf("expected max community 24, got %d", cfg.MaxCommunity)
	}
	if cfg.RebuildInterval != 30*time.Minute {
		t.Fatalf("expected 30m rebuild interval, got %s", cfg.RebuildInterval)
	}
}

func TestEnvDurationFallbackAndDisable(t *testing.T) {
	t.Setenv("TEST_DURATION", "bad")
	if got := envDuration("TEST_DURATION", time.Hour); got != time.Hour {
		t.Fatalf("expected fallback duration, got %s", got)
	}
	t.Setenv("TEST_DURATION", "0")
	if got := envDuration("TEST_DURATION", time.Hour); got != 0 {
		t.Fatalf("expected zero duration to disable ticker, got %s", got)
	}
}
