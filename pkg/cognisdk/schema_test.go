package cognisdk

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestJSONSchemasMarshal(t *testing.T) {
	for name, schema := range map[string]JSONSchema{
		"pack":              PackManifestJSONSchema(),
		"bundle":            PackBundleJSONSchema(),
		"feedback":          FeedbackProposalJSONSchema(),
		"bundle-summary":    PackBundleSummaryJSONSchema(),
		"digest":            PackBundleDigestCheckJSONSchema(),
		"diff":              PackBundleDiffJSONSchema(),
		"review":            PackBundleReviewJSONSchema(),
		"plan":              PackBundleApplyPlanJSONSchema(),
		"actions":           PackBundleApplyActionsJSONSchema(),
		"kinds":             PackBundleApplyActionKindsJSONSchema(),
		"checklist":         PackBundleApplyChecklistJSONSchema(),
		"checklist-summary": PackBundleApplyChecklistSummaryJSONSchema(),
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

func TestExportJSONSchemaArtifacts(t *testing.T) {
	dir := t.TempDir()
	artifacts, err := ExportJSONSchemaArtifacts(dir)
	if err != nil {
		t.Fatalf("export schemas: %v", err)
	}
	if len(artifacts) != len(JSONSchemaNames()) {
		t.Fatalf("artifact length = %d, want %d", len(artifacts), len(JSONSchemaNames()))
	}
	for _, artifact := range artifacts {
		if artifact.Name == "" || artifact.File == "" || artifact.Title == "" {
			t.Fatalf("artifact missing fields: %#v", artifact)
		}
		data, err := os.ReadFile(filepath.Join(dir, artifact.File))
		if err != nil {
			t.Fatalf("read exported schema %s: %v", artifact.File, err)
		}
		if !json.Valid(data) {
			t.Fatalf("exported schema is not valid json: %s", data)
		}
	}
}

func TestVerifyJSONSchemaArtifacts(t *testing.T) {
	dir := t.TempDir()
	if _, err := ExportJSONSchemaArtifacts(dir); err != nil {
		t.Fatalf("export schemas: %v", err)
	}
	checks, err := VerifyJSONSchemaArtifacts(dir)
	if err != nil {
		t.Fatalf("verify schemas: %v", err)
	}
	if len(checks) != len(JSONSchemaNames()) {
		t.Fatalf("checks length = %d, want %d", len(checks), len(JSONSchemaNames()))
	}
	for _, check := range checks {
		if check.Name == "" || check.File == "" || check.Expected == "" || check.Actual == "" {
			t.Fatalf("check missing fields: %#v", check)
		}
		if !check.Match || check.Error != "" {
			t.Fatalf("check did not match: %#v", check)
		}
	}
}

func TestVerifyJSONSchemaArtifactsDetectsStaleFile(t *testing.T) {
	dir := t.TempDir()
	if _, err := ExportJSONSchemaArtifacts(dir); err != nil {
		t.Fatalf("export schemas: %v", err)
	}
	path := filepath.Join(dir, "pack-bundle.schema.json")
	if err := os.WriteFile(path, []byte(`{"$id":"stale","title":"stale"}`), 0o644); err != nil {
		t.Fatalf("write stale schema: %v", err)
	}
	checks, err := VerifyJSONSchemaArtifacts(dir)
	if err == nil {
		t.Fatal("expected stale schema verification error")
	}
	found := false
	for _, check := range checks {
		if check.Name == "pack-bundle" {
			found = true
			if check.Match {
				t.Fatalf("stale schema unexpectedly matched: %#v", check)
			}
			if check.Actual != "stale" {
				t.Fatalf("stale schema actual id = %q", check.Actual)
			}
		}
	}
	if !found {
		t.Fatal("missing pack-bundle check")
	}
}

func TestVerifyJSONSchemaArtifactCatalog(t *testing.T) {
	dir := t.TempDir()
	artifacts, err := ExportJSONSchemaArtifacts(dir)
	if err != nil {
		t.Fatalf("export schemas: %v", err)
	}
	checks, err := VerifyJSONSchemaArtifactCatalog(dir, artifacts)
	if err != nil {
		t.Fatalf("verify schema catalog: %v", err)
	}
	if len(checks) != len(artifacts) {
		t.Fatalf("checks length = %d, want %d", len(checks), len(artifacts))
	}
}

func TestVerifyJSONSchemaArtifactCatalogRejectsIncompleteCatalog(t *testing.T) {
	dir := t.TempDir()
	artifacts, err := ExportJSONSchemaArtifacts(dir)
	if err != nil {
		t.Fatalf("export schemas: %v", err)
	}
	checks, err := VerifyJSONSchemaArtifactCatalog(dir, artifacts[:1])
	if err == nil {
		t.Fatal("expected incomplete catalog error")
	}
	if len(checks) != 1 || !checks[0].Match {
		t.Fatalf("unexpected checks for incomplete catalog: %#v", checks)
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

func TestJSONSchemaInfos(t *testing.T) {
	infos := JSONSchemaInfos()
	names := JSONSchemaNames()
	if len(infos) != len(names) {
		t.Fatalf("schema infos length = %d, want %d", len(infos), len(names))
	}
	for i, info := range infos {
		if info.Name != names[i] {
			t.Fatalf("schema info[%d] name = %q, want %q", i, info.Name, names[i])
		}
		if info.Title == "" || info.Schema == "" {
			t.Fatalf("schema info missing title or schema id: %#v", info)
		}
		if info.Description == "" {
			t.Fatalf("schema info missing description: %#v", info)
		}
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
	kindSchema := props["kind"].(map[string]any)
	enumValues := kindSchema["enum"].([]string)
	if len(enumValues) != len(PackBundleApplyActionKinds()) {
		t.Fatalf("action kind enum length = %d, want %d", len(enumValues), len(PackBundleApplyActionKinds()))
	}
	for _, kind := range PackBundleApplyActionKinds() {
		found := false
		for _, value := range enumValues {
			if value == string(kind) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("action kind enum missing %q", kind)
		}
	}
}

func TestPackBundleApplyActionKindsSchema(t *testing.T) {
	schema := PackBundleApplyActionKindsJSONSchema()
	if schema["type"] != "array" {
		t.Fatalf("action kinds schema type = %#v", schema["type"])
	}
	item := schema["items"].(map[string]any)
	props := item["properties"].(map[string]any)
	for _, field := range []string{"kind", "label", "description"} {
		if _, ok := props[field]; !ok {
			t.Fatalf("bundle apply action kinds item schema missing %q", field)
		}
	}
	kindSchema := props["kind"].(map[string]any)
	enumValues := kindSchema["enum"].([]string)
	if len(enumValues) != len(PackBundleApplyActionKinds()) {
		t.Fatalf("action kind info enum length = %d, want %d", len(enumValues), len(PackBundleApplyActionKinds()))
	}
}

func TestPackBundleApplyChecklistSchema(t *testing.T) {
	schema := PackBundleApplyChecklistJSONSchema()
	if schema["type"] != "array" {
		t.Fatalf("checklist schema type = %#v", schema["type"])
	}
	item := schema["items"].(map[string]any)
	props := item["properties"].(map[string]any)
	for _, field := range []string{"kind", "label", "description", "required", "done", "blocked", "message", "action", "info"} {
		if _, ok := props[field]; !ok {
			t.Fatalf("bundle apply checklist item schema missing %q", field)
		}
	}
}

func TestPackBundleApplyChecklistSummarySchema(t *testing.T) {
	schema := PackBundleApplyChecklistSummaryJSONSchema()
	if schema["type"] != "object" {
		t.Fatalf("checklist summary schema type = %#v", schema["type"])
	}
	props := schema["properties"].(map[string]any)
	for _, field := range []string{"total", "required", "optional", "done", "open", "blocked", "required_open", "required_done", "optional_open", "optional_done", "blocked_kinds", "required_kinds", "by_kind"} {
		if _, ok := props[field]; !ok {
			t.Fatalf("bundle apply checklist summary schema missing %q", field)
		}
	}
}
