package docparse

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"yunque-agent/internal/agentcore/knowledge"
	"yunque-agent/internal/integrations/mineru"
	"yunque-agent/pkg/skills"
)

type DocumentParseSkill struct {
	client    *mineru.Client
	outputDir string
	knowledge *knowledge.Store
}

func NewDocumentParseSkill(client *mineru.Client, outputDir string, knowledgeStore *knowledge.Store) *DocumentParseSkill {
	return &DocumentParseSkill{
		client:    client,
		outputDir: outputDir,
		knowledge: knowledgeStore,
	}
}

func (s *DocumentParseSkill) Name() string { return "document_parse" }

func (s *DocumentParseSkill) Description() string {
	return "Parse PDF, DOCX, PPTX, images, and other complex documents with MinerU. Returns markdown/json output and can optionally ingest the parsed markdown into the knowledge store."
}

func (s *DocumentParseSkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"file_path": map[string]any{
				"type":        "string",
				"description": "Absolute or relative path to the source document (PDF, DOCX, PPTX, image, etc.)",
			},
			"save_as": map[string]any{
				"type":        "string",
				"description": "Optional output markdown filename, e.g. report.md",
			},
			"ingest_to_knowledge": map[string]any{
				"type":        "boolean",
				"description": "Whether to ingest the parsed markdown into the knowledge store",
			},
		},
		"required": []string{"file_path"},
	}
}

func (s *DocumentParseSkill) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	if s.client == nil || !s.client.Enabled() {
		return "", fmt.Errorf("MinerU is not enabled. Set MINERU_ENABLED=true and configure MINERU_COMMAND first")
	}

	filePath, _ := args["file_path"].(string)
	if strings.TrimSpace(filePath) == "" {
		return "", fmt.Errorf("file_path is required")
	}

	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return "", fmt.Errorf("resolve file path: %w", err)
	}
	if _, err := os.Stat(absPath); err != nil {
		return "", fmt.Errorf("source document not found: %w", err)
	}

	result, err := s.client.ParseFile(ctx, absPath)
	if err != nil {
		return "", err
	}

	saveName, _ := args["save_as"].(string)
	if strings.TrimSpace(saveName) == "" {
		base := strings.TrimSuffix(filepath.Base(absPath), filepath.Ext(absPath))
		saveName = sanitizeName(base) + ".md"
	}
	outputPath, err := s.writeMarkdown(saveName, result.Markdown)
	if err != nil {
		return "", err
	}

	var jsonPath string
	if strings.TrimSpace(result.JSON) != "" {
		jsonName := strings.TrimSuffix(filepath.Base(outputPath), filepath.Ext(outputPath)) + ".json"
		jsonPath, err = s.writeJSON(jsonName, result.JSON)
		if err != nil {
			return "", err
		}
	}

	ingestToKnowledge, _ := args["ingest_to_knowledge"].(bool)
	var ingested string
	if ingestToKnowledge && s.knowledge != nil && strings.TrimSpace(result.Markdown) != "" {
		src, err := s.knowledge.IngestText(filepath.Base(outputPath), result.Markdown)
		if err != nil {
			return "", fmt.Errorf("parsed, but knowledge ingest failed: %w", err)
		}
		ingested = src.ID
	}

	var lines []string
	lines = append(lines, "Document parsed with MinerU.")
	lines = append(lines, "Markdown: "+outputPath)
	if jsonPath != "" {
		lines = append(lines, "JSON: "+jsonPath)
	}
	lines = append(lines, "Workspace: "+result.WorkDir)
	if ingested != "" {
		lines = append(lines, "Knowledge source: "+ingested)
	}
	if preview := previewText(result.Markdown); preview != "" {
		lines = append(lines, "")
		lines = append(lines, preview)
	}
	return strings.Join(lines, "\n"), nil
}

func (s *DocumentParseSkill) writeMarkdown(name, content string) (string, error) {
	if strings.TrimSpace(content) == "" {
		content = "_MinerU returned no markdown content._"
	}
	return s.writeFile(name, content)
}

func (s *DocumentParseSkill) writeJSON(name, content string) (string, error) {
	return s.writeFile(name, content)
}

func (s *DocumentParseSkill) writeFile(name, content string) (string, error) {
	dir := s.outputDir
	if strings.TrimSpace(dir) == "" {
		dir = "."
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create output dir: %w", err)
	}
	name = sanitizeName(name)
	path := filepath.Join(dir, name)
	if _, err := os.Stat(path); err == nil {
		ext := filepath.Ext(path)
		base := strings.TrimSuffix(filepath.Base(path), ext)
		path = filepath.Join(dir, fmt.Sprintf("%s_%s%s", base, time.Now().Format("150405"), ext))
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("write output file: %w", err)
	}
	return filepath.Abs(path)
}

func sanitizeName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "mineru_output.md"
	}
	return strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			return '_'
		}
		return r
	}, name)
}

func previewText(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	content = strings.ReplaceAll(content, "\r\n", "\n")
	if len(content) > 1200 {
		return content[:1200] + "\n..."
	}
	return content
}
