// Package rlsched implements reinforcement learning for task scheduling optimization.
//
// Uses tabular Q-Learning (Watkins, 1989) to learn optimal actions (model selection,
// priority ordering, resource allocation) given observed states (queue length,
// task type, time of day, recent performance).
//
// References:
//   - Watkins, Dayan, "Q-Learning", Machine Learning 8(3-4), 1992
//   - Sutton, Barto, "Reinforcement Learning: An Introduction", 2nd ed., 2018
package rlsched

import (
	"math"
	"math/rand"
	"sort"
	"sync"
	"time"
)

// QLearner implements tabular Q-Learning with epsilon-greedy exploration.
type QLearner struct {
	mu           sync.RWMutex
	qtable       map[stateAction]float64 // (state, action) → Q-value
	alpha        float64                 // learning rate (default: 0.1)
	gamma        float64                 // discount factor (default: 0.95)
	epsilon      float64                 // exploration rate (default: 0.15)
	epsilonDecay float64                 // per-episode decay (default: 0.999)
	epsilonMin   float64                 // minimum exploration (default: 0.01)
	actions      []string                // available action space
	episodes     int64                   // total learning episodes
	rng          *rand.Rand
}

type stateAction struct {
	state  string
	action string
}

// QLearnerConfig configures the Q-Learning agent.
type QLearnerConfig struct {
	Alpha        float64  // learning rate [0, 1]
	Gamma        float64  // discount factor [0, 1]
	Epsilon      float64  // initial exploration rate [0, 1]
	EpsilonDecay float64  // decay per episode (0.999 = slow decay)
	EpsilonMin   float64  // minimum exploration floor
	Actions      []string // available actions
}

// DefaultQLearnerConfig returns standard RL parameters.
func DefaultQLearnerConfig(actions []string) QLearnerConfig {
	return QLearnerConfig{
		Alpha:        0.1,
		Gamma:        0.95,
		Epsilon:      0.15,
		EpsilonDecay: 0.999,
		EpsilonMin:   0.01,
		Actions:      actions,
	}
}

