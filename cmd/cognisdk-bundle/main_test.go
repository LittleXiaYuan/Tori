package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yunque-agent/pkg/cognisdk"
)

func TestRunInitAndPromoteReadyBundle(t *testing.T) {
	dir := t.TempDir()
	current := filepath.Join(dir, "current.json")
	candidate := filepath.Join(dir, "candidate.json")
	promoted := filepath.Join(dir, "promoted.json")
	reviewOut := filepath.Join(dir, "review.json")

	if err := run([]string{"init", current}); err != nil {
		t.Fatalf("init current: %v", err)
	}
	if err := run([]string{"init", candidate, "--builtin"}); err != nil {
		t.Fatalf("init candidate: %v", err)
	}
	if err := run([]string{"promote", current, candidate, promoted, "--review-out", reviewOut}); err != nil {
		t.Fatalf("promote candidate: %v", err)
	}
	bundle, err := cognisdk.LoadPackBundle(promoted)
	if err != nil {
		t.Fatalf("load promoted bundle: %v", err)
	}
	if bundle.ID != "builtin-cogni-pack-bundle" {
		t.Fatalf("promoted bundle id = %q", bundle.ID)
	}
	reviewData, err := os.ReadFile(reviewOut)
	if err != nil {
		t.Fatalf("read review output: %v", err)
	}
	var review cognisdk.PackBundleReview
	if err := json.Unmarshal(reviewData, &review); err != nil {
		t.Fatalf("review output is not json: %v", err)
	}
	if review.Outcome != cognisdk.PackBundleReviewReady {
		t.Fatalf("review outcome = %q", review.Outcome)
	}
}

func TestRunPromoteRejectsReviewWithoutOverride(t *testing.T) {
	dir := t.TempDir()
	currentPack := cognisdk.XiaoyuCompanionPack()
	current, err := cognisdk.NewPackBundle("current", []cognisdk.PackManifest{currentPack}, []string{cognisdk.PackXiaoyuCompanion})
	if err != nil {
		t.Fatalf("current bundle: %v", err)
	}
	changed := currentPack
	changed.Version = "0.2.0"
	candidate, err := cognisdk.NewPackBundle("candidate", []cognisdk.PackManifest{changed}, []string{cognisdk.PackXiaoyuCompanion})
	if err != nil {
		t.Fatalf("candidate bundle: %v", err)
	}
	currentPath := filepath.Join(dir, "current.json")
	candidatePath := filepath.Join(dir, "candidate.json")
	outputPath := filepath.Join(dir, "promoted.json")
	if err := cognisdk.SavePackBundle(current, currentPath); err != nil {
		t.Fatalf("save current: %v", err)
	}
	if err := cognisdk.SavePackBundle(candidate, candidatePath); err != nil {
		t.Fatalf("save candidate: %v", err)
	}

	err = run([]string{"promote", currentPath, candidatePath, outputPath})
	if err == nil || !strings.Contains(err.Error(), "requires review") {
		t.Fatalf("expected requires review error, got %v", err)
	}
	if _, statErr := os.Stat(outputPath); !os.IsNotExist(statErr) {
		t.Fatalf("promote wrote output despite review gate: %v", statErr)
	}
	if err := run([]string{"promote", currentPath, candidatePath, outputPath, "--allow-review"}); err != nil {
		t.Fatalf("promote with allow-review: %v", err)
	}
	if _, err := os.Stat(outputPath); err != nil {
		t.Fatalf("expected promoted output: %v", err)
	}
}

