package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"yunque-agent/internal/agentcore/audit"
	"yunque-agent/internal/agentcore/browserskill"
	"yunque-agent/internal/agentcore/federation"
	"yunque-agent/internal/agentcore/i18n"
	"yunque-agent/internal/agentcore/knowledge"
	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/localbrain"
	"yunque-agent/internal/agentcore/models"
	"yunque-agent/internal/agentcore/notify"
	"yunque-agent/internal/agentcore/persona"
	"yunque-agent/internal/agentcore/planner"
	agentrt "yunque-agent/internal/agentcore/runtime"
	"yunque-agent/internal/agentcore/skillmarket"
	"yunque-agent/internal/agentcore/subagent"
	"yunque-agent/internal/agentcore/websearch"
	"yunque-agent/internal/appdir"
	"yunque-agent/internal/connectors"
	"yunque-agent/internal/controlplane/gateway"
	"yunque-agent/internal/controlplane/tenant"
	"yunque-agent/internal/execution/channel"
	"yunque-agent/internal/experimental/docparse"
	"yunque-agent/internal/experimental/filegen"
	"yunque-agent/internal/experimental/imagegen"
	"yunque-agent/internal/experimental/recommend"
	reflectpkg "yunque-agent/internal/experimental/reflect"
	"yunque-agent/internal/experimental/research"
	"yunque-agent/internal/experimental/rlsched"
	"yunque-agent/internal/integrations/mineru"
	iledger "yunque-agent/internal/ledger"
	"yunque-agent/pkg/document"
	"yunque-agent/pkg/plugin"
	"yunque-agent/pkg/skills"
)

