package subagent

import (
	"context"
	"fmt"
	"testing"
)

func TestHandoffRegistry_RegisterAndList(t *testing.T) {
	mgr := NewManager()
	hr := NewHandoffRegistry(mgr)

	err := hr.Register(HandoffConfig{Name: "weather", Description: "Weather lookups"})
	if err != nil {
		t.Fatal(err)
	}
	err = hr.Register(HandoffConfig{Name: "docs", Description: "Documentation search", ProviderID: "gpt4"})
	if err != nil {
		t.Fatal(err)
	}

	list := hr.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 configs, got %d", len(list))
	}
}

func TestHandoffRegistry_RegisterEmpty(t *testing.T) {
	mgr := NewManager()
	hr := NewHandoffRegistry(mgr)
	err := hr.Register(HandoffConfig{})
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestHandoffRegistry_GetAndUnregister(t *testing.T) {
	mgr := NewManager()
	hr := NewHandoffRegistry(mgr)
	hr.Register(HandoffConfig{Name: "calc", Description: "Calculator"})

	cfg, ok := hr.Get("calc")
	if !ok || cfg.Name != "calc" {
		t.Fatal("expected to find calc config")
	}

	_, ok = hr.Get("nonexistent")
	if ok {
		t.Fatal("expected not found")
	}

	if !hr.Unregister("calc") {
		t.Fatal("expected unregister to succeed")
	}
	if hr.Unregister("calc") {
		t.Fatal("expected second unregister to fail")
	}
}

func TestHandoffRegistry_ToolNames(t *testing.T) {
	mgr := NewManager()
	hr := NewHandoffRegistry(mgr)
	hr.Register(HandoffConfig{Name: "weather"})
	hr.Register(HandoffConfig{Name: "docs"})

	names := hr.ToolNames()
	if len(names) != 2 {
		t.Fatalf("expected 2 tool names, got %d", len(names))
	}

	found := map[string]bool{}
	for _, n := range names {
		found[n] = true
	}
	if !found["transfer_to_weather"] || !found["transfer_to_docs"] {
		t.Fatalf("unexpected tool names: %v", names)
	}
}

func TestHandoffRegistry_ToolDefinitions(t *testing.T) {
	mgr := NewManager()
	hr := NewHandoffRegistry(mgr)
	hr.Register(HandoffConfig{Name: "weather", Description: "Get weather info"})

	defs := hr.ToolDefinitions()
	if len(defs) != 1 {
		t.Fatalf("expected 1 definition, got %d", len(defs))
	}
	fn := defs[0]["function"].(map[string]any)
	if fn["name"] != "transfer_to_weather" {
		t.Fatalf("unexpected name: %v", fn["name"])
	}
}

func TestHandoffRegistry_IsHandoffCall(t *testing.T) {
	mgr := NewManager()
	hr := NewHandoffRegistry(mgr)
	hr.Register(HandoffConfig{Name: "weather"})

	name, ok := hr.IsHandoffCall("transfer_to_weather")
	if !ok || name != "weather" {
		t.Fatalf("expected weather, got %q ok=%v", name, ok)
	}

	_, ok = hr.IsHandoffCall("transfer_to_nonexist")
	if ok {
		t.Fatal("expected not found for unregistered agent")
	}

	_, ok = hr.IsHandoffCall("some_other_skill")
	if ok {
		t.Fatal("expected false for non-handoff skill")
	}
}

func TestHandoffRegistry_Execute(t *testing.T) {
	mgr := NewManager()
	hr := NewHandoffRegistry(mgr)
	hr.Register(HandoffConfig{Name: "weather", Description: "Weather agent", ProviderID: "gpt4"})

	// Set run function that echoes input
	hr.SetRunFunc(func(ctx context.Context, agentName, input, providerOverride string) (string, error) {
		if agentName != "weather" {
			return "", fmt.Errorf("unexpected agent: %s", agentName)
		}
		if providerOverride != "gpt4" {
			return "", fmt.Errorf("expected provider override gpt4, got %s", providerOverride)
		}
		return "北京今天晴，25°C", nil
	})

	result, err := hr.Execute(context.Background(), "parent-1", "weather", "北京今天天气怎么样")
	if err != nil {
		t.Fatal(err)
	}
	if result.AgentName != "weather" {
		t.Fatalf("expected agent weather, got %s", result.AgentName)
	}
	if result.Reply != "北京今天晴，25°C" {
		t.Fatalf("unexpected reply: %s", result.Reply)
	}

	// After execute, subagent should be destroyed (cleanup)
	if mgr.Count() != 0 {
		t.Fatalf("expected 0 subagents after cleanup, got %d", mgr.Count())
	}
}

func TestHandoffRegistry_ExecuteNotRegistered(t *testing.T) {
	mgr := NewManager()
	hr := NewHandoffRegistry(mgr)
	hr.SetRunFunc(func(ctx context.Context, agentName, input, providerOverride string) (string, error) {
		return "ok", nil
	})

	_, err := hr.Execute(context.Background(), "parent", "nonexistent", "test")
	if err == nil {
		t.Fatal("expected error for unregistered agent")
	}
}

func TestHandoffRegistry_ExecuteNoRunFunc(t *testing.T) {
	mgr := NewManager()
	hr := NewHandoffRegistry(mgr)
	hr.Register(HandoffConfig{Name: "test"})

	_, err := hr.Execute(context.Background(), "parent", "test", "input")
	if err == nil {
		t.Fatal("expected error when run function not set")
	}
}

func TestHandoffRegistry_ExecuteError(t *testing.T) {
	mgr := NewManager()
	hr := NewHandoffRegistry(mgr)
	hr.Register(HandoffConfig{Name: "failing"})
	hr.SetRunFunc(func(ctx context.Context, agentName, input, providerOverride string) (string, error) {
		return "", fmt.Errorf("LLM timeout")
	})

	_, err := hr.Execute(context.Background(), "parent", "failing", "input")
	if err == nil {
		t.Fatal("expected error on execution failure")
	}
}
