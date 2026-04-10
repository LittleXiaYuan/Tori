package gateway

import (
	"encoding/json"
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

func (g *Gateway) handleToolExec(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if os.Getenv("ENABLE_TOOLS_EXEC") != "true" {
		writeJSONStatus(w, http.StatusForbidden, map[string]string{
			"error": "Remote command execution is disabled. Set ENABLE_TOOLS_EXEC=true in .env to enable (high-risk).",
		})
		return
	}

	if g.toolsMgr == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "tools not configured"})
		return
	}

	var opts tools.ExecOptions
	if err := json.NewDecoder(r.Body).Decode(&opts); err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"})
		return
	}

	if opts.Cwd != "" {
		if filepath.IsAbs(opts.Cwd) {
			allowed := false
			if g.outputDir != "" {
				realOut, _ := filepath.EvalSymlinks(g.outputDir)
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

	if g.shellPolicy != nil {
		tid := tenantFromCtx(r.Context())
		policyResult, err := g.shellPolicy.Execute(r.Context(), opts, tid)
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
		json.NewEncoder(w).Encode(policyResult)
		return
	}

	result, err := g.toolsMgr.Exec(r.Context(), opts)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(result)
}

func (g *Gateway) handleToolList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.toolsMgr == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "tools not configured"})
		return
	}
	json.NewEncoder(w).Encode(map[string]any{"sessions": g.toolsMgr.List()})
}

func (g *Gateway) handleToolPoll(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.toolsMgr == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "tools not configured"})
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		json.NewEncoder(w).Encode(map[string]string{"error": "session id required"})
		return
	}
	lines, state, err := g.toolsMgr.PollSession(id)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]any{"lines": lines, "state": state})
}

func (g *Gateway) handleToolKill(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.toolsMgr == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "tools not configured"})
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		json.NewEncoder(w).Encode(map[string]string{"error": "session id required"})
		return
	}
	if err := g.toolsMgr.Kill(id); err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"killed": id})
}
