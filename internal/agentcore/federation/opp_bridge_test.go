package federation

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"yunque-agent/pkg/opp"
)

func newBridgeTestHub() (*Hub, *OPPBridge) {
	hub := NewHub(HubConfig{
		LocalAgent:    "yunque-main",
		LocalInstance: "localhost:9090",
		Secret:        "test-key",
	})
	bridge := NewOPPBridge(hub, opp.CapabilitiesPayload{
		AgentID:     "yunque-main",
		DisplayName: "云雀主Agent",
		Intents:     []string{"chat", "code.review"},
		Models: []opp.ModelInfo{
			{ID: "qwen-7b", Provider: "ollama", Tier: "fast", Local: true, Features: []string{"code"}},
		},
	})
	return hub, bridge
}

func TestOPPBridge_LocalCaps(t *testing.T) {
	_, bridge := newBridgeTestHub()
	caps := bridge.LocalCaps()
	if caps.AgentID != "yunque-main" {
		t.Errorf("agent_id = %s, want yunque-main", caps.AgentID)
	}
	if len(caps.Models) != 1 {
		t.Errorf("models = %d, want 1", len(caps.Models))
	}
}

func TestOPPBridge_UpdateLocalCaps(t *testing.T) {
	_, bridge := newBridgeTestHub()
	bridge.UpdateLocalCaps(opp.CapabilitiesPayload{
		AgentID: "updated",
		Models: []opp.ModelInfo{
			{ID: "glm-4", Tier: "smart"},
			{ID: "qwen-7b", Tier: "fast"},
		},
	})
	caps := bridge.LocalCaps()
	if caps.AgentID != "updated" {
		t.Errorf("agent_id = %s, want updated", caps.AgentID)
	}
	if len(caps.Models) != 2 {
		t.Errorf("models = %d, want 2", len(caps.Models))
	}
}

func TestOPPBridge_HandleDiscover(t *testing.T) {
	hub, bridge := newBridgeTestHub()
	_ = bridge

	msg := Message{
		ID: "d1", Type: MsgDiscover, From: "peer@remote:8080",
		To: hub.LocalID(), Payload: "", Timestamp: time.Now(), TTL: 3,
	}
	msg.Signature = hub.sign(msg)
	reply, err := hub.Receive(context.Background(), msg)
	if err != nil {
		t.Fatal(err)
	}
	if reply.Type != MsgCapReply {
		t.Errorf("type = %s, want cap_reply", reply.Type)
	}

	var caps opp.CapabilitiesPayload
	if err := json.Unmarshal([]byte(reply.Payload), &caps); err != nil {
		t.Fatal(err)
	}
	if caps.AgentID != "yunque-main" {
		t.Errorf("agent_id = %s, want yunque-main", caps.AgentID)
	}
}

func TestOPPBridge_HandleCapReply(t *testing.T) {
	hub, bridge := newBridgeTestHub()
	hub.AddPeer("finance-bot@remote:8080", []string{})

	capsJSON, _ := json.Marshal(opp.CapabilitiesPayload{
		AgentID: "finance-bot",
		Models: []opp.ModelInfo{
			{ID: "glm-4-finance", Tier: "expert", Features: []string{"code", "vision"}},
		},
		Adapters: []opp.AdapterInfo{
			{ID: "lora-fin", Domain: "finance", Rank: 16},
		},
		Intents: []string{"finance.report", "data.analyze"},
	})

	msg := Message{
		ID: "c1", Type: MsgCapReply, From: "finance-bot@remote:8080",
		To: hub.LocalID(), Payload: string(capsJSON), Timestamp: time.Now(), TTL: 3,
	}
	msg.Signature = hub.sign(msg)
	_, err := hub.Receive(context.Background(), msg)
	if err != nil {
		t.Fatal(err)
	}

	peers := bridge.ListPeerCaps()
	if len(peers) != 1 {
		t.Fatalf("peers = %d, want 1", len(peers))
	}
	if peers[0].Payload.AgentID != "finance-bot" {
		t.Errorf("peer agent_id = %s", peers[0].Payload.AgentID)
	}
}

