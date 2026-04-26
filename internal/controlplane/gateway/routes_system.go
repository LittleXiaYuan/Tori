package gateway

import (
	"encoding/json"
	"net/http"
	"time"

	"yunque-agent/internal/apperror"
	"yunque-agent/internal/desktop"
	"yunque-agent/internal/version"
)

// registerSystemRoutes registers system info, metrics, settings, tenants, backup, speech,
// federation, heartbeat, and file upload routes.
func (g *Gateway) registerSystemRoutes() {
	// Health & version (unauthenticated)
	g.mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		breaker := g.planner.LLMBreaker()
		health := map[string]any{
			"status":        "ok",
			"version":       version.Version,
			"breaker_state": breaker.State(),
			"uptime_sec":    int(time.Since(g.startTime).Seconds()),
		}
		if breaker.State() == "open" {
			health["status"] = "degraded"
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(health)
	})
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

	// System info & metrics
	g.mux.HandleFunc("/v1/system/info", g.requireAuth(g.handleSystemInfo))
	g.mux.HandleFunc("/v1/system/stats", g.requireAuth(g.handleSystemStats))
	g.mux.HandleFunc("/v1/metrics", g.requireAuth(g.handleMetrics))
	g.mux.HandleFunc("/v1/metrics/prometheus", g.requireAuth(g.handleMetricsPrometheus))
	g.mux.HandleFunc("/v1/cache/stats", g.requireAuth(g.handleCacheStats))

	// Tenants
	g.mux.HandleFunc("/v1/tenants", g.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			g.handleCreateTenant(w, r)
		case http.MethodGet:
			g.handleListTenants(w, r)
		default:
			apperror.WriteCode(w, apperror.CodeMethodNotAllow, "method not allowed")
		}
	}))

	// Settings (env config management + setup check)
	g.mux.HandleFunc("/api/settings/schema", g.requireAuth(g.handleSettingsSchema))
	g.mux.HandleFunc("/api/settings/config", g.requireAuth(g.handleSettingsConfig))
	g.mux.HandleFunc("/api/settings/check", g.requireSetupOrAuth(g.handleSettingsCheck))
	g.mux.HandleFunc("/v1/config/reload", g.requireAuth(g.handleConfigReload))
	g.mux.HandleFunc("/api/settings/detect-dirs", g.requireAuth(g.handleDetectDirs))

	// Backup & Restore
	g.mux.HandleFunc("/v1/backup/export", g.requireAuth(g.handleBackupExport))
	g.mux.HandleFunc("/v1/backup/import", g.requireAuth(g.handleBackupImport))
	g.mux.HandleFunc("/v1/backup/info", g.requireAuth(g.handleBackupInfo))

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

	// Heartbeat
	g.mux.HandleFunc("/v1/heartbeat", g.requireAuth(g.handleHeartbeat))
	g.mux.HandleFunc("/v1/heartbeat/trigger", g.requireAuth(g.handleHeartbeatTrigger))
	g.mux.HandleFunc("/v1/heartbeat/logs", g.requireAuth(g.handleHeartbeatLogs))

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

	// Cogni declarative AI-cognition shells (hot-pluggable)
	g.mux.HandleFunc("/v1/cognis", g.requireAuth(g.handleCognis))
	g.mux.HandleFunc("/v1/cognis/", g.requireAuth(g.handleCognis))
}
