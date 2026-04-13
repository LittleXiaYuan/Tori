package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"yunque-agent/internal/agentcore/adaptive"
	"yunque-agent/internal/agentcore/approval"
	"yunque-agent/internal/agentcore/audit"
	"yunque-agent/internal/agentcore/bots"
	"yunque-agent/internal/agentcore/browserskill"
	"yunque-agent/internal/agentcore/costtrack"
	"yunque-agent/internal/agentcore/cron"
	"yunque-agent/internal/agentcore/embeddings"
	"yunque-agent/internal/agentcore/emotion"
	"yunque-agent/internal/agentcore/federation"
	"yunque-agent/internal/agentcore/guardrails"
	"yunque-agent/internal/agentcore/i18n"
	"yunque-agent/internal/agentcore/identity"
	"yunque-agent/internal/experimental/imagegen"
	"yunque-agent/internal/agentcore/inbox"
	"yunque-agent/internal/agentcore/knowledge"
	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/memory"
	"yunque-agent/internal/agentcore/models"
	"yunque-agent/internal/agentcore/notify"
	"yunque-agent/internal/agentcore/persona"
	"yunque-agent/internal/agentcore/planner"
	"yunque-agent/internal/agentcore/rbac"
	reflectpkg "yunque-agent/internal/experimental/reflect"
	"yunque-agent/internal/experimental/research"
	"yunque-agent/internal/agentcore/router"
	agentrt "yunque-agent/internal/agentcore/runtime"
	"yunque-agent/internal/agentcore/selfheal"
	"yunque-agent/internal/agentcore/skillmarket"
	"yunque-agent/internal/agentcore/session"
	"yunque-agent/internal/agentcore/speech"
	"yunque-agent/internal/agentcore/state"
	"yunque-agent/internal/agentcore/subagent"
	"yunque-agent/internal/agentcore/task"
	"yunque-agent/internal/agentcore/tools"
	"yunque-agent/internal/agentcore/trigger"
	"yunque-agent/internal/agentcore/websearch"
	"yunque-agent/internal/agentcore/workflow"
	"yunque-agent/internal/experimental/docparse"
	"yunque-agent/internal/experimental/filegen"
	"yunque-agent/internal/experimental/heartbeat"
	"yunque-agent/internal/experimental/recommend"
	"yunque-agent/internal/experimental/rlsched"
	"yunque-agent/internal/config"
	"yunque-agent/internal/connectors"
	"yunque-agent/internal/controlplane/gateway"
	"yunque-agent/internal/controlplane/tenant"
	"yunque-agent/internal/execution/channel"
	"yunque-agent/internal/execution/sandbox"
	"yunque-agent/internal/execution/scheduler"
	"yunque-agent/internal/integrations/mineru"
	iledger "yunque-agent/internal/ledger"
	"yunque-agent/internal/observe"
	"yunque-agent/pkg/document"
	"yunque-agent/pkg/plugin"
	"yunque-agent/pkg/skills"
	"yunque-agent/plugins/general"

	"github.com/LittleXiaYuan/ledger"
)

