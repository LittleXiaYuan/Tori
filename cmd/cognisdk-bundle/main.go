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
	fmt.Println("  cognisdk-bundle diff <current.json> <candidate.json> [--markdown]")
	fmt.Println("  cognisdk-bundle golden <candidate.json> [--markdown]")
	fmt.Println("  cognisdk-bundle review <current.json> <candidate.json> [--markdown]")
	fmt.Println("  cognisdk-bundle promote <current.json> <candidate.json> <output.json> [--allow-review] [--review-out review.json]")
}