func TestRunPlanFailOnReviewGate(t *testing.T) {
	dir := t.TempDir()
	currentPack := cognisdk.XiaoyuCompanionPack()
	current, err := cognisdk.NewPackBundle("current", []cognisdk.PackManifest{currentPack}, []string{cognisdk.PackXiaoyuCompanion})
	if err != nil {
		t.Fatalf("current bundle: %v", err)
	}
	changed := currentPack
	changed.Version = "0.2.0"
	candidate, err := cognisdk.NewPackBundle("candidate", []cognisdk.PackManifest{changed}, []string{cognisdk.PackXiaoyuCompanion})
	if err != nil {
		t.Fatalf("candidate bundle: %v", err)
	}
	currentPath := filepath.Join(dir, "current.json")
	candidatePath := filepath.Join(dir, "candidate.json")
	planOut := filepath.Join(dir, "review-plan.json")
	if err := cognisdk.SavePackBundle(current, currentPath); err != nil {
		t.Fatalf("save current: %v", err)
	}
	if err := cognisdk.SavePackBundle(candidate, candidatePath); err != nil {
		t.Fatalf("save candidate: %v", err)
	}

	err = run([]string{"plan", currentPath, candidatePath, "--out", planOut, "--fail-on-review"})
	if err == nil || !strings.Contains(err.Error(), "requires review") {
		t.Fatalf("expected fail-on-review error, got %v", err)
	}
	data, readErr := os.ReadFile(planOut)
	if readErr != nil {
		t.Fatalf("plan output should be written before gate failure: %v", readErr)
	}
	var plan cognisdk.PackBundleApplyPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		t.Fatalf("plan output is not json: %v", err)
	}
	if !plan.RequiresReview || plan.Outcome != cognisdk.PackBundleReviewReview {
		t.Fatalf("unexpected review plan: %#v", plan)
	}
}

func TestRunPlanFailOnBlockedGate(t *testing.T) {
	dir := t.TempDir()
	badPack := cognisdk.PackManifest{
		ID:      "bad-pack",
		Version: "0.1.0",
		Type:    "cogni",
		GoldenTests: []cognisdk.GoldenTest{{
			Name:       "bad expectation",
			Input:      "hello",
			ExpectMode: "impossible_mode",
		}},
	}
	current, err := cognisdk.NewPackBundle("current", nil, nil)
	if err != nil {
		t.Fatalf("current bundle: %v", err)
	}
	candidate, err := cognisdk.NewPackBundle("candidate", []cognisdk.PackManifest{badPack}, []string{"bad-pack"})
	if err != nil {
		t.Fatalf("candidate bundle: %v", err)
	}
	currentPath := filepath.Join(dir, "current.json")
	candidatePath := filepath.Join(dir, "candidate.json")
	if err := cognisdk.SavePackBundle(current, currentPath); err != nil {
		t.Fatalf("save current: %v", err)
	}
	if err := cognisdk.SavePackBundle(candidate, candidatePath); err != nil {
		t.Fatalf("save candidate: %v", err)
	}

	err = run([]string{"plan", currentPath, candidatePath, "--fail-on-blocked"})
	if err == nil || !strings.Contains(err.Error(), "blocked") {
		t.Fatalf("expected fail-on-blocked error, got %v", err)
	}
}

func TestRunGoldenOutputsJSON(t *testing.T) {
	dir := t.TempDir()
	candidate := filepath.Join(dir, "candidate.json")
	if err := run([]string{"init", candidate, "--builtin"}); err != nil {
		t.Fatalf("init candidate: %v", err)
	}

	// Exercise the JSON path through the command without asserting stdout capture.
	if err := run([]string{"golden", candidate}); err != nil {
		t.Fatalf("golden command: %v", err)
	}

	data, err := os.ReadFile(candidate)
	if err != nil {
		t.Fatalf("read candidate: %v", err)
	}
	var bundle cognisdk.PackBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		t.Fatalf("candidate is not json bundle: %v", err)
	}
	if len(bundle.Packs) == 0 {
		t.Fatal("expected builtin candidate packs")
	}
}

func TestRunPromoteReviewOutRequiresPath(t *testing.T) {
	err := run([]string{"promote", "current.json", "candidate.json", "out.json", "--review-out"})
	if err == nil || !strings.Contains(err.Error(), "--review-out requires a path") {
		t.Fatalf("expected review-out path error, got %v", err)
	}
}

