package cognisdk

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// JSONSchema is a minimal JSON Schema document usable by frontends, plugins,
// and automation scripts that exchange Cognition SDK artifacts.
type JSONSchema map[string]any

// JSONSchemaInfo is a compact catalog entry for schema pickers, plugin
// settings, and automation UIs that need to discover SDK artifact contracts
// without embedding a hand-written list.
type JSONSchemaInfo struct {
	Name        string `json:"name" yaml:"name"`
	Title       string `json:"title" yaml:"title"`
	Schema      string `json:"schema" yaml:"schema"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

// JSONSchemaArtifact records one exported schema file. It lets automation
// scripts and plugin packagers know exactly which files were written without
// re-deriving paths from schema names.
type JSONSchemaArtifact struct {
	Name        string `json:"name" yaml:"name"`
	Title       string `json:"title" yaml:"title"`
	Schema      string `json:"schema" yaml:"schema"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	File        string `json:"file" yaml:"file"`
}

// JSONSchemaArtifactCheck records verification evidence for one schema
// artifact file. It is designed for CI, plugin installers, and frontend build
// steps that need a machine-readable proof that an exported schema bundle is
// complete before using it.
type JSONSchemaArtifactCheck struct {
	Name     string `json:"name" yaml:"name"`
	File     string `json:"file" yaml:"file"`
	Expected string `json:"expected" yaml:"expected"`
	Actual   string `json:"actual,omitempty" yaml:"actual,omitempty"`
	Match    bool   `json:"match" yaml:"match"`
	Error    string `json:"error,omitempty" yaml:"error,omitempty"`
}

// JSONSchemaNames returns stable names accepted by JSONSchemaByName.
func JSONSchemaNames() []string {
	return []string{"pack-manifest", "pack-bundle", "pack-bundle-summary", "pack-bundle-digest-check", "pack-bundle-diff", "pack-bundle-review", "pack-bundle-apply-plan", "pack-bundle-apply-actions", "pack-bundle-apply-action-kinds", "pack-bundle-apply-checklist", "pack-bundle-apply-checklist-summary", "feedback-proposal"}
}

// JSONSchemaInfos returns stable schema catalog metadata for non-Go callers
// that want to populate selectors or validate integration contracts before
// exporting a specific schema by name.
func JSONSchemaInfos() []JSONSchemaInfo {
	names := JSONSchemaNames()
	infos := make([]JSONSchemaInfo, 0, len(names))
	for _, name := range names {
		schema, ok := JSONSchemaByName(name)
		if !ok {
			continue
		}
		infos = append(infos, JSONSchemaInfo{
			Name:        name,
			Title:       schemaString(schema, "title"),
			Schema:      schemaString(schema, "$id"),
			Description: schemaDescription(name),
		})
	}
	return infos
}

// JSONSchemaByName returns a schema by its stable CLI/API name.
func JSONSchemaByName(name string) (JSONSchema, bool) {
	switch strings.TrimSpace(name) {
	case "pack-manifest":
		return PackManifestJSONSchema(), true
	case "pack-bundle":
		return PackBundleJSONSchema(), true
	case "pack-bundle-summary":
		return PackBundleSummaryJSONSchema(), true
	case "pack-bundle-digest-check":
		return PackBundleDigestCheckJSONSchema(), true
	case "pack-bundle-diff":
		return PackBundleDiffJSONSchema(), true
	case "pack-bundle-review":
		return PackBundleReviewJSONSchema(), true
	case "pack-bundle-apply-plan":
		return PackBundleApplyPlanJSONSchema(), true
	case "pack-bundle-apply-actions":
		return PackBundleApplyActionsJSONSchema(), true
	case "pack-bundle-apply-action-kinds":
		return PackBundleApplyActionKindsJSONSchema(), true
	case "pack-bundle-apply-checklist":
		return PackBundleApplyChecklistJSONSchema(), true
	case "pack-bundle-apply-checklist-summary":
		return PackBundleApplyChecklistSummaryJSONSchema(), true
	case "feedback-proposal":
		return FeedbackProposalJSONSchema(), true
	default:
		return nil, false
	}
}

