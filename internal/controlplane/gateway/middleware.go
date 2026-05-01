package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"yunque-agent/internal/agentcore/audit"
)

// securityHeaders sets standard security response headers on every request.
func (g *Gateway) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; "+
				"style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; "+
				"connect-src 'self' ws: wss:; font-src 'self' data:; frame-ancestors 'none'")
		if r.Header.Get("X-Forwarded-Proto") == "https" || r.TLS != nil {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

// corsMiddleware handles CORS preflight and response headers.
func (g *Gateway) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowed := g.corsOrigin(origin)
		if allowed != "" {
			w.Header().Set("Access-Control-Allow-Origin", allowed)
			if allowed != "*" {
				w.Header().Set("Vary", "Origin")
			}
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization,Content-Type,X-API-Key")
		w.Header().Set("Access-Control-Max-Age", "86400")
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// bodySizeLimit enforces request body size limits:
// 32MB for uploads, 1MB for everything else.
func bodySizeLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			maxBody := int64(1 << 20) // 1MB
			if strings.HasPrefix(r.URL.Path, "/v1/upload") {
				maxBody = 32 << 20 // 32MB
			}
			r.Body = http.MaxBytesReader(w, r.Body, maxBody)
		}
		next.ServeHTTP(w, r)
	})
}

// setupGate blocks all authenticated API access until the admin password is set
// (first-run enforcement). Health, auth, and setup endpoints are exempt.
func (g *Gateway) setupGate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if g.passwordStore != nil && !g.passwordStore.IsSetup() {
			p := r.URL.Path
			exempt := p == "/healthz" || p == "/livez" || p == "/readyz" ||
				strings.HasPrefix(p, "/healthz/") ||
				p == "/v1/version" || p == "/" ||
				strings.HasPrefix(p, "/v1/auth/") ||
				strings.HasPrefix(p, "/v1/setup/") ||
				strings.HasPrefix(p, "/api/settings/check") ||
				strings.HasPrefix(p, "/_next/") ||
				!strings.HasPrefix(p, "/v1/") && !strings.HasPrefix(p, "/api/")
			if !exempt {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte(`{"error":"password_required","message":"Admin password must be set before using the API. POST /v1/auth/set-password"}`))
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// rateLimitMiddleware applies global rate limiting for mutating requests.
// GET/OPTIONS/HEAD and health endpoints are exempt.
func (g *Gateway) rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "OPTIONS" && r.Method != "HEAD" &&
			r.URL.Path != "/healthz" && r.URL.Path != "/livez" && r.URL.Path != "/readyz" &&
			!strings.HasPrefix(r.URL.Path, "/healthz/") && r.URL.Path != "/v1/version" {
			key := tenantFromCtx(r.Context())
			if key == "" {
				key = "ip:" + r.RemoteAddr
			}
			if !g.limiter.Allow(key) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "60")
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"error":"rate limit exceeded","retry_after":60}`))
				slog.Warn("rate limited", "path", r.URL.Path, "key", key)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// requestTracking assigns a unique request ID and logs request completion.
func (g *Gateway) requestTracking(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		count := g.reqCount.Add(1)
		reqID := fmt.Sprintf("%d-%d", start.UnixMilli(), count)

		w.Header().Set("X-Request-ID", reqID)
		ctx := context.WithValue(r.Context(), ctxKeyReqID, reqID)

		sw := &statusWriter{ResponseWriter: w, code: 200}
		next.ServeHTTP(sw, r.WithContext(ctx))

		if sw.code == 401 && strings.HasPrefix(r.URL.Path, "/api/browser/ext/") {
			slog.Debug("http", "method", r.Method, "path", r.URL.Path, "status", sw.code, "req_id", reqID)
		} else {
			slog.Info("http", "method", r.Method, "path", r.URL.Path, "status", sw.code,
				"duration_ms", time.Since(start).Milliseconds(), "req_id", reqID)
		}
		if g.auditChain != nil {
			g.auditChain.Append(audit.EventSystem, tenantFromCtx(ctx), r.Method+" "+r.URL.Path,
				fmt.Sprintf("status=%d dur=%dms", sw.code, time.Since(start).Milliseconds()))
		}
	})
}

// buildMiddlewareChain composes all middleware in the correct order.
// The outermost middleware runs first.
func (g *Gateway) buildMiddlewareChain(handler http.Handler) http.Handler {
	chain := handler
	chain = g.rateLimitMiddleware(chain)
	chain = g.setupGate(chain)
	chain = bodySizeLimit(chain)
	chain = g.securityHeaders(chain)
	chain = g.corsMiddleware(chain)
	chain = g.requestTracking(chain)
	return chain
}
