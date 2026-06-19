package gateway

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"yunque-agent/pkg/cogni"
	"yunque-agent/pkg/packruntime"
)

func TestCogniRoute_RanksBids(t *testing.T) {
	gw, tm := newTestGatewayWithCogniKernelPack(t, packruntime.PackStatusEnabled)
	tenant := tm.Register("cogni-route")
	reg := cogni.NewRegistry()
	gw.SetCogniRegistry(reg, t.TempDir())

	bus := cogni.NewCogniBus(cogni.NewEvaluator(), cogni.DefaultBusConfig())
	bus.Register(&cogni.Declaration{
		ID: "reviewer",
		Activation: cogni.ActivationRules{
			Keywords: []string{"review"},
			MinScore: 0.2,
		},
	})
	bus.Register(&cogni.Declaration{
		ID: "translator",
		Activation: cogni.ActivationRules{
			Keywords: []string{"translate"},
			MinScore: 0.2,
		},
	})
	gw.SetCogniBus(bus)

	body, _ := json.Marshal(map[string]any{"message": "please review my code"})
	req := httptest.NewRequest(http.MethodPost, "/v1/cognis/route", bytes.NewReader(body))
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var result cogni.RouteResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(result.SelectedIDs) != 1 || result.SelectedIDs[0] != "reviewer" {
		t.Fatalf("expected reviewer to win, got %v (bids=%v)", result.SelectedIDs, result.AllBids)
	}
}

func TestCogniRoute_RequiresMessage(t *testing.T) {
	gw, tm := newTestGatewayWithCogniKernelPack(t, packruntime.PackStatusEnabled)
	tenant := tm.Register("cogni-route")
	gw.SetCogniRegistry(cogni.NewRegistry(), t.TempDir())
	gw.SetCogniBus(cogni.NewCogniBus(cogni.NewEvaluator(), cogni.DefaultBusConfig()))

	req := httptest.NewRequest(http.MethodPost, "/v1/cognis/route", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestCogniRoute_BusNotConfigured(t *testing.T) {
	gw, tm := newTestGatewayWithCogniKernelPack(t, packruntime.PackStatusEnabled)
	tenant := tm.Register("cogni-route")
	gw.SetCogniRegistry(cogni.NewRegistry(), t.TempDir())

	body, _ := json.Marshal(map[string]any{"message": "hello"})
	req := httptest.NewRequest(http.MethodPost, "/v1/cognis/route", bytes.NewReader(body))
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
}
