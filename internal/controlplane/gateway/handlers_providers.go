package gateway

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/apperror"
	"yunque-agent/internal/tori"
)

// handleProviderList returns all registered LLM providers.
func (g *Gateway) handleProviderList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "method not allowed")
		return
	}
	if g.providerReg == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"providers": []any{},
			"warning":   "LLM 提供商尚未配置，请前往设置页面或使用设置向导完成初始化",
		})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"providers": g.providerReg.List(),
		"mode":      g.providerReg.Mode(),
	})
}

// handleProviderTest tests connectivity of a provider.
func (g *Gateway) handleProviderTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "method not allowed")
		return
	}
	if g.providerReg == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "provider registry not available")
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid request body")
		return
	}
	if req.ID == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "provider id is required")
		return
	}
	if err := g.providerReg.TestProvider(r.Context(), req.ID); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   err.Error(),
		})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"success": true,
	})
}

// handleProviderEnable enables a provider.
func (g *Gateway) handleProviderEnable(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "method not allowed")
		return
	}
	if g.providerReg == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "provider registry not available")
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid request body")
		return
	}
	if err := g.providerReg.Enable(req.ID); err != nil {
		apperror.WriteCode(w, apperror.CodeNotFound, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

// handleProviderDisable disables a provider.
func (g *Gateway) handleProviderDisable(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "method not allowed")
		return
	}
	if g.providerReg == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "provider registry not available")
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid request body")
		return
	}
	if err := g.providerReg.Disable(req.ID); err != nil {
		apperror.WriteCode(w, apperror.CodeNotFound, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

// handleProviderSwitchModel changes the model of a provider at runtime.
func (g *Gateway) handleProviderSwitchModel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "method not allowed")
		return
	}
	if g.providerReg == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "provider registry not available")
		return
	}
	var req struct {
		ID    string `json:"id"`
		Model string `json:"model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid request body")
		return
	}
	if req.ID == "" || req.Model == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "id and model are required")
		return
	}
	if err := g.providerReg.SwitchModel(req.ID, req.Model); err != nil {
		apperror.WriteCode(w, apperror.CodeNotFound, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"ok": true, "model": req.Model})
}

// handleProviderSessionOverride sets/clears a session-level provider override.
func (g *Gateway) handleProviderSessionOverride(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "method not allowed")
		return
	}
	if g.providerReg == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "provider registry not available")
		return
	}
	var req struct {
		SessionID  string `json:"session_id"`
		ProviderID string `json:"provider_id"` // empty = clear override
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid request body")
		return
	}
	if req.SessionID == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "session_id is required")
		return
	}
	g.providerReg.SetSessionProvider(req.SessionID, req.ProviderID)
	action := "set"
	if req.ProviderID == "" {
		action = "cleared"
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"ok": true, "action": action})
}

// handleLocalDiscover probes a local LLM backend and returns available models.
func (g *Gateway) handleLocalDiscover(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "method not allowed")
		return
	}
	var req struct {
		BaseURL string `json:"base_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.BaseURL == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "base_url is required")
		return
	}
	result := llm.ProbeLocal(r.Context(), req.BaseURL)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// handleLocalRegister registers a local backend as a provider on-the-fly.
func (g *Gateway) handleLocalRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "method not allowed")
		return
	}
	if g.providerReg == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "provider registry not available")
		return
	}
	var req struct {
		BaseURL string `json:"base_url"`
		Model   string `json:"model"`
		Tier    string `json:"tier"`
		Backend string `json:"backend"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.BaseURL == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "base_url is required")
		return
	}
	cfg := llm.LocalAutoConfig{
		BaseURL: req.BaseURL,
		Model:   req.Model,
		Tier:    req.Tier,
		Backend: llm.LocalBackend(req.Backend),
	}
	pid, err := llm.AutoRegisterLocal(r.Context(), g.providerReg, cfg)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"ok": true, "provider_id": pid})
}

// handleProviderMode gets or sets the provider routing mode.
// GET  → returns current mode
// POST { "mode": "local"|"tori"|"hybrid" } → sets mode
func (g *Gateway) handleProviderMode(w http.ResponseWriter, r *http.Request) {
	if g.providerReg == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "provider registry not available")
		return
	}

	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"mode":  g.providerReg.Mode(),
			"bound": g.toriTokenStore != nil && g.toriTokenStore.IsBound(),
		})

	case http.MethodPost:
		var req struct {
			Mode string `json:"mode"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, "invalid body")
			return
		}
		switch llm.ProviderMode(req.Mode) {
		case llm.ProviderModeLocal, llm.ProviderModeTori, llm.ProviderModeHybrid:
			g.providerReg.SetMode(llm.ProviderMode(req.Mode))
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"ok": true, "mode": req.Mode})
		default:
			apperror.WriteCode(w, apperror.CodeBadRequest, "mode must be local, tori, or hybrid")
		}

	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "method not allowed")
	}
}

// handleProviderPresets returns all built-in provider preset templates.
func (g *Gateway) handleProviderPresets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "method not allowed")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"presets": llm.Presets(),
	})
}

