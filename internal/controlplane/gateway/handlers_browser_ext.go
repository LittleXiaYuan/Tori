package gateway

import (
	"net/http"
	"strings"
	"time"
)

func (g *Gateway) handleBrowserExtSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	if g.browserSessions == nil {
		writeJSONStatus(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "browser session store not initialized"})
		return
	}

	tid := tenantFromCtx(r.Context())
	record, err := g.browserSessions.Issue(tid)
	if err != nil {
		writeJSONStatus(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": err.Error()})
		return
	}

	wsURL := browserWebSocketURL(r)
	writeJSON(w, map[string]any{
		"ok":         true,
		"ws_url":     wsURL,
		"ticket":     record.Ticket,
		"nonce":      record.Nonce,
		"expires_at": record.ExpiresAt.Format(time.RFC3339),
		"ttl_sec":    int(time.Until(record.ExpiresAt).Seconds()),
	})
}

func browserWebSocketURL(r *http.Request) string {
	scheme := "ws"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "wss"
	}
	host := r.Host
	if host == "" {
		host = "localhost:9090"
	}
	return scheme + "://" + host + "/ws/browser"
}
