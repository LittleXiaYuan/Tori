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

	ldg "yunque-agent/internal/ledgercore"
)

// EvolutionCoordinator orchestrates the three evolution layers of the agent:
//
//  1. Memory layer   — every task updates the Ledger (facts, experiences)
//  2. Strategy layer — periodic prompt / threshold / routing adjustments
//  3. Weight layer   — rare, expensive LoRA fine-tuning of the small model
//
// Each higher layer is more expensive and harder to reverse, so the coordinator
// uses escalating thresholds: fast cheap updates first, trigger heavier layers
// only when cheaper ones plateau.
//
// Flow on task completion:
//
//	OnTaskComplete(outcome)
//	    │
//	    ├── ALWAYS: record memory entry in Ledger
//	    ├── EVERY N tasks: sample recent outcomes, update strategy
//	    └── IF strategy hit-rate drops below threshold AND
//	        enough new samples accumulated:
//	        → trigger LoRAScheduler.CheckAndTrigger
type EvolutionCoordinator struct {
	mu sync.Mutex

	ledger    *ldg.Ledger
	brain     *LocalBrain
	scheduler *LoRAScheduler
	metrics   *TrainingMetrics

	cfg   CoordinatorConfig
	state CoordinatorState

	stateFile string
}

// CoordinatorConfig controls escalation thresholds.
type CoordinatorConfig struct {
	// Strategy layer triggers
	StrategyInterval        int     // update strategy every N tasks (default 20)
	StrategySuccessTarget   float64 // target success rate; below → more aggressive evolution (default 0.8)
	StrategyMinObservations int     // minimum observations before strategy updates apply (default 50)

	// Weight layer triggers (on top of LoRAScheduler's own MinSamples/MinInterval)
	WeightHitRateThreshold float64       // trigger LoRA if rolling success rate drops below this (default 0.7)
	WeightMinNewTasks      int           // minimum new tasks since last LoRA run (default 100)
	WeightCooldown         time.Duration // minimum time between weight triggers (default 24h)

	// Persistence
	StateDir string
}

// DefaultCoordinatorConfig returns production defaults.
func DefaultCoordinatorConfig() CoordinatorConfig {
	return CoordinatorConfig{
		StrategyInterval:        20,
		StrategySuccessTarget:   0.8,
		StrategyMinObservations: 50,
		WeightHitRateThreshold:  0.7,
		WeightMinNewTasks:       100,
		WeightCooldown:          24 * time.Hour,
	}
}

// CoordinatorState is the coordinator's persistent state.
type CoordinatorState struct {
	TotalTasks         int            `json:"total_tasks"`
	SuccessTasks       int            `json:"success_tasks"`
	TasksSinceStrategy int            `json:"tasks_since_strategy"`
	TasksSinceWeights  int            `json:"tasks_since_weights"`
	LastStrategyUpdate time.Time      `json:"last_strategy_update"`
	LastWeightTrigger  time.Time      `json:"last_weight_trigger"`
	StrategyUpdates    int            `json:"strategy_updates"`
	WeightTriggers     int            `json:"weight_triggers"`
	RollingSuccessRate float64        `json:"rolling_success_rate"`
	RecentWindow       []bool         `json:"recent_window"` // last N task outcomes
	ByTenant           map[string]int `json:"by_tenant"`
}

// TaskOutcome describes a single task completion, used as the input signal
// for evolution decisions.
type TaskOutcome struct {
	TaskID      string
	TenantID    string
	Success     bool
	UserQuery   string
	FinalReply  string
	Reward      float64
	Steps       int
	Backtracks  int
	Duration    time.Duration
	CompletedAt time.Time
}

// EvolutionDecision reports what the coordinator did for a given task.
type EvolutionDecision struct {
	MemoryUpdated    bool   `json:"memory_updated"`
	StrategyUpdated  bool   `json:"strategy_updated"`
	WeightsTriggered bool   `json:"weights_triggered"`
	Reason           string `json:"reason,omitempty"`
}

const rollingWindowSize = 50

// NewEvolutionCoordinator wires the three layers together.
func NewEvolutionCoordinator(
	ledger *ldg.Ledger,
	brain *LocalBrain,
	scheduler *LoRAScheduler,
	metrics *TrainingMetrics,
	cfg CoordinatorConfig,
) *EvolutionCoordinator {
	ec := &EvolutionCoordinator{
		ledger:    ledger,
		brain:     brain,
		scheduler: scheduler,
		metrics:   metrics,
		cfg:       cfg,
		state: CoordinatorState{
			ByTenant: make(map[string]int),
		},
	}
	if cfg.StateDir != "" {
		ec.stateFile = filepath.Join(cfg.StateDir, "evolution_state.json")
		ec.loadState()
	}
	return ec
}