func TestRunInspectBundle(t *testing.T) {
	dir := t.TempDir()
	bundlePath := filepath.Join(dir, "bundle.json")
	if err := run([]string{"init", bundlePath, "--builtin"}); err != nil {
		t.Fatalf("init bundle: %v", err)
	}
	if err := run([]string{"inspect", bundlePath}); err != nil {
		t.Fatalf("inspect bundle: %v", err)
	}
	if err := run([]string{"inspect", bundlePath, "--markdown"}); err != nil {
		t.Fatalf("inspect bundle markdown: %v", err)
	}
}

func TestRunDigestBundle(t *testing.T) {
	dir := t.TempDir()
	bundlePath := filepath.Join(dir, "bundle.json")
	digestOut := filepath.Join(dir, "digest.txt")
	checkOut := filepath.Join(dir, "digest-check.json")
	mismatchOut := filepath.Join(dir, "digest-mismatch.json")
	if err := run([]string{"init", bundlePath, "--builtin"}); err != nil {
		t.Fatalf("init bundle: %v", err)
	}
	if err := run([]string{"digest", bundlePath}); err != nil {
		t.Fatalf("digest bundle: %v", err)
	}
	bundle, err := cognisdk.LoadPackBundle(bundlePath)
	if err != nil {
		t.Fatalf("load bundle: %v", err)
	}
	digest, err := cognisdk.DigestPackBundle(*bundle)
	if err != nil {
		t.Fatalf("compute digest: %v", err)
	}
	if err := run([]string{"digest", bundlePath, "--out", digestOut}); err != nil {
		t.Fatalf("digest out: %v", err)
	}
	digestData, err := os.ReadFile(digestOut)
	if err != nil {
		t.Fatalf("read digest output: %v", err)
	}
	if strings.TrimSpace(string(digestData)) != digest {
		t.Fatalf("digest output = %q, want %q", strings.TrimSpace(string(digestData)), digest)
	}
	if err := run([]string{"digest", bundlePath, "--expect", digest}); err != nil {
		t.Fatalf("digest expect match: %v", err)
	}
	if err := run([]string{"digest", bundlePath, "--expect", digest, "--out", checkOut}); err != nil {
		t.Fatalf("digest expect out: %v", err)
	}
	checkData, err := os.ReadFile(checkOut)
	if err != nil {
		t.Fatalf("read digest check output: %v", err)
	}
	var check cognisdk.PackBundleDigestCheck
	if err := json.Unmarshal(checkData, &check); err != nil {
		t.Fatalf("digest check output is not json: %v", err)
	}
	if !check.Match || check.Actual != digest || check.Expected != digest {
		t.Fatalf("unexpected digest check: %#v", check)
	}
	if err := run([]string{"digest", bundlePath, "--expect", "sha256:wrong", "--out", mismatchOut}); err == nil || !strings.Contains(err.Error(), "digest mismatch") {
		t.Fatalf("expected digest mismatch error, got %v", err)
	}
	mismatchData, err := os.ReadFile(mismatchOut)
	if err != nil {
		t.Fatalf("mismatch output should be written before failure: %v", err)
	}
	var mismatch cognisdk.PackBundleDigestCheck
	if err := json.Unmarshal(mismatchData, &mismatch); err != nil {
		t.Fatalf("mismatch digest check output is not json: %v", err)
	}
	if mismatch.Match || mismatch.Actual != digest || mismatch.Expected != "sha256:wrong" {
		t.Fatalf("unexpected mismatch digest check: %#v", mismatch)
	}
	if err := run([]string{"digest", bundlePath, "--bad", digest}); err == nil || !strings.Contains(err.Error(), "unknown digest option") {
		t.Fatalf("expected unknown digest option error, got %v", err)
	}
	if err := run([]string{"digest", bundlePath, "--expect"}); err == nil || !strings.Contains(err.Error(), "--expect requires a digest") {
		t.Fatalf("expected expect path error, got %v", err)
	}
	if err := run([]string{"digest", bundlePath, "--out"}); err == nil || !strings.Contains(err.Error(), "--out requires a path") {
		t.Fatalf("expected digest out path error, got %v", err)
	}
	if err := run([]string{"digest"}); err == nil || !strings.Contains(err.Error(), "usage: cognisdk-bundle digest") {
		t.Fatalf("expected digest usage error, got %v", err)
	}
}

