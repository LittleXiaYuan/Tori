package main

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	rdebug "runtime/debug"
	"time"

	"yunque-agent/internal/appdir"
	"yunque-agent/internal/supervisor"
	"yunque-agent/internal/version"
	"yunque-agent/pkg/safego"
)

// selfHealthCheck probes the local agent's /livez endpoint. Returns 0 on
// success, 1 on failure. Used as Docker HEALTHCHECK in scratch/distroless
// images where curl is not available.
func selfHealthCheck() int {
	addr := os.Getenv("AGENT_ADDR")
	if addr == "" {
		addr = ":9090"
	}
	if len(os.Args) >= 3 {
		addr = os.Args[2]
	}
	host := addr
	if host[0] == ':' {
		host = "localhost" + host
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://" + host + "/livez")
	if err != nil {
		fmt.Fprintf(os.Stderr, "healthcheck failed: %v\n", err)
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "healthcheck failed: status %d\n", resp.StatusCode)
		return 1
	}
	return 0
}

func setupLogging() {
	// Logs regularly contain request IDs, tenant IDs, and occasional LLM
	// prompts/responses that should never be world-readable on shared hosts.
	// 0o600 aligns with .env permissions; operators mounting the log dir into
	// an external log aggregator can run that aggregator as the same user.
	logPath := filepath.Join(appdir.DataDir(), "agent.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		// Fallback: log next to exe
		if exe, exeErr := os.Executable(); exeErr == nil {
			fallback := filepath.Join(filepath.Dir(exe), "agent.log")
			logFile, err = os.OpenFile(fallback, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		}
	}
	if err != nil || logFile == nil {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))
		return
	}
	w := io.MultiWriter(os.Stderr, logFile)
	slog.SetDefault(slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: slog.LevelInfo})))
}

func main() {
	// Self-contained health check for scratch/distroless containers (no curl needed).
	// Usage: ./agent --healthcheck [addr]
	if len(os.Args) >= 2 && os.Args[1] == "--healthcheck" {
		os.Exit(selfHealthCheck())
	}

	setupLogging()
	safego.SetPanicLogPath(appdir.File("panic.log"))

	if supervisor.ShouldSupervise() {
		os.Exit(supervisor.Run())
	}

	// Top-level panic recovery — last safety net for any unrecovered panic
	defer func() {
		if r := recover(); r != nil {
			stack := string(rdebug.Stack())
			slog.Error("FATAL PANIC — process crashing",
				"panic", r, "stack", stack)
			// Write to panic.log for post-mortem
			if f, err := os.OpenFile(appdir.File("panic.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600); err == nil {
				entry := fmt.Sprintf(
					"=== FATAL PANIC at %s ===\npanic: %v\n%s\n\n",
					time.Now().Format(time.RFC3339), r, stack,
				)
				f.WriteString(entry)
				f.Close()
			}
			os.Exit(2)
		}
	}()

	slog.Info(version.String())
	slog.Info("data directory", "path", appdir.DataDir())

	if err := appdir.MaybeMigrateLegacy(); err != nil {
		slog.Warn("legacy data migration failed", "err", err)
	}

	cfg := loadConfig()

	app, err := newApp(cfg)
	if err != nil {
		slog.Error("initialization failed", "err", err)
		os.Exit(1)
	}

	if err := initGateway(app); err != nil {
		slog.Error("gateway failed", "err", err)
		os.Exit(1)
	}
}
