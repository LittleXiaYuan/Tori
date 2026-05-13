package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"yunque-agent/pkg/cognisdk"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		printUsage()
		return nil
	}

	markdown := false
	if len(args) > 0 && args[len(args)-1] == "--markdown" {
		markdown = true
		args = args[:len(args)-1]
	}

	switch args[0] {

	case "init":
		if len(args) < 2 || len(args) > 3 {
			return fmt.Errorf("usage: cognisdk-bundle init <output.json> [--builtin]")
		}
		includeBuiltin := len(args) == 3 && args[2] == "--builtin"
		if len(args) == 3 && !includeBuiltin {
			return fmt.Errorf("unknown init option %q", args[2])
		}
		var packs []cognisdk.PackManifest
		var enabled []string
		bundleID := "empty-cogni-pack-bundle"
		if includeBuiltin {
			packs = cognisdk.BuiltinPacks()
			enabled = make([]string, 0, len(packs))
			for _, pack := range packs {
				enabled = append(enabled, pack.ID)
			}
			bundleID = "builtin-cogni-pack-bundle"
		}
		bundle, err := cognisdk.NewPackBundle(bundleID, packs, enabled)
		if err != nil {
			return err
		}
		return cognisdk.SavePackBundle(bundle, args[1])

	case "digest":
		digestOptions, normalizedArgs, err := parseDigestOptions(args)
		if err != nil {
			return err
		}
		args = normalizedArgs
		if len(args) != 2 {
			return fmt.Errorf("usage: cognisdk-bundle digest <bundle.json> [--expect sha256:...] [--out digest-check.json]")
		}
		bundle, err := cognisdk.LoadPackBundle(args[1])
		if err != nil {
			return err
		}
		if digestOptions.Expect != "" {
			check, err := cognisdk.VerifyPackBundleDigest(*bundle, digestOptions.Expect)
			if err != nil {
				return err
			}
			if digestOptions.Out != "" {
				if err := saveJSONFile(check, digestOptions.Out); err != nil {
					return err
				}
			} else if err := printJSON(check); err != nil {
				return err
			}
			if !check.Match {
				return fmt.Errorf("bundle digest mismatch: expected %s, got %s", check.Expected, check.Actual)
			}
			return nil
		}
		digest, err := cognisdk.DigestPackBundle(*bundle)
		if err != nil {
			return err
		}
		if digestOptions.Out != "" {
			return saveTextFile(digest+"\n", digestOptions.Out)
		}
		fmt.Println(digest)
		return nil

	case "inspect":
		if len(args) != 2 {
			return fmt.Errorf("usage: cognisdk-bundle inspect <bundle.json> [--markdown]")
		}
		bundle, err := cognisdk.LoadPackBundle(args[1])
		if err != nil {
			return err
		}
		summary, err := cognisdk.SummarizePackBundle(*bundle)
		if err != nil {
			return err
		}
		if markdown {
			fmt.Print(cognisdk.RenderPackBundleSummaryMarkdown(summary))
			return nil
		}
		return printJSON(summary)
	case "diff":
		if len(args) != 3 {
			return fmt.Errorf("usage: cognisdk-bundle diff <current.json> <candidate.json> [--markdown]")
		}
		current, candidate, err := loadPair(args[1], args[2])
		if err != nil {
			return err
		}
		diff, err := cognisdk.DiffPackBundles(*current, *candidate)
		if err != nil {
			return err
		}
		if markdown {
			fmt.Print(cognisdk.RenderPackBundleDiffMarkdown(diff))
			return nil
		}
		return printJSON(diff)
	case "golden":
		if len(args) != 2 {
			return fmt.Errorf("usage: cognisdk-bundle golden <candidate.json> [--markdown]")
		}
		candidate, err := cognisdk.LoadPackBundle(args[1])
		if err != nil {
			return err
		}
		summary, err := cognisdk.RunPackBundleGoldenTests(context.Background(), *candidate)
		if err != nil {
			return err
		}
		if markdown {
			fmt.Print(cognisdk.RenderGoldenTestSummaryMarkdown(summary))
			return nil
		}
		return printJSON(summary)

	case "action-kinds":
		actionKindsOptions, normalizedArgs, err := parseActionKindsOptions(args)
		if err != nil {
			return err
		}
		args = normalizedArgs
		if len(args) != 1 {
			return fmt.Errorf("usage: cognisdk-bundle action-kinds [--details] [--markdown] [--out action-kinds.json]")
		}
		if actionKindsOptions.Details {
			infos := cognisdk.PackBundleApplyActionKindInfos()
			if actionKindsOptions.Out != "" {
				if markdown {
					return saveTextFile(renderApplyActionKindInfosMarkdown(infos), actionKindsOptions.Out)
				}
				return saveJSONFile(infos, actionKindsOptions.Out)
			}
			if markdown {
				fmt.Print(renderApplyActionKindInfosMarkdown(infos))
				return nil
			}
			return printJSON(infos)
		}
		kinds := cognisdk.PackBundleApplyActionKinds()
		if actionKindsOptions.Out != "" {
			if markdown {
				return saveTextFile(renderApplyActionKindsMarkdown(kinds), actionKindsOptions.Out)
			}
			return saveJSONFile(kinds, actionKindsOptions.Out)
		}
		if markdown {
			fmt.Print(renderApplyActionKindsMarkdown(kinds))
			return nil
		}
		return printJSON(kinds)

	case "actions":
		planOptions, normalizedArgs, err := parseActionsOptions(args)
		if err != nil {
			return err
		}
		args = normalizedArgs
		if len(args) != 3 {
			return fmt.Errorf("usage: cognisdk-bundle actions <current.json> <candidate.json> [--markdown] [--out actions.json] [--kind action_kind] [--fail-on-review] [--fail-on-blocked]")
		}
		current, candidate, err := loadPair(args[1], args[2])
		if err != nil {
			return err
		}
		plan, err := cognisdk.PlanPackBundleApply(context.Background(), *current, *candidate)
		if err != nil {
			return err
		}
		actions := cognisdk.FilterPackBundleApplyActions(plan.Actions, planOptions.Kinds...)
		if planOptions.Out != "" {
			if markdown {
				if err := saveTextFile(renderApplyActionsMarkdown(actions), planOptions.Out); err != nil {
					return err
				}
			} else if err := saveJSONFile(actions, planOptions.Out); err != nil {
				return err
			}
		} else if markdown {
			fmt.Print(renderApplyActionsMarkdown(actions))
		} else if err := printJSON(actions); err != nil {
			return err
		}
		return enforcePlanGate(plan, planOptions)

	case "checklist":
		planOptions, normalizedArgs, err := parseActionsOptions(args)
		if err != nil {
			return err
		}
		args = normalizedArgs
		if len(args) != 3 {
			return fmt.Errorf("usage: cognisdk-bundle checklist <current.json> <candidate.json> [--markdown] [--out checklist.json] [--kind action_kind] [--fail-on-review] [--fail-on-blocked]")
		}
		current, candidate, err := loadPair(args[1], args[2])
		if err != nil {
			return err
		}
		plan, err := cognisdk.PlanPackBundleApply(context.Background(), *current, *candidate)
		if err != nil {
			return err
		}
		checklist := cognisdk.FilterPackBundleApplyChecklistItems(cognisdk.BuildPackBundleApplyChecklist(plan), planOptions.Kinds...)
		if planOptions.Out != "" {
			if markdown {
				if err := saveTextFile(renderApplyChecklistMarkdown(checklist), planOptions.Out); err != nil {
					return err
				}
			} else if err := saveJSONFile(checklist, planOptions.Out); err != nil {
				return err
			}
		} else if markdown {
			fmt.Print(renderApplyChecklistMarkdown(checklist))
		} else if err := printJSON(checklist); err != nil {
			return err
		}
		return enforcePlanGate(plan, planOptions)

	case "plan":
		planOptions, normalizedArgs, err := parsePlanOptions(args)
		if err != nil {
			return err
		}
		args = normalizedArgs
		if len(args) != 3 {
			return fmt.Errorf("usage: cognisdk-bundle plan <current.json> <candidate.json> [--markdown] [--out plan.json] [--fail-on-review] [--fail-on-blocked]")
		}
		current, candidate, err := loadPair(args[1], args[2])
		if err != nil {
			return err
		}
		plan, err := cognisdk.PlanPackBundleApply(context.Background(), *current, *candidate)
		if err != nil {
			return err
		}
		if planOptions.Out != "" {
			if markdown {
				if err := saveTextFile(cognisdk.RenderPackBundleApplyPlanMarkdown(plan), planOptions.Out); err != nil {
					return err
				}
			} else if err := saveJSONFile(plan, planOptions.Out); err != nil {
				return err
			}
		} else if markdown {
			fmt.Print(cognisdk.RenderPackBundleApplyPlanMarkdown(plan))
		} else if err := printJSON(plan); err != nil {
			return err
		}
		return enforcePlanGate(plan, planOptions)

	case "promote":
		allowReview, reviewOut, normalizedArgs, err := parsePromoteOptions(args)
		if err != nil {
			return err
		}
		args = normalizedArgs
		if len(args) != 4 {
			return fmt.Errorf("usage: cognisdk-bundle promote <current.json> <candidate.json> <output.json> [--allow-review] [--review-out review.json]")
		}
		current, candidate, err := loadPair(args[1], args[2])
		if err != nil {
			return err
		}
		review, err := cognisdk.ReviewPackBundleCandidate(context.Background(), *current, *candidate)
		if err != nil {
			return err
		}
		if review.Outcome == cognisdk.PackBundleReviewBlocked {
			return fmt.Errorf("candidate bundle blocked: %s", review.Reason)
		}
		if reviewOut != "" {
			if err := saveJSONFile(review, reviewOut); err != nil {
				return err
			}
		}
		if review.Outcome == cognisdk.PackBundleReviewReview && !allowReview {
			return fmt.Errorf("candidate bundle requires review: %s", review.Reason)
		}
		if err := cognisdk.SavePackBundle(*candidate, args[3]); err != nil {
			return err
		}
		fmt.Printf("promoted %s to %s (outcome=%s)\n", candidate.ID, args[3], review.Outcome)
		return nil
	case "review":
		if len(args) != 3 {
			return fmt.Errorf("usage: cognisdk-bundle review <current.json> <candidate.json> [--markdown]")
		}
		current, candidate, err := loadPair(args[1], args[2])
		if err != nil {
			return err
		}
		review, err := cognisdk.ReviewPackBundleCandidate(context.Background(), *current, *candidate)
		if err != nil {
			return err
		}
		if markdown {
			fmt.Print(cognisdk.RenderPackBundleReviewMarkdown(review))
			return nil
		}
		return printJSON(review)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

type planCLIOptions struct {
	Out           string
	Kinds         []cognisdk.PackBundleApplyActionKind
	FailOnReview  bool
	FailOnBlocked bool
}

type digestCLIOptions struct {
	Expect string
	Out    string
}

type actionKindsCLIOptions struct {
	Out     string
	Details bool
}

func parseActionKindsOptions(args []string) (actionKindsCLIOptions, []string, error) {
	var opts actionKindsCLIOptions
	normalized := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--details":
			opts.Details = true
		case "--out":
			if i+1 >= len(args) {
				return actionKindsCLIOptions{}, nil, fmt.Errorf("--out requires a path")
			}
			opts.Out = args[i+1]
			i++
		default:
			if len(args[i]) > 0 && args[i][0] == '-' {
				return actionKindsCLIOptions{}, nil, fmt.Errorf("unknown action-kinds option %q", args[i])
			}
			normalized = append(normalized, args[i])
		}
	}
	return opts, normalized, nil
}