func TestRunActionKinds(t *testing.T) {
	dir := t.TempDir()
	jsonOut := filepath.Join(dir, "action-kinds.json")
	markdownOut := filepath.Join(dir, "action-kinds.md")
	detailsOut := filepath.Join(dir, "action-kind-infos.json")
	detailsMarkdownOut := filepath.Join(dir, "action-kind-infos.md")
	if err := run([]string{"action-kinds"}); err != nil {
		t.Fatalf("action-kinds: %v", err)
	}
	if err := run([]string{"action-kinds", "--markdown"}); err != nil {
		t.Fatalf("action-kinds markdown: %v", err)
	}
	if err := run([]string{"action-kinds", "--details"}); err != nil {
		t.Fatalf("action-kinds details: %v", err)
	}
	if err := run([]string{"action-kinds", "--details", "--markdown"}); err != nil {
		t.Fatalf("action-kinds details markdown: %v", err)
	}
	if err := run([]string{"action-kinds", "--out", jsonOut}); err != nil {
		t.Fatalf("action-kinds out: %v", err)
	}
	jsonData, err := os.ReadFile(jsonOut)
	if err != nil {
		t.Fatalf("read action kinds json: %v", err)
	}
	var kinds []cognisdk.PackBundleApplyActionKind
	if err := json.Unmarshal(jsonData, &kinds); err != nil {
		t.Fatalf("action kinds output is not json: %v", err)
	}
	if len(kinds) == 0 || kinds[0] != cognisdk.PackBundleApplyActionKeepRollback {
		t.Fatalf("unexpected action kinds output: %#v", kinds)
	}
	if err := run([]string{"action-kinds", "--out", markdownOut, "--markdown"}); err != nil {
		t.Fatalf("action-kinds markdown out: %v", err)
	}
	markdownData, err := os.ReadFile(markdownOut)
	if err != nil {
		t.Fatalf("read action kinds markdown: %v", err)
	}
	if !strings.Contains(string(markdownData), "Cogni Pack Bundle Apply Action Kinds") {
		t.Fatalf("action kinds markdown missing heading: %s", markdownData)
	}
	if err := run([]string{"action-kinds", "--details", "--out", detailsOut}); err != nil {
		t.Fatalf("action-kinds details out: %v", err)
	}
	detailsData, err := os.ReadFile(detailsOut)
	if err != nil {
		t.Fatalf("read action kinds details json: %v", err)
	}
	var infos []cognisdk.PackBundleApplyActionKindInfo
	if err := json.Unmarshal(detailsData, &infos); err != nil {
		t.Fatalf("action kind details output is not json: %v", err)
	}
	if len(infos) == 0 || infos[0].Kind != cognisdk.PackBundleApplyActionKeepRollback || infos[0].Label == "" {
		t.Fatalf("unexpected action kind details output: %#v", infos)
	}
	if err := run([]string{"action-kinds", "--details", "--out", detailsMarkdownOut, "--markdown"}); err != nil {
		t.Fatalf("action-kinds details markdown out: %v", err)
	}
	detailsMarkdownData, err := os.ReadFile(detailsMarkdownOut)
	if err != nil {
		t.Fatalf("read action kind details markdown: %v", err)
	}
	if !strings.Contains(string(detailsMarkdownData), "Keep rollback bundle") {
		t.Fatalf("action kind details markdown missing label: %s", detailsMarkdownData)
	}
	if err := run([]string{"action-kinds", "--out"}); err == nil || !strings.Contains(err.Error(), "--out requires a path") {
		t.Fatalf("expected action-kinds out path error, got %v", err)
	}
	if err := run([]string{"action-kinds", "--bad"}); err == nil || !strings.Contains(err.Error(), "unknown action-kinds option") {
		t.Fatalf("expected action-kinds unknown option error, got %v", err)
	}
	if err := run([]string{"action-kinds", "extra"}); err == nil || !strings.Contains(err.Error(), "usage: cognisdk-bundle action-kinds") {
		t.Fatalf("expected action-kinds usage error, got %v", err)
	}
}

