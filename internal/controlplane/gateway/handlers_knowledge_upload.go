package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"yunque-agent/internal/agentcore/knowledge"
	"yunque-agent/internal/integrations/mineru"
	"yunque-agent/pkg/safego"
)

func (g *Gateway) handleKBUpload(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.knowledgeStore == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "knowledge base not configured"})
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(405)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	file, header, err := r.FormFile("file")
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "file field required (max 10MB)"})
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "read file failed"})
		return
	}

	ext := strings.ToLower(filepath.Ext(header.Filename))
	var (
		src       any
		parseMeta map[string]any
	)
	switch {
	case ext == ".txt" || ext == ".md":
		src, err = g.knowledgeStore.IngestText(header.Filename, string(data))
	case isMinerUSupportedExt(ext):
		if g.documentParser == nil || !g.documentParser.Enabled() {
			json.NewEncoder(w).Encode(map[string]string{"error": "unsupported format: " + ext + " (enable MinerU to parse this file type)"})
			return
		}
		parseResult, parseErr := g.ingestKnowledgeWithMinerU(r.Context(), header.Filename, data)
		if parseErr != nil {
			json.NewEncoder(w).Encode(map[string]string{"error": parseErr.Error()})
			return
		}
		src = parseResult.Source
		parseMeta = parseResult.Response()
	default:
		json.NewEncoder(w).Encode(map[string]string{"error": "unsupported format: " + ext + " (use .txt, .md or enable MinerU for PDF/Office/image files)"})
		return
	}
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	safego.Go("knowledge-reindex", func() {
		if err := g.knowledgeStore.BuildIndex(context.Background()); err != nil {
			slog.Warn("knowledge: reindex after upload failed", "err", err)
		}
	})

	resp := map[string]any{"source": src, "stats": g.knowledgeStore.Stats()}
	if parseMeta != nil {
		resp["parse"] = parseMeta
	}
	json.NewEncoder(w).Encode(resp)
}

type mineruParsePayload struct {
	Result   *mineru.ParseResult
	Markdown string
	Parse    map[string]any
}

type mineruKnowledgeIngestResult struct {
	Source *knowledge.Source
	Parse  map[string]any
}

func (r *mineruKnowledgeIngestResult) Response() map[string]any {
	if r == nil {
		return nil
	}
	return r.Parse
}

func (g *Gateway) parseFileWithMinerU(ctx context.Context, filename string, data []byte) (*mineruParsePayload, error) {
	if g.documentParser == nil || !g.documentParser.Enabled() {
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

	result, err := g.documentParser.ParseFile(ctx, tmpPath)
	if err != nil {
		return nil, err
	}
	markdown := strings.TrimSpace(result.Markdown)
	if markdown == "" {
		return nil, fmt.Errorf("MinerU did not return markdown content")
	}
	preview := truncateMarkdownPreview(markdown, 1200)
	return &mineruParsePayload{
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

func truncateMarkdownPreview(markdown string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(markdown)
	if len(runes) <= limit {
		return markdown
	}
	return string(runes[:limit]) + "..."
}

func (g *Gateway) ingestKnowledgeWithMinerU(ctx context.Context, filename string, data []byte) (*mineruKnowledgeIngestResult, error) {
	parsed, err := g.parseFileWithMinerU(ctx, filename, data)
	if err != nil {
		return nil, err
	}
	name := filename
	ext := strings.ToLower(filepath.Ext(filename))
	if ext != "" {
		name = strings.TrimSuffix(filename, ext) + ".md"
	}
	src, err := g.knowledgeStore.IngestText(name, parsed.Markdown)
	if err != nil {
		return nil, err
	}
	if src != nil {
		src.Type = knowledge.SourceFile
		src.Path = filename
	}

	return &mineruKnowledgeIngestResult{Source: src, Parse: parsed.Parse}, nil
}

func isMinerUSupportedExt(ext string) bool {
	switch strings.ToLower(strings.TrimSpace(ext)) {
	case ".pdf", ".doc", ".docx", ".ppt", ".pptx", ".xls", ".xlsx", ".png", ".jpg", ".jpeg", ".webp", ".bmp", ".tif", ".tiff":
		return true
	default:
		return false
	}
}
