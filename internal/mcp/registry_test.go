package mcp

import (
	"context"
	"testing"
)

type stubProvider struct {
	tools []Tool
}

func (s *stubProvider) ListTools(_ context.Context) ([]Tool, error) {
	return s.tools, nil
}

func (s *stubProvider) CallTool(_ context.Context, name string, args map[string]any) (*CallResult, error) {
	return SuccessResult("called:" + name), nil
}

func TestRegistryBasic(t *testing.T) {
	r := NewRegistry()
	p := &stubProvider{}

	tool := Tool{Name: "echo", Description: "echoes input"}
	if err := r.Register(p, tool); err != nil {
		t.Fatalf("register: %v", err)
	}
	if r.Count() != 1 {
		t.Fatalf("count: got %d, want 1", r.Count())
	}

	// Duplicate registration should fail
	if err := r.Register(p, tool); err == nil {
		t.Fatal("expected error on duplicate register")
	}

	// Lookup
	prov, found, ok := r.Lookup("echo")
	if !ok || prov == nil {
		t.Fatal("lookup failed")
	}
	if found.Name != "echo" {
		t.Fatalf("lookup name: got %s", found.Name)
	}

	// List
	tools := r.List()
	if len(tools) != 1 || tools[0].Name != "echo" {
		t.Fatalf("list: unexpected %v", tools)
	}

	// Unregister
	r.Unregister("echo")
	if r.Count() != 0 {
		t.Fatal("unregister failed")
	}
}

func TestRegistryEmptyName(t *testing.T) {
	r := NewRegistry()
	p := &stubProvider{}
	if err := r.Register(p, Tool{Name: ""}); err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestRegistryNilProvider(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(nil, Tool{Name: "x"}); err == nil {
		t.Fatal("expected error for nil provider")
	}
}

func TestRegistryClear(t *testing.T) {
	r := NewRegistry()
	p := &stubProvider{}
	r.Register(p, Tool{Name: "a"})
	r.Register(p, Tool{Name: "b"})
	r.Clear()
	if r.Count() != 0 {
		t.Fatal("clear failed")
	}
}
