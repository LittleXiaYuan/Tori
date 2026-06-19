// Package sandboxpack owns the high-risk sandbox and cloud desktop HTTP
// surface as a native capability pack. Pack Runtime controls enablement while
// the pack preserves the original auth/admin requirements internally.
package sandboxpack

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"yunque-agent/internal/apperror"
	"yunque-agent/internal/execution/sandbox"
	"yunque-agent/internal/tori"
	"yunque-agent/pkg/packruntime"
)

const PackID = "yunque.pack.sandbox"

type Gateway interface {
	TenantOf(ctx context.Context) string
	ToriTokenStore() *tori.TokenStore
	RequireAuth(http.HandlerFunc) http.HandlerFunc
	RequireAdmin(http.HandlerFunc) http.HandlerFunc
}

type Handler struct {
	gateway        Gateway
	cloudRunner    *sandbox.CloudRunner
	desktopSandbox *sandbox.DesktopSandbox
	desktopMu      sync.Mutex
	host           packruntime.Host
	started        atomic.Bool
}

func New(gateway Gateway) *Handler {
	return &Handler{gateway: gateway}
}

var _ packruntime.Module = (*Handler)(nil)

func (h *Handler) PackID() string { return PackID }

func (h *Handler) Init(host packruntime.Host) error {
	h.host = host
	return nil
}

func (h *Handler) Start(ctx context.Context) error {
	h.started.Store(true)
	if h.host != nil {
		h.host.Logger().Info("sandbox pack started", "pack", PackID)
	}
	return nil
}

func (h *Handler) Stop(ctx context.Context) error {
	h.started.Store(false)
	return nil
}

func (h *Handler) Routes() []packruntime.BackendRoute {
	return []packruntime.BackendRoute{
		{Method: http.MethodPost, Path: "/v1/sandbox/exec", Handler: h.admin(h.Exec), Auth: packruntime.BackendRouteAuthPassthrough},
		{Method: http.MethodGet, Path: "/v1/sandbox/probe", Handler: h.admin(h.Probe), Auth: packruntime.BackendRouteAuthPassthrough},
		{Method: http.MethodPost, Path: "/v1/sandbox/desktop", Handler: h.admin(h.CreateDesktop), Auth: packruntime.BackendRouteAuthPassthrough},
		{Method: http.MethodGet, Path: "/v1/sandbox/desktop/status", Handler: h.Status},
		{Methods: []string{http.MethodDelete, http.MethodPost}, Path: "/v1/sandbox/desktop/destroy", Handler: h.DestroyDesktop},
	}
}

func RouteSpecs() []packruntime.BackendRouteSpec {
	return []packruntime.BackendRouteSpec{
		{Method: http.MethodPost, Path: "/v1/sandbox/exec", Description: "Execute an admin-approved command inside the local sandbox policy."},
		{Method: http.MethodGet, Path: "/v1/sandbox/probe", Description: "Probe cloud sandbox and Tori cloud desktop readiness."},
		{Method: http.MethodPost, Path: "/v1/sandbox/desktop", Description: "Create or reuse an authenticated cloud desktop sandbox."},
		{Method: http.MethodGet, Path: "/v1/sandbox/desktop/status", Description: "Read the current cloud desktop sandbox status."},
		{Method: http.MethodDelete, Path: "/v1/sandbox/desktop/destroy", Description: "Destroy the current cloud desktop sandbox."},
		{Method: http.MethodPost, Path: "/v1/sandbox/desktop/destroy", Description: "Destroy the current cloud desktop sandbox."},
	}
}

func Paths() []string {
	seen := map[string]bool{}
	var paths []string
	for _, spec := range RouteSpecs() {
		if seen[spec.Path] {
			continue
		}
		seen[spec.Path] = true
		paths = append(paths, spec.Path)
	}
	return paths
}

func (h *Handler) admin(next http.HandlerFunc) http.HandlerFunc {
	if h.gateway == nil {
		return next
	}
	wrapped := next
	wrapped = h.gateway.RequireAdmin(wrapped)
	wrapped = h.gateway.RequireAuth(wrapped)
	return wrapped
}

func (h *Handler) tenantOf(ctx context.Context) string {
	if h.gateway == nil {
		return "default"
	}
	return h.gateway.TenantOf(ctx)
}

func (h *Handler) tokenStore() *tori.TokenStore {
	if h.gateway == nil {
		return nil
	}
	return h.gateway.ToriTokenStore()
}

func (h *Handler) Exec(w http.ResponseWriter, r *http.Request) {
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

	tid := h.tenantOf(r.Context())
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
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) Probe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET only")
		return
	}
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
	ts := h.tokenStore()
	if os.Getenv("SANDBOX_CLOUD_API_KEY") != "" {
		source = "env:SANDBOX_CLOUD_API_KEY"
	} else if ts != nil && ts.IsBound() {
		source = "tori_oauth_bound"
	} else if toriBase != "" && os.Getenv("LLM_API_KEY") != "" {
		source = "auto:TORI_API_BASE_URL+LLM_API_KEY"
	} else {
		source = "none"
	}
	result["key_source"] = source

	h.desktopMu.Lock()
	result["cloud_runner_ready"] = h.cloudRunner != nil
	result["desktop_running"] = h.desktopSandbox != nil
	h.desktopMu.Unlock()

	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) CreateDesktop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	h.desktopMu.Lock()
	defer h.desktopMu.Unlock()

	if h.desktopSandbox != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok": true, "sandbox": h.desktopSandbox,
			"message": "desktop sandbox already running",
		})
		return
	}

	if h.cloudRunner == nil {
		cfg := sandbox.CloudConfig{
			Enabled: true,
			APIKey:  os.Getenv("SANDBOX_CLOUD_API_KEY"),
			BaseURL: os.Getenv("SANDBOX_CLOUD_BASE_URL"),
		}
		if cfg.APIKey == "" {
			if ts := h.tokenStore(); ts != nil && ts.IsBound() {
				t := ts.Get()
				if t != nil && t.APIKey != "" {
					cfg.APIKey = t.APIKey
					if cfg.BaseURL == "" && t.ToriBaseURL != "" {
						cfg.BaseURL = strings.TrimRight(t.ToriBaseURL, "/") + "/v1"
					}
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
		h.cloudRunner = cr
	}

	ds, err := h.cloudRunner.CreateDesktop(r.Context())
	if err != nil {
		apperror.Write(w, apperror.Wrap(apperror.CodeSandboxError, "create desktop failed", err))
		return
	}
	h.desktopSandbox = ds
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "sandbox": ds})
}

func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET only")
		return
	}
	writeJSON(w, http.StatusOK, h.StatusMap(r.Context()))
}

func (h *Handler) StatusMap(ctx context.Context) map[string]any {
	h.desktopMu.Lock()
	ds := h.desktopSandbox
	cr := h.cloudRunner
	h.desktopMu.Unlock()

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

func (h *Handler) DestroyDesktop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "DELETE or POST only")
		return
	}
	h.desktopMu.Lock()
	defer h.desktopMu.Unlock()

	if h.desktopSandbox == nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "no desktop sandbox running"})
		return
	}

	if h.cloudRunner != nil {
		_ = h.cloudRunner.DestroyDesktop(r.Context(), h.desktopSandbox.ID)
	}
	h.desktopSandbox = nil
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "desktop sandbox destroyed"})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
