package cognisdk

import (
	"context"
	"fmt"
	"strings"
)

// GoldenTestResult reports a single pack regression case.
type GoldenTestResult struct {
	Name   string   `json:"name" yaml:"name"`
	Passed bool     `json:"passed" yaml:"passed"`
	Errors []string `json:"errors,omitempty" yaml:"errors,omitempty"`
}

// GoldenTestSummary is a compact CI/automation-friendly bundle gate result.
type GoldenTestSummary struct {
	Passed  int                `json:"passed" yaml:"passed"`
	Failed  int                `json:"failed" yaml:"failed"`
	Results []GoldenTestResult `json:"results" yaml:"results"`
}

// RunPackBundleGoldenTests restores an engine from a portable bundle and runs
// the merged pack golden tests without applying the bundle to any host state.
func RunPackBundleGoldenTests(ctx context.Context, bundle PackBundle) (GoldenTestSummary, error) {
	pm, err := NewPackManagerFromBundle(bundle)
	if err != nil {
		return GoldenTestSummary{}, err
	}
	engine := &Engine{manager: pm}
	merged := pm.Merge()
	results := RunGoldenTests(ctx, engine, merged.GoldenTests)
	return SummarizeGoldenTests(results), nil
}

// SummarizeGoldenTests counts passing and failing golden test results.
func SummarizeGoldenTests(results []GoldenTestResult) GoldenTestSummary {
	summary := GoldenTestSummary{Results: append([]GoldenTestResult(nil), results...)}
	for _, result := range results {
		if result.Passed {
			summary.Passed++
		} else {
			summary.Failed++
		}
	}
	return summary
}

// RenderGoldenTestSummaryMarkdown renders a compact gate report for CI logs,
// plugin review pages, or release notes.
func RenderGoldenTestSummaryMarkdown(summary GoldenTestSummary) string {
	var b strings.Builder
	b.WriteString("## Cogni Pack Golden Tests\n\n")
	fmt.Fprintf(&b, "- passed: %d\n", summary.Passed)
	fmt.Fprintf(&b, "- failed: %d\n", summary.Failed)
	if len(summary.Results) > 0 {
		b.WriteString("\n### Results\n")
		for _, result := range summary.Results {
			status := "PASS"
			if !result.Passed {
				status = "FAIL"
			}
			fmt.Fprintf(&b, "- [%s] %s\n", status, result.Name)
			for _, err := range result.Errors {
				fmt.Fprintf(&b, "  - %s\n", err)
			}
		}
	}
	return strings.TrimSpace(b.String()) + "\n"
}

// RunGoldenTests evaluates the supplied golden cases against the engine.
func RunGoldenTests(ctx context.Context, engine *Engine, tests []GoldenTest) []GoldenTestResult {
	results := make([]GoldenTestResult, 0, len(tests))
	for _, tc := range tests {
		result := GoldenTestResult{Name: tc.Name, Passed: true}
		if engine == nil {
			result.Passed = false
			result.Errors = append(result.Errors, "engine is nil")
			results = append(results, result)
			continue
		}

		input := Input{Message: tc.Input, RequestedToolAction: tc.RequestedToolAction}
		out := engine.Evaluate(ctx, input)

		if tc.ExpectMode != "" && out.Disposition.Mode != tc.ExpectMode {
			result.Passed = false
			result.Errors = append(result.Errors, fmt.Sprintf("mode=%q want %q", out.Disposition.Mode, tc.ExpectMode))
		}
		if tc.ExpectToolPolicy != "" && out.Disposition.ToolPolicy != tc.ExpectToolPolicy {
			result.Passed = false
			result.Errors = append(result.Errors, fmt.Sprintf("tool_policy=%q want %q", out.Disposition.ToolPolicy, tc.ExpectToolPolicy))
		}
		for _, phrase := range tc.MustSayContains {
			if !strings.Contains(strings.Join(out.Disposition.MustSay, "\n"), phrase) {
				result.Passed = false
				result.Errors = append(result.Errors, fmt.Sprintf("must_say missing %q", phrase))
			}
		}
		for _, phrase := range tc.MustAvoidContains {
			if !containsString(out.Disposition.MustAvoid, phrase) {
				result.Passed = false
				result.Errors = append(result.Errors, fmt.Sprintf("must_avoid missing %q", phrase))
			}
		}
		results = append(results, result)
	}
	return results
}
