package knowledge

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"yunque-agent/internal/integrations/mineru"
)

// DocumentParser is the narrow document parsing capability used by knowledge
// upload ingestion. *mineru.Client satisfies it; tests can provide a stub.
type DocumentParser interface {
	Enabled() bool
	ParseFile(ctx context.Context, filePath string) (*mineru.ParseResult, error)
}

// ParsePayload is the normalized parsed-document payload returned by MinerU.
type ParsePayload struct {
	Result   *mineru.ParseResult
	Markdown string
	Parse    map[string]any
}

// UploadResult is the domain result for a knowledge upload ingestion.
type UploadResult struct {
	Source *Source
	Parse  map[string]any
}

// IsMinerUSupportedExt reports whether a file extension can be parsed by the
// configured MinerU document parser.
func IsMinerUSupportedExt(ext string) bool {
	switch strings.ToLower(strings.TrimSpace(ext)) {
	case ".pdf", ".doc", ".docx", ".ppt", ".pptx", ".xls", ".xlsx", ".png", ".jpg", ".jpeg", ".webp", ".bmp", ".tif", ".tiff":
		return true
	default:
		return false
	}
}

// ParseFileWithMinerU parses uploaded bytes through the configured document
// parser. It writes a temporary file because MinerU consumes file paths.
func ParseFileWithMinerU(ctx context.Context, parser DocumentParser, filename string, data []byte) (*ParsePayload, error) {
	if parser == nil || !parser.Enabled() {
		return nil, fmt.Errorf("MinerU is not enabled")
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("uploaded file is empty")
	}

	ext := strings.ToLower(filepath.Ext(filename))
	tmpFile, err := os.CreateTemp("", "yunque-mineru-*"+ext)
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)
	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return nil, fmt.Errorf("write temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return nil, fmt.Errorf("close temp file: %w", err)
	}

	result, err := parser.ParseFile(ctx, tmpPath)
	if err != nil {
		return nil, err
	}
	markdown := strings.TrimSpace(result.Markdown)
	if markdown == "" {
		return nil, fmt.Errorf("MinerU did not return markdown content")
	}
	preview := TruncateMarkdownPreview(markdown, 1200)
	return &ParsePayload{
		Result:   result,
		Markdown: markdown,
		Parse: map[string]any{
			"parser":          "mineru",
			"backend":         result.Backend,
			"markdown_chars":  len([]rune(markdown)),
			"has_layout_json": strings.TrimSpace(result.JSON) != "",
			"preview":         preview,
		},
	}, nil
}

// TruncateMarkdownPreview returns a UTF-8-safe markdown preview.
func TruncateMarkdownPreview(markdown string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(markdown)
	if len(runes) <= limit {
		return markdown
	}
	return string(runes[:limit]) + "..."
}

// IngestWithMinerU parses the uploaded document and stores its markdown in the
// knowledge store as a file source.
func IngestWithMinerU(ctx context.Context, store *Store, parser DocumentParser, filename string, data []byte) (*UploadResult, error) {
	if store == nil {
		return nil, fmt.Errorf("knowledge base not configured")
	}
	parsed, err := ParseFileWithMinerU(ctx, parser, filename, data)
	if err != nil {
		return nil, err
	}
	name := filename
	ext := strings.ToLower(filepath.Ext(filename))
	if ext != "" {
		name = strings.TrimSuffix(filename, ext) + ".md"
	}
	src, err := store.IngestText(name, parsed.Markdown)
	if err != nil {
		return nil, err
	}
	if src != nil {
		src.Type = SourceFile
		src.Path = filename
	}

	return &UploadResult{Source: src, Parse: parsed.Parse}, nil
}

// IngestUpload ingests raw uploaded bytes into the knowledge store. Plain text
// and markdown are stored directly; parser-backed document formats use MinerU.
func IngestUpload(ctx context.Context, store *Store, parser DocumentParser, filename string, data []byte) (*UploadResult, error) {
	if store == nil {
		return nil, fmt.Errorf("knowledge base not configured")
	}
	ext := strings.ToLower(filepath.Ext(filename))
	switch {
	case ext == ".txt" || ext == ".md":
		src, err := store.IngestText(filename, string(data))
		if err != nil {
			return nil, err
		}
		return &UploadResult{Source: src}, nil
	case IsMinerUSupportedExt(ext):
		if parser == nil || !parser.Enabled() {
			return nil, fmt.Errorf("unsupported format: %s (enable MinerU to parse this file type)", ext)
		}
		return IngestWithMinerU(ctx, store, parser, filename, data)
	default:
		return nil, fmt.Errorf("unsupported format: %s (use .txt, .md or enable MinerU for PDF/Office/image files)", ext)
	}
}
