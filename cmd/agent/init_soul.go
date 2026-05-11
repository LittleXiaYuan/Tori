package main

// init_soul.go — Deep Soul Layer initialization.
// Extracted from initTasks: Trait Mining, MetaCog, Curiosity, World Model,
// Iterate, Causal, TaskDistill, Eval, Distill, MultiAgent, Trust,
// Review Gate, SkillGrow, Version Registry, ReAct, and the nighttime scheduler.

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	iforestpkg "yunque-agent/internal/agentcore/anomaly"
	"yunque-agent/internal/agentcore/multiagent"
	"yunque-agent/internal/agentcore/planner"
	"yunque-agent/internal/agentcore/review"
	agentrt "yunque-agent/internal/agentcore/runtime"
	"yunque-agent/internal/agentcore/selfheal"
	"yunque-agent/internal/agentcore/trust"
	"yunque-agent/internal/agentcore/version"
	"yunque-agent/internal/experimental/causal"
	"yunque-agent/internal/experimental/curiosity"
	"yunque-agent/internal/experimental/distill"
	"yunque-agent/internal/experimental/eval"
	"yunque-agent/internal/experimental/iterate"
	"yunque-agent/internal/experimental/metacog"
	reactpkg "yunque-agent/internal/experimental/react"
	"yunque-agent/internal/experimental/skillgrow"
	"yunque-agent/internal/experimental/taskdistill"
	"yunque-agent/internal/experimental/trait"
	"yunque-agent/internal/experimental/world"

	"github.com/LittleXiaYuan/ledger"

	iledger "yunque-agent/internal/ledger"
)

// soulDeps bundles dependencies needed by the soul-layer initializer.
type soulDeps struct {
	app            *agentrt.App
	costAwareLLM   func(ctx context.Context, system, user string) (string, error)
	typedLdg       *ledger.Ledger
	skillLifecycle *selfheal.Lifecycle
}

