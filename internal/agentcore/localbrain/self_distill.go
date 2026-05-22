package localbrain

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	ldg "yunque-agent/internal/ledgercore"
)

// SelfDistillPipeline orchestrates the full self-distillation cognitive loop:
//
//	Collect → Score → Export → Train → Evaluate → Deploy → Report
//
// Each step is independently retriable. The pipeline records its state
// so it can resume after interruptions. For the MVP this runs end-to-end
// in a single function call; production would split across NightScheduler ticks.
type SelfDistillPipeline struct {
	ledger    *ldg.Ledger
	scheduler *LoRAScheduler
	brain     *LocalBrain
	trainer   *LoRATrainer
	evaluator *LoRAEvaluator
	scorer    *ConversationScorer
	dataDir   string
}

// SelfDistillConfig controls the distillation run.
type SelfDistillConfig struct {
	TenantID     string  `json:"tenant_id"`
	BaseModel    string  `json:"base_model"`
	MinSamples   int     `json:"min_samples"`
	MinScore     float64 `json:"min_score"`
	NumEpochs    int     `json:"num_epochs"`
	LoRARank     int     `json:"lora_rank"`
	LearningRate float64 `json:"learning_rate"`
	DaysLookback int     `json:"days_lookback"`
	MaxSeqLength int     `json:"max_seq_length"`
	AdapterDir   string  `json:"adapter_dir"`
}

// DefaultSelfDistillConfig returns a CPU-friendly demo configuration.
func DefaultSelfDistillConfig() SelfDistillConfig {
	return SelfDistillConfig{
		TenantID:     "default",
		BaseModel:    "Qwen/Qwen2.5-3B",
		MinSamples:   20,
		MinScore:     0.6,
		NumEpochs:    2,
		LoRARank:     8,
		LearningRate: 2e-4,
		DaysLookback: 7,
		MaxSeqLength: 1024,
		AdapterDir:   "./data/adapters",
	}
}

// DistillReport is the final output of a distillation run, designed
// for PPT slides and demo dashboards.
type DistillReport struct {
	RunID       string            `json:"run_id"`
	StartedAt   time.Time         `json:"started_at"`
	CompletedAt time.Time         `json:"completed_at"`
	Duration    time.Duration     `json:"duration"`
	Config      SelfDistillConfig `json:"config"`

	// Collection phase
	RawConversations int `json:"raw_conversations"`
	ScoredSamples    int `json:"scored_samples"`
	QualifiedSamples int `json:"qualified_samples"`

	// Scoring distribution
	ScoreDistribution ScoreDistribution `json:"score_distribution"`

	// Training phase
	TrainResult *TrainResult `json:"train_result,omitempty"`
	DataPath    string       `json:"data_path"`

	// Evaluation phase
	EvalBefore  *EvalResult `json:"eval_before,omitempty"`
	EvalAfter   *EvalResult `json:"eval_after,omitempty"`
	Improvement float64     `json:"improvement"`

	// Deploy phase
	AdapterName string `json:"adapter_name"`
	Deployed    bool   `json:"deployed"`

	// Summary
	Success bool      `json:"success"`
	Error   string    `json:"error,omitempty"`
	Steps   []StepLog `json:"steps"`
}

// ScoreDistribution breaks down conversation quality scores.
type ScoreDistribution struct {
	Excellent int     `json:"excellent"` // >= 0.9
	Good      int     `json:"good"`      // 0.7 - 0.9
	Fair      int     `json:"fair"`      // 0.5 - 0.7
	Poor      int     `json:"poor"`      // < 0.5
	Average   float64 `json:"average"`
}

// StepLog records timing and status for each pipeline step.
type StepLog struct {
	Name     string        `json:"name"`
	Status   string        `json:"status"` // ok / skipped / failed
	Duration time.Duration `json:"duration"`
	Detail   string        `json:"detail,omitempty"`
}

