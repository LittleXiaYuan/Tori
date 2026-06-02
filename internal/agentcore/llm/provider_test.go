package llm

import (
	"context"
	"encoding/json"
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
	if reg.Pool().Has("chat2") {
		t.Error("disabled chat provider should not be registered in the pool")
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

func TestProviderRegistryDisableRemovesPoolEntriesAndEnableRestores(t *testing.T) {
	pool := NewPool()
	reg := NewProviderRegistry(pool)
	if err := reg.Register(ProviderConfig{
		ID:      "local-ollama",
		Type:    ProviderTypeChat,
		Source:  ProviderSourceLocal,
		BaseURL: "http://localhost:11434/v1",
		APIKeys: []string{"local"},
		Model:   "qwen3.5:4b",
		Enabled: true,
		Tier:    "fast",
	}); err != nil {
		t.Fatalf("register provider: %v", err)
	}
	if !pool.Has("local-ollama") || !pool.Has("fast") {
		t.Fatal("expected enabled provider to sync into pool")
	}

	if err := reg.Disable("local-ollama"); err != nil {
		t.Fatalf("disable provider: %v", err)
	}
	if pool.Has("local-ollama") || pool.Has("fast") {
		t.Fatal("disabled provider should be removed from pool")
	}

	if err := reg.Enable("local-ollama"); err != nil {
		t.Fatalf("enable provider: %v", err)
	}
	if !pool.Has("local-ollama") || !pool.Has("fast") {
		t.Fatal("enabled provider should be restored into pool")
	}
}

func TestProviderRegistryLoadFromStoreFilteredSkipsLocalWithoutPersisting(t *testing.T) {
	store := newTestProviderStore()
	configs := []ProviderConfig{
		{
			ID:           "local-ollama",
			DisplayName:  "Local ollama (qwen3.5:4b)",
			Type:         ProviderTypeChat,
			Source:       ProviderSourceLocal,
			BaseURL:      "http://localhost:11434/v1",
			APIKeys:      []string{"local"},
			Model:        "qwen3.5:4b",
			Enabled:      true,
			Tier:         "fast",
			Capabilities: []Capability{CapChat},
		},
		{
			ID:      "remote",
			Type:    ProviderTypeChat,
			Source:  ProviderSourceDirect,
			BaseURL: "https://api.example.com/v1",
			APIKeys: []string{"key"},
			Model:   "remote-model",
			Enabled: true,
		},
	}
	if err := store.Put(context.Background(), "all", configs); err != nil {
		t.Fatalf("put providers: %v", err)
	}

	reg := NewProviderRegistry(NewPool())
	reg.SetPersistStore(store)
	count, skipped, err := reg.LoadFromStoreFiltered(func(cfg ProviderConfig) bool {
		return cfg.Source != ProviderSourceLocal
	})
	if err != nil {
		t.Fatalf("LoadFromStoreFiltered: %v", err)
	}
	if count != 1 || skipped != 1 {
		t.Fatalf("expected loaded=1 skipped=1, got loaded=%d skipped=%d", count, skipped)
	}
	if reg.Get("remote") == nil {
		t.Fatal("expected remote provider to load")
	}
	p := reg.Get("local-ollama")
	if p == nil {
		t.Fatal("local provider should remain visible in registry")
	}
	if p.Enabled() {
		t.Fatal("local provider should be disabled")
	}
	if reg.Pool().Has("fast") {
		t.Fatal("skipped local provider should not register its fast tier")
	}

	var persisted []ProviderConfig
	found, err := store.Get(context.Background(), "all", &persisted)
	if err != nil || !found {
		t.Fatalf("read persisted providers: found=%v err=%v", found, err)
	}
	if len(persisted) != 2 {
		t.Fatalf("filter load should not destructively rewrite persisted providers, got %d", len(persisted))
	}
}

type testProviderStore struct {
	values map[string][]byte
}

func newTestProviderStore() *testProviderStore {
	return &testProviderStore{values: make(map[string][]byte)}
}

func (s *testProviderStore) Put(_ context.Context, key string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	s.values[key] = data
	return nil
}

func (s *testProviderStore) Get(_ context.Context, key string, dest any) (bool, error) {
	data, ok := s.values[key]
	if !ok {
		return false, nil
	}
	if err := json.Unmarshal(data, dest); err != nil {
		return true, err
	}
	return true, nil
}