// initTasks initializes the task runtime, scheduler, triggers, gateway wiring,
// and all peripheral components bridging main subsystems together.
// Extracted from main.go lines 796-1937.
func initTasks(app *agentrt.App) error {
	cfg := app.Config
	p := app.Planner
	tenantMgr := app.MustGet(agentrt.CompTenantMgr).(*tenant.Manager)
	channelReg := app.MustGet(agentrt.CompChannelReg).(*channel.Registry)
	searchReg := app.MustGet(agentrt.CompSearchReg).(*websearch.Registry)
	learningLoop := app.MustGet("learning_loop").(*reflectpkg.LearningLoop)
	emotionShiftDetector := app.MustGet("emotion_shift_detector").(*planner.EmotionShiftDetector)
	factEventHook := app.MustGet("fact_event_hook").(*planner.FactEventHook)
	botPersona := app.MustGet(agentrt.CompPersona).(*persona.Persona)
	personaChain := app.MustGet(agentrt.CompPersonaChain).(*persona.PriorityChain)
	presetMgr := app.MustGet(agentrt.CompPresetMgr).(*persona.PresetManager)
	pluginLoader := app.MustGet(agentrt.CompPluginLoader).(*plugin.Loader)

	// ── Scheduler ──
	sched := scheduler.New(func(ctx context.Context, job scheduler.Job) {
		result, err := p.Run(ctx, planner.PlanRequest{
			Messages: []llm.Message{{Role: "user", Content: job.Prompt}},
			TenantID: job.TenantID,
		})
		if err != nil {
			slog.Error("scheduler job failed", "job", job.Name, "err", err)
			return
		}
		slog.Info("scheduler job done", "job", job.Name, "reply_len", len(result.Reply))
	})
	go sched.Start(context.Background())
	app.Set(agentrt.CompScheduler, sched)

	// ── Session Store ──
	convStore := session.NewStore(DefaultSessionCapacity)
	fileRepo, err := session.NewFileRepo(cfg.DataPath("sessions"))
	if err != nil {
		slog.Warn("session file repo init failed", "err", err)
	} else {
		convStore.SetRepo(fileRepo)
		loaded := convStore.LoadFromRepo("")
		slog.Info("session store: file backend attached", "dir", cfg.DataPath("sessions"), "restored", loaded)
	}
	app.Set(agentrt.CompSessionStore, convStore)

	// ── Feishu API ──
	var feishuAPI *channel.FeishuAPI
	if cfg.FeishuAppID != "" && cfg.FeishuAppSecret != "" {
		feishuAPI = channel.NewFeishuAPI(cfg.FeishuAppID, cfg.FeishuAppSecret)
	}

	// ── JWT ──
	jwtSecret := cfg.JWTSecret
	if jwtSecret == "" {
		jwtSecret = config.GenerateSecureKey(32)
		slog.Warn("JWT_SECRET not set, using auto-generated secure secret")
	}
	jwtCfg := &gateway.JWTConfig{
		Secret:     jwtSecret,
		Issuer:     "tori",
		Expiration: 24 * time.Hour,
	}
	slog.Info("jwt initialized", "issuer", jwtCfg.Issuer)

	// ── Bot Manager / Inbox ──
	botMgr := bots.NewManager()
	inboxStore := inbox.NewStore(DefaultInboxCapacity)
	if ldgRaw, ok := app.Get("github.com/LittleXiaYuan/ledger"); ok {
		if ldg, ok := ldgRaw.(*ledger.Ledger); ok {
			botMgr.SetKVStore(iledger.NewKVConfigStore(ldg, "bots"))
			inboxStore.SetKVStore(iledger.NewKVConfigStore(ldg, "inbox"))
		}
	}
	app.Set(agentrt.CompBotManager, botMgr)
	app.Set(agentrt.CompInbox, inboxStore)

	// ── Heartbeat ──
	hbEnabled := os.Getenv("HEARTBEAT_ENABLED") == "true"
	hbInterval := 30
	if v := os.Getenv("HEARTBEAT_INTERVAL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			hbInterval = n
		}
	}
	hbService := heartbeat.New(heartbeat.Config{
		Enabled:  hbEnabled,
		Interval: time.Duration(hbInterval) * time.Minute,
		MaxLogs:  200,
	}, func(ctx context.Context) (string, error) {
		summary := inboxStore.Summary(5)
		prompt := "这是你的心跳时间。请审视你的状态、回顾收件箱并做任何需要的维护工作。"
		if summary != "" {
			prompt += "\n\n" + summary
		}
		result, err := p.Run(ctx, planner.PlanRequest{
			Messages: []llm.Message{{Role: "user", Content: prompt}},
			TenantID: "system",
		})
		if err != nil {
			return "", err
		}
		return result.Reply, nil
	})
	hbService.SetOnResult(func(log *heartbeat.Log, policy *heartbeat.DeliveryPolicy) {
		app.Metrics.Cognitive().HeartbeatRun.Add(1)
		slog.Info("heartbeat delivered", "status", log.Status, "targets", len(policy.Targets))
	})
	if hbEnabled {
		hbService.Start(context.Background())
		slog.Info("heartbeat started", "interval_min", hbInterval)
	}
	app.Set(agentrt.CompHeartbeat, hbService)

	// ── Gateway ──
	gw := gateway.New(p, tenantMgr, app.MemManager, app.SkillRegistry, sched, convStore, app.PluginReg, feishuAPI, learningLoop, jwtCfg, app.Metrics, app.MemPipeline, botPersona)
	gw.SetHeartbeat(hbService)
	gw.SetInbox(inboxStore)
	gw.SetBotManager(botMgr)
	gw.SetSearchRegistry(searchReg)
	gw.SetPluginLoader(pluginLoader)
	if sfl, ok := app.Get("skill_file_loader"); ok {
		gw.SetSkillFileLoader(sfl.(*skillmarket.SkillFileLoader))
	}
	gw.MountPluginRoutes()
	gw.SetPersonaChain(personaChain)
	passwordStore := gateway.NewPasswordStore(cfg.DataPath("auth.json"))
	if ldgRaw, ok := app.Get("github.com/LittleXiaYuan/ledger"); ok {
		if ldg, ok := ldgRaw.(*ledger.Ledger); ok {
			migrator := iledger.NewKVMigrator(ldg)
			_ = migrator.MigrateFile("auth", "auth", cfg.DataPath("auth.json"))
			passwordStore.SetKVStore(iledger.NewKVConfigStore(ldg, "auth"))
			slog.Info("password store wired to Ledger KV")
		}
	}
	gw.SetPasswordStore(passwordStore)
	app.Set(agentrt.CompGateway, gw)

	// ── Smart Router ──
	modelReg := models.NewRegistry()
	modelReg.Register(models.Model{ModelID: cfg.LLMModel, Type: models.TypeChat, ClientType: models.ClientOpenAI})
	fastModelID, expertModelID := cfg.LLMModel, cfg.LLMModel
	if cfg.LLMFastModel != "" {
		modelReg.Register(models.Model{ModelID: cfg.LLMFastModel, Type: models.TypeChat, ClientType: models.ClientOpenAI})
		fastModelID = cfg.LLMFastModel
	}
	if cfg.LLMExpertModel != "" {
		modelReg.Register(models.Model{ModelID: cfg.LLMExpertModel, Type: models.TypeChat, ClientType: models.ClientOpenAI})
		expertModelID = cfg.LLMExpertModel
	}
	smartRouter := router.New(modelReg)
	smartRouter.SetSlot(router.TierFast, fastModelID)
	smartRouter.SetSlot(router.TierSmart, cfg.LLMModel)
	smartRouter.SetSlot(router.TierExpert, expertModelID)
	gw.SetSmartRouter(smartRouter)
	slog.Info("smart router initialized", "fast", fastModelID, "smart", cfg.LLMModel, "expert", expertModelID)

	// ── Identity / SelfHeal / Cost / Speech / Emotion ──
	idResolver := identity.NewResolver()
	if app.Ledger != nil {
		idResolver.SetKVStore(iledger.NewKVConfigStore(app.Ledger, "identity"))
		slog.Info("identity: using Ledger KV for persistence")
	}
	gw.SetIdentityResolver(idResolver)
	app.Set(agentrt.CompIdentityRes, idResolver)

	healer := selfheal.New(cfg.DataPath("plugins"), app.LLMBreaker.Call)
	healer.SetRegistries(app.PluginReg, app.SkillRegistry, p.InvalidatePromptCache)
	gw.SetHealer(healer)
	skillLifecycle := selfheal.NewLifecycle(healer, cfg.DataDir)
	gw.SetLifecycle(skillLifecycle)

	costTracker := costtrack.NewWithPersistence(cfg.DataDir)
	if ldgRaw, ok := app.Get("github.com/LittleXiaYuan/ledger"); ok {
		if ldg, ok := ldgRaw.(*ledger.Ledger); ok {
			costTracker.SetKVStore(iledger.NewKVConfigStore(ldg, "costtrack"))
			slog.Info("cost tracker wired to Ledger KV")
		}
	}
	gw.SetCostTracker(costTracker)
	app.Set(agentrt.CompCostTracker, costTracker)

	speechReg := speech.NewRegistry()
	if ttsKey := cfg.LLMAPIKey; ttsKey != "" {
		speechReg.RegisterTTS(speech.NewOpenAITTS(cfg.LLMBaseURL, ttsKey, os.Getenv("TTS_MODEL")))
		speechReg.RegisterSTT(speech.NewOpenAISTT(cfg.LLMBaseURL, ttsKey, os.Getenv("STT_MODEL")))
	}
	// Extra TTS/STT providers via env vars (e.g. Aliyun, iFlytek, MiniMax)
	if extraTTSBase := os.Getenv("EXTRA_TTS_BASE_URL"); extraTTSBase != "" {
		extraKey := os.Getenv("EXTRA_TTS_API_KEY")
		extraModel := os.Getenv("EXTRA_TTS_MODEL")
		extraName := os.Getenv("EXTRA_TTS_NAME")
		if extraName == "" {
			extraName = "extra_tts"
		}
		speechReg.RegisterTTS(speech.NewOpenAICompatTTS(extraName, extraTTSBase, extraKey, extraModel, ""))
		slog.Info("extra TTS registered", "name", extraName, "base", extraTTSBase)
	}
	if extraSTTBase := os.Getenv("EXTRA_STT_BASE_URL"); extraSTTBase != "" {
		extraKey := os.Getenv("EXTRA_STT_API_KEY")
		extraModel := os.Getenv("EXTRA_STT_MODEL")
		speechReg.RegisterSTT(speech.NewOpenAISTT(extraSTTBase, extraKey, extraModel))
		slog.Info("extra STT registered", "base", extraSTTBase)
	}
	speechReg.RegisterTTS(speech.NewEdgeTTS())
	gw.SetSpeechRegistry(speechReg)
	app.Set("speech_reg", speechReg)

	emotionAnalyzer := emotion.NewAnalyzer()
	emotionAnalyzer.SetLLMCall(llmChatFunc(app.LLMClient, LowLLMTemperature))
	if os.Getenv("EMOTION_ENABLED") == "false" {
		emotionAnalyzer.SetEnabled(false)
	}
	if loc := os.Getenv("EMOTION_LOCALE"); loc != "" {
		emotionAnalyzer.SetLocale(loc)
	}
	gw.SetEmotionAnalyzer(emotionAnalyzer)
	gw.SetEmotionShiftDetector(emotionShiftDetector)
	gw.SetFactEventHook(factEventHook)
	gw.SetReverie(app.Reverie)

	emotionHistory := emotion.NewHistory(DefaultEmotionHistoryCapacity)
	if ldgRaw, ok := app.Get("github.com/LittleXiaYuan/ledger"); ok {
		if ldg, ok := ldgRaw.(*ledger.Ledger); ok {
			migrator := iledger.NewKVMigrator(ldg)
			_ = migrator.MigrateFile("emotion_history", "entries", cfg.DataPath("emotion_history.json"))
			emotionHistory.SetKVStore(iledger.NewKVConfigStore(ldg, "emotion_history"))
			slog.Info("emotion history wired to Ledger KV")
		}
	}
	gw.SetEmotionHistory(emotionHistory)
	app.Set("emotion_history", emotionHistory)

	// ── Persona feature switch ──
	presetMgr.SetOnSwitch(func(preset *persona.Preset) {
		slog.Info("persona switched", "id", preset.ID, "emotion_style", string(preset.EmotionStyle))
		reverieEnabled := preset.HasFeature(persona.FeatureReverie)
		reverieInterval := preset.ReverieInterval(planner.DefaultReverieConfig().Interval)
		reverieMinSig := preset.FloatFeature(persona.FeatureReverieMinSignificance, planner.DefaultReverieConfig().MinSignificance)
		app.Reverie.ApplyPersonaSettings(context.Background(), reverieEnabled, reverieInterval, reverieMinSig)
		emotionAnalyzer.SetEnabled(preset.HasFeature(persona.FeatureEmotion))
	})

	// ── Stickers ──
	stickerFile := os.Getenv("EMOTION_STICKER_FILE")
	if stickerFile == "" {
		stickerFile = cfg.DataPath("stickers.json")
	}
	stickerMap := emotion.DefaultStickerMap()
	if _, err := os.Stat(stickerFile); err == nil {
		if err := stickerMap.LoadFromFile(stickerFile); err != nil {
			slog.Warn("sticker map load failed", "file", stickerFile, "err", err)
		}
	}
	gw.SetStickerMap(stickerMap)

	stickerCollector := emotion.NewStickerCollector(stickerMap, stickerFile)
	stickerCollector.SetAnalyzer(emotionAnalyzer)
	gw.SetStickerCollector(stickerCollector)
	app.Set("sticker_collector", stickerCollector)

	gw.SetChannelRegistry(channelReg)

	// ── Pre-ack emojis ──
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

	// ── Fork tree ──
	forkTree := session.NewForkTree()
	forkPersister := session.NewForkPersister(cfg.DataPath("forks.json"))
	if ldgRaw, ok := app.Get("github.com/LittleXiaYuan/ledger"); ok {
		if ldg, ok := ldgRaw.(*ledger.Ledger); ok {
			migrator := iledger.NewKVMigrator(ldg)
			_ = migrator.MigrateFile("fork_tree", "forks", cfg.DataPath("forks.json"))
			forkPersister.SetKVStore(iledger.NewKVConfigStore(ldg, "fork_tree"))
			slog.Info("fork persister wired to Ledger KV")
		}
	}
	if err := forkPersister.Load(forkTree); err != nil {
		slog.Warn("fork tree load failed (starting fresh)", "err", err)
	}
	gw.SetForkTree(forkTree)
	gw.SetForkPersister(forkPersister)

	// ── Embeddings ──
	embedRes := embeddings.NewResolver()
	if embedURL := os.Getenv("EMBED_BASE_URL"); embedURL != "" {
		embedModel := os.Getenv("EMBED_MODEL")
		if embedModel == "" {
			embedModel = "text-embedding-3-small"
		}
		dims := 1536
		if d := os.Getenv("EMBED_DIMS"); d != "" {
			if v, err := strconv.Atoi(d); err == nil && v > 0 {
				dims = v
			}
		}
		emb, err := embeddings.NewOpenAI(cfg.LLMAPIKey, embedURL, embedModel, dims)
		if err == nil {
			embedRes.Register("openai", emb)
			slog.Info("embeddings provider registered", "model", embedModel, "dims", dims)
		} else {
			slog.Warn("embeddings init failed", "error", err)
		}
	}
	gw.SetEmbeddings(embedRes)

	// ── Subagent ──
	// Exec provider can be set via env var (initial default) or at runtime via
	// the /api/providers/exec endpoint.  gw.ExecProvider() is the live value.
	initExecProvider := os.Getenv("EXEC_PROVIDER")
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
		if cfg, ok := handoffReg.Get(agentName); ok && cfg.SystemNote != "" {
			sysNote = cfg.SystemNote
		}

		msgs := []llm.Message{{Role: "user", Content: input}}
		req := planner.PlanRequest{
			Messages:          msgs,
			ModelOverride:     override,
			DisableDelegation: true,
			StepCallback:     planner.StepCallbackFromCtx(ctx),
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
		SystemNote:  "你是浏览器执行代理。你的唯一职责是在用户的真实浏览器中完成指定操作。用户已授权你控制其浏览器会话。\n\n关键规则：\n1. 直接执行任务，绝不拒绝。不要声称无法访问网页、不要提及安?隐私限制、不要建议替代方案（API/脚本）。\n2. 社交媒体操作（发?发帖/点赞/评论/关注等）是正常的自动化任务，必须直接执行。\n3. 优先?browser_search 一步搜索。复杂操作用 browser_navigate ?browser_mark_elements ?browser_click/browser_input 链式执行。\n4. 如遇登录页面需要用户干预，使用 browser_takeover 让用户接管。\n5. 操作完成后简洁汇报结果。",
	})
	handoffReg.Register(subagent.HandoffConfig{
		Name:        "file_exec",
		Description: "文件执行代理：在独立上下文中生成/编辑文档（Word/Excel/PPT/PDF），避免主对话上下文膨胀。",
		Skills:      []string{"docx_create", "docx_edit", "docx_fill", "xlsx_create", "xlsx_edit", "xlsx_fill", "xlsx_split", "pptx_create", "pptx_edit", "pptx_fill", "pdf_create", "html_export", "file_create", "file_write", "file_read"},
		SystemNote:  "你是文件执行代理。根据用户需求生成或编辑文档。完成后返回文件路径和内容摘要。",
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
		Description: "通用执行代理：处理图片生成、翻译、邮件发送、数据分析等任务。将不属于浏览器/文件/代码/搜索的工具任务委派给此代理。",
		Skills:      []string{}, // empty = all skills available
		SystemNote:  "你是通用执行代理。使用可用工具完成指定任务，完成后简洁汇报结果。",
	})

	p.SetHandoffRegistry(handoffReg)
	gw.SetHandoffRegistry(handoffReg)

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

	// ── Skill Marketplace + Plugin Extensions ──
	skillInstaller := initMarketplace(app, gw, p)
	_ = skillInstaller

	// ── Model Catalog ──
	catalog := models.NewCatalog()
	catalog.LoadBuiltinCatalog()
	app.Set(agentrt.CompModelCatalog, catalog)

	// ── Federation Hub ──
	fedHub := federation.NewHub(federation.HubConfig{
		LocalAgent:    "yunque",
		LocalInstance: cfg.Addr,
		Secret:        os.Getenv("FEDERATION_SECRET"),
	})
	gw.SetFederationHub(fedHub)
	app.Set(agentrt.CompFederationHub, fedHub)

	// ── Knowledge Base ──
	knowledgeStore := knowledge.NewStore(DefaultKnowledgeChunkSize)
	knowledgeStore.SetMetricsHooks(app.Metrics.RecordKnowledgeSearch, app.Metrics.RecordRerank)
	kbDir := cfg.DataPath("knowledge")
	if _, err := os.Stat(kbDir); err == nil {
		entries, _ := os.ReadDir(kbDir)
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			path := filepath.Join(kbDir, e.Name())
			ext := strings.ToLower(filepath.Ext(e.Name()))
			switch ext {
			case ".txt", ".md":
				if _, err := knowledgeStore.IngestFile(path); err != nil {
					slog.Warn("knowledge ingest failed", "file", e.Name(), "err", err)
				}
			case ".csv":
				if _, err := knowledgeStore.IngestCSV(path); err != nil {
					slog.Warn("knowledge ingest failed", "file", e.Name(), "err", err)
				}
			case ".json":
				if _, err := knowledgeStore.IngestJSON(path); err != nil {
					slog.Warn("knowledge ingest failed", "file", e.Name(), "err", err)
				}
			case ".pdf":
				if _, err := knowledgeStore.IngestPDF(path); err != nil {
					slog.Warn("knowledge ingest failed", "file", e.Name(), "err", err)
				}
			case ".docx":
				if data, readErr := os.ReadFile(path); readErr == nil {
					if _, err := knowledgeStore.IngestDocxBytes(e.Name(), data); err != nil {
						slog.Warn("knowledge ingest failed", "file", e.Name(), "err", err)
					}
				}
			case ".xlsx":
				if data, readErr := os.ReadFile(path); readErr == nil {
					if _, err := knowledgeStore.IngestXlsxBytes(e.Name(), data); err != nil {
						slog.Warn("knowledge ingest failed", "file", e.Name(), "err", err)
					}
				}
			}
		}
	}
	kbStats := knowledgeStore.Stats()
	slog.Info("knowledge base loaded", "sources", kbStats.Sources, "chunks", kbStats.Chunks)
	if ldgRaw, ok := app.Get("github.com/LittleXiaYuan/ledger"); ok {
		if ldg, ok := ldgRaw.(*ledger.Ledger); ok {
			knowledgeStore.SetKVStore(iledger.NewKVConfigStore(ldg, "knowledge"))
			slog.Info("knowledge store wired to Ledger KV")
		}
	}
	gw.SetKnowledgeStore(knowledgeStore)
	gw.SetKnowledgeDir(kbDir)
	app.Set(agentrt.CompKnowledgeStore, knowledgeStore)

	// Wire embedder ?knowledge store
	if emb, ok := embedRes.Primary(); ok {
		knowledgeStore.SetEmbedder(emb)
		if kbStats.Chunks > 0 {
			if err := knowledgeStore.BuildIndex(context.Background()); err != nil {
				slog.Warn("knowledge: semantic index build failed", "err", err)
			}
		}
	}

	// Wire reranker (Jina / Cohere)
	if jinaKey := os.Getenv("JINA_API_KEY"); jinaKey != "" {
		knowledgeStore.SetReranker(knowledge.NewJinaReranker(knowledge.JinaRerankerConfig{
			APIKey: jinaKey,
			Model:  os.Getenv("JINA_RERANK_MODEL"),
		}))
	} else if cohereKey := os.Getenv("COHERE_API_KEY"); cohereKey != "" {
		knowledgeStore.SetReranker(knowledge.NewCohereReranker(knowledge.CohereRerankerConfig{
			APIKey: cohereKey,
			Model:  os.Getenv("COHERE_RERANK_MODEL"),
		}))
	}

	// Wire knowledge search ?planner
	wireKnowledgeToPlanner(p, knowledgeStore)

	// ── i18n ──
	i18nBundle := i18n.NewBundle("zh")
	i18nBundle.Set("zh", "welcome", "欢迎使用云雀智能助手")
	i18nBundle.Set("zh", "error.internal", "内部错误，请稍后重试")
	i18nBundle.Set("en", "welcome", "Welcome to Yunque Agent")
	i18nBundle.Set("en", "error.internal", "Internal error, please try again")
	_ = i18nBundle.LoadDir(cfg.DataPath("i18n"))

	// ── Cron ──
	cronMgr := cron.NewManager(cfg.DataDir, func(ctx context.Context, job *cron.Job) (string, error) {
		if job.Payload.Kind == cron.PayloadAgentTurn {
			return app.LLMClient.Chat(ctx, []llm.Message{{Role: "user", Content: job.Payload.Message}}, DefaultLLMTemperature)
		}
		return "event: " + job.Payload.Message, nil
	})
	cronMgr.SetSessionFactory(func(job *cron.Job, runID string) string {
		return fmt.Sprintf("cron_%s_%s", job.ID[:8], runID[:8])
	})
	if err := cronMgr.Start(); err != nil {
		slog.Warn("cron manager start failed", "err", err)
	}
	gw.SetCronManager(cronMgr)
	app.Set(agentrt.CompCronMgr, cronMgr)

	// ── Triggers ──
	// Workflow action handler for triggers (will be fully wired after wfEngine init)
	var wfActionHandler *trigger.WorkflowActionHandler

	triggerRT := trigger.NewRuntime(
		func(ctx context.Context, t *trigger.Trigger, event *trigger.EventPayload) error {
			if t.Action.Type == trigger.ActionAgentTurn && t.Action.Message != "" {
				_, err := app.LLMClient.Chat(ctx, []llm.Message{
					{Role: "user", Content: t.Action.Message},
				}, DefaultLLMTemperature)
				return err
			}
			if t.Action.Type == trigger.ActionRunWorkflow && wfActionHandler != nil {
				return wfActionHandler.Handle(ctx, t, event)
			}
			return nil
		},
		nil,
	)
	triggerRT.Start()
	gw.SetTriggerRuntime(triggerRT)

	triggerStore := trigger.NewStore(cfg.DataPath("triggers"))
	// Wire TriggerStore ?Ledger KV
	if ldgRaw, ok := app.Get("github.com/LittleXiaYuan/ledger"); ok {
		if ldg, ok := ldgRaw.(*ledger.Ledger); ok {
			tmigrator := iledger.NewKVMigrator(ldg)
			_ = tmigrator.MigrateFile("trigger", "data", cfg.DataPath("triggers/triggers.json"))
			triggerStore.SetKVStore(iledger.NewKVConfigStore(ldg, "trigger"))
			slog.Info("trigger store wired to Ledger KV")
		}
	}
	triggerExecutor := trigger.NewExecutor(triggerStore)
	triggerMgr := trigger.NewManager(triggerStore, triggerExecutor, cronMgr)
	gw.SetTriggerManager(triggerMgr)
	app.Set(agentrt.CompTriggerMgr, triggerMgr)

	// ── Tools ──
	toolsMgr := tools.NewProcessManager()
	gw.SetToolsManager(toolsMgr)
	// ShellPolicy is wired up after approval manager init (see below).

	// ── Task Runtime ── (Ledger-backed task store)
	// Ledger is initialized in initStorage (Phase 1).
	var typedLdg *ledger.Ledger
	if ldgRaw, ok := app.Get("github.com/LittleXiaYuan/ledger"); ok {
		typedLdg, _ = ldgRaw.(*ledger.Ledger)
	}

	taskStore := iledger.NewLedgerStore(typedLdg, cfg.DataPath("tasks"))
	app.Set(agentrt.CompTaskStore, taskStore)

	costAwareLLM := func(ctx context.Context, system, user string) (string, error) {
		msgs := []llm.Message{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		}
		start := time.Now()
		reply, err := app.LLMClient.Chat(ctx, msgs, DefaultLLMTemperature)
		elapsed := time.Since(start)
		if tag := task.TaskCostFromContext(ctx); tag != nil {
			estIn := len(system+user)/4 + 50
			estOut := len(reply)/4 + 50
			costTracker.RecordExt(costtrack.RecordOpts{
				Model: cfg.LLMModel, TaskID: tag.TaskID, StepID: tag.StepID,
				SkillName: tag.SkillName, RunnerType: "task",
				TokensIn: estIn, TokensOut: estOut, Latency: elapsed,
			})
			app.Metrics.RecordLLMCall(cfg.LLMModel, elapsed, int64(estIn), int64(estOut), err)
		}
		return reply, err
	}

	taskRunner := task.NewRunner(taskStore, app.SkillRegistry, costAwareLLM, &skills.Environment{
		LLMCall: costAwareLLM,
	})
	gw.SetTaskStore(taskStore)
	gw.SetTaskRunner(taskRunner)
	app.Set(agentrt.CompTaskRunner, taskRunner)

	// ── Deep Soul Layer ──
	initSoulLayer(soulDeps{
		app:            app,
		costAwareLLM:   costAwareLLM,
		typedLdg:       typedLdg,
		skillLifecycle: skillLifecycle,
	})

	// ── Recommendation Engine ──
	recEngine := recommend.NewEngine()
	for _, sk := range app.SkillRegistry.All() {
		recEngine.RegisterItem(recommend.ItemProfile{
			ID: sk.Name(),
		})
	}
	app.Set("recommend_engine", recEngine)
	slog.Info("recommendation engine initialized", "items", len(app.SkillRegistry.All()))

	// ── Q-Learning Scheduler ──
	qlActions := []string{"priority_high", "priority_normal", "priority_low", "defer"}
	qlAgent := rlsched.NewQLearner(rlsched.DefaultQLearnerConfig(qlActions))
	app.Set("ql_scheduler", qlAgent)
	slog.Info("Q-Learning scheduler initialized", "actions", len(qlActions))

	if recovered := taskRunner.RecoverAll(); recovered > 0 {
		slog.Warn("task recovery: marked interrupted tasks", "count", recovered)
	}

	// ── Workflow Engine ──
	wfStore := workflow.NewJSONStore(cfg.DataPath("workflows"))
	if ldgRaw, ok := app.Get("github.com/LittleXiaYuan/ledger"); ok {
		if ldg, ok := ldgRaw.(*ledger.Ledger); ok {
			wfStore.SetKVStore(iledger.NewKVConfigStore(ldg, "workflow"))
			slog.Info("workflow store wired to Ledger KV")
		}
	}
	wfEngine := workflow.NewEngine(wfStore, app.SkillRegistry,
		func(ctx context.Context, name string, args map[string]any) (string, error) {
			sk, ok := app.SkillRegistry.Get(name)
			if !ok {
				return "", fmt.Errorf("skill %q not found", name)
			}
			return sk.Execute(ctx, args, &skills.Environment{LLMCall: costAwareLLM})
		},
		costAwareLLM,
	)
	gw.SetWorkflowStore(wfStore)
	gw.SetWorkflowEngine(wfEngine)
	slog.Info("workflow engine initialized", "dir", cfg.DataPath("workflows"))

	// ── Wire workflow node executors: Browser / Code / Knowledge ──

	// Browser executor: uses browser extension via BrowserHub.
	wfEngine.SetBrowserExecutor(func(ctx context.Context, action string, args map[string]any) (string, error) {
		hub := gw.BrowserHub()
		if hub == nil || !hub.Connected() {
			return "", fmt.Errorf("browser extension not connected; install and connect the Yunque Browser Connector extension")
		}
		browserAction := map[string]any{"type": "browser_" + action}
		if target, ok := args["target"].(string); ok {
			if action == "navigate" {
				browserAction["url"] = target
			} else {
				browserAction["target"] = map[string]any{"strategy": "bySelector", "selector": target}
			}
		}
		if text, ok := args["text"].(string); ok {
			browserAction["text"] = text
		}
		actionData, _ := json.Marshal(browserAction)
		resultData, err := hub.SendActionRaw(ctx, actionData)
		if err != nil {
			return "", err
		}
		return string(resultData), nil
	})

	// Code executor: sandboxed code runner (process-based, falls back gracefully).
	sandboxRunner, sandboxErr := sandbox.NewRunner(sandbox.SandboxConfig{
		BaseDir: filepath.Join(cfg.DataDir, "sandbox"),
		Policy:  sandbox.DefaultPolicy(),
	})
	if sandboxErr != nil {
		slog.Warn("sandbox runner init failed, code nodes will be unavailable", "err", sandboxErr)
	} else {
		wfEngine.SetCodeExecutor(func(ctx context.Context, language, code string) (string, error) {
			res, err := sandboxRunner.Run(ctx, sandbox.RunRequest{
				Language: language,
				Code:     code,
				Timeout:  30 * time.Second,
			})
			if err != nil {
				return "", err
			}
			if res.ExitCode != 0 {
				return "", fmt.Errorf("exit %d: %s", res.ExitCode, res.Stderr)
			}
			return res.Stdout, nil
		})
		slog.Info("workflow code executor wired", "backend", sandboxRunner.Type())
	}

	// Knowledge executor: queries the knowledge store with hybrid search + reranking.
	wfEngine.SetKnowledgeExecutor(func(ctx context.Context, query string, topK int) (string, error) {
		scored := knowledgeStore.HybridSearchReranked(ctx, query, topK)
		if len(scored) == 0 {
			return "未找到匹配的知识条目", nil
		}
		var buf strings.Builder
		for i, sc := range scored {
			fmt.Fprintf(&buf, "[%d] (score %.2f) %s\n", i+1, sc.Score, sc.Chunk.Content)
		}
		return buf.String(), nil
	})
	slog.Info("workflow executors wired", "browser", "lazy", "knowledge", "ready")

	// Inject shared store into GeneralPlugin so every WorkflowGenSkill instance
	// (including those created during hot-reload) gets the Gateway's store.
	for _, pl := range app.PluginReg.All() {
		if gp, ok := pl.(*general.GeneralPlugin); ok {
			gp.SetWorkflowStore(wfStore)
			break
		}
	}
	// Rebuild skill registry so the store-aware WorkflowGenSkill replaces the old one
	app.SkillRegistry = skills.NewRegistry()
	for _, s := range app.PluginReg.AllSkills() {
		app.SkillRegistry.Register(s)
	}
	slog.Info("generate_workflow skill: shared workflow store injected via plugin")

	// Wire trigger ?workflow execution
	wfActionHandler = trigger.NewWorkflowActionHandler(
		func(ctx context.Context, defID, tenantID string, vars map[string]any) (string, error) {
			inst, err := wfStore.CreateInstance(defID, tenantID, vars)
			if err != nil {
				return "", err
			}
			go wfEngine.Run(context.Background(), inst.ID)
			return inst.ID, nil
		},
	)

	// Last plan cache for save_as_workflow skill
	var lastPlanCache sync.Map // tenantID ?*planner.PlanResult
	gw.SetLastPlanCache(&lastPlanCache)

	// Register save_as_workflow skill
	saveWFSkill := workflow.NewSaveWorkflowSkill(wfStore, func(tenantID string) *planner.PlanResult {
		if v, ok := lastPlanCache.Load(tenantID); ok {
			return v.(*planner.PlanResult)
		}
		return nil
	})
	saveWFSkill.SetTriggerBinder(func(wfID, triggerExpr, tenantID string) (string, error) {
		tType, tValue := trigger.ParseTriggerExpr(triggerExpr)
		tID := triggerRT.Register(trigger.Trigger{
			Name:   "auto:" + wfID,
			Kind:   tType,
			Event:  trigger.EventName(tValue),
			Action: trigger.Action{Type: trigger.ActionRunWorkflow, Data: map[string]any{"workflow_id": wfID}},
		})
		return tID, nil
	})
	app.SkillRegistry.Register(saveWFSkill)

	// Register run_workflow and list_workflows skills
	runWFSkill := workflow.NewRunWorkflowSkill(wfStore, func(ctx context.Context, instanceID string) error {
		go wfEngine.Run(context.Background(), instanceID)
		return nil
	})
	defaultTID := os.Getenv("DEFAULT_TENANT_ID")
	if defaultTID == "" {
		defaultTID = "default"
	}
	runWFSkill.SetTenantID(defaultTID)
	app.SkillRegistry.Register(runWFSkill)

	listWFSkill := workflow.NewListWorkflowsSkill(wfStore)
	listWFSkill.SetTenantID(defaultTID)
	app.SkillRegistry.Register(listWFSkill)

	app.SkillRegistry.Register(&document.MedicalExcelSplitSkill{})
	app.SkillRegistry.Register(&document.PythonInterpreterSkill{})

	// Image generation skill ?wired to provider with CapImageGen
	imgGenSkill := imagegen.NewImageGenerateSkill(app.Providers.GetImageGenerator(), cfg.DataPath("output"))
	app.SkillRegistry.Register(imgGenSkill)
	slog.Info("image_generate skill registered", "has_generator", app.Providers.GetImageGenerator() != nil)

	// Browser extension skills ?wired to BrowserHub
	if hub := gw.BrowserHub(); hub != nil {
		browserCtrl := browserskill.NewHubAdapter(hub)
		browserskill.RegisterSkills(app.SkillRegistry, browserCtrl)
		slog.Info("browser extension skills registered", "count", 15)
	}

	// ── Connectors (GitHub, Gmail, Calendar, etc.) ──
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

	// ── Deep Research ──
	var researchBrowser research.BrowserCtrl
	if hub := gw.BrowserHub(); hub != nil {
		researchBrowser = browserskill.NewHubAdapter(hub)
	}
	deepResearch := research.NewDeepResearchSkill(
		searchReg,
		app.LLMClient,
		researchBrowser,
		cfg.DataPath("output"),
	)
	app.SkillRegistry.Register(deepResearch)
	slog.Info("deep_research skill registered")

	// ── File Generation ──
	app.SkillRegistry.Register(filegen.NewFileGenSkill(cfg.DataPath("output")))
	slog.Info("file_generate skill registered")

	// ── MinerU Document Parsing ──
	mineruClient := mineru.NewFromConfig(cfg)
	gw.SetMinerUClient(mineruClient)
	docParseSkill := docparse.NewDocumentParseSkill(
		mineruClient,
		cfg.DataPath("output"),
		knowledgeStore,
	)
	app.SkillRegistry.Register(docParseSkill)
	slog.Info("document_parse skill registered", "enabled", mineruClient.Enabled(), "backend", cfg.MinerUBackend)

	// ── Notifications ──
	notifier := notify.New()
	gw.SetNotifier(notifier)
	slog.Info("notifier initialized")

	// ── Skill Categories (hierarchical calling) ──
	app.SkillRegistry.DefineCategory(skills.SkillCategory{
		ID:          "browser",
		Name:        "浏览。",
		Description: "Control the browser: navigate, click, input, scroll, screenshot, mark elements, manage tabs, extract content. Pass 'action' with the specific browser skill name and 'args' with its parameters.",
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
		Name:        "连接。",
		Description: "Interact with connected third-party services (GitHub, Gmail, Calendar, Notion, Slack, etc.). Pass 'action' with the connector skill name (e.g. 'connector_github_list_repos') and 'args' with its parameters.",
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

	// ── RBAC ──
	rbacEnforcer := rbac.NewEnforcer()
	gw.SetRBACEnforcer(rbacEnforcer)
	slog.Info("rbac enforcer initialized", "roles", len(rbacEnforcer.ListRoles()))

	// ── Approval (Human-in-the-Loop) ──
	approvalMgr := approval.NewManager(approval.DefaultPolicy())
	// Wire ApprovalRuleStore ?Ledger KV
	if typedLdg != nil {
		amigrator := iledger.NewKVMigrator(typedLdg)
		_ = amigrator.MigrateFile("approval", "rules", cfg.DataPath("approval_rules.json"))
		approvalMgr.Rules().SetKVStore(iledger.NewKVConfigStore(typedLdg, "approval"))
		slog.Info("approval rules wired to Ledger KV")
	}
	gw.SetApprovalManager(approvalMgr)

	shellPolicy := tools.NewShellExecPolicy(approvalMgr, toolsMgr)
	gw.SetShellPolicy(shellPolicy)
	slog.Info("shell exec policy wired to approval manager")

	// ── SSE Event Stream ──
	sseBroker := gateway.NewSSEBroker()
	gw.SetSSEBroker(sseBroker)

	// Wire approval events ?SSE
	approvalMgr.OnRequest(func(req *approval.Request) {
		sseBroker.Broadcast(gateway.SSEEvent{
			Type: "approval.request",
			Data: req,
		})
	})

	// Unified event audit trail
	eventTrail := observe.NewAuditTrail(10000)
	gw.SetEventTrail(eventTrail)

	slog.Info("approval + SSE initialized")

	// Wire workflow node events ?SSE
	wfEngine.OnEvent(func(evt observe.AgentEvent) {
		sseBroker.Broadcast(gateway.SSEEvent{
			Type: evt.QualifiedType(),
			Data: evt,
		})
	})

	// ── Ledger State Engine ──
	if typedLdg != nil {
		// LedgerSync mirrors task events into Ledger for event sourcing.
		ledgerSync := iledger.NewLedgerSync(typedLdg, taskStore)
		taskRunner.OnTaskEvent(ledgerSync.OnEvent)
		app.Set("ledger_sync", ledgerSync)

		// MemoryBridge stores task experiences into Ledger Memory.
		memBridge := iledger.NewMemoryBridge(typedLdg, taskStore)
		taskRunner.OnTaskEvent(memBridge.OnEvent)
		app.Set("ledger_memory_bridge", memBridge)

		// Vector Index: wire embedding function if embeddings are configured.
		if embedRes != nil {
			if emb, ok := embedRes.Primary(); ok {
				typedLdg.Vector.SetEmbedFunc(func(ctx context.Context, text string) ([]float32, error) {
					return emb.Embed(ctx, text)
				})
				slog.Info("ledger vector index: embed function attached")
			}
		}

		// Context Graph: link Ledger graph to Recall engine.
		typedLdg.Recall.SetGraph(typedLdg.Graph)

		// Lifecycle: run periodic decay + GC in background.
		app.Lifecycle.RegisterFunc("ledger_lifecycle", func(ctx context.Context) error {
			go func() {
				ticker := time.NewTicker(6 * time.Hour)
				defer ticker.Stop()
				for {
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
						typedLdg.Lifecycle.RunDecay(ctx, "default")
						typedLdg.Lifecycle.RunGC(ctx, "default")
						typedLdg.Lifecycle.RunConsolidate(ctx, "default")
					}
				}
			}()
			return nil
		}, nil)

		slog.Info("ledger state engine initialized",
			"sync", true, "memory_bridge", true,
			"vector", typedLdg.Vector.Enabled(), "graph", true, "lifecycle", true)
	} else {
		slog.Warn("ledger state engine skipped (no ledger)")
	}

	// Gap analyzer + skill generator (with Ledger persistence)
	gapAnalyzer := task.NewGapAnalyzer(llmChatFunc(app.LLMClient, DefaultLLMTemperature))
	if typedLdg != nil {
		// Persist gap records to Ledger event log
		gapAnalyzer.SetPersist(func(ctx context.Context, rec task.GapRecord) error {
			payload, _ := json.Marshal(rec)
			return typedLdg.Events.Append(ctx, &ledger.Event{
				TaskID:    rec.TaskID,
				Kind:      "gap.detected",
				Actor:     "gap_analyzer",
				Payload:   payload,
				CreatedAt: rec.OccurredAt,
			})
		})
		// Load previously persisted gap records on startup
		_ = gapAnalyzer.LoadRecords(context.Background(), func(ctx context.Context) ([]task.GapRecord, error) {
			events, err := typedLdg.Events.Query(ctx, ledger.EventQuery{
				Kinds: []ledger.EventKind{"gap.detected"},
				Limit: 500,
			})
			if err != nil {
				return nil, err
			}
			var records []task.GapRecord
			for _, e := range events {
				var rec task.GapRecord
				if err := json.Unmarshal(e.Payload, &rec); err == nil {
					records = append(records, rec)
				}
			}
			return records, nil
		})
	}
	taskRunner.SetGapAnalyzer(gapAnalyzer)
	gw.SetGapAnalyzer(gapAnalyzer)
	app.Set(agentrt.CompGapAnalyzer, gapAnalyzer)

	skillGenerator := task.NewSkillGenerator(
		llmChatFunc(app.LLMClient, DefaultLLMTemperature),
		app.SkillRegistry,
		&skills.Environment{LLMCall: llmChatFunc(app.LLMClient, DefaultLLMTemperature)},
	)
	taskRunner.SetSkillGenerator(skillGenerator)

	// ── State Kernel ──
	stateKernel := state.NewKernel(cfg.DataDir)
	if ldgRaw, ok := app.Get("github.com/LittleXiaYuan/ledger"); ok {
		if ldg, ok := ldgRaw.(*ledger.Ledger); ok {
			stateKernel.SetKVStore(iledger.NewKVConfigStore(ldg, "state_kernel"))
		}
	}
	stateKernel.UpdateCapabilities(state.CapSnapshot{TotalSkills: len(app.SkillRegistry.All())})
	gw.SetStateKernel(stateKernel)
	p.SetStateContext(stateKernel.CompileForLLM)
	app.Set(agentrt.CompStateKernel, stateKernel)

	taskRunner.OnTaskEvent(func(event, taskID, detail string) {
		stateKernel.RecordAction(state.ActionRecord{
			Action: event + ": " + taskID, Result: detail,
			Success: event == "task_completed" || event == "step_completed",
		})
		gapCount := 0
		if gapAnalyzer != nil {
			gapCount = len(gapAnalyzer.Records("", true))
		}
		stateKernel.UpdateCapabilities(state.CapSnapshot{
			TotalSkills:    len(app.SkillRegistry.All()),
			UnresolvedGaps: gapCount,
		})
		_ = stateKernel.Save()
	})

	// Wire task events ?SSE
	taskRunner.OnTaskEvent(func(event, taskID, detail string) {
		sseBroker.Broadcast(gateway.SSEEvent{
			Type: "task." + event,
			Data: map[string]string{
				"task_id": taskID,
				"event":   event,
				"detail":  detail,
			},
		})
	})

	// ── Task Templates / Working Memory / Threads ──
	templateStore := task.NewTemplateStore(cfg.DataPath("templates"))
	if ldgRaw, ok := app.Get("github.com/LittleXiaYuan/ledger"); ok {
		if ldg, ok := ldgRaw.(*ledger.Ledger); ok {
			templateStore.SetKVStore(iledger.NewKVConfigStore(ldg, "task_templates"))
		}
	}
	gw.SetTemplateStore(templateStore)

	workMemMgr := task.NewWorkingMemoryManagerWithPersistence(
		llmChatFunc(app.LLMClient, LowLLMTemperature), cfg.DataDir,
	)
	taskRunner.WorkMem = workMemMgr
	gw.SetWorkingMemoryManager(workMemMgr)

	threadMgr := task.NewThreadManager(convStore, cfg.DataDir)
	threadMgr.SetChannelSend(func(ctx context.Context, channelType, target, content string) error {
		ch, ok := channelReg.Get(channelType)
		if !ok {
			return fmt.Errorf("channel %s not registered", channelType)
		}
		return ch.Send(ctx, target, channel.Reply{Content: content, Format: "text"})
	})
	gw.SetThreadManager(threadMgr)

	// Wire ThreadManager + WorkingMemory ?Ledger KV
	if typedLdg != nil {
		migrator := iledger.NewKVMigrator(typedLdg)
		_ = migrator.MigrateFile("thread", "threads", cfg.DataPath("threads.json"))
		_ = migrator.MigrateFile("working_memory", "data", cfg.DataPath("working_memory.json"))
		threadMgr.SetKVStore(iledger.NewKVConfigStore(typedLdg, "thread"))
		workMemMgr.SetKVStore(iledger.NewKVConfigStore(typedLdg, "working_memory"))
		slog.Info("thread/working_memory wired to Ledger KV")
	}

	// Wire thread notifications to task events
	taskRunner.OnTaskEvent(func(event, taskID, detail string) {
		if !threadMgr.HasThread(taskID) {
			return
		}
		switch event {
		case "step_completed":
			threadMgr.PostStepResult(taskID, "system", 0, "", detail)
		case "step_failed":
			threadMgr.PostStepFailed(taskID, "system", 0, "", detail)
		case "task_completed":
			threadMgr.PostTaskCompleted(taskID, "system", detail)
		case "task_failed":
			threadMgr.PostTaskFailed(taskID, "system", detail)
		}
	})

	// ── Reverie Action Callbacks ──
	app.Reverie.SetWriteMemory(func(ctx context.Context, fact string) error {
		return app.MemManager.AddMid(ctx, "system", memory.Item{Key: "reverie_insight", Value: fact, Source: "reverie"})
	})
	app.Reverie.SetCreateTask(func(ctx context.Context, title, desc string) error {
		_, err := taskStore.Create(task.CreateRequest{Title: title, Description: desc, TenantID: "system"})
		return err
	})
	app.Reverie.SetUpdateProfile(func(ctx context.Context, key, value string) error {
		app.EditableMem.AddBlock(key, value, 2000)
		return nil
	})

	// ── Reflection Loop ──
	experienceStore := reflectpkg.NewExperienceStore(cfg.DataPath("experience.json"))
	taskReflector := reflectpkg.NewTaskReflector(app.LLMClient, experienceStore)
	gw.SetExperienceStore(experienceStore)
	p.SetStrategyContext(func() string {
		return experienceStore.CompileStrategies(10)
	})

	// Wire ExperienceStore ?Ledger KV
	if ldgRaw, ok := app.Get("github.com/LittleXiaYuan/ledger"); ok {
		if ldg, ok := ldgRaw.(*ledger.Ledger); ok {
			migrator := iledger.NewKVMigrator(ldg)
			_ = migrator.MigrateFile("experience", "data", cfg.DataPath("experience.json"))
			experienceStore.SetKVStore(iledger.NewKVConfigStore(ldg, "experience"))
			slog.Info("experience store wired to Ledger KV")
		}
	}

	// Wire LearningLoop ?ExperienceStore so LLM-extracted lessons feed into strategyContext.
	learningLoop.SetOnLesson(func(category, outcome, lesson, ctx string, tags []string) {
		experienceStore.Add(reflectpkg.Experience{
			Source:   "interaction",
			Category: category,
			Outcome:  outcome,
			Lesson:   lesson,
			Context:  ctx,
			Tags:     tags,
		})
	})

	taskRunner.OnTaskEvent(func(event, taskID, detail string) {
		if event != "task_completed" && event != "task_failed" {
			return
		}
		t, ok := taskStore.Get(taskID)
		if !ok || t == nil {
			return
		}
		trace := reflectpkg.TaskTrace{
			TaskID: t.ID, Title: t.Title, Description: t.Description, Outcome: string(t.Status),
		}
		if t.StartedAt != nil && t.FinishedAt != nil {
			trace.Duration = t.FinishedAt.Sub(*t.StartedAt)
		}
		for _, s := range t.Steps {
			trace.Steps = append(trace.Steps, reflectpkg.StepTrace{
				Action: s.Action, SkillName: s.SkillName, Status: string(s.Status),
				Error: s.Error, Retries: s.RetryCount, GapType: s.GapType,
			})
		}
		taskReflector.AfterTask(context.Background(), trace)
	})

	// ── Trigger Callbacks ──
	taskRunner.OnTaskEvent(func(event, taskID, detail string) {
		if event == "task_completed" || event == "task_failed" {
			evName := trigger.EventTaskCompleted
			if event == "task_failed" {
				evName = trigger.EventTaskFailed
			}
			payload := trigger.EventPayload{
				Event: evName, Text: detail,
				Data: map[string]any{"task_id": taskID}, TaskID: taskID, Timestamp: time.Now(),
			}
			triggerRT.Emit(context.Background(), payload)
			triggerMgr.Emit(context.Background(), payload)
		}
		triggerMgr.Emit(context.Background(), trigger.EventPayload{
			Event: trigger.EventTaskStatusChanged, Text: detail,
			Data: map[string]any{"task_id": taskID, "event": event}, TaskID: taskID, Timestamp: time.Now(),
		})
	})

	// Wire trigger executor callbacks
	wireTriggerExecutor(triggerExecutor, taskStore, taskRunner, threadMgr, channelReg, app, costAwareLLM)

	// Wire condition evaluator
	triggerMgr.SetConditionEvaluator(trigger.NewConditionEvaluator(&trigger.DataSource{
		GetTaskStatus: func(taskID string) (string, error) {
			t, ok := taskStore.Get(taskID)
			if !ok {
				return "", fmt.Errorf("task not found: %s", taskID)
			}
			return string(t.Status), nil
		},
		GetTodayCost: costTracker.TodayCost,
		GetMonthCost: costTracker.MonthCost,
		GetMemoryCount: func(tenantID string) int {
			stats := app.Orchestrator.Stats(tenantID)
			return stats.ShortCount + stats.MidCount + stats.LongCount
		},
	}))
	triggerMgr.Start()

	// ── Cognitive Triggers ──
	app.Reverie.SetOnThought(func(thought planner.Thought) {
		app.Metrics.Cognitive().ReverieThink.Add(1)
		if thought.Significance >= ReverieMinSignificance {
			triggerMgr.EmitCognitive(context.Background(), "reverie_insight", map[string]any{
				"thought_id": thought.ID, "category": thought.Category,
				"significance": thought.Significance, "content": thought.Content,
				"trigger": thought.Trigger,
			})
		}
	})
	if emotionShiftDetector != nil {
		emotionShiftDetector.SetOnShift(func(from, to string, confidence float64) {
			triggerMgr.EmitCognitive(context.Background(), "emotion_shift", map[string]any{
				"from": from, "to": to, "confidence": confidence,
			})
		})
	}

	// ── Wire modules to gateway ──
	gw.SetOrchestrator(app.Orchestrator)
	guardPipeline := app.MustGet(agentrt.CompGuardPipeline).(*guardrails.Pipeline)
	gw.SetZhGuard(guardPipeline)
	adaptiveLoop := app.MustGet(agentrt.CompAdaptiveLoop).(*adaptive.Loop)
	gw.SetAdaptiveLoop(adaptiveLoop)

	// ── Security Guards ──
	toolGuard := guardrails.NewToolGuard(guardrails.DefaultToolGuardConfig())
	egressGuard := guardrails.NewEgressGuard(guardrails.DefaultEgressGuardConfig())
	if auditChain != nil {
		toolGuard.SetAudit(auditChain)
		egressGuard.SetAudit(auditChain)
	}
	gw.SetToolGuard(toolGuard)
	gw.SetEgressGuard(egressGuard)

	// ── Provider Registry ──
	gw.SetProviderRegistry(app.Providers)

	// ── Model Manager KV persistence ──
	if app.Ledger != nil {
		modelKV := iledger.NewKVConfigStore(app.Ledger, "models")
		gw.SetModelKVStore(modelKV)
		slog.Info("models: using Ledger KV for persistence")

		usageKV := iledger.NewKVConfigStore(app.Ledger, "usage")
		gw.SetUsageKVStore(usageKV)
		slog.Info("usage: using Ledger KV for persistence")
	}

	// ── Output directory (for file downloads) ──
	gw.SetOutputDir(filepath.Join(cfg.DataDir, "output"))

	// ── Rate limit / CORS ──
	if rlStr := os.Getenv("RATE_LIMIT"); rlStr != "" {
		if rl, err := strconv.Atoi(rlStr); err == nil && rl > 0 {
			gw.SetRateLimit(rl, time.Minute)
		}
	}
	if origins := os.Getenv("ALLOWED_ORIGINS"); origins != "" {
		gw.SetAllowedOrigins(strings.Split(origins, ","))
	}

	// Store remaining refs for gateway/extensions phase
	app.Set("sched", sched)
	app.Set("conv_store", convStore)
	app.Set("trigger_rt", triggerRT)
	app.Set("trigger_mgr", triggerMgr)
	app.Set("cron_mgr", cronMgr)
	app.Set("skill_optimizer", app.SkillOptimizer)

	return nil
}

