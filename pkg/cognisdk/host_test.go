package cognisdk

import (
	"context"
	"strings"
	"testing"
)

func TestHostAdapterBuildContext(t *testing.T) {
	adapter := NewHostAdapter(Config{})
	ctx := adapter.BuildContext(context.Background(), "你会永远陪我吗？", "tenant-a", "chat")
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
	ctx := adapter.BuildContext(context.Background(), "请帮我修复测试", "tenant-a", "chat")
	if !strings.Contains(ctx, "deliver_work") {
		t.Fatalf("bundle adapter did not activate work pack: %s", ctx)
	}
}
