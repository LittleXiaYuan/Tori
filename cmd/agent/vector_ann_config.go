package main

import (
	"context"
	"log/slog"
	"os"
	"strconv"
	"strings"

	agentrt "yunque-agent/internal/agentcore/runtime"

	"github.com/LittleXiaYuan/ledger"
)

func configureLedgerVectorANN(ctx context.Context, app *agentrt.App, ldg *ledger.Ledger, tenantID string) {
	backend, ok := parseVectorANNBackend(os.Getenv("VECTOR_ANN_BACKEND"))
	if !ok {
		slog.Warn("ledger vector ANN backend invalid; falling back to brute-force",
			"value", os.Getenv("VECTOR_ANN_BACKEND"))
	}
	app.Set("vector_ann_backend", string(backend))

	switch backend {
	case ledger.VectorANNHNSW:
		cfg := ledger.DefaultHNSWConfig()
		cfg.M = envInt("VECTOR_HNSW_M", cfg.M)
		cfg.EfConstruction = envInt("VECTOR_HNSW_EF_CONSTRUCTION", cfg.EfConstruction)
		cfg.EfSearch = envInt("VECTOR_HNSW_EF_SEARCH", cfg.EfSearch)
		ldg.Vector.EnableHNSW(cfg)
		if ldg.Vector.Dimensions() > 0 {
			if err := ldg.Vector.TrainHNSW(ctx, tenantID, cfg); err != nil {
				slog.Warn("ledger vector ANN: HNSW startup training failed", "tenant", tenantID, "err", err)
			}
		}
		size, maxLevel := ldg.Vector.HNSWStats()
		app.Set("hnsw_index", ldg.Vector)
		slog.Info("ledger vector ANN backend enabled",
			"backend", backend, "tenant", tenantID, "size", size, "max_level", maxLevel,
			"m", cfg.M, "ef_construction", cfg.EfConstruction, "ef_search", cfg.EfSearch)
	case ledger.VectorANNIVF:
		cfg := ledger.DefaultIVFConfig()
		cfg.NumClusters = envInt("VECTOR_IVF_CLUSTERS", cfg.NumClusters)
		cfg.NumProbe = envInt("VECTOR_IVF_PROBE", cfg.NumProbe)
		cfg.MinPointsToTrain = envInt("VECTOR_IVF_MIN_TRAIN", cfg.MinPointsToTrain)
		ldg.Vector.EnableIVF(cfg)
		if ldg.Vector.Dimensions() > 0 {
			if err := ldg.Vector.TrainIVF(ctx, tenantID); err != nil {
				slog.Warn("ledger vector ANN: IVF startup training failed", "tenant", tenantID, "err", err)
			}
		}
		clusters, totalVectors := ldg.Vector.IVFStats()
		slog.Info("ledger vector ANN backend enabled",
			"backend", backend, "tenant", tenantID, "clusters", clusters, "vectors", totalVectors,
			"min_train", cfg.MinPointsToTrain)
	default:
		ldg.Vector.SetANNBackend(ledger.VectorANNBruteForce)
		slog.Info("ledger vector ANN backend disabled; using brute-force search")
	}
}

func parseVectorANNBackend(raw string) (ledger.VectorANNBackend, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "brute", "bruteforce", "brute_force", "linear":
		return ledger.VectorANNBruteForce, true
	case "ivf":
		return ledger.VectorANNIVF, true
	case "hnsw":
		return ledger.VectorANNHNSW, true
	default:
		return ledger.VectorANNBruteForce, false
	}
}

func envInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}

func defaultTenantID() string {
	if v := strings.TrimSpace(os.Getenv("DEFAULT_TENANT_ID")); v != "" {
		return v
	}
	return "default"
}
