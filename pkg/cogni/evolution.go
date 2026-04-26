package cogni

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ── Experiment Types ──

// Experiment records a single skill evolution attempt.
type Experiment struct {
	ID            string    `json:"id"`
	CogniID       string    `json:"cogni_id"`
	Date          time.Time `json:"date"`
	Change        string    `json:"change"`
	BaselineScore float64   `json:"baseline_score"`
	ResultScore   float64   `json:"result_score"`
	Delta         float64   `json:"delta"`
	Status        string    `json:"status"` // kept | reverted | pending
	Reason        string    `json:"reason,omitempty"`
	AffectedTasks []string  `json:"affected_tasks,omitempty"`
}

// BenchResult is the structured output of a benchmark run.
type BenchResult struct {
	Score       float64        `json:"score"`
	Total       int            `json:"total"`
	Passed      int            `json:"passed"`
	Failed      int            `json:"failed"`
	Failures    []TaskFailure  `json:"failures,omitempty"`
	Duration    time.Duration  `json:"duration"`
}

// TaskFailure records one benchmark task that failed.
type TaskFailure struct {
	TaskID   string `json:"task_id"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
	Error    string `json:"error,omitempty"`
}

// SkillMutation is a proposed change to a skill or declaration.
type SkillMutation struct {
	SkillName   string         `json:"skill_name"`
	MutationType string        `json:"mutation_type"` // parameter | prompt | timeout | new_skill
	Before      map[string]any `json:"before,omitempty"`
	After       map[string]any `json:"after"`
	Rationale   string         `json:"rationale"`
}

// ── Benchmark Runner ──

// BenchFunc is the host-provided benchmark execution primitive.
// It runs all benchmark tasks against the current skill set and returns results.
type BenchFunc func(ctx context.Context, cogniID string) (*BenchResult, error)

// ── Failure Analyzer ──

// AnalyzeFunc uses LLM to analyze benchmark failures and propose mutations.
type AnalyzeFunc func(ctx context.Context, failures []TaskFailure) ([]SkillMutation, error)

// ── Evolution Engine ──

// EvolutionConfig controls the skill evolution loop behavior.
type EvolutionConfig struct {
	MaxRounds       int     `json:"max_rounds"`       // max optimization rounds (default 5)
	MinImprovement  float64 `json:"min_improvement"`  // minimum score delta to keep (default 0.5)
	TargetScore     float64 `json:"target_score"`     // stop when reached (default 0, meaning no target)
	StaleRounds     int     `json:"stale_rounds"`     // stop after N rounds with no improvement (default 3)
	AutoRevert      bool    `json:"auto_revert"`      // revert on regression (default true)
}

func DefaultEvolutionConfig() EvolutionConfig {
	return EvolutionConfig{
		MaxRounds:      5,
		MinImprovement: 0.5,
		StaleRounds:    3,
		AutoRevert:     true,
	}
}

// EvolutionEngine manages the benchmark-driven skill optimization loop.
type EvolutionEngine struct {
	mu          sync.Mutex
	registry    *Registry
	bench       BenchFunc
	analyze     AnalyzeFunc
	applyMut    func(cogniID string, mutations []SkillMutation) error
	revertMut   func(cogniID string, mutations []SkillMutation) error
	cfg         EvolutionConfig
	experiments map[string][]Experiment // keyed by cogni ID
	dataDir     string
	running     map[string]bool // cogni IDs currently evolving
}

func NewEvolutionEngine(cfg EvolutionConfig, dataDir string) *EvolutionEngine {
	if cfg.MaxRounds <= 0 {
		cfg.MaxRounds = 5
	}
	if cfg.StaleRounds <= 0 {
		cfg.StaleRounds = 3
	}
	return &EvolutionEngine{
		cfg:         cfg,
		dataDir:     dataDir,
		experiments: make(map[string][]Experiment),
		running:     make(map[string]bool),
	}
}

func (ee *EvolutionEngine) SetBenchFunc(fn BenchFunc)     { ee.bench = fn }
func (ee *EvolutionEngine) SetAnalyzeFunc(fn AnalyzeFunc) { ee.analyze = fn }
func (ee *EvolutionEngine) SetRegistry(r *Registry)       { ee.registry = r }

func (ee *EvolutionEngine) SetApplyFunc(fn func(cogniID string, mutations []SkillMutation) error) {
	ee.applyMut = fn
}

func (ee *EvolutionEngine) SetRevertFunc(fn func(cogniID string, mutations []SkillMutation) error) {
	ee.revertMut = fn
}

// IsRunning reports whether an evolution loop is currently active for a cogni.
func (ee *EvolutionEngine) IsRunning(cogniID string) bool {
	ee.mu.Lock()
	defer ee.mu.Unlock()
	return ee.running[cogniID]
}

// Run executes the skill evolution loop for a specific cogni.
// It runs benchmark → analyze failures → mutate → re-benchmark → gate.
func (ee *EvolutionEngine) Run(ctx context.Context, cogniID string) ([]Experiment, error) {
	ee.mu.Lock()
	if ee.running[cogniID] {
		ee.mu.Unlock()
		return nil, fmt.Errorf("evolution already running for %q", cogniID)
	}
	ee.running[cogniID] = true
	ee.mu.Unlock()

	defer func() {
		ee.mu.Lock()
		delete(ee.running, cogniID)
		ee.mu.Unlock()
	}()

	if ee.bench == nil || ee.analyze == nil {
		return nil, fmt.Errorf("evolution: bench or analyze function not configured")
	}

	var results []Experiment
	staleCount := 0
	bestScore := 0.0

	// Step 1: Baseline benchmark
	baseline, err := ee.bench(ctx, cogniID)
	if err != nil {
		return nil, fmt.Errorf("evolution: baseline bench failed: %w", err)
	}
	bestScore = baseline.Score
	slog.Info("evolution: baseline", "cogni", cogniID, "score", baseline.Score)

	for round := 0; round < ee.cfg.MaxRounds; round++ {
		if ctx.Err() != nil {
			break
		}

		if ee.cfg.TargetScore > 0 && bestScore >= ee.cfg.TargetScore {
			slog.Info("evolution: target score reached", "cogni", cogniID, "score", bestScore)
			break
		}

		if staleCount >= ee.cfg.StaleRounds {
			slog.Info("evolution: stale — stopping", "cogni", cogniID, "rounds", staleCount)
			break
		}

		// Step 2: Analyze failures
		if len(baseline.Failures) == 0 {
			slog.Info("evolution: no failures to analyze", "cogni", cogniID)
			break
		}

		mutations, err := ee.analyze(ctx, baseline.Failures)
		if err != nil || len(mutations) == 0 {
			slog.Warn("evolution: analyze returned no mutations", "cogni", cogniID, "err", err)
			staleCount++
			continue
		}

		// Step 3: Apply mutations
		changeDesc := fmt.Sprintf("round %d: %d mutations", round+1, len(mutations))
		if ee.applyMut != nil {
			if err := ee.applyMut(cogniID, mutations); err != nil {
				slog.Warn("evolution: apply failed", "cogni", cogniID, "err", err)
				staleCount++
				continue
			}
		}

		// Step 4: Re-benchmark
		result, err := ee.bench(ctx, cogniID)
		if err != nil {
			slog.Warn("evolution: re-bench failed", "cogni", cogniID, "err", err)
			staleCount++
			continue
		}

		delta := result.Score - bestScore
		exp := Experiment{
			ID:            fmt.Sprintf("exp-%s-%d", cogniID, time.Now().UnixMilli()),
			CogniID:       cogniID,
			Date:          time.Now(),
			Change:        changeDesc,
			BaselineScore: bestScore,
			ResultScore:   result.Score,
			Delta:         delta,
		}

		// Step 5: Regression gate
		if delta >= ee.cfg.MinImprovement {
			exp.Status = "kept"
			exp.Reason = fmt.Sprintf("score improved by %.1f%%", delta)
			bestScore = result.Score
			staleCount = 0
			slog.Info("evolution: improvement kept",
				"cogni", cogniID, "delta", delta, "new_score", result.Score)
		} else if delta < 0 && ee.cfg.AutoRevert {
			exp.Status = "reverted"
			exp.Reason = fmt.Sprintf("score regressed by %.1f%%", -delta)
			if ee.revertMut != nil {
				_ = ee.revertMut(cogniID, mutations)
			}
			staleCount++
			slog.Info("evolution: regression reverted",
				"cogni", cogniID, "delta", delta)
		} else {
			exp.Status = "reverted"
			exp.Reason = fmt.Sprintf("improvement %.1f%% below threshold %.1f%%", delta, ee.cfg.MinImprovement)
			if ee.revertMut != nil {
				_ = ee.revertMut(cogniID, mutations)
			}
			staleCount++
		}

		results = append(results, exp)
		baseline = result
	}

	// Persist experiments
	ee.mu.Lock()
	ee.experiments[cogniID] = append(ee.experiments[cogniID], results...)
	ee.mu.Unlock()
	ee.persist(cogniID)

	return results, nil
}

// Experiments returns all experiments for a cogni.
func (ee *EvolutionEngine) Experiments(cogniID string) []Experiment {
	ee.mu.Lock()
	defer ee.mu.Unlock()

	if exps, ok := ee.experiments[cogniID]; ok {
		out := make([]Experiment, len(exps))
		copy(out, exps)
		return out
	}
	return nil
}

// AllExperiments returns experiments for all cognis.
func (ee *EvolutionEngine) AllExperiments() map[string][]Experiment {
	ee.mu.Lock()
	defer ee.mu.Unlock()
	out := make(map[string][]Experiment, len(ee.experiments))
	for k, v := range ee.experiments {
		c := make([]Experiment, len(v))
		copy(c, v)
		out[k] = c
	}
	return out
}

func (ee *EvolutionEngine) persist(cogniID string) {
	if ee.dataDir == "" {
		return
	}
	dir := filepath.Join(ee.dataDir, "evolution")
	os.MkdirAll(dir, 0755)

	ee.mu.Lock()
	src := ee.experiments[cogniID]
	exps := make([]Experiment, len(src))
	copy(exps, src)
	ee.mu.Unlock()

	data, err := json.MarshalIndent(exps, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(dir, cogniID+".json"), data, 0644)
}

func (ee *EvolutionEngine) Load(cogniID string) {
	if ee.dataDir == "" {
		return
	}
	path := filepath.Join(ee.dataDir, "evolution", cogniID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var exps []Experiment
	if err := json.Unmarshal(data, &exps); err != nil {
		return
	}
	ee.mu.Lock()
	ee.experiments[cogniID] = exps
	ee.mu.Unlock()
}
