package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGenerateAndValidateJWT(t *testing.T) {
	cfg := JWTConfig{Secret: "test-secret-key", Issuer: "yunque-test", Expiration: time.Hour}
	token, err := GenerateJWT(cfg, "tenant-1", "admin")
	if err != nil {
		t.Fatalf("GenerateJWT: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	claims, err := ValidateJWT(cfg, token)
	if err != nil {
		t.Fatalf("ValidateJWT: %v", err)
	}
	if claims.TenantID != "tenant-1" {
		t.Errorf("TenantID = %q, want %q", claims.TenantID, "tenant-1")
	}
	if claims.Role != "admin" {
		t.Errorf("Role = %q, want %q", claims.Role, "admin")
	}
	if claims.Iss != "yunque-test" {
		t.Errorf("Issuer = %q, want %q", claims.Iss, "yunque-test")
	}
}

func TestValidateJWT_ExpiredToken(t *testing.T) {
	cfg := JWTConfig{Secret: "test-secret", Issuer: "test", Expiration: -1 * time.Hour}
	token, _ := GenerateJWT(cfg, "t1", "user")
	_, err := ValidateJWT(cfg, token)
	if err != errTokenExpired {
		t.Fatalf("expected errTokenExpired, got %v", err)
	}
}

func TestValidateJWT_WrongSecret(t *testing.T) {
	cfg := JWTConfig{Secret: "correct-secret", Issuer: "test", Expiration: time.Hour}
	token, _ := GenerateJWT(cfg, "t1", "user")

	wrongCfg := JWTConfig{Secret: "wrong-secret", Issuer: "test", Expiration: time.Hour}
	_, err := ValidateJWT(wrongCfg, token)
	if err != errInvalidSignature {
		t.Fatalf("expected errInvalidSignature, got %v", err)
	}
}

func TestValidateJWT_InvalidFormat(t *testing.T) {
	cfg := JWTConfig{Secret: "s", Issuer: "test", Expiration: time.Hour}
	for _, bad := range []string{"", "abc", "a.b", "a.b.c.d"} {
		_, err := ValidateJWT(cfg, bad)
		if err == nil {
			t.Errorf("ValidateJWT(%q): expected error, got nil", bad)
		}
	}
}

func TestValidateJWT_WrongIssuer(t *testing.T) {
	cfg := JWTConfig{Secret: "s", Issuer: "issuer-a", Expiration: time.Hour}
	token, _ := GenerateJWT(cfg, "t1", "user")

	verifyCfg := JWTConfig{Secret: "s", Issuer: "issuer-b", Expiration: time.Hour}
	_, err := ValidateJWT(verifyCfg, token)
	if err != errInvalidIssuer {
		t.Fatalf("expected errInvalidIssuer, got %v", err)
	}
}

func TestRequireAuth_ValidAPIKey(t *testing.T) {
	gw, tm := newTestGateway()
	tn := tm.Register("auth-test")

	handler := gw.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", tn.APIKey)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with valid API key, got %d", w.Code)
	}
}

func TestRequireAuth_ValidJWT(t *testing.T) {
	gw, tm := newTestGateway()
	tm.Register("jwt-tenant")

	token, _ := GenerateJWT(*gw.jwtCfg, "jwt-tenant", "admin")

	handler := gw.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		tid := tenantFromCtx(r.Context())
		role := roleFromCtx(r.Context())
		json.NewEncoder(w).Encode(map[string]string{"tenant": tid, "role": role})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with valid JWT, got %d", w.Code)
	}
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["tenant"] != "jwt-tenant" {
		t.Errorf("tenant = %q, want %q", resp["tenant"], "jwt-tenant")
	}
	if resp["role"] != "admin" {
		t.Errorf("role = %q, want %q", resp["role"], "admin")
	}
}

func TestRequireAuth_MissingCredentials(t *testing.T) {
	gw, _ := newTestGateway()

	called := false
	handler := gw.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if called {
		t.Fatal("handler should not be called without auth")
	}
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without credentials, got %d", w.Code)
	}
}

func TestRequireAuth_InvalidAPIKey(t *testing.T) {
	gw, _ := newTestGateway()

	called := false
	handler := gw.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "nonexistent-key")
	w := httptest.NewRecorder()
	handler(w, req)

	if called {
		t.Fatal("handler should not be called with invalid key")
	}
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with invalid API key, got %d", w.Code)
	}
}