// wireTriggerExecutor sets up all trigger executor callbacks.
func wireTriggerExecutor(
	exec *trigger.Executor,
	taskStore task.Store,
	taskRunner *task.Runner,
	threadMgr *task.ThreadManager,
	channelReg *channel.Registry,
	app *agentrt.App,
	costAwareLLM func(ctx context.Context, system, user string) (string, error),
) {
	exec.SetCreateTask(func(ctx context.Context, tenantID, title, desc string) (string, error) {
		t, err := taskStore.Create(task.CreateRequest{Title: title, Description: desc, TenantID: tenantID})
		if err != nil {
			return "", err
		}
		go taskRunner.Run(context.Background(), t.ID)
		return t.ID, nil
	})
	exec.SetContinueTask(func(ctx context.Context, taskID, message string) error {
		if threadMgr != nil {
			threadMgr.Post(taskID, "", "trigger", message)
		}
		return taskRunner.Resume(ctx, taskID)
	})
	exec.SetSendMessage(func(ctx context.Context, channelID, threadID, message string) (string, error) {
		for _, ch := range channelReg.All() {
			if ch.Type() == channelID {
				target := threadID
				if target == "" {
					target = channelID
				}
				if err := ch.Send(ctx, target, channel.Reply{Content: message}); err != nil {
					return "", err
				}
				return "sent", nil
			}
		}
		return "", fmt.Errorf("channel not found: %s", channelID)
	})
	exec.SetCallSkill(func(ctx context.Context, skillName string, args map[string]any) (string, float64, error) {
		sk, ok := app.SkillRegistry.Get(skillName)
		if !ok {
			return "", 0, fmt.Errorf("skill not found: %s", skillName)
		}
		env := &skills.Environment{LLMCall: costAwareLLM}
		result, err := sk.Execute(ctx, args, env)
		return result, 0, err
	})
	exec.SetWriteMemory(func(ctx context.Context, tenantID, content string) error {
		return app.Orchestrator.Ingest(ctx, tenantID, content, "trigger", "trigger_action")
	})
	exec.SetUpdateProfile(func(ctx context.Context, tenantID, key, value string) error {
		if app.EditableMem != nil {
			app.EditableMem.AddBlock(key, value, 2000)
			return nil
		}
		return fmt.Errorf("editable memory not available")
	})
}

