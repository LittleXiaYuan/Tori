package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sort"
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
	"yunque-agent/internal/agentcore/tasksched/rlsched"
	"yunque-agent/internal/agentcore/taskskills/docparse"
	"yunque-agent/internal/agentcore/taskskills/filegen"
	"yunque-agent/internal/agentcore/taskskills/imagegen"
	"yunque-agent/internal/agentcore/taskskills/research"
	"yunque-agent/internal/agentcore/websearch"
	"yunque-agent/internal/appdir"
	"yunque-agent/internal/cognicore/recommend"
	"yunque-agent/internal/connectors"
	"yunque-agent/internal/controlplane/gateway"
	"yunque-agent/internal/controlplane/tenant"
	"yunque-agent/internal/execution/channel"
	reflectpkg "yunque-agent/internal/experimental/reflect"
	"yunque-agent/internal/integrations/mineru"
	iledger "yunque-agent/internal/ledger"
	backuppack "yunque-agent/internal/packs/backup"
	browserintentpack "yunque-agent/internal/packs/browserintent"
	chaosprobepack "yunque-agent/internal/packs/chaosprobe"
	cognitivecanarypack "yunque-agent/internal/packs/cognitivecanary"
	computerusepack "yunque-agent/internal/packs/computeruse"
	guardrailfuzzerpack "yunque-agent/internal/packs/guardrailfuzzer"
	lorapack "yunque-agent/internal/packs/lora"
	memorytimetravelpack "yunque-agent/internal/packs/memorytimetravel"
	rpareplaypack "yunque-agent/internal/packs/rpareplay"
	sbomdriftpack "yunque-agent/internal/packs/sbomdrift"
	skillanomalypack "yunque-agent/internal/packs/skillanomaly"
	wasmpluginpack "yunque-agent/internal/packs/wasmplugin"
	"yunque-agent/pkg/document"
	"yunque-agent/pkg/packruntime"
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

	// wasm-plugin pack: the remote-install executor (blueprint Appendix B) is
	// off unless an operator sets WASM_REMOTE_INSTALL_ENFORCE=true. Its trusted
	// installer is injected after the Gateway exists (SetInstallFromCache below).
	wasmPluginPack := wasmpluginpack.New(wasmpluginpack.Config{
		PluginDir:            cfg.DataPath("plugins"),
		DataDir:              cfg.DataPath("wasm-plugin"),
		RemoteInstallEnforce: strings.EqualFold(strings.TrimSpace(os.Getenv("WASM_REMOTE_INSTALL_ENFORCE")), "true"),
	})

	// ── Phase 2: Gateway ──
	gw := gateway.NewFromConfig(gateway.GatewayConfig{
		Planner:   p,
		Tenants:   tenantMgr,
		Memory:    app.MemManager,
		Skills:    app.SkillRegistry,
		Scheduler: sa.sched,
		ConvStore: sa.convStore,
		Plugins:   app.PluginReg,
		FeishuAPI: sa.feishuAPI,
		Learning:  learningLoop,
		JWTConfig: sa.jwtCfg,
		Metrics:   app.Metrics,
		Pipeline:  app.MemPipeline,
		Persona:   botPersona,
		BackendPacks: []packruntime.BackendModule{
			backuppack.DefaultHandler(),
			chaosprobepack.New(chaosprobepack.Config{DataDir: cfg.DataPath("chaos-probe")}),
			cognitivecanarypack.New(cognitivecanarypack.Config{DataDir: cfg.DataPath("cognitive-canary")}),
			guardrailfuzzerpack.New(guardrailfuzzerpack.Config{DataDir: cfg.DataPath("guardrail-fuzzer")}),
			memorytimetravelpack.New(memorytimetravelpack.Config{
				DataDir:                  cfg.DataPath("memory-time-travel"),
				TemporalKV:               memoryTimeTravelTemporalKV(app),
				NativeKVHistoryPreviewer: memoryTimeTravelNativeKVHistoryPreviewer(app),
				MemoryPersisterWriteback: memoryPersisterTemporalWritebackReady(app),
				MerkleVerifier:           memoryTimeTravelMerkleVerifier(app),
			}),
			rpareplaypack.New(rpareplaypack.Config{DataDir: cfg.DataPath("rpa-replay")}),
			sbomdriftpack.New(sbomdriftpack.Config{RepoRoot: ".", DataDir: cfg.DataPath("sbom-drift")}),
			skillanomalypack.New(skillanomalypack.Config{DataDir: cfg.DataPath("skill-anomaly")}),
			wasmPluginPack,
		},
	})
	// Inject the trusted remote-install installer now that the Gateway exists
	// (it resolves the pack registry + trust root lazily at request time).
	// InstallFromYqpack re-verifies SHA-256 + Ed25519 signature and extracts/
	// registers atomically — fail closed, nothing installed on any failure.
	wasmPluginPack.SetInstallFromCache(func(_ context.Context, cachePath string) error {
		reg := gw.PackRegistry()
		if reg == nil {
			return fmt.Errorf("pack registry not available")
		}
		_, err := reg.InstallFromYqpack(cachePath, packruntime.InstallOptions{
			TrustRoot: gw.PackTrustRoot(),
			Source:    "yqpack-remote",
		})
		return err
	})
	gw.SetPlannerResumeJobStore(cfg.DataPath("planner", "resume_plan_jobs.jsonl"))
	packRegistry, err := packruntime.NewRegistry(cfg.DataPath("packs"))
	if err != nil {
		slog.Warn("pack runtime registry disabled", "err", err)
	} else {
		ensureBuiltinPacks(packRegistry)
		gw.SetPackRegistry(packRegistry)
		trustRoot := packruntime.NewTrustRoot(cfg.DataPath("packs"))
		if err := trustRoot.LoadDisk(); err != nil {
			slog.Warn("pack trust root disk keys not loaded", "err", err)
		}
		gw.SetPackTrustRoot(trustRoot)
		gw.SetPackCatalogSources(cfg.PackCatalogSourceDirs())
		app.Set(agentrt.CompPackRuntimeRegistry, packRegistry)
		slog.Info("pack runtime registry initialized", "dir", cfg.DataPath("packs"), "installed", len(packRegistry.List()), "catalog_sources", cfg.PackCatalogSourceDirs())

		// Optional pack-UI isolation listener: serving DLC bundles from their
		// own loopback port gives iframes a real cross-origin boundary to the
		// shell (defense in depth beyond the sandbox attribute). Opt-in because
		// the packaged desktop webview CSP must whitelist the extra origin.
		if addr := strings.TrimSpace(os.Getenv("PACK_UI_ADDR")); addr != "" {
			if _, packUISrv, uiErr := gw.StartPackUIServer(addr); uiErr != nil {
				slog.Warn("pack ui isolation listener failed", "addr", addr, "err", uiErr)
			} else {
				app.Lifecycle.RegisterFunc("pack_ui_server",
					func(context.Context) error { return nil },
					func(ctx context.Context) error { return packUISrv.Shutdown(ctx) })
			}
		}
	}
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

	loraScheduler := appLoRAScheduler(app)
	trainingMetrics := appTrainingMetrics(app)
	evolutionCoordinator := appEvolutionCoordinator(app)
	gw.SetLoRAScheduler(loraScheduler)
	gw.SetTrainingMetrics(trainingMetrics)
	gw.SetEvolutionCoordinator(evolutionCoordinator)
	// Browser Intent + Computer Use + LoRA migrated to the v2 Module lifecycle
	// (Tier 0 microkernel). Computer Use is manifest-default-disabled: it exposes
	// status/plan/read-only browser screenshot only when explicitly enabled.
	_ = gw.RegisterModule(browserintentpack.NewHandler(gw))
	_ = gw.RegisterModule(computerusepack.New(gw))
	_ = gw.RegisterModule(lorapack.NewHandler(lorapack.Options{
		Scheduler: loraScheduler,
		Metrics:   trainingMetrics,
		Evolution: evolutionCoordinator,
		Distill:   appSelfDistillPipeline(app),
	}))

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
	cfg := app.Config
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
	if strings.HasPrefix(initExecProvider, "local-") && !cfg.LocalModelsEnabled {
		slog.Info("exec provider ignored because local models are disabled", "provider", initExecProvider)
		initExecProvider = "smart"
	}
	if initExecProvider != "smart" && initExecProvider != "" && app.Providers != nil {
		provider := app.Providers.Get(initExecProvider)
		if provider == nil || !provider.Enabled() {
			slog.Warn("exec provider unavailable, falling back to smart", "provider", initExecProvider)
			initExecProvider = "smart"
		}
	}
	gw.SetExecProvider(initExecProvider)
	slog.Info("exec agent provider configured", "provider", initExecProvider)

	// file_exec generates whole documents (PPT/Word) in a single long LLM turn.
	// A slow reasoning model (e.g. gpt-5.5) stalls tens of seconds before the
	// first byte and repeatedly blew past the handoff timeout mid-generation —
	// the roadshow demo's main failure mode. Route document generation to the
	// fast (non-reasoning) tier when one is configured; fall back to the exec
	// provider otherwise so nothing breaks when no fast model exists.
	fileExecProvider := strings.TrimSpace(os.Getenv("FILE_EXEC_PROVIDER"))
	if fileExecProvider == "" && app.LLMPool != nil && app.LLMPool.Has("fast") {
		fileExecProvider = "fast"
	}
	if fileExecProvider != "" {
		slog.Info("file_exec provider configured (split from main brain)", "provider", fileExecProvider)
	}

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
		ProviderID:  fileExecProvider,
		SystemNote:  "你是文件执行代理。你只能使用文件/文档相关工具完成任务；不要调用浏览器、搜索网页或代码执行。若输入里已经包含 [Parsed document] 或文档正文，直接整理内容并回答；若只有文件名，先用 file_open/file_search 在工作区查找和读取。\n\n生成纪律：若一次需要产出多份文档（例如同时要 PPT 和 Word），逐份生成——先完整做完一份、调用对应生成工具落盘，再开始下一份；单轮不要并行拼装多份大文档，避免一次生成内容过长导致超时。",
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

