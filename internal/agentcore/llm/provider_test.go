package llm

import (
	"context"
	"testing"
)

func TestProviderRegistryRegisterAndList(t *testing.T) {
	reg := NewProviderRegistry(nil)
	err := reg.Register(ProviderConfig{
		ID:      "test-openai",
		Type:    ProviderTypeChat,
		BaseURL: "https://api.openai.com/v1",
		APIKeys: []string{"sk-test"},
		Model:   "gpt-4o",
		Enabled: true,
		Tier:    "smart",
	})
	if err != nil {
		t.Fatal(err)
	}

	list := reg.List()
	if len(list) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(list))
	}
	if list[0].ID != "test-openai" {
		t.Errorf("expected id test-openai, got %s", list[0].ID)
	}
	if list[0].Model != "gpt-4o" {
		t.Errorf("expected model gpt-4o, got %s", list[0].Model)
	}
	if list[0].KeyCount != 1 {
		t.Errorf("expected 1 key, got %d", list[0].KeyCount)
	}
}

func TestProviderRegistryEnableDisable(t *testing.T) {
	reg := NewProviderRegistry(nil)
	_ = reg.Register(ProviderConfig{
		ID:      "p1",
		Type:    ProviderTypeChat,
		BaseURL: "https://api.example.com/v1",
		APIKeys: []string{"key"},
		Model:   "m1",
		Enabled: true,
	})

	p := reg.Get("p1")
	if !p.Enabled() {
		t.Fatal("expected enabled")
	}

	if err := reg.Disable("p1"); err != nil {
		t.Fatal(err)
	}
	if p.Enabled() {
		t.Fatal("expected disabled")
	}

	if err := reg.Enable("p1"); err != nil {
		t.Fatal(err)
	}
	if !p.Enabled() {
		t.Fatal("expected enabled again")
	}
}

func TestProviderRegistryKeyRotation(t *testing.T) {
	reg := NewProviderRegistry(nil)
	_ = reg.Register(ProviderConfig{
		ID:      "multi-key",
		Type:    ProviderTypeChat,
		BaseURL: "https://api.example.com/v1",
		APIKeys: []string{"key-a", "key-b", "key-c"},
		Model:   "m1",
		Enabled: true,
	})

	p := reg.Get("multi-key")
	// First rotation should go to index 1 (key-b)
	k := p.RotateKey()
	if k != "key-b" {
		t.Errorf("expected key-b, got %s", k)
	}
	// Second rotation should go to index 2 (key-c)
	k = p.RotateKey()
	if k != "key-c" {
		t.Errorf("expected key-c, got %s", k)
	}
	// Third wraps to index 0 (key-a)
	k = p.RotateKey()
	if k != "key-a" {
		t.Errorf("expected key-a, got %s", k)
	}
}

func TestProviderRegistrySessionOverride(t *testing.T) {
	reg := NewProviderRegistry(nil)
	_ = reg.Register(ProviderConfig{
		ID:      "default",
		Type:    ProviderTypeChat,
		BaseURL: "https://api.example.com/v1",
		APIKeys: []string{"key"},
		Model:   "default-model",
		Enabled: true,
	})
	_ = reg.Register(ProviderConfig{
		ID:      "premium",
		Type:    ProviderTypeChat,
		BaseURL: "https://api.example.com/v1",
		APIKeys: []string{"key2"},
		Model:   "premium-model",
		Enabled: true,
	})

	// No session override
	p := reg.GetForSession("sess-1")
	if p != nil {
		t.Fatal("expected nil (no override)")
	}

	// Set override
	reg.SetSessionProvider("sess-1", "premium")
	p = reg.GetForSession("sess-1")
	if p == nil || p.Config.ID != "premium" {
		t.Fatal("expected premium provider")
	}

	// Clear override
	reg.ClearSessionProvider("sess-1")
	p = reg.GetForSession("sess-1")
	if p != nil {
		t.Fatal("expected nil after clear")
	}
}

