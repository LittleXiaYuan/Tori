package gateway

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRateLimiter_PerTenantIsolation(t *testing.T) {
	rl := NewRateLimiter(2, time.Minute)
	defer rl.Stop()

	if !rl.Allow("a") || !rl.Allow("a") {
		t.Fatal("tenant 'a' first 2 requests should be allowed")
	}
	if rl.Allow("a") {
		t.Fatal("tenant 'a' 3rd request should be blocked")
	}

	if !rl.Allow("b") {
		t.Fatal("tenant 'b' should not be affected by 'a'")
	}
}

func TestRateLimiter_Refill(t *testing.T) {
	rl := NewRateLimiter(1, 50*time.Millisecond)
	defer rl.Stop()

	if !rl.Allow("t") {
		t.Fatal("first request should be allowed")
	}
	if rl.Allow("t") {
		t.Fatal("second immediate request should be blocked")
	}

	time.Sleep(60 * time.Millisecond)

	if !rl.Allow("t") {
		t.Fatal("request after refill interval should be allowed")
	}
}

func TestRateLimitMiddleware_ReturnsRetryAfter(t *testing.T) {
	gw, _ := newTestGateway()
	gw.SetRateLimit(1, time.Minute)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	chain := gw.rateLimitMiddleware(handler)

	req := httptest.NewRequest("POST", "/v1/chat", strings.NewReader("{}"))
	w := httptest.NewRecorder()
	chain.ServeHTTP(w, req)

	req = httptest.NewRequest("POST", "/v1/chat", strings.NewReader("{}"))
	w = httptest.NewRecorder()
	chain.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w.Code)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Fatal("missing Retry-After header")
	}
}

func TestRateLimitMiddleware_ExemptsGET(t *testing.T) {
	gw, _ := newTestGateway()
	gw.SetRateLimit(1, time.Minute)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	chain := gw.rateLimitMiddleware(handler)

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/v1/skills", nil)
		w := httptest.NewRecorder()
		chain.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("GET request %d should be exempt from rate limiting, got %d", i, w.Code)
		}
	}
}

func TestRateLimitMiddleware_ExemptsHealthEndpoints(t *testing.T) {
	gw, _ := newTestGateway()
	gw.SetRateLimit(1, time.Minute)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	chain := gw.rateLimitMiddleware(handler)

	for _, path := range []string{"/healthz", "/livez", "/readyz", "/healthz/cognitive"} {
		req := httptest.NewRequest("POST", path, nil)
		w := httptest.NewRecorder()
		chain.ServeHTTP(w, req)
		if w.Code == http.StatusTooManyRequests {
			t.Fatalf("%s should be exempt from rate limiting", path)
		}
	}
}

func TestRateLimitMiddleware_ExemptsOPTIONS(t *testing.T) {
	gw, _ := newTestGateway()
	gw.SetRateLimit(1, time.Minute)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	chain := gw.rateLimitMiddleware(handler)

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("OPTIONS", "/v1/chat", nil)
		w := httptest.NewRecorder()
		chain.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("OPTIONS should be exempt from rate limiting, got %d", w.Code)
		}
	}
}

func TestRateLimiterMiddleware_PerTenantKeyFromContext(t *testing.T) {
	gw, _ := newTestGateway()
	gw.SetRateLimit(1, time.Minute)

	handler := gw.limiter.Middleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req1 := httptest.NewRequest("POST", "/v1/chat", strings.NewReader("{}"))
	req1 = req1.WithContext(contextWithTenant(req1.Context(), "tenant-A"))
	w1 := httptest.NewRecorder()
	handler(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("tenant-A first request expected 200, got %d", w1.Code)
	}

	req2 := httptest.NewRequest("POST", "/v1/chat", strings.NewReader("{}"))
	req2 = req2.WithContext(contextWithTenant(req2.Context(), "tenant-B"))
	w2 := httptest.NewRecorder()
	handler(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("tenant-B should not be affected by tenant-A, got %d", w2.Code)
	}
}

func TestSecurityHeaders(t *testing.T) {
	gw, _ := newTestGateway()

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	chain := gw.securityHeaders(inner)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	chain.ServeHTTP(w, req)

	expectedHeaders := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
	}
	for h, v := range expectedHeaders {
		if got := w.Header().Get(h); got != v {
			t.Errorf("%s = %q, want %q", h, got, v)
		}
	}

	if got := w.Header().Get("Content-Security-Policy"); got == "" {
		t.Error("missing Content-Security-Policy header")
	}
	if got := w.Header().Get("Permissions-Policy"); got == "" {
		t.Error("missing Permissions-Policy header")
	}
}

