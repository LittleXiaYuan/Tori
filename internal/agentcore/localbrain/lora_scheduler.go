package localbrain

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	ldg "github.com/LittleXiaYuan/ledger"
	iledger "yunque-agent/internal/ledger"
	"yunque-agent/pkg/safego"
)

// LoRAScheduler orchestrates the full LoRA lifecycle:
//   data accumulation → training trigger → quality evaluation → hot deploy → rollback
//
// It connects the training data produced by DataCollector and NightScheduler
// to the actual fine-tuning and deployment on a vLLM server.
//
// Flow:
//  1. Monitor: check if enough new training data has accumulated
//  2. Prepare: merge exported JSONL files into a training dataset
//  3. Train: submit training job (via TrainFunc callback — pluggable)
//  4. Evaluate: run quality checks on the new adapter
//  5. Deploy: hot-load the new LoRA via LoRAAdapter
//  6. Monitor: A/B test for regression, rollback if needed
type LoRAScheduler struct {
	mu sync.Mutex

	ledger   *ldg.Ledger
	adapter  *LoRAAdapter
	trainFn  TrainFunc
	evalFn   EvalFunc
	brain    *LocalBrain
	dataDir  string
	kvs      *iledger.KVConfigStore
	metrics  *TrainingMetrics

	config       SchedulerConfig
	state        SchedulerState            // default tenant state (backward compat)
	tenantStates map[string]*SchedulerState // per-tenant state for multi-tenant isolation
}

// SchedulerConfig configures LoRA training triggers and thresholds.
type SchedulerConfig struct {
	MinSamples      int           // minimum training samples before triggering (default 200)
	MinInterval     time.Duration // minimum time between training runs (default 24h)
	EvalMinScore    float64       // minimum eval score to accept new adapter (default 0.7)
	MaxAdapters     int           // max concurrent loaded adapters (default 3)
	BaseModel       string        // base model identifier
	TrainingDataDir string        // where JSONL training files live
	AdapterDir      string        // where trained adapter weights are stored
	ABTestDuration  time.Duration // how long to A/B test before committing (default 1h)
	FilterEnabled   bool          // whether to run data quality filtering before training (default true)
	FilterConfig    FilterConfig  // data quality filter settings
}

func DefaultSchedulerConfig() SchedulerConfig {
	return SchedulerConfig{
		MinSamples:      200,
		MinInterval:     24 * time.Hour,
		EvalMinScore:    0.7,
		MaxAdapters:     3,
		TrainingDataDir: "./data/training",
		AdapterDir:      "./data/adapters",
		ABTestDuration:  1 * time.Hour,
		FilterEnabled:   true,
		FilterConfig:    DefaultFilterConfig(),
	}
}

// SchedulerState tracks the current state of the LoRA training pipeline.
type SchedulerState struct {
	LastTrainTime  time.Time      `json:"last_train_time"`
	LastTrainResult *TrainResult  `json:"last_train_result"`
	CurrentAdapter string         `json:"current_adapter"` // active LoRA adapter name
	PreviousAdapter string        `json:"previous_adapter"` // for rollback
	ABTestActive   bool           `json:"ab_test_active"`
	ABTestStart    time.Time      `json:"ab_test_start"`
	ABTestMetrics  ABTestMetrics  `json:"ab_test_metrics"`
	TotalTrains    int            `json:"total_trains"`
	TotalRollbacks int            `json:"total_rollbacks"`
}

// ABTestMetrics compares new vs old adapter performance.
type ABTestMetrics struct {
	NewAdapterQueries  int     `json:"new_adapter_queries"`
	NewAdapterScore    float64 `json:"new_adapter_score"`
	OldAdapterQueries  int     `json:"old_adapter_queries"`
	OldAdapterScore    float64 `json:"old_adapter_score"`
}

// TrainFunc submits a LoRA training job. The implementation depends on the
// training backend (vLLM, Tinker, local PEFT, etc.).
// Returns the path to the trained adapter weights.
type TrainFunc func(ctx context.Context, job TrainJob) (*TrainResult, error)

