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

func (g *Gateway) handleKBImportURL(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.knowledgeStore == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "knowledge base not configured"})
		return
	}
	var req struct {
		URL           string `json:"url"`
		Name          string `json:"name"`
		CrawlChildren bool   `json:"crawl_children"`
		MaxPages      int    `json:"max_pages"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.URL) == "" {
		json.NewEncoder(w).Encode(map[string]string{"error": "url required"})
		return
	}

	if req.MaxPages <= 0 {
		req.MaxPages = 5
	}
	if req.MaxPages > 20 {
		req.MaxPages = 20
	}

	page, err := fetchKnowledgeURLPage(strings.TrimSpace(req.URL), strings.TrimSpace(req.Name))
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	imported := make([]*knowledge.Source, 0, req.MaxPages)
	src, err := g.knowledgeStore.IngestURL(page.Name, page.URL, page.Content)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	imported = append(imported, src)

	if req.CrawlChildren {
		for _, childURL := range extractDeepWikiChildLinks(page.URL, page.RawHTML, req.MaxPages-1) {
			childPage, childErr := fetchKnowledgeURLPage(childURL, "")
			if childErr != nil {
				slog.Warn("knowledge: import child url failed", "url", childURL, "err", childErr)
				continue
			}
			childSrc, childErr := g.knowledgeStore.IngestURL(childPage.Name, childPage.URL, childPage.Content)
			if childErr != nil {
				slog.Warn("knowledge: ingest child url failed", "url", childURL, "err", childErr)
				continue
			}
			imported = append(imported, childSrc)
		}
	}

	safego.Go("knowledge-reindex", func() {
		if err := g.knowledgeStore.BuildIndex(context.Background()); err != nil {
			slog.Warn("knowledge: reindex after import-url failed", "err", err)
		}
	})

	json.NewEncoder(w).Encode(map[string]any{"source": src, "sources": imported, "imported": len(imported), "tree": buildKnowledgeImportTree(page, imported), "stats": g.knowledgeStore.Stats()})
}

// handleKBImportRepo handles importing a local repository or code directory.
//
// SECURITY: to prevent arbitrary local-file exfiltration via an authenticated
// user trivially setting `path` to `/etc` or `C:\Users`, the resolved path
// must be rooted under one of:
//   - the configured output dir (`g.outputDir`)
//   - any directory listed in the `KB_IMPORT_ROOTS` env (`;` or `:` separated)
//
// Operators who need wider access can opt in with `KB_IMPORT_ALLOW_ANY=true`,
// which restores the legacy behaviour and is logged loudly on every request.
func (g *Gateway) handleKBImportRepo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.knowledgeStore == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "knowledge base not configured"})
		return
	}
	var req struct {
		Path     string `json:"path"`
		MaxFiles int    `json:"max_files"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Path) == "" {
		json.NewEncoder(w).Encode(map[string]string{"error": "path required"})
		return
	}
	userPath := strings.TrimSpace(req.Path)
	resolvedPath, err := resolveKBRepoPath(g.outputDir, userPath)
	if err != nil {
		slog.Warn("knowledge: import-repo rejected",
			"tenant", tenantFromCtx(r.Context()),
			"path", userPath,
			"err", err)
		writeJSONStatus(w, http.StatusForbidden, map[string]string{"error": err.Error()})
		return
	}
	src, err := g.knowledgeStore.IngestDirectory(resolvedPath, req.MaxFiles)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	safego.Go("knowledge-reindex", func() {
		if err := g.knowledgeStore.BuildIndex(context.Background()); err != nil {
			slog.Warn("knowledge: reindex after import-repo failed", "err", err)
		}
	})
	json.NewEncoder(w).Encode(map[string]any{"source": src, "stats": g.knowledgeStore.Stats()})
}

func resolveKBRepoPath(outputDir, userPath string) (string, error) {
	if strings.EqualFold(os.Getenv("KB_IMPORT_ALLOW_ANY"), "true") {
		slog.Warn("knowledge: KB_IMPORT_ALLOW_ANY=true — arbitrary directory import enabled", "path", userPath)
		abs, err := filepath.Abs(userPath)
		if err != nil {
			return "", fmt.Errorf("invalid path: %w", err)
		}
		return abs, nil
	}

	abs, err := filepath.Abs(userPath)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}
	realCandidate, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("path not accessible")
	}

	roots := collectKBImportRoots(outputDir)
	if len(roots) == 0 {
		return "", fmt.Errorf("no KB import roots configured; set KB_IMPORT_ROOTS or configure outputDir")
	}
	for _, root := range roots {
		realRoot, err := filepath.EvalSymlinks(root)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(realRoot, realCandidate)
		if err != nil {
			continue
		}
		if rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
			return realCandidate, nil
		}
	}
	return "", fmt.Errorf("path must be inside the configured KB import roots")
}

func collectKBImportRoots(outputDir string) []string {
	var out []string
	add := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" {
			return
		}
		info, err := os.Stat(p)
		if err != nil || !info.IsDir() {
			return
		}
		out = append(out, p)
	}
	add(outputDir)
	raw := os.Getenv("KB_IMPORT_ROOTS")
	if raw != "" {
		for _, part := range strings.FieldsFunc(raw, func(r rune) bool { return r == ';' || r == ':' }) {
			add(part)
		}
	}
	return out
}