func setupPeeredBridge(t *testing.T) *OPPBridge {
	t.Helper()
	hub, bridge := newBridgeTestHub()
	hub.AddPeer("finance@r:1", []string{})
	hub.AddPeer("vision@r:2", []string{})
	hub.AddPeer("code@r:3", []string{})

	bridge.mu.Lock()
	bridge.caps["finance@r:1"] = &PeerCapabilities{
		PeerID: "finance@r:1", Healthy: true, Latency: 50 * time.Millisecond,
		Payload: opp.CapabilitiesPayload{
			AgentID:  "finance-bot",
			Models:   []opp.ModelInfo{{ID: "glm-4", Tier: "expert", Features: []string{"code", "function_calling"}}},
			Adapters: []opp.AdapterInfo{{ID: "lora-fin", Domain: "finance", Rank: 16}},
			Intents:  []string{"finance.report"},
		},
	}
	bridge.caps["vision@r:2"] = &PeerCapabilities{
		PeerID: "vision@r:2", Healthy: true, Latency: 100 * time.Millisecond,
		Payload: opp.CapabilitiesPayload{
			AgentID: "vision-bot",
			Models:  []opp.ModelInfo{{ID: "qwen-vl", Tier: "smart", Features: []string{"vision", "code"}, Local: true}},
			Intents: []string{"image.analyze"},
		},
	}
	bridge.caps["code@r:3"] = &PeerCapabilities{
		PeerID: "code@r:3", Healthy: true, Latency: 30 * time.Millisecond,
		Payload: opp.CapabilitiesPayload{
			AgentID:  "code-bot",
			Models:   []opp.ModelInfo{{ID: "deepseek-coder", Tier: "smart", Features: []string{"code", "long_context"}, MaxCtx: 32000}},
			Adapters: []opp.AdapterInfo{{ID: "lora-code", Domain: "code", Rank: 32}},
			Intents:  []string{"code.review", "code.generate"},
		},
	}
	bridge.mu.Unlock()
	return bridge
}

func TestOPPBridge_FindByFeature(t *testing.T) {
	bridge := setupPeeredBridge(t)

	vision := bridge.FindByFeature("vision")
	if len(vision) != 1 || vision[0].Payload.AgentID != "vision-bot" {
		t.Errorf("vision search: got %d results", len(vision))
	}

	code := bridge.FindByFeature("code")
	if len(code) != 3 {
		t.Errorf("code search: got %d, want 3", len(code))
	}
}

func TestOPPBridge_FindByAdapter(t *testing.T) {
	bridge := setupPeeredBridge(t)

	fin := bridge.FindByAdapter("finance")
	if len(fin) != 1 || fin[0].Payload.AgentID != "finance-bot" {
		t.Errorf("finance adapter: got %d results", len(fin))
	}

	none := bridge.FindByAdapter("medical")
	if len(none) != 0 {
		t.Errorf("medical adapter: got %d, want 0", len(none))
	}
}

func TestOPPBridge_FindByIntent(t *testing.T) {
	bridge := setupPeeredBridge(t)

	cr := bridge.FindByIntent("code.review")
	if len(cr) != 1 || cr[0].Payload.AgentID != "code-bot" {
		t.Errorf("code.review intent: got %d results", len(cr))
	}
}

func TestOPPBridge_Route_BasicTier(t *testing.T) {
	bridge := setupPeeredBridge(t)

	results := bridge.Route(opp.ModelRequirements{MinTier: "expert"})
	if len(results) != 1 {
		t.Fatalf("expert tier: got %d, want 1 (only finance-bot)", len(results))
	}
	if results[0].AgentID != "finance-bot" {
		t.Errorf("best = %s, want finance-bot", results[0].AgentID)
	}
}

func TestOPPBridge_Route_FeatureMatch(t *testing.T) {
	bridge := setupPeeredBridge(t)

	results := bridge.Route(opp.ModelRequirements{
		MinTier:  "smart",
		Features: []string{"code", "long_context"},
	})
	if len(results) == 0 {
		t.Fatal("expected at least 1 result for code+long_context")
	}
	if results[0].AgentID != "code-bot" {
		t.Errorf("best = %s, want code-bot (has long_context)", results[0].AgentID)
	}
}

