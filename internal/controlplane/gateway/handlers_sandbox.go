package gateway

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"yunque-agent/internal/apperror"
	"yunque-agent/internal/execution/sandbox"
)

func (g *Gateway) handleSandboxExec(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	var req struct {
		Command string   `json:"command"`
		Args    []string `json:"args"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Command == "" {
		apperror.WriteCode(w, apperror.CodeMissingField, "command is required")
		return
	}

	tid := tenantFromCtx(r.Context())
	slog.Info("sandbox exec",
		"tenant", tid,
		"command", req.Command,
		"args", req.Args,
		"remote_addr", r.RemoteAddr,
	)

	sb, err := sandbox.New("", sandbox.DefaultPolicy())
	if err != nil {
		slog.Warn("sandbox exec failed: init", "tenant", tid, "command", req.Command, "err", err)
		apperror.Write(w, apperror.Wrap(apperror.CodeSandboxError, "sandbox init failed", err))
		return
	}
	defer sb.Cleanup()
	result, err := sb.Exec(r.Context(), req.Command, req.Args...)
	if err != nil {
		slog.Warn("sandbox exec failed: run", "tenant", tid, "command", req.Command, "err", err)
		apperror.Write(w, apperror.Wrap(apperror.CodeSandboxError, "sandbox exec failed", err))
		return
	}
	slog.Info("sandbox exec completed", "tenant", tid, "command", req.Command)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (g *Gateway) handleSandboxProbe(w http.ResponseWriter, r *http.Request) {
	result := map[string]any{
		"sandbox_cloud_api_key_set":  os.Getenv("SANDBOX_CLOUD_API_KEY") != "",
		"sandbox_cloud_base_url_set": os.Getenv("SANDBOX_CLOUD_BASE_URL") != "",
		"tori_api_base_url_set":      os.Getenv("TORI_API_BASE_URL") != "",
		"llm_api_key_set":            os.Getenv("LLM_API_KEY") != "",
	}

	toriBase := strings.TrimSpace(os.Getenv("TORI_API_BASE_URL"))
	if toriBase == "" {
		toriBase = strings.TrimSpace(os.Getenv("SANDBOX_CLOUD_BASE_URL"))
	}

	if toriBase != "" {
		trimmed := strings.TrimRight(toriBase, "/")
		probeURL := trimmed + "/sandboxes/status"
		if !strings.HasSuffix(trimmed, "/v1") {
			probeURL = trimmed + "/v1/sandboxes/status"
		}
		ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, probeURL, nil)
		if err == nil {
			client := &http.Client{Timeout: 8 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				result["probe_error"] = err.Error()
			} else {
				defer resp.Body.Close()
				body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
				var probeResp map[string]any
				if json.Unmarshal(body, &probeResp) == nil {
					result["tori_sandbox_status"] = probeResp
				} else {
					result["tori_sandbox_raw"] = string(body)
				}
				result["probe_status_code"] = resp.StatusCode
			}
		}
	} else {
		result["probe_error"] = "no TORI_API_BASE_URL or SANDBOX_CLOUD_BASE_URL configured"
	}

	var source string
	if os.Getenv("SANDBOX_CLOUD_API_KEY") != "" {
		source = "env:SANDBOX_CLOUD_API_KEY"
	} else if g.toriTokenStore != nil && g.toriTokenStore.IsBound() {
		source = "tori_oauth_bound"
	} else if toriBase != "" && os.Getenv("LLM_API_KEY") != "" {
		source = "auto:TORI_API_BASE_URL+LLM_API_KEY"
	} else {
		source = "none"
	}
	result["key_source"] = source

	g.desktopMu.Lock()
	result["cloud_runner_ready"] = g.cloudRunner != nil
	result["desktop_running"] = g.desktopSandbox != nil
	g.desktopMu.Unlock()

	writeJSON(w, result)
}

func (g *Gateway) handleDesktopCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	g.desktopMu.Lock()
	defer g.desktopMu.Unlock()

	if g.desktopSandbox != nil {
		writeJSON(w, map[string]any{
			"ok": true, "sandbox": g.desktopSandbox,
			"message": "desktop sandbox already running",
		})
		return
	}

	if g.cloudRunner == nil {
		cfg := sandbox.CloudConfig{
			Enabled: true,
			APIKey:  os.Getenv("SANDBOX_CLOUD_API_KEY"),
			BaseURL: os.Getenv("SANDBOX_CLOUD_BASE_URL"),
		}
		if cfg.APIKey == "" && g.toriTokenStore != nil && g.toriTokenStore.IsBound() {
			t := g.toriTokenStore.Get()
			if t != nil && t.APIKey != "" {
				cfg.APIKey = t.APIKey
				if cfg.BaseURL == "" && t.ToriBaseURL != "" {
					cfg.BaseURL = strings.TrimRight(t.ToriBaseURL, "/") + "/v1"
				}
			}
		}
		if cfg.APIKey == "" {
			if toriBase := strings.TrimSpace(os.Getenv("TORI_API_BASE_URL")); toriBase != "" {
				if llmKey := strings.TrimSpace(os.Getenv("LLM_API_KEY")); llmKey != "" {
					cfg.APIKey = llmKey
					trimmed := strings.TrimRight(toriBase, "/")
					if strings.HasSuffix(trimmed, "/v1") {
						cfg.BaseURL = trimmed
					} else {
						cfg.BaseURL = trimmed + "/v1"
					}
				}
			}
		}
		if cfg.APIKey == "" {
			apperror.WriteCode(w, apperror.CodeMissingField, "SANDBOX_CLOUD_API_KEY not configured and Tori not bound")
			return
		}
		cr, err := sandbox.NewCloudRunner(cfg)
		if err != nil {
			apperror.Write(w, apperror.Wrap(apperror.CodeSandboxError, "cloud runner init failed", err))
			return
		}
		g.cloudRunner = cr
	}

	ds, err := g.cloudRunner.CreateDesktop(r.Context())
	if err != nil {
		apperror.Write(w, apperror.Wrap(apperror.CodeSandboxError, "create desktop failed", err))
		return
	}
	g.desktopSandbox = ds
	writeJSON(w, map[string]any{"ok": true, "sandbox": ds})
}

func (g *Gateway) handleDesktopStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, g.DesktopSandboxStatus(r.Context()))
}

// DesktopSandboxStatus exposes a read-only cloud desktop snapshot for packs
// that need computer-use readiness without gaining create/destroy privileges.
func (g *Gateway) DesktopSandboxStatus(ctx context.Context) map[string]any {
	g.desktopMu.Lock()
	ds := g.desktopSandbox
	cr := g.cloudRunner
	g.desktopMu.Unlock()

	if ds == nil {
		return map[string]any{"ok": true, "available": cr != nil, "running": false}
	}

	info := map[string]any{"ok": true, "available": true, "running": true, "sandbox": ds}
	if cr != nil {
		status, err := cr.DesktopStatus(ctx, ds.ID)
		if err != nil {
			info["alive"] = false
			info["error"] = err.Error()
		} else {
			info["alive"] = true
			info["upstream"] = status
		}
	}
	return info
}

func (g *Gateway) handleDesktopDestroy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "DELETE or POST only")
		return
	}
	g.desktopMu.Lock()
	defer g.desktopMu.Unlock()

	if g.desktopSandbox == nil {
		writeJSON(w, map[string]any{"ok": true, "message": "no desktop sandbox running"})
		return
	}

	if g.cloudRunner != nil {
		_ = g.cloudRunner.DestroyDesktop(r.Context(), g.desktopSandbox.ID)
	}
	g.desktopSandbox = nil
	writeJSON(w, map[string]any{"ok": true, "message": "desktop sandbox destroyed"})
}