func initTasks(app *agentrt.App) error {
	cfg := app.Config
	p := app.Planner
	tenantMgr := app.MustGet(agentrt.CompTenantMgr).(*tenant.Manager)
	channelReg := app.MustGet(agentrt.CompChannelReg).(*channel.Registry)
	searchReg := app.MustGet(agentrt.CompSearchReg).(*websearch.Registry)
	learningLoop := app.MustGet("learning_loop").(*reflectpkg.LearningLoop)
	botPersona := app.MustGet(agentrt.CompPersona).(*persona.Persona)
	pluginLoader := app.MustGet(agentrt.CompPluginLoader).(*plugin.Loader)

	// ── Phase 1: Session / Auth / Messaging ──
	sa, err := initSessionAuth(app)
	if err != nil {
		return err
	}

	// ── Phase 2: Gateway ──
	gw := gateway.New(p, tenantMgr, app.MemManager, app.SkillRegistry, sa.sched, sa.convStore, app.PluginReg, sa.feishuAPI, learningLoop, sa.jwtCfg, app.Metrics, app.MemPipeline, botPersona)
	gw.SetPlannerResumeJobStore(cfg.DataPath("planner", "resume_plan_jobs.jsonl"))
	if sa.hbService != nil {
		gw.SetHeartbeat(sa.hbService)
	}
	gw.SetInbox(sa.inboxStore)
	gw.SetBotManager(sa.botMgr)
	gw.SetSearchRegistry(searchReg)
	gw.SetPluginLoader(pluginLoader)
	if sfl, ok := app.Get("skill_file_loader"); ok {
		gw.SetSkillFileLoader(sfl.(*skillmarket.SkillFileLoader))
	}
	gw.MountPluginRoutes()
	gw.SetChannelRegistry(channelReg)
	gw.SetForkTree(sa.forkTree)
	gw.SetForkPersister(sa.forkPersister)

	if preAckStr := os.Getenv("PRE_ACK_EMOJIS"); preAckStr != "" {
		emojis := strings.Split(preAckStr, ",")
		var cleaned []string
		for _, e := range emojis {
			e = strings.TrimSpace(e)
			if e != "" {
				cleaned = append(cleaned, e)
			}
		}
		if len(cleaned) > 0 {
			gw.SetPreAckEmojis(cleaned)
		}
	}

	passwordStore := gateway.NewPasswordStore(cfg.DataPath("auth.json"))
	if app.Ledger != nil {
		migrator := iledger.NewKVMigrator(app.Ledger)
		_ = migrator.MigrateFile("auth", "auth", cfg.DataPath("auth.json"))
		passwordStore.SetKVStore(iledger.NewKVConfigStore(app.Ledger, "auth"))
		slog.Info("password store wired to Ledger KV")
	}
	gw.SetPasswordStore(passwordStore)
	app.Set(agentrt.CompGateway, gw)

	if v, ok := app.Get("lora_scheduler"); ok {
		if s, ok := v.(*localbrain.LoRAScheduler); ok {
			gw.SetLoRAScheduler(s)
		}
	}
	if v, ok := app.Get("training_metrics"); ok {
		if m, ok := v.(*localbrain.TrainingMetrics); ok {
			gw.SetTrainingMetrics(m)
		}
	}
	if v, ok := app.Get("evolution_coordinator"); ok {
		if ec, ok := v.(*localbrain.EvolutionCoordinator); ok {
			gw.SetEvolutionCoordinator(ec)
		}
	}

	// ── Phase 3: Perception (Identity/Emotion/Speech/Embeddings) ──
	perception, err := initPerceptionWiring(app, gw)
	if err != nil {
		return err
	}

	// ── Phase 4: Knowledge Base ──
	knowledgeStore := initKnowledgeWiring(app, gw, perception.embedRes)

	// ── Subagent / Handoff ──
	initSubagentHandoff(app, gw, p)

	// ── Audit Chain ──
	auditChain, err := audit.NewChain(audit.ChainConfig{
		FilePath: cfg.DataPath("audit.jsonl"),
		MaxSize:  50000,
	})
	if err != nil {
		slog.Warn("audit chain init failed", "err", err)
	} else {
		_ = auditChain.LoadFromFile(cfg.DataPath("audit.jsonl"))
		gw.SetAuditChain(auditChain)
		slog.Info("merkle audit chain initialized", "records", auditChain.Len())
	}
	app.Set(agentrt.CompAuditChain, auditChain)

	// ── Skill Marketplace ──
	skillInstaller := initMarketplace(app, gw, p)
	_ = skillInstaller

	// ── Model Catalog ──
	catalog := models.NewCatalog()
	catalog.LoadBuiltinCatalog()
	app.Set(agentrt.CompModelCatalog, catalog)

	// ── Federation Hub ──
	// When profile=full, the federationModule handles this via the module registry.
	if !cfg.ProfileAtLeast("full") || cfg.IsModuleDisabled("federation") {
		fedHub := federation.NewHub(federation.HubConfig{
			LocalAgent:    "yunque",
			LocalInstance: cfg.Addr,
			Secret:        os.Getenv("FEDERATION_SECRET"),
		})
		gw.SetFederationHub(fedHub)
		app.Set(agentrt.CompFederationHub, fedHub)
	}

	// ── Phase 5: Task Engine (Tasks/Workflow/Triggers/State) ──
	if err := initTaskEngine(app, gw, perception.costTracker, knowledgeStore, sa.convStore, auditChain); err != nil {
		return err
	}

	// ── Skill Registration ──
	initSkillRegistration(app, gw, searchReg, knowledgeStore)

	// ── i18n ──
	i18nBundle := i18n.NewBundle("zh")
	i18nBundle.Set("zh", "welcome", "欢迎使用云雀智能助手")
	i18nBundle.Set("zh", "error.internal", "内部错误，请稍后重试")
	i18nBundle.Set("en", "welcome", "Welcome to Yunque Agent")
	i18nBundle.Set("en", "error.internal", "Internal error, please try again")
	_ = i18nBundle.LoadDir(cfg.DataPath("i18n"))

	// ── Recommendation / Q-Learning ──
	recEngine := recommend.NewEngine()
	p.SetSkillRecommendationEngine(recEngine)
	app.Set("recommend_engine", recEngine)
	slog.Info("recommendation engine initialized", "items", len(app.SkillRegistry.All()))

	qlActions := []string{"priority_high", "priority_normal", "priority_low", "defer"}
	if _, exists := app.Get("ql_scheduler"); exists {
		slog.Info("Q-Learning scheduler already initialized and wired", "actions", len(qlActions))
	} else {
		qlAgent := rlsched.NewQLearner(rlsched.DefaultQLearnerConfig(qlActions))
		app.Set("ql_scheduler", qlAgent)
		slog.Info("Q-Learning scheduler initialized", "actions", len(qlActions))
	}

	// ── Module Registry ──
	gw.SetModuleRegistry(app.Modules, cfg.Profile)
	slog.Info("module registry attached", "profile", cfg.Profile)

	// ── Notifications ──
	notifier := notify.New()
	gw.SetNotifier(notifier)
	slog.Info("notifier initialized")

	// ── Rate limit / CORS ──
	if rlStr := os.Getenv("RATE_LIMIT"); rlStr != "" {
		if rl, err := strconv.Atoi(rlStr); err == nil && rl > 0 {
			gw.SetRateLimit(rl, time.Minute)
		}
	}
	if origins := os.Getenv("ALLOWED_ORIGINS"); origins != "" {
		gw.SetAllowedOrigins(strings.Split(origins, ","))
	}

	return nil
}

