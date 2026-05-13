package cognisdk

import (
	"strings"
	"testing"
)

func TestPlanPackBundleApplyReady(t *testing.T) {
	current, err := NewPackBundle("current", []PackManifest{XiaoyuCompanionPack()}, []string{PackXiaoyuCompanion})
	if err != nil {
		t.Fatalf("current bundle: %v", err)
	}
	candidate, err := NewPackBundle("candidate", BuiltinPacks(), []string{PackXiaoyuCompanion, PackYunqueWork})
	if err != nil {
		t.Fatalf("candidate bundle: %v", err)
	}

	plan, err := PlanPackBundleApply(t.Context(), current, candidate)
	if err != nil {
		t.Fatalf("plan apply: %v", err)
	}
	if plan.Outcome != PackBundleReviewReady || plan.RequiresReview || plan.Blocked {
		t.Fatalf("unexpected apply plan gate: %#v", plan)
	}
	if !strings.HasPrefix(plan.FromDigest, "sha256:") || !strings.HasPrefix(plan.CandidateDigest, "sha256:") {
		t.Fatalf("plan missing digests: %#v", plan)
	}
	if len(plan.Diff.AddedPacks) != 1 || plan.Diff.AddedPacks[0].ID != PackYunqueWork {
		t.Fatalf("expected yunque work add action diff: %#v", plan.Diff)
	}
	joined := strings.Join(plan.RecommendedActions, "\n")
	for _, want := range []string{"keep rollback bundle", "verify current digest", "add pack \"" + PackYunqueWork + "\"", "write candidate bundle"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("apply actions missing %q: %#v", want, plan.RecommendedActions)
		}
	}
}

func TestPlanPackBundleApplyRequiresReviewForChangedPack(t *testing.T) {
	currentPack := XiaoyuCompanionPack()
	current, err := NewPackBundle("current", []PackManifest{currentPack}, []string{PackXiaoyuCompanion})
	if err != nil {
		t.Fatalf("current bundle: %v", err)
	}
	changed := currentPack
	changed.Version = "0.2.0"
	candidate, err := NewPackBundle("candidate", []PackManifest{changed}, []string{PackXiaoyuCompanion})
	if err != nil {
		t.Fatalf("candidate bundle: %v", err)
	}
	plan, err := PlanPackBundleApply(t.Context(), current, candidate)
	if err != nil {
		t.Fatalf("plan apply: %v", err)
	}
	if plan.Outcome != PackBundleReviewReview || !plan.RequiresReview || plan.Blocked {
		t.Fatalf("expected review gate: %#v", plan)
	}
	if !strings.Contains(strings.Join(plan.RecommendedActions, "\n"), "require a human or policy approval") {
		t.Fatalf("missing approval action: %#v", plan.RecommendedActions)
	}
}

func TestPlanPackBundleApplyBlocksGoldenFailure(t *testing.T) {
	badPack := PackManifest{
		ID:      "bad-pack",
		Version: "0.1.0",
		Type:    "cogni",
		GoldenTests: []GoldenTest{{
			Name:       "bad expectation",
			Input:      "hello",
			ExpectMode: "impossible_mode",
		}},
	}
	current, err := NewPackBundle("current", nil, nil)
	if err != nil {
		t.Fatalf("current bundle: %v", err)
	}
	candidate, err := NewPackBundle("candidate", []PackManifest{badPack}, []string{"bad-pack"})
	if err != nil {
		t.Fatalf("candidate bundle: %v", err)
	}
	plan, err := PlanPackBundleApply(t.Context(), current, candidate)
	if err != nil {
		t.Fatalf("plan apply: %v", err)
	}
	if plan.Outcome != PackBundleReviewBlocked || !plan.Blocked {
		t.Fatalf("expected blocked plan: %#v", plan)
	}
	if !strings.Contains(strings.Join(plan.RecommendedActions, "\n"), "do not promote") {
		t.Fatalf("missing do-not-promote action: %#v", plan.RecommendedActions)
	}
}

func TestFilterPackBundleApplyActions(t *testing.T) {
	actions := []PackBundleApplyAction{
		{Kind: PackBundleApplyActionKeepRollback, Message: "keep"},
		{Kind: PackBundleApplyActionAddPack, PackID: PackYunqueWork, Message: "add"},
		{Kind: PackBundleApplyActionWriteCandidate, Message: "write"},
	}
	if got := FilterPackBundleApplyActions(actions); len(got) != len(actions) {
		t.Fatalf("no-kind filter changed actions: %#v", got)
	}
	filtered := FilterPackBundleApplyActions(actions, PackBundleApplyActionAddPack, PackBundleApplyActionWriteCandidate)
	if len(filtered) != 2 {
		t.Fatalf("expected two filtered actions, got %#v", filtered)
	}
	if filtered[0].Kind != PackBundleApplyActionAddPack || filtered[1].Kind != PackBundleApplyActionWriteCandidate {
		t.Fatalf("unexpected filtered actions: %#v", filtered)
	}
	if got := FilterPackBundleApplyActions(actions, PackBundleApplyActionRemovePack); len(got) != 0 {
		t.Fatalf("expected empty filter result, got %#v", got)
	}
}