func TestOPPBridge_Route_PreferLocal(t *testing.T) {
	bridge := setupPeeredBridge(t)

	results := bridge.Route(opp.ModelRequirements{
		MinTier:     "smart",
		Features:    []string{"code"},
		PreferLocal: true,
	})
	if len(results) == 0 {
		t.Fatal("expected results")
	}
	if results[0].AgentID != "vision-bot" {
		t.Errorf("best = %s, want vision-bot (local model preferred)", results[0].AgentID)
	}
}

func TestOPPBridge_Route_PreferAdapter(t *testing.T) {
	bridge := setupPeeredBridge(t)

	results := bridge.Route(opp.ModelRequirements{
		MinTier:       "smart",
		Features:      []string{"code"},
		PreferAdapter: "code",
	})
	if len(results) == 0 {
		t.Fatal("expected results")
	}
	if results[0].AgentID != "code-bot" {
		t.Errorf("best = %s, want code-bot (code adapter preferred)", results[0].AgentID)
	}
}

func TestOPPBridge_Route_NoMatch(t *testing.T) {
	bridge := setupPeeredBridge(t)

	results := bridge.Route(opp.ModelRequirements{
		MinTier:  "expert",
		Features: []string{"vision"},
	})
	if len(results) != 0 {
		t.Errorf("got %d results, want 0 (no expert with vision)", len(results))
	}
}

func TestOPPBridge_Stats(t *testing.T) {
	bridge := setupPeeredBridge(t)
	stats := bridge.Stats()

	if stats["peers_with_caps"].(int) != 3 {
		t.Errorf("peers_with_caps = %v, want 3", stats["peers_with_caps"])
	}
	if stats["total_adapters"].(int) != 2 {
		t.Errorf("total_adapters = %v, want 2", stats["total_adapters"])
	}
}

func TestOPPBridge_HandleTask_NoHandler(t *testing.T) {
	hub, bridge := newBridgeTestHub()
	_ = bridge
	hub.AddPeer("caller@remote:8080", []string{})

	dp := opp.DelegatePayload{Intent: opp.IntentEnvelope{Name: "test"}}
	payload, _ := json.Marshal(dp)

	msg := Message{
		ID: "t1", Type: MsgTask, From: "caller@remote:8080",
		To: hub.LocalID(), Payload: string(payload), Timestamp: time.Now(), TTL: 3,
	}
	msg.Signature = hub.sign(msg)
	reply, err := hub.Receive(context.Background(), msg)
	if err != nil {
		t.Fatal(err)
	}
	if reply == nil {
		t.Fatal("expected reply")
	}
	if reply.Type != MsgResult {
		t.Errorf("type = %s, want result", reply.Type)
	}
}

func TestOPPBridge_HandleTask_WithHandler(t *testing.T) {
	hub, bridge := newBridgeTestHub()
	hub.AddPeer("caller@remote:8080", []string{})

	bridge.SetDelegateHandler(func(ctx context.Context, dp opp.DelegatePayload) (*opp.DelegateResultPayload, error) {
		return &opp.DelegateResultPayload{
			DelegatedTo: "yunque-main",
			Result:      opp.ResultPayload{Status: "success", Output: "done: " + dp.Intent.Name},
		}, nil
	})

	dp := opp.DelegatePayload{Intent: opp.IntentEnvelope{Name: "code.review"}}
	payload, _ := json.Marshal(dp)

	msg := Message{
		ID: "t2", Type: MsgTask, From: "caller@remote:8080",
		To: hub.LocalID(), Payload: string(payload), Timestamp: time.Now(), TTL: 3,
	}
	msg.Signature = hub.sign(msg)
	reply, err := hub.Receive(context.Background(), msg)
	if err != nil {
		t.Fatal(err)
	}

	var result opp.DelegateResultPayload
	if err := json.Unmarshal([]byte(reply.Payload), &result); err != nil {
		t.Fatal(err)
	}
	if result.Result.Status != "success" {
		t.Errorf("status = %s", result.Result.Status)
	}
	if result.Result.Output != "done: code.review" {
		t.Errorf("output = %s", result.Result.Output)
	}
}

func TestPlannerAdapter_Interface(t *testing.T) {
	hub, bridge := newBridgeTestHub()
	transport := NewTransport(hub)
	adapter := NewPlannerAdapter(bridge, transport)

	caps := adapter.LocalCaps()
	if caps.AgentID != "yunque-main" {
		t.Errorf("adapter.LocalCaps().AgentID = %s", caps.AgentID)
	}
}
