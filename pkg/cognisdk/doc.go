// Package cognisdk exposes the experimental local cognition layer for Yunque
// hosts, plugins, frontends, CI jobs, and automation scripts.
//
// The package is intentionally small and data-oriented. It evaluates
// PerceptionState, InnerState, ResponseDisposition, declarative Cogni Pack
// manifests, feedback proposals, bundle diffs, bundle review gates, and JSON
// schemas. It does not provide an LLM provider, planner execution loop, tool
// runner, UI, database, memory store, or remote pack marketplace.
//
// The usual host flow is:
//
//	engine := cognisdk.NewEngine(cognisdk.Config{})
//	result := engine.Evaluate(ctx, cognisdk.Input{Message: "请先修复测试"})
//	contextBlock := cognisdk.RenderMarkdown(result)
//
// Post-turn feedback can be converted into non-mutating proposals:
//
//	proposal := engine.ProposeUpdates(ctx, result, cognisdk.AuditFeedback{
//		Kind:    cognisdk.FeedbackPreference,
//		Message: "以后发布前先给我可回滚清单。",
//	})
//
// Declarative packs can be exchanged as portable bundles and reviewed before a
// caller chooses to load them:
//
//	review, err := cognisdk.ReviewPackBundleCandidate(ctx, current, candidate)
//	if err != nil {
//		return err
//	}
//	if review.Outcome == cognisdk.PackBundleReviewReady {
//		adapter, err := cognisdk.NewHostAdapterFromBundle(candidate)
//		_ = adapter
//		return err
//	}
//
// Loading a bundle validates declarative manifests and restores enabled pack
// state; it never executes arbitrary code. Applying review decisions, writing
// Memory/Ledger state, and preserving rollback snapshots remain host concerns.
package cognisdk
