package general

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"yunque-agent/pkg/skills"
)

// FileWriteSkill allows the agent to create and write files.
// Writes are restricted to allowed directories (task artifact dirs, sandbox, etc.).
type FileWriteSkill struct {
	allowedDirs []string // directories where writing is permitted
}

func NewFileWriteSkill(allowedDirs []string) *FileWriteSkill {
	return &FileWriteSkill{allowedDirs: allowedDirs}
}

func (s *FileWriteSkill) Name() string { return "file_create" }
func (s *FileWriteSkill) Description() string {
	return "创建文件到指定目录，支持文本和代码文件，可指定写入模式（create/overwrite/append）。用于生成报告、代码文件、配置文件等任务产出物"
}

func (s *FileWriteSkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "文件路径（相对于输出目录或绝对路径）",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "要写入的文件内容",
			},
			"mode": map[string]any{
				"type":        "string",
				"description": "写入模式: create（默认，不覆盖已有文件）、overwrite（覆盖）、append（追加）",
				"enum":        []string{"create", "overwrite", "append"},
			},
		},
		"required": []string{"path", "content"},
	}
}

func (s *FileWriteSkill) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	path, _ := args["path"].(string)
	content, _ := args["content"].(string)
	mode, _ := args["mode"].(string)

	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	if content == "" {
		return "", fmt.Errorf("content is required")
	}
	if mode == "" {
		mode = "create"
	}

	// Resolve absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	// Security: verify the path is under an allowed directory
	if !s.isAllowed(absPath, env) {
		return "", fmt.Errorf("access denied: %s is not under an allowed output directory", absPath)
	}

	// Prevent path traversal
	cleanPath := filepath.Clean(absPath)
	if cleanPath != absPath {
		absPath = cleanPath
	}

	// Create parent directories
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create directory: %w", err)
	}

	switch mode {
	case "create":
		if _, err := os.Stat(absPath); err == nil {
			return "", fmt.Errorf("file already exists: %s (use mode=overwrite to replace)", absPath)
		}
		if err := os.WriteFile(absPath, []byte(content), 0o644); err != nil {
			return "", fmt.Errorf("write: %w", err)
		}

	case "overwrite":
		if err := os.WriteFile(absPath, []byte(content), 0o644); err != nil {
			return "", fmt.Errorf("write: %w", err)
		}

	case "append":
		f, err := os.OpenFile(absPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return "", fmt.Errorf("open for append: %w", err)
		}
		defer f.Close()
		if _, err := f.WriteString(content); err != nil {
			return "", fmt.Errorf("append: %w", err)
		}

	default:
		return "", fmt.Errorf("unknown mode: %s", mode)
	}

	info, _ := os.Stat(absPath)
	size := int64(0)
	if info != nil {
		size = info.Size()
	}

	return fmt.Sprintf("已写入 %s（%d 字节，模式：%s）", absPath, size, mode), nil
}

func (s *FileWriteSkill) isAllowed(absPath string, env *skills.Environment) bool {
	for _, dir := range mergeWorkspacePaths(s.allowedDirs, env) {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		if strings.HasPrefix(absPath, absDir+string(filepath.Separator)) || absPath == absDir {
			return true
		}
	}
	return false
}