// ConversationScorer evaluates dialogue quality from Ledger history.
// It computes a composite score based on:
//   - User satisfaction signals (explicit thumbs-up, implicit continued engagement)
//   - Response completeness (tool usage vs simple text)
//   - Conversation flow (no repeated questions, escalation success)
type ConversationScorer struct {
	llm LLMScoreFunc
}

// LLMScoreFunc scores a single conversation turn for training quality.
type LLMScoreFunc func(ctx context.Context, userQuery, assistantReply string) (float64, error)

// NewConversationScorer creates a scorer, optionally backed by LLM.
func NewConversationScorer(llm LLMScoreFunc) *ConversationScorer {
	return &ConversationScorer{llm: llm}
}

// DistillSample extends the base training data with response text and
// satisfaction signal for the distillation pipeline. These fields don't
// exist on the core TrainingSample (which is intent-classification focused).
type DistillSample struct {
	Input     string `json:"input"`
	Output    string `json:"output"`
	Tier      string `json:"tier"`
	Upgraded  bool   `json:"upgraded"`
	Satisfied bool   `json:"satisfied"`
}

// ScoredSample is a training sample with a quality score attached.
type ScoredSample struct {
	DistillSample
	Score  float64 `json:"score"`
	Reason string  `json:"reason,omitempty"`
}

// ScoreHeuristic scores a sample without LLM using rule-based heuristics.
func (cs *ConversationScorer) ScoreHeuristic(sample DistillSample) ScoredSample {
	score := 0.5
	reasons := make([]string, 0, 4)

	if sample.Satisfied {
		score += 0.2
		reasons = append(reasons, "user_satisfied")
	}
	if sample.Upgraded {
		score -= 0.1
		reasons = append(reasons, "was_upgraded")
	}

	replyLen := len(sample.Output)
	if replyLen > 200 {
		score += 0.1
		reasons = append(reasons, "detailed_reply")
	} else if replyLen < 20 {
		score -= 0.1
		reasons = append(reasons, "terse_reply")
	}

	switch sample.Tier {
	case "smart", "expert":
		score += 0.1
		reasons = append(reasons, "complex_task")
	}

	if score > 1.0 {
		score = 1.0
	}
	if score < 0.0 {
		score = 0.0
	}

	return ScoredSample{
		DistillSample: sample,
		Score:         score,
		Reason:        strings.Join(reasons, ","),
	}
}

// ScoreBatch scores a batch using heuristics + optional LLM refinement.
func (cs *ConversationScorer) ScoreBatch(ctx context.Context, samples []DistillSample) []ScoredSample {
	scored := make([]ScoredSample, 0, len(samples))
	for _, s := range samples {
		ss := cs.ScoreHeuristic(s)

		if cs.llm != nil {
			llmScore, err := cs.llm(ctx, s.Input, s.Output)
			if err == nil && llmScore > 0 {
				ss.Score = ss.Score*0.4 + llmScore*0.6
			}
		}

		scored = append(scored, ss)
	}
	return scored
}

// NewSelfDistillPipeline creates the distillation orchestrator.
func NewSelfDistillPipeline(
	ledger *ldg.Ledger,
	scheduler *LoRAScheduler,
	brain *LocalBrain,
	trainer *LoRATrainer,
	evaluator *LoRAEvaluator,
	scorer *ConversationScorer,
	dataDir string,
) *SelfDistillPipeline {
	if dataDir == "" {
		dataDir = "./data/distill"
	}
	return &SelfDistillPipeline{
		ledger:    ledger,
		scheduler: scheduler,
		brain:     brain,
		trainer:   trainer,
		evaluator: evaluator,
		scorer:    scorer,
		dataDir:   dataDir,
	}
}