// PackManifestJSONSchema returns the public schema for a single declarative
// Cogni Pack manifest. It intentionally models the stable phase-1 fields only.
func PackManifestJSONSchema() JSONSchema {
	return JSONSchema{
		"$schema":              "https://json-schema.org/draft/2020-12/schema",
		"$id":                  "https://yunque.local/schemas/cognisdk/pack-manifest.json",
		"title":                "Cognition SDK Pack Manifest",
		"type":                 "object",
		"additionalProperties": true,
		"required":             []string{"id", "version", "type"},
		"properties": map[string]any{
			"id":           stringSchema(),
			"version":      stringSchema(),
			"type":         stringSchema(),
			"display_name": stringSchema(),
			"provides":     stringArraySchema(),
			"permissions":  stringArraySchema(),
			"belief_seeds": map[string]any{"type": "array", "items": beliefNodeSchema()},
			"disposition_rules": map[string]any{
				"type":  "array",
				"items": dispositionRuleSchema(),
			},
			"boundary":         boundaryPolicySchema(),
			"render_templates": map[string]any{"type": "array", "items": renderTemplateSchema()},
			"golden_tests":     map[string]any{"type": "array", "items": goldenTestSchema()},
			"optional_lora": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"adapter":  stringSchema(),
					"required": map[string]any{"type": "boolean"},
				},
			},
		},
	}
}

// PackBundleJSONSchema returns the schema for portable Cogni Pack bundles.
func PackBundleJSONSchema() JSONSchema {
	return JSONSchema{
		"$schema":              "https://json-schema.org/draft/2020-12/schema",
		"$id":                  "https://yunque.local/schemas/cognisdk/pack-bundle.json",
		"title":                "Cognition SDK Pack Bundle",
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"version", "id", "packs"},
		"properties": map[string]any{
			"version":       map[string]any{"type": "integer", "const": currentPackBundleVersion},
			"id":            stringSchema(),
			"created_at":    map[string]any{"type": "string", "format": "date-time"},
			"packs":         map[string]any{"type": "array", "items": PackManifestJSONSchema()},
			"enabled_packs": stringArraySchema(),
			"metadata":      stringMapSchema(),
		},
	}
}

// PackBundleSummaryJSONSchema returns the schema for bundle inspect summaries.
func PackBundleSummaryJSONSchema() JSONSchema {
	return JSONSchema{
		"$schema":              "https://json-schema.org/draft/2020-12/schema",
		"$id":                  "https://yunque.local/schemas/cognisdk/pack-bundle-summary.json",
		"title":                "Cognition SDK Pack Bundle Summary",
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"id", "version", "pack_count", "enabled_count", "disabled_count", "golden_test_count", "packs"},
		"properties": map[string]any{
			"id":                stringSchema(),
			"version":           map[string]any{"type": "integer"},
			"digest":            stringSchema(),
			"pack_count":        map[string]any{"type": "integer"},
			"enabled_count":     map[string]any{"type": "integer"},
			"disabled_count":    map[string]any{"type": "integer"},
			"golden_test_count": map[string]any{"type": "integer"},
			"packs":             map[string]any{"type": "array", "items": packStatusSchema()},
		},
	}
}

// PackBundleDigestCheckJSONSchema returns the schema for digest assertion
// results emitted by VerifyPackBundleDigest and cognisdk-bundle digest --expect.
func PackBundleDigestCheckJSONSchema() JSONSchema {
	return JSONSchema{
		"$schema":              "https://json-schema.org/draft/2020-12/schema",
		"$id":                  "https://yunque.local/schemas/cognisdk/pack-bundle-digest-check.json",
		"title":                "Cognition SDK Pack Bundle Digest Check",
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"bundle_id", "expected", "actual", "match"},
		"properties": map[string]any{
			"bundle_id": stringSchema(),
			"expected":  stringSchema(),
			"actual":    stringSchema(),
			"match":     map[string]any{"type": "boolean"},
		},
	}
}