// TrainJob describes a LoRA training task.
type TrainJob struct {
	BaseModel       string   `json:"base_model"`
	DataPath        string   `json:"data_path"`         // JSONL training data
	OutputDir       string   `json:"output_dir"`        // where to save adapter weights
	AdapterName     string   `json:"adapter_name"`      // unique name for this adapter version
	NumEpochs       int      `json:"num_epochs"`        // default 3
	LoRARank        int      `json:"lora_rank"`         // default 16
	LearningRate    float64  `json:"learning_rate"`     // default 2e-4
	MaxSeqLength    int      `json:"max_seq_length"`    // default 2048
	Seed            int      `json:"seed"`              // default 42
	TargetModules   []string `json:"target_modules"`    // auto-inferred if empty
	TrustRemoteCode bool     `json:"trust_remote_code"` // default false
	ResumeFrom      string   `json:"resume_from"`       // previous adapter path for incremental training
}

// TrainResult is the output of a training job.
type TrainResult struct {
	AdapterName string        `json:"adapter_name"`
	AdapterPath string        `json:"adapter_path"`
	Samples     int           `json:"samples"`
	Epochs      int           `json:"epochs"`
	FinalLoss   float64       `json:"final_loss"`
	Duration    time.Duration `json:"duration"`
	Success     bool          `json:"success"`
	Error       string        `json:"error,omitempty"`
}

// EvalFunc runs quality checks on a newly trained adapter.
// Returns a score [0,1] and detailed results.
type EvalFunc func(ctx context.Context, adapterName string, evalData []EvalSample) (*EvalResult, error)

// EvalSample is a single evaluation example.
type EvalSample struct {
	Input    string `json:"input"`
	Expected string `json:"expected"`
}

// EvalResult contains evaluation metrics for a trained adapter.
type EvalResult struct {
	Score       float64 `json:"score"`
	Accuracy    float64 `json:"accuracy"`
	Consistency float64 `json:"consistency"`
	Regression  float64 `json:"regression"` // how much worse than baseline (0 = no regression)
	Samples     int     `json:"samples"`
	Passed      bool    `json:"passed"`
	Details     string  `json:"details,omitempty"`
}

// NewLoRAScheduler creates a LoRA training lifecycle scheduler.
func NewLoRAScheduler(l *ldg.Ledger, adapter *LoRAAdapter, brain *LocalBrain, cfg SchedulerConfig) *LoRAScheduler {
	return &LoRAScheduler{
		ledger:       l,
		adapter:      adapter,
		brain:        brain,
		config:       cfg,
		tenantStates: make(map[string]*SchedulerState),
	}
}

// TenantState returns the scheduler state for a specific tenant.
// Creates a new state if the tenant hasn't been seen before.
func (ls *LoRAScheduler) TenantState(tenantID string) SchedulerState {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	if tenantID == "" || tenantID == "default" {
		return ls.state
	}
	if s, ok := ls.tenantStates[tenantID]; ok {
		return *s
	}
	return SchedulerState{}
}

func (ls *LoRAScheduler) getOrCreateTenantState(tenantID string) *SchedulerState {
	if tenantID == "" || tenantID == "default" {
		return &ls.state
	}
	if s, ok := ls.tenantStates[tenantID]; ok {
		return s
	}
	s := &SchedulerState{}
	ls.tenantStates[tenantID] = s
	return s
}

// SetTrainFunc sets the training backend callback.
func (ls *LoRAScheduler) SetTrainFunc(fn TrainFunc)  { ls.trainFn = fn }

// SetEvalFunc sets the evaluation callback.
func (ls *LoRAScheduler) SetEvalFunc(fn EvalFunc) { ls.evalFn = fn }

// SetMetrics enables training observability.
func (ls *LoRAScheduler) SetMetrics(m *TrainingMetrics) { ls.metrics = m }

// Metrics returns the training metrics recorder (may be nil).
func (ls *LoRAScheduler) Metrics() *TrainingMetrics { return ls.metrics }

// SetKVStore enables Ledger KV-backed persistence for scheduler state.
func (ls *LoRAScheduler) SetKVStore(kvs *iledger.KVConfigStore) {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	ls.kvs = kvs
	ls.loadStateFromKV()
}