// Run executes the full self-distillation pipeline.
func (p *SelfDistillPipeline) Run(ctx context.Context, cfg SelfDistillConfig) *DistillReport {
	report := &DistillReport{
		RunID:     fmt.Sprintf("distill_%d", time.Now().UnixNano()),
		StartedAt: time.Now(),
		Config:    cfg,
	}

	os.MkdirAll(p.dataDir, 0755)

	// Step 1: Collect raw conversation data
	rawSamples := p.stepCollect(ctx, cfg, report)

	// Step 2: Score conversations
	scoredSamples := p.stepScore(ctx, rawSamples, report)

	// Step 3: Filter and export training data
	dataPath := p.stepExport(scoredSamples, cfg, report)
	if report.Error != "" {
		p.finalize(report)
		return report
	}

	// Step 4: Baseline evaluation (before training)
	p.stepEvalBefore(ctx, cfg, report)

	// Step 5: Train LoRA adapter
	p.stepTrain(ctx, dataPath, cfg, report)
	if report.Error != "" {
		p.finalize(report)
		return report
	}

	// Step 6: Evaluate after training
	p.stepEvalAfter(ctx, cfg, report)

	// Step 7: Deploy (conditional on improvement)
	p.stepDeploy(ctx, cfg, report)

	report.Success = report.Error == ""
	p.finalize(report)
	return report
}

func (p *SelfDistillPipeline) stepCollect(ctx context.Context, cfg SelfDistillConfig, report *DistillReport) []DistillSample {
	start := time.Now()
	var samples []DistillSample

	// Convert LocalBrain's TrainingSample to DistillSample
	if p.brain != nil {
		tenantID := cfg.TenantID
		if tenantID == "" {
			tenantID = "default"
		}
		for _, ts := range p.brain.ExportTrainingData(tenantID) {
			samples = append(samples, DistillSample{
				Input:    ts.Input,
				Output:   ts.Intent.Category,
				Tier:     ts.Tier,
				Upgraded: ts.Upgraded,
			})
		}
	}

	// Also pull from Ledger experience entries
	if p.ledger != nil && len(samples) < cfg.MinSamples {
		entries, err := p.ledger.Memory.Search(ctx, ldg.MemoryQuery{
			TenantID: cfg.TenantID,
			Kinds:    []ldg.MemoryKind{ldg.MemoryExperience},
			Limit:    500,
		})
		if err == nil {
			for _, e := range entries {
				var data map[string]any
				if json.Unmarshal([]byte(e.Content), &data) != nil {
					continue
				}
				query, _ := data["user_query"].(string)
				reply, _ := data["final_reply"].(string)
				success, _ := data["success"].(bool)
				if query == "" || reply == "" {
					continue
				}
				samples = append(samples, DistillSample{
					Input:     query,
					Output:    reply,
					Satisfied: success,
					Tier:      "smart",
				})
			}
		}
	}

	report.RawConversations = len(samples)
	report.Steps = append(report.Steps, StepLog{
		Name:     "collect",
		Status:   "ok",
		Duration: time.Since(start),
		Detail:   fmt.Sprintf("%d raw samples collected", len(samples)),
	})

	slog.Info("distill: collected", "samples", len(samples))
	return samples
}

func (p *SelfDistillPipeline) stepScore(ctx context.Context, raw []DistillSample, report *DistillReport) []ScoredSample {
	start := time.Now()

	if p.scorer == nil {
		scored := make([]ScoredSample, len(raw))
		for i, s := range raw {
			scored[i] = ScoredSample{DistillSample: s, Score: 0.7}
		}
		report.ScoredSamples = len(scored)
		report.Steps = append(report.Steps, StepLog{
			Name:     "score",
			Status:   "skipped",
			Duration: time.Since(start),
			Detail:   "no scorer configured, default score=0.7",
		})
		return scored
	}

	scored := p.scorer.ScoreBatch(ctx, raw)
	report.ScoredSamples = len(scored)

	var dist ScoreDistribution
	var totalScore float64
	for _, s := range scored {
		totalScore += s.Score
		switch {
		case s.Score >= 0.9:
			dist.Excellent++
		case s.Score >= 0.7:
			dist.Good++
		case s.Score >= 0.5:
			dist.Fair++
		default:
			dist.Poor++
		}
	}
	if len(scored) > 0 {
		dist.Average = totalScore / float64(len(scored))
	}
	report.ScoreDistribution = dist

	report.Steps = append(report.Steps, StepLog{
		Name:     "score",
		Status:   "ok",
		Duration: time.Since(start),
		Detail:   fmt.Sprintf("%d scored (avg=%.2f)", len(scored), dist.Average),
	})

	slog.Info("distill: scored", "samples", len(scored), "avg", dist.Average)
	return scored
}

