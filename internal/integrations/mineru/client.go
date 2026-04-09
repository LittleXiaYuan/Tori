package mineru

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"yunque-agent/internal/config"
)

type Client struct {
	enabled   bool
	backend   string
	command   string
	cliArgs   []string
	outputDir string
	timeout   time.Duration
}

type ParseResult struct {
	Backend      string
	WorkDir      string
	MarkdownPath string
	Markdown     string
	JSONPath     string
	JSON         string
	Stdout       string
	Stderr       string
}

func NewFromConfig(cfg *config.Config) *Client {
	outputDir := cfg.MinerUOutputDir
	if strings.TrimSpace(outputDir) == "" {
		outputDir = cfg.DataPath("mineru")
	}

	timeout := time.Duration(cfg.MinerUTimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}

	return &Client{
		enabled:   cfg.MinerUEnabled,
		backend:   strings.ToLower(strings.TrimSpace(cfg.MinerUBackend)),
		command:   strings.TrimSpace(cfg.MinerUCommand),
		cliArgs:   strings.Fields(strings.TrimSpace(cfg.MinerUCLIArgs)),
		outputDir: outputDir,
		timeout:   timeout,
	}
}

func (c *Client) Enabled() bool {
	return c != nil && c.enabled
}

func (c *Client) ParseFile(ctx context.Context, filePath string) (*ParseResult, error) {
	if c == nil || !c.enabled {
		return nil, fmt.Errorf("MinerU is not enabled")
	}
	switch c.backend {
	case "", "cli":
		return c.parseCLI(ctx, filePath)
	default:
		return nil, fmt.Errorf("unsupported MinerU backend: %s", c.backend)
	}
}

func (c *Client) parseCLI(ctx context.Context, filePath string) (*ParseResult, error) {
	if c.command == "" {
		return nil, fmt.Errorf("MINERU_COMMAND is not configured")
	}

	absFile, err := filepath.Abs(filePath)
	if err != nil {
		return nil, fmt.Errorf("resolve file path: %w", err)
	}
	if _, err := os.Stat(absFile); err != nil {
		return nil, fmt.Errorf("stat input file: %w", err)
	}

	runDir := filepath.Join(c.outputDir, time.Now().Format("20060102-150405"))
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return nil, fmt.Errorf("create MinerU output dir: %w", err)
	}

	args := c.cliArgs
	if len(args) == 0 {
		args = []string{"-p", "{input_file}", "-o", "{output_dir}", "-b", "pipeline"}
	}
	args = expandArgs(args, absFile, runDir)

	timeout := c.timeout
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, c.command, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Dir = filepath.Dir(absFile)

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("MinerU CLI failed: %w\nstderr: %s", err, strings.TrimSpace(stderr.String()))
	}

	mdPath, _ := pickFirstByExt(runDir, ".md")
	jsonPath, _ := pickFirstByExt(runDir, ".json")
	if mdPath == "" && jsonPath == "" {
		return nil, fmt.Errorf("MinerU finished but no markdown/json output was found in %s", runDir)
	}

	result := &ParseResult{
		Backend: c.backendOrDefault(),
		WorkDir: runDir,
		Stdout:  stdout.String(),
		Stderr:  stderr.String(),
	}
	if mdPath != "" {
		data, err := os.ReadFile(mdPath)
		if err != nil {
			return nil, fmt.Errorf("read MinerU markdown: %w", err)
		}
		result.MarkdownPath = mdPath
		result.Markdown = string(data)
	}
	if jsonPath != "" {
		data, err := os.ReadFile(jsonPath)
		if err != nil {
			return nil, fmt.Errorf("read MinerU json: %w", err)
		}
		result.JSONPath = jsonPath
		result.JSON = string(data)
	}
	return result, nil
}

func (c *Client) backendOrDefault() string {
	if strings.TrimSpace(c.backend) == "" {
		return "cli"
	}
	return c.backend
}

func expandArgs(args []string, inputFile, outputDir string) []string {
	out := make([]string, 0, len(args))
	for _, arg := range args {
		arg = strings.ReplaceAll(arg, "{input_file}", inputFile)
		arg = strings.ReplaceAll(arg, "{output_dir}", outputDir)
		out = append(out, arg)
	}
	return out
}

func pickFirstByExt(root, ext string) (string, error) {
	var matches []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ext) {
			matches = append(matches, path)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", nil
	}
	sort.Strings(matches)
	return matches[0], nil
}
