package cognisdk

import (
	"context"
	"fmt"
	"strings"
)

// GoldenTestResult reports a single pack regression case.
type GoldenTestResult struct {
	Name   string
	Passed bool
	Errors []string
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
