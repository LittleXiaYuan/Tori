package planner

import (
	"context"
	"encoding/json"
	"testing"
)

// TestConversationPair_PersonaMemorySchema locks the JSON keys that the
// self-distill stepCollect reader depends on. If these drift, the online
// training-data → distillation chain silently breaks (the exporter would train
// on a generic assistant instead of the captured persona + memory).
func TestConversationPair_PersonaMemorySchema(t *testing.T) {
	p := conversationPair{
		UserMessage:    "帮我写个周报",
		AssistReply:    "好的，这是你的周报…",
		Persona:        "你是小羽。",
		RecalledMemory: "<recalled_memories>用户偏好简洁</recalled_memories>",
	}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m["user_message"] != "帮我写个周报" {
		t.Fatalf("user_message key wrong: %v", m["user_message"])
	}
	if m["assist_reply"] != "好的，这是你的周报…" {
		t.Fatalf("assist_reply key wrong: %v", m["assist_reply"])
	}
	if m["persona"] != "你是小羽。" {
		t.Fatalf("persona key wrong: %v", m["persona"])
	}
	if m["recalled_memories"] == nil {
		t.Fatalf("recalled_memories key missing: %v", m)
	}
}

// TestDataCollector_ProvidersOptional ensures persona/memory providers are
// nil-safe (the collector works with or without them).
func TestDataCollector_ProvidersOptional(t *testing.T) {
	dc := NewDataCollector(nil, DataCollectorConfig{Enabled: true})
	dc.SetPersonaProvider(nil)
	dc.SetMemoryProvider(nil)
	dc.SetPersonaProvider(func() string { return "你是小羽。" })
	dc.SetMemoryProvider(func(context.Context, string, string) string { return "mem" })
	if dc.personaFunc == nil || dc.memoryFunc == nil {
		t.Fatal("providers should be set")
	}
}