func parseDigestOptions(args []string) (digestCLIOptions, []string, error) {
	var opts digestCLIOptions
	normalized := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--expect":
			if i+1 >= len(args) {
				return digestCLIOptions{}, nil, fmt.Errorf("--expect requires a digest")
			}
			opts.Expect = args[i+1]
			i++
		case "--out":
			if i+1 >= len(args) {
				return digestCLIOptions{}, nil, fmt.Errorf("--out requires a path")
			}
			opts.Out = args[i+1]
			i++
		default:
			if len(args[i]) > 0 && args[i][0] == '-' {
				return digestCLIOptions{}, nil, fmt.Errorf("unknown digest option %q", args[i])
			}
			normalized = append(normalized, args[i])
		}
	}
	return opts, normalized, nil
}

func parseActionsOptions(args []string) (planCLIOptions, []string, error) {
	var opts planCLIOptions
	normalized := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--out":
			if i+1 >= len(args) {
				return planCLIOptions{}, nil, fmt.Errorf("--out requires a path")
			}
			opts.Out = args[i+1]
			i++
		case "--kind":
			if i+1 >= len(args) {
				return planCLIOptions{}, nil, fmt.Errorf("--kind requires an action kind")
			}
			kind := cognisdk.PackBundleApplyActionKind(args[i+1])
			if !cognisdk.KnownPackBundleApplyActionKind(kind) {
				return planCLIOptions{}, nil, fmt.Errorf("unknown action kind %q", args[i+1])
			}
			opts.Kinds = append(opts.Kinds, kind)
			i++
		case "--fail-on-review":
			opts.FailOnReview = true
		case "--fail-on-blocked":
			opts.FailOnBlocked = true
		default:
			normalized = append(normalized, args[i])
		}
	}
	return opts, normalized, nil
}