func TestPackBundleApplyActionKinds(t *testing.T) {
	kinds := PackBundleApplyActionKinds()
	if len(kinds) == 0 {
		t.Fatal("expected action kinds")
	}
	seen := map[PackBundleApplyActionKind]bool{}
	for _, kind := range kinds {
		if seen[kind] {
			t.Fatalf("duplicate action kind %q", kind)
		}
		seen[kind] = true
		if !KnownPackBundleApplyActionKind(kind) {
			t.Fatalf("listed action kind not known: %q", kind)
		}
	}
	for _, want := range []PackBundleApplyActionKind{PackBundleApplyActionAddPack, PackBundleApplyActionVerifyDigest, PackBundleApplyActionWriteCandidate} {
		if !seen[want] {
			t.Fatalf("action kind list missing %q", want)
		}
	}
	if KnownPackBundleApplyActionKind(PackBundleApplyActionKind("missing_kind")) {
		t.Fatal("unexpected missing action kind accepted")
	}
}

func TestPackBundleApplyActionKindInfos(t *testing.T) {
	kinds := PackBundleApplyActionKinds()
	infos := PackBundleApplyActionKindInfos()
	if len(infos) != len(kinds) {
		t.Fatalf("kind info length = %d, want %d", len(infos), len(kinds))
	}
	for i, info := range infos {
		if info.Kind != kinds[i] {
			t.Fatalf("kind info[%d] = %q, want %q", i, info.Kind, kinds[i])
		}
		if !KnownPackBundleApplyActionKind(info.Kind) {
			t.Fatalf("kind info not known: %#v", info)
		}
		if strings.TrimSpace(info.Label) == "" || strings.TrimSpace(info.Description) == "" {
			t.Fatalf("kind info missing copy: %#v", info)
		}
	}
}

func TestDescribePackBundleApplyActionKind(t *testing.T) {
	info, ok := DescribePackBundleApplyActionKind(PackBundleApplyActionAddPack)
	if !ok {
		t.Fatal("expected add_pack description")
	}
	if info.Kind != PackBundleApplyActionAddPack || strings.TrimSpace(info.Label) == "" || strings.TrimSpace(info.Description) == "" {
		t.Fatalf("unexpected add_pack info: %#v", info)
	}
	if _, ok := DescribePackBundleApplyActionKind(PackBundleApplyActionKind("missing_kind")); ok {
		t.Fatal("unexpected missing action kind info")
	}
}

func TestBuildPackBundleApplyChecklist(t *testing.T) {
	plan := PackBundleApplyPlan{
		Actions: []PackBundleApplyAction{
			{Kind: PackBundleApplyActionKeepRollback, BundleID: "current", Message: "keep current"},
			{Kind: PackBundleApplyActionAddPack, PackID: PackYunqueWork, Message: "add work"},
			{Kind: PackBundleApplyActionWriteCandidate, BundleID: "candidate", Message: "write candidate"},
		},
	}
	items := BuildPackBundleApplyChecklist(plan)
	if len(items) != len(plan.Actions) {
		t.Fatalf("checklist length = %d, want %d", len(items), len(plan.Actions))
	}
	if items[0].Kind != PackBundleApplyActionKeepRollback || !items[0].Required || items[0].Done || items[0].Blocked {
		t.Fatalf("unexpected rollback item: %#v", items[0])
	}
	if items[1].Kind != PackBundleApplyActionAddPack || items[1].Required || items[1].Label == "" || items[1].Action == nil || items[1].Action.PackID != PackYunqueWork {
		t.Fatalf("unexpected add_pack item: %#v", items[1])
	}
	if items[2].Kind != PackBundleApplyActionWriteCandidate || !items[2].Required {
		t.Fatalf("unexpected write item: %#v", items[2])
	}
}

func TestBuildPackBundleApplyChecklistBlocked(t *testing.T) {
	plan := PackBundleApplyPlan{
		Blocked: true,
		Actions: []PackBundleApplyAction{
			{Kind: PackBundleApplyActionStopBlocked, Message: "stop"},
		},
	}
	items := BuildPackBundleApplyChecklist(plan)
	if len(items) != 1 || !items[0].Required || !items[0].Blocked {
		t.Fatalf("unexpected blocked checklist: %#v", items)
	}
}

func TestFilterPackBundleApplyChecklistItems(t *testing.T) {
	items := []PackBundleApplyChecklistItem{
		{Kind: PackBundleApplyActionKeepRollback, Message: "keep"},
		{Kind: PackBundleApplyActionAddPack, Message: "add"},
		{Kind: PackBundleApplyActionWriteCandidate, Message: "write"},
	}
	if got := FilterPackBundleApplyChecklistItems(items); len(got) != len(items) {
		t.Fatalf("no-kind checklist filter changed items: %#v", got)
	}
	filtered := FilterPackBundleApplyChecklistItems(items, PackBundleApplyActionAddPack, PackBundleApplyActionWriteCandidate)
	if len(filtered) != 2 {
		t.Fatalf("expected two filtered checklist items, got %#v", filtered)
	}
	if filtered[0].Kind != PackBundleApplyActionAddPack || filtered[1].Kind != PackBundleApplyActionWriteCandidate {
		t.Fatalf("unexpected filtered checklist items: %#v", filtered)
	}
	if got := FilterPackBundleApplyChecklistItems(items, PackBundleApplyActionRemovePack); len(got) != 0 {
		t.Fatalf("expected empty checklist filter result, got %#v", got)
	}
}

