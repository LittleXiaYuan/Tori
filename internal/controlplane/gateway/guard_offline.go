package gateway

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// offlineRoleValue is the reserved tier/role name of the background engine
// (小羽 / RWKV-7). It is a latency-tolerant local model dedicated to the
// CogniKernel dreaming / self-evolution loops and must never serve a
// user-facing request.
const offlineRoleValue = "offline"

// offlineRoleKeys are the request fields a client could use to try to force the
// front-stage chat onto the background engine. We block all of them so a slow
// local-model inference can never leak latency into the interactive path.
var offlineRoleKeys = []string{
	"role", "tier", "model_tier", "thinking_level",
	"thinking", "model", "provider", "engine",
}

// guardNoOfflineRole rejects, with 403 Forbidden, any front-stage chat request
// that targets the offline background engine. The request body is buffered and
// then restored so the downstream handler can still decode it normally.
func (g *Gateway) guardNoOfflineRole(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil && (r.Method == http.MethodPost || r.Method == http.MethodPut) {
			body, err := io.ReadAll(io.LimitReader(r.Body, 8<<20))
			_ = r.Body.Close()
			if err == nil {
				// Restore the body for the real handler.
				r.Body = io.NopCloser(bytes.NewReader(body))
				if bodyTargetsOfflineEngine(body) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusForbidden)
					_ = json.NewEncoder(w).Encode(map[string]any{
						"error":   "offline_role_forbidden",
						"message": "front-stage chat may not target the offline (background) engine",
					})
					return
				}
			}
		}
		next(w, r)
	}
}

// bodyTargetsOfflineEngine reports whether the JSON request body asks for the
// offline engine via any known role/tier/model field (case-insensitive). A body
// that is not a JSON object is treated as benign — schema validation belongs to
// the handler, not this guard.
func bodyTargetsOfflineEngine(body []byte) bool {
	if len(body) == 0 {
		return false
	}
	var generic map[string]any
	if err := json.Unmarshal(body, &generic); err != nil {
		return false
	}
	for _, key := range offlineRoleKeys {
		v, ok := generic[key]
		if !ok {
			continue
		}
		if s, ok := v.(string); ok && strings.EqualFold(strings.TrimSpace(s), offlineRoleValue) {
			return true
		}
	}
	return false
}