func parsePlanOptions(args []string) (planCLIOptions, []string, error) {
	var opts planCLIOptions
	normalized := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--out":
			if i+1 >= len(args) {
				return planCLIOptions{}, nil, fmt.Errorf("--out requires a path")
			}
			opts.Out = args[i+1]
			i++
		case "--fail-on-review":
			opts.FailOnReview = true
		case "--fail-on-blocked":
			opts.FailOnBlocked = true
		default:
			normalized = append(normalized, args[i])
		}
	}
	return opts, normalized, nil
}

func enforcePlanGate(plan cognisdk.PackBundleApplyPlan, opts planCLIOptions) error {
	if opts.FailOnBlocked && plan.Blocked {
		return fmt.Errorf("candidate bundle blocked: %s", plan.Reason)
	}
	if opts.FailOnReview && plan.RequiresReview {
		return fmt.Errorf("candidate bundle requires review: %s", plan.Reason)
	}
	return nil
}

func parsePromoteOptions(args []string) (bool, string, []string, error) {
	allowReview := false
	reviewOut := ""
	normalized := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--allow-review":
			allowReview = true
		case "--review-out":
			if i+1 >= len(args) {
				return false, "", nil, fmt.Errorf("--review-out requires a path")
			}
			reviewOut = args[i+1]
			i++
		default:
			normalized = append(normalized, args[i])
		}
	}
	return allowReview, reviewOut, normalized, nil
}

