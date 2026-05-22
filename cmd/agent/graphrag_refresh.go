package main

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"time"

	agentrt "yunque-agent/internal/agentcore/runtime"
	"yunque-agent/pkg/safego"

	"yunque-agent/internal/ledgercore"
)

const (
	defaultGraphRAGMaxCommunitySize = 10
	defaultGraphRAGRebuildInterval  = 6 * time.Hour
)

type graphRAGRuntimeConfig struct {
	Enabled         bool
	MaxCommunity    int
	RebuildInterval time.Duration
}

func initGraphRAGRuntime(app *agentrt.App, ldg *ledger.Ledger) {
	cfg := graphRAGConfigFromEnv()
	if !cfg.Enabled {
		slog.Info("GraphRAG disabled by configuration")
		return
	}

	graphRAG := ledger.NewGraphRAG(ldg.Backend())
	rebuild := func(ctx context.Context, reason string) {
		start := time.Now()
		if err := graphRAG.BuildCommunities(ctx, cfg.MaxCommunity); err != nil {
			slog.Warn("GraphRAG community rebuild failed", "reason", reason, "err", err)
			return
		}
		ldg.Recall.SetGraphRAG(graphRAG)
		app.Set("graphrag", graphRAG)
		slog.Info("GraphRAG community index ready",
			"reason", reason,
			"communities", len(graphRAG.Communities()),
			"max_community", cfg.MaxCommunity,
			"elapsed", time.Since(start))
	}

	rebuild(context.Background(), "startup")
	if cfg.RebuildInterval <= 0 {
		return
	}

	app.Lifecycle.RegisterFunc("graphrag_rebuild", func(ctx context.Context) error {
		safego.Go("graphrag-rebuild-ticker", func() {
			ticker := time.NewTicker(cfg.RebuildInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					rebuild(ctx, "periodic")
				}
			}
		})
		return nil
	}, nil)
	slog.Info("GraphRAG periodic rebuild scheduled", "interval", cfg.RebuildInterval)
}

func graphRAGConfigFromEnv() graphRAGRuntimeConfig {
	return graphRAGRuntimeConfig{
		Enabled:         envBool("GRAPHRAG_ENABLED", true),
		MaxCommunity:    envInt("GRAPHRAG_MAX_COMMUNITY_SIZE", defaultGraphRAGMaxCommunitySize),
		RebuildInterval: envDuration("GRAPHRAG_REBUILD_INTERVAL", defaultGraphRAGRebuildInterval),
	}
}

func envBool(key string, fallback bool) bool {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if raw == "" {
		return fallback
	}
	switch raw {
	case "1", "true", "yes", "on", "enabled":
		return true
	case "0", "false", "no", "off", "disabled":
		return false
	default:
		return fallback
	}
}

func envDuration(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	if raw == "0" {
		return 0
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}
	return d
}