// State returns a snapshot of the coordinator state.
func (ec *EvolutionCoordinator) State() CoordinatorState {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	return ec.state
}

// OnTaskComplete is the single entry point called after every task.
// Returns what the coordinator decided to do.
func (ec *EvolutionCoordinator) OnTaskComplete(ctx context.Context, outcome TaskOutcome) EvolutionDecision {
	ec.mu.Lock()

	decision := EvolutionDecision{}

	ec.state.TotalTasks++
	ec.state.TasksSinceStrategy++
	ec.state.TasksSinceWeights++
	if outcome.Success {
		ec.state.SuccessTasks++
	}
	if outcome.TenantID != "" {
		ec.state.ByTenant[outcome.TenantID]++
	}

	ec.updateRollingWindow(outcome.Success)

	doStrategy := ec.shouldUpdateStrategy()
	doWeights := ec.shouldTriggerWeights()
	ec.mu.Unlock()

	// Layer 1: always update memory (I/O, no lock)
	if ec.updateMemory(ctx, outcome) {
		decision.MemoryUpdated = true
	}

	// Layer 2: periodic strategy update (no lock needed for brain.RecordFeedback)
	if doStrategy {
		if ec.updateStrategy(outcome) {
			decision.StrategyUpdated = true
			ec.mu.Lock()
			ec.state.TasksSinceStrategy = 0
			ec.state.LastStrategyUpdate = time.Now()
			ec.state.StrategyUpdates++
			ec.mu.Unlock()
		}
	}

	// Layer 3: weight evolution (LoRA training) — conditional on signal
	if doWeights {
		if err := ec.triggerWeights(ctx, outcome.TenantID); err != nil {
			slog.Warn("evolution: weight trigger failed", "err", err)
			decision.Reason = fmt.Sprintf("weight trigger failed: %v", err)
		} else {
			decision.WeightsTriggered = true
			ec.mu.Lock()
			ec.state.TasksSinceWeights = 0
			ec.state.LastWeightTrigger = time.Now()
			ec.state.WeightTriggers++
			ec.mu.Unlock()
		}
	}

	ec.mu.Lock()
	ec.persistState()
	successRate := ec.state.RollingSuccessRate
	ec.mu.Unlock()

	if decision.MemoryUpdated || decision.StrategyUpdated || decision.WeightsTriggered {
		slog.Info("evolution: decision",
			"task", outcome.TaskID,
			"tenant", outcome.TenantID,
			"memory", decision.MemoryUpdated,
			"strategy", decision.StrategyUpdated,
			"weights", decision.WeightsTriggered,
			"success_rate", fmt.Sprintf("%.3f", successRate),
		)
	}

	return decision
}

// ── Layer 1: Memory ──

func (ec *EvolutionCoordinator) updateMemory(ctx context.Context, outcome TaskOutcome) bool {
	if ec.ledger == nil {
		return false
	}

	tenantID := outcome.TenantID
	if tenantID == "" {
		tenantID = "default"
	}

	content := map[string]interface{}{
		"task_id":      outcome.TaskID,
		"user_query":   outcome.UserQuery,
		"final_reply":  outcome.FinalReply,
		"success":      outcome.Success,
		"reward":       outcome.Reward,
		"steps":        outcome.Steps,
		"backtracks":   outcome.Backtracks,
		"duration_ms":  outcome.Duration.Milliseconds(),
		"completed_at": outcome.CompletedAt.Format(time.RFC3339),
	}
	contentJSON, err := json.Marshal(content)
	if err != nil {
		return false
	}

	entry := &ldg.MemoryEntry{
		TenantID: tenantID,
		Kind:     ldg.MemoryExperience,
		Source:   "evolution_coordinator",
		Key:      "evolution.task." + outcome.TaskID,
		Content:  string(contentJSON),
	}
	if outcome.Reward > 0 {
		entry.Confidence = outcome.Reward
	}

	if err := ec.ledger.Memory.Put(ctx, entry); err != nil {
		slog.Warn("evolution: memory put failed", "task", outcome.TaskID, "err", err)
		return false
	}
	return true
}

// ── Layer 2: Strategy ──

func (ec *EvolutionCoordinator) shouldUpdateStrategy() bool {
	if ec.state.TasksSinceStrategy < ec.cfg.StrategyInterval {
		return false
	}
	if ec.state.TotalTasks < ec.cfg.StrategyMinObservations {
		return false
	}
	return true
}

