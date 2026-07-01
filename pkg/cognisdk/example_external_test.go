package cognisdk_test

import (
	"context"
	"fmt"
	"strings"

	"yunque-agent/pkg/cognisdk"
)

func ExampleHostAdapter_BuildContext() {
	adapter := cognisdk.NewHostAdapter(cognisdk.Config{})
	// technical scope: work pack's "deliver_work" value activates for the work task.
	ctx := adapter.BuildContext(context.Background(), "请先帮我修复测试", "tenant-a", "chat", "technical")

	fmt.Println(strings.Contains(ctx, "deliver_work"))
	// Output: true
}

func ExampleEngine_ProposeUpdates() {
	engine := cognisdk.NewEngine(cognisdk.Config{})
	result := engine.Evaluate(context.Background(), cognisdk.Input{Message: "你会永远陪我吗？"})
	proposal := engine.ProposeUpdates(context.Background(), result, cognisdk.AuditFeedback{
		Kind:    cognisdk.FeedbackBoundaryViolation,
		Message: "不能承诺永久陪伴。",
	})

	fmt.Println(proposal.Outcome)
	// Output: review_required
}

func ExamplePackManager_ExportBundle() {
	manager := cognisdk.NewPackManager(cognisdk.BuiltinPacks()...)
	_ = manager.Disable(cognisdk.PackPersonalCompanion)
	bundle, _ := manager.ExportBundle("automation-cogni-packs")
	restored, _ := cognisdk.NewHostAdapterFromBundle(bundle)
	result := restored.Evaluate(context.Background(), cognisdk.Input{
		Message: "请帮我删除这些文件",
		RequestedToolAction: &cognisdk.ToolAction{
			Name: "remove_workspace_files",
			Kind: "delete",
			Risk: cognisdk.RiskHigh,
		},
	})

	fmt.Println(bundle.ID)
	fmt.Println(bundle.EnabledPacks[0])
	fmt.Println(result.Disposition.ToolPolicy)
	// Output:
	// automation-cogni-packs
	// yunque-work-pack
	// require_confirmation
}

func ExampleRenderFeedbackProposalMarkdown() {
	proposal := cognisdk.BuildFeedbackProposal(cognisdk.Result{}, cognisdk.AuditFeedback{
		Kind:    cognisdk.FeedbackPreference,
		Message: "以后发布前先给我可回滚清单。",
	})
	markdown := cognisdk.RenderFeedbackProposalMarkdown(proposal)

	fmt.Println(strings.Contains(markdown, "add_preference"))
	// Output: true
}

func ExamplePlanPackBundleApply_actions() {
	current, _ := cognisdk.NewPackBundle("current", []cognisdk.PackManifest{cognisdk.PersonalCompanionPack()}, []string{cognisdk.PackPersonalCompanion})
	candidate, _ := cognisdk.NewPackBundle("candidate", cognisdk.BuiltinPacks(), []string{cognisdk.PackPersonalCompanion, cognisdk.PackYunqueWork})
	plan, _ := cognisdk.PlanPackBundleApply(context.Background(), current, candidate)

	for _, action := range plan.Actions {
		if action.Kind == cognisdk.PackBundleApplyActionAddPack {
			fmt.Println(action.Kind)
			fmt.Println(action.PackID)
		}
	}
	// Output:
	// add_pack
	// yunque-work-pack
}
