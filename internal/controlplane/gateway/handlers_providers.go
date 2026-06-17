package gateway

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/apperror"
)

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
		cfg.DisplayName = preset.Name
		cfg.BaseURL = preset.BaseURL
		cfg.PresetID = req.PresetID
		cfg.Dialect = preset.Dialect
		// Fill default model BEFORE generating provider ID
		if req.Model == "" && len(preset.Models) > 0 {
			req.Model = preset.Models[0].ID
			cfg.Tier = preset.Models[0].Tier
			cfg.Capabilities = preset.Models[0].Capabilities
		}
		cfg.ID = req.PresetID + "-" + req.Model
		for _, pm := range preset.Models {
			if pm.ID == req.Model {
				if cfg.Tier == "" {
					cfg.Tier = pm.Tier
				}
				cfg.Capabilities = pm.Capabilities
				if pm.ContextWindow > 0 {
					cfg.ContextWindow = pm.ContextWindow
				}
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
