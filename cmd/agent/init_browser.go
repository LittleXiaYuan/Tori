package main

import (
	"log/slog"

	agentrt "yunque-agent/internal/agentcore/runtime"
)

// initBrowser is a no-op — headless browser engine has been replaced by the
// Yunque Browser Connector extension (Chrome extension + WebSocket).
// Browser skills are registered via browserskill package in init_tasks.go.
func initBrowser(app *agentrt.App) error {
	slog.Info("browser: headless engine removed — using extension-based browser connector")
	return nil
}

// wireDocxVerifier is a no-op — browser engine no longer available for DOCX preview.
func wireDocxVerifier(app *agentrt.App) {}
