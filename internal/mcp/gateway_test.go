package mcp

import (
	"context"
	"testing"
	"time"
)

func TestGatewayListAndCall(t *testing.T) {
	p := &stubProvider{
		tools: []Tool{
			{Name: "greet", Description: "says hello"},
			{Name: "calc", Description: "calculates"},
		},
	}
	gw := NewGateway([]Provider{p}, 5*time.Second)

	tools, err := gw.ListTools(context.Background())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("tools count: got %d, want 2", len(tools))
	}

	result, err := gw.CallTool(context.Background(), CallRequest{Name: "greet", Arguments: map[string]any{"name": "world"}})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %v", result.Content)
	}
	if len(result.Content) == 0 || result.Content[0].Text != "called:greet" {
		t.Fatalf("unexpected result: %v", result.Content)
	}
}

func TestGatewayToolNotFound(t *testing.T) {
	gw := NewGateway([]Provider{&stubProvider{}}, time.Second)
	result, err := gw.CallTool(context.Background(), CallRequest{Name: "missing"})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for missing tool")
	}
}

func TestGatewayEmptyName(t *testing.T) {
	gw := NewGateway(nil, time.Second)
	result, err := gw.CallTool(context.Background(), CallRequest{Name: ""})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for empty name")
	}
}

func TestGatewayAddProvider(t *testing.T) {
	gw := NewGateway(nil, time.Second)
	if gw.ToolCount(context.Background()) != 0 {
		t.Fatal("expected 0 tools initially")
	}
	gw.AddProvider(&stubProvider{tools: []Tool{{Name: "x"}}})
	if gw.ToolCount(context.Background()) != 1 {
		t.Fatal("expected 1 tool after add")
	}
}

func TestGatewayProtocolInfo(t *testing.T) {
	gw := NewGateway(nil, time.Second)
	info := gw.ProtocolInfo()
	if info["protocolVersion"] == nil {
		t.Fatal("missing protocolVersion")
	}
}
