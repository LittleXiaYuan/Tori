package sandbox

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/daytonaio/daytona/libs/sdk-go/pkg/daytona"
	"github.com/daytonaio/daytona/libs/sdk-go/pkg/options"
	"github.com/daytonaio/daytona/libs/sdk-go/pkg/types"
)

// DaytonaConfig configures the Daytona sandbox runtime.
type DaytonaConfig struct {
	Enabled bool          `json:"enabled"`
	APIKey  string        `json:"api_key"`
	APIURL  string        `json:"api_url"`
	Target  string        `json:"target"` // region, e.g. "us", "eu"
	Image   string        `json:"image"`  // default Docker image
	Timeout time.Duration `json:"timeout"`
}

// DefaultDaytonaConfig returns sensible defaults.
func DefaultDaytonaConfig() DaytonaConfig {
	return DaytonaConfig{
		APIURL:  "https://app.daytona.io/api",
		Target:  "us",
		Image:   "python:3.12-slim",
		Timeout: 120 * time.Second,
	}
}

// DaytonaRunner implements Runner using the Daytona Go SDK.
type DaytonaRunner struct {
	client  *daytona.Client
	cfg     DaytonaConfig
	sandbox *daytona.Sandbox
}

// NewDaytonaRunner creates a Daytona-backed Runner.
func NewDaytonaRunner(cfg DaytonaConfig) (*DaytonaRunner, error) {
	if cfg.APIKey == "" {
		cfg.APIKey = os.Getenv("DAYTONA_API_KEY")
	}
	if cfg.APIURL == "" {
		cfg.APIURL = os.Getenv("DAYTONA_API_URL")
		if cfg.APIURL == "" {
			cfg.APIURL = DefaultDaytonaConfig().APIURL
		}
	}
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("daytona: API key required (set DAYTONA_API_KEY or config)")
	}

	dcfg := &types.DaytonaConfig{
		APIKey: cfg.APIKey,
		APIUrl: cfg.APIURL,
	}
	if cfg.Target != "" {
		dcfg.Target = cfg.Target
	}

	client, err := daytona.NewClientWithConfig(dcfg)
	if err != nil {
		return nil, fmt.Errorf("daytona: create client: %w", err)
	}

	return &DaytonaRunner{client: client, cfg: cfg}, nil
}

func (d *DaytonaRunner) Type() string { return "daytona" }

func (d *DaytonaRunner) Run(ctx context.Context, req RunRequest) (*RunResult, error) {
	start := time.Now()

	sandbox, err := d.getOrCreateSandbox(ctx)
	if err != nil {
		return nil, fmt.Errorf("daytona: sandbox: %w", err)
	}

	timeout := d.cfg.Timeout
	if req.Timeout > 0 {
		timeout = req.Timeout
	}

	if req.Code != "" {
		return d.runCode(ctx, sandbox, req, timeout, start)
	}
	if req.Command != "" {
		return d.runCommand(ctx, sandbox, req, timeout, start)
	}
	return nil, fmt.Errorf("daytona: no code or command specified")
}

func (d *DaytonaRunner) runCode(ctx context.Context, sb *daytona.Sandbox, req RunRequest, timeout time.Duration, start time.Time) (*RunResult, error) {
	for name, content := range req.Files {
		if err := sb.FileSystem.UploadFile(ctx, "/workspace/"+name, content); err != nil {
			slog.Warn("daytona: upload file failed", "file", name, "err", err)
		}
	}

	result, err := sb.Process.CodeRun(ctx, req.Code,
		options.WithCodeRunLanguage(mapLanguage(req.Language)),
		options.WithCodeRunTimeout(timeout),
	)
	if err != nil {
		return &RunResult{ExitCode: 1, Stderr: err.Error(), Duration: time.Since(start)}, nil
	}

	return &RunResult{
		ExitCode: result.ExitCode,
		Stdout:   result.Result,
		Duration: time.Since(start),
	}, nil
}

func (d *DaytonaRunner) runCommand(ctx context.Context, sb *daytona.Sandbox, req RunRequest, timeout time.Duration, start time.Time) (*RunResult, error) {
	cmd := req.Command
	if len(req.Args) > 0 {
		cmd += " " + strings.Join(req.Args, " ")
	}

	opts := []func(*options.ExecuteCommand){
		options.WithExecuteTimeout(timeout),
	}
	if len(req.Env) > 0 {
		opts = append(opts, options.WithCommandEnv(req.Env))
	}

	result, err := sb.Process.ExecuteCommand(ctx, cmd, opts...)
	if err != nil {
		return &RunResult{ExitCode: 1, Stderr: err.Error(), Duration: time.Since(start)}, nil
	}

	return &RunResult{
		ExitCode: result.ExitCode,
		Stdout:   result.Result,
		Duration: time.Since(start),
	}, nil
}

func (d *DaytonaRunner) getOrCreateSandbox(ctx context.Context) (*daytona.Sandbox, error) {
	if d.sandbox != nil {
		return d.sandbox, nil
	}

	image := d.cfg.Image
	if image == "" {
		image = "python:3.12-slim"
	}

	sandbox, err := d.client.Create(ctx, types.ImageParams{
		Image: image,
	}, options.WithTimeout(90*time.Second))
	if err != nil {
		return nil, err
	}

	d.sandbox = sandbox
	slog.Info("daytona: sandbox created", "id", sandbox.ID, "name", sandbox.Name)
	return sandbox, nil
}

func (d *DaytonaRunner) Close() error {
	if d.sandbox != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := d.sandbox.Delete(ctx); err != nil {
			slog.Warn("daytona: cleanup sandbox failed", "err", err)
		}
		d.sandbox = nil
	}
	return nil
}

func mapLanguage(lang string) types.CodeLanguage {
	switch strings.ToLower(lang) {
	case "python":
		return types.CodeLanguagePython
	case "javascript", "js", "node":
		return types.CodeLanguageJavaScript
	case "typescript", "ts":
		return types.CodeLanguageTypeScript
	default:
		return types.CodeLanguagePython
	}
}
