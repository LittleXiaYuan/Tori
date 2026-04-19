package orchestrator

import (
	"os/exec"
	"runtime"
)

type IDEInfo struct {
	Name          string `json:"name"`
	Binary        string `json:"binary"`
	Available     bool   `json:"available"`
	Path          string `json:"path,omitempty"`
	MCPConfigPath string `json:"mcp_config_path"`
	RulesPath     string `json:"rules_path,omitempty"`
	HasAdapter    bool   `json:"has_adapter"`
}

type ideCandidate struct {
	name          string
	binaries      []string // try multiple binary names
	mcpConfigPath string
	rulesPath     string
	hasAdapter    bool
}

var knownIDEs = []ideCandidate{
	// --- IDE with built-in adapter ---
	{name: "Cursor", binaries: binaryVariants("cursor"), mcpConfigPath: ".cursor/mcp.json", rulesPath: ".cursor/rules/yunque-worker.mdc", hasAdapter: true},
	{name: "Claude Code", binaries: binaryVariants("claude"), mcpConfigPath: ".mcp/mcp.json", rulesPath: "CLAUDE.md", hasAdapter: true},
	{name: "Windsurf", binaries: binaryVariants("windsurf"), mcpConfigPath: ".windsurf/mcp_config.json", rulesPath: ".windsurfrules", hasAdapter: true},
	{name: "Trae", binaries: binaryVariants("trae"), mcpConfigPath: ".trae/mcp.json", rulesPath: ".trae/rules/yunque-worker.md", hasAdapter: true},

	// --- IDE / Editors (auto-register via GenericAdapter) ---
	{name: "VS Code", binaries: binaryVariants("code"), mcpConfigPath: ".vscode/mcp.json", rulesPath: ".github/copilot-instructions.md"},
	{name: "VS Code Insiders", binaries: binaryVariants("code-insiders"), mcpConfigPath: ".vscode/mcp.json", rulesPath: ".github/copilot-instructions.md"},
	{name: "JetBrains IDEA", binaries: binaryVariants("idea"), mcpConfigPath: ".idea/mcp.json", rulesPath: ""},
	{name: "JetBrains GoLand", binaries: binaryVariants("goland"), mcpConfigPath: ".idea/mcp.json", rulesPath: ""},
	{name: "JetBrains PyCharm", binaries: binaryVariants("pycharm"), mcpConfigPath: ".idea/mcp.json", rulesPath: ""},
	{name: "JetBrains WebStorm", binaries: binaryVariants("webstorm"), mcpConfigPath: ".idea/mcp.json", rulesPath: ""},
	{name: "Zed", binaries: binaryVariants("zed"), mcpConfigPath: ".zed/mcp.json", rulesPath: ""},
	{name: "Neovim", binaries: binaryVariants("nvim"), mcpConfigPath: ".nvim/mcp.json", rulesPath: ""},

	// --- CLI Coding Agents ---
	{name: "Codex CLI", binaries: binaryVariants("codex"), mcpConfigPath: ".mcp/mcp.json", rulesPath: "AGENTS.md"},
	{name: "Aider", binaries: binaryVariants("aider"), mcpConfigPath: ".mcp/mcp.json", rulesPath: ".aider.conf.yml"},
	{name: "Cline", binaries: binaryVariants("cline"), mcpConfigPath: ".mcp/mcp.json", rulesPath: ""},
	{name: "Continue", binaries: binaryVariants("continue"), mcpConfigPath: ".continue/mcp.json", rulesPath: ""},
	{name: "Goose", binaries: binaryVariants("goose"), mcpConfigPath: ".mcp/mcp.json", rulesPath: ""},
	{name: "OpenHands", binaries: binaryVariants("openhands"), mcpConfigPath: ".mcp/mcp.json", rulesPath: ""},
	{name: "OpenCode", binaries: binaryVariants("opencode"), mcpConfigPath: ".mcp/mcp.json", rulesPath: ""},
	{name: "Gemini CLI", binaries: binaryVariants("gemini"), mcpConfigPath: ".mcp/mcp.json", rulesPath: "GEMINI.md"},
}

func DetectIDEs() []IDEInfo {
	var result []IDEInfo
	for _, c := range knownIDEs {
		info := IDEInfo{
			Name:          c.name,
			MCPConfigPath: c.mcpConfigPath,
			RulesPath:     c.rulesPath,
			HasAdapter:    c.hasAdapter,
		}
		for _, bin := range c.binaries {
			path, err := exec.LookPath(bin)
			if err == nil {
				info.Available = true
				info.Binary = bin
				info.Path = path
				break
			}
		}
		if !info.Available {
			info.Binary = c.binaries[0]
		}
		result = append(result, info)
	}
	return result
}

// AutoRegisterAdapters detects installed IDEs that lack built-in adapters
// and registers them as GenericAdapters.
func AutoRegisterAdapters(launcher *Launcher) int {
	count := 0
	for _, ide := range DetectIDEs() {
		if !ide.Available || ide.HasAdapter {
			continue
		}
		cfg := GenericAdapterConfig{
			AdapterName:   ide.Name,
			Binary:        ide.Binary,
			MCPConfigPath: ide.MCPConfigPath,
			RulesFilePath: ide.RulesPath,
		}
		launcher.RegisterAdapter(NewGenericAdapter(cfg))
		count++
	}
	return count
}

func binaryVariants(base string) []string {
	if runtime.GOOS == "windows" {
		return []string{base + ".exe", base + ".cmd", base}
	}
	return []string{base}
}