func (p *SelfDistillPipeline) stepExport(scored []ScoredSample, cfg SelfDistillConfig, report *DistillReport) string {
	start := time.Now()

	minScore := cfg.MinScore
	if minScore <= 0 {
		minScore = 0.5
	}

	var qualified []ScoredSample
	for _, s := range scored {
		if s.Score >= minScore {
			qualified = append(qualified, s)
		}
	}
	report.QualifiedSamples = len(qualified)

	if len(qualified) < cfg.MinSamples {
		report.Error = fmt.Sprintf("insufficient training data: %d samples (need %d)", len(qualified), cfg.MinSamples)
		report.Steps = append(report.Steps, StepLog{
			Name:     "export",
			Status:   "failed",
			Duration: time.Since(start),
			Detail:   report.Error,
		})
		return ""
	}

	// Export as OpenAI ChatML JSONL
	dataPath := filepath.Join(p.dataDir, fmt.Sprintf("distill_%s.jsonl", time.Now().Format("20060102_150405")))
	f, err := os.Create(dataPath)
	if err != nil {
		report.Error = fmt.Sprintf("create data file: %v", err)
		report.Steps = append(report.Steps, StepLog{
			Name:     "export",
			Status:   "failed",
			Duration: time.Since(start),
			Detail:   report.Error,
		})
		return ""
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	for _, s := range qualified {
		chatml := map[string]any{
			"messages": []map[string]string{
				{"role": "system", "content": "You are a helpful assistant."},
				{"role": "user", "content": s.Input},
				{"role": "assistant", "content": s.Output},
			},
		}
		if err := enc.Encode(chatml); err != nil {
			report.Error = fmt.Sprintf("write training sample: %v", err)
			report.Steps = append(report.Steps, StepLog{
				Name:     "export",
				Status:   "failed",
				Duration: time.Since(start),
				Detail:   report.Error,
			})
			return ""
		}
	}

	report.DataPath = dataPath
	report.Steps = append(report.Steps, StepLog{
		Name:     "export",
		Status:   "ok",
		Duration: time.Since(start),
		Detail:   fmt.Sprintf("%d qualified samples → %s", len(qualified), dataPath),
	})

	slog.Info("distill: exported", "path", dataPath, "samples", len(qualified))
	return dataPath
}

func (p *SelfDistillPipeline) stepEvalBefore(ctx context.Context, cfg SelfDistillConfig, report *DistillReport) {
	start := time.Now()

	if p.evaluator == nil {
		report.Steps = append(report.Steps, StepLog{
			Name:     "eval_before",
			Status:   "skipped",
			Duration: time.Since(start),
			Detail:   "no evaluator configured",
		})
		return
	}

	evalSamples := p.generateEvalSamples(ctx, cfg)
	if len(evalSamples) == 0 {
		report.Steps = append(report.Steps, StepLog{
			Name:     "eval_before",
			Status:   "skipped",
			Duration: time.Since(start),
			Detail:   "no eval samples available",
		})
		return
	}

	result, err := p.evaluator.EvalFunc()(ctx, "", evalSamples)
	if err != nil {
		report.Steps = append(report.Steps, StepLog{
			Name:     "eval_before",
			Status:   "failed",
			Duration: time.Since(start),
			Detail:   err.Error(),
		})
		return
	}

	report.EvalBefore = result
	report.Steps = append(report.Steps, StepLog{
		Name:     "eval_before",
		Status:   "ok",
		Duration: time.Since(start),
		Detail:   fmt.Sprintf("baseline score=%.3f", result.Score),
	})
}

func (p *SelfDistillPipeline) stepTrain(ctx context.Context, dataPath string, cfg SelfDistillConfig, report *DistillReport) {
	start := time.Now()

	if p.trainer == nil {
		report.Error = "trainer not configured"
		report.Steps = append(report.Steps, StepLog{
			Name:     "train",
			Status:   "failed",
			Duration: time.Since(start),
			Detail:   report.Error,
		})
		return
	}

	adapterName := fmt.Sprintf("distill_%s", time.Now().Format("20060102_150405"))
	outputDir := filepath.Join(cfg.AdapterDir, adapterName)
	os.MkdirAll(outputDir, 0755)

	job := TrainJob{
		BaseModel:    cfg.BaseModel,
		DataPath:     dataPath,
		OutputDir:    outputDir,
		AdapterName:  adapterName,
		NumEpochs:    cfg.NumEpochs,
		LoRARank:     cfg.LoRARank,
		LearningRate: cfg.LearningRate,
		MaxSeqLength: cfg.MaxSeqLength,
	}

	result, err := p.trainer.TrainFunc()(ctx, job)
	if err != nil {
		report.Error = fmt.Sprintf("training failed: %v", err)
		report.Steps = append(report.Steps, StepLog{
			Name:     "train",
			Status:   "failed",
			Duration: time.Since(start),
			Detail:   report.Error,
		})
		return
	}

	if !result.Success {
		report.Error = fmt.Sprintf("training unsuccessful: %s", result.Error)
		report.Steps = append(report.Steps, StepLog{
			Name:     "train",
			Status:   "failed",
			Duration: time.Since(start),
			Detail:   report.Error,
		})
		return
	}

	report.TrainResult = result
	report.AdapterName = adapterName
	report.Steps = append(report.Steps, StepLog{
		Name:     "train",
		Status:   "ok",
		Duration: time.Since(start),
		Detail:   fmt.Sprintf("adapter=%s loss=%.4f samples=%d", adapterName, result.FinalLoss, result.Samples),
	})

	slog.Info("distill: training complete",
		"adapter", adapterName,
		"loss", result.FinalLoss,
		"duration", result.Duration,
	)
}

func (p *SelfDistillPipeline) stepEvalAfter(ctx context.Context, cfg SelfDistillConfig, report *DistillReport) {
	start := time.Now()

	if p.evaluator == nil || report.AdapterName == "" {
		report.Steps = append(report.Steps, StepLog{
			Name:     "eval_after",
			Status:   "skipped",
			Duration: time.Since(start),
			Detail:   "no evaluator or no adapter",
		})
		return
	}

	evalSamples := p.generateEvalSamples(ctx, cfg)
	if len(evalSamples) == 0 {
		report.Steps = append(report.Steps, StepLog{
			Name:     "eval_after",
			Status:   "skipped",
			Duration: time.Since(start),
			Detail:   "no eval samples",
		})
		return
	}

	result, err := p.evaluator.EvalFunc()(ctx, report.AdapterName, evalSamples)
	if err != nil {
		report.Steps = append(report.Steps, StepLog{
			Name:     "eval_after",
			Status:   "failed",
			Duration: time.Since(start),
			Detail:   err.Error(),
		})
		return
	}

	report.EvalAfter = result
	if report.EvalBefore != nil {
		report.Improvement = result.Score - report.EvalBefore.Score
	}

	report.Steps = append(report.Steps, StepLog{
		Name:     "eval_after",
		Status:   "ok",
		Duration: time.Since(start),
		Detail:   fmt.Sprintf("post-train score=%.3f improvement=%.3f", result.Score, report.Improvement),
	})
}

func (p *SelfDistillPipeline) stepDeploy(ctx context.Context, cfg SelfDistillConfig, report *DistillReport) {
	start := time.Now()

	if report.TrainResult == nil || !report.TrainResult.Success {
		report.Steps = append(report.Steps, StepLog{
			Name:     "deploy",
			Status:   "skipped",
			Duration: time.Since(start),
			Detail:   "no successful training result",
		})
		return
	}

	if report.EvalAfter != nil && !report.EvalAfter.Passed {
		report.Steps = append(report.Steps, StepLog{
			Name:     "deploy",
			Status:   "skipped",
			Duration: time.Since(start),
			Detail:   fmt.Sprintf("eval not passed (score=%.3f)", report.EvalAfter.Score),
		})
		return
	}

	if report.Improvement < -0.05 {
		report.Steps = append(report.Steps, StepLog{
			Name:     "deploy",
			Status:   "skipped",
			Duration: time.Since(start),
			Detail:   fmt.Sprintf("regression detected (%.3f)", report.Improvement),
		})
		return
	}

	if p.scheduler != nil && p.scheduler.adapter != nil {
		err := p.scheduler.adapter.Load(ctx, report.AdapterName, report.TrainResult.AdapterPath, cfg.BaseModel)
		if err != nil {
			report.Steps = append(report.Steps, StepLog{
				Name:     "deploy",
				Status:   "failed",
				Duration: time.Since(start),
				Detail:   err.Error(),
			})
			return
		}
		report.Deployed = true
	} else {
		report.Deployed = false
	}

	report.Steps = append(report.Steps, StepLog{
		Name:     "deploy",
		Status:   "ok",
		Duration: time.Since(start),
		Detail:   fmt.Sprintf("adapter=%s deployed=%v", report.AdapterName, report.Deployed),
	})

	slog.Info("distill: deploy complete", "adapter", report.AdapterName, "deployed", report.Deployed)
}

func (p *SelfDistillPipeline) generateEvalSamples(ctx context.Context, cfg SelfDistillConfig) []EvalSample {
	if p.ledger == nil {
		return nil
	}

	entries, err := p.ledger.Memory.Search(ctx, ldg.MemoryQuery{
		TenantID: cfg.TenantID,
		Kinds:    []ldg.MemoryKind{ldg.MemoryExperience},
		Limit:    20,
	})
	if err != nil || len(entries) == 0 {
		return nil
	}

	var samples []EvalSample
	for _, e := range entries {
		var data map[string]any
		if json.Unmarshal([]byte(e.Content), &data) != nil {
			continue
		}
		query, _ := data["user_query"].(string)
		reply, _ := data["final_reply"].(string)
		if query == "" || reply == "" {
			continue
		}
		samples = append(samples, EvalSample{
			Input:    query,
			Expected: reply,
		})
	}
	return samples
}

func (p *SelfDistillPipeline) finalize(report *DistillReport) {
	report.CompletedAt = time.Now()
	report.Duration = report.CompletedAt.Sub(report.StartedAt)

	// Save report to disk
	reportPath := filepath.Join(p.dataDir, report.RunID+".json")
	data, err := json.MarshalIndent(report, "", "  ")
	if err == nil {
		os.WriteFile(reportPath, data, 0644)
		slog.Info("distill: report saved", "path", reportPath)
	}

	slog.Info("distill: pipeline complete",
		"run_id", report.RunID,
		"success", report.Success,
		"duration", report.Duration,
		"samples", report.QualifiedSamples,
		"improvement", report.Improvement,
	)
}

// ListReports returns all distillation reports from the data directory.
func (p *SelfDistillPipeline) ListReports() []*DistillReport {
	entries, err := os.ReadDir(p.dataDir)
	if err != nil {
		return nil
	}

	var reports []*DistillReport
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "distill_") || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(p.dataDir, e.Name()))
		if err != nil {
			continue
		}
		var r DistillReport
		if json.Unmarshal(data, &r) == nil {
			reports = append(reports, &r)
		}
	}
	return reports
}
