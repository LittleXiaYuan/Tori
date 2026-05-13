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
	if err := run([]string{"init", bundlePath, "--builtin"}); err != nil {
		t.Fatalf("init bundle: %v", err)
	}
	if err := run([]string{"digest", bundlePath}); err != nil {
		t.Fatalf("digest bundle: %v", err)
	}
	if err := run([]string{"digest"}); err == nil || !strings.Contains(err.Error(), "usage: cognisdk-bundle digest") {
		t.Fatalf("expected digest usage error, got %v", err)
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