func renderApplyActionKindsMarkdown(kinds []cognisdk.PackBundleApplyActionKind) string {
	var out string
	out += "## Cogni Pack Bundle Apply Action Kinds\n\n"
	if len(kinds) == 0 {
		out += "No action kinds.\n"
		return out
	}
	for _, kind := range kinds {
		out += fmt.Sprintf("- `%s`\n", kind)
	}
	return out
}

func renderApplyActionKindInfosMarkdown(infos []cognisdk.PackBundleApplyActionKindInfo) string {
	var out string
	out += "## Cogni Pack Bundle Apply Action Kinds\n\n"
	if len(infos) == 0 {
		out += "No action kinds.\n"
		return out
	}
	for _, info := range infos {
		out += fmt.Sprintf("- `%s` — %s: %s\n", info.Kind, info.Label, info.Description)
	}
	return out
}

func renderApplyActionsMarkdown(actions []cognisdk.PackBundleApplyAction) string {
	var out string
	out += "## Cogni Pack Bundle Apply Actions\n\n"
	if len(actions) == 0 {
		out += "No actions.\n"
		return out
	}
	for _, action := range actions {
		out += fmt.Sprintf("- `%s`", action.Kind)
		if action.PackID != "" {
			out += fmt.Sprintf(" pack=%s", action.PackID)
		}
		if action.BundleID != "" {
			out += fmt.Sprintf(" bundle=%s", action.BundleID)
		}
		if action.Digest != "" {
			out += fmt.Sprintf(" digest=%s", action.Digest)
		}
		if action.FromVersion != "" || action.ToVersion != "" {
			out += fmt.Sprintf(" version=%s->%s", emptyCLI(action.FromVersion), emptyCLI(action.ToVersion))
		}
		if action.Message != "" {
			out += fmt.Sprintf(": %s", action.Message)
		}
		out += "\n"
	}
	return out
}

