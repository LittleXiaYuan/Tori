package cognisdk

import (
	"context"
	"strings"
	"testing"
)

func TestHostAdapterBuildContext(t *testing.T) {
	adapter := NewHostAdapter(Config{})
	// emotional scope: the "no forever promise" boundary declares Scopes:["emotional"],
	// so it MUST activate here and render into the context block.
	ctx := adapter.BuildContext(context.Background(), "你会永远陪我吗？", "tenant-a", "chat", "emotional")
	if ctx == "" {
		t.Fatal("expected non-empty belief context")
	}
	if !strings.Contains(ctx, "## 内心状态") {
		t.Fatalf("missing markdown heading: %s", ctx)
	}
	if !strings.Contains(ctx, "comfort_with_truth") {
		t.Fatalf("missing disposition mode: %s", ctx)
	}
	if !strings.Contains(ctx, "永远不会离开你") {
		t.Fatalf("missing boundary phrase: %s", ctx)
	}
}

// TestHostAdapterBuildContext_ScopeGate verifies #34 at the host boundary:
// the emotional boundary belief (Scopes:["emotional"]) MUST appear in an
// emotional turn and MUST NOT appear in a technical turn. This is the runtime
// proof that the scope gate is actually wired through BuildContext.
//
// The message embeds the boundary's full Chinese Statement verbatim because
// pkg/belief's activation is keyword-based and Chinese has no word boundaries
// (strings.Fields treats the whole Statement as one token). This isolates scope
// gate as the ONLY variable — same message, same keywords, only scope differs.
// (See engine.go:201 — production will use semantic similarity instead.)
func TestHostAdapterBuildContext_ScopeGate(t *testing.T) {
	adapter := NewHostAdapter(Config{})
	const msg = "你能保证不能虚假承诺永久陪伴吗，顺便帮我看看代码"
	emotional := adapter.BuildContext(context.Background(), msg, "tenant-a", "chat", "emotional")
	technical := adapter.BuildContext(context.Background(), msg, "tenant-a", "chat", "technical")

	if !strings.Contains(emotional, "不能虚假承诺永久陪伴") {
		t.Fatalf("FAIL: emotional boundary should activate in emotional scope:\n%s", emotional)
	}
	if strings.Contains(technical, "不能虚假承诺永久陪伴") {
		t.Fatalf("FAIL: emotional boundary leaked into technical scope — #34 scope gate broken:\n%s", technical)
	}
}

func TestHostAdapterProposeUpdates(t *testing.T) {
	adapter := NewHostAdapter(Config{})
	result := adapter.Evaluate(context.Background(), Input{Message: "你会永远陪我吗？"})

	proposal := adapter.ProposeUpdates(context.Background(), result, AuditFeedback{
		ID:              "host-fb-1",
		Kind:            FeedbackBoundaryViolation,
		Message:         "不能承诺永久陪伴。",
		TargetBeliefIDs: []string{"pc.boundary.no_forever_promise"},
	})

	if proposal.Outcome != FeedbackOutcomeReviewRequired {
		t.Fatalf("outcome = %q, want review_required", proposal.Outcome)
	}
	if len(proposal.Proposals) != 1 {
		t.Fatalf("expected one proposal, got %#v", proposal.Proposals)
	}
}

func TestRenderFeedbackProposalMarkdown(t *testing.T) {
	proposal := BuildFeedbackProposal(Result{}, AuditFeedback{
		ID:       "render-fb-1",
		Kind:     FeedbackPreference,
		Message:  "以后先给可回滚清单。",
		Evidence: []string{"外部脚本记录"},
	})

	markdown := RenderFeedbackProposalMarkdown(proposal)
	for _, want := range []string{"## 反馈提案", "outcome: proposed", "Belief Update Proposals", "add_preference", "以后先给可回滚清单。"} {
		if !strings.Contains(markdown, want) {
			t.Fatalf("rendered markdown missing %q:\n%s", want, markdown)
		}
	}
}

func TestNewHostAdapterFromBundle(t *testing.T) {
	bundle, err := NewPackBundle("host-bundle", []PackManifest{YunqueWorkPack()}, []string{PackYunqueWork})
	if err != nil {
		t.Fatalf("new bundle: %v", err)
	}
	adapter, err := NewHostAdapterFromBundle(bundle)
	if err != nil {
		t.Fatalf("host adapter from bundle: %v", err)
	}
	ctx := adapter.BuildContext(context.Background(), "请帮我修复测试", "tenant-a", "chat", "technical")
	if !strings.Contains(ctx, "deliver_work") {
		t.Fatalf("bundle adapter did not activate work pack: %s", ctx)
	}
}
