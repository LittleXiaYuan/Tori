package gateway

import (
	"encoding/json"
	"net/http"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/apperror"
)

// handleProviderList returns all registered LLM providers.
func (g *Gateway) handleProviderList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "method not allowed")
		return
	}
	if g.providerReg == nil {
		json.NewEncoder(w).Encode(map[string]any{"providers": []any{}})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"providers": g.providerReg.List(),
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
