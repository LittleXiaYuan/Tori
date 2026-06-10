package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"yunque-agent/internal/config"
	"yunque-agent/internal/execution/sandbox"
)

// Setup & Onboarding API Handlers.
//
// Endpoints:
//   GET  /v1/setup/detect        detect environment (OS, GPU, Docker, LLM providers)
//   GET  /v1/setup/health        health check configured providers
//   GET  /v1/setup/templates     list scenario templates
//   POST /v1/setup/test-provider test a provider connection from the backend
//   POST /v1/setup/apply         apply a scenario template

// onboardingKVStore abstracts Ledger KV (namespace baked in) to avoid import
// cycles — same shape as the auth/model KV stores.
type onboardingKVStore interface {
	Put(ctx context.Context, key string, value any) error
	Get(ctx context.Context, key string, dest any) (bool, error)
}

// onboardingState is the server-persisted first-run guide state. Storing it in
// the Ledger (not browser localStorage) means the guide is shown exactly once
// per install and stays consistent across web/desktop and device changes.
type onboardingState struct {
	Completed   bool   `json:"completed"`
	CompletedAt string `json:"completed_at,omitempty"`
}

// handleOnboardingState reads (GET) or records (POST {completed:true}) whether
// the first-run onboarding guide has been completed. Falls back gracefully to
// "not completed" when no KV is wired so the client can still use localStorage.
func (g *Gateway) handleOnboardingState(w http.ResponseWriter, r *http.Request) {
	if g.onboardingKV == nil {
		writeJSON(w, onboardingState{})
		return
	}
	ctx := r.Context()
	switch r.Method {
	case http.MethodGet:
		var st onboardingState
		if _, err := g.onboardingKV.Get(ctx, "state", &st); err != nil {
			slog.Warn("onboarding: kv get failed", "err", err)
		}
		writeJSON(w, st)
	case http.MethodPost:
		var req onboardingState
		_ = json.NewDecoder(r.Body).Decode(&req)
		st := onboardingState{Completed: req.Completed}
		if req.Completed {
			st.CompletedAt = time.Now().UTC().Format(time.RFC3339)
		}
		if err := g.onboardingKV.Put(ctx, "state", st); err != nil {
			slog.Warn("onboarding: kv put failed", "err", err)
			http.Error(w, "persist failed", http.StatusInternalServerError)
			return
		}
		writeJSON(w, st)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleSetupDetect performs environment detection and returns the result.
func (g *Gateway) handleSetupDetect(w http.ResponseWriter, r *http.Request) {
	cfg := config.Load()
	result := config.DetectEnvironment(cfg)
	if err := config.SaveSetupResult(result); err != nil {
		slog.Warn("setup: failed to save result", "err", err)
	}
	writeJSON(w, result)
}

// handleSetupHealth checks all configured LLM provider connections.
func (g *Gateway) handleSetupHealth(w http.ResponseWriter, r *http.Request) {
	cfg := config.Load()
	result := config.DetectEnvironment(cfg)
	writeJSON(w, map[string]any{
		"providers":  result.Providers,
		"has_docker": result.HasDocker,
		"has_gpu":    result.HasGPU,
		"has_ollama": result.HasOllama,
	})
}

// handleSetupTemplates returns available scenario templates.
func (g *Gateway) handleSetupTemplates(w http.ResponseWriter, r *http.Request) {
	templates := config.BuiltinTemplates()
	writeJSON(w, map[string]any{
		"templates": templates,
		"count":     len(templates),
	})
}

// handleSetupTestProvider tests a provider using backend-side networking.
func (g *Gateway) handleSetupTestProvider(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"POST only"}`, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		BaseURL string `json:"base_url"`
		APIKey  string `json:"api_key"`
		Model   string `json:"model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	req.BaseURL = strings.TrimSpace(req.BaseURL)
	req.APIKey = strings.TrimSpace(req.APIKey)
	req.Model = strings.TrimSpace(req.Model)
	if req.BaseURL == "" {
		http.Error(w, `{"error":"base_url is required"}`, http.StatusBadRequest)
		return
	}

	provider := config.TestProviderConnection(req.BaseURL, req.APIKey, req.Model)
	writeJSON(w, map[string]any{
		"ok":       provider.Available,
		"provider": provider,
	})
}

// handleSetupApply applies a scenario template, persists the generated env,
// and also returns the rendered env content for preview/debugging.
func (g *Gateway) handleSetupApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"POST only"}`, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		TemplateID string         `json:"template_id"`
		APIKey     string         `json:"api_key"`
		BaseURL    string         `json:"base_url"`
		Model      string         `json:"model"`
		Overrides  map[string]any `json:"overrides"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	req.TemplateID = strings.TrimSpace(req.TemplateID)
	req.APIKey = firstNonEmptyString(req.APIKey, stringFromMap(req.Overrides, "api_key"))
	req.BaseURL = strings.TrimSpace(firstNonEmptyString(req.BaseURL, stringFromMap(req.Overrides, "base_url")))
	req.Model = strings.TrimSpace(firstNonEmptyString(req.Model, stringFromMap(req.Overrides, "model")))

	if req.TemplateID == "" {
		http.Error(w, `{"error":"template_id is required"}`, http.StatusBadRequest)
		return
	}
	if req.BaseURL == "" {
		http.Error(w, `{"error":"base_url is required"}`, http.StatusBadRequest)
		return
	}
	if req.Model == "" {
		http.Error(w, `{"error":"model is required"}`, http.StatusBadRequest)
		return
	}

	var selected *config.ScenarioTemplate
	for _, t := range config.BuiltinTemplates() {
		if t.ID == req.TemplateID {
			selected = &t
			break
		}
	}
	if selected == nil {
		http.Error(w, `{"error":"template not found"}`, http.StatusNotFound)
		return
	}

	envContent := config.GenerateEnvFile(*selected, req.APIKey, req.BaseURL, req.Model)
	values := readEnvFile()
	values["LLM_BASE_URL"] = req.BaseURL
	values["LLM_MODEL"] = req.Model
	if req.APIKey != "" || values["LLM_API_KEY"] == "" {
		values["LLM_API_KEY"] = req.APIKey
	}
	if values["AGENT_ADDR"] == "" {
		values["AGENT_ADDR"] = ":9090"
	}
	values["SANDBOX_TIER"] = selected.SandboxTier
	for k, v := range selected.EnvVars {
		values[k] = v
	}

	if err := writeEnvFile(values); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to write .env: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{
		"ok":               true,
		"status":           "applied",
		"applied":          selected.ID,
		"persisted":        true,
		"restart_required": true,
		"template":         selected,
		"env_content":      envContent,
		"message":          "Configuration was saved to .env. Restart the service or reload config from Settings.",
	})
}

