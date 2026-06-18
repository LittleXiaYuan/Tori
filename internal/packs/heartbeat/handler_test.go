package heartbeatpack

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"yunque-agent/internal/agentcore/runtime/heartbeat"
)

func TestHeartbeatPackStatusNilService(t *testing.T) {
	h := NewProvider(nil)
	req := httptest.NewRequest(http.MethodGet, "/v1/heartbeat", nil)
	rec := httptest.NewRecorder()

	h.Status(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHeartbeatPackTriggerAndLogs(t *testing.T) {
	svc := heartbeat.New(heartbeat.Config{Interval: time.Hour, MaxLogs: 10}, func(_ context.Context) (string, error) {
		return "ok", nil
	})
	h := NewProvider(func() *heartbeat.Service { return svc })

	triggerReq := httptest.NewRequest(http.MethodPost, "/v1/heartbeat/trigger", nil)
	triggerRec := httptest.NewRecorder()
	h.Trigger(triggerRec, triggerReq)
	if triggerRec.Code != http.StatusOK {
		t.Fatalf("trigger status = %d body=%s", triggerRec.Code, triggerRec.Body.String())
	}

	logReq := httptest.NewRequest(http.MethodGet, "/v1/heartbeat/logs?limit=1", nil)
	logRec := httptest.NewRecorder()
	h.Logs(logRec, logReq)
	if logRec.Code != http.StatusOK {
		t.Fatalf("logs status = %d body=%s", logRec.Code, logRec.Body.String())
	}
	var logs []heartbeat.Log
	if err := json.Unmarshal(logRec.Body.Bytes(), &logs); err != nil {
		t.Fatal(err)
	}
	if len(logs) != 1 || logs[0].Status != "ok" {
		t.Fatalf("unexpected logs: %+v", logs)
	}
}

func TestHeartbeatPackRoutesAndSpecsStayAligned(t *testing.T) {
	routes := map[string]map[string]bool{}
	for _, route := range (&Handler{}).Routes() {
		if routes[route.Path] == nil {
			routes[route.Path] = map[string]bool{}
		}
		if route.Method != "" {
			routes[route.Path][route.Method] = true
		}
		for _, method := range route.Methods {
			routes[route.Path][method] = true
		}
	}
	for _, spec := range RouteSpecs() {
		if !routes[spec.Path][spec.Method] {
			t.Fatalf("routeSpec %s %s not mounted by Routes()", spec.Method, spec.Path)
		}
	}
}
