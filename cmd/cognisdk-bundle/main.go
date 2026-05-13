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
	fmt.Println("  cognisdk-bundle diff <current.json> <candidate.json> [--markdown]")
	fmt.Println("  cognisdk-bundle golden <candidate.json> [--markdown]")
	fmt.Println("  cognisdk-bundle review <current.json> <candidate.json> [--markdown]")
}
