package cogni

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestExampleMCPDemoDeclaration validates the shipped example cogni declaration
// that exercises the full capability path (activation → MCP → surface authority
// → arbitration-friendly priority → experience). It guards that the example
// stays parseable, valid, and behaviorally correct as the schema evolves, so the
// "drop it into data/cognis and it runs" promise holds.
func TestExampleMCPDemoDeclaration(t *testing.T) {
	path := filepath.Join("..", "..", "examples", "cognis", "mcp-demo-assistant.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read example declaration: %v", err)
	}

	var d Declaration
	if err := json.Unmarshal(data, &d); err != nil {
		t.Fatalf("parse example declaration: %v", err)
	}
	if err := d.Validate(); err != nil {
		t.Fatalf("example declaration invalid: %v", err)
	}

	// Exercises the unified capability surface: MCP servers + tool filter.
	if len(d.MCP.Servers) != 1 || d.MCP.Servers[0].Command != "npx" {
		t.Fatalf("example should declare one npx MCP server, got %#v", d.MCP.Servers)
	}
	if d.MCP.ToolFilter == nil || len(d.MCP.ToolFilter.Include) == 0 {
		t.Fatalf("example should declare an MCP tool filter, got %#v", d.MCP.ToolFilter)
	}

	// Non-identity surface → authoritative path (P1) is exercised.
	if isIdentitySurface(d.Surface) {
		t.Fatal("example surface must be non-identity to exercise the authoritative path")
	}

	// Experience enabled → P4 recording accrues for this cogni.
	if !d.Experience.Enabled {
		t.Fatal("example should enable experience so the self-tuning loop records outcomes")
	}

	// Declaration checks (the cogni's own CI) must all pass.
	results := VerifyDeclaration(&d, NewEvaluator())
	if len(results) == 0 {
		t.Fatal("example should declare activation checks")
	}
	for _, r := range results {
		if !r.Passed {
			t.Fatalf("example check %q failed: %s (active=%v score=%.2f)", r.CheckName, r.Reason, r.GotActive, r.GotScore)
		}
	}
}