func (ec *EvolutionCoordinator) updateStrategy(outcome TaskOutcome) bool {
	if ec.brain == nil {
		return false
	}

	successRate := ec.state.RollingSuccessRate
	target := ec.cfg.StrategySuccessTarget

	// Record feedback into LocalBrain so its internal UserPatterns adapts.
	tenantID := outcome.TenantID
	if tenantID == "" {
		tenantID = "default"
	}
	tier := "local"
	if outcome.Backtracks > 0 || outcome.Steps > 5 {
		tier = "smart"
	}
	intent := Intent{
		Category:   coarseIntentKey(outcome.UserQuery),
		Complexity: complexityFromSteps(outcome.Steps),
		Confidence: outcome.Reward,
	}
	ec.brain.RecordFeedback(tenantID, outcome.UserQuery, intent, tier, outcome.Backtracks > 0, outcome.Success)

	slog.Info("evolution: strategy updated",
		"tenant", tenantID,
		"success_rate", fmt.Sprintf("%.3f", successRate),
		"target", target,
		"observations", ec.state.TotalTasks,
	)
	return true
}

func complexityFromSteps(steps int) string {
	switch {
	case steps <= 3:
		return "simple"
	case steps <= 8:
		return "medium"
	default:
		return "hard"
	}
}

// ── Layer 3: Weights (LoRA) ──

func (ec *EvolutionCoordinator) shouldTriggerWeights() bool {
	if ec.scheduler == nil {
		return false
	}
	if ec.state.TasksSinceWeights < ec.cfg.WeightMinNewTasks {
		return false
	}
	if !ec.state.LastWeightTrigger.IsZero() && time.Since(ec.state.LastWeightTrigger) < ec.cfg.WeightCooldown {
		return false
	}
	// Trigger when rolling success rate is below threshold — the cheaper layers
	// haven't been enough, so it's time to update model weights.
	if ec.state.RollingSuccessRate >= ec.cfg.WeightHitRateThreshold {
		return false
	}
	// Require enough evidence
	if len(ec.state.RecentWindow) < rollingWindowSize/2 {
		return false
	}
	return true
}

func (ec *EvolutionCoordinator) triggerWeights(ctx context.Context, tenantID string) error {
	if tenantID == "" {
		tenantID = "default"
	}
	slog.Info("evolution: triggering weight update (LoRA)",
		"tenant", tenantID,
		"success_rate", fmt.Sprintf("%.3f", ec.state.RollingSuccessRate),
		"threshold", ec.cfg.WeightHitRateThreshold,
	)
	return ec.scheduler.CheckAndTrigger(ctx, tenantID)
}

// ── Rolling stats ──

func (ec *EvolutionCoordinator) updateRollingWindow(success bool) {
	ec.state.RecentWindow = append(ec.state.RecentWindow, success)
	if len(ec.state.RecentWindow) > rollingWindowSize {
		ec.state.RecentWindow = ec.state.RecentWindow[len(ec.state.RecentWindow)-rollingWindowSize:]
	}

	successes := 0
	for _, s := range ec.state.RecentWindow {
		if s {
			successes++
		}
	}
	if len(ec.state.RecentWindow) > 0 {
		ec.state.RollingSuccessRate = float64(successes) / float64(len(ec.state.RecentWindow))
	}
}

func coarseIntentKey(query string) string {
	trimmed := ""
	for _, r := range query {
		if r == ' ' || r == '\n' || r == '\t' {
			break
		}
		trimmed += string(r)
		if len(trimmed) >= 16 {
			break
		}
	}
	return trimmed
}

// ── Persistence ──

func (ec *EvolutionCoordinator) persistState() {
	if ec.stateFile == "" {
		return
	}
	os.MkdirAll(filepath.Dir(ec.stateFile), 0755)
	data, err := json.MarshalIndent(ec.state, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(ec.stateFile, data, 0644)
}

func (ec *EvolutionCoordinator) loadState() {
	if ec.stateFile == "" {
		return
	}
	data, err := os.ReadFile(ec.stateFile)
	if err != nil {
		return
	}
	var s CoordinatorState
	if json.Unmarshal(data, &s) == nil {
		ec.state = s
		if ec.state.ByTenant == nil {
			ec.state.ByTenant = make(map[string]int)
		}
		slog.Info("evolution: loaded state",
			"total_tasks", s.TotalTasks,
			"success_rate", fmt.Sprintf("%.3f", s.RollingSuccessRate),
		)
	}
}
