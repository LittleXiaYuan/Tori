package planner

import (
	"testing"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/localbrain"
)

func TestRuntimeStrategyServiceModes(t *testing.T) {
	service := NewRuntimeStrategyService()
	if service.ReActMode() || service.LongHorizonMode() {
		t.Fatal("expected modes disabled by default")
	}
	service.SetReActMode(true)
	service.SetLongHorizonMode(true)
	if !service.ReActMode() || !service.LongHorizonMode() {
		t.Fatal("expected mode setters to enable runtime strategies")
	}
}

func TestRuntimeStrategyServiceLocalBrainGetter(t *testing.T) {
	brain := localbrain.New(nil, nil)
	service := NewRuntimeStrategyService()
	service.SetLocalBrain(brain)
	if service.LocalBrain() != brain {
		t.Fatal("expected attached local brain")
	}
}

func TestRuntimeStrategyServiceSelectProviderByCapability(t *testing.T) {
	reg := llm.NewProviderRegistry(llm.NewPool())
	if err := reg.Register(llm.ProviderConfig{
		ID:           "vision-provider",
		Type:         llm.ProviderTypeChat,
		BaseURL:      "http://example.invalid",
		Model:        "vision-model",
		Enabled:      true,
		Capabilities: []llm.Capability{llm.CapChat, llm.CapVision},
		Priority:     1,
	}); err != nil {
		t.Fatalf("register provider: %v", err)
	}

	service := NewRuntimeStrategyService()
	service.SetProviderRegistry(reg)
	got := service.SelectProviderByCapability(llm.CapVision)
	if got == nil || got.Config.ID != "vision-provider" {
		t.Fatalf("expected vision provider, got %#v", got)
	}
}

func TestNilRuntimeStrategyServiceIsNoop(t *testing.T) {
	var service *RuntimeStrategyService
	if service.ReActMode() || service.LongHorizonMode() {
		t.Fatal("nil service should report disabled modes")
	}
	if got := service.LocalBrain(); got != nil {
		t.Fatalf("nil service should have no local brain, got %#v", got)
	}
	if got := service.AgenticThinking(); got != nil {
		t.Fatalf("nil service should have no agentic thinking, got %#v", got)
	}
	if got := service.SelectProviderByCapability(llm.CapVision); got != nil {
		t.Fatalf("nil service should select no provider, got %#v", got)
	}
}