func renderApplyChecklistMarkdown(items []cognisdk.PackBundleApplyChecklistItem) string {
	var out string
	out += "## Cogni Pack Bundle Apply Checklist\n\n"
	if len(items) == 0 {
		out += "No checklist items.\n"
		return out
	}
	for _, item := range items {
		mark := "[ ]"
		if item.Done {
			mark = "[x]"
		}
		out += fmt.Sprintf("- %s `%s` — %s", mark, item.Kind, item.Label)
		if item.Required {
			out += " required"
		}
		if item.Blocked {
			out += " blocked"
		}
		if item.Message != "" {
			out += fmt.Sprintf(": %s", item.Message)
		}
		out += "\n"
	}
	return out
}

func emptyCLI(value string) string {
	if value == "" {
		return "unknown"
	}
	return value
}

func saveTextFile(value string, path string) error {
	if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
		return fmt.Errorf("write %q: %w", path, err)
	}
	return nil
}

func saveJSONFile(value any, path string) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %q: %w", path, err)
	}
	return nil
}

func loadPair(currentPath, candidatePath string) (*cognisdk.PackBundle, *cognisdk.PackBundle, error) {
	current, err := cognisdk.LoadPackBundle(currentPath)
	if err != nil {
		return nil, nil, err
	}
	candidate, err := cognisdk.LoadPackBundle(candidatePath)
	if err != nil {
		return nil, nil, err
	}
	return current, candidate, nil
}

func printJSON(value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  cognisdk-bundle init <output.json> [--builtin]")
	fmt.Println("  cognisdk-bundle action-kinds [--details] [--markdown] [--out action-kinds.json]")
	fmt.Println("  cognisdk-bundle actions <current.json> <candidate.json> [--markdown] [--out actions.json] [--kind action_kind] [--fail-on-review] [--fail-on-blocked]")
	fmt.Println("  cognisdk-bundle checklist <current.json> <candidate.json> [--markdown] [--out checklist.json] [--kind action_kind] [--fail-on-review] [--fail-on-blocked]")
	fmt.Println("  cognisdk-bundle digest <bundle.json> [--expect sha256:...] [--out digest-check.json]")
	fmt.Println("  cognisdk-bundle inspect <bundle.json> [--markdown]")
	fmt.Println("  cognisdk-bundle diff <current.json> <candidate.json> [--markdown]")
	fmt.Println("  cognisdk-bundle golden <candidate.json> [--markdown]")
	fmt.Println("  cognisdk-bundle plan <current.json> <candidate.json> [--markdown] [--out plan.json] [--fail-on-review] [--fail-on-blocked]")
	fmt.Println("  cognisdk-bundle review <current.json> <candidate.json> [--markdown]")
	fmt.Println("  cognisdk-bundle promote <current.json> <candidate.json> <output.json> [--allow-review] [--review-out review.json]")
}