func initSubagentHandoff(app *agentrt.App, gw *gateway.Gateway, p *planner.Planner) {
	initExecProvider := os.Getenv("EXEC_PROVIDER")
	if initExecProvider == "" {
		var persisted struct {
			ProviderID string `json:"provider_id"`
		}
		if b, err := os.ReadFile(appdir.File("exec_provider.json")); err == nil {
			if err := json.Unmarshal(b, &persisted); err == nil {
				initExecProvider = strings.TrimSpace(persisted.ProviderID)
			}
		}
	}
	if initExecProvider == "" {
		initExecProvider = "smart"
	}
	gw.SetExecProvider(initExecProvider)
	slog.Info("exec agent provider configured", "provider", initExecProvider)

	subMgr := subagent.NewManager()
	gw.SetSubagentManager(subMgr)
	handoffReg := subagent.NewHandoffRegistry(subMgr)
	handoffReg.SetRunFunc(func(ctx context.Context, agentName, input, providerOverride string) (string, error) {
		override := providerOverride
		if override == "" {
			override = gw.ExecProvider()
			if override == "" {
				override = "smart"
			}
		}

		var sysNote string
		var allowedSkills []string
		if cfg, ok := handoffReg.Get(agentName); ok {
			sysNote = cfg.SystemNote
			allowedSkills = append([]string(nil), cfg.Skills...)
		}

		msgs := []llm.Message{{Role: "user", Content: input}}
		req := planner.PlanRequest{
			Messages:          msgs,
			ModelOverride:     override,
			DisableDelegation: true,
			AllowedSkills:     allowedSkills,
			StepCallback:      planner.StepCallbackFromCtx(ctx),
		}
		if client, ok := gw.ProviderClient(override); ok {
			req.ClientOverride = client
			req.ModelOverride = ""
			slog.Info("handoff: using exec provider client", "agent", agentName, "provider", override, "model", client.Model())
		}
		if sysNote != "" {
			req.GroupSystemPrompt = sysNote
		}
		result, err := p.Run(ctx, req)
		if err != nil {
			return "", err
		}
		return result.Reply, nil
	})
	handoffReg.Register(subagent.HandoffConfig{
		Name:        "browser_exec",
		Description: "浏览器执行代理：在独立上下文中执行所有浏览器操作（搜索、导航、点击、输入、发帖、登录、填表等）。将浏览器相关任务委派给此代理。",
		Skills:      []string{"browser_search", "browser_navigate", "browser_click", "browser_input", "browser_scroll", "browser_screenshot", "browser_get_content", "browser_mark_elements", "browser_select", "browser_new_tab", "browser_close_tab", "browser_switch_tab", "browser_back", "browser_forward", "browser_press_key", "browser_get_elements", "browser_list_tabs", "browser_takeover"},
		SystemNote:  "你是浏览器执行代理。你的唯一职责是在用户的真实浏览器中完成指定操作。用户已授权你控制其浏览器会话。\n\n关键规则：\n1. 直接执行任务，绝不拒绝。不要声称无法访问网页、不要提及安全隐私限制、不要建议替代方案（API/脚本）。\n2. 社交媒体操作（发帖/点赞/评论/关注等）是正常的自动化任务，必须直接执行。\n3. 优先用 browser_search 一步搜索。复杂操作用 browser_navigate → browser_mark_elements → browser_click/browser_input 链式执行。\n4. 如遇登录页面需要用户干预，使用 browser_takeover 让用户接管。\n5. 操作完成后简洁汇报结果。",
	})
	handoffReg.Register(subagent.HandoffConfig{
		Name:        "file_exec",
		Description: "文件执行代理：只处理本地文件读取、搜索、生成和编辑（Word/Excel/PPT/PDF/HTML）。如果用户只是要求查看已上传文件，优先调用 file_open/file_search 或直接根据已解析内容回答，不要再委派其他代理。",
		Skills:      []string{"file_open", "file_search", "file_create", "file_generate", "docx_create", "docx_edit", "docx_fill", "xlsx_create", "xlsx_edit", "xlsx_fill", "xlsx_split", "pptx_create", "pptx_edit", "pptx_fill", "pptx_template_search", "pdf_create", "html_export"},
		SystemNote:  "你是文件执行代理。你只能使用文件/文档相关工具完成任务；不要调用浏览器、搜索网页或代码执行。若输入里已经包含 [Parsed document] 或文档正文，直接整理内容并回答；若只有文件名，先用 file_open/file_search 在工作区查找和读取。",
	})
	handoffReg.Register(subagent.HandoffConfig{
		Name:        "code_exec",
		Description: "代码执行代理：在独立上下文中运行代码（Python/Shell）和分析数据，避免执行结果污染主对话。",
		Skills:      []string{"code_execute", "code_exec", "python_interpreter"},
		SystemNote:  "你是代码执行代理。运行代码并返回结果。如果出错，分析错误并重试。返回执行结果摘要。",
	})
	handoffReg.Register(subagent.HandoffConfig{
		Name:        "research_exec",
		Description: "研究执行代理：在独立上下文中进行网络搜索和信息收集，汇总后返回结果。",
		Skills:      []string{"web_search", "web_fetch", "deep_research"},
		SystemNote:  "你是研究执行代理。使用搜索工具收集信息，汇总为结构化摘要后返回。",
	})
	handoffReg.Register(subagent.HandoffConfig{
		Name:        "general_exec",
		Description: "通用执行代理：只处理轻量通用任务，例如翻译、图片生成、邮件发送；不要用于文件解析、代码执行、浏览器操作或联网研究。",
		Skills:      []string{"translate", "image_generate", "image_gen", "send_email"},
		SystemNote:  "你是通用执行代理。你只能处理翻译、图片生成、邮件发送等轻量通用任务。遇到文件解析、代码执行、浏览器或联网研究，应说明需要交给对应专用代理，而不是自己尝试。",
	})

	p.SetHandoffRegistry(handoffReg)
	gw.SetHandoffRegistry(handoffReg)
}