func appLoRAScheduler(app *agentrt.App) *localbrain.LoRAScheduler {
	if v, ok := app.Get("lora_scheduler"); ok {
		if s, ok := v.(*localbrain.LoRAScheduler); ok {
			return s
		}
	}
	return nil
}

func appTrainingMetrics(app *agentrt.App) *localbrain.TrainingMetrics {
	if v, ok := app.Get("training_metrics"); ok {
		if m, ok := v.(*localbrain.TrainingMetrics); ok {
			return m
		}
	}
	return nil
}

func appEvolutionCoordinator(app *agentrt.App) *localbrain.EvolutionCoordinator {
	if v, ok := app.Get("evolution_coordinator"); ok {
		if ec, ok := v.(*localbrain.EvolutionCoordinator); ok {
			return ec
		}
	}
	return nil
}

func appSelfDistillPipeline(app *agentrt.App) *localbrain.SelfDistillPipeline {
	if v, ok := app.Get("self_distill_pipeline"); ok {
		if p, ok := v.(*localbrain.SelfDistillPipeline); ok {
			return p
		}
	}
	return nil
}

func memoryPersisterTemporalWritebackReady(app *agentrt.App) bool {
	if app == nil {
		return false
	}
	persister, ok := app.Get(agentrt.CompMemPersister)
	if !ok {
		return false
	}
	ready, ok := persister.(interface{ TemporalWritebackReady() bool })
	return ok && ready.TemporalWritebackReady()
}

