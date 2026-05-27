package gateway

import (
	"encoding/json"
	"net/http"
	"runtime"
	"strconv"
	"time"

	"yunque-agent/internal/version"
)

type subsystemCheck struct {
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

func checkOK(detail string) subsystemCheck {
	return subsystemCheck{Status: "ok", Detail: detail}
}

func checkDegraded(detail string) subsystemCheck {
	return subsystemCheck{Status: "degraded", Detail: detail}
}

func checkDown(detail string) subsystemCheck {
	return subsystemCheck{Status: "down", Detail: detail}
}

// handleLivez — minimal liveness probe. If the HTTP server can respond, the
// process is alive. K8s uses this to decide whether to restart the container.
func (g *Gateway) handleLivez(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status":     "ok",
		"uptime_sec": int(time.Since(g.startTime).Seconds()),
	})
}

// handleReadyz — readiness probe. Returns 503 if critical subsystems are not
// initialized. K8s uses this to decide whether to route traffic to the pod.
func (g *Gateway) handleReadyz(w http.ResponseWriter, r *http.Request) {
	checks := make(map[string]subsystemCheck)
	allReady := true

	if g.planner != nil {
		modelHealth := g.planner.ModelRuntimeHealth()
		if !modelHealth.Configured {
			checks["llm"] = checkDown("model runtime not configured")
			allReady = false
		} else if modelHealth.BreakerState == "open" {
			checks["llm"] = checkDegraded("circuit breaker open (failures=" + strconv.Itoa(modelHealth.Failures) + ")")
		} else {
			checks["llm"] = checkOK("breaker=" + modelHealth.BreakerState)
		}
	} else {
		checks["llm"] = checkDown("planner not initialized")
		allReady = false
	}

	if g.convStore != nil {
		checks["conversations"] = checkOK("initialized")
	} else {
		checks["conversations"] = checkDown("store not initialized")
		allReady = false
	}

	if g.memory != nil {
		checks["memory"] = checkOK("initialized")
	} else {
		checks["memory"] = checkDegraded("memory manager not initialized")
	}

	if g.ledgerHealth != nil {
		if err := g.ledgerHealth.HealthCheck(r.Context()); err != nil {
			checks["ledger"] = checkDown(err.Error())
			allReady = false
		} else {
			checks["ledger"] = checkOK("healthy")
		}
	} else {
		checks["ledger"] = checkDegraded("ledger health checker not initialized")
	}

	status := "ready"
	httpCode := http.StatusOK
	if !allReady {
		status = "not_ready"
		httpCode = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpCode)
	json.NewEncoder(w).Encode(map[string]any{
		"status":     status,
		"version":    version.Version,
		"uptime_sec": int(time.Since(g.startTime).Seconds()),
		"checks":     checks,
	})
}

