package main

import (
	"context"
	"log/slog"
	"os"
	"strconv"

	"yunque-agent/internal/agentcore/costtrack"
	"yunque-agent/internal/agentcore/embeddings"
	"yunque-agent/internal/agentcore/emotion"
	"yunque-agent/internal/agentcore/identity"
	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/memory"
	"yunque-agent/internal/agentcore/models"
	"yunque-agent/internal/agentcore/persona"
	"yunque-agent/internal/agentcore/planner"
	"yunque-agent/internal/agentcore/router"
	agentrt "yunque-agent/internal/agentcore/runtime"
	"yunque-agent/internal/agentcore/selfheal"
	"yunque-agent/internal/agentcore/speech"
	"yunque-agent/internal/controlplane/gateway"
	iledger "yunque-agent/internal/ledger"

	"yunque-agent/internal/ledgercore"
)

type perceptionResult struct {
	embedRes    *embeddings.Resolver
	costTracker *costtrack.Tracker
}

func initPerceptionWiring(app *agentrt.App, gw *gateway.Gateway) (*perceptionResult, error) {
	cfg := app.Config
	r := &perceptionResult{}

	presetMgr := app.MustGet(agentrt.CompPresetMgr).(*persona.PresetManager)
	personaChain := app.MustGet(agentrt.CompPersonaChain).(*persona.PriorityChain)
	emotionShiftDetector := app.MustGet("emotion_shift_detector").(*planner.EmotionShiftDetector)
	factEventHook := app.MustGet("fact_event_hook").(*planner.FactEventHook)

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

	bandit := router.NewModelBandit(router.PolicyThompson)
	bandit.RegisterArm(router.TierFast, fastModelID)
	bandit.RegisterArm(router.TierSmart, cfg.LLMModel)
	bandit.RegisterArm(router.TierExpert, expertModelID)
	if cfg.LLMFastModel != "" && cfg.LLMFastModel != fastModelID {
		bandit.RegisterArm(router.TierFast, cfg.LLMFastModel)
	}
	smartRouter.SetBandit(bandit)
	app.Set("model_bandit", bandit)

	gw.SetSmartRouter(smartRouter)
	slog.Info("smart router initialized", "fast", fastModelID, "smart", cfg.LLMModel, "expert", expertModelID, "bandit", "thompson")

	// ── Identity / SelfHeal / Cost ──
	idResolver := identity.NewResolver()
	if app.Ledger != nil {
		idResolver.SetKVStore(iledger.NewKVConfigStore(app.Ledger, "identity"))
		slog.Info("identity: using Ledger KV for persistence")
	}
	gw.SetIdentityResolver(idResolver)
	app.Set(agentrt.CompIdentityRes, idResolver)

	healer := selfheal.New(cfg.DataPath("plugins"), app.LLMBreaker.Call)
	healer.SetRegistries(app.PluginReg, app.SkillRegistry, app.Planner.InvalidatePromptCache)
	gw.SetHealer(healer)
	skillLifecycle := selfheal.NewLifecycle(healer, cfg.DataDir)
	gw.SetLifecycle(skillLifecycle)
	app.Set("skill_lifecycle", skillLifecycle)

	r.costTracker = costtrack.NewWithPersistence(cfg.DataDir)
	if ldgRaw, ok := app.Get(agentrt.CompLedger); ok {
		if ldg, ok := ldgRaw.(*ledger.Ledger); ok {
			r.costTracker.SetKVStore(iledger.NewKVConfigStore(ldg, "costtrack"))
			slog.Info("cost tracker wired to Ledger KV")
		}
	}
	gw.SetCostTracker(r.costTracker)
	app.Set(agentrt.CompCostTracker, r.costTracker)

	// ── Speech ──
	speechReg := speech.NewRegistry()
	if ttsKey := cfg.LLMAPIKey; ttsKey != "" {
		speechReg.RegisterTTS(speech.NewOpenAITTS(cfg.LLMBaseURL, ttsKey, os.Getenv("TTS_MODEL")))
		speechReg.RegisterSTT(speech.NewOpenAISTT(cfg.LLMBaseURL, ttsKey, os.Getenv("STT_MODEL")))
	}
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

	// ── Emotion ──
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
	if ldgRaw, ok := app.Get(agentrt.CompLedger); ok {
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

	gw.SetPersonaChain(personaChain)

	// ── Embeddings ──
	r.embedRes = embeddings.NewResolver()
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
		// Embeddings can come from a different provider than the chat LLM (e.g.
		// DeepSeek/Kimi for chat but a dedicated or local embedder for vectors).
		// Use EMBED_API_KEY when set, otherwise fall back to the LLM key.
		embedKey := os.Getenv("EMBED_API_KEY")
		if embedKey == "" {
			embedKey = cfg.LLMAPIKey
		}
		emb, err := embeddings.NewOpenAI(embedKey, embedURL, embedModel, dims)
		if err == nil {
			r.embedRes.Register("openai", emb)
			slog.Info("embeddings provider registered", "model", embedModel, "dims", dims)
		} else {
			slog.Warn("embeddings init failed", "error", err)
		}
	}
	gw.SetEmbeddings(r.embedRes)
	app.Set("embed_resolver", r.embedRes)

	// ── Memory conflict detector ──
	conflictLLM := func(ctx context.Context, system, user string) (string, error) {
		return app.LLMClient.Chat(ctx, []llm.Message{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		}, 0.2)
	}
	conflictDetector := memory.NewConflictDetector(conflictLLM)
	if primary, ok := r.embedRes.Primary(); ok {
		gateCfg := memory.DefaultEmbeddingGateConfig()
		conflictDetector.SetEmbeddingGate(func(ctx context.Context, text string) ([]float32, error) {
			return primary.Embed(ctx, text)
		}, gateCfg)
		slog.Info("memory conflict detector: embedding gate enabled",
			"model", primary.Model(),
			"threshold", gateCfg.Threshold)
	} else {
		slog.Info("memory conflict detector: keyword + LLM only (no embedder configured)")
	}
	app.Orchestrator.SetConflictDetector(conflictDetector)

	return r, nil
}