// State returns a snapshot of the current scheduler state.
func (ls *LoRAScheduler) State() SchedulerState {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	return ls.state
}

// CheckAndTrigger checks if conditions are met for a new training run,
// and if so, executes the full pipeline. This is the main entry point
// called by the NightScheduler or a cron trigger.
func (ls *LoRAScheduler) CheckAndTrigger(ctx context.Context, tenantID string) error {
	ls.mu.Lock()

	if ls.trainFn == nil {
		ls.mu.Unlock()
		return fmt.Errorf("lora_scheduler: no training function configured")
	}

	tstate := ls.getOrCreateTenantState(tenantID)
	if time.Since(tstate.LastTrainTime) < ls.config.MinInterval {
		ls.mu.Unlock()
		slog.Debug("lora_scheduler: too soon since last train", "tenant", tenantID, "last", tstate.LastTrainTime)
		return nil
	}

	samples, dataPath, err := ls.countAvailableSamples()
	if err != nil {
		ls.mu.Unlock()
		return fmt.Errorf("lora_scheduler: count samples: %w", err)
	}
	if samples < ls.config.MinSamples {
		ls.mu.Unlock()
		slog.Debug("lora_scheduler: insufficient samples", "have", samples, "need", ls.config.MinSamples)
		return nil
	}

	slog.Info("lora_scheduler: triggering training",
		"tenant", tenantID,
		"samples", samples,
		"data_path", dataPath,
	)
	ls.mu.Unlock()

	return ls.runPipeline(ctx, tenantID, dataPath)
}

