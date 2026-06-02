package main

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"yunque-agent/internal/agentcore/adaptive"
	"yunque-agent/internal/agentcore/guardrails"
	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/memory"
	"yunque-agent/internal/agentcore/persona"
	agentrt "yunque-agent/internal/agentcore/runtime"
	"yunque-agent/internal/config"
	"yunque-agent/internal/controlplane/tenant"
	reflectpkg "yunque-agent/internal/experimental/reflect"
	iledger "yunque-agent/internal/ledger"
	pluginpkg "yunque-agent/pkg/plugin"
	"yunque-agent/pkg/safego"

	"yunque-agent/internal/ledgercore"

	"os"
)

// initMemory initializes the memory subsystem, guardrails, persona, reflect, and tenant.
// Extracted from main.go lines 293-411.
func initMemory(app *agentrt.App) error {
	cfg := app.Config

	// Memory layers
	app.ShortMem = memory.NewShortTerm(30 * time.Minute)
	app.MidMem = memory.NewMidTerm()
	app.LongMem = memory.NewLongTerm()
	app.MemManager = memory.NewManager(app.ShortMem, app.MidMem, app.LongMem)

	// Persistence: prefer Ledger (SQLite), fallback to JSON file
	if ldgRaw, ok := app.Get(agentrt.CompLedger); ok {
		if ldg, ok := ldgRaw.(*ledger.Ledger); ok {
			ledgerPersister := iledger.NewLedgerPersister(
				ldg,
				app.MidMem,
				app.LongMem,
				cfg.DataPath("memory.json"),
				iledger.WithLedgerPersisterTemporalKV(iledger.NewTemporalKVStore(ldg)),
			)
			app.MemManager.SetPersister(ledgerPersister)
			app.Set(agentrt.CompMemPersister, ledgerPersister)
			slog.Info("memory persistence: Ledger (SQLite)", "temporal_writeback", ledgerPersister.TemporalWritebackReady())
		}
	} else {
		memPersister := memory.NewPersister(cfg.DataPath("memory.json"), app.MidMem, app.LongMem)
		app.MemManager.SetPersister(memPersister)
		app.Set(agentrt.CompMemPersister, memPersister)
		slog.Info("memory persistence: JSON file (no Ledger available)")
	}

	// Memory GC (using NewTicker + context for clean shutdown)
	app.Lifecycle.RegisterFunc("memory_gc", func(ctx context.Context) error {
		safego.Go("memory-gc-ticker", func() {
			ticker := time.NewTicker(5 * time.Minute)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					app.ShortMem.GC()
				}
			}
		})
		return nil
	}, nil)

	// Memory pipeline (LLM-driven fact extraction)
	app.MemPipeline = memory.NewPipeline(func(ctx context.Context, msgs []memory.ChatMessage) (string, error) {
		llmMsgs := make([]llm.Message, len(msgs))
		for i, m := range msgs {
			llmMsgs[i] = llm.Message{Role: m.Role, Content: m.Content}
		}
		return app.LLMClient.ChatJSON(ctx, llmMsgs)
	}, app.MemManager)
	app.MemPipeline.SetDailyDir(cfg.DataPath("memory", "daily"))

	// Wire CognitivePlugin.OnMemoryExtract + HookManager.HookMemoryExtract into the pipeline.
	// This is deferred until after plugin registration (done in initPlugins), so we use a
	// lazy closure that checks at call time.
	app.MemPipeline.SetFactTransform(func(ctx context.Context, facts []string) []string {
		// Step 1: CognitivePlugin transformation
		if app.PluginReg != nil {
			pluginFacts := make([]pluginpkg.ExtractedFact, len(facts))
			for i, f := range facts {
				pluginFacts[i] = pluginpkg.ExtractedFact{Key: "", Value: f, Source: "pipeline"}
			}
			transformed := app.PluginReg.TransformFacts(ctx, pluginFacts)
			if transformed == nil {
				return nil
			}
			facts = make([]string, len(transformed))
			for i, f := range transformed {
				facts[i] = f.Value
			}
		}

		// Step 2: HookManager memory.extract event
		if hookMgrRaw, ok := app.Get("hook_manager"); ok {
			if hm, ok := hookMgrRaw.(*pluginpkg.HookManager); ok {
				hm.EmitAll(ctx, pluginpkg.HookMemoryExtract, map[string]any{
					"facts": facts,
					"count": len(facts),
				})
			}
		}
		return facts
	})

	// Load daily memory files in parallel
	type dailyResult struct {
		loaded int
		err    error
	}
	dailyCh := make(chan dailyResult, 1)
	safego.Go("memory-daily-load", func() {
		loaded, err := memory.LoadDailyFiles(cfg.DataPath("memory", "daily"), app.MemManager)
		dailyCh <- dailyResult{loaded: loaded, err: err}
	})

	// Knowledge graph + editable memory
	app.KnGraph = memory.NewGraph()
	app.EditableMem = memory.NewEditableMemory()

	// Orchestrator ?five-layer unified recall
	orchCfg := memory.DefaultOrchestratorConfig()
	if v := os.Getenv("DEFAULT_TENANT_ID"); v != "" {
		orchCfg.PrimaryTenant = v
	}
	app.Orchestrator = memory.NewOrchestrator(orchCfg, app.MemManager, app.KnGraph, app.EditableMem)
	app.Orchestrator.SetOnPromote(func() { app.Metrics.Cognitive().MemoryPromote.Add(1) })

	// TF-IDF importance scorer: replaces heuristic keyword matching with statistical scoring.
	// Rare/specific content gets higher importance → promoted to mid/long-term memory.
	if ldgRaw, ok := app.Get(agentrt.CompLedger); ok {
		if ldg, ok := ldgRaw.(*ledger.Ledger); ok {
			tfidfScorer := ledger.NewTFIDFScorer()
			existing, _ := ldg.Memory.Search(context.Background(), ledger.MemoryQuery{Limit: 500})
			for _, m := range existing {
				tfidfScorer.AddDocument(m.Content)
			}
			app.Set("tfidf_scorer", tfidfScorer)

			app.Orchestrator.SetImportanceFunc(func(_ context.Context, content string) memory.Importance {
				tfidfScorer.AddDocument(content)
				result := tfidfScorer.Score(content)
				switch {
				case result.Score >= 0.7:
					return memory.ImportanceHigh
				case result.Score >= 0.4:
					return memory.ImportanceMedium
				default:
					return memory.ImportanceLow
				}
			})
			slog.Info("TF-IDF importance scorer initialized", "corpus", tfidfScorer.CorpusSize(), "vocab", tfidfScorer.VocabSize())

			// BM25 index for hybrid retrieval
			bm25Idx := ledger.NewBM25Index()
			for _, m := range existing {
				bm25Idx.Add(m.ID, m.Content)
			}
			ldg.Recall.SetBM25(bm25Idx)
			app.Set("bm25_index", bm25Idx)
			slog.Info("BM25 index initialized", "docs", bm25Idx.Size())

			initGraphRAGRuntime(app, ldg)

			slog.Info("vector ANN backend will be configured after embeddings are wired",
				"env", "VECTOR_ANN_BACKEND")
		}
	}
	slog.Info("memory orchestrator initialized (5-layer)")

	// Guardrails pipeline (Chinese + English)
	guardPipeline := guardrails.NewPipeline()
	guardPipeline.Add(guardrails.NewZhPIIGuard(true, false))
	guardPipeline.Add(guardrails.NewZhInjectionGuard())
	guardPipeline.Add(guardrails.NewInjectionGuard())
	guardPipeline.Add(guardrails.NewZhModerationGuard(true))
	app.Set(agentrt.CompGuardPipeline, guardPipeline)
	slog.Info("guardrails initialized")

	// Orchestrator persistence: Ledger or file
	if ldgRaw, ok := app.Get(agentrt.CompLedger); ok {
		if ldg, ok := ldgRaw.(*ledger.Ledger); ok {
			orchPersister := iledger.NewLedgerOrchPersister(ldg, app.KnGraph, app.EditableMem)
			app.Set(agentrt.CompOrchPersist, orchPersister)
			slog.Info("orchestrator persistence: Ledger (SQLite)")
		}
	} else {
		orchPersister := memory.NewOrchestratorPersister(cfg.DataDir, app.KnGraph, app.EditableMem)
		app.Set(agentrt.CompOrchPersist, orchPersister)
		slog.Info("orchestrator persistence: JSON files")
	}
	adaptiveLoop := adaptive.NewLoop()
	app.Set(agentrt.CompAdaptiveLoop, adaptiveLoop)
	{
		var loadWg sync.WaitGroup
		loadWg.Add(2)
		safego.Go("orch-persist-load", func() {
			defer loadWg.Done()
			if op, ok := app.Get(agentrt.CompOrchPersist); ok {
				if loader, lok := op.(interface{ Load() error }); lok {
					loader.Load()
				}
			}
		})
		safego.Go("adaptive-loop-load", func() {
			defer loadWg.Done()
			adaptiveLoop.LoadFrom(cfg.DataPath("adaptive.json"))
		})
		loadWg.Wait()
	}
	slog.Info("adaptive loop initialized")

	// Persona
	personaDir := os.Getenv("PERSONA_DIR")
	if personaDir == "" {
		personaDir = cfg.DataPath("persona")
	}
	botPersona, err := persona.New(personaDir)
	if err != nil {
		slog.Warn("persona init failed, using defaults", "err", err)
		botPersona, _ = persona.New(cfg.DataPath("persona"))
	}
	presetMgr := persona.NewPresetManager()
	personaChain := persona.NewPriorityChain(botPersona, presetMgr)
	app.Set(agentrt.CompPersona, botPersona)
	app.Set(agentrt.CompPresetMgr, presetMgr)
	app.Set(agentrt.CompPersonaChain, personaChain)
	slog.Info("persona loaded", "dir", personaDir, "presets", len(presetMgr.List()))

	// Wait for daily memory load
	if result := <-dailyCh; result.err != nil {
		slog.Warn("daily memory reload failed", "err", result.err)
	} else if result.loaded > 0 {
		slog.Info("daily memory files loaded", "facts", result.loaded)
	}

	// Reflect engine
	reflectEngine := reflectpkg.NewEngine(app.LLMClient)
	app.Set(agentrt.CompReflectEngine, reflectEngine)

	// Tenant manager
	tenantMgr := tenant.NewManager()
	defTID := os.Getenv("DEFAULT_TENANT_ID")
	if defTID == "" {
		defTID = "default"
	}
	defTKey := os.Getenv("DEFAULT_API_KEY")
	if defTKey == "" {
		defTKey = os.Getenv("DEFAULT_TENANT_KEY")
		if defTKey != "" {
			slog.Warn("DEFAULT_TENANT_KEY is deprecated, please rename it to DEFAULT_API_KEY")
		}
	}
	if defTKey == "" {
		generated, err := config.GenerateSecureKey(32)
		if err != nil {
			// A missing CSPRNG at boot is a deploy-level problem, not a
			// business logic one — bail out so operators see a proper
			// init failure instead of a panic stack.
			return fmt.Errorf("initMemory: failed to auto-generate default tenant API key: %w", err)
		}
		defTKey = generated
		slog.Warn("DEFAULT_API_KEY not set, using auto-generated key (not persisted across restarts)")
	}
	if tenantMgr.ByID(defTID) == nil {
		tenantMgr.RegisterWithID(defTID, "default", defTKey)
	}
	app.Set(agentrt.CompTenantMgr, tenantMgr)
	slog.Info("tenant manager ready", "count", len(tenantMgr.List()))

	// Periodic memory promotion (using NewTicker + context for clean shutdown)
	app.Lifecycle.RegisterFunc("memory_promote", func(ctx context.Context) error {
		safego.Go("memory-promote-ticker", func() {
			ticker := time.NewTicker(10 * time.Minute)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					for _, t := range tenantMgr.List() {
						n := app.Orchestrator.Promote(context.Background(), t.ID)
						if n > 0 {
							slog.Info("memory promote", "tenant", t.ID, "promoted", n)
						}
					}
				}
			}
		})
		return nil
	}, nil)

	return nil
}