func TestSecurityHeaders_HSTS(t *testing.T) {
	gw, _ := newTestGateway()

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	chain := gw.securityHeaders(inner)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	w := httptest.NewRecorder()
	chain.ServeHTTP(w, req)

	if got := w.Header().Get("Strict-Transport-Security"); !strings.Contains(got, "max-age=") {
		t.Errorf("expected HSTS header with max-age, got %q", got)
	}
}

func TestCORSMiddleware_Preflight(t *testing.T) {
	gw, _ := newTestGateway()
	gw.SetAllowedOrigins([]string{"https://example.com"})

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("inner handler should not be called for OPTIONS")
	})
	chain := gw.corsMiddleware(inner)

	req := httptest.NewRequest("OPTIONS", "/v1/chat", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()
	chain.ServeHTTP(w, req)

	if w.Code != 204 {
		t.Fatalf("expected 204 for preflight, got %d", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://example.com" {
		t.Fatalf("ACAO = %q, want %q", got, "https://example.com")
	}
}

func TestCORSMiddleware_RejectsUnknownOrigin(t *testing.T) {
	gw, _ := newTestGateway()
	gw.SetAllowedOrigins([]string{"https://myapp.com"})

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	chain := gw.corsMiddleware(inner)

	req := httptest.NewRequest("GET", "/v1/chat", nil)
	req.Header.Set("Origin", "https://evil.com")
	w := httptest.NewRecorder()
	chain.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("should not set ACAO for unknown origin, got %q", got)
	}
}

func TestCORSMiddleware_Wildcard(t *testing.T) {
	gw, _ := newTestGateway()
	gw.SetAllowedOrigins([]string{"*"})

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	chain := gw.corsMiddleware(inner)

	req := httptest.NewRequest("GET", "/v1/chat", nil)
	req.Header.Set("Origin", "https://any-site.com")
	w := httptest.NewRecorder()
	chain.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("ACAO = %q, want %q", got, "*")
	}
}

func TestCORSMiddleware_NoOrigins(t *testing.T) {
	gw, _ := newTestGateway()

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	chain := gw.corsMiddleware(inner)

	req := httptest.NewRequest("GET", "/v1/chat", nil)
	req.Header.Set("Origin", "https://site.com")
	w := httptest.NewRecorder()
	chain.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("no origins configured should mean no CORS header, got %q", got)
	}
}

func TestCORSMiddleware_NoOriginsAllowsLoopbackDesktopDev(t *testing.T) {
	gw, _ := newTestGateway()

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	chain := gw.corsMiddleware(inner)

	req := httptest.NewRequest("GET", "/healthz", nil)
	req.Header.Set("Origin", "http://localhost:3001")
	w := httptest.NewRecorder()
	chain.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3001" {
		t.Fatalf("loopback dev origin should be allowed, got %q", got)
	}
	if got := w.Header().Get("Vary"); got != "Origin" {
		t.Fatalf("expected Vary: Origin for reflected loopback origin, got %q", got)
	}
}

func TestBodySizeLimit_NormalRequest(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 1024)
		n, _ := r.Body.Read(buf)
		w.Write(buf[:n])
	})
	chain := bodySizeLimit(inner)

	body := strings.Repeat("x", 512)
	req := httptest.NewRequest("POST", "/v1/chat", strings.NewReader(body))
	w := httptest.NewRecorder()
	chain.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for normal-sized body, got %d", w.Code)
	}
}

func TestSetupGate_BlocksWhenPasswordNotSet(t *testing.T) {
	gw, _ := newTestGateway()
	ps := NewPasswordStore("")
	gw.passwordStore = ps

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	chain := gw.setupGate(inner)

	req := httptest.NewRequest("GET", "/v1/skills", nil)
	w := httptest.NewRecorder()
	chain.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when password not set, got %d", w.Code)
	}
}

func TestSetupGate_ExemptsHealthEndpoints(t *testing.T) {
	gw, _ := newTestGateway()
	ps := NewPasswordStore("")
	gw.passwordStore = ps

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	chain := gw.setupGate(inner)

	exempt := []string{"/healthz", "/livez", "/readyz", "/v1/version", "/", "/v1/auth/login", "/v1/setup/detect"}
	for _, path := range exempt {
		req := httptest.NewRequest("GET", path, nil)
		w := httptest.NewRecorder()
		chain.ServeHTTP(w, req)
		if w.Code == http.StatusForbidden {
			t.Errorf("%s should be exempt from setup gate, got 403", path)
		}
	}
}