func memoryTimeTravelTemporalKV(app *agentrt.App) *iledger.TemporalKVStore {
	if app == nil || app.Ledger == nil {
		return nil
	}
	return iledger.NewTemporalKVStore(app.Ledger)
}

type memoryTimeTravelKVHistoryPreviewAdapter struct {
	store *iledger.TemporalKVStore
}

func memoryTimeTravelNativeKVHistoryPreviewer(app *agentrt.App) memorytimetravelpack.NativeKVHistoryPreviewer {
	store := memoryTimeTravelTemporalKV(app)
	if store == nil {
		return nil
	}
	return memoryTimeTravelKVHistoryPreviewAdapter{store: store}
}

func (a memoryTimeTravelKVHistoryPreviewAdapter) PreviewNativeKVHistoryRows(ctx context.Context, namespace string, limit int) (memorytimetravelpack.NativeKVHistoryMigrationPreview, error) {
	if a.store == nil {
		return memorytimetravelpack.NativeKVHistoryMigrationPreview{}, errors.New("memory time travel native kv_history preview adapter is not attached")
	}
	preview, err := a.store.PreviewNativeKVHistoryRows(ctx, namespace, limit)
	if err != nil {
		return memorytimetravelpack.NativeKVHistoryMigrationPreview{}, err
	}
	rows := make([]memorytimetravelpack.NativeKVHistoryRowPreview, 0, len(preview.Rows))
	for _, row := range preview.Rows {
		rows = append(rows, memorytimetravelpack.NativeKVHistoryRowPreview{
			ID:            row.ID,
			Namespace:     row.Namespace,
			Key:           row.Key,
			Version:       row.Version,
			Value:         append([]byte(nil), row.Value...),
			ValueSHA256:   row.ValueSHA256,
			UpdatedAt:     row.UpdatedAt,
			ArchivedAt:    row.ArchivedAt,
			Current:       row.Current,
			AuditSeq:      row.AuditSeq,
			AuditHash:     row.AuditHash,
			SourceAdapter: row.SourceAdapter,
		})
	}
	return memorytimetravelpack.NativeKVHistoryMigrationPreview{
		Namespace:               preview.Namespace,
		GeneratedAt:             preview.GeneratedAt,
		SourceNamespace:         preview.SourceNamespace,
		NativeTable:             preview.NativeTable,
		ScannedDocumentCount:    preview.ScannedDocumentCount,
		PreviewRowCount:         preview.PreviewRowCount,
		ReturnedRowCount:        preview.ReturnedRowCount,
		Limit:                   preview.Limit,
		WritesNativeKVHistory:   preview.WritesNativeKVHistory,
		MigratesKVHistory:       preview.MigratesKVHistory,
		UsesReservedKVNamespace: preview.UsesReservedKVNamespace,
		Rows:                    rows,
		Notes:                   append([]string{}, preview.Notes...),
	}, nil
}