func TestRenderApplyActionKindsMarkdown(t *testing.T) {
	markdown := renderApplyActionKindsMarkdown([]cognisdk.PackBundleApplyActionKind{cognisdk.PackBundleApplyActionAddPack})
	if !strings.Contains(markdown, "Cogni Pack Bundle Apply Action Kinds") || !strings.Contains(markdown, "add_pack") {
		t.Fatalf("unexpected action kinds markdown: %s", markdown)
	}
}

func TestRenderApplyActionKindInfosMarkdown(t *testing.T) {
	markdown := renderApplyActionKindInfosMarkdown([]cognisdk.PackBundleApplyActionKindInfo{{
		Kind:        cognisdk.PackBundleApplyActionAddPack,
		Label:       "Add pack",
		Description: "Add a pack.",
	}})
	if !strings.Contains(markdown, "Cogni Pack Bundle Apply Action Kinds") || !strings.Contains(markdown, "Add pack") || !strings.Contains(markdown, "Add a pack.") {
		t.Fatalf("unexpected action kind infos markdown: %s", markdown)
	}
}

func TestRunActionsBundle(t *testing.T) {
	dir := t.TempDir()
	current := filepath.Join(dir, "current.json")
	candidate := filepath.Join(dir, "candidate.json")
	actionsOut := filepath.Join(dir, "actions.json")
	actionsMarkdownOut := filepath.Join(dir, "actions.md")
	if err := run([]string{"init", current}); err != nil {
		t.Fatalf("init current: %v", err)
	}
	if err := run([]string{"init", candidate, "--builtin"}); err != nil {
		t.Fatalf("init candidate: %v", err)
	}
	if err := run([]string{"actions", current, candidate}); err != nil {
		t.Fatalf("actions bundle: %v", err)
	}
	if err := run([]string{"actions", current, candidate, "--markdown"}); err != nil {
		t.Fatalf("actions bundle markdown: %v", err)
	}
	if err := run([]string{"actions", current, candidate, "--out", actionsOut}); err != nil {
		t.Fatalf("actions bundle out: %v", err)
	}
	data, err := os.ReadFile(actionsOut)
	if err != nil {
		t.Fatalf("read actions output: %v", err)
	}
	var actions []cognisdk.PackBundleApplyAction
	if err := json.Unmarshal(data, &actions); err != nil {
		t.Fatalf("actions output is not json: %v", err)
	}
	if len(actions) == 0 || actions[0].Kind != cognisdk.PackBundleApplyActionKeepRollback {
		t.Fatalf("unexpected actions output: %#v", actions)
	}
	filteredOut := filepath.Join(dir, "actions-add-pack.json")
	if err := run([]string{"actions", current, candidate, "--kind", "add_pack", "--out", filteredOut}); err != nil {
		t.Fatalf("actions kind filter out: %v", err)
	}
	filteredData, err := os.ReadFile(filteredOut)
	if err != nil {
		t.Fatalf("read filtered actions output: %v", err)
	}
	var filtered []cognisdk.PackBundleApplyAction
	if err := json.Unmarshal(filteredData, &filtered); err != nil {
		t.Fatalf("filtered actions output is not json: %v", err)
	}
	if len(filtered) == 0 {
		t.Fatal("expected filtered add_pack actions")
	}
	for _, action := range filtered {
		if action.Kind != cognisdk.PackBundleApplyActionAddPack {
			t.Fatalf("unexpected filtered action: %#v", action)
		}
	}
	if err := run([]string{"actions", current, candidate, "--kind", "missing_kind"}); err == nil || !strings.Contains(err.Error(), "unknown action kind") {
		t.Fatalf("expected unknown action kind error, got %v", err)
	}
	if err := run([]string{"actions", current, candidate, "--kind"}); err == nil || !strings.Contains(err.Error(), "--kind requires an action kind") {
		t.Fatalf("expected kind path error, got %v", err)
	}
	if err := run([]string{"actions", current, candidate, "--out", actionsMarkdownOut, "--markdown"}); err != nil {
		t.Fatalf("actions bundle markdown out: %v", err)
	}
	markdownData, err := os.ReadFile(actionsMarkdownOut)
	if err != nil {
		t.Fatalf("read actions markdown output: %v", err)
	}
	if !strings.Contains(string(markdownData), "Cogni Pack Bundle Apply Actions") {
		t.Fatalf("actions markdown missing heading: %s", markdownData)
	}
	if err := run([]string{"actions", current}); err == nil || !strings.Contains(err.Error(), "usage: cognisdk-bundle actions") {
		t.Fatalf("expected actions usage error, got %v", err)
	}
}

