package gateway

import (
	"encoding/json"
	"net/http"
	"time"

	"yunque-agent/internal/sbom"
	"yunque-agent/internal/version"
)

// registerSystemRoutes registers system info, metrics, settings, tenants, backup, speech,
// federation, heartbeat, and file upload routes.
func (g *Gateway) registerSystemRoutes() {
	// ── Multi-layer health probes (unauthenticated, K8s-compatible) ──

	// /healthz — backward-compatible simple probe (returns 200 always)
	g.mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		modelHealth := g.planner.ModelRuntimeHealth()
		health := map[string]any{
			"status":        "ok",
			"version":       version.Version,
			"breaker_state": modelHealth.BreakerState,
			"uptime_sec":    int(time.Since(g.startTime).Seconds()),
		}
		if !modelHealth.Configured {
			health["status"] = "degraded"
			health["breaker_state"] = "unconfigured"
		} else if modelHealth.BreakerState == "open" {
			health["status"] = "degraded"
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(health)
	})

	// /livez — liveness probe: process alive, can serve HTTP
	g.mux.HandleFunc("/livez", g.handleLivez)

	// /readyz — readiness probe: dependencies initialized, ready to accept traffic
	g.mux.HandleFunc("/readyz", g.handleReadyz)

	// /sbom — embedded CycloneDX SBOM (unauthenticated, read-only supply chain info)
	g.mux.HandleFunc("/sbom", func(w http.ResponseWriter, r *http.Request) {
		data := sbom.Get()
		if data == nil {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":"no SBOM embedded in this build"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	})

	// /healthz/cognitive — deep cognitive subsystem health (memory, knowledge, LLM, cogni)
	g.mux.HandleFunc("/healthz/cognitive", g.handleCognitiveHealth)
	g.mux.HandleFunc("/v1/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		info := version.Get()
		resp := map[string]any{
			"version":    info.Version,
			"git_commit": info.GitCommit,
			"build_date": info.BuildDate,
			"go_version": info.GoVersion,
			"os":         info.OS,
			"arch":       info.Arch,
		}
		if g.updateChecker != nil {
			if tag, url, hasNew := g.updateChecker(); tag != "" {
				resp["update_available"] = hasNew
				resp["latest_version"] = tag
				resp["latest_url"] = url
			}
		}
		json.NewEncoder(w).Encode(resp)
	})

	// Desktop shell controls (/v1/desktop*) are owned by the desktop pack
	// (internal/packs/desktop), mounted via gw.RegisterModule.

	// System info / metrics / cache stats and tenants migrated to the
	// control-plane pack (internal/packs/controlplane).

	// Settings (env config management + setup check)
	g.mux.HandleFunc("/api/settings/schema", g.requireAuth(g.handleSettingsSchema))
	g.mux.HandleFunc("/api/settings/config", g.requireAuth(g.handleSettingsConfig))
	g.mux.HandleFunc("/api/settings/check", g.requireSetupOrAuth(g.handleSettingsCheck))
	g.mux.HandleFunc("/v1/config/reload", g.requireAuth(g.handleConfigReload))
	g.mux.HandleFunc("/api/settings/detect-dirs", g.requireAuth(g.handleDetectDirs))

	// Tori integration (/v1/tori*) is owned by the Tori pack
	// (internal/packs/tori), mounted via gw.RegisterModule.

	// File upload
	g.mux.HandleFunc("/v1/upload", g.requireAuth(g.handleFileUpload))

	// Speech (/v1/speech*) is owned by the speech pack
	// (internal/packs/speech), mounted via gw.RegisterModule.

	// Heartbeat (/v1/heartbeat*) is owned by the heartbeat pack
	// (internal/packs/heartbeat), mounted via gw.RegisterModule.

	// Federation (/v1/federation*) is owned by the federation pack
	// (internal/packs/federation), mounted via gw.RegisterModule.

	// Modules (/v1/modules) is owned by the modules pack
	// (internal/packs/modules), mounted via gw.RegisterModule.

	// NL Config — natural language → structured configuration
	g.mux.HandleFunc("/v1/nl-config", g.requireAuth(g.handleNLConfig))
	g.mux.HandleFunc("/v1/nl-config/", g.requireAuth(g.handleNLConfig))
}
