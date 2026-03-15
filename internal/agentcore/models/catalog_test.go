package models

import (
	"testing"
)

func newTestCatalog() *Catalog {
	c := NewCatalog()
	c.LoadBuiltinCatalog()
	return c
}

func TestCatalogLoadBuiltin(t *testing.T) {
	c := newTestCatalog()
	if c.Count() < 10 {
		t.Errorf("expected at least 10 builtin models, got %d", c.Count())
	}
}

func TestCatalogGet(t *testing.T) {
	c := newTestCatalog()
	e, ok := c.Get("gpt-4o")
	if !ok {
		t.Fatal("gpt-4o not found")
	}
	if e.Provider != ProviderOpenAI {
		t.Errorf("expected openai, got %s", e.Provider)
	}
	if e.ContextWindow != 128000 {
		t.Errorf("expected 128000, got %d", e.ContextWindow)
	}
}

func TestCatalogGetByAlias(t *testing.T) {
	c := newTestCatalog()
	e, ok := c.Get("gpt4o")
	if !ok {
		t.Fatal("alias gpt4o not found")
	}
	if e.ModelID != "gpt-4o" {
		t.Errorf("expected gpt-4o, got %s", e.ModelID)
	}

	e2, ok := c.Get("deepseek-r1")
	if !ok {
		t.Fatal("alias deepseek-r1 not found")
	}
	if e2.ModelID != "deepseek-reasoner" {
		t.Errorf("expected deepseek-reasoner, got %s", e2.ModelID)
	}
}

func TestCatalogFindByCapabilities(t *testing.T) {
	c := newTestCatalog()

	// Find vision models
	vision := c.FindByCapabilities(CapVision)
	if len(vision) == 0 {
		t.Fatal("expected vision-capable models")
	}
	for _, m := range vision {
		if !m.HasCapability(CapVision) {
			t.Errorf("%s should have vision capability", m.ModelID)
		}
	}

	// Find reasoning + coding
	reasoning := c.FindByCapabilities(CapReasoning, CapCoding)
	if len(reasoning) == 0 {
		t.Fatal("expected reasoning+coding models")
	}
	for _, m := range reasoning {
		if !m.HasAllCapabilities(CapReasoning, CapCoding) {
			t.Errorf("%s missing required capabilities", m.ModelID)
		}
	}
}

func TestCatalogFindByProvider(t *testing.T) {
	c := newTestCatalog()
	ds := c.FindByProvider(ProviderDeepSeek)
	if len(ds) < 2 {
		t.Errorf("expected at least 2 DeepSeek models, got %d", len(ds))
	}
	for _, m := range ds {
		if m.Provider != ProviderDeepSeek {
			t.Errorf("expected deepseek provider, got %s", m.Provider)
		}
	}
}

func TestCatalogFindByTier(t *testing.T) {
	c := newTestCatalog()
	economy := c.FindByTier(TierEconomy)
	if len(economy) == 0 {
		t.Fatal("expected economy tier models")
	}
	for _, m := range economy {
		if m.Tier != TierEconomy {
			t.Errorf("expected economy tier, got %d", m.Tier)
		}
	}
}

func TestCatalogFindCheapest(t *testing.T) {
	c := newTestCatalog()
	cheapest, ok := c.FindCheapest(CapChat, CapStreaming)
	if !ok {
		t.Fatal("expected at least one chat+streaming model")
	}
	if cheapest.Pricing.InputPerMToken <= 0 {
		t.Error("expected positive pricing")
	}
}

func TestCatalogFindBest(t *testing.T) {
	c := newTestCatalog()
	best, ok := c.FindBest(CapChat, CapToolUse)
	if !ok {
		t.Fatal("expected at least one chat+tool_use model")
	}
	if best.Tier != TierPremium {
		t.Errorf("expected premium tier for best, got %d", best.Tier)
	}
}

func TestCatalogSearch(t *testing.T) {
	c := newTestCatalog()
	results := c.Search("deepseek")
	if len(results) < 2 {
		t.Errorf("expected at least 2 deepseek results, got %d", len(results))
	}

	results = c.Search("claude")
	if len(results) < 1 {
		t.Errorf("expected at least 1 claude result, got %d", len(results))
	}
}

func TestCatalogEstimateCost(t *testing.T) {
	c := newTestCatalog()
	e, _ := c.Get("gpt-4o")
	cost := e.EstimateCost(1000, 500)
	if cost <= 0 {
		t.Error("expected positive cost estimate")
	}
	// 1000 input tokens * 2.5/1M + 500 output tokens * 10.0/1M
	expected := 1000.0/1_000_000*2.5 + 500.0/1_000_000*10.0
	if cost != expected {
		t.Errorf("expected %.10f, got %.10f", expected, cost)
	}
}

func TestCatalogStats(t *testing.T) {
	c := newTestCatalog()
	stats := c.Stats()
	total := stats["total"].(int)
	if total < 10 {
		t.Errorf("expected at least 10, got %d", total)
	}
	providers := stats["providers"].(map[ProviderName]int)
	if providers[ProviderOpenAI] < 2 {
		t.Error("expected at least 2 OpenAI models")
	}
}

func TestCatalogDeprecated(t *testing.T) {
	c := NewCatalog()
	c.Add(CatalogEntry{
		ModelID: "old-model", DisplayName: "Old", Provider: ProviderCustom,
		Tier: TierEconomy, Capabilities: []Capability{CapChat},
		Deprecated: true,
	})
	c.Add(CatalogEntry{
		ModelID: "new-model", DisplayName: "New", Provider: ProviderCustom,
		Tier: TierEconomy, Capabilities: []Capability{CapChat},
	})
	results := c.FindByCapabilities(CapChat)
	if len(results) != 1 {
		t.Errorf("expected 1 non-deprecated, got %d", len(results))
	}
	if results[0].ModelID != "new-model" {
		t.Error("expected new-model only")
	}
}

func TestCatalogHasCapability(t *testing.T) {
	e := CatalogEntry{
		Capabilities: []Capability{CapChat, CapVision, CapToolUse},
	}
	if !e.HasCapability(CapChat) {
		t.Error("should have chat")
	}
	if e.HasCapability(CapReasoning) {
		t.Error("should not have reasoning")
	}
	if !e.HasAllCapabilities(CapChat, CapVision) {
		t.Error("should have chat+vision")
	}
	if e.HasAllCapabilities(CapChat, CapReasoning) {
		t.Error("should not have chat+reasoning")
	}
}

func TestCatalogAll(t *testing.T) {
	c := newTestCatalog()
	all := c.All()
	if len(all) != c.Count() {
		t.Errorf("All() count mismatch: %d vs %d", len(all), c.Count())
	}
}