func initSkillRegistration(app *agentrt.App, gw *gateway.Gateway, searchReg *websearch.Registry, knowledgeStore *knowledge.Store) {
	cfg := app.Config
	p := app.Planner

	app.SkillRegistry.Register(&document.MedicalExcelSplitSkill{})
	app.SkillRegistry.Register(&document.PythonInterpreterSkill{})

	imgGenSkill := imagegen.NewImageGenerateSkill(app.Providers.GetImageGenerator(), cfg.DataPath("output"))
	app.SkillRegistry.Register(imgGenSkill)
	slog.Info("image_generate skill registered", "has_generator", app.Providers.GetImageGenerator() != nil)

	if hub := gw.BrowserHub(); hub != nil {
		browserCtrl := browserskill.NewHubAdapter(hub)
		browserskill.RegisterSkills(app.SkillRegistry, browserCtrl)
		slog.Info("browser extension skills registered", "count", 15)
	}

	connReg := connectors.NewRegistry()
	for _, def := range connectors.PresetDefs() {
		connReg.RegisterDef(def)
	}
	connReg.RegisterHandler("github", connectors.NewGitHubHandler())
	connReg.RegisterHandler("notion", connectors.NewNotionHandler())
	connReg.RegisterHandler("slack", connectors.NewSlackHandler())
	connReg.RegisterHandler("linear", connectors.NewLinearHandler())
	if jiraURL := os.Getenv("JIRA_BASE_URL"); jiraURL != "" {
		connReg.RegisterHandler("jira", connectors.NewJiraHandler(jiraURL))
		slog.Info("jira connector enabled", "base_url", jiraURL)
	}
	connReg.RegisterHandler("gmail", connectors.NewGmailFullHandler())
	connReg.RegisterHandler("google_calendar", connectors.NewGoogleCalendarFullHandler())
	connReg.RegisterHandler("outlook_mail", connectors.NewOutlookMailFullHandler())
	connReg.RegisterHandler("outlook_calendar", connectors.NewOutlookCalendarFullHandler())
	connReg.LoadPersisted(context.Background())
	gw.SetConnectorRegistry(connReg)
	connSkillCount := connectors.RegisterSkills(app.SkillRegistry, connReg)
	slog.Info("connector registry initialized", "defs", len(connectors.PresetDefs()), "skills", connSkillCount)

	var researchBrowser research.BrowserCtrl
	if hub := gw.BrowserHub(); hub != nil {
		researchBrowser = browserskill.NewHubAdapter(hub)
	}
	deepResearch := research.NewDeepResearchSkill(searchReg, app.LLMClient, researchBrowser, cfg.DataPath("output"))
	app.SkillRegistry.Register(deepResearch)
	slog.Info("deep_research skill registered")

	app.SkillRegistry.Register(filegen.NewFileGenSkill(cfg.DataPath("output")))
	slog.Info("file_generate skill registered")

	mineruClient := mineru.NewFromConfig(cfg)
	gw.SetMinerUClient(mineruClient)
	docParseSkill := docparse.NewDocumentParseSkill(mineruClient, cfg.DataPath("output"), knowledgeStore)
	app.SkillRegistry.Register(docParseSkill)
	slog.Info("document_parse skill registered", "enabled", mineruClient.Enabled(), "backend", cfg.MinerUBackend)

	// ── Skill Categories ──
	app.SkillRegistry.DefineCategory(skills.SkillCategory{
		ID:          "browser",
		Name:        "浏览器",
		Description: "Control the browser: navigate, click, input, scroll, screenshot, mark elements, manage tabs, extract content.",
	})
	for _, s := range []string{
		"browser_navigate", "browser_click", "browser_input", "browser_scroll",
		"browser_press_key", "browser_get_content", "browser_screenshot",
		"browser_move_mouse", "browser_get_elements", "browser_mark_elements",
		"browser_unmark_elements", "browser_list_tabs", "browser_switch_tab",
		"browser_new_tab", "browser_close_tab",
	} {
		app.SkillRegistry.AssignCategory(s, "browser")
	}

	app.SkillRegistry.DefineCategory(skills.SkillCategory{
		ID:          "connector",
		Name:        "连接器",
		Description: "Interact with connected third-party services (GitHub, Gmail, Calendar, Notion, Slack, etc.).",
	})
	for _, s := range app.SkillRegistry.All() {
		if strings.HasPrefix(s.Name(), "connector_") {
			app.SkillRegistry.AssignCategory(s.Name(), "connector")
		}
	}

	totalSkills := len(app.SkillRegistry.All())
	totalCats := len(app.SkillRegistry.Categories())
	uncategorized := len(app.SkillRegistry.UncategorizedSkills())
	slog.Info("skill hierarchy configured",
		"total_skills", totalSkills,
		"categories", totalCats,
		"uncategorized", uncategorized,
		"hierarchical", totalSkills > 25 && totalCats > 0,
	)

	p.InvalidatePromptCache()
}
