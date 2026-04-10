package filegen

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"yunque-agent/pkg/skills"
)

// FileGenSkill writes AI-generated content to files in the output directory.
type FileGenSkill struct {
	outputDir string
}

func NewFileGenSkill(outputDir string) *FileGenSkill {
	return &FileGenSkill{outputDir: outputDir}
}

func (s *FileGenSkill) Name() string { return "file_generate" }
func (s *FileGenSkill) Description() string {
	return "Save generated content (code, markdown, HTML, text, CSV, JSON, etc.) to a file. Use this whenever the user asks you to create, write, or save a document/file."
}
func (s *FileGenSkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"filename": map[string]any{
				"type":        "string",
				"description": "Output filename with extension (e.g. report.md, script.py, data.csv, page.html)",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "The complete content to write to the file",
			},
			"subfolder": map[string]any{
				"type":        "string",
				"description": "Optional subfolder within output directory (e.g. 'reports', 'code', 'docs')",
			},
		},
		"required": []string{"filename", "content"},
	}
}

func (s *FileGenSkill) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	filename, _ := args["filename"].(string)
	content, _ := args["content"].(string)
	subfolder, _ := args["subfolder"].(string)

	if filename == "" {
		return "", fmt.Errorf("filename is required")
	}
	if content == "" {
		return "", fmt.Errorf("content is required")
	}

	filename = sanitizeFilename(filename)

	dir := s.outputDir
	if dir == "" {
		dir = "."
	}
	if subfolder != "" {
		subfolder = sanitizeFilename(subfolder)
		dir = filepath.Join(dir, subfolder)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	path := filepath.Join(dir, filename)

	if _, err := os.Stat(path); err == nil {
		ext := filepath.Ext(filename)
		base := strings.TrimSuffix(filename, ext)
		path = filepath.Join(dir, fmt.Sprintf("%s_%s%s", base, time.Now().Format("150405"), ext))
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	abs, _ := filepath.Abs(path)
	slog.Info("file generated", "path", abs, "size", len(content))

	ext := strings.ToLower(filepath.Ext(filename))
	typeLabel := fileTypeLabel(ext)

	return fmt.Sprintf("文件已保存 (%s)\n路径: %s\n大小: %d bytes", typeLabel, abs, len(content)), nil
}

func sanitizeFilename(name string) string {
	name = strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			return '_'
		}
		return r
	}, name)
	if len(name) > 200 {
		name = name[:200]
	}
	return name
}

func fileTypeLabel(ext string) string {
	switch ext {
	case ".md":
		return "Markdown"
	case ".html", ".htm":
		return "HTML"
	case ".py":
		return "Python"
	case ".go":
		return "Go"
	case ".js":
		return "JavaScript"
	case ".ts":
		return "TypeScript"
	case ".tsx":
		return "TypeScript React"
	case ".jsx":
		return "React"
	case ".css":
		return "CSS"
	case ".json":
		return "JSON"
	case ".csv":
		return "CSV"
	case ".txt":
		return "Text"
	case ".sql":
		return "SQL"
	case ".sh":
		return "Shell"
	case ".yaml", ".yml":
		return "YAML"
	case ".xml":
		return "XML"
	default:
		return "File"
	}
}
