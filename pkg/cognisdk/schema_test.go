package cognisdk

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestJSONSchemasMarshal(t *testing.T) {
	for name, schema := range map[string]JSONSchema{
		"pack":     PackManifestJSONSchema(),
		"bundle":   PackBundleJSONSchema(),
		"feedback": FeedbackProposalJSONSchema(),
		"summary":  PackBundleSummaryJSONSchema(),
		"digest":   PackBundleDigestCheckJSONSchema(),
		"diff":     PackBundleDiffJSONSchema(),
		"review":   PackBundleReviewJSONSchema(),
		"plan":     PackBundleApplyPlanJSONSchema(),
		"actions":  PackBundleApplyActionsJSONSchema(),
	} {
		data, err := json.Marshal(schema)
		if err != nil {
			t.Fatalf("marshal %s schema: %v", name, err)
		}
		if !json.Valid(data) {
			t.Fatalf("%s schema did not produce valid json", name)
		}
		if schema["$schema"] == "" || schema["title"] == "" {
			t.Fatalf("%s schema missing schema metadata: %#v", name, schema)
		}
	}
}

func TestPackBundleSchemaNamesCoreFields(t *testing.T) {
	schema := PackBundleJSONSchema()
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("schema properties missing: %#v", schema)
	}
	for _, field := range []string{"version", "id", "packs", "enabled_packs", "metadata"} {
		if _, ok := props[field]; !ok {
			t.Fatalf("bundle schema missing %q", field)
		}
	}
}

func TestFeedbackProposalSchemaNamesReviewFields(t *testing.T) {
	schema := FeedbackProposalJSONSchema()
	props := schema["properties"].(map[string]any)
	proposals := props["proposals"].(map[string]any)
	item := proposals["items"].(map[string]any)
	itemProps := item["properties"].(map[string]any)
	for _, field := range []string{"action", "requires_review", "read_only_target", "confidence_delta"} {
		if _, ok := itemProps[field]; !ok {
			t.Fatalf("feedback proposal item schema missing %q", field)
		}
	}
}

func TestSaveJSONSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pack-bundle.schema.json")
	if err := SaveJSONSchema(PackBundleJSONSchema(), path); err != nil {
		t.Fatalf("save schema: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved schema: %v", err)
	}
	if !json.Valid(data) {
		t.Fatalf("saved schema is not valid json: %s", data)
	}
}

func TestPackBundleDigestCheckSchemaNamesFields(t *testing.T) {
	schema := PackBundleDigestCheckJSONSchema()
	props := schema["properties"].(map[string]any)
	for _, field := range []string{"bundle_id", "expected", "actual", "match"} {
		if _, ok := props[field]; !ok {
			t.Fatalf("bundle digest check schema missing %q", field)
		}
	}
}

func TestPackBundleDiffSchemaNamesReviewFields(t *testing.T) {
	schema := PackBundleDiffJSONSchema()
	props := schema["properties"].(map[string]any)
	for _, field := range []string{"added_packs", "removed_packs", "changed_packs", "enabled_packs", "disabled_packs"} {
		if _, ok := props[field]; !ok {
			t.Fatalf("bundle diff schema missing %q", field)
		}
	}
}

func TestPackBundleReviewSchemaNamesGateFields(t *testing.T) {
	schema := PackBundleReviewJSONSchema()
	props := schema["properties"].(map[string]any)
	for _, field := range []string{"from_digest", "candidate_digest", "outcome", "rollback_bundle_id", "diff", "golden_tests"} {
		if _, ok := props[field]; !ok {
			t.Fatalf("bundle review schema missing %q", field)
		}
	}
}

func TestJSONSchemaByName(t *testing.T) {
	names := JSONSchemaNames()
	if len(names) == 0 {
		t.Fatal("expected schema names")
	}
	for _, name := range names {
		schema, ok := JSONSchemaByName(name)
		if !ok {
			t.Fatalf("schema name %q not found", name)
		}
		if schema["title"] == "" {
			t.Fatalf("schema %q missing title", name)
		}
	}
	if _, ok := JSONSchemaByName("missing"); ok {
		t.Fatal("unexpected schema for missing name")
	}
}

func TestPackBundleSummarySchemaNamesInspectFields(t *testing.T) {
	schema := PackBundleSummaryJSONSchema()
	props := schema["properties"].(map[string]any)
	for _, field := range []string{"digest", "pack_count", "enabled_count", "disabled_count", "golden_test_count", "packs"} {
		if _, ok := props[field]; !ok {
			t.Fatalf("bundle summary schema missing %q", field)
		}
	}
}

func TestPackBundleApplyPlanSchemaNamesFields(t *testing.T) {
	schema := PackBundleApplyPlanJSONSchema()
	props := schema["properties"].(map[string]any)
	for _, field := range []string{"from_digest", "candidate_digest", "requires_review", "blocked", "rollback_bundle_id", "recommended_actions", "actions", "diff", "golden_tests"} {
		if _, ok := props[field]; !ok {
			t.Fatalf("bundle apply plan schema missing %q", field)
		}
	}
}

func TestPackBundleApplyActionsSchema(t *testing.T) {
	schema := PackBundleApplyActionsJSONSchema()
	if schema["type"] != "array" {
		t.Fatalf("actions schema type = %#v", schema["type"])
	}
	item := schema["items"].(map[string]any)
	props := item["properties"].(map[string]any)
	for _, field := range []string{"kind", "pack_id", "from_version", "to_version", "digest", "bundle_id", "message"} {
		if _, ok := props[field]; !ok {
			t.Fatalf("bundle apply actions item schema missing %q", field)
		}
	}
}
