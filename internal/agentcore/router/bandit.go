package router

import (
	"math"
	"math/rand"
	"sort"
	"sync"
	"time"
)

// ModelBandit uses multi-armed bandit algorithms to learn the optimal
// model for each complexity tier based on observed outcomes.
// Each (tier, model) pair is an "arm" that accumulates reward statistics.
//
// Supports two policies:
//   - UCB1: deterministic, good convergence guarantees
//   - Thompson Sampling: stochastic, better in practice with few samples
type ModelBandit struct {
	mu     sync.RWMutex
	arms   map[armKey]*armStats
	policy BanditPolicy
	rng    *rand.Rand
}

// BanditPolicy selects how the bandit picks arms.
type BanditPolicy int

const (
	PolicyUCB1     BanditPolicy = iota // Upper Confidence Bound
	PolicyThompson                     // Thompson Sampling (Beta distribution)
)

type armKey struct {
	tier    Tier
	modelID string
}

type armStats struct {
	Pulls    int64   // total times this arm was pulled
	Rewards  float64 // cumulative reward (0-1 per pull)
	Successes int64  // for Thompson: Beta(alpha, beta) where alpha=successes+1
	Failures  int64  // beta = failures + 1
	AvgLatencyMs float64
	LastPull time.Time
}

// NewModelBandit creates a bandit with the specified policy.
func NewModelBandit(policy BanditPolicy) *ModelBandit {
	return &ModelBandit{
		arms:   make(map[armKey]*armStats),
		policy: policy,
		rng:    rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// RegisterArm declares a (tier, model) pair as available for selection.
func (b *ModelBandit) RegisterArm(tier Tier, modelID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	key := armKey{tier, modelID}
	if _, exists := b.arms[key]; !exists {
		b.arms[key] = &armStats{}
	}
}

// Select picks the best model for a given tier using the bandit policy.
// Returns the model ID and whether a recommendation was made.
// If no arms are registered for this tier, returns ("", false).
func (b *ModelBandit) Select(tier Tier) (string, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	candidates := b.armsForTier(tier)
	if len(candidates) == 0 {
		return "", false
	}

	if len(candidates) == 1 {
		return candidates[0].modelID, true
	}

	switch b.policy {
	case PolicyThompson:
		return b.selectThompson(candidates), true
	default:
		return b.selectUCB1(candidates), true
	}
}

func (b *ModelBandit) armsForTier(tier Tier) []struct {
	modelID string
	stats   *armStats
} {
	var result []struct {
		modelID string
		stats   *armStats
	}
	for key, stats := range b.arms {
		if key.tier == tier {
			result = append(result, struct {
				modelID string
				stats   *armStats
			}{key.modelID, stats})
		}
	}
	return result
}

func (b *ModelBandit) selectUCB1(candidates []struct {
	modelID string
	stats   *armStats
}) string {
	var totalPulls int64
	for _, c := range candidates {
		totalPulls += c.stats.Pulls
	}
	if totalPulls == 0 {
		totalPulls = 1
	}

	type scored struct {
		modelID string
		ucb     float64
	}
	var results []scored

	for _, c := range candidates {
		if c.stats.Pulls == 0 {
			return c.modelID // explore unpulled arms first
		}

		avgReward := c.stats.Rewards / float64(c.stats.Pulls)
		exploration := math.Sqrt(2 * math.Log(float64(totalPulls)) / float64(c.stats.Pulls))
		ucb := avgReward + exploration

		results = append(results, scored{c.modelID, ucb})
	}

	sort.Slice(results, func(i, j int) bool { return results[i].ucb > results[j].ucb })
	return results[0].modelID
}

func (b *ModelBandit) selectThompson(candidates []struct {
	modelID string
	stats   *armStats
}) string {
	type scored struct {
		modelID string
		sample  float64
	}
	var results []scored

	for _, c := range candidates {
		alpha := float64(c.stats.Successes + 1)
		beta := float64(c.stats.Failures + 1)
		sample := b.betaSample(alpha, beta)
		results = append(results, scored{c.modelID, sample})
	}

	sort.Slice(results, func(i, j int) bool { return results[i].sample > results[j].sample })
	return results[0].modelID
}

// betaSample draws from Beta(alpha, beta) using the Joehnk method.
func (b *ModelBandit) betaSample(alpha, beta float64) float64 {
	if alpha <= 0 {
		alpha = 1
	}
	if beta <= 0 {
		beta = 1
	}
	x := b.gammaSample(alpha)
	y := b.gammaSample(beta)
	if x+y == 0 {
		return 0.5
	}
	return x / (x + y)
}

func (b *ModelBandit) gammaSample(shape float64) float64 {
	if shape < 1 {
		u := b.rng.Float64()
		return b.gammaSample(shape+1) * math.Pow(u, 1.0/shape)
	}
	d := shape - 1.0/3.0
	c := 1.0 / math.Sqrt(9.0*d)
	for {
		var x, v float64
		for {
			x = b.rng.NormFloat64()
			v = 1.0 + c*x
			if v > 0 {
				break
			}
		}
		v = v * v * v
		u := b.rng.Float64()
		if u < 1.0-0.0331*(x*x)*(x*x) {
			return d * v
		}
		if math.Log(u) < 0.5*x*x+d*(1.0-v+math.Log(v)) {
			return d * v
		}
	}
}

// RecordOutcome updates the bandit with the result of using a model.
// reward should be in [0, 1] where 1 = perfect outcome.
// latencyMs is the response time for cost-aware optimization.
func (b *ModelBandit) RecordOutcome(tier Tier, modelID string, reward float64, latencyMs float64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	key := armKey{tier, modelID}
	stats, exists := b.arms[key]
	if !exists {
		stats = &armStats{}
		b.arms[key] = stats
	}

	stats.Pulls++
	stats.Rewards += reward
	stats.LastPull = time.Now()

	if reward >= 0.5 {
		stats.Successes++
	} else {
		stats.Failures++
	}

	// Exponential moving average for latency
	if stats.AvgLatencyMs == 0 {
		stats.AvgLatencyMs = latencyMs
	} else {
		stats.AvgLatencyMs = stats.AvgLatencyMs*0.8 + latencyMs*0.2
	}
}

// BanditSnapshot is a serializable view of bandit state.
type BanditSnapshot struct {
	Arms []ArmSnapshot `json:"arms"`
}

// ArmSnapshot describes one (tier, model) arm.
type ArmSnapshot struct {
	Tier         string  `json:"tier"`
	ModelID      string  `json:"model_id"`
	Pulls        int64   `json:"pulls"`
	AvgReward    float64 `json:"avg_reward"`
	Successes    int64   `json:"successes"`
	Failures     int64   `json:"failures"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
	UCBScore     float64 `json:"ucb_score,omitempty"`
}

// Snapshot returns current bandit statistics.
func (b *ModelBandit) Snapshot() BanditSnapshot {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var totalPulls int64
	for _, s := range b.arms {
		totalPulls += s.Pulls
	}
	if totalPulls == 0 {
		totalPulls = 1
	}

	arms := make([]ArmSnapshot, 0, len(b.arms))
	for key, stats := range b.arms {
		avgReward := 0.0
		ucb := 0.0
		if stats.Pulls > 0 {
			avgReward = stats.Rewards / float64(stats.Pulls)
			ucb = avgReward + math.Sqrt(2*math.Log(float64(totalPulls))/float64(stats.Pulls))
		}
		arms = append(arms, ArmSnapshot{
			Tier:         key.tier.String(),
			ModelID:      key.modelID,
			Pulls:        stats.Pulls,
			AvgReward:    math.Round(avgReward*1000) / 1000,
			Successes:    stats.Successes,
			Failures:     stats.Failures,
			AvgLatencyMs: math.Round(stats.AvgLatencyMs*10) / 10,
			UCBScore:     math.Round(ucb*1000) / 1000,
		})
	}
	sort.Slice(arms, func(i, j int) bool {
		if arms[i].Tier != arms[j].Tier {
			return arms[i].Tier < arms[j].Tier
		}
		return arms[i].UCBScore > arms[j].UCBScore
	})

	return BanditSnapshot{Arms: arms}
}
