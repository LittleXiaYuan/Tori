package ledger

import (
	"context"
	"encoding/json"
	"math"
	"sync"
	"time"
)

// MetaCognition is the second-order reasoning engine: it evaluates
// the quality of the agent's own reflection and decision-making
// processes, then dynamically adjusts strategy parameters.
//
// Core metrics:
//   - Reflection ROI: did reflecting lead to better subsequent decisions?
//   - Strategy efficiency: which Think→Act patterns succeed most often?
//   - Cognitive load: is the agent thrashing or making steady progress?
//
// Reference: inspired by AtomMem (arXiv:2601.08323) and AgeMem
// (arXiv:2601.01885) approaches to learned policy over cognitive ops.
type MetaCognition struct {
	mu      sync.Mutex
	backend Backend
	events  *EventStore

	episodes []metacogEpisode
	stats    MetaCogStats
}

type metacogEpisode struct {
	TaskID         string
	ReflectCount   int
	BacktrackCount int
	DecisionCount  int
	AvgConfidence  float64
	TaskSuccess    bool
	TotalSteps     int
	Duration       time.Duration
	AnalyzedAt     time.Time
}

// MetaCogStats aggregates cross-task metacognitive metrics.
type MetaCogStats struct {
	TotalEpisodes int     `json:"total_episodes"`
	ReflectROI    float64 `json:"reflect_roi"`    // P(success|reflected) / P(success|no_reflect)
	AvgCogLoad    float64 `json:"avg_cog_load"`   // normalized cognitive load (0=calm, 1=thrashing)
	OptimalReflectRate float64 `json:"optimal_reflect_rate"` // recommended reflections per decision

	SuccessWithReflect    int `json:"success_with_reflect"`
	FailWithReflect       int `json:"fail_with_reflect"`
	SuccessWithoutReflect int `json:"success_without_reflect"`
	FailWithoutReflect    int `json:"fail_without_reflect"`
}

// NewMetaCognition creates a metacognitive analyzer.
func NewMetaCognition(backend Backend, events *EventStore) *MetaCognition {
	return &MetaCognition{backend: backend, events: events}
}

