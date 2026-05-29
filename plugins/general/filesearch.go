package general

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"yunque-agent/internal/execution/sandbox"
	"yunque-agent/pkg/skills"
)

// FileSearchSkill allows the agent to search and read files on the host system (read-only).
type FileSearchSkill struct {
	hostReadPaths []string
}

func NewFileSearchSkill(hostReadPaths []string) *FileSearchSkill {
	return &FileSearchSkill{hostReadPaths: hostReadPaths}
}

func (s *FileSearchSkill) Name() string        { return "file_search" }
func (s *FileSearchSkill) Description() string { return "搜索和读取主机文件系统（只读），支持文件名搜索、内容搜索、目录浏览、文件读取" }
func (s *FileSearchSkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"description": "操作类型: search(文件名搜索), grep(内容搜索), read(读取文件), list(列目录)",
				"enum":        []string{"search", "grep", "read", "list"},
			},
			"path":    map[string]any{"type": "string", "description": "目标路径（目录或文件）"},
			"query":   map[string]any{"type": "string", "description": "搜索关键词"},
		},
		"required": []string{"action", "path"},
	}
}

func (s *FileSearchSkill) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	action, _ := args["action"].(string)
	path, _ := args["path"].(string)
	query, _ := args["query"].(string)

	if path == "" {
		return "", fmt.Errorf("path is required")
	}

	policy := sandbox.DefaultPolicy()
	policy.HostReadPaths = mergeWorkspacePaths(s.hostReadPaths, env)

	sb, err := sandbox.New(os.TempDir(), policy)
	if err != nil {
		return "", fmt.Errorf("sandbox: %w", err)
	}
	defer sb.Cleanup()

	switch action {
	case "search":
		if query == "" {
			return "", fmt.Errorf("query required for search")
		}
		matches, err := sb.SearchHostFiles(path, query)
		if err != nil {
			return "", err
		}
		out, _ := json.Marshal(map[string]any{"matches": matches, "count": len(matches)})
		return string(out), nil

	case "grep":
		if query == "" {
			return "", fmt.Errorf("query required for grep")
		}
		matches, err := sb.GrepHostFile(path, query)
		if err != nil {
			return "", err
		}
		out, _ := json.Marshal(map[string]any{"matches": matches, "count": len(matches)})
		return string(out), nil

	case "read":
		content, err := sb.ReadHostFile(path)
		if err != nil {
			return "", err
		}
		// Limit output for LLM context
		if len(content) > 16000 {
			content = content[:16000] + "\n...[truncated]"
		}
		return content, nil

	case "list":
		entries, err := sb.ListHostDir(path)
		if err != nil {
			return "", err
		}
		return strings.Join(entries, "\n"), nil

	default:
		return "", fmt.Errorf("unknown action: %s (use search/grep/read/list)", action)
	}
}

// mergeWorkspacePaths returns the global read roots plus any per-conversation
// workspace directories the environment carries. Each workspace entry must be
// an absolute path that resolves to an existing directory, so a malformed or
// non-existent value can't silently widen the read surface. This is the
// Cursor-style "opened folder" affordance: the user (via the client) opts a
// directory into the read set for their own session without editing the
// global HOST_READ_PATHS config.
func mergeWorkspacePaths(base []string, env *skills.Environment) []string {
	if env == nil || len(env.WorkspacePaths) == 0 {
		return base
	}
	merged := append([]string{}, base...)
	for _, p := range env.WorkspacePaths {
		p = strings.TrimSpace(p)
		if p == "" || !filepath.IsAbs(p) {
			continue
		}
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			merged = append(merged, p)
		}
	}
	return merged
}
