package controlplanepack

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"yunque-agent/internal/agentcore/planner"
	"yunque-agent/internal/observe"
)

type fakeControlPlaneGateway struct {
	snapshot   observe.MetricsSnapshot
	prometheus string
	health     planner.ModelRuntimeHealth
	cache      map[string]any
	stats      map[string]any
	bridged    int
}

func (f *fakeControlPlaneGateway) HandleControlPlanePack(w http.ResponseWriter, r *http.Request) {
	f.bridged++
	w.WriteHeader(http.StatusTeapot)
}

func (f *fakeControlPlaneGateway) MetricsSnapshot() observe.MetricsSnapshot { return f.snapshot }

func (f *fakeControlPlaneGateway) MetricsPrometheus() string { return f.prometheus }

func (f *fakeControlPlaneGateway) ModelRuntimeHealth() planner.ModelRuntimeHealth {
	return f.health
}

func (f *fakeControlPlaneGateway) LLMResponseCacheStats() map[string]any { return f.cache }

func (f *fakeControlPlaneGateway) SystemStats(ctx context.Context) map[string]any { return f.stats }

func TestObservabilityRoutesAreNative(t *testing.T) {
	gw := &fakeControlPlaneGateway{
		prometheus: "yunque_requests_total 1\n",
		health:     planner.ModelRuntimeHealth{BreakerState: "closed", Configured: true},
		cache:      map[string]any{"size": 2},
		stats:      map[string]any{"requests_total": 3},
	}
	h := NewHandler(gw)
	byPath := map[string]http.HandlerFunc{}
	for _, rt := range h.Routes() {
		byPath[rt.Path] = rt.Handler
	}

	for _, path := range []string{"/v1/system/info", "/v1/system/stats", "/v1/metrics", "/v1/metrics/prometheus", "/v1/cache/stats"} {
		rec := httptest.NewRecorder()
		byPath[path](rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want 200: %s", path, rec.Code, rec.Body.String())
		}
	}
	if gw.bridged != 0 {
		t.Fatalf("observability routes should not call bridge, calls=%d", gw.bridged)
	}
}

func TestMetricsSanitizesRecentErrors(t *testing.T) {
	metrics := observe.New()
	raw := `handoff agent "general_exec" execution failed: all fallback LLM clients failed: context deadline exceeded`
	metrics.RecordRequest(10*time.Millisecond, 0, 0, errors.New(raw))
	h := NewHandler(&fakeControlPlaneGateway{snapshot: metrics.Snapshot()})
	rec := httptest.NewRecorder()

	h.handleMetrics(rec, httptest.NewRequest(http.MethodGet, "/v1/metrics", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	body := strings.ToLower(rec.Body.String())
	for _, banned := range []string{"handoff agent", "execution failed", "context deadline exceeded", "all fallback"} {
		if strings.Contains(body, banned) {
			t.Fatalf("metrics response should be friendly; found %q in %s", banned, rec.Body.String())
		}
	}
	var snap observe.MetricsSnapshot
	if err := json.NewDecoder(rec.Body).Decode(&snap); err != nil {
		t.Fatalf("decode metrics: %v", err)
	}
	if len(snap.RecentErrors) != 1 || !strings.Contains(snap.RecentErrors[0].Message, "已保留现场") {
		t.Fatalf("expected friendly error, got %+v", snap.RecentErrors)
	}
}

func TestCacheStatsOmitsNilCache(t *testing.T) {
	h := NewHandler(&fakeControlPlaneGateway{})
	rec := httptest.NewRecorder()
	h.handleCacheStats(rec, httptest.NewRequest(http.MethodGet, "/v1/cache/stats", nil))
	if strings.Contains(rec.Body.String(), "llm_response_cache") {
		t.Fatalf("nil cache stats should be omitted, got %s", rec.Body.String())
	}
}
