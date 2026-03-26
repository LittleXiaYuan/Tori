package gateway

import (
	"encoding/json"
	"net/http"
	"time"

	"yunque-agent/internal/apperror"
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
		json.NewEncoder(w).Encode(version.Get())
	})

	// System info & metrics
	g.mux.HandleFunc("/v1/system/info", g.requireAuth(g.handleSystemInfo))
	g.mux.HandleFunc("/v1/system/stats", g.requireAuth(g.handleSystemStats))
	g.mux.HandleFunc("/v1/metrics", g.requireAuth(g.handleMetrics))
	g.mux.HandleFunc("/v1/metrics/prometheus", g.handleMetricsPrometheus)
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
	g.mux.HandleFunc("/api/settings/check", g.handleSettingsCheck) // no auth — needed for first-run setup

	// Backup & Restore
	g.mux.HandleFunc("/v1/backup/export", g.requireAuth(g.handleBackupExport))
	g.mux.HandleFunc("/v1/backup/import", g.requireAuth(g.handleBackupImport))
	g.mux.HandleFunc("/v1/backup/info", g.requireAuth(g.handleBackupInfo))

	// File upload
	g.mux.HandleFunc("/v1/upload", g.requireAuth(g.handleFileUpload))

	// Speech (TTS / STT)
	g.mux.HandleFunc("/v1/speech/tts", g.requireAuth(g.handleTTS))
	g.mux.HandleFunc("/v1/speech/stt", g.requireAuth(g.handleSTT))
	g.mux.HandleFunc("/v1/speech/voices", g.requireAuth(g.handleVoices))

	// Heartbeat
	g.mux.HandleFunc("/v1/heartbeat", g.requireAuth(g.handleHeartbeat))
	g.mux.HandleFunc("/v1/heartbeat/trigger", g.requireAuth(g.handleHeartbeatTrigger))
	g.mux.HandleFunc("/v1/heartbeat/logs", g.requireAuth(g.handleHeartbeatLogs))

	// Federation
	g.mux.HandleFunc("/v1/federation/peers", g.requireAuth(g.handleFedPeers))
	g.mux.HandleFunc("/v1/federation/stats", g.requireAuth(g.handleFedStats))
}
