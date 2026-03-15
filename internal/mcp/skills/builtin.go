package skills

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"yunque-agent/internal/mcp"
)

// Builtin provides built-in MCP tools: web_fetch, code_exec, file_read, file_write, file_list.
type Builtin struct {
	workDir string
	client  *http.Client
}

// NewBuiltin creates a built-in skill provider.
func NewBuiltin(workDir string) *Builtin {
	if workDir == "" {
		workDir = os.TempDir()
	}
	return &Builtin{
		workDir: workDir,
		client:  &http.Client{Timeout: 15 * time.Second},
	}
}

func (b *Builtin) ListTools(_ context.Context) ([]mcp.Tool, error) {
	return []mcp.Tool{
		{
			Name:        "web_fetch",
			Description: "Fetch the content of a URL and return the body text (max 32KB).",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"url": map[string]any{"type": "string", "description": "The URL to fetch"},
				},
				"required": []string{"url"},
			},
		},
		{
			Name:        "code_exec",
			Description: "Execute a code snippet in Python or Node.js and return stdout/stderr.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"language": map[string]any{"type": "string", "enum": []string{"python", "node"}, "description": "Programming language"},
					"code":     map[string]any{"type": "string", "description": "Code to execute"},
				},
				"required": []string{"language", "code"},
			},
		},
		{
			Name:        "file_read",
			Description: "Read the contents of a file in the workspace.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{"type": "string", "description": "Relative file path"},
				},
				"required": []string{"path"},
			},
		},
		{
			Name:        "file_write",
			Description: "Write content to a file in the workspace.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path":    map[string]any{"type": "string", "description": "Relative file path"},
					"content": map[string]any{"type": "string", "description": "File content"},
				},
				"required": []string{"path", "content"},
			},
		},
		{
			Name:        "file_list",
			Description: "List files and directories in a workspace path.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{"type": "string", "description": "Relative directory path (default: .)"},
				},
			},
		},
	}, nil
}

func (b *Builtin) CallTool(ctx context.Context, name string, args map[string]any) (*mcp.CallResult, error) {
	switch name {
	case "web_fetch":
		return b.webFetch(ctx, args)
	case "code_exec":
		return b.codeExec(ctx, args)
	case "file_read":
		return b.fileRead(args)
	case "file_write":
		return b.fileWrite(args)
	case "file_list":
		return b.fileList(args)
	default:
		return nil, mcp.ErrToolNotFound
	}
}

func (b *Builtin) webFetch(ctx context.Context, args map[string]any) (*mcp.CallResult, error) {
	url := mcp.StringArg(args, "url")
	if url == "" {
		return mcp.ErrorResult("url is required"), nil
	}
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "https://" + url
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return mcp.ErrorResult(err.Error()), nil
	}
	req.Header.Set("User-Agent", "YunqueAgent/1.0")

	resp, err := b.client.Do(req)
	if err != nil {
		return mcp.ErrorResult(fmt.Sprintf("fetch failed: %v", err)), nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 32*1024))
	if err != nil {
		return mcp.ErrorResult(err.Error()), nil
	}

	return mcp.SuccessResult(fmt.Sprintf("HTTP %d\n\n%s", resp.StatusCode, string(body))), nil
}

func (b *Builtin) codeExec(ctx context.Context, args map[string]any) (*mcp.CallResult, error) {
	lang := mcp.StringArg(args, "language")
	code := mcp.StringArg(args, "code")
	if code == "" {
		return mcp.ErrorResult("code is required"), nil
	}

	var cmd *exec.Cmd
	var ext string
	switch lang {
	case "python":
		interpreter := "python3"
		if runtime.GOOS == "windows" {
			interpreter = "python"
		}
		ext = ".py"
		tmpFile := filepath.Join(b.workDir, fmt.Sprintf("exec_%d%s", time.Now().UnixNano(), ext))
		if err := os.WriteFile(tmpFile, []byte(code), 0644); err != nil {
			return mcp.ErrorResult(err.Error()), nil
		}
		defer os.Remove(tmpFile)
		cmd = exec.CommandContext(ctx, interpreter, tmpFile)
	case "node":
		ext = ".js"
		tmpFile := filepath.Join(b.workDir, fmt.Sprintf("exec_%d%s", time.Now().UnixNano(), ext))
		if err := os.WriteFile(tmpFile, []byte(code), 0644); err != nil {
			return mcp.ErrorResult(err.Error()), nil
		}
		defer os.Remove(tmpFile)
		cmd = exec.CommandContext(ctx, "node", tmpFile)
	default:
		return mcp.ErrorResult("unsupported language: " + lang + " (use python or node)"), nil
	}

	execCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd = exec.CommandContext(execCtx, cmd.Path, cmd.Args[1:]...)
	cmd.Dir = b.workDir

	out, err := cmd.CombinedOutput()
	result := string(out)
	if len(result) > 32*1024 {
		result = result[:32*1024] + "\n...(truncated)"
	}
	if err != nil {
		return mcp.SuccessResult(fmt.Sprintf("Exit error: %v\n\n%s", err, result)), nil
	}
	return mcp.SuccessResult(result), nil
}

func (b *Builtin) fileRead(args map[string]any) (*mcp.CallResult, error) {
	path := mcp.StringArg(args, "path")
	if path == "" {
		return mcp.ErrorResult("path is required"), nil
	}

	fullPath := filepath.Join(b.workDir, filepath.Clean(path))
	if !strings.HasPrefix(fullPath, b.workDir) {
		return mcp.ErrorResult("path escape not allowed"), nil
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return mcp.ErrorResult(err.Error()), nil
	}
	if len(data) > 64*1024 {
		data = append(data[:64*1024], []byte("\n...(truncated)")...)
	}
	return mcp.SuccessResult(string(data)), nil
}

func (b *Builtin) fileWrite(args map[string]any) (*mcp.CallResult, error) {
	path := mcp.StringArg(args, "path")
	content := mcp.StringArg(args, "content")
	if path == "" {
		return mcp.ErrorResult("path is required"), nil
	}

	fullPath := filepath.Join(b.workDir, filepath.Clean(path))
	if !strings.HasPrefix(fullPath, b.workDir) {
		return mcp.ErrorResult("path escape not allowed"), nil
	}

	os.MkdirAll(filepath.Dir(fullPath), 0755)
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		return mcp.ErrorResult(err.Error()), nil
	}
	return mcp.SuccessResult(fmt.Sprintf("wrote %d bytes to %s", len(content), path)), nil
}

func (b *Builtin) fileList(args map[string]any) (*mcp.CallResult, error) {
	path := mcp.StringArg(args, "path")
	if path == "" {
		path = "."
	}

	fullPath := filepath.Join(b.workDir, filepath.Clean(path))
	if !strings.HasPrefix(fullPath, b.workDir) {
		return mcp.ErrorResult("path escape not allowed"), nil
	}

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return mcp.ErrorResult(err.Error()), nil
	}

	var lines []string
	for _, e := range entries {
		info, _ := e.Info()
		suffix := ""
		size := ""
		if e.IsDir() {
			suffix = "/"
		} else if info != nil {
			size = fmt.Sprintf(" (%d bytes)", info.Size())
		}
		lines = append(lines, fmt.Sprintf("%s%s%s", e.Name(), suffix, size))
	}
	if len(lines) == 0 {
		return mcp.SuccessResult("(empty directory)"), nil
	}
	return mcp.SuccessResult(strings.Join(lines, "\n")), nil
}
