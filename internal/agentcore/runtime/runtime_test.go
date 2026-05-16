package runtime

import (
	"context"
	"testing"
)

func TestNewPool(t *testing.T) {
	p := NewPool()
	if p.Count() != 0 {
		t.Fatalf("new pool count = %d, want 0", p.Count())
	}
	if p.Default() != nil {
		t.Fatal("new pool default should be nil")
	}
}

func TestPoolRegisterAndGet(t *testing.T) {
	p := NewPool()
	rt := &AgentRuntime{Config: AgentConfig{ID: "a1", Name: "Agent 1"}}
	p.Register(rt)

	// First registered becomes default
	got, ok := p.Get("a1")
	if !ok || got != rt {
		t.Fatal("Get(a1) failed")
	}
	if p.Default() != rt {
		t.Fatal("first registered should be default")
	}
	if p.Count() != 1 {
		t.Fatalf("Count = %d, want 1", p.Count())
	}
}

func TestPoolSetDefault(t *testing.T) {
	p := NewPool()
	rt1 := &AgentRuntime{Config: AgentConfig{ID: "a1", Name: "Agent 1"}}
	rt2 := &AgentRuntime{Config: AgentConfig{ID: "a2", Name: "Agent 2"}}
	p.Register(rt1)
	p.Register(rt2)

	p.SetDefault("a2")
	if p.Default() != rt2 {
		t.Fatal("default should be a2")
	}
}

func TestPoolResolve(t *testing.T) {
	p := NewPool()
	rt1 := &AgentRuntime{Config: AgentConfig{ID: "a1", Name: "Agent 1"}}
	p.Register(rt1)

	// Known ID
	if got := p.Resolve("a1"); got != rt1 {
		t.Error("Resolve(a1) should return a1")
	}
	// Unknown ID falls back to default
	if got := p.Resolve("unknown"); got != rt1 {
		t.Error("Resolve(unknown) should return default")
	}
	// Empty string returns default
	if got := p.Resolve(""); got != rt1 {
		t.Error("Resolve('') should return default")
	}
}

func TestPoolRemove(t *testing.T) {
	p := NewPool()
	rt1 := &AgentRuntime{Config: AgentConfig{ID: "a1"}}
	rt2 := &AgentRuntime{Config: AgentConfig{ID: "a2"}}
	p.Register(rt1)
	p.Register(rt2)

	if err := p.Remove("a1"); err != nil {
		t.Fatal(err)
	}
	if p.Count() != 1 {
		t.Fatalf("count after remove = %d, want 1", p.Count())
	}
	// Default should be promoted to a2
	if p.Default() == nil || p.Default().ID() != "a2" {
		t.Error("default should be promoted after removing default")
	}
	// Remove non-existent
	if err := p.Remove("nope"); err == nil {
		t.Error("expected error removing non-existent agent")
	}
}

func TestPoolList(t *testing.T) {
	p := NewPool()
	p.Register(&AgentRuntime{Config: AgentConfig{ID: "a1", Name: "Agent 1"}})
	p.Register(&AgentRuntime{Config: AgentConfig{ID: "a2", Name: "Agent 2"}})

	configs := p.List()
	if len(configs) != 2 {
		t.Fatalf("List() len = %d, want 2", len(configs))
	}
}

func TestLifecycle(t *testing.T) {
	lc := NewLifecycle()
	var order []string

	lc.RegisterFunc("a", func(ctx context.Context) error {
		order = append(order, "start-a")
		return nil
	}, func(ctx context.Context) error {
		order = append(order, "stop-a")
		return nil
	})
	lc.RegisterFunc("b", func(ctx context.Context) error {
		order = append(order, "start-b")
		return nil
	}, func(ctx context.Context) error {
		order = append(order, "stop-b")
		return nil
	})

	ctx := context.Background()
	if err := lc.Start(ctx); err != nil {
		t.Fatal(err)
	}
	lc.Stop(ctx)

	// Start runs in order, Stop runs in reverse
	expected := []string{"start-a", "start-b", "stop-b", "stop-a"}
	if len(order) != len(expected) {
		t.Fatalf("order = %v, want %v", order, expected)
	}
	for i, want := range expected {
		if order[i] != want {
			t.Errorf("order[%d] = %s, want %s", i, order[i], want)
		}
	}
}

func TestAgentRuntime(t *testing.T) {
	rt := &AgentRuntime{Config: AgentConfig{ID: "test", Name: "Test Agent"}}
	if rt.ID() != "test" {
		t.Errorf("ID() = %q, want test", rt.ID())
	}
	if rt.Name() != "Test Agent" {
		t.Errorf("Name() = %q, want Test Agent", rt.Name())
	}
}

