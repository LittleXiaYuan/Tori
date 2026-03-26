package airi

import (
	"encoding/json"
	"testing"
)

// ── Protocol Tests ──

func TestAiriEventRoundTrip(t *testing.T) {
	identity := NewModuleIdentity("test-instance-1")

	event := NewInputTextEvent("Hello from Yunque!", identity)

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	parsed, err := ParseEvent(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if parsed.Type != "input:text" {
		t.Errorf("type = %q, want %q", parsed.Type, "input:text")
	}

	textData, err := ParseData[InputTextData](parsed)
	if err != nil {
		t.Fatalf("parse data: %v", err)
	}

	if textData.Text != "Hello from Yunque!" {
		t.Errorf("text = %q, want %q", textData.Text, "Hello from Yunque!")
	}

	if parsed.Metadata == nil || parsed.Metadata.Source == nil {
		t.Fatal("metadata.source is nil")
	}
	if parsed.Metadata.Source.Plugin.ID != "yunque-agent" {
		t.Errorf("plugin ID = %q, want %q", parsed.Metadata.Source.Plugin.ID, "yunque-agent")
	}
}

func TestAuthenticateEvent(t *testing.T) {
	identity := NewModuleIdentity("test-1")
	event := NewAuthenticateEvent("my-secret-token", identity)

	if event.Type != "module:authenticate" {
		t.Errorf("type = %q, want %q", event.Type, "module:authenticate")
	}

	data, _ := ParseData[AuthenticateData](&event)
	if data.Token != "my-secret-token" {
		t.Errorf("token = %q, want %q", data.Token, "my-secret-token")
	}
}

func TestAnnounceEvent(t *testing.T) {
	identity := NewModuleIdentity("test-1")
	event := NewAnnounceEvent("yunque-agent", identity)

	if event.Type != "module:announce" {
		t.Errorf("type = %q, want %q", event.Type, "module:announce")
	}

	data, _ := ParseData[AnnounceData](&event)
	if data.Name != "yunque-agent" {
		t.Errorf("name = %q, want %q", data.Name, "yunque-agent")
	}
	if len(data.PossibleEvents) == 0 {
		t.Error("possibleEvents should not be empty")
	}
}

func TestHeartbeatEvent(t *testing.T) {
	identity := NewModuleIdentity("test-1")
	event := NewHeartbeatPingEvent(identity)

	if event.Type != "transport:connection:heartbeat" {
		t.Errorf("type = %q, want %q", event.Type, "transport:connection:heartbeat")
	}

	data, _ := ParseData[HeartbeatData](&event)
	if data.Kind != "ping" {
		t.Errorf("kind = %q, want %q", data.Kind, "ping")
	}
	if data.Message != "🩵" {
		t.Errorf("message = %q, want %q", data.Message, "🩵")
	}
}

func TestParseEventType(t *testing.T) {
	raw := []byte(`{"type":"input:text","data":{"text":"hello"}}`)
	typ, err := ParseEventType(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if typ != "input:text" {
		t.Errorf("type = %q, want %q", typ, "input:text")
	}
}

// ── Mapping Tests ──

func TestMapEmotionToVRM(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"joy", "happy"},
		{"happy", "happy"},
		{"love", "happy"},
		{"anger", "angry"},
		{"annoyance", "angry"},
		{"sadness", "sad"},
		{"grief", "sad"},
		{"fear", "surprised"},
		{"surprise", "surprised"},
		{"curiosity", "curious"},
		{"interest", "curious"},
		{"awkward", "awkward"},
		{"embarrassment", "awkward"},
		{"question", "question"},
		{"confusion", "question"},
		{"think", "think"},
		{"contemplation", "think"},
		{"trust", "happy"},
		{"neutral", "neutral"},
		{"unknown_emotion", "neutral"},
		{"", "neutral"},
	}

	for _, tt := range tests {
		got := mapEmotionToVRM(tt.input)
		if got != tt.want {
			t.Errorf("mapEmotionToVRM(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// ── trimToolCalls Tests ──

func TestTrimToolCalls(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "plain text",
			input: "Hello, how are you?",
			want:  "Hello, how are you?",
		},
		{
			name:  "tool_calls only",
			input: `{"tool_calls":[{"name":"search","arguments":{"q":"test"}}]}`,
			want:  "",
		},
		{
			name:  "tool_calls followed by text",
			input: `{"tool_calls":[{"name":"search"}]} Here is the result`,
			want:  "Here is the result",
		},
		{
			name:  "skill_calls only",
			input: `{"skill_calls":[{"name":"translate"}]}`,
			want:  "",
		},
		{
			name:  "whitespace only",
			input: "   \n\t  ",
			want:  "",
		},
		{
			name:  "empty",
			input: "",
			want:  "",
		},
		{
			name:  "regular JSON (not tool_calls)",
			input: `{"message":"hello"}`,
			want:  `{"message":"hello"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := trimToolCalls(tt.input)
			if got != tt.want {
				t.Errorf("trimToolCalls() = %q, want %q", got, tt.want)
			}
		})
	}
}
