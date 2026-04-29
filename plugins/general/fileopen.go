package general

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"yunque-agent/pkg/skills"
)

// FileOpenSkill opens a file or directory using the OS default application.
type FileOpenSkill struct {
	allowedPaths []string
}

func NewFileOpenSkill(allowedPaths []string) *FileOpenSkill {
	return &FileOpenSkill{allowedPaths: allowedPaths}
}

func (s *FileOpenSkill) Name() string { return "file_open" }
func (s *FileOpenSkill) Description() string {
	return "用系统默认程序打开文件或文件夹。支持打开文档、图片、PDF、文件夹等"
}

func (s *FileOpenSkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "要打开的文件或文件夹路径（绝对路径或相对于输出目录）",
			},
		},
		"required": []string{"path"},
	}
}

func (s *FileOpenSkill) Execute(_ context.Context, args map[string]any, env *skills.Environment) (string, error) {
	path, _ := args["path"].(string)
	if path == "" {
		return "", fmt.Errorf("path is required")
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return "", fmt.Errorf("path does not exist: %s", absPath)
	}

	if !isUnderAllowed(absPath, s.allowedPaths) {
		return "", fmt.Errorf("access denied: %s is not under an allowed directory", absPath)
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", absPath)
	case "darwin":
		cmd = exec.Command("open", absPath)
	default:
		cmd = exec.Command("xdg-open", absPath)
	}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("open failed: %w", err)
	}

	return fmt.Sprintf("已打开 %s", absPath), nil
}