func TestProviderRegistryByType(t *testing.T) {
	reg := NewProviderRegistry(nil)
	_ = reg.Register(ProviderConfig{
		ID: "chat1", Type: ProviderTypeChat,
		BaseURL: "https://a.com/v1", APIKeys: []string{"k"}, Model: "m", Enabled: true,
	})
	_ = reg.Register(ProviderConfig{
		ID: "embed1", Type: ProviderTypeEmbedding,
		BaseURL: "https://b.com/v1", APIKeys: []string{"k"}, Model: "e", Enabled: true,
	})
	_ = reg.Register(ProviderConfig{
		ID: "chat2", Type: ProviderTypeChat,
		BaseURL: "https://c.com/v1", APIKeys: []string{"k"}, Model: "m2", Enabled: false,
	})

	chatProviders := reg.ByType(ProviderTypeChat)
	if len(chatProviders) != 1 { // chat2 is disabled
		t.Errorf("expected 1 enabled chat provider, got %d", len(chatProviders))
	}

	embProviders := reg.ByType(ProviderTypeEmbedding)
	if len(embProviders) != 1 {
		t.Errorf("expected 1 embedding provider, got %d", len(embProviders))
	}
}

func TestProviderRegistrySwitchModel(t *testing.T) {
	reg := NewProviderRegistry(nil)
	_ = reg.Register(ProviderConfig{
		ID:      "openai",
		Type:    ProviderTypeChat,
		BaseURL: "https://api.openai.com/v1",
		APIKeys: []string{"sk-test"},
		Model:   "gpt-4o",
		Enabled: true,
		Tier:    "smart",
	})

	p := reg.Get("openai")
	if p.Client.Model() != "gpt-4o" {
		t.Fatalf("expected gpt-4o, got %s", p.Client.Model())
	}

	err := reg.SwitchModel("openai", "gpt-4o-mini")
	if err != nil {
		t.Fatal(err)
	}

	p = reg.Get("openai")
	if p.Client.Model() != "gpt-4o-mini" {
		t.Fatalf("expected gpt-4o-mini, got %s", p.Client.Model())
	}

	// Pool should also be updated
	poolClient := reg.Pool().Get("openai")
	if poolClient == nil {
		t.Fatal("expected pool entry for openai")
	}
	if poolClient.Model() != "gpt-4o-mini" {
		t.Fatalf("expected pool model gpt-4o-mini, got %s", poolClient.Model())
	}
}

func TestProviderRegistryValidation(t *testing.T) {
	reg := NewProviderRegistry(nil)

	// Missing ID
	err := reg.Register(ProviderConfig{
		Type: ProviderTypeChat, BaseURL: "url", APIKeys: []string{"k"}, Model: "m",
	})
	if err == nil {
		t.Fatal("expected error for missing ID")
	}

	// Missing base_url
	err = reg.Register(ProviderConfig{
		ID: "x", Type: ProviderTypeChat, APIKeys: []string{"k"}, Model: "m",
	})
	if err == nil {
		t.Fatal("expected error for missing base_url")
	}

	// Empty api_keys — should succeed (local backends like Ollama don't need keys)
	err = reg.Register(ProviderConfig{
		ID: "x", Type: ProviderTypeChat, BaseURL: "url", Model: "m",
	})
	if err != nil {
		t.Fatalf("empty api_keys should be allowed for local backends: %v", err)
	}
}

func TestProviderTestNotFound(t *testing.T) {
	reg := NewProviderRegistry(nil)
	err := reg.TestProvider(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent provider")
	}
}

func TestProviderRegistryPoolSync(t *testing.T) {
	pool := NewPool()
	reg := NewProviderRegistry(pool)

	_ = reg.Register(ProviderConfig{
		ID:      "ds",
		Type:    ProviderTypeChat,
		BaseURL: "https://api.deepseek.com/v1",
		APIKeys: []string{"sk-ds"},
		Model:   "deepseek-chat",
		Enabled: true,
		Tier:    "fast",
	})

	// Should be in pool under both ID and tier
	if !pool.Has("ds") {
		t.Error("expected pool to have 'ds'")
	}
	if !pool.Has("fast") {
		t.Error("expected pool to have 'fast'")
	}
}
