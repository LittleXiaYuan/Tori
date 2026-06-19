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

func TestCogniKernelPackServesFederationAndEconomics(t *testing.T) {
	gw, tm := newTestGatewayWithCogniKernelPack(t, packruntime.PackStatusEnabled)
	tenant := tm.Register("cogni-federation")
	registry := cogni.NewRegistry()
	if err := registry.Add(&cogni.Declaration{ID: "reviewer"}, "test"); err != nil {
		t.Fatalf("add cogni: %v", err)
	}
	gw.SetCogniRegistry(registry, t.TempDir())
	federation := cogni.NewCogniFederation("local", "http://local", registry)
	gw.SetCogniFederation(federation)
	tracker := cogni.NewCostTracker()
	tracker.Record(cogni.CostEntry{CogniID: "reviewer", Cost: 0.5, Tokens: 1000, Operation: "route"})
	gw.SetCogniCostTracker(tracker)

	req := httptest.NewRequest(http.MethodGet, "/v1/cognis/federation", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("federation status=%d body=%s", w.Code, w.Body.String())
	}
	var status map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode federation status: %v", err)
	}
	if status["enabled"] != true || status["self_id"] != "local" {
		t.Fatalf("unexpected federation status: %#v", status)
	}

	body, _ := json.Marshal(cogni.FederationPeer{ID: "peer-1", Name: "Peer", URL: "http://peer"})
	req = httptest.NewRequest(http.MethodPost, "/v1/cognis/federation/peers", bytes.NewReader(body))
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("add peer status=%d body=%s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/cognis/economics", nil)
	req.Header.Set("X-API-Key", tenant.APIKey)
	w = httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("economics status=%d body=%s", w.Code, w.Body.String())
	}
	var economics map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &economics); err != nil {
		t.Fatalf("decode economics: %v", err)
	}
	if economics["enabled"] != true {
		t.Fatalf("unexpected economics status: %#v", economics)
	}
}
