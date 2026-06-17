package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"yunque-agent/internal/observe"
)

func TestMetricsEndpointSanitizesRecentErrors(t *testing.T) {
	gw, tm := newTestGatewayMigrationEnabled() // /v1/metrics is owned by the control-plane pack
	tenant := tm.Register("metrics-friendly-errors")
	raw := `handoff agent "general_exec" execution failed: planner fc step 1: all fallback LLM clients failed: Post "https://api.moonshot.ai/v1/chat/completions": EOF; context deadline exceeded`
	gw.metrics.RecordRequest(10*time.Millisecond, 0, 0, errors.New(raw))

	req := authedRequest(http.MethodGet, "/v1/metrics", "", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected metrics 200, got %d body=%s", w.Code, w.Body.String())
	}
	body := strings.ToLower(w.Body.String())
	for _, banned := range []string{"handoff agent", "execution failed", "context deadline exceeded", "all fallback", "eof", "api.moonshot.ai"} {
		if strings.Contains(body, banned) {
			t.Fatalf("metrics response should be friendly by default; found %q in %s", banned, w.Body.String())
		}
	}
	var snap observe.MetricsSnapshot
	if err := json.NewDecoder(w.Body).Decode(&snap); err != nil {
		t.Fatalf("decode metrics snapshot: %v", err)
	}
	if len(snap.RecentErrors) != 1 || !strings.Contains(snap.RecentErrors[0].Message, "已保留现场") {
		t.Fatalf("expected friendly recent error, got %+v", snap.RecentErrors)
	}
}

func TestSanitizeMetricsSnapshotForUserDoesNotMutateRawTracker(t *testing.T) {
	gw, tm := newTestGatewayMigrationEnabled()
	tenant := tm.Register("metrics-raw-tracker")
	raw := `context deadline exceeded`
	gw.metrics.RecordRequest(10*time.Millisecond, 0, 0, errors.New(raw))
	req := authedRequest(http.MethodGet, "/v1/metrics", "", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected metrics 200, got %d body=%s", w.Code, w.Body.String())
	}
	if strings.Contains(strings.ToLower(w.Body.String()), raw) {
		t.Fatalf("metrics response should hide raw timeout, got %s", w.Body.String())
	}

	rawSnap := gw.metrics.Snapshot()
	if len(rawSnap.RecentErrors) != 1 || !strings.Contains(rawSnap.RecentErrors[0].Message, raw) {
		t.Fatalf("expected internal metrics tracker to keep raw diagnostics, got %+v", rawSnap.RecentErrors)
	}
}

func TestMetricsEndpointContextCancellationIsFriendly(t *testing.T) {
	gw, tm := newTestGatewayMigrationEnabled() // /v1/metrics is owned by the control-plane pack
	tenant := tm.Register("metrics-context-cancel")
	gw.metrics.RecordRequest(10*time.Millisecond, 0, 0, context.DeadlineExceeded)

	req := authedRequest(http.MethodGet, "/v1/metrics", "", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected metrics 200, got %d body=%s", w.Code, w.Body.String())
	}
	if strings.Contains(strings.ToLower(w.Body.String()), "context deadline exceeded") {
		t.Fatalf("metrics response should hide raw timeout, got %s", w.Body.String())
	}
}