// AnalyzeTask processes a completed task's reasoning trace and records
// an episode for cross-task learning.
func (mc *MetaCognition) AnalyzeTask(ctx context.Context, taskID string, taskSuccess bool) (*MetaCogEpisodeResult, error) {
	trace, err := mc.events.GetReasoningTrace(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if trace.Summary == nil || trace.Summary.TotalSteps == 0 {
		return nil, nil
	}

	s := trace.Summary
	var duration time.Duration
	if len(trace.Events) >= 2 {
		duration = trace.Events[len(trace.Events)-1].CreatedAt.Sub(trace.Events[0].CreatedAt)
	}

	ep := metacogEpisode{
		TaskID:         taskID,
		ReflectCount:   s.Reflections,
		BacktrackCount: s.Backtracks,
		DecisionCount:  s.Decisions,
		AvgConfidence:  s.AvgConfidence,
		TaskSuccess:    taskSuccess,
		TotalSteps:     s.TotalSteps,
		Duration:       duration,
		AnalyzedAt:     time.Now(),
	}

	mc.mu.Lock()
	mc.episodes = append(mc.episodes, ep)
	if len(mc.episodes) > 500 {
		mc.episodes = mc.episodes[len(mc.episodes)-500:]
	}
	mc.recomputeStats()
	mc.mu.Unlock()

	result := &MetaCogEpisodeResult{
		TaskID:        taskID,
		CognitiveLoad: mc.cognitiveLoad(ep),
		ReflectROI:    mc.stats.ReflectROI,
		Recommendation: mc.recommend(ep),
	}

	mc.emitMetaCogEvent(ctx, taskID, result)
	return result, nil
}

// MetaCogEpisodeResult is returned after analyzing a single task episode.
type MetaCogEpisodeResult struct {
	TaskID         string  `json:"task_id"`
	CognitiveLoad  float64 `json:"cognitive_load"`  // 0-1
	ReflectROI     float64 `json:"reflect_roi"`
	Recommendation string  `json:"recommendation"`
}

// Stats returns the current cross-task metacognitive statistics.
func (mc *MetaCognition) Stats() MetaCogStats {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	return mc.stats
}

// ShouldReflect uses accumulated statistics to recommend whether
// the agent should perform a reflection at the current step.
// Returns true if reflection is likely to improve outcomes.
func (mc *MetaCognition) ShouldReflect(currentConfidence float64, stepsSinceLastReflect int) bool {
	mc.mu.Lock()
	roi := mc.stats.ReflectROI
	optRate := mc.stats.OptimalReflectRate
	mc.mu.Unlock()

	if roi <= 0 {
		return stepsSinceLastReflect >= 5
	}

	if roi < 1.0 {
		return stepsSinceLastReflect >= 8 && currentConfidence < 0.4
	}

	// ROI > 1: reflection helps. Use optimal rate.
	if optRate > 0 {
		return stepsSinceLastReflect >= int(1.0/optRate)
	}
	return stepsSinceLastReflect >= 3
}

// ShouldUpgradeModel recommends switching to a stronger model based
// on cognitive load indicators.
func (mc *MetaCognition) ShouldUpgradeModel(backtracks, totalSteps int, avgConfidence float64) bool {
	if totalSteps == 0 {
		return false
	}
	backtrackRate := float64(backtracks) / float64(totalSteps)
	return backtrackRate > 0.3 || avgConfidence < 0.4
}

// ShouldDecompose recommends switching from ReAct to Long Horizon DAG
// planning based on task complexity indicators.
func (mc *MetaCognition) ShouldDecompose(totalSteps, backtracks int) bool {
	return totalSteps > 10 && backtracks >= 3
}

// ── internal ──

func (mc *MetaCognition) recomputeStats() {
	stats := MetaCogStats{TotalEpisodes: len(mc.episodes)}

	for _, ep := range mc.episodes {
		hasReflect := ep.ReflectCount > 0
		if ep.TaskSuccess {
			if hasReflect {
				stats.SuccessWithReflect++
			} else {
				stats.SuccessWithoutReflect++
			}
		} else {
			if hasReflect {
				stats.FailWithReflect++
			} else {
				stats.FailWithoutReflect++
			}
		}
	}

	// Reflect ROI = P(success|reflected) / P(success|not_reflected)
	// Requires minimum 10 observations per group for statistical significance.
	totalWithReflect := stats.SuccessWithReflect + stats.FailWithReflect
	totalWithout := stats.SuccessWithoutReflect + stats.FailWithoutReflect

	const minSamples = 10
	if totalWithReflect >= minSamples && totalWithout >= minSamples {
		pSuccessReflect := float64(stats.SuccessWithReflect) / float64(totalWithReflect)
		pSuccessNoReflect := float64(stats.SuccessWithoutReflect) / float64(totalWithout)
		if pSuccessNoReflect > 0 {
			stats.ReflectROI = pSuccessReflect / pSuccessNoReflect
		}
	} else {
		stats.ReflectROI = 1.0 // neutral until enough data
	}

	// Optimal reflect rate: among successful episodes with reflections,
	// what's the median reflect/decision ratio?
	var rates []float64
	for _, ep := range mc.episodes {
		if ep.TaskSuccess && ep.ReflectCount > 0 && ep.DecisionCount > 0 {
			rates = append(rates, float64(ep.ReflectCount)/float64(ep.DecisionCount))
		}
	}
	if len(rates) > 0 {
		stats.OptimalReflectRate = median(rates)
	}

	// Average cognitive load
	var loadSum float64
	for _, ep := range mc.episodes {
		loadSum += mc.cognitiveLoad(ep)
	}
	if len(mc.episodes) > 0 {
		stats.AvgCogLoad = loadSum / float64(len(mc.episodes))
	}

	mc.stats = stats
}

// cognitiveLoad computes a normalized load score for a single episode.
// High backtrack rate + low confidence + many steps = high load.
func (mc *MetaCognition) cognitiveLoad(ep metacogEpisode) float64 {
	if ep.TotalSteps == 0 {
		return 0
	}

	backtrackRate := float64(ep.BacktrackCount) / float64(ep.TotalSteps)
	confDeficit := 1.0 - ep.AvgConfidence
	stepPressure := math.Min(float64(ep.TotalSteps)/20.0, 1.0) // saturate at 20 steps

	load := 0.4*backtrackRate + 0.35*confDeficit + 0.25*stepPressure
	return math.Min(load, 1.0)
}

func (mc *MetaCognition) recommend(ep metacogEpisode) string {
	load := mc.cognitiveLoad(ep)

	switch {
	case load > 0.7:
		return "high_cognitive_load: consider task decomposition or model upgrade"
	case load > 0.4 && ep.ReflectCount == 0:
		return "moderate_load_no_reflect: add reflection checkpoints"
	case !ep.TaskSuccess && ep.BacktrackCount >= 3:
		return "excessive_backtracking: switch to DAG planning"
	case ep.TaskSuccess && load < 0.2:
		return "smooth_execution: current strategy is effective"
	default:
		return "normal: continue current approach"
	}
}

func (mc *MetaCognition) emitMetaCogEvent(ctx context.Context, taskID string, result *MetaCogEpisodeResult) {
	payload, _ := json.Marshal(result)
	mc.events.Append(ctx, &Event{
		TaskID:    taskID,
		Kind:      "metacognition.analysis",
		Actor:     "metacognition",
		Payload:   payload,
		CreatedAt: time.Now(),
	})
}

func median(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	n := len(vals)
	sorted := make([]float64, n)
	copy(sorted, vals)
	for i := 1; i < n; i++ {
		for j := i; j > 0 && sorted[j] < sorted[j-1]; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	if n%2 == 0 {
		return (sorted[n/2-1] + sorted[n/2]) / 2
	}
	return sorted[n/2]
}