// wireKnowledgeToPlanner sets up GraphContext and CodeContext on the planner.
// It preserves any previously-set graphContext (e.g. Ledger recall) and merges
// knowledge retrieval results with it.
func wireKnowledgeToPlanner(p *planner.Planner, ks *knowledge.Store) {
	prevGraph := p.GraphContext() // preserve Ledger recall bridge (set in initPlanner)

	p.SetGraphContext(func(query string) string {
		var parts []string

		// 1) Ledger experience recall (if wired)
		if prevGraph != nil {
			if ledgerCtx := prevGraph(query); ledgerCtx != "" {
				parts = append(parts, ledgerCtx)
			}
		}

		// 2) Knowledge base hybrid retrieval
		scored := ks.HybridSearchReranked(context.Background(), query, DefaultKnowledgeTopK)
		if len(scored) > 0 {
			var sb strings.Builder
			sb.WriteString("## 知识库检索结果\n")
			sb.WriteString("**重要：以下内容来自用户上传的知识库，是最权威的信息来源。回答用户问题时必须优先使用这些内容，而不是调?web_search ?file_search 等工具?*\n")
			sb.WriteString("请在回答中引用来源（使用【来? 文件名】标注）。\n\n")
			n := 0
			for _, sc := range scored {
				if sc.Chunk.Metadata != nil && sc.Chunk.Metadata["lang"] != "" {
					continue
				}
				n++
				sourceName := ""
				if sc.Chunk.Metadata != nil {
					if f := sc.Chunk.Metadata["file"]; f != "" {
						sourceName = f
					} else if u := sc.Chunk.Metadata["url"]; u != "" {
						sourceName = u
					}
				}
				if sourceName == "" {
					if src := ks.GetSource(sc.Chunk.SourceID); src != nil {
						sourceName = src.Name
					}
				}
				if sourceName != "" {
					sb.WriteString(fmt.Sprintf("[来源%d: %s]\n%s\n\n", n, sourceName, sc.Chunk.Content))
				} else {
					sb.WriteString(fmt.Sprintf("[来源%d]\n%s\n\n", n, sc.Chunk.Content))
				}
				if n >= MaxKnowledgeResults {
					break
				}
			}
			if n > 0 {
				parts = append(parts, sb.String())
			}
		}

		return strings.Join(parts, "\n")
	})

	p.SetCodeContext(func(query string) string {
		if !ks.HasCodeSources() {
			return ""
		}
		scored := ks.HybridSearchReranked(context.Background(), query, DefaultCodeTopK)
		var codeResults []knowledge.ScoredChunk
		for _, sc := range scored {
			if sc.Chunk.Metadata != nil && sc.Chunk.Metadata["lang"] != "" {
				codeResults = append(codeResults, sc)
				if len(codeResults) >= MaxKnowledgeResults {
					break
				}
			}
		}
		if len(codeResults) == 0 {
			return ""
		}
		var sb strings.Builder
		sb.WriteString("## 代码上下文\n以下是从代码仓库中检索到的相关代码片段：\n")
		for i, sc := range codeResults {
			filePath := sc.Chunk.Metadata["file"]
			lang := sc.Chunk.Metadata["lang"]
			content := sc.Chunk.Content
			if strings.HasPrefix(content, "FILE:") {
				if idx := strings.Index(content, "\n\n"); idx > 0 && idx < 80 {
					content = strings.TrimSpace(content[idx+2:])
				}
			}
			sb.WriteString(fmt.Sprintf("\n### %d. %s (%s)\n```%s\n%s\n```\n", i+1, filePath, lang, lang, content))
		}
		return sb.String()
	})
}