// PackBundleDiffJSONSchema returns the schema for non-mutating bundle review diffs.
func PackBundleDiffJSONSchema() JSONSchema {
	return JSONSchema{
		"$schema":              "https://json-schema.org/draft/2020-12/schema",
		"$id":                  "https://yunque.local/schemas/cognisdk/pack-bundle-diff.json",
		"title":                "Cognition SDK Pack Bundle Diff",
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"from_id", "to_id"},
		"properties": map[string]any{
			"from_id":        stringSchema(),
			"to_id":          stringSchema(),
			"added_packs":    map[string]any{"type": "array", "items": packStatusSchema()},
			"removed_packs":  map[string]any{"type": "array", "items": packStatusSchema()},
			"changed_packs":  map[string]any{"type": "array", "items": packChangeSchema()},
			"enabled_packs":  stringArraySchema(),
			"disabled_packs": stringArraySchema(),
		},
	}
}

// PackBundleReviewJSONSchema returns the schema for candidate bundle review reports.
func PackBundleReviewJSONSchema() JSONSchema {
	return JSONSchema{
		"$schema":              "https://json-schema.org/draft/2020-12/schema",
		"$id":                  "https://yunque.local/schemas/cognisdk/pack-bundle-review.json",
		"title":                "Cognition SDK Pack Bundle Review",
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"from_id", "candidate_id", "outcome", "reason", "diff", "golden_tests"},
		"properties": map[string]any{
			"from_id":            stringSchema(),
			"candidate_id":       stringSchema(),
			"from_digest":        stringSchema(),
			"candidate_digest":   stringSchema(),
			"outcome":            enumSchema(string(PackBundleReviewReady), string(PackBundleReviewReview), string(PackBundleReviewBlocked)),
			"reason":             stringSchema(),
			"rollback_bundle_id": stringSchema(),
			"diff":               PackBundleDiffJSONSchema(),
			"golden_tests":       goldenTestSummarySchema(),
		},
	}
}

// PackBundleApplyPlanJSONSchema returns the schema for non-mutating bundle apply plans.
func PackBundleApplyPlanJSONSchema() JSONSchema {
	return JSONSchema{
		"$schema":              "https://json-schema.org/draft/2020-12/schema",
		"$id":                  "https://yunque.local/schemas/cognisdk/pack-bundle-apply-plan.json",
		"title":                "Cognition SDK Pack Bundle Apply Plan",
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"from_id", "candidate_id", "outcome", "reason", "requires_review", "blocked", "recommended_actions", "actions", "diff", "golden_tests"},
		"properties": map[string]any{
			"from_id":             stringSchema(),
			"candidate_id":        stringSchema(),
			"from_digest":         stringSchema(),
			"candidate_digest":    stringSchema(),
			"outcome":             enumSchema(string(PackBundleReviewReady), string(PackBundleReviewReview), string(PackBundleReviewBlocked)),
			"reason":              stringSchema(),
			"requires_review":     map[string]any{"type": "boolean"},
			"blocked":             map[string]any{"type": "boolean"},
			"rollback_bundle_id":  stringSchema(),
			"recommended_actions": stringArraySchema(),
			"actions":             map[string]any{"type": "array", "items": packBundleApplyActionSchema()},
			"diff":                PackBundleDiffJSONSchema(),
			"golden_tests":        goldenTestSummarySchema(),
		},
	}
}

// PackBundleApplyActionsJSONSchema returns the schema for the script-friendly
// actions array emitted by cognisdk-bundle actions.
func PackBundleApplyActionsJSONSchema() JSONSchema {
	return JSONSchema{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"$id":     "https://yunque.local/schemas/cognisdk/pack-bundle-apply-actions.json",
		"title":   "Cognition SDK Pack Bundle Apply Actions",
		"type":    "array",
		"items":   packBundleApplyActionSchema(),
	}
}

// PackBundleApplyActionKindsJSONSchema returns the schema for the detailed
// action-kind metadata emitted by cognisdk-bundle action-kinds --details.
func PackBundleApplyActionKindsJSONSchema() JSONSchema {
	return JSONSchema{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"$id":     "https://yunque.local/schemas/cognisdk/pack-bundle-apply-action-kinds.json",
		"title":   "Cognition SDK Pack Bundle Apply Action Kinds",
		"type":    "array",
		"items":   packBundleApplyActionKindInfoSchema(),
	}
}

// PackBundleApplyChecklistJSONSchema returns the schema for the UI-friendly
// checklist emitted by BuildPackBundleApplyChecklist and cognisdk-bundle
// checklist.
func PackBundleApplyChecklistJSONSchema() JSONSchema {
	return JSONSchema{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"$id":     "https://yunque.local/schemas/cognisdk/pack-bundle-apply-checklist.json",
		"title":   "Cognition SDK Pack Bundle Apply Checklist",
		"type":    "array",
		"items":   packBundleApplyChecklistItemSchema(),
	}
}