func (ls *LoRAScheduler) runPipeline(ctx context.Context, tenantID, dataPath string) error {
	ls.mu.Lock()
	filterEnabled := ls.config.FilterEnabled
	filterConfig := ls.config.FilterConfig
	minSamples := ls.config.MinSamples
	baseModel := ls.config.BaseModel
	adapterDir := ls.config.AdapterDir
	tstate := ls.getOrCreateTenantState(tenantID)
	var resumeFrom string
	if tstate.CurrentAdapter != "" && tstate.LastTrainResult != nil {
		prevPath := tstate.LastTrainResult.AdapterPath
		if prevPath != "" {
			if _, err := os.Stat(prevPath); err == nil {
				resumeFrom = prevPath
			}
		}
	}
	ls.mu.Unlock()

	// Data quality filtering
	if filterEnabled {
		filter := NewTrainingFilter(filterConfig)
		filteredPath, stats, err := filter.FilterFile(dataPath)
		if err != nil {
			slog.Warn("lora_scheduler: filter failed, using unfiltered data", "err", err)
		} else if stats.Kept < minSamples {
			slog.Warn("lora_scheduler: too few samples after filtering",
				"before", stats.TotalRead,
				"after", stats.Kept,
				"need", minSamples,
			)
			return nil
		} else {
			slog.Info("lora_scheduler: data filtered",
				"before", stats.TotalRead,
				"after", stats.Kept,
				"dropped_dup", stats.DroppedDup,
				"dropped_quality", stats.DroppedTooShort+stats.DroppedTooLong+stats.DroppedGarbage,
			)
			dataPath = filteredPath
		}
	}

	adapterName := fmt.Sprintf("yunque-%s-%s", tenantID, time.Now().Format("20060102"))
	outputDir := filepath.Join(adapterDir, adapterName)
	os.MkdirAll(outputDir, 0755)

	if resumeFrom != "" {
		slog.Info("lora_scheduler: incremental training from previous adapter",
			"tenant", tenantID,
			"resume_from", resumeFrom,
		)
	}

	job := TrainJob{
		BaseModel:    baseModel,
		DataPath:     dataPath,
		OutputDir:    outputDir,
		AdapterName:  adapterName,
		NumEpochs:    3,
		LoRARank:     16,
		LearningRate: 2e-4,
		MaxSeqLength: 2048,
		Seed:         42,
		ResumeFrom:   resumeFrom,
	}

	slog.Info("lora_scheduler: starting training", "job", adapterName)
	trainStart := time.Now()
	result, err := ls.trainFn(ctx, job)

	record := TrainingRecord{
		TenantID:     tenantID,
		AdapterName:  adapterName,
		BaseModel:    ls.config.BaseModel,
		StartTime:    trainStart,
		LoRARank:     job.LoRARank,
		LearningRate: job.LearningRate,
		Incremental:  resumeFrom != "",
		ResumeFrom:   resumeFrom,
	}

	if err != nil {
		record.EndTime = time.Now()
		record.Duration = record.EndTime.Sub(record.StartTime)
		record.Error = err.Error()
		ls.recordMetrics(record)
		return fmt.Errorf("lora_scheduler: training failed: %w", err)
	}

	record.Success = result.Success
	record.FinalLoss = result.FinalLoss
	record.Samples = result.Samples
	record.Epochs = result.Epochs
	record.Duration = result.Duration
	record.EndTime = trainStart.Add(result.Duration)

	if !result.Success {
		record.Error = result.Error
		ls.recordMetrics(record)
		return fmt.Errorf("lora_scheduler: training unsuccessful: %s", result.Error)
	}

	ls.mu.Lock()
	tstate = ls.getOrCreateTenantState(tenantID)
	tstate.LastTrainTime = time.Now()
	tstate.LastTrainResult = result
	tstate.TotalTrains++
	ls.state.LastTrainTime = tstate.LastTrainTime
	ls.state.LastTrainResult = tstate.LastTrainResult
	ls.state.TotalTrains++
	ls.mu.Unlock()
	slog.Info("lora_scheduler: training complete",
		"adapter", adapterName,
		"loss", result.FinalLoss,
		"duration", result.Duration,
	)

	if ls.evalFn != nil {
		evalResult, err := ls.evaluate(ctx, adapterName)
		if err != nil {
			slog.Warn("lora_scheduler: evaluation failed, skipping deploy", "err", err)
			ls.recordMetrics(record)
			return err
		}
		record.EvalScore = evalResult.Score
		record.EvalPassed = evalResult.Passed
		if !evalResult.Passed {
			slog.Warn("lora_scheduler: evaluation failed quality gate",
				"score", evalResult.Score,
				"threshold", ls.config.EvalMinScore,
			)
			ls.recordMetrics(record)
			return fmt.Errorf("quality gate failed: score %.2f < %.2f", evalResult.Score, ls.config.EvalMinScore)
		}
		slog.Info("lora_scheduler: evaluation passed",
			"score", evalResult.Score,
			"accuracy", evalResult.Accuracy,
		)
	}

	ls.mu.Lock()
	tstate = ls.getOrCreateTenantState(tenantID)
	if err := ls.deploy(ctx, tstate, adapterName, result.AdapterPath); err != nil {
		ls.mu.Unlock()
		ls.recordMetrics(record)
		return fmt.Errorf("lora_scheduler: deploy failed: %w", err)
	}
	ls.mu.Unlock()

	record.Deployed = true
	ls.recordMetrics(record)
	return nil
}

func (ls *LoRAScheduler) recordMetrics(r TrainingRecord) {
	if ls.metrics != nil {
		ls.metrics.Record(r)
	}
}

func (ls *LoRAScheduler) evaluate(ctx context.Context, adapterName string) (*EvalResult, error) {
	evalSamples := ls.generateEvalSamples(ctx)
	if len(evalSamples) == 0 {
		return &EvalResult{Score: 1.0, Passed: true, Details: "no eval samples available, auto-pass"}, nil
	}
	return ls.evalFn(ctx, adapterName, evalSamples)
}

func (ls *LoRAScheduler) deploy(ctx context.Context, tstate *SchedulerState, adapterName, adapterPath string) error {
	if ls.adapter == nil {
		slog.Info("lora_scheduler: no adapter manager, skipping deploy", "adapter", adapterName)
		return nil
	}

	tstate.PreviousAdapter = tstate.CurrentAdapter
	ls.state.PreviousAdapter = ls.state.CurrentAdapter

	if err := ls.adapter.Load(ctx, adapterName, adapterPath, ls.config.BaseModel); err != nil {
		return err
	}

	tstate.CurrentAdapter = adapterName
	tstate.ABTestActive = true
	tstate.ABTestStart = time.Now()
	tstate.ABTestMetrics = ABTestMetrics{}

	ls.state.CurrentAdapter = adapterName
	ls.state.ABTestActive = true
	ls.state.ABTestStart = time.Now()
	ls.state.ABTestMetrics = ABTestMetrics{}

	slog.Info("lora_scheduler: deployed new adapter, A/B test started",
		"new", adapterName,
		"previous", ls.state.PreviousAdapter,
		"duration", ls.config.ABTestDuration,
	)

	ls.persistState()
	return nil
}