func TestSummarizePackBundleApplyChecklist(t *testing.T) {
	items := []PackBundleApplyChecklistItem{
		{Kind: PackBundleApplyActionKeepRollback, Required: true, Done: true, Message: "keep"},
		{Kind: PackBundleApplyActionVerifyDigest, Required: true, Message: "verify"},
		{Kind: PackBundleApplyActionAddPack, Message: "add"},
		{Kind: PackBundleApplyActionStopBlocked, Required: true, Blocked: true, Message: "stop"},
	}
	summary := SummarizePackBundleApplyChecklist(items)
	if summary.Total != 4 || summary.Required != 3 || summary.Optional != 1 {
		t.Fatalf("unexpected summary totals: %#v", summary)
	}
	if summary.Done != 1 || summary.Open != 3 || summary.RequiredDone != 1 || summary.RequiredOpen != 2 || summary.OptionalOpen != 1 {
		t.Fatalf("unexpected summary progress: %#v", summary)
	}
	if summary.Blocked != 1 || len(summary.BlockedKinds) != 1 || summary.BlockedKinds[0] != PackBundleApplyActionStopBlocked {
		t.Fatalf("unexpected blocked summary: %#v", summary)
	}
	if summary.ByKind[PackBundleApplyActionVerifyDigest] != 1 || summary.ByKind[PackBundleApplyActionAddPack] != 1 {
		t.Fatalf("unexpected kind counts: %#v", summary.ByKind)
	}
}

func TestRenderPackBundleApplyChecklistSummaryMarkdown(t *testing.T) {
	summary := PackBundleApplyChecklistSummary{
		Total:         2,
		Required:      1,
		Optional:      1,
		Open:          2,
		Blocked:       1,
		RequiredOpen:  1,
		BlockedKinds:  []PackBundleApplyActionKind{PackBundleApplyActionStopBlocked},
		RequiredKinds: []PackBundleApplyActionKind{PackBundleApplyActionVerifyDigest},
		ByKind: map[PackBundleApplyActionKind]int{
			PackBundleApplyActionVerifyDigest: 1,
			PackBundleApplyActionAddPack:      1,
		},
	}
	markdown := RenderPackBundleApplyChecklistSummaryMarkdown(summary)
	for _, want := range []string{"Cogni Pack Bundle Apply Checklist Summary", "required: 1", "blocked_kinds: stop_blocked", "`verify_digest`: 1"} {
		if !strings.Contains(markdown, want) {
			t.Fatalf("checklist summary markdown missing %q:\n%s", want, markdown)
		}
	}
}

func TestRenderPackBundleApplyChecklistMarkdown(t *testing.T) {
	markdown := RenderPackBundleApplyChecklistMarkdown([]PackBundleApplyChecklistItem{{
		Kind:     PackBundleApplyActionVerifyDigest,
		Label:    "Verify digest",
		Required: true,
		Message:  "verify current digest",
	}})
	if !strings.Contains(markdown, "Cogni Pack Bundle Apply Checklist") || !strings.Contains(markdown, "Verify digest") || !strings.Contains(markdown, "required") {
		t.Fatalf("unexpected checklist markdown: %s", markdown)
	}
}

func TestRenderPackBundleApplyPlanMarkdown(t *testing.T) {
	plan := PackBundleApplyPlan{
		FromID:             "current",
		CandidateID:        "candidate",
		FromDigest:         "sha256:current",
		CandidateDigest:    "sha256:candidate",
		Outcome:            PackBundleReviewReview,
		Reason:             "needs approval",
		RequiresReview:     true,
		RollbackBundleID:   "current",
		RecommendedActions: []string{"require a human or policy approval before writing the candidate bundle"},
		Diff:               PackBundleDiff{FromID: "current", ToID: "candidate"},
		GoldenTests:        GoldenTestSummary{Passed: 1},
	}
	markdown := RenderPackBundleApplyPlanMarkdown(plan)
	for _, want := range []string{"Cogni Pack Bundle Apply Plan", "requires_review: true", "current_digest: sha256:current", "Recommended Actions", "Cogni Pack Bundle Diff", "Cogni Pack Golden Tests"} {
		if !strings.Contains(markdown, want) {
			t.Fatalf("apply plan markdown missing %q:\n%s", want, markdown)
		}
	}
}

func hasApplyAction(actions []PackBundleApplyAction, kind PackBundleApplyActionKind) bool {
	for _, action := range actions {
		if action.Kind == kind {
			return true
		}
	}
	return false
}

func hasPackApplyAction(actions []PackBundleApplyAction, kind PackBundleApplyActionKind, packID string) bool {
	for _, action := range actions {
		if action.Kind == kind && action.PackID == packID {
			return true
		}
	}
	return false
}
