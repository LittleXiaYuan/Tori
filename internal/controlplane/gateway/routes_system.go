package gateway

import (
	"encoding/json"
	"net/http"
	"time"

	"yunque-agent/internal/desktop"
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

	// Desktop controls (console toggle; Windows only, no-op on other platforms)
	g.mux.HandleFunc("/v1/desktop/console", g.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodPost:
			hidden := desktop.ToggleConsole()
			json.NewEncoder(w).Encode(map[string]any{"console_hidden": hidden})
		default:
			json.NewEncoder(w).Encode(map[string]any{"console_hidden": desktop.IsConsoleHidden()})
		}
	}))

	// Auto-start toggle (Windows registry, no-op on other platforms)
	g.mux.HandleFunc("/v1/desktop/autostart", g.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodPost:
			enabled := !desktop.IsAutoStartEnabled()
			if err := desktop.SetAutoStart(enabled); err != nil {
				json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"autostart_enabled": enabled})
		default:
			json.NewEncoder(w).Encode(map[string]any{"autostart_enabled": desktop.IsAutoStartEnabled()})
		}
	}))

	// System info / metrics / cache stats and tenants migrated to the
	// control-plane pack (internal/packs/controlplane).

	// Settings (env config management + setup check)
	g.mux.HandleFunc("/api/settings/schema", g.requireAuth(g.handleSettingsSchema))
	g.mux.HandleFunc("/api/settings/config", g.requireAuth(g.handleSettingsConfig))
	g.mux.HandleFunc("/api/settings/check", g.requireSetupOrAuth(g.handleSettingsCheck))
	g.mux.HandleFunc("/v1/config/reload", g.requireAuth(g.handleConfigReload))
	g.mux.HandleFunc("/api/settings/detect-dirs", g.requireAuth(g.handleDetectDirs))

	// Tori Integration (OAuth2 bind/unbind + health/usage)
	g.mux.HandleFunc("/v1/tori/bind", g.requireAuth(g.handleToriBind))
	g.mux.HandleFunc("/v1/tori/status", g.requireAuth(g.handleToriStatus))
	g.mux.HandleFunc("/v1/tori/unbind", g.requireAuth(g.handleToriUnbind))
	g.mux.HandleFunc("/v1/tori/health", g.requireAuth(g.handleToriHealth))
	g.mux.HandleFunc("/v1/tori/usage", g.requireAuth(g.handleToriUsage))

	// File upload
	g.mux.HandleFunc("/v1/upload", g.requireAuth(g.handleFileUpload))

	// Speech (TTS / STT)
	g.mux.HandleFunc("/v1/speech/tts", g.requireAuth(g.handleTTS))
	g.mux.HandleFunc("/v1/speech/stt", g.requireAuth(g.handleSTT))
	g.mux.HandleFunc("/v1/speech/stt/stream", g.requireAuth(g.handleSTTStream))
	g.mux.HandleFunc("/v1/speech/voices", g.requireAuth(g.handleVoices))

	// Heartbeat (/v1/heartbeat*) is owned by the heartbeat pack
	// (internal/packs/heartbeat), mounted via gw.RegisterModule.

	// Federation (legacy)
	g.mux.HandleFunc("/v1/federation/peers", g.requireAuth(g.handleFedPeers))
	g.mux.HandleFunc("/v1/federation/stats", g.requireAuth(g.handleFedStats))

	// Federation OPP v3 (model-aware A2A)
	g.mux.HandleFunc("/v1/federation/capabilities", g.requireAuth(g.handleFedCapabilities))
	g.mux.HandleFunc("/v1/federation/discover", g.requireAuth(g.handleFedDiscover))
	g.mux.HandleFunc("/v1/federation/delegate", g.requireAuth(g.handleFedDelegate))
	g.mux.HandleFunc("/v1/federation/bridge/stats", g.requireAuth(g.handleFedBridgeStats))
	g.mux.HandleFunc("/v1/federation/broadcast", g.requireAuth(g.handleFedBroadcast))
	if g.fedTransport != nil {
		g.mux.HandleFunc("/v1/federation/receive", g.fedTransport.HTTPHandler())
	}

	// Modules (hot-pluggable subsystems)
	g.mux.HandleFunc("/v1/modules", g.requireAuth(g.handleModules))

	// NL Config — natural language → structured configuration
	g.mux.HandleFunc("/v1/nl-config", g.requireAuth(g.handleNLConfig))
	g.mux.HandleFunc("/v1/nl-config/", g.requireAuth(g.handleNLConfig))
}
