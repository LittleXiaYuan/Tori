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
