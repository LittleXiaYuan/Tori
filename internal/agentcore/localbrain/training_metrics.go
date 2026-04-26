package localbrain

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// TrainingMetrics records and queries the full training history for
// observability, debugging, and performance tracking.
type TrainingMetrics struct {
	mu      sync.RWMutex
	records []TrainingRecord
	dataDir string
}

// TrainingRecord is a single training run's metadata for observability.
type TrainingRecord struct {
	ID          string        `json:"id"`
	TenantID    string        `json:"tenant_id"`
	AdapterName string        `json:"adapter_name"`
	BaseModel   string        `json:"base_model"`
	StartTime   time.Time     `json:"start_time"`
	EndTime     time.Time     `json:"end_time"`
	Duration    time.Duration `json:"duration"`

	// Training params
	Samples      int     `json:"samples"`
	Epochs       int     `json:"epochs"`
	LoRARank     int     `json:"lora_rank"`
	LearningRate float64 `json:"learning_rate"`

	// Results
	FinalLoss    float64 `json:"final_loss"`
	Success      bool    `json:"success"`
	Error        string  `json:"error,omitempty"`
	Incremental  bool    `json:"incremental"`
	ResumeFrom   string  `json:"resume_from,omitempty"`

	// Filter stats
	FilterStats *FilterStats `json:"filter_stats,omitempty"`

	// Eval results
	EvalScore    float64 `json:"eval_score,omitempty"`
	EvalPassed   bool    `json:"eval_passed,omitempty"`

	// Deploy status
	Deployed     bool   `json:"deployed"`
	RolledBack   bool   `json:"rolled_back"`
}

// TrainingSummary provides aggregate metrics across all training runs.
type TrainingSummary struct {
	TotalRuns      int            `json:"total_runs"`
	SuccessCount   int            `json:"success_count"`
	FailureCount   int            `json:"failure_count"`
	DeployCount    int            `json:"deploy_count"`
	RollbackCount  int            `json:"rollback_count"`
	AvgLoss        float64        `json:"avg_loss"`
	AvgDuration    time.Duration  `json:"avg_duration"`
	AvgSamples     float64        `json:"avg_samples"`
	LastTrainTime  time.Time      `json:"last_train_time"`
	ByTenant       map[string]int `json:"by_tenant"`
}

func NewTrainingMetrics(dataDir string) *TrainingMetrics {
	tm := &TrainingMetrics{
		dataDir: dataDir,
	}
	tm.load()
	return tm
}

// Record adds a training run to the history.
func (tm *TrainingMetrics) Record(r TrainingRecord) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if r.ID == "" {
		r.ID = fmt.Sprintf("%s-%d", r.AdapterName, time.Now().UnixMilli())
	}

	tm.records = append(tm.records, r)
	tm.persist()

	slog.Info("training_metrics: recorded",
		"id", r.ID,
		"tenant", r.TenantID,
		"adapter", r.AdapterName,
		"success", r.Success,
		"loss", r.FinalLoss,
		"samples", r.Samples,
		"duration", r.Duration,
	)
}

// Summary returns aggregate metrics.
func (tm *TrainingMetrics) Summary() TrainingSummary {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	s := TrainingSummary{
		ByTenant: make(map[string]int),
	}

	var totalLoss float64
	var totalDuration time.Duration
	var totalSamples int
	successWithLoss := 0

	for _, r := range tm.records {
		s.TotalRuns++
		if r.Success {
			s.SuccessCount++
			if r.FinalLoss > 0 {
				totalLoss += r.FinalLoss
				successWithLoss++
			}
		} else {
			s.FailureCount++
		}
		if r.Deployed {
			s.DeployCount++
		}
		if r.RolledBack {
			s.RollbackCount++
		}
		totalDuration += r.Duration
		totalSamples += r.Samples
		s.ByTenant[r.TenantID]++

		if r.EndTime.After(s.LastTrainTime) {
			s.LastTrainTime = r.EndTime
		}
	}

	if s.TotalRuns > 0 {
		s.AvgDuration = totalDuration / time.Duration(s.TotalRuns)
		s.AvgSamples = float64(totalSamples) / float64(s.TotalRuns)
	}
	if successWithLoss > 0 {
		s.AvgLoss = totalLoss / float64(successWithLoss)
	}

	return s
}

// Recent returns the last N training records.
func (tm *TrainingMetrics) Recent(n int) []TrainingRecord {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	if n <= 0 || len(tm.records) == 0 {
		return nil
	}
	start := len(tm.records) - n
	if start < 0 {
		start = 0
	}

	result := make([]TrainingRecord, len(tm.records[start:]))
	copy(result, tm.records[start:])
	return result
}

// ForTenant returns all records for a specific tenant.
func (tm *TrainingMetrics) ForTenant(tenantID string) []TrainingRecord {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	var result []TrainingRecord
	for _, r := range tm.records {
		if r.TenantID == tenantID {
			result = append(result, r)
		}
	}
	return result
}

// Count returns total number of records.
func (tm *TrainingMetrics) Count() int {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return len(tm.records)
}

func (tm *TrainingMetrics) persist() {
	if tm.dataDir == "" {
		return
	}
	os.MkdirAll(tm.dataDir, 0755)

	data, err := json.MarshalIndent(tm.records, "", "  ")
	if err != nil {
		slog.Warn("training_metrics: persist failed", "err", err)
		return
	}

	path := filepath.Join(tm.dataDir, "training_history.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		slog.Warn("training_metrics: write failed", "path", path, "err", err)
	}
}

func (tm *TrainingMetrics) load() {
	if tm.dataDir == "" {
		return
	}

	path := filepath.Join(tm.dataDir, "training_history.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	var records []TrainingRecord
	if json.Unmarshal(data, &records) == nil {
		tm.records = records
		slog.Info("training_metrics: loaded history", "count", len(records))
	}
}