func TestSetupGate_AllowsAfterPasswordSet(t *testing.T) {
	gw, _ := newTestGateway()
	ps := NewPasswordStore("")
	ps.SetPassword("secure-password-123")
	gw.passwordStore = ps

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	chain := gw.setupGate(inner)

	req := httptest.NewRequest("GET", "/v1/skills", nil)
	w := httptest.NewRecorder()
	chain.ServeHTTP(w, req)

	if w.Code == http.StatusForbidden {
		t.Fatal("should not block after password is set")
	}
}

func TestRequestTracking_AssignsRequestID(t *testing.T) {
	gw, _ := newTestGateway()

	var capturedID string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = RequestID(r.Context())
		w.WriteHeader(http.StatusOK)
	})
	chain := gw.requestTracking(inner)

	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	chain.ServeHTTP(w, req)

	if capturedID == "" {
		t.Fatal("request ID should be set in context")
	}
	if w.Header().Get("X-Request-ID") == "" {
		t.Fatal("X-Request-ID header should be set")
	}
	if w.Header().Get("X-Request-ID") != capturedID {
		t.Fatal("X-Request-ID header should match context value")
	}
}

func TestRequestTracking_SequentialIDs(t *testing.T) {
	gw, _ := newTestGateway()

	var ids []string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ids = append(ids, w.Header().Get("X-Request-ID"))
		w.WriteHeader(http.StatusOK)
	})
	chain := gw.requestTracking(inner)

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/healthz", nil)
		w := httptest.NewRecorder()
		chain.ServeHTTP(w, req)
	}

	if len(ids) != 3 {
		t.Fatalf("expected 3 IDs, got %d", len(ids))
	}
	seen := make(map[string]bool)
	for _, id := range ids {
		if seen[id] {
			t.Fatalf("duplicate request ID: %s", id)
		}
		seen[id] = true
	}
}

func TestBuildMiddlewareChain_AllLayersApplied(t *testing.T) {
	gw, _ := newTestGateway()
	gw.SetAllowedOrigins([]string{"*"})

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	chain := gw.buildMiddlewareChain(inner)

	req := httptest.NewRequest("GET", "/healthz", nil)
	req.Header.Set("Origin", "https://test.com")
	w := httptest.NewRecorder()
	chain.ServeHTTP(w, req)

	if w.Header().Get("X-Request-ID") == "" {
		t.Error("missing X-Request-ID — requestTracking not applied")
	}
	if w.Header().Get("X-Content-Type-Options") == "" {
		t.Error("missing security header — securityHeaders not applied")
	}
	if w.Header().Get("Access-Control-Allow-Origin") == "" {
		t.Error("missing CORS header — corsMiddleware not applied")
	}
}

func TestStatusWriter_CapturesCode(t *testing.T) {
	w := httptest.NewRecorder()
	sw := &statusWriter{ResponseWriter: w, code: 200}

	sw.WriteHeader(http.StatusNotFound)

	if sw.code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", sw.code)
	}
}

func TestStatusWriter_Flush(t *testing.T) {
	w := httptest.NewRecorder()
	sw := &statusWriter{ResponseWriter: w, code: 200}

	sw.Flush()
}

func TestCorsOrigin_LogicCases(t *testing.T) {
	tests := []struct {
		name     string
		origins  []string
		input    string
		expected string
	}{
		{"empty list", nil, "https://any.com", ""},
		{"empty list allows localhost", nil, "http://localhost:3001", "http://localhost:3001"},
		{"empty list allows loopback ip", nil, "http://127.0.0.1:3001", "http://127.0.0.1:3001"},
		{"wildcard", []string{"*"}, "https://any.com", "*"},
		{"exact match", []string{"https://my.com"}, "https://my.com", "https://my.com"},
		{"no match", []string{"https://a.com"}, "https://b.com", ""},
		{"multi match first", []string{"https://a.com", "https://b.com"}, "https://a.com", "https://a.com"},
		{"multi match second", []string{"https://a.com", "https://b.com"}, "https://b.com", "https://b.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &Gateway{allowedOrigins: tt.origins}
			got := g.corsOrigin(tt.input)
			if got != tt.expected {
				t.Errorf("corsOrigin(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