func memoryTimeTravelMerkleVerifier(app *agentrt.App) memorytimetravelpack.MerkleVerifier {
	return memorytimetravelpack.MerkleVerifierFunc(func(_ context.Context, limit int) (memorytimetravelpack.MerkleVerification, error) {
		checkedAt := time.Now().UTC()
		if app == nil {
			return memorytimetravelpack.MerkleVerification{
				Ready:        false,
				Valid:        false,
				InvalidIndex: -1,
				CheckedAt:    checkedAt,
				Notes:        []string{"App runtime is not available; Merkle audit-chain verification cannot run."},
			}, nil
		}
		value, ok := app.Get(agentrt.CompAuditChain)
		if !ok || value == nil {
			return memorytimetravelpack.MerkleVerification{
				Ready:        false,
				Valid:        false,
				InvalidIndex: -1,
				CheckedAt:    checkedAt,
				Notes:        []string{"Audit chain component is not initialized yet; retry after runtime bootstrap completes."},
			}, nil
		}
		chain, ok := value.(*audit.Chain)
		if !ok || chain == nil {
			return memorytimetravelpack.MerkleVerification{
				Ready:        false,
				Valid:        false,
				InvalidIndex: -1,
				CheckedAt:    checkedAt,
				Notes:        []string{"Audit chain component has an unexpected runtime type."},
			}, nil
		}
		invalidIndex := chain.Verify()
		last := chain.Last()
		out := memorytimetravelpack.MerkleVerification{
			Ready:        true,
			Valid:        invalidIndex == -1,
			InvalidIndex: invalidIndex,
			RecordCount:  chain.Len(),
			CheckedAt:    checkedAt,
			Notes:        []string{"Read-only verification checks the in-memory Merkle audit chain; historical file rehydration and per-KV proof linkage remain separate follow-up work."},
		}
		if last != nil {
			out.LastHash = last.Hash
			out.LastSeq = last.Seq
		}
		for _, rec := range chain.Tail(limit) {
			out.RecentRecords = append(out.RecentRecords, memorytimetravelpack.MerkleAuditRecord{
				Seq:       rec.Seq,
				Timestamp: rec.Timestamp,
				Type:      string(rec.Type),
				Actor:     rec.Actor,
				Action:    rec.Action,
				PrevHash:  rec.PrevHash,
				Hash:      rec.Hash,
			})
		}
		return out, nil
	})
}

