package guardrails

import (
	"context"
	"fmt"
	"strings"
)

// KeywordCategory defines a keyword-based moderation rule.
type KeywordCategory struct {
	Name     string
	Keywords []string
	Block    bool    // true = block, false = warn
	Score    float64 // severity 0-1
}

// KeywordGuard checks input against keyword categories.
// Merged from experimental/moderation for unified safety pipeline.
type KeywordGuard struct {
	categories []KeywordCategory
}

// NewKeywordGuard creates a keyword guard with sensible defaults.
func NewKeywordGuard() *KeywordGuard {
	return &KeywordGuard{
		categories: []KeywordCategory{
			{Name: "hate_speech", Keywords: []string{"hate", "racist", "sexist"}, Block: true, Score: 0.9},
			{Name: "self_harm", Keywords: []string{"suicide", "self-harm", "kill myself"}, Block: true, Score: 0.95},
			{Name: "violence", Keywords: []string{"murder", "assault", "bomb threat"}, Block: true, Score: 0.85},
			{Name: "profanity", Keywords: []string{"fuck", "shit"}, Block: false, Score: 0.3},
		},
	}
}

// AddCategory adds a custom keyword category.
func (g *KeywordGuard) AddCategory(cat KeywordCategory) {
	g.categories = append(g.categories, cat)
}

// NewKeywordGuardFromList creates a keyword guard from a name and keyword list.
// Used by the plugin extension system to register custom safety rules.
func NewKeywordGuardFromList(name string, keywords []string) *KeywordGuard {
	return &KeywordGuard{
		categories: []KeywordCategory{
			{Name: name, Keywords: keywords, Block: true, Score: 0.8},
		},
	}
}

// Name implements Guard.
func (g *KeywordGuard) Name() string { return "keyword_moderation" }

// Check implements Guard.
func (g *KeywordGuard) Check(_ context.Context, input string) CheckResult {
	lower := strings.ToLower(input)
	var warnings []string

	for _, cat := range g.categories {
		for _, kw := range cat.Keywords {
			if strings.Contains(lower, strings.ToLower(kw)) {
				msg := fmt.Sprintf("[%s] matched keyword: %s", cat.Name, kw)
				if cat.Block {
					return CheckResult{Passed: false, Blocked: true, Rule: cat.Name, Warnings: []string{msg}}
				}
				warnings = append(warnings, msg)
			}
		}
	}

	return CheckResult{Passed: true, Warnings: warnings}
}