// initSoulLayer sets up cognitive subsystems: trait mining, metacognition,
// curiosity, world model, iterate engine, causal reasoning, task distillation,
// evaluation, knowledge distillation, multi-agent, trust, review, skill growth,
// version registry, and the nighttime scheduler.
func initSoulLayer(deps soulDeps) {
	app := deps.app
	cfg := app.Config
	costAwareLLM := deps.costAwareLLM
	typedLdg := deps.typedLdg

	// 1. Trait Mining → extract user preferences from every conversation
	traitStore := trait.NewStore(cfg.DataPath("traits"))
	traitMiner := trait.NewMiner(traitStore, func(ctx context.Context, message string) ([]trait.MineResult, error) {
		app.Metrics.Cognitive().TraitMine.Add(1)
		system := "你是用户偏好分析器。从用户消息中提取偏好维度。\n" +
			"输出JSON数组: [{\"dimension\":\"...\", \"preference\":\"...\", \"confidence\": 0.0-1.0}]\n" +
			"维度包括: communication_style, domain_preference, interaction_pattern, content_interest, " +
			"language_preference, tone_preference, expertise_level, work_schedule\n" +
			"如果消息中没有明显偏好，返回空数组 []。只输出JSON。"
		reply, err := costAwareLLM(ctx, system, message)
		if err != nil {
			return nil, err
		}
		var results []trait.MineResult
		json.Unmarshal([]byte(reply), &results)
		return results, nil
	})
	app.Set("trait_store", traitStore)
	app.Set("trait_miner", traitMiner)
	slog.Info("trait: store loaded", "traits", len(traitStore.All()))

	// 2. MetaCog → real-time reasoning anomaly detection + Isolation Forest
	if typedLdg != nil {
		metaCog := metacog.NewFromLedger(typedLdg, metacog.DefaultThresholds())

		// Isolation Forest for statistical anomaly detection (supplements threshold-based MetaCog)
		iforest := iforestpkg.NewIsolationForest(iforestpkg.DefaultIForestConfig())
		metaCog.SetIsolationForest(iforest, 0.65)
		app.Set("metacog_iforest", iforest)

		metaCog.SetAlertFunc(func(alert metacog.Alert) {
			app.Metrics.Cognitive().MetaCogAlert.Add(1)
			slog.Warn("metacog: anomaly detected",
				"kind", alert.Kind,
				"severity", alert.Severity,
				"task", alert.TaskID,
				"msg", alert.Message,
			)
			if alert.Severity == metacog.SeverityCritical || alert.Severity == metacog.SeverityWarning {
				if busRaw, ok := app.Get("reverie_event_bus"); ok {
					if bus, ok := busRaw.(*planner.ReverieEventBus); ok {
						bus.Emit(planner.ReverieEvent{
							Type:    planner.EventMetaCogAlert,
							Trigger: fmt.Sprintf("%s: %s", alert.Kind, alert.Message),
							Data: map[string]string{
								"task_id":  alert.TaskID,
								"severity": string(alert.Severity),
							},
						})
					}
				}
			}
		})
		metaCog.Start()
		app.Set("metacog", metaCog)
		slog.Info("metacog: monitor started (+ reverie bridge)")
	}

	// 3. Curiosity → idle exploration
	if typedLdg != nil {
		curiosityMod := curiosity.New(typedLdg)
		curiosityMod.SetExploreFn(func(ctx context.Context, q curiosity.Question) (*curiosity.Result, error) {
			app.Metrics.Cognitive().CuriosityExplore.Add(1)
			system := "你是一个知识探索者。请回答以下问题并提取关键事实。\n" +
				"输出JSON: {\"findings\": [\"...\"], \"new_facts\": [\"...\"], \"confidence\": 0.0-1.0, \"useful\": true/false}"
			reply, err := costAwareLLM(ctx, system, q.Question)
			if err != nil {
				return nil, err
			}
			var result curiosity.Result
			json.Unmarshal([]byte(reply), &result)
			result.Question = q.Question
			return &result, nil
		})
		app.Set("curiosity", curiosityMod)
		slog.Info("curiosity: module ready")
	}

	// 3.5 World Model → track external environment state
	if typedLdg != nil {
		worldModel := world.NewModel(typedLdg, "default")
		if err := worldModel.Load(context.Background()); err != nil {
			slog.Debug("world: load from memory skipped", "err", err)
		}
		app.Set("world_model", worldModel)
		slog.Info("world: model initialized", "entries", worldModel.Size())
	}

	// 4. Iterate → self-improvement engine (nighttime mode: 2:00-5:00 AM)
	iterateCfg := iterate.Config{
		Enabled:     true,
		TokenBudget: 8000,
		MaxRounds:   3,
		Cooldown:    6 * time.Hour,
		AutoApprove: true,
		DataDir:     cfg.DataPath("iterate"),
	}
	iterEngine := iterate.NewEngine(iterateCfg)
	iterEngine.SetLLMCall(func(ctx context.Context, system, user string) (string, int, error) {
		reply, err := costAwareLLM(ctx, system, user)
		estTokens := (len(system) + len(user) + len(reply)) / 4
		return reply, estTokens, err
	})
	iterDiscusser := iterate.NewDiscusser(func(ctx context.Context, system, user string) (string, int, error) {
		reply, err := costAwareLLM(ctx, system, user)
		estTokens := (len(system) + len(user) + len(reply)) / 4
		return reply, estTokens, err
	})
	iterEngine.SetDiscusser(iterDiscusser)
	iterEngine.SetOnExecute(func(ctx context.Context, proposal *iterate.Proposal) error {
		switch proposal.Type {
		case iterate.PropAddMemory:
			if typedLdg != nil {
				typedLdg.Memory.Put(ctx, &ledger.MemoryEntry{
					TenantID:   "default",
					Kind:       ledger.MemoryFact,
					Key:        "iterate." + proposal.ID,
					Content:    proposal.Description,
					Source:     "iterate",
					Confidence: 0.7,
				})
			}
			return nil
		case iterate.PropFixBehavior:
			slog.Info("iterate: behavior fix applied", "title", proposal.Title)
			return nil
		case iterate.PropAdjustPersona:
			slog.Info("iterate: persona adjustment proposed", "title", proposal.Title)
			return nil
		case iterate.PropInstallSkill:
			_, err := deps.skillLifecycle.GenerateCandidate(ctx, proposal.Description)
			return err
		default:
			return fmt.Errorf("unknown proposal type: %s", proposal.Type)
		}
	})
	iterEngine.SetOnRecord(func(ctx context.Context, summary string) {
		if typedLdg != nil {
			typedLdg.Memory.Put(ctx, &ledger.MemoryEntry{
				TenantID:   "default",
				Kind:       ledger.MemoryExperience,
				Key:        "iterate.cycle." + fmt.Sprintf("%d", time.Now().UnixMilli()),
				Content:    summary,
				Source:     "iterate",
				Confidence: 0.9,
			})
		}
	})
	app.Set("iterate_engine", iterEngine)
	slog.Info("iterate: engine initialized", "budget", iterateCfg.TokenBudget, "cooldown", iterateCfg.Cooldown, "nighttime", "02:00-05:00")

	// Nighttime scheduler goroutine
	go runNighttimeScheduler(app, iterEngine, typedLdg)
	slog.Info("nighttime scheduler: started", "window", "02:00-05:00", "check_interval", "30m")

	// 5. Causal Engine
	if typedLdg != nil {
		causalEngine := causal.New(typedLdg)
		app.Set("causal_engine", causalEngine)
		slog.Info("causal: engine initialized")
	}

	// 6. Task Distiller
	if typedLdg != nil {
		taskDistiller := taskdistill.New(typedLdg)
		taskDistiller.SetAnalyzeFunc(func(ctx context.Context, summary taskdistill.TaskEventSummary) (*taskdistill.Result, error) {
			system := "你是任务经验分析师。分析以下任务执行摘要，提取可复用的模式、行为规则和工具洞见。\n" +
				"输出JSON: {\"patterns\": [{\"name\":\"\", \"description\":\"\", \"trigger\":\"\", \"confidence\": 0.0-1.0}], " +
				"\"rules\": [{\"condition\":\"\", \"action\":\"\", \"rationale\":\"\", \"confidence\": 0.0-1.0}], " +
				"\"tool_insights\": [{\"tool_name\":\"\", \"context\":\"\", \"observation\":\"\", \"score\": 0.0-1.0}]}"
			reply, err := costAwareLLM(ctx, system, summary.ToPrompt())
			if err != nil {
				return nil, err
			}
			var result taskdistill.Result
			json.Unmarshal([]byte(reply), &result)
			return &result, nil
		})
		app.Set("task_distiller", taskDistiller)
		slog.Info("taskdistill: distiller initialized")

		// 7. Eval
		evaluator := eval.New(typedLdg)
		evaluator.SetDistiller(taskDistiller)
		evaluator.SetEvalFunc(func(ctx context.Context, summary taskdistill.TaskEventSummary) (*eval.EvalResult, error) {
			system := "你是任务质量评估师。评估以下任务执行的质量。\n" +
				"输出JSON: {\"goal_achieved\": 0.0-1.0, \"efficiency\": 0.0-1.0, \"side_effects\": [], " +
				"\"quality_score\": 0.0-1.0, \"reasoning\": \"\", \"suggestions\": [], \"should_distill\": true/false}"
			reply, err := costAwareLLM(ctx, system, summary.ToPrompt())
			if err != nil {
				return nil, err
			}
			var result eval.EvalResult
			json.Unmarshal([]byte(reply), &result)
			return &result, nil
		})
		app.Set("evaluator", evaluator)
		slog.Info("eval: evaluator initialized")
	}

	// 8. Distill → compress Expert model outputs into reusable Fast rules
	knowledgeDistiller := distill.New(func(ctx context.Context, system, user string) (string, error) {
		return costAwareLLM(ctx, system, user)
	})
	if typedLdg != nil {
		knowledgeDistiller.SetStore(func(ctx context.Context, key, value, category string) error {
			return typedLdg.Memory.Put(ctx, &ledger.MemoryEntry{
				TenantID:   "default",
				Kind:       ledger.MemoryRule,
				Key:        "distill." + key,
				Content:    value,
				Source:     "distillation:" + category,
				Confidence: 0.8,
			})
		})
		knowledgeDistiller.SetSearch(func(ctx context.Context, query string) (string, bool) {
			entries, _ := typedLdg.Memory.Search(ctx, ledger.MemoryQuery{
				TenantID: "default",
				Kinds:    []ledger.MemoryKind{ledger.MemoryRule},
				Query:    query,
				Limit:    1,
			})
			if len(entries) > 0 && entries[0].Confidence > 0.6 {
				return entries[0].Content, true
			}
			return "", false
		})
	}
	app.Set("knowledge_distiller", knowledgeDistiller)
	slog.Info("distill: knowledge distiller initialized")

	// 9. MultiAgent
	app.Set("multiagent_factory", func(team multiagent.Team) *multiagent.Supervisor {
		return multiagent.NewSupervisor(team, func(ctx context.Context, role multiagent.AgentRole, msg multiagent.Message) (string, error) {
			return costAwareLLM(ctx, role.SystemPrompt, msg.Content)
		})
	})
	slog.Info("multiagent: supervisor factory registered")

	// 10. Trust
	trustTracker := trust.NewTracker(cfg.DataPath("trust_scores.json"))
	app.Set("trust_tracker", trustTracker)
	slog.Info("trust: tracker loaded", "skills", len(trustTracker.All()))

	// 11. Review Gate
	reviewGate := review.NewGate()
	reviewGate.SetLLMReview(func(ctx context.Context, operation string) (bool, error) {
		system := "你是安全审查员。判断以下操作是否安全。只回复 'yes' 或 'no'。"
		reply, err := costAwareLLM(ctx, system, operation)
		if err != nil {
			return true, err // fail-open
		}
		return strings.Contains(strings.ToLower(reply), "yes"), nil
	})
	app.Set("review_gate", reviewGate)
	slog.Info("review: gate initialized")

	// 12. SkillGrow
	skillGrowDetector := skillgrow.NewDetector(3)
	if typedLdg != nil {
		skillGrowDetector.SetMemSearch(func(ctx context.Context, query string) (int, string) {
			entries, _ := typedLdg.Memory.Search(ctx, ledger.MemoryQuery{
				TenantID: "default",
				Kinds:    []ledger.MemoryKind{ledger.MemoryExperience},
				Query:    query,
				Limit:    10,
			})
			if len(entries) > 0 {
				return len(entries), entries[0].Content
			}
			return 0, ""
		})
		skillGrowDetector.SetOnProposal(func(ctx context.Context, pattern, suggestion string) {
			slog.Info("skillgrow: pattern detected, proposing skill", "pattern", pattern)
			typedLdg.Memory.Put(ctx, &ledger.MemoryEntry{
				TenantID:   "default",
				Kind:       ledger.MemoryFact,
				Key:        "skillgrow.proposal." + fmt.Sprintf("%d", time.Now().UnixMilli()),
				Content:    suggestion,
				Source:     "skillgrow",
				Confidence: 0.7,
			})
		})
	}
	if typedLdg != nil {
		skillGrowDetector.SetKVStore(iledger.NewKVConfigStore(typedLdg, "skillgrow"))
	}
	app.Set("skillgrow_detector", skillGrowDetector)
	slog.Info("skillgrow: detector initialized", "threshold", 3)

	// 13. Version Registry
	versionReg := version.NewRegistry()
	versionReg.Register(version.Component{
		ID: "yunque-agent", Name: "Yunque Agent",
		CurrentVersion: "0.1.0-dev", Source: "builtin",
	})
	app.Set("version_registry", versionReg)
	slog.Info("version: registry initialized", "components", versionReg.Count())

	// 14. ReAct Runner
	if typedLdg != nil {
		reactRunner := reactpkg.NewRunner(typedLdg)
		app.Set("react_runner", reactRunner)
		slog.Info("react: runner initialized")
	}
}

