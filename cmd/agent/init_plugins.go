package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	agentrt "yunque-agent/internal/agentcore/runtime"
	"yunque-agent/internal/agentcore/skillmarket"
	"yunque-agent/internal/agentcore/task"
	"yunque-agent/internal/agentcore/websearch"
	"yunque-agent/internal/config"
	"yunque-agent/internal/execution/sandbox"
	iledger "yunque-agent/internal/ledger"
	mcppkg "yunque-agent/internal/mcp"
	mcpskills "yunque-agent/internal/mcp/skills"
	"yunque-agent/internal/observe"
	"yunque-agent/pkg/document"
	"yunque-agent/pkg/plugin"
	"yunque-agent/pkg/safego"
	"yunque-agent/pkg/skills"
	"yunque-agent/plugins/education"
	"yunque-agent/plugins/general"

	"yunque-agent/internal/ledgercore"
)

// initPlugins initializes the plugin registry, skill registry, web search,
// MCP gateway, and observability.
// Extracted from main.go lines 425-530.
func initPlugins(app *agentrt.App) error {
	cfg := app.Config

	// Web search registry
	searchReg := websearch.NewRegistry()
	if braveKey := os.Getenv("BRAVE_API_KEY"); braveKey != "" {
		searchReg.Register(websearch.NewBrave(braveKey))
		slog.Info("brave search registered")
	}
	if tavilyKey := os.Getenv("TAVILY_API_KEY"); tavilyKey != "" {
		searchReg.Register(websearch.NewTavily(tavilyKey))
		slog.Info("tavily search registered")
	}
	if searxURL := os.Getenv("SEARXNG_URL"); searxURL != "" {
		searchReg.Register(websearch.NewSearXNG(searxURL))
		slog.Info("searxng search registered")
	}
	app.Set(agentrt.CompSearchReg, searchReg)

	// Host paths for plugin file access — auto-discover user directories
	// when not explicitly configured.
	hostPaths := splitConfiguredPaths(cfg.HostReadPaths)
	if len(hostPaths) == 0 {
		hostPaths = config.DefaultReadPaths()
		if len(hostPaths) == 0 {
			cwd, _ := os.Getwd()
			hostPaths = []string{cwd}
		} else {
			slog.Info("file access: auto-discovered user directories", "paths", hostPaths)
		}
	}
	writePaths := splitConfiguredPaths(cfg.HostWritePaths)

	// Python environment for Office skills (detect system Python or use Go fallback).
	// Detection touches the filesystem and, when a system Python is present, spawns the
	// interpreter to probe packages (~0.2-0.4s warm, more on a cold disk). Running it
	// synchronously here used to block bootstrap — and therefore the desktop loader,
	// which waits on /healthz before revealing the app. We resolve it in the background
	// instead (goroutine after the initial skill-registry build below): office skills
	// serve via the Go engine immediately and upgrade to the Python engine once detection
	// finishes, so startup / healthz never pay for interpreter probing.
	pyEnv := sandbox.NewPythonEnv(cfg.DataDir)
	app.Set("python_env", pyEnv)

	// Plugin registry
	app.PluginReg = plugin.NewRegistry()
	generalPlugin := general.New(hostPaths)
	if len(writePaths) > 0 {
		generalPlugin.SetHostWritePaths(writePaths)
		slog.Info("file access: configured writable directories", "paths", writePaths)
	}
	// Bridge websearch →plugin search function
	if len(searchReg.List()) > 0 {
		generalPlugin.SetSearchFunc(func(ctx context.Context, query string, limit int) ([]general.SearchResult, error) {
			results, err := searchReg.Search(ctx, query, limit)
			if err != nil {
				return nil, err
			}
			out := make([]general.SearchResult, len(results))
			for i, r := range results {
				out[i] = general.SearchResult{Title: r.Title, URL: r.URL, Snippet: r.Snippet}
			}
			return out, nil
		})
		slog.Info("web search: external providers bridged to plugin")
	}
	app.PluginReg.Register(generalPlugin)
	app.PluginReg.Register(education.New())

	// Plugin state manager
	pluginStateMgr := plugin.NewPluginStateManager(cfg.DataPath("plugin_state"))
	app.Set(agentrt.CompPluginStateMgr, pluginStateMgr)

	// Wire dynamic skills to Ledger KV
	if ldgRaw, ok := app.Get(agentrt.CompLedger); ok {
		if ldg, ok := ldgRaw.(*ledger.Ledger); ok {
			migrator := iledger.NewKVMigrator(ldg)
			_ = migrator.MigrateFile("dynamic_skills", "defs", cfg.DataPath("dynamic_skills.json"))
			task.SetDynamicSkillsKV(iledger.NewKVConfigStore(ldg, "dynamic_skills"))
			slog.Info("dynamic skills wired to Ledger KV")
		}
	}

	// Skill registry → populated from all plugin skills; rebuilt on hot-reload
	app.SkillRegistry = skills.NewRegistry()
	rebuildSkillRegistry := func() {
		// Collect the full baseline skill set first, then swap in one shot so
		// concurrent request handlers never observe an empty registry during
		// the rebuild window (plugin hot-reload + ReAct request interleaving
		// would otherwise panic on concurrent map read/write).
		baseline := append([]skills.Skill{}, app.PluginReg.AllSkills()...)
		baseline = append(baseline,
			&document.MedicalExcelSplitSkill{},
			&document.PythonInterpreterSkill{},
		)
		app.SkillRegistry.ReplaceAll(baseline)

		// Dynamic skills live in a JSON file; loader calls Register() internally,
		// so it needs to run after ReplaceAll to survive the swap.
		if err := task.LoadDynamicSkills(app.SkillRegistry, cfg.DataPath("dynamic_skills.json")); err != nil {
			slog.Warn("failed to load dynamic skills", "err", err)
		}
		slog.Info("skill registry rebuilt", "skills", len(app.SkillRegistry.All()))
	}

	// Script plugin loader with hot-reload → rebuilds SkillRegistry on change
	pluginLoader := plugin.NewLoader(cfg.DataPath("plugins"), app.PluginReg, func() {
		slog.Info("plugin change detected, rebuilding skill registry")
		rebuildSkillRegistry()
		// Invalidate planner prompt cache so new skills appear to the LLM
		if app.Planner != nil {
			app.Planner.InvalidatePromptCache()
			slog.Info("planner prompt cache invalidated after plugin hot-reload")
		}
	})
	scriptCount := pluginLoader.LoadAll()
	pluginLoader.Watch(PluginWatchInterval)
	if scriptCount > 0 {
		slog.Info("script plugins loaded from data/plugins", "count", scriptCount)
	}
	app.Set(agentrt.CompPluginLoader, pluginLoader)

	// Load Go shared library plugins (.so files) on Linux/macOS
	soLoader := plugin.NewSOLoader(cfg.DataPath("plugins"), app.PluginReg)
	soCount := soLoader.LoadAll()
	if soCount > 0 {
		slog.Info("Go .so plugins loaded", "count", soCount)
	}

	// Initial population from built-in plugins
	rebuildSkillRegistry()

	// Resolve the Python environment off the critical path. When a system Python with
	// the Office packages is detected, inject it and rebuild the skill registry so the
	// Docx/Pptx skills upgrade from the Go engine to the richer Python engine without a
	// restart (ReplaceAll makes the swap concurrency-safe). The skills' LLM-facing
	// names/descriptions are identical either way, so no planner prompt-cache flush is
	// needed. Until detection finishes the Go fallback handles requests, and /healthz
	// (hence the desktop loader) never blocks on interpreter probing.
	safego.Go("python-env-resolve", func() {
		if pyBin := pyEnv.PythonBin(); pyBin != "" {
			generalPlugin.SetPythonBin(pyBin)
			rebuildSkillRegistry()
			slog.Info("office skills: Python engine available", "bin", pyBin, "tier", pyEnv.Tier().String())
		} else {
			slog.Info("office skills: Go-only mode (no Python detected)")
		}
	})

	// SkillFileLoader → auto-scan data/skills/ for SKILL.md packages
	skillFileLoader := skillmarket.NewSkillFileLoader(cfg.DataPath("skills"), app.SkillRegistry, func() {
		slog.Info("skill file change detected, rebuilding")
		rebuildSkillRegistry()
		if app.Planner != nil {
			app.Planner.InvalidatePromptCache()
		}
	})
	fileSkillCount := skillFileLoader.LoadAll()
	skillFileLoader.Watch(PluginWatchInterval)
	app.Set("skill_file_loader", skillFileLoader)
	if fileSkillCount > 0 {
		slog.Info("file-based skills loaded from data/skills", "count", fileSkillCount)
	}

	slog.Info("plugins loaded", "plugins", len(app.PluginReg.All()), "skills", len(app.SkillRegistry.All()))

	// Hook manager → lifecycle event bus for plugins
	hookMgr := plugin.NewHookManager()
	app.Set("hook_manager", hookMgr)
	// Register hook handlers for script plugins that declare hooks
	for _, p := range app.PluginReg.All() {
		sp, ok := p.(*plugin.ScriptPlugin)
		if !ok {
			continue
		}
		for _, hookName := range sp.Manifest().Hooks {
			hookName := hookName // capture
			sp := sp             // capture
			hookMgr.Register(hookName, sp.Name(), func(ctx context.Context, payload plugin.HookPayload) error {
				_, err := sp.CallHook(ctx, payload)
				return err
			})
			slog.Info("plugin hook registered", "plugin", sp.Name(), "hook", hookName)
		}
	}

	// Service plugin manager → starts service-type script plugins
	svcMgr := plugin.NewServiceManager()
	app.Set("service_manager", svcMgr)
	svcCtx := context.Background() // service plugins run for the lifetime of the agent
	for _, p := range app.PluginReg.All() {
		sp, ok := p.(*plugin.ScriptPlugin)
		if !ok || sp.Manifest().Type != plugin.PluginTypeService {
			continue
		}
		if err := svcMgr.Start(svcCtx, sp); err != nil {
			slog.Warn("service plugin start failed", "plugin", sp.Name(), "err", err)
		} else {
			slog.Info("service plugin started", "plugin", sp.Name())
		}
	}

	// Plugin lifecycle ?call OnInit
	pluginEnv := &plugin.PluginEnv{
		LLMCall: llmChatFunc(app.LLMClient, DefaultLLMTemperature),
	}
	app.PluginReg.InitAll(context.Background(), pluginStateMgr, pluginEnv)

	// MCP Gateway ?federate tools from built-in + external MCP servers
	mcpBuiltin := mcpskills.NewBuiltin(filepath.Join(cfg.DataDir, "output"))
	mcpGw := mcppkg.NewGateway([]mcppkg.Provider{mcpBuiltin}, MCPConnectTimeout)
	app.Set(agentrt.CompMCPGateway, mcpGw)

	// Load external MCP servers
	if mcpCfg, err := mcppkg.LoadConfig(cfg.DataPath("mcp.json")); err == nil {
		n := mcppkg.ConnectAll(context.Background(), mcpCfg, mcpGw)
		slog.Info("mcp external servers connected", "count", n)
	} else if !os.IsNotExist(err) {
		slog.Warn("mcp config load failed", "err", err)
	}

	// Register MCP tools as skills
	if err := mcppkg.RegisterAll(mcpGw, app.SkillRegistry); err != nil {
		slog.Warn("mcp skill registration failed", "err", err)
	} else {
		slog.Info("mcp tools registered", "total", mcpGw.ToolCount(context.Background()))
	}

	// Observability
	app.Metrics = observe.New()
	slog.Info("observability initialized")

	return nil
}

func splitConfiguredPaths(raw string) []string {
	var out []string
	for _, p := range strings.Split(raw, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
