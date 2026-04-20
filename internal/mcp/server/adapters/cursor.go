// Package adapters provides MCP configuration templates and worker
// integration logic for external tools like Cursor, Claude Code, Windsurf.
package adapters

import (
	"encoding/json"
	"fmt"
)

// CursorConfig generates a Cursor MCP server configuration that connects
// back to the yunque dispatch server.
//
// Users add this to their Cursor settings (Settings → MCP) or to
// .cursor/mcp.json in their project root.
type CursorConfig struct {
	ServerURL string `json:"server_url"` // e.g. "http://localhost:9800/mcp/v1"
	Token     string `json:"token,omitempty"`
}

// GenerateMCPJSON returns a JSON config block suitable for Cursor's
// mcpServers configuration. When Token is empty, the Authorization header
// is omitted so Cursor does not send a bare "Bearer " (which the gateway
// would reject now that /mcp/v1 POST requires valid credentials).
func (c *CursorConfig) GenerateMCPJSON() (string, error) {
	entry := map[string]any{
		"url": c.ServerURL,
	}
	if c.Token != "" {
		entry["headers"] = map[string]string{
			"Authorization": fmt.Sprintf("Bearer %s", c.Token),
		}
	}
	config := map[string]any{
		"mcpServers": map[string]any{
			"yunque-dispatch": entry,
		},
	}
	b, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// WorkerInstructions returns the system prompt / rule that should be added
// to the Cursor project so the AI agent knows how to poll and execute tasks.
func WorkerInstructions(workerName, workerType string, capabilities []string) string {
	capsJSON, _ := json.Marshal(capabilities)
	return fmt.Sprintf(`# Yunque Worker Instructions

You are connected to the Yunque orchestrator as a worker.
Your role is to poll for tasks, execute them, and report results.

## Startup

On first message or when asked to "start working", call the register_worker tool:

register_worker({
  "name": %q,
  "type": %q,
  "capabilities": %s,
  "max_concurrency": 1
})

Save the returned worker_id for all subsequent calls.

## Work Loop

1. Call get_pending_tasks with your worker_id
2. If tasks are available, call claim_task for the highest priority one
3. Call get_task_context to get full details
4. Execute the task using your coding capabilities
5. Call report_progress periodically with status updates
6. When done, call submit_result with success=true and a summary
7. If you encounter an error, call submit_result with success=false
8. Go back to step 1

## Important

- Always call report_progress at least every 5 minutes
- If a task seems too complex, break it into smaller steps in your progress reports
- Include relevant file paths and code snippets in your result summary
`, workerName, workerType, string(capsJSON))
}

// ClaudeCodeConfig generates a config for Claude Code CLI integration.
type ClaudeCodeConfig struct {
	ServerURL string `json:"server_url"`
	Token     string `json:"token,omitempty"`
}

// GenerateMCPJSON returns a JSON config for Claude Code's MCP settings.
// Authorization headers are emitted only when Token is non-empty so the
// generated config does not silently drop the credential the user set
// via /v1/workers/config.
func (c *ClaudeCodeConfig) GenerateMCPJSON() (string, error) {
	entry := map[string]any{
		"type": "url",
		"url":  c.ServerURL,
	}
	if c.Token != "" {
		entry["headers"] = map[string]string{
			"Authorization": fmt.Sprintf("Bearer %s", c.Token),
		}
	}
	config := map[string]any{
		"mcpServers": map[string]any{
			"yunque-dispatch": entry,
		},
	}
	b, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// WindsurfConfig generates config for Windsurf editor.
type WindsurfConfig struct {
	ServerURL string `json:"server_url"`
	Token     string `json:"token,omitempty"`
}

// GenerateMCPJSON returns a JSON config for Windsurf's MCP settings.
// Windsurf requires stdio transport (command+args), not URL-based.
func (c *WindsurfConfig) GenerateMCPJSON() (string, error) {
	serverEntry := map[string]any{
		"command": "npx",
		"args":    []string{"@cloudtori/yunque-bridge", "-s", c.ServerURL},
	}
	if c.Token != "" {
		serverEntry["env"] = map[string]string{
			"YUNQUE_TOKEN": c.Token,
		}
	}
	config := map[string]any{
		"mcpServers": map[string]any{
			"yunque-dispatch": serverEntry,
		},
	}
	b, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}