// runNighttimeScheduler runs iterate, curiosity, eval, causal, lifecycle, and
// compaction during low-traffic hours (2:00-5:00 AM).
func runNighttimeScheduler(app *agentrt.App, iterEngine *iterate.Engine, typedLdg *ledger.Ledger) {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		hour := time.Now().Hour()
		if hour < 2 || hour >= 5 {
			continue
		}

		// Run iterate cycle
		if iterEngine.Enabled() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			cycleLog, err := iterEngine.RunCycle(ctx)
			cancel()
			if err != nil {
				slog.Debug("iterate: nighttime cycle skipped", "reason", err)
			} else if cycleLog != nil {
				slog.Info("iterate: nighttime cycle complete",
					"proposals", len(cycleLog.Proposals),
					"tokens", cycleLog.TokensUsed,
					"stopped_by", cycleLog.StoppedBy,
				)
			}
		}

		// Run curiosity exploration
		if cm, ok := app.Get("curiosity"); ok {
			if curiosityMod, ok := cm.(*curiosity.Module); ok {
				if curiosityMod.ShouldExplore(context.Background(), "default") {
					ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
					results, err := curiosityMod.Explore(ctx, "default")
					cancel()
					if err != nil {
						slog.Debug("curiosity: nighttime explore failed", "err", err)
					} else {
						slog.Info("curiosity: nighttime explore complete", "results", len(results))
					}
				}
			}
		}

		// Run eval + distill batch
		if ev, ok := app.Get("evaluator"); ok {
			if evaluator, ok := ev.(*eval.Evaluator); ok {
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
				results, err := evaluator.EvaluateBatch(ctx, "default", 10)
				cancel()
				if err != nil {
					slog.Debug("eval: nighttime batch failed", "err", err)
				} else if len(results) > 0 {
					slog.Info("eval: nighttime batch complete", "evaluated", len(results))
				}
			}
		}

		// Run causal failure pattern analysis
		if ce, ok := app.Get("causal_engine"); ok {
			if causalEngine, ok := ce.(*causal.CausalEngine); ok {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
				patterns, err := causalEngine.AnalyzeFailurePatterns(ctx, "default", 20)
				cancel()
				if err == nil && len(patterns) > 0 {
					slog.Info("causal: failure patterns found", "count", len(patterns))
				}
			}
		}

		// Run Ledger memory lifecycle
		if typedLdg != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			result, err := typedLdg.Lifecycle.RunAll(ctx, "default")
			cancel()
			if err != nil {
				slog.Debug("lifecycle: nighttime run failed", "err", err)
			} else if result != nil {
				slog.Info("lifecycle: nighttime maintenance complete",
					"decayed", result.Decayed,
					"gc", result.GarbageCol,
					"consolidated", result.Consolidated,
					"expired", result.Expired,
				)
			}
		}

		// Run event log compaction
		if typedLdg != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			compactResult, err := typedLdg.CompactEvents(ctx, "default", ledger.DefaultCompactConfig())
			cancel()
			if err != nil {
				slog.Debug("compact: nighttime run failed", "err", err)
			} else if compactResult.TasksCompacted > 0 {
				slog.Info("compact: nighttime compaction complete",
					"compacted", compactResult.TasksCompacted,
					"events_removed", compactResult.EventsRemoved,
				)
			}
		}

		slog.Debug("nighttime: soul evolution cycle done", "hour", hour)
	}
}