// PackBundleApplyChecklistSummaryJSONSchema returns the schema for compact
// checklist dashboard counters emitted by SummarizePackBundleApplyChecklist
// and cognisdk-bundle checklist-summary.
func PackBundleApplyChecklistSummaryJSONSchema() JSONSchema {
	return JSONSchema{
		"$schema":              "https://json-schema.org/draft/2020-12/schema",
		"$id":                  "https://yunque.local/schemas/cognisdk/pack-bundle-apply-checklist-summary.json",
		"title":                "Cognition SDK Pack Bundle Apply Checklist Summary",
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"total", "required", "optional", "done", "open", "blocked", "required_open", "required_done", "optional_open", "optional_done", "by_kind"},
		"properties": map[string]any{
			"total":          integerSchema(),
			"required":       integerSchema(),
			"optional":       integerSchema(),
			"done":           integerSchema(),
			"open":           integerSchema(),
			"blocked":        integerSchema(),
			"required_open":  integerSchema(),
			"required_done":  integerSchema(),
			"optional_open":  integerSchema(),
			"optional_done":  integerSchema(),
			"blocked_kinds":  map[string]any{"type": "array", "items": enumSchema(packBundleApplyActionKindStrings()...)},
			"required_kinds": map[string]any{"type": "array", "items": enumSchema(packBundleApplyActionKindStrings()...)},
			"by_kind": map[string]any{
				"type":                 "object",
				"additionalProperties": integerSchema(),
			},
		},
	}
}

// FeedbackProposalJSONSchema returns the schema for non-mutating feedback
// proposals returned by BuildFeedbackProposal or Engine.ProposeUpdates.
func FeedbackProposalJSONSchema() JSONSchema {
	return JSONSchema{
		"$schema":              "https://json-schema.org/draft/2020-12/schema",
		"$id":                  "https://yunque.local/schemas/cognisdk/feedback-proposal.json",
		"title":                "Cognition SDK Feedback Proposal",
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"id", "time", "outcome", "summary"},
		"properties": map[string]any{
			"id":      stringSchema(),
			"time":    map[string]any{"type": "string", "format": "date-time"},
			"outcome": enumSchema(string(FeedbackOutcomeNoAction), string(FeedbackOutcomeProposed), string(FeedbackOutcomeReviewRequired)),
			"summary": stringSchema(),
			"proposals": map[string]any{
				"type":  "array",
				"items": beliefUpdateProposalSchema(),
			},
			"audit_events": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": true,
					"properties": map[string]any{
						"time":     map[string]any{"type": "string", "format": "date-time"},
						"type":     stringSchema(),
						"message":  stringSchema(),
						"metadata": stringMapSchema(),
					},
				},
			},
		},
	}
}

// SaveJSONSchema writes a schema document as pretty JSON.
func SaveJSONSchema(schema JSONSchema, path string) error {
	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return fmt.Errorf("cognisdk.schema: marshal: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("cognisdk.schema: write %q: %w", path, err)
	}
	return nil
}

// ExportJSONSchemaArtifacts writes every public Cognition SDK JSON Schema into
// outputDir and returns a catalog of the created files. The returned File
// values are relative filenames, so callers can move the directory as a small
// portable schema artifact bundle.
func ExportJSONSchemaArtifacts(outputDir string) ([]JSONSchemaArtifact, error) {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, fmt.Errorf("cognisdk.schema: create %q: %w", outputDir, err)
	}
	infos := JSONSchemaInfos()
	artifacts := make([]JSONSchemaArtifact, 0, len(infos))
	for _, info := range infos {
		schema, ok := JSONSchemaByName(info.Name)
		if !ok {
			continue
		}
		file := info.Name + ".schema.json"
		if err := SaveJSONSchema(schema, filepath.Join(outputDir, file)); err != nil {
			return nil, err
		}
		artifacts = append(artifacts, JSONSchemaArtifact{
			Name:        info.Name,
			Title:       info.Title,
			Schema:      info.Schema,
			Description: info.Description,
			File:        file,
		})
	}
	return artifacts, nil
}