func TestRunPlanBundle(t *testing.T) {
	dir := t.TempDir()
	current := filepath.Join(dir, "current.json")
	candidate := filepath.Join(dir, "candidate.json")
	planOut := filepath.Join(dir, "plan.json")
	planMarkdownOut := filepath.Join(dir, "plan.md")
	if err := run([]string{"init", current}); err != nil {
		t.Fatalf("init current: %v", err)
	}
	if err := run([]string{"init", candidate, "--builtin"}); err != nil {
		t.Fatalf("init candidate: %v", err)
	}
	if err := run([]string{"plan", current, candidate}); err != nil {
		t.Fatalf("plan bundle: %v", err)
	}
	if err := run([]string{"plan", current, candidate, "--markdown"}); err != nil {
		t.Fatalf("plan bundle markdown: %v", err)
	}
	if err := run([]string{"plan", current, candidate, "--out", planOut}); err != nil {
		t.Fatalf("plan bundle out: %v", err)
	}
	data, err := os.ReadFile(planOut)
	if err != nil {
		t.Fatalf("read plan output: %v", err)
	}
	var plan cognisdk.PackBundleApplyPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		t.Fatalf("plan output is not json: %v", err)
	}
	if plan.Outcome != cognisdk.PackBundleReviewReady || plan.CandidateDigest == "" {
		t.Fatalf("unexpected plan output: %#v", plan)
	}
	if err := run([]string{"plan", current, candidate, "--out", planMarkdownOut, "--markdown"}); err != nil {
		t.Fatalf("plan bundle markdown out: %v", err)
	}
	markdownData, err := os.ReadFile(planMarkdownOut)
	if err != nil {
		t.Fatalf("read markdown plan output: %v", err)
	}
	if !strings.Contains(string(markdownData), "Cogni Pack Bundle Apply Plan") {
		t.Fatalf("markdown plan output missing heading: %s", markdownData)
	}
	if err := run([]string{"plan", current, candidate, "--out"}); err == nil || !strings.Contains(err.Error(), "--out requires a path") {
		t.Fatalf("expected plan out path error, got %v", err)
	}
	if err := run([]string{"plan", current}); err == nil || !strings.Contains(err.Error(), "usage: cognisdk-bundle plan") {
		t.Fatalf("expected plan usage error, got %v", err)
	}
}
