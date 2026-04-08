package gateway

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"yunque-agent/internal/apperror"
)

// handleBrowserStatus returns the browser extension connection status.
func (g *Gateway) handleBrowserStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	hub := g.browserHub
	tid := tenantFromCtx(r.Context())
	connected := hub != nil && hub.ConnectedForTenant(tid)

	status := map[string]any{
		"enabled":             connected,
		"connected":           connected,
		"extension_connected": connected,
		"state":               "disabled",
	}

	if connected {
		status["state"] = "extension"
		hub.mu.Lock()
		status["version"] = hub.version
		hub.mu.Unlock()
	}

	json.NewEncoder(w).Encode(status)
}

// handleBrowserConfig returns browser configuration (extension-only mode).
func (g *Gateway) handleBrowserConfig(w http.ResponseWriter, r *http.Request) {
	hub := g.browserHub
	tid := tenantFromCtx(r.Context())
	connected := hub != nil && hub.ConnectedForTenant(tid)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"mode":      "extension",
		"connected": connected,
		"headless":  false,
	})
}

// handleBrowserScreenshotLatest returns the latest screenshot via extension.
func (g *Gateway) handleBrowserScreenshotLatest(w http.ResponseWriter, r *http.Request) {
	hub := g.browserHub
	tid := tenantFromCtx(r.Context())
	if hub == nil || !hub.ConnectedForTenant(tid) {
		apperror.WriteCode(w, apperror.CodeInternal, "browser extension not connected for current tenant")
		return
	}

	result, err := hub.SendAction(r.Context(), BrowserAction{Type: "browser_screenshot"})
	if err != nil {
		apperror.WriteCode(w, apperror.CodeInternal, "screenshot failed: "+err.Error())
		return
	}
	if !result.OK {
		apperror.WriteCode(w, apperror.CodeInternal, "screenshot failed: "+result.Error)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"screenshot": stripDataPrefix(result.Screenshot),
		"timestamp":  time.Now().Format(time.RFC3339),
	})
}

// handleBrowserNavigate navigates the browser to a URL via extension.
func (g *Gateway) handleBrowserNavigate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}

	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.URL == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "url is required")
		return
	}

	hub := g.browserHub
	tid := tenantFromCtx(r.Context())
	if hub == nil || !hub.ConnectedForTenant(tid) {
		apperror.WriteCode(w, apperror.CodeInternal, "browser extension not connected for current tenant")
		return
	}

	result, err := hub.SendAction(r.Context(), BrowserAction{Type: "browser_navigate", URL: req.URL})
	if err != nil {
		apperror.WriteCode(w, apperror.CodeInternal, "navigate failed: "+err.Error())
		return
	}
	if !result.OK {
		apperror.WriteCode(w, apperror.CodeInternal, "navigate failed: "+result.Error)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"screenshot": stripDataPrefix(result.Screenshot),
		"title":      result.Title,
		"url":        result.URL,
	})
}

// handleBrowserScreenshot takes a screenshot via extension.
func (g *Gateway) handleBrowserScreenshot(w http.ResponseWriter, r *http.Request) {
	hub := g.browserHub
	tid := tenantFromCtx(r.Context())
	if hub == nil || !hub.ConnectedForTenant(tid) {
		apperror.WriteCode(w, apperror.CodeInternal, "browser extension not connected for current tenant")
		return
	}

	result, err := hub.SendAction(r.Context(), BrowserAction{Type: "browser_screenshot"})
	if err != nil {
		apperror.WriteCode(w, apperror.CodeInternal, "screenshot failed: "+err.Error())
		return
	}
	if !result.OK {
		apperror.WriteCode(w, apperror.CodeInternal, "screenshot failed: "+result.Error)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"screenshot": stripDataPrefix(result.Screenshot),
		"timestamp":  time.Now().Format(time.RFC3339),
	})
}

// handleBrowserOCR extracts page content via extension.
func (g *Gateway) handleBrowserOCR(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}

	hub := g.browserHub
	tid := tenantFromCtx(r.Context())
	if hub == nil || !hub.ConnectedForTenant(tid) {
		apperror.WriteCode(w, apperror.CodeInternal, "browser extension not connected for current tenant")
		return
	}

	result, err := hub.SendAction(r.Context(), BrowserAction{Type: "browser_get_content"})
	if err != nil {
		apperror.WriteCode(w, apperror.CodeInternal, "content extraction failed: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"text":   result.Content,
		"result": result.Content,
	})
}

// handleOPPPending returns pending OPP items (placeholder — extension doesn't have OPP yet).
func (g *Gateway) handleOPPPending(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"items": []any{},
		"total": 0,
	})
}

// handleOPPDecide processes OPP decisions (placeholder).
func (g *Gateway) handleOPPDecide(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}

	var req struct {
		ProblemID string `json:"problem_id"`
		ID        string `json:"id"`
		Decision  string `json:"decision"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON")
		return
	}
	if req.ProblemID == "" {
		req.ProblemID = req.ID
	}
	if req.ProblemID == "" {
		apperror.WriteCode(w, apperror.CodeMissingField, "problem_id or id is required")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status":     "resolved",
		"problem_id": req.ProblemID,
	})
}

func stripDataPrefix(s string) string {
	if i := strings.Index(s, "base64,"); i >= 0 {
		return s[i+7:]
	}
	return s
}