func TestRequireAuth_ExpiredJWT(t *testing.T) {
	gw, _ := newTestGateway()

	expiredCfg := JWTConfig{Secret: gw.jwtCfg.Secret, Issuer: gw.jwtCfg.Issuer, Expiration: -time.Hour}
	token, _ := GenerateJWT(expiredCfg, "t1", "user")

	called := false
	handler := gw.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler(w, req)

	if called {
		t.Fatal("handler should not be called with expired JWT")
	}
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with expired JWT, got %d", w.Code)
	}
}

func TestAuthTokenFromHeaders_BearerFormat(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer my-token-123")
	got := authTokenFromHeaders(req)
	if got != "my-token-123" {
		t.Fatalf("expected 'my-token-123', got %q", got)
	}
}

func TestAuthTokenFromHeaders_APIKeyHeader(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-API-Key", "key-456")
	got := authTokenFromHeaders(req)
	if got != "key-456" {
		t.Fatalf("expected 'key-456', got %q", got)
	}
}

func TestAuthTokenFromHeaders_APIKeyPrecedence(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-API-Key", "api-key")
	req.Header.Set("Authorization", "Bearer jwt-token")
	got := authTokenFromHeaders(req)
	if got != "api-key" {
		t.Fatalf("X-API-Key should take precedence, got %q", got)
	}
}

func TestAuthTokenFromHeaders_Empty(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	got := authTokenFromHeaders(req)
	if got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestAuthTokenFromQuery(t *testing.T) {
	for _, key := range []string{"key", "api_key", "token", "access_token"} {
		req := httptest.NewRequest("GET", "/?"+key+"=val-"+key, nil)
		got := authTokenFromQuery(req)
		if got != "val-"+key {
			t.Errorf("authTokenFromQuery with %s: got %q, want %q", key, got, "val-"+key)
		}
	}
	req := httptest.NewRequest("GET", "/?unrelated=abc", nil)
	if got := authTokenFromQuery(req); got != "" {
		t.Errorf("expected empty for unrelated query param, got %q", got)
	}
}

func TestRequireAdmin_AdminRole(t *testing.T) {
	gw, _ := newTestGateway()

	inner := gw.requireAdmin(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/admin", nil)
	ctx := contextWithTenant(req.Context(), "t1")
	ctx = context.WithValue(ctx, ctxRoleKey, "admin")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	inner(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for admin role, got %d", w.Code)
	}
}

func TestRequireAdmin_UserRole(t *testing.T) {
	gw, _ := newTestGateway()

	called := false
	inner := gw.requireAdmin(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	req := httptest.NewRequest("GET", "/admin", nil)
	ctx := contextWithTenant(req.Context(), "t1")
	ctx = context.WithValue(ctx, ctxRoleKey, "user")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	inner(w, req)

	if called {
		t.Fatal("should not be called for non-admin")
	}
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for user role, got %d", w.Code)
	}
}

func TestRoleFromCtx_Default(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	role := roleFromCtx(req.Context())
	if role != "user" {
		t.Fatalf("default role should be 'user', got %q", role)
	}
}

func TestBase64URLRoundTrip(t *testing.T) {
	inputs := [][]byte{
		[]byte("hello"),
		[]byte(""),
		[]byte("a"),
		[]byte{0, 1, 2, 255, 254},
		[]byte("test with padding!!!"),
	}
	for _, input := range inputs {
		encoded := base64URLEncode(input)
		decoded, err := base64URLDecode(encoded)
		if err != nil {
			t.Fatalf("base64URLDecode(%q): %v", encoded, err)
		}
		if string(decoded) != string(input) {
			t.Errorf("roundtrip failed for %q", input)
		}
	}
}

func TestRequireAuth_TenantContext(t *testing.T) {
	gw, tm := newTestGateway()
	tn := tm.Register("ctx-test-org")

	var capturedTenant string
	handler := gw.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		capturedTenant = tenantFromCtx(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", tn.APIKey)
	w := httptest.NewRecorder()
	handler(w, req)

	if capturedTenant != tn.ID {
		t.Fatalf("tenant from context = %q, want %q", capturedTenant, tn.ID)
	}
}

func TestRequireAuth_JWTRoleInContext(t *testing.T) {
	gw, _ := newTestGateway()
	token, _ := GenerateJWT(*gw.jwtCfg, "role-tenant", "admin")

	var capturedRole string
	handler := gw.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		capturedRole = roleFromCtx(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if capturedRole != "admin" {
		t.Fatalf("role from context = %q, want %q", capturedRole, "admin")
	}
}
