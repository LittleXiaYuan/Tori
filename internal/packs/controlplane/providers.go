package controlplanepack

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/appdir"
	"yunque-agent/internal/apperror"
	"yunque-agent/internal/tori"
)

func (h *Handler) providerGateway() providerGateway {
	g, _ := h.gateway.(providerGateway)
	return g
}

func (h *Handler) providerRegistry() *llm.ProviderRegistry {
	g := h.providerGateway()
	if g == nil {
		return nil
	}
	return g.ProviderRegistry()
}

func (h *Handler) handleProviderList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "method not allowed")
		return
	}
	reg := h.providerRegistry()
	if reg == nil {
		writeJSON(w, map[string]any{
			"providers": []any{},
			"warning":   "LLM 提供商尚未配置，请前往设置页面或使用设置向导完成初始化",
		})
		return
	}
	writeJSON(w, map[string]any{
		"providers": reg.List(),
		"mode":      reg.Mode(),
	})
}

func (h *Handler) handleProviderTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "method not allowed")
		return
	}
	reg := h.providerRegistry()
	if reg == nil {
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
	if err := reg.TestProvider(r.Context(), req.ID); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   err.Error(),
		})
		return
	}
	writeJSON(w, map[string]any{"success": true})
}

func (h *Handler) handleProviderEnable(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "method not allowed")
		return
	}
	reg := h.providerRegistry()
	if reg == nil {
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
	if err := reg.Enable(req.ID); err != nil {
		apperror.WriteCode(w, apperror.CodeNotFound, err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (h *Handler) handleProviderDisable(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "method not allowed")
		return
	}
	reg := h.providerRegistry()
	if reg == nil {
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
	if err := reg.Disable(req.ID); err != nil {
		apperror.WriteCode(w, apperror.CodeNotFound, err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (h *Handler) handleProviderSwitchModel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "method not allowed")
		return
	}
	reg := h.providerRegistry()
	if reg == nil {
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
	if err := reg.SwitchModel(req.ID, req.Model); err != nil {
		apperror.WriteCode(w, apperror.CodeNotFound, err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true, "model": req.Model})
}

func (h *Handler) handleProviderSessionOverride(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "method not allowed")
		return
	}
	reg := h.providerRegistry()
	if reg == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "provider registry not available")
		return
	}
	var req struct {
		SessionID  string `json:"session_id"`
		ProviderID string `json:"provider_id"`
		// Mode selects 小羽模式("xiaoyu", default) or API模式("api"). Distinct
		// from ProviderID (which model answers) — Mode gates whether this
		// session's turns feed self-distill collection and how many
		// concurrent async sub-agent handoffs it may run. See
		// ProviderRegistry.SetSessionMode.
		Mode string `json:"mode,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid request body")
		return
	}
	if req.SessionID == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "session_id is required")
		return
	}
	if req.ProviderID != "" {
		p := reg.Get(req.ProviderID)
		if p == nil || !p.Enabled() {
			apperror.WriteCode(w, apperror.CodeBadRequest, "provider is not available")
			return
		}
	}
	modeOnly := req.ProviderID == "" && (req.Mode == "xiaoyu" || req.Mode == "api")
	action := "set"
	if modeOnly {
		// A mode-only switch (小羽↔API toggle with no model change) must not
		// clear an existing provider override — only an explicit empty
		// provider_id with no mode does that.
		action = "unchanged"
	} else {
		reg.SetSessionProvider(req.SessionID, req.ProviderID)
		if req.ProviderID == "" {
			action = "cleared"
		}
	}
	if req.Mode == "xiaoyu" || req.Mode == "api" {
		reg.SetSessionMode(req.SessionID, req.Mode)
	}
	writeJSON(w, map[string]any{"ok": true, "action": action, "mode": reg.SessionMode(req.SessionID)})
}

func (h *Handler) handleLocalDiscover(w http.ResponseWriter, r *http.Request) {
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
	writeJSON(w, llm.ProbeLocal(r.Context(), req.BaseURL))
}

func (h *Handler) handleLocalRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "method not allowed")
		return
	}
	reg := h.providerRegistry()
	if reg == nil {
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
	pid, err := llm.AutoRegisterLocal(r.Context(), reg, cfg)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true, "provider_id": pid})
}

func (h *Handler) handleProviderDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "method not allowed")
		return
	}
	reg := h.providerRegistry()
	if reg == nil {
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
	if err := reg.Delete(req.ID); err != nil {
		apperror.WriteCode(w, apperror.CodeNotFound, err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (h *Handler) handleToriDiscover(w http.ResponseWriter, r *http.Request) {
	g := h.providerGateway()
	if g == nil || g.ToriTokenStore() == nil || !g.ToriTokenStore().IsBound() {
		apperror.WriteCode(w, apperror.CodeBadRequest, "not bound to Tori")
		return
	}

	t := g.ToriTokenStore().Get()
	models, err := tori.DiscoverModels(t.ToriBaseURL, t.APIKey)
	if err != nil {
		writeJSON(w, map[string]any{
			"ok":    false,
			"error": err.Error(),
		})
		return
	}

	autoRegister := r.URL.Query().Get("auto_register") == "true" || r.URL.Query().Get("register") == "true"
	registered := 0
	if autoRegister {
		if reg := h.providerRegistry(); reg != nil {
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
				if err := reg.Register(cfg); err == nil {
					registered++
				}
			}
		}
	}

	writeJSON(w, map[string]any{
		"ok":         true,
		"models":     models,
		"registered": registered,
	})
}

func (h *Handler) handleExecProvider(w http.ResponseWriter, r *http.Request) {
	g := h.providerGateway()
	if g == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "provider gateway not configured")
		return
	}
	reg := h.providerRegistry()

	switch r.Method {
	case http.MethodGet:
		var providers []string
		if reg != nil {
			for _, p := range reg.List() {
				providers = append(providers, p.ID)
			}
		}
		writeJSON(w, map[string]any{
			"exec_provider":       g.ExecProvider(),
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
		if req.ProviderID != "" && req.ProviderID != "smart" {
			if reg == nil {
				apperror.WriteCode(w, apperror.CodeBadRequest, "provider is not available")
				return
			}
			p := reg.Get(req.ProviderID)
			if p == nil || !p.Enabled() {
				apperror.WriteCode(w, apperror.CodeBadRequest, "provider is not available")
				return
			}
		}
		g.SetExecProvider(req.ProviderID)
		_ = os.WriteFile(appdir.File("exec_provider.json"), []byte(`{"provider_id":`+strconvQuote(req.ProviderID)+`}`), 0o600)
		slog.Info("exec provider updated via API", "provider", req.ProviderID)
		writeJSON(w, map[string]any{"ok": true, "exec_provider": req.ProviderID})

	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "method not allowed")
	}
}

func strconvQuote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// handleImageGenProvider lets settings pin which provider GetImageGenerator
// uses, independent of the global exec (chat) provider — image generation
// and chat may reasonably use different providers.
func (h *Handler) handleImageGenProvider(w http.ResponseWriter, r *http.Request) {
	reg := h.providerRegistry()
	if reg == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "provider registry not configured")
		return
	}

	switch r.Method {
	case http.MethodGet:
		writeJSON(w, map[string]any{
			"image_gen_provider":  reg.ImageGenProvider(),
			"available_providers": reg.ImageGenCapableProviders(),
		})

	case http.MethodPost:
		var req struct {
			ProviderID string `json:"provider_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, "invalid body")
			return
		}
		if req.ProviderID != "" {
			p := reg.Get(req.ProviderID)
			if p == nil || !p.Enabled() {
				apperror.WriteCode(w, apperror.CodeBadRequest, "provider is not available")
				return
			}
		}
		reg.SetImageGenProvider(req.ProviderID)
		_ = os.WriteFile(appdir.File("image_gen_provider.json"), []byte(`{"provider_id":`+strconvQuote(req.ProviderID)+`}`), 0o600)
		slog.Info("image gen provider updated via API", "provider", req.ProviderID)
		writeJSON(w, map[string]any{"ok": true, "image_gen_provider": req.ProviderID})

	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "method not allowed")
	}
}

func (h *Handler) handleRouterStats(w http.ResponseWriter, r *http.Request) {
	g := h.providerGateway()
	if g == nil || g.SmartRouter() == nil {
		writeJSON(w, map[string]string{"status": "not configured"})
		return
	}
	writeJSON(w, map[string]any{
		"slots": g.SmartRouter().GetSlots(),
		"stats": g.SmartRouter().GetStats(),
	})
}

func (h *Handler) handleBreakerReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "method not allowed")
		return
	}
	count := 0
	if reg := h.providerRegistry(); reg != nil {
		count = reg.ResetAllBreakers()
	}
	slog.Info("breaker reset: all circuit breakers cleared", "providers", count)
	writeJSON(w, map[string]any{"ok": true, "reset_count": count})
}
