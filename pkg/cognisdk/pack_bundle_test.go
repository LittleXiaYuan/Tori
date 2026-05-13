package cognisdk

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestPackManagerExportBundleRoundTrip(t *testing.T) {
	pm := NewPackManager(BuiltinPacks()...)
	if err := pm.Disable(PackXiaoyuCompanion); err != nil {
		t.Fatalf("disable companion pack: %v", err)
	}

	bundle, err := pm.ExportBundle("work-only-bundle")
	if err != nil {
		t.Fatalf("export bundle: %v", err)
	}
	if bundle.ID != "work-only-bundle" {
		t.Fatalf("bundle id = %q", bundle.ID)
	}
	if len(bundle.Packs) != 2 {
		t.Fatalf("expected 2 packs in bundle, got %d", len(bundle.Packs))
	}
	if len(bundle.EnabledPacks) != 1 || bundle.EnabledPacks[0] != PackYunqueWork {
		t.Fatalf("enabled packs not preserved: %#v", bundle.EnabledPacks)
	}

	restored, err := NewPackManagerFromBundle(bundle)
	if err != nil {
		t.Fatalf("restore bundle: %v", err)
	}
	merged := restored.Merge()
	if containsString(merged.PackIDs, PackXiaoyuCompanion) {
		t.Fatalf("disabled pack was re-enabled: %#v", merged.PackIDs)
	}
	if !containsString(merged.PackIDs, PackYunqueWork) {
		t.Fatalf("enabled work pack missing: %#v", merged.PackIDs)
	}
}

func TestSaveAndLoadPackBundle(t *testing.T) {
	bundle, err := NewPackBundle("portable", []PackManifest{XiaoyuCompanionPack()}, []string{PackXiaoyuCompanion})
	if err != nil {
		t.Fatalf("new bundle: %v", err)
	}
	path := filepath.Join(t.TempDir(), "portable.cogni-bundle.json")
	if err := SavePackBundle(bundle, path); err != nil {
		t.Fatalf("save bundle: %v", err)
	}
	loaded, err := LoadPackBundle(path)
	if err != nil {
		t.Fatalf("load bundle: %v", err)
	}
	if loaded.ID != bundle.ID || len(loaded.Packs) != 1 || loaded.Packs[0].ID != PackXiaoyuCompanion {
		t.Fatalf("loaded bundle mismatch: %#v", loaded)
	}
}

func TestValidatePackBundleRejectsMissingEnabledPack(t *testing.T) {
	bundle, err := NewPackBundle("bad", []PackManifest{XiaoyuCompanionPack()}, nil)
	if err != nil {
		t.Fatalf("new bundle: %v", err)
	}
	bundle.EnabledPacks = []string{"missing-pack"}
	if err := ValidatePackBundle(bundle); err == nil {
		t.Fatal("expected missing enabled pack to fail")
	}
}

func TestRunPackBundleGoldenTests(t *testing.T) {
	bundle, err := NewPackBundle("golden-bundle", BuiltinPacks(), []string{PackXiaoyuCompanion, PackYunqueWork})
	if err != nil {
		t.Fatalf("new bundle: %v", err)
	}
	summary, err := RunPackBundleGoldenTests(t.Context(), bundle)
	if err != nil {
		t.Fatalf("run bundle golden tests: %v", err)
	}
	if summary.Failed != 0 {
		t.Fatalf("expected no failures: %#v", summary)
	}
	if summary.Passed == 0 || len(summary.Results) == 0 {
		t.Fatalf("expected bundle golden tests to run: %#v", summary)
	}
}

func TestRenderGoldenTestSummaryMarkdown(t *testing.T) {
	summary := GoldenTestSummary{Passed: 1, Failed: 1, Results: []GoldenTestResult{
		{Name: "ok", Passed: true},
		{Name: "bad", Passed: false, Errors: []string{"mode mismatch"}},
	}}
	markdown := RenderGoldenTestSummaryMarkdown(summary)
	for _, want := range []string{"Cogni Pack Golden Tests", "passed: 1", "failed: 1", "[FAIL] bad", "mode mismatch"} {
		if !strings.Contains(markdown, want) {
			t.Fatalf("golden summary markdown missing %q:\n%s", want, markdown)
		}
	}
}

func TestSummarizePackBundle(t *testing.T) {
	bundle, err := NewPackBundle("summary", BuiltinPacks(), []string{PackXiaoyuCompanion})
	if err != nil {
		t.Fatalf("new bundle: %v", err)
	}
	summary, err := SummarizePackBundle(bundle)
	if err != nil {
		t.Fatalf("summarize bundle: %v", err)
	}
	if summary.PackCount != 2 || summary.EnabledCount != 1 || summary.DisabledCount != 1 {
		t.Fatalf("unexpected summary counts: %#v", summary)
	}
	if summary.GoldenTestCount == 0 {
		t.Fatalf("expected golden test count: %#v", summary)
	}
	if !strings.HasPrefix(summary.Digest, "sha256:") {
		t.Fatalf("expected bundle digest: %#v", summary)
	}
	markdown := RenderPackBundleSummaryMarkdown(summary)
	for _, want := range []string{"Cogni Pack Bundle Summary", "enabled: 1", PackXiaoyuCompanion} {
		if !strings.Contains(markdown, want) {
			t.Fatalf("summary markdown missing %q:\n%s", want, markdown)
		}
	}
}

func TestDigestPackBundleIsStableAcrossPackOrder(t *testing.T) {
	left, err := NewPackBundle("digest", []PackManifest{XiaoyuCompanionPack(), YunqueWorkPack()}, []string{PackYunqueWork, PackXiaoyuCompanion})
	if err != nil {
		t.Fatalf("left bundle: %v", err)
	}
	right, err := NewPackBundle("digest", []PackManifest{YunqueWorkPack(), XiaoyuCompanionPack()}, []string{PackXiaoyuCompanion, PackYunqueWork})
	if err != nil {
		t.Fatalf("right bundle: %v", err)
	}
	leftDigest, err := DigestPackBundle(left)
	if err != nil {
		t.Fatalf("left digest: %v", err)
	}
	rightDigest, err := DigestPackBundle(right)
	if err != nil {
		t.Fatalf("right digest: %v", err)
	}
	if leftDigest != rightDigest {
		t.Fatalf("digest changed with order: %s != %s", leftDigest, rightDigest)
	}
}
