package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/LittleXiaYuan/ledger"

	"yunque-agent/internal/agentcore/localbrain"
	agentrt "yunque-agent/internal/agentcore/runtime"
	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/experimental/causal"
	"yunque-agent/internal/experimental/curiosity"
	"yunque-agent/internal/experimental/metacog"
	"yunque-agent/internal/experimental/microagent"
	"yunque-agent/internal/experimental/recommend"
	"yunque-agent/internal/experimental/rlsched"
	"yunque-agent/internal/experimental/taskdistill"
	"yunque-agent/internal/experimental/world"
	iledger "yunque-agent/internal/ledger"
)

// initIntelligence is Phase 6.8: sets up LocalBrain, AgenticThinking,
// MetaCog monitor, World Model, and Causal Engine.
// These are the "intelligence layer" - they enhance but don't replace
// existing planner/router functionality.
func initIntelligence(app *agentrt.App) error {
	// ?? LocalBrain: local small model decision layer ??
	// Priority: "local" pool key ??"fast" key ??first available key.
	// LocalBrain pre-classifies intent cheaply; even when backed by the same
	// model, the fast-path (NeedTools=false for greetings) avoids injecting
	// 40+ tool definitions and cuts latency for trivial queries.
	var brain *localbrain.LocalBrain
	lbClient := app.LLMPool.Get("local")
	lbLabel := "local"
	if lbClient == nil {
		lbClient = app.LLMPool.Get("fast")
		lbLabel = "fast"
	}
	if lbClient == nil {
		lbClient = app.LLMPool.Get("smart")
		lbLabel = "smart (fallback)"
	}
	if lbClient == nil {
		lbClient = app.LLMPool.Get("primary")
		lbLabel = "primary (fallback)"
	}
	if lbClient != nil {
		brain = localbrain.New(lbClient, app.LLMPool)
		app.Planner.SetLocalBrain(brain)

		if routerRaw, ok := app.Get(agentrt.CompSmartRouter); ok {
			if r, ok := routerRaw.(interface {
				SetLocalBrain(*localbrain.LocalBrain)
			}); ok {
				r.SetLocalBrain(brain)
			}
		}

		slog.Info("localbrain: initialized", "backend", lbLabel)
	} else {
		slog.Warn("localbrain: no LLM client available, disabled")
	}
	app.Set("localbrain", brain)

	// ?? LoRA Scheduler: training lifecycle for small model evolution ??
	if brain != nil && app.Ledger != nil {
		loraCfg := localbrain.DefaultSchedulerConfig()
		loraCfg.BaseModel = os.Getenv("LOCALBRAIN_BASE_MODEL")
		if loraCfg.BaseModel == "" {
			loraCfg.BaseModel = "qwen-2.5-7b"
		}
		if dir := os.Getenv("LORA_ADAPTER_DIR"); dir != "" {
			loraCfg.AdapterDir = dir
		}
		if dir := os.Getenv("TRAINING_OUTPUT_DIR"); dir != "" {
			loraCfg.TrainingDataDir = dir
		}

		var loraAdapter *localbrain.LoRAAdapter
		if vllmURL := os.Getenv("VLLM_BASE_URL"); vllmURL != "" {
			loraAdapter = localbrain.NewLoRAAdapter(localbrain.LoRAAdapterConfig{
				BaseURL: vllmURL,
				APIKey:  os.Getenv("VLLM_API_KEY"),
			})
		}

		scheduler := localbrain.NewLoRAScheduler(app.Ledger, loraAdapter, brain, loraCfg)
		scheduler.LoadState()
		// Wire LoRAScheduler ??Ledger KV
		if app.Ledger != nil {
			lmigrator := iledger.NewKVMigrator(app.Ledger)
			stateFile := loraCfg.AdapterDir
			if stateFile == "" {
				stateFile = "./data/adapters"
			}
			_ = lmigrator.MigrateFile("lora_scheduler", "state", stateFile+"/scheduler_state.json")
			scheduler.SetKVStore(iledger.NewKVConfigStore(app.Ledger, "lora_scheduler"))
			slog.Info("lora scheduler wired to Ledger KV")
		}
		app.Set("lora_scheduler", scheduler)
		slog.Info("lora scheduler: initialized",
			"base_model", loraCfg.BaseModel,
			"min_samples", loraCfg.MinSamples,
			"has_vllm", loraAdapter != nil,
		)
	}

	// AgenticThinking: dynamic think depth
	var ldg *ledger.Ledger
	if ldgRaw, ok := app.Get(agentrt.CompLedger); ok {
		ldg, _ = ldgRaw.(*ledger.Ledger)
	}

	agenticCfg := localbrain.DefaultAgenticConfig()
	at := localbrain.NewAgenticThinking(brain, app.LLMPool, ldg, agenticCfg)
	app.Planner.SetAgenticThinking(at)
	app.Set("agentic_thinking", at)
	slog.Info("agentic thinking: initialized", "default_level", agenticCfg.DefaultThinkLevel)

	// ?? MetaCog Monitor: runtime anomaly detection ??
	if ldg != nil {
		thresholds := metacog.DefaultThresholds()
		monitor := metacog.NewFromLedger(ldg, thresholds)

		// Wire alert to slog + optional LocalBrain metacog adaptation
		monitor.SetAlertFunc(func(alert metacog.Alert) {
			slog.Warn("metacog alert",
				"task", alert.TaskID,
				"kind", alert.Kind,
				"severity", alert.Severity,
				"message", alert.Message,
			)
		})
		monitor.Start()
		app.Set("metacog_monitor", monitor)
		app.Lifecycle.RegisterFunc("metacog", nil, func(ctx context.Context) error {
			monitor.Stop()
			return nil
		})
		slog.Info("metacog monitor: started")
	}

	// ?? World Model: environment state tracking ??
	if ldg != nil {
		wm := world.NewModel(ldg, "system")
		_ = wm.Load(context.Background())
		app.Set("world_model", wm)
		slog.Info("world model: loaded")
	}

	// ?? Causal Engine: root cause analysis ??
	if ldg != nil {
		ce := causal.New(ldg)
		app.Set("causal_engine", ce)
		slog.Info("causal engine: initialized")
	}

	// ?? Curiosity Module: autonomous exploration when idle ??
	if ldg != nil {
		cm := curiosity.New(ldg)
		if app.LLMPool != nil {
			fastClient := app.LLMPool.Get("fast")
			if fastClient == nil {
				fastClient = app.LLMPool.Get("primary")
			}
			if fastClient != nil {
				cm.SetExploreFn(func(ctx context.Context, q curiosity.Question) (*curiosity.Result, error) {
					system := "You are an AI knowledge explorer. Investigate thoroughly and provide useful findings."
					user := "Question: " + q.Question
					if q.Context != "" {
						user += "\nContext: " + q.Context
					}
					reply, err := fastClient.Chat(ctx, []llm.Message{
						{Role: "system", Content: system},
						{Role: "user", Content: user},
					}, 0.7)
					if err != nil {
						return nil, err
					}
					return &curiosity.Result{
						Question:   q.Question,
						Findings:   []string{reply},
						NewFacts:   []string{reply},
						Confidence: 0.6,
						Useful:     len(reply) > 50,
					}, nil
				})
			}
		}
		app.Set("curiosity_module", cm)
		slog.Info("curiosity module: initialized")
	}

	// ?? Task Distiller: extract patterns from completed tasks ??
	if ldg != nil {
		td := taskdistill.New(ldg)
		app.Set("task_distiller", td)
		slog.Info("task distiller: initialized")
	}

	// ?? MicroAgent Registry: domain-specific prompt enhancement ??
	maRegistry := microagent.NewRegistry()
	maDir := app.Config.DataPath("microagents")
	if _, err := os.Stat(maDir); err == nil {
		count, loadErr := microagent.LoadFromDirectory(maDir, microagent.ScopeGlobal, maRegistry)
		if loadErr != nil {
			slog.Warn("microagent: load error", "dir", maDir, "err", loadErr)
		} else if count > 0 {
			slog.Info("microagent: loaded from directory", "dir", maDir, "count", count)
		}
	}
	app.Set("microagent_registry", maRegistry)
	slog.Info("microagent registry: initialized", "agents", len(maRegistry.All()))

	// ?? RL Scheduler: Q-Learning task scheduling optimization ??
	rlActions := []string{"model_fast", "model_smart", "priority_high", "priority_normal", "priority_low"}
	rlCfg := rlsched.DefaultQLearnerConfig(rlActions)
	ql := rlsched.NewQLearner(rlCfg)
	app.Set("rl_scheduler", ql)
	slog.Info("rl scheduler: initialized", "actions", len(rlActions), "epsilon", rlCfg.Epsilon)

	// ?? Recommend Engine: personalized response/skill suggestions ??
	recEngine := recommend.NewEngine()
	app.Set("recommend_engine", recEngine)
	slog.Info("recommend engine: initialized")

	return nil
}