func ensureBuiltinPacks(registry *packruntime.Registry) {
	if registry == nil {
		return
	}
	// Pack refactor — Phase A, brick 1: the builtin pack set is now declared by
	// the on-disk manifests under packs/official/ and discovered dynamically,
	// instead of a hardcoded path list. This is the first step toward "every
	// surface is a manifest-declared pack". Discovery falls back to the known
	// official set if the directory cannot be located (unusual install layout).
	manifestPaths := discoverBuiltinPackManifestPaths()
	if len(manifestPaths) == 0 {
		manifestPaths = fallbackBuiltinPackManifestPaths()
		slog.Warn("builtin pack discovery found no manifests; using fallback set", "count", len(manifestPaths))
	}
	for _, manifestPath := range manifestPaths {
		manifest, err := loadBuiltinPackManifest(manifestPath)
		if err != nil {
			slog.Warn("builtin pack manifest skipped", "path", manifestPath, "err", err)
			continue
		}
		if _, ok := registry.Get(manifest.ID); ok {
			continue
		}
		if _, err := registry.Install(manifest, manifestPath); err != nil {
			slog.Warn("builtin pack install failed", "id", manifest.ID, "path", manifestPath, "err", err)
			continue
		}
		slog.Info("builtin pack manifest installed", "id", manifest.ID, "state", manifest.DefaultState)
	}
}

// builtinPackExcluded lists official pack directories that must NOT be
// auto-seeded as builtins. dlc-demo-pack ships as a manual-install DLC
// reference example, so it stays opt-in even though it lives under packs/official.
var builtinPackExcluded = map[string]bool{
	"dlc-demo-pack": true,
}

// discoverBuiltinPackManifestPaths scans packs/official/*/pack.json so the
// builtin pack set is sourced from on-disk manifests rather than a hardcoded
// list. Returns repo-relative manifest paths in deterministic order; empty when
// the directory cannot be located (the caller then uses the fallback set).
func discoverBuiltinPackManifestPaths() []string {
	dir := locateBuiltinPackDir()
	if dir == "" {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		slog.Warn("builtin pack discovery: read dir failed", "dir", dir, "err", err)
		return nil
	}
	var paths []string
	for _, entry := range entries {
		if !entry.IsDir() || builtinPackExcluded[entry.Name()] {
			continue
		}
		if _, statErr := os.Stat(filepath.Join(dir, entry.Name(), "pack.json")); statErr != nil {
			continue
		}
		paths = append(paths, "packs/official/"+entry.Name()+"/pack.json")
	}
	sort.Strings(paths)
	return paths
}

// locateBuiltinPackDir returns the first existing packs/official directory among
// the same candidate roots used to resolve individual builtin manifests.
func locateBuiltinPackDir() string {
	rel := filepath.Join("packs", "official")
	candidates := []string{rel, filepath.Join("..", "..", rel)}
	if _, file, _, ok := runtime.Caller(0); ok {
		repoRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
		candidates = append(candidates, filepath.Join(repoRoot, rel))
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			return c
		}
	}
	return ""
}

// fallbackBuiltinPackManifestPaths is the known first-party pack set, used only
// when on-disk discovery cannot locate packs/official (e.g. a trimmed install).
func fallbackBuiltinPackManifestPaths() []string {
	return []string{
		"packs/official/backup-pack/pack.json",
		"packs/official/channels-pack/pack.json",
		"packs/official/cogni-kernel-pack/pack.json",
		"packs/official/connectors-pack/pack.json",
		"packs/official/cost-pack/pack.json",
		"packs/official/desktop-pack/pack.json",
		"packs/official/federation-pack/pack.json",
		"packs/official/forks-pack/pack.json",
		"packs/official/heartbeat-pack/pack.json",
		"packs/official/identity-pack/pack.json",
		"packs/official/inner-life-pack/pack.json",
		"packs/official/mcp-dispatch-pack/pack.json",
		"packs/official/night-school-pack/pack.json",
		"packs/official/experience-pack/pack.json",
		"packs/official/world-model-pack/pack.json",
		"packs/official/micro-agent-pack/pack.json",
		"packs/official/notifications-pack/pack.json",
		"packs/official/orchestrator-pack/pack.json",
		"packs/official/persona-pack/pack.json",
		"packs/official/planner-recovery-pack/pack.json",
		"packs/official/rbac-pack/pack.json",
		"packs/official/reflection-pack/pack.json",
		"packs/official/retrieval-pack/pack.json",
		"packs/official/lora-pack/pack.json",
		"packs/official/browser-intent-pack/pack.json",
		"packs/official/computer-use-pack/pack.json",
		"packs/official/chaos-probe-pack/pack.json",
		"packs/official/cognitive-canary-pack/pack.json",
		"packs/official/guardrail-fuzzer-pack/pack.json",
		"packs/official/memory-time-travel-pack/pack.json",
		"packs/official/rpa-replay-pack/pack.json",
		"packs/official/market-pack/pack.json",
		"packs/official/scheduler-pack/pack.json",
		"packs/official/session-queue-pack/pack.json",
		"packs/official/skillhub-pack/pack.json",
		"packs/official/speech-pack/pack.json",
		"packs/official/modules-pack/pack.json",
		"packs/official/sbom-drift-pack/pack.json",
		"packs/official/skill-anomaly-pack/pack.json",
		"packs/official/subagents-pack/pack.json",
		"packs/official/tori-pack/pack.json",
		"packs/official/trace-pack/pack.json",
		"packs/official/wasm-plugin-pack/pack.json",
	}
}

