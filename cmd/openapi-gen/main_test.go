package main

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestGeneratedSpecParses verifies that docs/openapi.yaml (if present) is
// valid YAML and conforms to the minimum OpenAPI 3.1 shape we expect.
//
// Run from repo root:
//
//	go test ./cmd/openapi-gen
//
// Skipped if the spec hasn't been generated yet.
func TestGeneratedSpecParses(t *testing.T) {
	const path = "../../docs/openapi.yaml"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("spec not generated: %v (run `go run ./cmd/openapi-gen`)", err)
	}

	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("yaml unmarshal: %v", err)
	}

	if v, _ := doc["openapi"].(string); !strings.HasPrefix(v, "3.1") {
		t.Errorf("openapi version should be 3.1.x, got %q", v)
	}
	if _, ok := doc["info"].(map[string]any); !ok {
		t.Error("missing or malformed info section")
	}
	paths, ok := doc["paths"].(map[string]any)
	if !ok {
		t.Fatal("missing paths section")
	}
	if len(paths) < 100 {
		t.Errorf("expected ≥100 paths, got %d", len(paths))
	}

	seenOpIDs := map[string]string{}
	for p, raw := range paths {
		methods, ok := raw.(map[string]any)
		if !ok {
			t.Errorf("path %q: not a map", p)
			continue
		}
		for method, opRaw := range methods {
			op, ok := opRaw.(map[string]any)
			if !ok {
				t.Errorf("%s %s: not a map", strings.ToUpper(method), p)
				continue
			}
			opID, _ := op["operationId"].(string)
			if opID == "" {
				t.Errorf("%s %s: missing operationId", strings.ToUpper(method), p)
				continue
			}
			if prev, ok := seenOpIDs[opID]; ok {
				t.Errorf("duplicate operationId %q in both %q and %q", opID, prev, p)
			}
			seenOpIDs[opID] = p
			if _, ok := op["responses"]; !ok {
				t.Errorf("%s %s: missing responses", strings.ToUpper(method), p)
			}
		}
	}
}

func TestShouldInclude(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"/v1/chat", true},
		{"/api/providers", true},
		{"/webhook/feishu", true},
		{"/healthz", true},
		{"/", false},
		{"", false},
		{"/static/foo.js", false},
		{"/v1/", true},
	}
	for _, c := range cases {
		if got := shouldInclude(c.path); got != c.want {
			t.Errorf("shouldInclude(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

func TestMakeOperationID(t *testing.T) {
	cases := []struct {
		method, path, want string
	}{
		{"get", "/v1/cognis", "get_v1_cognis"},
		{"post", "/v1/cognis/{id}/evolve", "post_v1_cognis_id_evolve"},
		{"get", "/api/skillhub/check-updates", "get_api_skillhub_check_updates"},
	}
	for _, c := range cases {
		if got := makeOperationID(c.method, c.path); got != c.want {
			t.Errorf("makeOperationID(%q, %q) = %q, want %q", c.method, c.path, got, c.want)
		}
	}
}
