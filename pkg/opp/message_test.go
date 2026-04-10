package opp

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestParseMessage_RoundTrip(t *testing.T) {
	original := NewIntent("agent-a", "agent-b", "sess-1", IntentEnvelope{
		Name: "ops.deploy", Version: "1.0",
	})

	data, err := original.Bytes()
	if err != nil {
		t.Fatalf("Bytes() failed: %v", err)
	}

	parsed, err := ParseMessage(data)
	if err != nil {
		t.Fatalf("ParseMessage() failed: %v", err)
	}

	if parsed.Type != MsgIntent {
		t.Errorf("Type = %s, want INTENT", parsed.Type)
	}

	intent, err := parsed.DecodeIntent()
	if err != nil {
		t.Fatalf("DecodeIntent() failed: %v", err)
	}
	if intent.Intent.Name != "ops.deploy" {
		t.Errorf("intent.name = %s, want ops.deploy", intent.Intent.Name)
	}
}

func TestParseMessage_InvalidJSON(t *testing.T) {
	_, err := ParseMessage([]byte("not json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestMessage_DecodePayload_Empty(t *testing.T) {
	msg := &Message{Payload: json.RawMessage{}}
	var target map[string]any
	if err := msg.DecodePayload(&target); err != nil {
		t.Errorf("DecodePayload on empty should return nil, got %v", err)
	}
}

func TestMessage_Validate(t *testing.T) {
	valid := NewIntent("a", "b", "s", IntentEnvelope{Name: "x", Version: "1"})
	if err := valid.Validate(); err != nil {
		t.Errorf("expected valid, got %v", err)
	}

	invalid := *valid
	invalid.Source = ""
	if err := invalid.Validate(); !errors.Is(err, ErrValidation) {
		t.Errorf("expected ErrValidation for missing source, got %v", err)
	}
}

func TestIntentPayload_Validate(t *testing.T) {
	good := IntentPayload{Intent: IntentEnvelope{Name: "x", Version: "1"}}
	if err := good.Validate(); err != nil {
		t.Errorf("expected valid, got %v", err)
	}

	bad := IntentPayload{Intent: IntentEnvelope{Name: "x"}}
	if err := bad.Validate(); !errors.Is(err, ErrValidation) {
		t.Errorf("expected ErrValidation for missing version")
	}
}

func TestCapabilities_RoundTrip(t *testing.T) {
	msg := NewCapabilities("agent-a", "hub", "s1", CapabilitiesPayload{
		AgentID:     "agent-a",
		DisplayName: "云雀Agent",
		Intents:     []string{"ops.deploy", "file.read"},
		Models: []ModelInfo{
			{ID: "qwen-7b", Provider: "ollama", Tier: "fast", Local: true, Features: []string{"code"}},
			{ID: "glm-4", Provider: "api", Tier: "smart", Features: []string{"vision", "function_calling"}},
		},
		Adapters: []AdapterInfo{
			{ID: "lora-finance-v2", Name: "金融专家", BaseModel: "qwen-7b", Type: "lora", Domain: "finance", Rank: 16},
		},
	})

	data, err := msg.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseMessage(data)
	if err != nil {
		t.Fatal(err)
	}
	caps, err := parsed.DecodeCapabilities()
	if err != nil {
		t.Fatal(err)
	}
	if caps.AgentID != "agent-a" {
		t.Errorf("agent_id = %s, want agent-a", caps.AgentID)
	}
	if len(caps.Models) != 2 {
		t.Fatalf("models count = %d, want 2", len(caps.Models))
	}
	if caps.Models[0].ID != "qwen-7b" {
		t.Errorf("model[0].id = %s, want qwen-7b", caps.Models[0].ID)
	}
	if len(caps.Adapters) != 1 || caps.Adapters[0].Domain != "finance" {
		t.Errorf("adapter domain mismatch")
	}
}

func TestDelegate_RoundTrip(t *testing.T) {
	msg := NewDelegate("hub", "worker-a", "s1", DelegatePayload{
		Intent: IntentEnvelope{Name: "code.review", Version: "1.0", Payload: map[string]string{"repo": "main"}},
		ModelRequirements: &ModelRequirements{
			MinTier:       "smart",
			Features:      []string{"code", "long_context"},
			PreferAdapter: "code",
		},
		FallbackAgents: []string{"worker-b", "worker-c"},
	})
	data, _ := msg.Bytes()
	parsed, _ := ParseMessage(data)
	dp, err := parsed.DecodeDelegate()
	if err != nil {
		t.Fatal(err)
	}
	if dp.Intent.Name != "code.review" {
		t.Errorf("intent = %s, want code.review", dp.Intent.Name)
	}
	if dp.ModelRequirements == nil || dp.ModelRequirements.MinTier != "smart" {
		t.Error("model requirements missing or wrong tier")
	}
	if len(dp.FallbackAgents) != 2 {
		t.Errorf("fallback_agents = %d, want 2", len(dp.FallbackAgents))
	}
}

func TestFeedback_Validate(t *testing.T) {
	good := FeedbackPayload{TaskID: "t1", Rating: 0.85}
	if err := good.Validate(); err != nil {
		t.Errorf("expected valid, got %v", err)
	}
	bad := FeedbackPayload{TaskID: "", Rating: 0.5}
	if err := bad.Validate(); !errors.Is(err, ErrValidation) {
		t.Error("expected validation error for empty task_id")
	}
	outOfRange := FeedbackPayload{TaskID: "t1", Rating: 1.5}
	if err := outOfRange.Validate(); !errors.Is(err, ErrValidation) {
		t.Error("expected validation error for rating > 1.0")
	}
}

func TestIntentWithModel_RoundTrip(t *testing.T) {
	msg := NewIntentWithModel("caller", "agent", "s1",
		IntentEnvelope{Name: "data.analyze", Version: "1.0"},
		ModelRequirements{MinTier: "expert", Features: []string{"vision"}, PreferLocal: true},
	)
	data, _ := msg.Bytes()
	parsed, _ := ParseMessage(data)
	ip, _ := parsed.DecodeIntent()
	if ip.ModelRequirements == nil {
		t.Fatal("model_requirements should be present")
	}
	if ip.ModelRequirements.MinTier != "expert" {
		t.Errorf("tier = %s, want expert", ip.ModelRequirements.MinTier)
	}
	if !ip.ModelRequirements.PreferLocal {
		t.Error("prefer_local should be true")
	}
}

func TestCapabilities_Validate(t *testing.T) {
	good := CapabilitiesPayload{AgentID: "a1"}
	if err := good.Validate(); err != nil {
		t.Errorf("expected valid: %v", err)
	}
	bad := CapabilitiesPayload{}
	if err := bad.Validate(); !errors.Is(err, ErrValidation) {
		t.Error("expected validation error for empty agent_id")
	}
}

func TestDelegatePayload_Validate(t *testing.T) {
	good := DelegatePayload{Intent: IntentEnvelope{Name: "x"}}
	if err := good.Validate(); err != nil {
		t.Errorf("expected valid: %v", err)
	}
	bad := DelegatePayload{Intent: IntentEnvelope{}}
	if err := bad.Validate(); !errors.Is(err, ErrValidation) {
		t.Error("expected validation error for empty intent name")
	}
}