// NewQLearner creates a Q-Learning agent.
func NewQLearner(cfg QLearnerConfig) *QLearner {
	if cfg.Alpha <= 0 {
		cfg.Alpha = 0.1
	}
	if cfg.Gamma <= 0 {
		cfg.Gamma = 0.95
	}
	if cfg.Epsilon <= 0 {
		cfg.Epsilon = 0.15
	}
	if cfg.EpsilonDecay <= 0 {
		cfg.EpsilonDecay = 0.999
	}
	if cfg.EpsilonMin < 0 {
		cfg.EpsilonMin = 0.01
	}
	return &QLearner{
		qtable:       make(map[stateAction]float64),
		alpha:        cfg.Alpha,
		gamma:        cfg.Gamma,
		epsilon:      cfg.Epsilon,
		epsilonDecay: cfg.EpsilonDecay,
		epsilonMin:   cfg.EpsilonMin,
		actions:      cfg.Actions,
		rng:          rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// SelectAction chooses an action for the given state using epsilon-greedy policy.
// Returns the selected action and whether it was exploratory.
func (q *QLearner) SelectAction(state string) (string, bool) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	if len(q.actions) == 0 {
		return "", false
	}

	// Epsilon-greedy: explore with probability epsilon
	if q.rng.Float64() < q.epsilon {
		return q.actions[q.rng.Intn(len(q.actions))], true
	}

	// Exploit: pick action with highest Q-value
	return q.bestAction(state), false
}

// Update performs the Q-Learning update after observing a transition.
//
//	Q(s, a) ← Q(s, a) + α × [reward + γ × max_a' Q(s', a') - Q(s, a)]
//
// reward should reflect task outcome quality (0-1 typical, can be negative for penalties).
func (q *QLearner) Update(state, action string, reward float64, nextState string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	sa := stateAction{state, action}
	currentQ := q.qtable[sa]

	// Find max Q-value for next state
	maxNextQ := math.Inf(-1)
	for _, a := range q.actions {
		nsa := stateAction{nextState, a}
		if qv, ok := q.qtable[nsa]; ok && qv > maxNextQ {
			maxNextQ = qv
		}
	}
	if math.IsInf(maxNextQ, -1) {
		maxNextQ = 0
	}

	// Q-Learning update
	tdTarget := reward + q.gamma*maxNextQ
	q.qtable[sa] = currentQ + q.alpha*(tdTarget-currentQ)

	q.episodes++
	// Decay exploration
	q.epsilon = math.Max(q.epsilonMin, q.epsilon*q.epsilonDecay)
}

// QValue returns the current Q-value for a state-action pair.
func (q *QLearner) QValue(state, action string) float64 {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.qtable[stateAction{state, action}]
}

// BestAction returns the greedy (exploitation) action for a state.
func (q *QLearner) BestAction(state string) string {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.bestAction(state)
}

func (q *QLearner) bestAction(state string) string {
	best := q.actions[0]
	bestQ := math.Inf(-1)
	for _, a := range q.actions {
		qv := q.qtable[stateAction{state, a}]
		if qv > bestQ {
			bestQ = qv
			best = a
		}
	}
	return best
}

// Episodes returns the total number of learning updates.
func (q *QLearner) Episodes() int64 {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.episodes
}

// Epsilon returns the current exploration rate.
func (q *QLearner) Epsilon() float64 {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.epsilon
}

// PolicySnapshot returns the learned policy: for each known state, the best action.
type PolicyEntry struct {
	State  string  `json:"state"`
	Action string  `json:"action"`
	QValue float64 `json:"q_value"`
}

// Policy returns the current learned policy.
func (q *QLearner) Policy() []PolicyEntry {
	q.mu.RLock()
	defer q.mu.RUnlock()

	stateSet := make(map[string]bool)
	for sa := range q.qtable {
		stateSet[sa.state] = true
	}

	var entries []PolicyEntry
	for state := range stateSet {
		best := q.bestAction(state)
		entries = append(entries, PolicyEntry{
			State:  state,
			Action: best,
			QValue: math.Round(q.qtable[stateAction{state, best}]*1000) / 1000,
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].QValue > entries[j].QValue })
	return entries
}

// StateEncoder discretizes continuous features into a state string.
// Useful for converting task/system metrics into Q-Learning states.
type StateEncoder struct {
	bins map[string][]float64 // feature name → bin boundaries
}

// NewStateEncoder creates an encoder with configurable bin boundaries.
func NewStateEncoder() *StateEncoder {
	return &StateEncoder{bins: make(map[string][]float64)}
}

// AddFeature defines bins for a feature dimension.
// Example: AddFeature("queue_len", []float64{0, 3, 10, 50}) creates bins:
//
//	"queue_len=0", "queue_len=1-3", "queue_len=4-10", "queue_len=11-50", "queue_len=50+"
func (se *StateEncoder) AddFeature(name string, boundaries []float64) {
	sort.Float64s(boundaries)
	se.bins[name] = boundaries
}

// Encode converts feature values to a discrete state string.
func (se *StateEncoder) Encode(features map[string]float64) string {
	parts := make([]string, 0, len(features))
	for name, val := range features {
		boundaries, ok := se.bins[name]
		if !ok {
			continue
		}
		bin := len(boundaries)
		for i, b := range boundaries {
			if val <= b {
				bin = i
				break
			}
		}
		parts = append(parts, name+"="+bucketLabel(bin, boundaries))
	}
	sort.Strings(parts)
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += "|"
		}
		result += p
	}
	return result
}

func bucketLabel(bin int, boundaries []float64) string {
	if bin >= len(boundaries) {
		return "high"
	}
	if bin == 0 {
		return "low"
	}
	return "mid"
}
