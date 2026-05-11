package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"
)

type fakeHealthChecker struct {
	err error
}

func (f fakeHealthChecker) HealthCheck(context.Context) error {
	return f.err
}

func TestLivez(t *testing.T) {
	gw, _ := newTestGateway()
	req := httptest.NewRequest("GET", "/livez", nil)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected status=ok, got %v", body["status"])
	}
	if _, ok := body["uptime_sec"]; !ok {
		t.Fatal("missing uptime_sec field")
	}
}

func TestReadyz(t *testing.T) {
	gw, _ := newTestGateway()
	req := httptest.NewRequest("GET", "/readyz", nil)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["status"] != "ready" {
		t.Fatalf("expected status=ready, got %v", body["status"])
	}
	checks, ok := body["checks"].(map[string]any)
	if !ok {
		t.Fatal("missing checks map")
	}
	for _, key := range []string{"llm", "conversations", "memory", "ledger"} {
		if _, exists := checks[key]; !exists {
			t.Errorf("missing check: %s", key)
		}
	}
}

func TestReadyzFailsWhenLedgerUnhealthy(t *testing.T) {
	gw, _ := newTestGateway()
	gw.SetLedgerHealthChecker(fakeHealthChecker{err: errors.New("schema version mismatch")})

	req := httptest.NewRequest("GET", "/readyz", nil)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 503 {
		t.Fatalf("expected 503, got %d", w.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["status"] != "not_ready" {
		t.Fatalf("expected status=not_ready, got %v", body["status"])
	}
	checks := body["checks"].(map[string]any)
	ledgerCheck := checks["ledger"].(map[string]any)
	if ledgerCheck["status"] != "down" {
		t.Fatalf("expected ledger down, got %v", ledgerCheck["status"])
	}
}

func TestReadyzDegradesWhenLedgerHealthCheckerMissing(t *testing.T) {
	gw, _ := newTestGateway()
	gw.SetLedgerHealthChecker(nil)

	req := httptest.NewRequest("GET", "/readyz", nil)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["status"] != "ready" {
		t.Fatalf("expected status=ready, got %v", body["status"])
	}
	checks := body["checks"].(map[string]any)
	ledgerCheck := checks["ledger"].(map[string]any)
	if ledgerCheck["status"] != "degraded" {
		t.Fatalf("expected ledger degraded, got %v", ledgerCheck["status"])
	}
}

func TestCognitiveHealth(t *testing.T) {
	gw, _ := newTestGateway()
	req := httptest.NewRequest("GET", "/healthz/cognitive", nil)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	status, ok := body["status"].(string)
	if !ok {
		t.Fatal("missing status field")
	}
	if status != "healthy" && status != "degraded" {
		t.Fatalf("unexpected status: %s", status)
	}

	checks, ok := body["checks"].(map[string]any)
	if !ok {
		t.Fatal("missing checks map")
	}
	if len(checks) == 0 {
		t.Fatal("expected non-empty checks")
	}

	// Must include LLM breaker check at minimum
	if _, exists := checks["llm_breaker"]; !exists {
		t.Error("missing llm_breaker check")
	}
	if _, exists := checks["ledger"]; !exists {
		t.Error("missing ledger check")
	}

	// Summary must be present
	summary, ok := body["summary"].(map[string]any)
	if !ok {
		t.Fatal("missing summary map")
	}
	if _, exists := summary["ok"]; !exists {
		t.Error("missing ok count in summary")
	}

	// Resources must be present
	resources, ok := body["resources"].(map[string]any)
	if !ok {
		t.Fatal("missing resources map")
	}
	if _, exists := resources["goroutines"]; !exists {
		t.Error("missing goroutines in resources")
	}

	// Timestamp
	if _, exists := body["timestamp"]; !exists {
		t.Error("missing timestamp")
	}
}

func TestCognitiveHealthFailsWhenLedgerUnhealthy(t *testing.T) {
	gw, _ := newTestGateway()
	gw.SetLedgerHealthChecker(fakeHealthChecker{err: errors.New("foreign_keys disabled")})

	req := httptest.NewRequest("GET", "/healthz/cognitive", nil)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != 503 {
		t.Fatalf("expected 503, got %d", w.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["status"] != "unhealthy" {
		t.Fatalf("expected status=unhealthy, got %v", body["status"])
	}
}

func TestHealthEndpointsNoAuth(t *testing.T) {
	gw, _ := newTestGateway()
	gw.SetPasswordStore(NewPasswordStore(""))

	for _, path := range []string{"/healthz", "/livez", "/readyz", "/healthz/cognitive"} {
		req := httptest.NewRequest("GET", path, nil)
		w := httptest.NewRecorder()
		gw.ServeHTTP(w, req)
		if w.Code == 401 || w.Code == 403 {
			t.Errorf("%s returned %d — health probes must not require auth", path, w.Code)
		}
	}
}