// Rollback reverts to the previous LoRA adapter.
func (ls *LoRAScheduler) Rollback(ctx context.Context) error {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	if ls.state.PreviousAdapter == "" {
		return fmt.Errorf("lora_scheduler: no previous adapter to rollback to")
	}

	current := ls.state.CurrentAdapter

	if ls.adapter != nil && current != "" {
		ls.adapter.Unload(ctx, current)
	}

	ls.state.CurrentAdapter = ls.state.PreviousAdapter
	ls.state.PreviousAdapter = ""
	ls.state.ABTestActive = false
	ls.state.TotalRollbacks++

	slog.Info("lora_scheduler: rolled back",
		"from", current,
		"to", ls.state.CurrentAdapter,
	)

	ls.persistState()
	return nil
}

// RecordABMetric records a quality signal during A/B testing.
func (ls *LoRAScheduler) RecordABMetric(isNewAdapter bool, score float64) {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	if !ls.state.ABTestActive {
		return
	}

	if isNewAdapter {
		ls.state.ABTestMetrics.NewAdapterQueries++
		n := float64(ls.state.ABTestMetrics.NewAdapterQueries)
		ls.state.ABTestMetrics.NewAdapterScore = (ls.state.ABTestMetrics.NewAdapterScore*(n-1) + score) / n
	} else {
		ls.state.ABTestMetrics.OldAdapterQueries++
		n := float64(ls.state.ABTestMetrics.OldAdapterQueries)
		ls.state.ABTestMetrics.OldAdapterScore = (ls.state.ABTestMetrics.OldAdapterScore*(n-1) + score) / n
	}

	if time.Since(ls.state.ABTestStart) > ls.config.ABTestDuration {
		ls.finalizeABTest()
	}
}

func (ls *LoRAScheduler) finalizeABTest() {
	m := ls.state.ABTestMetrics
	ls.state.ABTestActive = false

	if m.NewAdapterQueries < 10 || m.OldAdapterQueries < 10 {
		slog.Info("lora_scheduler: A/B test insufficient data, keeping new adapter")
		return
	}

	improvement := m.NewAdapterScore - m.OldAdapterScore
	if improvement < -0.05 {
		slog.Warn("lora_scheduler: A/B test shows regression, initiating rollback",
			"new_score", m.NewAdapterScore,
			"old_score", m.OldAdapterScore,
			"delta", improvement,
		)
	safego.Go("lora-rollback", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		ls.Rollback(ctx)
	})
	} else {
		slog.Info("lora_scheduler: A/B test passed, keeping new adapter",
			"new_score", m.NewAdapterScore,
			"old_score", m.OldAdapterScore,
			"delta", improvement,
		)
		ls.state.PreviousAdapter = ""
	}
}

// ActiveModel returns the model name to use for inference, including
// LoRA adapter suffix if one is active.
func (ls *LoRAScheduler) ActiveModel() string {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	if ls.adapter == nil || ls.state.CurrentAdapter == "" {
		return ls.config.BaseModel
	}
	return ls.adapter.ModelName(ls.config.BaseModel, ls.state.CurrentAdapter)
}

func (ls *LoRAScheduler) countAvailableSamples() (int, string, error) {
	dataDir := ls.config.TrainingDataDir
	if dataDir == "" {
		dataDir = "./data/training"
	}

	entries, err := os.ReadDir(dataDir)
	if err != nil {
		return 0, "", err
	}

	total := 0
	var files []string
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".jsonl" {
			continue
		}
		path := filepath.Join(dataDir, e.Name())
		count := countJSONLLines(path)
		total += count
		if count > 0 {
			files = append(files, path)
		}
	}
	if len(files) == 0 {
		return 0, "", nil
	}
	if len(files) == 1 {
		return total, files[0], nil
	}

	merged, err := ls.mergeJSONLFiles(files, dataDir)
	if err != nil {
		return total, files[len(files)-1], nil
	}
	return total, merged, nil
}