// handleProviderRegister registers a new provider from a preset or custom config.
// POST { "preset_id": "deepseek", "api_key": "sk-...", "model": "deepseek-chat" }
// or   { "base_url": "https://custom.api/v1", "api_key": "...", "model": "...", "name": "..." }
func (g *Gateway) handleProviderRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "method not allowed")
		return
	}
	if g.providerReg == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "provider registry not available")
		return
	}

	var req struct {
		PresetID string `json:"preset_id"`
		BaseURL  string `json:"base_url"`
		APIKey   string `json:"api_key"`
		Model    string `json:"model"`
		Name     string `json:"name"`
		Tier     string `json:"tier"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid body")
		return
	}

	cfg := llm.ProviderConfig{
		Type:    llm.ProviderTypeChat,
		Source:  llm.ProviderSourceDirect,
		Enabled: true,
	}

	if req.PresetID != "" {
		preset := llm.PresetByID(req.PresetID)
		if preset == nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, "unknown preset: "+req.PresetID)
			return
		}
		cfg.ID = req.PresetID + "-" + req.Model
		cfg.DisplayName = preset.Name
		cfg.BaseURL = preset.BaseURL
		cfg.PresetID = req.PresetID
		cfg.Dialect = preset.Dialect
		if req.Model == "" && len(preset.Models) > 0 {
			req.Model = preset.Models[0].ID
			cfg.Tier = preset.Models[0].Tier
			cfg.Capabilities = preset.Models[0].Capabilities
		}
		// Inherit capabilities from matching preset model
		for _, pm := range preset.Models {
			if pm.ID == req.Model {
				if cfg.Tier == "" {
					cfg.Tier = pm.Tier
				}
				cfg.Capabilities = pm.Capabilities
				break
			}
		}
	}

	if req.BaseURL != "" {
		cfg.BaseURL = req.BaseURL
	}
	if req.Name != "" {
		cfg.DisplayName = req.Name
	}
	if req.Model != "" {
		cfg.Model = req.Model
	}
	if req.APIKey != "" {
		cfg.APIKeys = []string{req.APIKey}
	}
	if req.Tier != "" {
		cfg.Tier = req.Tier
	}
	if cfg.ID == "" {
		cfg.ID = "custom-" + req.Model
	}

	if cfg.BaseURL == "" || cfg.Model == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "base_url and model are required")
		return
	}

	if err := g.providerReg.Register(cfg); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, err.Error())
		return
	}

	slog.Info("provider registered", "id", cfg.ID, "source", cfg.Source, "model", cfg.Model)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"ok": true, "provider_id": cfg.ID})
}

// handleProviderDelete removes a provider from the registry.
func (g *Gateway) handleProviderDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "method not allowed")
		return
	}
	if g.providerReg == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "provider registry not available")
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "id is required")
		return
	}
	if err := g.providerReg.Delete(req.ID); err != nil {
		apperror.WriteCode(w, apperror.CodeNotFound, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

// handleToriDiscover discovers available models from the bound Tori instance
// and optionally auto-registers them.
func (g *Gateway) handleToriDiscover(w http.ResponseWriter, r *http.Request) {
	if g.toriTokenStore == nil || !g.toriTokenStore.IsBound() {
		apperror.WriteCode(w, apperror.CodeBadRequest, "not bound to Tori")
		return
	}

	t := g.toriTokenStore.Get()
	models, err := tori.DiscoverModels(t.ToriBaseURL, t.APIKey)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok":    false,
			"error": err.Error(),
		})
		return
	}

	autoRegister := r.URL.Query().Get("register") == "true"
	registered := 0

	if autoRegister && g.providerReg != nil {
		for _, m := range models {
			cfg := llm.ProviderConfig{
				ID:          "tori-" + m.ID,
				DisplayName: "Tori: " + m.ID,
				Type:        llm.ProviderTypeChat,
				Source:      llm.ProviderSourceTori,
				BaseURL:     t.ToriBaseURL + "/v1",
				APIKeys:     []string{t.APIKey},
				Model:       m.ID,
				Enabled:     true,
				Priority:    100,
			}
			if err := g.providerReg.Register(cfg); err == nil {
				registered++
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"ok":         true,
		"models":     models,
		"registered": registered,
	})
}

// handleExecProvider gets or sets the exec-layer LLM provider.
// GET  → returns current exec provider
// POST { "provider_id": "moonshot-kimi-k2.5" } → sets exec provider
func (g *Gateway) handleExecProvider(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		pid := g.ExecProvider()
		w.Header().Set("Content-Type", "application/json")
		var providers []string
		if g.providerReg != nil {
			for _, p := range g.providerReg.List() {
				providers = append(providers, p.ID)
			}
		}
		json.NewEncoder(w).Encode(map[string]any{
			"exec_provider":      pid,
			"available_providers": providers,
		})

	case http.MethodPost:
		var req struct {
			ProviderID string `json:"provider_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, "invalid body")
			return
		}
		g.SetExecProvider(req.ProviderID)
		slog.Info("exec provider updated via API", "provider", req.ProviderID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "exec_provider": req.ProviderID})

	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "method not allowed")
	}
}

// handleBreakerReset manually resets all LLM circuit breakers.
func (g *Gateway) handleBreakerReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "method not allowed")
		return
	}
	count := 0
	if g.providerReg != nil {
		count = g.providerReg.ResetAllBreakers()
	}
	slog.Info("breaker reset: all circuit breakers cleared", "providers", count)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"ok": true, "reset_count": count})
}
