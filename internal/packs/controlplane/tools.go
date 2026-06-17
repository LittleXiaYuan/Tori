package controlplanepack

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"yunque-agent/internal/agentcore/tools"
)

var envDenyList = map[string]bool{
	"PATH": true, "HOME": true, "USER": true, "SHELL": true,
	"LD_PRELOAD": true, "LD_LIBRARY_PATH": true, "DYLD_INSERT_LIBRARIES": true,
	"SYSTEMROOT": true, "COMSPEC": true, "WINDIR": true,
	"LLM_API_KEY": true, "JWT_SECRET": true, "ADMIN_PASSWORD_HASH": true,
}

func truncateCmd(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func (h *Handler) handleToolExec(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if os.Getenv("ENABLE_TOOLS_EXEC") != "true" {
		writeJSONStatus(w, http.StatusForbidden, map[string]string{
			"error": "Remote command execution is disabled. Set ENABLE_TOOLS_EXEC=true in .env to enable (high-risk).",
		})
		return
	}

	shellPolicy := h.gateway.ShellPolicy()
	if shellPolicy == nil && !strings.EqualFold(os.Getenv("TOOLS_EXEC_ALLOW_UNRESTRICTED"), "true") {
		writeJSONStatus(w, http.StatusForbidden, map[string]string{
			"error": "Shell execution policy is not configured. Configure shellPolicy, or set TOOLS_EXEC_ALLOW_UNRESTRICTED=true to accept the RCE risk.",
		})
		return
	}

	toolsMgr := h.gateway.ToolsManager()
	if toolsMgr == nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "tools not configured"})
		return
	}

	var opts tools.ExecOptions
	if err := json.NewDecoder(r.Body).Decode(&opts); err != nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"})
		return
	}
	if shellPolicy == nil {
		slog.Warn("tools/exec: unrestricted mode — command forwarded without shell policy",
			"tenant", h.gateway.TenantOf(r.Context()),
			"cmd_prefix", truncateCmd(opts.Command, 40))
	}

	if opts.Cwd != "" && filepath.IsAbs(opts.Cwd) {
		allowed := false
		if outputDir := h.gateway.OutputDir(); outputDir != "" {
			realOut, _ := filepath.EvalSymlinks(outputDir)
			realCwd, _ := filepath.EvalSymlinks(opts.Cwd)
			if realOut != "" && realCwd != "" {
				if rel, err := filepath.Rel(realOut, realCwd); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
					allowed = true
				}
			}
		}
		if !allowed {
			writeJSONStatus(w, http.StatusForbidden, map[string]string{
				"error": "cwd must be a relative path or within the output directory",
			})
			return
		}
	}

	sanitized := make([]string, 0, len(opts.Env))
	for _, kv := range opts.Env {
		key := strings.SplitN(kv, "=", 2)[0]
		if envDenyList[strings.ToUpper(key)] {
			continue
		}
		sanitized = append(sanitized, kv)
	}
	opts.Env = sanitized

	if shellPolicy != nil {
		tid := h.gateway.TenantOf(r.Context())
		policyResult, err := shellPolicy.Execute(r.Context(), opts, tid)
		if err != nil {
			writeJSONStatus(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if !policyResult.Approved {
			writeJSONStatus(w, http.StatusForbidden, map[string]any{
				"error":    "Command blocked by shell policy",
				"risk":     policyResult.Risk,
				"patterns": policyResult.Patterns,
			})
			return
		}
		_ = json.NewEncoder(w).Encode(policyResult)
		return
	}

	result, err := toolsMgr.Exec(r.Context(), opts)
	if err != nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(result)
}

func (h *Handler) handleToolList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	toolsMgr := h.gateway.ToolsManager()
	if toolsMgr == nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "tools not configured"})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"sessions": toolsMgr.List()})
}

func (h *Handler) handleToolPoll(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	toolsMgr := h.gateway.ToolsManager()
	if toolsMgr == nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "tools not configured"})
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "session id required"})
		return
	}
	lines, state, err := toolsMgr.PollSession(id)
	if err != nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"lines": lines, "state": state})
}

func (h *Handler) handleToolKill(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	toolsMgr := h.gateway.ToolsManager()
	if toolsMgr == nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "tools not configured"})
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "session id required"})
		return
	}
	if err := toolsMgr.Kill(id); err != nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"killed": id})
}

func writeJSONStatus(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