func (ls *LoRAScheduler) mergeJSONLFiles(files []string, dataDir string) (string, error) {
	merged := filepath.Join(dataDir, fmt.Sprintf("merged_%s.jsonl", time.Now().Format("20060102_150405")))
	out, err := os.Create(merged)
	if err != nil {
		return "", err
	}
	defer out.Close()

	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		out.Write(data)
		if len(data) > 0 && data[len(data)-1] != '\n' {
			out.Write([]byte("\n"))
		}
	}
	return merged, nil
}

func countJSONLLines(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	count := 0
	for _, b := range data {
		if b == '\n' {
			count++
		}
	}
	return count
}

func (ls *LoRAScheduler) generateEvalSamples(ctx context.Context) []EvalSample {
	if ls.ledger == nil {
		return nil
	}

	entries, err := ls.ledger.Memory.Search(ctx, ldg.MemoryQuery{
		Kinds: []ldg.MemoryKind{ldg.MemoryExperience},
		Limit: 20,
	})
	if err != nil || len(entries) == 0 {
		return nil
	}

	var samples []EvalSample
	for _, e := range entries {
		if e.Source != "training_data" {
			continue
		}
		var pair struct {
			UserMessage string `json:"user_message"`
			AssistReply string `json:"assist_reply"`
		}
		if json.Unmarshal([]byte(e.Content), &pair) == nil && pair.UserMessage != "" {
			samples = append(samples, EvalSample{
				Input:    pair.UserMessage,
				Expected: pair.AssistReply,
			})
		}
		if len(samples) >= 10 {
			break
		}
	}
	return samples
}

type persistedState struct {
	Global  SchedulerState               `json:"global"`
	Tenants map[string]*SchedulerState   `json:"tenants,omitempty"`
}

func (ls *LoRAScheduler) persistState() {
	ps := persistedState{
		Global:  ls.state,
		Tenants: ls.tenantStates,
	}
	if ls.kvs != nil {
		if err := ls.kvs.Put(context.Background(), "state", ps); err != nil {
			slog.Warn("lora_scheduler: kv save failed, falling back to file", "err", err)
		} else {
			return
		}
	}
	stateDir := ls.config.AdapterDir
	if stateDir == "" {
		stateDir = "./data/adapters"
	}
	os.MkdirAll(stateDir, 0755)

	data, err := json.MarshalIndent(ps, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(filepath.Join(stateDir, "scheduler_state.json"), data, 0644)
}

func (ls *LoRAScheduler) loadStateFromKV() {
	if ls.kvs == nil {
		return
	}
	var ps persistedState
	found, err := ls.kvs.Get(context.Background(), "state", &ps)
	if err != nil {
		var state SchedulerState
		if found2, err2 := ls.kvs.Get(context.Background(), "state", &state); err2 == nil && found2 {
			ls.state = state
			slog.Info("lora_scheduler: loaded legacy state from Ledger KV")
			return
		}
		slog.Warn("lora_scheduler: kv load failed", "err", err)
		return
	}
	if found {
		ls.state = ps.Global
		if ps.Tenants != nil {
			ls.tenantStates = ps.Tenants
		}
		slog.Info("lora_scheduler: loaded state from Ledger KV", "tenants", len(ls.tenantStates))
	}
}

// LoadState restores scheduler state from disk.
func (ls *LoRAScheduler) LoadState() {
	stateDir := ls.config.AdapterDir
	if stateDir == "" {
		stateDir = "./data/adapters"
	}
	data, err := os.ReadFile(filepath.Join(stateDir, "scheduler_state.json"))
	if err != nil {
		return
	}
	var ps persistedState
	if json.Unmarshal(data, &ps) == nil && (ps.Global.TotalTrains > 0 || ps.Global.CurrentAdapter != "" || ps.Tenants != nil) {
		ls.state = ps.Global
		if ps.Tenants != nil {
			ls.tenantStates = ps.Tenants
		}
		return
	}
	json.Unmarshal(data, &ls.state)
}