func stringFromMap(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	raw, ok := values[key]
	if !ok || raw == nil {
		return ""
	}
	s, _ := raw.(string)
	return s
}

func firstNonEmptyString(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// handleInstallComponent installs an optional component on-demand.
// POST /v1/setup/install-component { "component_id": "python_office" }
// Supports SSE streaming via Accept: text/event-stream header for real-time progress.
func (g *Gateway) handleInstallComponent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"POST only"}`, http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ComponentID string `json:"component_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	useSSE := r.Header.Get("Accept") == "text/event-stream"

	switch req.ComponentID {
	case "python_office":
		pyEnv := sandbox.NewPythonEnv("data")
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
		defer cancel()

		if useSSE {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			flusher, _ := w.(http.Flusher)

			sendSSE := func(p sandbox.InstallProgress) {
				data, _ := json.Marshal(p)
				fmt.Fprintf(w, "data: %s\n\n", data)
				if flusher != nil {
					flusher.Flush()
				}
			}

			if err := pyEnv.EnsureEmbeddedWithProgress(ctx, sendSSE); err != nil {
				errData, _ := json.Marshal(map[string]any{"stage": "error", "detail": err.Error()})
				fmt.Fprintf(w, "data: %s\n\n", errData)
				if flusher != nil {
					flusher.Flush()
				}
				return
			}
			return
		}

		if err := pyEnv.EnsureEmbedded(ctx); err != nil {
			slog.Error("install python_office failed", "err", err)
			writeJSON(w, map[string]any{"success": false, "error": err.Error()})
			return
		}
		slog.Info("python_office installed successfully")
		writeJSON(w, map[string]any{"success": true, "message": "Python Office ??????"})
	default:
		writeJSON(w, map[string]any{"success": false, "error": "??????????: " + req.ComponentID})
	}
}