// handleCognitiveHealth performs a deep inspection of all cognitive subsystems:
// LLM routing, memory pipeline, knowledge base, cogni registry, self-heal, and
// observability. Reports "cognitive fitness" to enable proactive alerting
// before users notice degradation.
func (g *Gateway) handleCognitiveHealth(w http.ResponseWriter, r *http.Request) {
	checks := make(map[string]subsystemCheck)
	degradedCount := 0
	downCount := 0

	record := func(name string, c subsystemCheck) {
		checks[name] = c
		switch c.Status {
		case "degraded":
			degradedCount++
		case "down":
			downCount++
		}
	}

	// ── LLM Circuit Breaker ──
	if g.planner != nil {
		modelHealth := g.planner.ModelRuntimeHealth()
		if !modelHealth.Configured {
			record("llm_breaker", checkDown("model runtime not configured"))
		} else {
			switch modelHealth.BreakerState {
			case "open":
				record("llm_breaker", checkDegraded("open — LLM calls blocked (failures="+strconv.Itoa(modelHealth.Failures)+")"))
			case "half-open":
				record("llm_breaker", checkDegraded("half-open — probing recovery"))
			default:
				record("llm_breaker", checkOK("closed"))
			}
		}
	} else {
		record("llm_breaker", checkDown("planner not initialized"))
	}

	// ── Smart Router (multi-model) ──
	if g.smartRouter != nil {
		record("smart_router", checkOK("active"))
	} else {
		record("smart_router", checkDegraded("not configured — single-model mode"))
	}

	// ── Memory Pipeline ──
	if g.memory != nil {
		record("memory_manager", checkOK("active"))
	} else {
		record("memory_manager", checkDegraded("not initialized"))
	}
	if g.pipeline != nil {
		record("memory_pipeline", checkOK("active"))
	} else {
		record("memory_pipeline", checkDegraded("not initialized"))
	}

	// ── Ledger (persistent cognitive state) ──
	if g.ledgerHealth != nil {
		if err := g.ledgerHealth.HealthCheck(r.Context()); err != nil {
			record("ledger", checkDown(err.Error()))
		} else {
			record("ledger", checkOK("healthy"))
		}
	} else {
		record("ledger", checkDegraded("ledger health checker not initialized"))
	}

	// ── Knowledge Store (RAG) ──
	if g.knowledgeStore != nil {
		record("knowledge_store", checkOK("active"))
	} else {
		record("knowledge_store", checkDegraded("not initialized — RAG disabled"))
	}

	// ── Embedding Resolver ──
	if g.embedResolver != nil {
		record("embeddings", checkOK("active"))
	} else {
		record("embeddings", checkDegraded("not configured — semantic search unavailable"))
	}

	// ── Cogni Registry (declarative AI shells) ──
	if g.cogniRegistry != nil {
		count := len(g.cogniRegistry.List())
		record("cogni_registry", checkOK(strconv.Itoa(count)+" cognis loaded"))
	} else {
		record("cogni_registry", checkDegraded("not initialized"))
	}

	// ── Self-Heal ──
	if g.healer != nil {
		record("self_heal", checkOK("active"))
	} else {
		record("self_heal", checkDegraded("not configured"))
	}

	// ── Guardrails ──
	if g.zhGuard != nil {
		record("guardrails", checkOK("pipeline active"))
	} else {
		record("guardrails", checkDegraded("not configured"))
	}

	// ── Observability Metrics ──
	if g.metrics != nil {
		snap := g.metrics.Snapshot()
		detail := "requests=" + strconv.FormatInt(snap.RequestsTotal, 10) +
			" tokens_in=" + strconv.FormatInt(snap.TokensIn, 10) +
			" tokens_out=" + strconv.FormatInt(snap.TokensOut, 10)
		record("metrics", checkOK(detail))
	} else {
		record("metrics", checkDegraded("metrics collector not initialized"))
	}

	// ── Cognitive Counters ──
	if g.metrics != nil && g.metrics.Cognitive() != nil {
		cog := g.metrics.Cognitive()
		detail := "memory_ingest=" + strconv.FormatInt(cog.MemoryIngest.Load(), 10) +
			" recalls=" + strconv.FormatInt(cog.MemoryRecall.Load(), 10) +
			" reflections=" + strconv.FormatInt(cog.ReflectEval.Load(), 10) +
			" heartbeats=" + strconv.FormatInt(cog.HeartbeatRun.Load(), 10)
		record("cognitive_counters", checkOK(detail))
	}

	// ── Persona ──
	if g.persona != nil {
		record("persona", checkOK("loaded"))
	} else {
		record("persona", checkDegraded("no persona configured"))
	}

	// ── Runtime Pool ──
	if g.runtimePool != nil {
		record("runtime_pool", checkOK("active"))
	} else {
		record("runtime_pool", checkDegraded("not initialized"))
	}

	// ── Process Resources ──
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	resources := map[string]any{
		"goroutines":    runtime.NumGoroutine(),
		"heap_alloc_mb": float64(memStats.HeapAlloc) / (1024 * 1024),
		"heap_sys_mb":   float64(memStats.HeapSys) / (1024 * 1024),
		"gc_cycles":     memStats.NumGC,
		"gc_pause_ms":   float64(memStats.PauseTotalNs) / 1e6,
	}

	status := "healthy"
	httpCode := http.StatusOK
	if downCount > 0 {
		status = "unhealthy"
		httpCode = http.StatusServiceUnavailable
	} else if degradedCount > 0 {
		status = "degraded"
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpCode)
	json.NewEncoder(w).Encode(map[string]any{
		"status":     status,
		"version":    version.Version,
		"uptime_sec": int(time.Since(g.startTime).Seconds()),
		"checks":     checks,
		"summary":    map[string]int{"ok": len(checks) - degradedCount - downCount, "degraded": degradedCount, "down": downCount},
		"resources":  resources,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
	})
}