// VerifyJSONSchemaArtifacts checks that outputDir contains every public schema
// artifact and that each file exactly matches the SDK's current canonical
// schema document. The returned checks are stable and JSON-friendly; the error
// is non-nil when any artifact is missing, invalid, unknown, or stale.
func VerifyJSONSchemaArtifacts(outputDir string) ([]JSONSchemaArtifactCheck, error) {
	infos := JSONSchemaInfos()
	checks := make([]JSONSchemaArtifactCheck, 0, len(infos))
	var failures []string
	for _, info := range infos {
		file := info.Name + ".schema.json"
		check := JSONSchemaArtifactCheck{
			Name:     info.Name,
			File:     file,
			Expected: info.Schema,
		}
		actual, match, err := verifyJSONSchemaArtifactFile(outputDir, file, info)
		if err != nil {
			check.Error = err.Error()
			failures = append(failures, fmt.Sprintf("%s: %s", file, err.Error()))
		}
		check.Actual = actual
		check.Match = match
		if err == nil && !match {
			failures = append(failures, fmt.Sprintf("%s: schema document mismatch", file))
		}
		checks = append(checks, check)
	}
	if len(failures) > 0 {
		return checks, fmt.Errorf("cognisdk.schema: verify artifacts failed: %s", strings.Join(failures, "; "))
	}
	return checks, nil
}

// VerifyJSONSchemaArtifactCatalog checks a caller-provided artifact catalog and
// then verifies the referenced files against the SDK's canonical schema
// documents. The catalog is treated as data: entries must use known schema
// names and relative filenames so a schema bundle remains portable.
func VerifyJSONSchemaArtifactCatalog(outputDir string, artifacts []JSONSchemaArtifact) ([]JSONSchemaArtifactCheck, error) {
	if len(artifacts) == 0 {
		return nil, fmt.Errorf("cognisdk.schema: verify catalog: empty artifact catalog")
	}
	infosByName := map[string]JSONSchemaInfo{}
	for _, info := range JSONSchemaInfos() {
		infosByName[info.Name] = info
	}
	seen := map[string]bool{}
	checks := make([]JSONSchemaArtifactCheck, 0, len(artifacts))
	var failures []string
	for _, artifact := range artifacts {
		info, ok := infosByName[artifact.Name]
		file := artifact.File
		if file == "" {
			file = artifact.Name + ".schema.json"
		}
		check := JSONSchemaArtifactCheck{
			Name:     artifact.Name,
			File:     file,
			Expected: info.Schema,
		}
		switch {
		case artifact.Name == "":
			check.Error = "missing schema artifact name"
		case !ok:
			check.Error = "unknown schema artifact"
		case seen[artifact.Name]:
			check.Error = "duplicate schema artifact"
		case !safeRelativeSchemaArtifactFile(file):
			check.Error = "schema artifact file must be a relative filename"
		case artifact.Schema != "" && artifact.Schema != info.Schema:
			check.Error = "schema artifact id mismatch"
		}
		if check.Error != "" {
			failures = append(failures, fmt.Sprintf("%s: %s", artifact.Name, check.Error))
			checks = append(checks, check)
			continue
		}
		seen[artifact.Name] = true
		actual, match, err := verifyJSONSchemaArtifactFile(outputDir, file, info)
		if err != nil {
			check.Error = err.Error()
			failures = append(failures, fmt.Sprintf("%s: %s", file, err.Error()))
		}
		check.Actual = actual
		check.Match = match
		if err == nil && !match {
			failures = append(failures, fmt.Sprintf("%s: schema document mismatch", file))
		}
		checks = append(checks, check)
	}
	for _, info := range JSONSchemaInfos() {
		if !seen[info.Name] {
			failures = append(failures, fmt.Sprintf("%s: missing from artifact catalog", info.Name))
		}
	}
	if len(failures) > 0 {
		return checks, fmt.Errorf("cognisdk.schema: verify catalog failed: %s", strings.Join(failures, "; "))
	}
	return checks, nil
}