func loadBuiltinPackManifest(manifestPath string) (packruntime.Manifest, error) {
	var lastErr error
	for _, candidate := range builtinPackManifestCandidates(manifestPath) {
		manifest, err := packruntime.LoadManifest(candidate)
		if err == nil {
			return manifest, nil
		}
		lastErr = err
	}
	return packruntime.Manifest{}, lastErr
}

func builtinPackManifestCandidates(manifestPath string) []string {
	candidates := []string{manifestPath, filepath.Join("..", "..", manifestPath)}
	if _, file, _, ok := runtime.Caller(0); ok {
		repoRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
		candidates = append(candidates, filepath.Join(repoRoot, manifestPath))
	}
	return candidates
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

	// file / image / research / workflow categories — assigned by skill-name
	// prefix so (1) the intent router can actually narrow these domains
	// (previously their keyword buckets were inert) and (2) AutoOrganizer can
	// group these skills into structured cognis (FromCapsules). Skills that match
	// no rule stay uncategorized = always available, so general tools like
	// code_execute / computer_use are never narrowed out.
	app.SkillRegistry.DefineCategory(skills.SkillCategory{
		ID:          "file",
		Name:        "文件与文档",
		Description: "Create, edit, convert and manage documents/files: Word/Excel/PPT/PDF, HTML export, archives, file read/write, document parsing.",
	})
	app.SkillRegistry.DefineCategory(skills.SkillCategory{
		ID:          "image",
		Name:        "图像",
		Description: "Generate and edit images and illustrations.",
	})
	app.SkillRegistry.DefineCategory(skills.SkillCategory{
		ID:          "research",
		Name:        "研究",
		Description: "Research the web and produce analyses/reports: web search, deep research.",
	})
	app.SkillRegistry.DefineCategory(skills.SkillCategory{
		ID:          "workflow",
		Name:        "工作流",
		Description: "Create, run and manage multi-step workflows and automations.",
	})
	for _, s := range app.SkillRegistry.All() {
		n := s.Name()
		if app.SkillRegistry.CategoryOf(n) != "" {
			continue // already in browser/connector
		}
		if cat := categorizeSkillName(n); cat != "" {
			app.SkillRegistry.AssignCategory(n, cat)
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

	// Surface intent-router drift: keyword buckets without a backing category are
	// inert (ScoreCategories skips them), so intent narrowing silently never
	// fires for them. Wiring those categories is a deliberate taxonomy decision.
	if dead := app.SkillRegistry.UnbackedIntentBuckets(); len(dead) > 0 {
		slog.Warn("skill intent router: keyword buckets have no matching category and are inert; define these categories to enable intent narrowing",
			"inert_buckets", dead)
	}

	p.InvalidatePromptCache()
}

// categorizeSkillName maps a skill name to one of the file/image/research/
// workflow intent categories by name prefix, or "" when it belongs to none
// (left uncategorized = always available). browser/connector are assigned by
// their own rules before this runs. Keeping this as a pure function makes the
// taxonomy unit-testable and the boundary explicit.
func categorizeSkillName(name string) string {
	switch {
	case hasAnyPrefix(name, "docx_", "xlsx_", "pptx_", "pdf_", "file_", "zip_", "deck_", "html_") ||
		name == "document_parse" || name == "doc_parse":
		return "file"
	case strings.HasPrefix(name, "image_"):
		return "image"
	case name == "deep_research" || name == "translate" ||
		strings.HasPrefix(name, "research_") || strings.HasPrefix(name, "web_"):
		return "research"
	case strings.Contains(name, "workflow") || name == "orchestrate_task":
		return "workflow"
	}
	return ""
}

func hasAnyPrefix(s string, prefixes ...string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}
