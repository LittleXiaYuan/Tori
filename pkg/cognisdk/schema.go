package cognisdk

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// JSONSchema is a minimal JSON Schema document usable by frontends, plugins,
// and automation scripts that exchange Cognition SDK artifacts.
type JSONSchema map[string]any

// JSONSchemaNames returns stable names accepted by JSONSchemaByName.
func JSONSchemaNames() []string {
	return []string{"pack-manifest", "pack-bundle", "pack-bundle-summary", "pack-bundle-diff", "pack-bundle-review", "pack-bundle-apply-plan", "pack-bundle-apply-actions", "feedback-proposal"}
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
	case "pack-bundle-diff":
		return PackBundleDiffJSONSchema(), true
	case "pack-bundle-review":
		return PackBundleReviewJSONSchema(), true
	case "pack-bundle-apply-plan":
		return PackBundleApplyPlanJSONSchema(), true
	case "pack-bundle-apply-actions":
		return PackBundleApplyActionsJSONSchema(), true
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

func packBundleApplyActionSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"kind", "message"},
		"properties": map[string]any{
			"kind": enumSchema(
				string(PackBundleApplyActionKeepRollback),
				string(PackBundleApplyActionVerifyDigest),
				string(PackBundleApplyActionRequireReview),
				string(PackBundleApplyActionStopBlocked),
				string(PackBundleApplyActionAddPack),
				string(PackBundleApplyActionReplacePack),
				string(PackBundleApplyActionRemovePack),
				string(PackBundleApplyActionEnablePack),
				string(PackBundleApplyActionDisablePack),
				string(PackBundleApplyActionNoop),
				string(PackBundleApplyActionWriteCandidate),
			),
			"pack_id":      stringSchema(),
			"from_version": stringSchema(),
			"to_version":   stringSchema(),
			"digest":       stringSchema(),
			"bundle_id":    stringSchema(),
			"message":      stringSchema(),
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

func stringSchema() map[string]any { return map[string]any{"type": "string"} }

func stringArraySchema() map[string]any {
	return map[string]any{"type": "array", "items": stringSchema()}
}

func stringMapSchema() map[string]any {
	return map[string]any{"type": "object", "additionalProperties": stringSchema()}
}

func enumSchema(values ...string) map[string]any {
	return map[string]any{"type": "string", "enum": values}
}