func verifyJSONSchemaArtifactFile(outputDir, file string, info JSONSchemaInfo) (string, bool, error) {
	if !safeRelativeSchemaArtifactFile(file) {
		return "", false, fmt.Errorf("schema artifact file must be a relative filename")
	}
	data, err := os.ReadFile(filepath.Join(outputDir, file))
	if err != nil {
		return "", false, fmt.Errorf("read: %w", err)
	}
	var actual JSONSchema
	if err := json.Unmarshal(data, &actual); err != nil {
		return "", false, fmt.Errorf("invalid json: %w", err)
	}
	actualID := schemaString(actual, "$id")
	expected, ok := JSONSchemaByName(info.Name)
	if !ok {
		return actualID, false, fmt.Errorf("unknown schema %q", info.Name)
	}
	expectedData, err := json.Marshal(expected)
	if err != nil {
		return actualID, false, fmt.Errorf("marshal expected: %w", err)
	}
	actualData, err := json.Marshal(actual)
	if err != nil {
		return actualID, false, fmt.Errorf("marshal actual: %w", err)
	}
	return actualID, string(actualData) == string(expectedData), nil
}

func safeRelativeSchemaArtifactFile(file string) bool {
	return file != "" && !filepath.IsAbs(file) && filepath.Clean(file) == file && !strings.HasPrefix(file, "..")
}

func packBundleApplyActionSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"kind", "message"},
		"properties": map[string]any{
			"kind":         enumSchema(packBundleApplyActionKindStrings()...),
			"pack_id":      stringSchema(),
			"from_version": stringSchema(),
			"to_version":   stringSchema(),
			"digest":       stringSchema(),
			"bundle_id":    stringSchema(),
			"message":      stringSchema(),
		},
	}
}

func packBundleApplyActionKindInfoSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"kind", "label", "description"},
		"properties": map[string]any{
			"kind":        enumSchema(packBundleApplyActionKindStrings()...),
			"label":       stringSchema(),
			"description": stringSchema(),
		},
	}
}

func packBundleApplyChecklistItemSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"kind", "label", "description", "required", "done", "blocked", "message", "info"},
		"properties": map[string]any{
			"kind":        enumSchema(packBundleApplyActionKindStrings()...),
			"label":       stringSchema(),
			"description": stringSchema(),
			"required":    map[string]any{"type": "boolean"},
			"done":        map[string]any{"type": "boolean"},
			"blocked":     map[string]any{"type": "boolean"},
			"message":     stringSchema(),
			"action":      packBundleApplyActionSchema(),
			"info":        packBundleApplyActionKindInfoSchema(),
		},
	}
}

func goldenTestSummarySchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"passed", "failed", "results"},
		"properties": map[string]any{
			"passed": map[string]any{"type": "integer"},
			"failed": map[string]any{"type": "integer"},
			"results": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"name", "passed"},
					"properties": map[string]any{
						"name":   stringSchema(),
						"passed": map[string]any{"type": "boolean"},
						"errors": stringArraySchema(),
					},
				},
			},
		},
	}
}

func packStatusSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"id", "version", "type", "enabled"},
		"properties": map[string]any{
			"id":           stringSchema(),
			"version":      stringSchema(),
			"type":         stringSchema(),
			"display_name": stringSchema(),
			"enabled":      map[string]any{"type": "boolean"},
			"provides":     stringArraySchema(),
		},
	}
}

func packChangeSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"id"},
		"properties": map[string]any{
			"id":           stringSchema(),
			"from_version": stringSchema(),
			"to_version":   stringSchema(),
			"reason":       stringSchema(),
		},
	}
}

func beliefNodeSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"id", "kind", "statement"},
		"properties": map[string]any{
			"id":          stringSchema(),
			"kind":        enumSchema(string(BeliefRoot), string(BeliefValue), string(BeliefRelational), string(BeliefBoundary), string(BeliefPreference)),
			"statement":   stringSchema(),
			"confidence":  map[string]any{"type": "number", "minimum": 0, "maximum": 1},
			"source_pack": stringSchema(),
			"read_only":   map[string]any{"type": "boolean"},
		},
	}
}

func dispositionRuleSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": true,
		"required":             []string{"id", "when", "mode"},
		"properties": map[string]any{
			"id":          stringSchema(),
			"when":        map[string]any{"type": "object", "additionalProperties": true},
			"mode":        stringSchema(),
			"tone":        stringSchema(),
			"priority":    map[string]any{"type": "integer"},
			"must_say":    stringArraySchema(),
			"must_avoid":  stringArraySchema(),
			"tool_policy": enumSchema(string(ToolPolicyAllow), string(ToolPolicyRequireConfirmation)),
			"template_id": stringSchema(),
			"source_pack": stringSchema(),
		},
	}
}

func boundaryPolicySchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"must_say":          stringArraySchema(),
			"must_avoid":        stringArraySchema(),
			"high_risk_actions": stringArraySchema(),
			"default_tool":      enumSchema(string(ToolPolicyAllow), string(ToolPolicyRequireConfirmation)),
		},
	}
}

func renderTemplateSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"id", "body"},
		"properties": map[string]any{
			"id":          stringSchema(),
			"description": stringSchema(),
			"body":        stringSchema(),
		},
	}
}

func goldenTestSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": true,
		"required":             []string{"name", "input"},
		"properties": map[string]any{
			"name":                stringSchema(),
			"input":               stringSchema(),
			"expect_mode":         stringSchema(),
			"expect_tool_policy":  enumSchema(string(ToolPolicyAllow), string(ToolPolicyRequireConfirmation)),
			"must_say_contains":   stringArraySchema(),
			"must_avoid_contains": stringArraySchema(),
		},
	}
}

func beliefUpdateProposalSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"id", "action", "reason", "requires_review"},
		"properties": map[string]any{
			"id":                 stringSchema(),
			"action":             enumSchema(string(BeliefUpdateAddPreference), string(BeliefUpdateReinforce), string(BeliefUpdateWeaken), string(BeliefUpdateReviewOnly)),
			"belief_id":          stringSchema(),
			"kind":               enumSchema(string(BeliefRoot), string(BeliefValue), string(BeliefRelational), string(BeliefBoundary), string(BeliefPreference)),
			"statement":          stringSchema(),
			"confidence_delta":   map[string]any{"type": "number"},
			"reason":             stringSchema(),
			"requires_review":    map[string]any{"type": "boolean"},
			"read_only_target":   map[string]any{"type": "boolean"},
			"source_feedback_id": stringSchema(),
			"evidence":           stringArraySchema(),
		},
	}
}

func packBundleApplyActionKindStrings() []string {
	kinds := PackBundleApplyActionKinds()
	values := make([]string, 0, len(kinds))
	for _, kind := range kinds {
		values = append(values, string(kind))
	}
	return values
}

func stringSchema() map[string]any { return map[string]any{"type": "string"} }

func integerSchema() map[string]any { return map[string]any{"type": "integer"} }

func stringArraySchema() map[string]any {
	return map[string]any{"type": "array", "items": stringSchema()}
}

func stringMapSchema() map[string]any {
	return map[string]any{"type": "object", "additionalProperties": stringSchema()}
}

func enumSchema(values ...string) map[string]any {
	return map[string]any{"type": "string", "enum": values}
}

func schemaString(schema JSONSchema, key string) string {
	value, _ := schema[key].(string)
	return value
}

func schemaDescription(name string) string {
	switch name {
	case "pack-manifest":
		return "Single declarative Cogni Pack manifest."
	case "pack-bundle":
		return "Portable collection of declarative Cogni Packs."
	case "pack-bundle-summary":
		return "Compact bundle inspection summary for previews and logs."
	case "pack-bundle-digest-check":
		return "Digest verification result for CI, install, and rollback checks."
	case "pack-bundle-diff":
		return "Non-mutating diff between current and candidate bundles."
	case "pack-bundle-review":
		return "Candidate review report with diff, golden-test, and rollback evidence."
	case "pack-bundle-apply-plan":
		return "Dry-run apply plan with gates, actions, diff, and golden-test evidence."
	case "pack-bundle-apply-actions":
		return "Script-friendly apply action list derived from an apply plan."
	case "pack-bundle-apply-action-kinds":
		return "Stable apply action vocabulary with UI labels and descriptions."
	case "pack-bundle-apply-checklist":
		return "UI-friendly apply checklist for plugin installers and dashboards."
	case "pack-bundle-apply-checklist-summary":
		return "Compact apply checklist counters for plugin dashboards and CI summaries."
	case "feedback-proposal":
		return "Non-mutating belief update proposal derived from audit feedback."
	default:
		return ""
	}
}
