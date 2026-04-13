package gateway

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os/exec"
	"runtime"

	"yunque-agent/internal/tori"
)

// handleToriBind starts the OAuth2 PKCE flow to bind a Tori account.
// POST /v1/tori/bind { "tori_url": "https://..." }
func (g *Gateway) handleToriBind(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if g.toriTokenStore == nil {
		writeJSONStatus(w, http.StatusServiceUnavailable, map[string]string{"error": "tori module not initialized"})
		return
	}

	if g.toriTokenStore.IsBound() {
		writeJSONStatus(w, http.StatusConflict, map[string]string{"error": "already bound, unbind first"})
		return
	}

	var body struct {
		ToriURL string `json:"tori_url"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	cfg := tori.DefaultOAuthConfig()
	if body.ToriURL != "" {
		cfg.ToriBaseURL = body.ToriURL
	}

	authorizeURL, resultCh, err := tori.StartBindFlow(context.Background(), cfg)
	if err != nil {
		slog.Error("tori: start bind flow failed", "err", err)
		writeJSONStatus(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	openBrowser(authorizeURL)

	go func() {
		for result := range resultCh {
			if result.Err != nil {
				slog.Error("tori: bind failed", "err", result.Err)
				return
			}
			if result.Token != nil {
			storeURL := body.ToriURL
			if storeURL == "" {
				storeURL = cfg.ToriBaseURL
			}
			if err := g.toriTokenStore.Store(result.Token, result.UserInfo, storeURL); err != nil {
				slog.Error("tori: store token failed", "err", err)
				return
			}
			apiKey := ""
			if result.UserInfo != nil {
				apiKey = result.UserInfo.APIKey
			}
			tori.ApplyLLMConfig(storeURL, apiKey)
				slog.Info("tori: bind successful",
					"user", result.UserInfo.Username,
					"tori_url", body.ToriURL)
			}
		}
	}()

	writeJSONStatus(w, http.StatusOK, map[string]any{
		"status":        "pending",
		"authorize_url": authorizeURL,
		"message":       "Please complete authorization in your browser",
	})
}

// handleToriStatus returns the current Tori binding status.
// GET /v1/tori/status
func (g *Gateway) handleToriStatus(w http.ResponseWriter, r *http.Request) {
	if g.toriTokenStore == nil {
		writeJSONStatus(w, http.StatusOK, tori.BindingStatus{Bound: false})
		return
	}
	writeJSONStatus(w, http.StatusOK, tori.GetBindingStatus(g.toriTokenStore))
}

// handleToriUnbind removes the Tori binding.
// POST /v1/tori/unbind
func (g *Gateway) handleToriUnbind(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if g.toriTokenStore == nil {
		writeJSONStatus(w, http.StatusOK, map[string]string{"status": "not_bound"})
		return
	}

	if !g.toriTokenStore.IsBound() {
		writeJSONStatus(w, http.StatusOK, map[string]string{"status": "not_bound"})
		return
	}

	if err := g.toriTokenStore.Clear(); err != nil {
		slog.Error("tori: clear token failed", "err", err)
		writeJSONStatus(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	tori.RestoreLLMConfig()
	slog.Info("tori: unbound")

	writeJSONStatus(w, http.StatusOK, map[string]string{"status": "unbound"})
}

// handleToriHealth checks if the bound Tori instance is healthy.
// GET /v1/tori/health
func (g *Gateway) handleToriHealth(w http.ResponseWriter, r *http.Request) {
	if g.toriTokenStore == nil || !g.toriTokenStore.IsBound() {
		writeJSONStatus(w, http.StatusOK, map[string]any{"status": "not_bound"})
		return
	}
	t := g.toriTokenStore.Get()
	if t == nil {
		writeJSONStatus(w, http.StatusOK, map[string]any{"status": "not_bound"})
		return
	}
	health, err := tori.CheckHealth(t.ToriBaseURL)
	if err != nil {
		writeJSONStatus(w, http.StatusOK, map[string]any{"status": "unreachable", "error": err.Error()})
		return
	}
	writeJSONStatus(w, http.StatusOK, health)
}

// handleToriUsage returns usage summary from the bound Tori instance.
// GET /v1/tori/usage
func (g *Gateway) handleToriUsage(w http.ResponseWriter, r *http.Request) {
	if g.toriTokenStore == nil || !g.toriTokenStore.IsBound() {
		writeJSONStatus(w, http.StatusOK, map[string]any{"error": "not bound"})
		return
	}
	t := g.toriTokenStore.Get()
	if t == nil {
		writeJSONStatus(w, http.StatusOK, map[string]any{"error": "not bound"})
		return
	}
	usage, err := tori.FetchUsage(t.ToriBaseURL, t.APIKey)
	if err != nil {
		writeJSONStatus(w, http.StatusOK, map[string]any{"error": err.Error()})
		return
	}
	writeJSONStatus(w, http.StatusOK, usage)
}

func writeJSONStatus(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	cmd.Start()
}
