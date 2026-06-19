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
		{"/mcp/v1", true},
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

func TestExtractPathsScansGatewaySubpackages(t *testing.T) {
	paths, err := extractPaths("../../internal/controlplane/gateway")
	if err != nil {
		t.Fatalf("extractPaths: %v", err)
	}
	seen := map[string]bool{}
	for _, p := range paths {
		seen[p] = true
	}
	for _, want := range []string{"/v1/workflows"} {
		if !seen[want] {
			t.Fatalf("expected recursive route scan to include %s", want)
		}
	}
}

func TestExtractPathsFromDirsScansPackRoutes(t *testing.T) {
	paths, err := extractPathsFromDirs([]string{
		"../../internal/controlplane/gateway",
		"../../internal/packs",
	})
	if err != nil {
		t.Fatalf("extractPathsFromDirs: %v", err)
	}
	seen := map[string]bool{}
	for _, p := range paths {
		seen[p] = true
	}
	for _, want := range []string{"/api/connectors", "/api/notify/channels", "/api/skillhub/search", "/mcp/v1", "/v1/channels/groups", "/v1/federation/peers", "/v1/fork", "/v1/heartbeat", "/v1/identity/profiles", "/v1/market/search", "/v1/modules", "/v1/orchestrator/status", "/v1/persona", "/v1/planner/checkpoints", "/v1/rbac/check", "/v1/reflect/experiences", "/v1/scheduler/jobs", "/v1/search/providers", "/v1/sessions/queue", "/v1/speech/voices", "/v1/subagent", "/v1/trace/recent"} {
		if !seen[want] {
			t.Fatalf("expected pack route scan to include %s", want)
		}
	}
	for _, want := range []string{"/v1/state", "/v1/state/goals", "/v1/state/focus", "/v1/state/resources"} {
		if !seen[want] {
			t.Fatalf("expected pack route scan to include %s", want)
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

func TestGeneratedSpecIncludesPlannerRecoveryEndpoints(t *testing.T) {
	const path = "../../docs/openapi.yaml"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("spec not generated: %v (run `go run ./cmd/openapi-gen`)", err)
	}

	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("yaml unmarshal: %v", err)
	}
	paths, ok := doc["paths"].(map[string]any)
	if !ok {
		t.Fatal("missing paths section")
	}

	want := map[string]string{
		"/v1/planner/checkpoints":                  "get",
		"/v1/planner/execution-state":              "get",
		"/v1/planner/checkpoints/recover":          "post",
		"/v1/planner/checkpoints/resume":           "post",
		"/v1/planner/checkpoints/resume-plan":      "post",
		"/v1/planner/checkpoints/resume-plan/jobs": "get",
	}
	for p, method := range want {
		raw, ok := paths[p]
		if !ok {
			t.Fatalf("missing planner recovery path %s", p)
		}
		methods, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("path %s is not a map", p)
		}
		if _, ok := methods[method]; !ok {
			t.Fatalf("path %s missing method %s", p, method)
		}
		for gotMethod := range methods {
			if gotMethod != method {
				t.Fatalf("path %s should only expose %s, found extra method %s", p, method, gotMethod)
			}
		}
	}

	recoverOp := paths["/v1/planner/checkpoints/recover"].(map[string]any)["post"].(map[string]any)
	if recoverOp["requestBody"] == nil {
		t.Fatal("recover endpoint should include a request body schema")
	}
	responses := recoverOp["responses"].(map[string]any)
	okResp := responses["200"].(map[string]any)
	content := okResp["content"].(map[string]any)
	appJSON := content["application/json"].(map[string]any)
	schema := appJSON["schema"].(map[string]any)
	props := schema["properties"].(map[string]any)
	for _, field := range []string{"prompt", "recovery_plan", "checkpoint"} {
		if _, ok := props[field]; !ok {
			t.Fatalf("recover response schema missing %s", field)
		}
	}
}
