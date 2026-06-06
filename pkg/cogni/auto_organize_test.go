package cogni

import (
	"context"
	"testing"
)

func TestGenerateKeywordsSplitsSkillNames(t *testing.T) {
	kw := generateKeywords("file", []SkillInfo{
		{Name: "docx_create"},
		{Name: "pdf_export"},
	})
	set := map[string]bool{}
	for _, k := range kw {
		set[k] = true
	}
	for _, want := range []string{"file", "docx", "create", "pdf", "export"} {
		if !set[want] {
			t.Fatalf("expected keyword %q in %v", want, kw)
		}
	}
	// The old behavior mashed underscores into one unmatchable token.
	if set["docxcreate"] || set["pdfexport"] {
		t.Fatalf("keywords should be split tokens, not mashed: %v", kw)
	}
}

// TestAutoOrganizedCogniActivatesOnDomainMessage proves the runtime impact:
// an auto-organized skill cogni now actually activates on a natural-language
// domain message (it previously could not, because its keywords were mashed
// skill names like "docxcreate").
func TestAutoOrganizedCogniActivatesOnDomainMessage(t *testing.T) {
	ao := NewAutoOrganizer(NewRegistry(), nil)
	decl := ao.buildDeclaration(context.Background(), "auto:file", "file", []SkillInfo{
		{Name: "docx_create", Description: "create a docx"},
		{Name: "pdf_export", Description: "export a pdf"},
	})

	acts := NewEvaluator().Evaluate([]*Declaration{decl}, Session{Message: "帮我 create 一个 pdf"})
	if len(acts) == 0 || !acts[0].Activated {
		t.Fatalf("auto-organized cogni should activate on a domain message, got %#v", acts)
	}
}
