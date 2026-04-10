package general

import (
	"fmt"
	"log/slog"
	"os"
)

// DocxVerifier previously rendered DOCX previews via headless Chrome.
// The headless browser engine has been removed in favor of the browser extension.
// This stub preserves the interface for backward compatibility.
type DocxVerifier struct {
	outputDir string
}

func NewDocxVerifier(outputDir string) *DocxVerifier {
	return &DocxVerifier{outputDir: outputDir}
}

// RenderPreview is a no-op stub — headless browser engine was removed.
func (v *DocxVerifier) RenderPreview(_ interface{}, _ interface{}, _ string) (string, error) {
	slog.Warn("docx_verify: preview not available (headless browser removed)")
	return "", fmt.Errorf("docx_verify: headless browser removed; install Yunque Browser Connector extension for preview")
}

// StructuralCheck performs fast Go-side validation on the payload without
// needing a browser or LLM. Returns a list of warnings (empty = OK).
func StructuralCheck(p docxPayload) []string {
	var warnings []string

	if len(p.Blocks) == 0 {
		warnings = append(warnings, "document has no content blocks")
		return warnings
	}

	hasTitle := false
	hasBody := false
	emptyParagraphs := 0
	for _, blk := range p.Blocks {
		switch blk.Type {
		case "paragraph":
			if blk.Style == "Title" {
				hasTitle = true
			}
			if blk.Text == "" {
				emptyParagraphs++
			} else {
				hasBody = true
			}
		case "table":
			if len(blk.Rows) == 0 {
				warnings = append(warnings, "empty table block")
			} else {
				expectedCols := len(blk.Rows[0])
				for i, row := range blk.Rows[1:] {
					if len(row) != expectedCols {
						warnings = append(warnings, fmt.Sprintf("table row %d has %d cols, expected %d", i+2, len(row), expectedCols))
					}
				}
			}
		case "image":
			if blk.Path == "" {
				warnings = append(warnings, "image block with empty path")
			} else if _, err := os.Stat(blk.Path); os.IsNotExist(err) {
				warnings = append(warnings, fmt.Sprintf("image file not found: %s", blk.Path))
			}
		}
	}

	if !hasTitle {
		warnings = append(warnings, "document has no title")
	}
	if !hasBody {
		warnings = append(warnings, "document has no body text")
	}
	if emptyParagraphs > 3 {
		warnings = append(warnings, fmt.Sprintf("excessive empty paragraphs: %d", emptyParagraphs))
	}

	return warnings
}