func TestBindingRouter(t *testing.T) {
	p := NewPool()
	rt1 := &AgentRuntime{Config: AgentConfig{ID: "bot-feishu"}}
	rt2 := &AgentRuntime{Config: AgentConfig{ID: "bot-telegram"}}
	p.Register(rt1)
	p.Register(rt2)

	r := NewRouter(p)
	r.AddBinding(Binding{
		Key:      BindingKey{Channel: "feishu"},
		AgentID:  "bot-feishu",
		Priority: 10,
	})
	r.AddBinding(Binding{
		Key:      BindingKey{Channel: "telegram"},
		AgentID:  "bot-telegram",
		Priority: 5,
	})

	// Feishu key should resolve to bot-feishu
	got := r.Resolve(BindingKey{Channel: "feishu", Peer: "user1"})
	if got == nil || got.ID() != "bot-feishu" {
		t.Error("feishu binding should resolve to bot-feishu")
	}

	// Telegram key
	got = r.Resolve(BindingKey{Channel: "telegram"})
	if got == nil || got.ID() != "bot-telegram" {
		t.Error("telegram binding should resolve to bot-telegram")
	}

	// Unknown channel falls back to default
	got = r.Resolve(BindingKey{Channel: "discord"})
	if got == nil || got.ID() != "bot-feishu" {
		t.Error("unknown channel should resolve to default (bot-feishu)")
	}

	// Remove bindings
	removed := r.RemoveBindingsForAgent("bot-feishu")
	if removed != 1 {
		t.Errorf("removed = %d, want 1", removed)
	}
	if len(r.Bindings()) != 1 {
		t.Error("expected 1 binding remaining")
	}
}

func TestAgentPool(t *testing.T) {
	ap := NewAgentPool()

	// No factory
	_, err := ap.GetOrCreate(AgentConfig{ID: "a1", Name: "A1"})
	if err == nil {
		t.Error("expected error without factory")
	}

	// Set factory
	ap.SetFactory(func(cfg AgentConfig) (*AgentRuntime, error) {
		return &AgentRuntime{Config: cfg}, nil
	})

	rt, err := ap.GetOrCreate(AgentConfig{ID: "a1", Name: "A1"})
	if err != nil {
		t.Fatal(err)
	}
	if rt.ID() != "a1" {
		t.Errorf("ID = %q, want a1", rt.ID())
	}

	// Get existing
	got, ok := ap.Get("a1")
	if !ok || got != rt {
		t.Error("Get(a1) should return same runtime")
	}

	// Second GetOrCreate returns cached
	rt2, _ := ap.GetOrCreate(AgentConfig{ID: "a1"})
	if rt2 != rt {
		t.Error("GetOrCreate should return cached")
	}

	// List
	if ids := ap.List(); len(ids) != 1 {
		t.Errorf("List() = %v, want [a1]", ids)
	}

	// Remove
	if !ap.Remove("a1") {
		t.Error("Remove should succeed")
	}
	if ap.Remove("a1") {
		t.Error("second Remove should fail")
	}
}

func TestModuleRegistryListUsesLiveModuleStatus(t *testing.T) {
	registry := NewModuleRegistry()
	module := &testStatusModule{enabled: true, running: true}
	registry.Register(module)
	registry.InitAll(context.Background(), &App{}, "lite", map[string]bool{})

	statuses := registry.List()
	if len(statuses) != 1 || !statuses[0].Enabled || !statuses[0].Running {
		t.Fatalf("expected live module status enabled/running, got %#v", statuses)
	}
	if !registry.IsEnabled("test-status") {
		t.Fatalf("expected IsEnabled to use live running status")
	}

	module.enabled = false
	module.running = false
	statuses = registry.List()
	if len(statuses) != 1 || statuses[0].Enabled || statuses[0].Running {
		t.Fatalf("expected live module status to reflect runtime gate off, got %#v", statuses)
	}
	if registry.IsEnabled("test-status") {
		t.Fatalf("expected IsEnabled to reflect runtime gate off")
	}
}

type testStatusModule struct {
	enabled bool
	running bool
}

func (m *testStatusModule) Name() string        { return "test-status" }
func (m *testStatusModule) Description() string { return "test dynamic status module" }
func (m *testStatusModule) Profile() string     { return "lite" }
func (m *testStatusModule) Init(context.Context, *App) error {
	return nil
}
func (m *testStatusModule) Start(context.Context) error { return nil }
func (m *testStatusModule) Stop() error                 { return nil }
func (m *testStatusModule) Status() ModuleStatus {
	return ModuleStatus{
		Name:        m.Name(),
		Description: m.Description(),
		Profile:     m.Profile(),
		Enabled:     m.enabled,
		Running:     m.running,
	}
}
