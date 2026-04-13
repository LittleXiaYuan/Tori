package rlsched

import (
	"testing"
)

func TestQLearnerBasicUpdate(t *testing.T) {
	q := NewQLearner(DefaultQLearnerConfig([]string{"fast", "smart", "expert"}))

	q.Update("easy_task", "fast", 1.0, "done")
	q.Update("hard_task", "expert", 0.9, "done")
	q.Update("hard_task", "fast", -0.5, "failed")

	if q.QValue("easy_task", "fast") <= 0 {
		t.Error("fast on easy_task should have positive Q-value")
	}
	if q.QValue("hard_task", "fast") >= 0 {
		t.Error("fast on hard_task should have negative Q-value (penalized)")
	}
	if q.BestAction("hard_task") != "expert" {
		t.Errorf("best action for hard_task should be expert, got %s", q.BestAction("hard_task"))
	}
}

func TestQLearnerConvergence(t *testing.T) {
	actions := []string{"A", "B", "C"}
	q := NewQLearner(QLearnerConfig{
		Alpha:        0.2,
		Gamma:        0.9,
		Epsilon:      0.1,
		EpsilonDecay: 0.99,
		EpsilonMin:   0.01,
		Actions:      actions,
	})

	for i := 0; i < 500; i++ {
		q.Update("state1", "A", 1.0, "state2")
		q.Update("state1", "B", 0.2, "state2")
		q.Update("state1", "C", -0.5, "state2")
	}

	best := q.BestAction("state1")
	if best != "A" {
		t.Errorf("after 500 episodes, best action for state1 should be A (reward=1.0), got %s", best)
	}

	qA := q.QValue("state1", "A")
	qC := q.QValue("state1", "C")
	if qA <= qC {
		t.Errorf("Q(state1, A) should be > Q(state1, C), got A=%.3f C=%.3f", qA, qC)
	}
}

func TestQLearnerExploration(t *testing.T) {
	q := NewQLearner(QLearnerConfig{
		Alpha:   0.1,
		Gamma:   0.9,
		Epsilon: 1.0, // always explore
		Actions: []string{"x", "y"},
	})

	seen := map[string]int{}
	for i := 0; i < 100; i++ {
		action, isExplore := q.SelectAction("s")
		if !isExplore {
			t.Error("with epsilon=1.0, all actions should be exploratory")
		}
		seen[action]++
	}
	if len(seen) < 2 {
		t.Error("with full exploration, should see both actions")
	}
}

func TestQLearnerEpsilonDecay(t *testing.T) {
	q := NewQLearner(QLearnerConfig{
		Alpha:        0.1,
		Gamma:        0.9,
		Epsilon:      0.5,
		EpsilonDecay: 0.9,
		EpsilonMin:   0.01,
		Actions:      []string{"a"},
	})

	initialEps := q.Epsilon()
	for i := 0; i < 50; i++ {
		q.Update("s", "a", 1.0, "s2")
	}
	finalEps := q.Epsilon()

	if finalEps >= initialEps {
		t.Errorf("epsilon should decay: initial=%.3f final=%.3f", initialEps, finalEps)
	}
	if finalEps < 0.01 {
		t.Errorf("epsilon should not go below min: %.3f", finalEps)
	}
}

func TestQLearnerPolicy(t *testing.T) {
	q := NewQLearner(DefaultQLearnerConfig([]string{"run", "skip"}))

	q.Update("idle", "run", 0.8, "busy")
	q.Update("idle", "skip", 0.1, "idle")
	q.Update("busy", "skip", 0.5, "idle")

	policy := q.Policy()
	if len(policy) == 0 {
		t.Fatal("expected non-empty policy")
	}
	for _, p := range policy {
		t.Logf("state=%s action=%s q=%.3f", p.State, p.Action, p.QValue)
	}
}

func TestQLearnerEpisodeCount(t *testing.T) {
	q := NewQLearner(DefaultQLearnerConfig([]string{"a"}))
	if q.Episodes() != 0 {
		t.Error("new learner should have 0 episodes")
	}
	q.Update("s", "a", 1, "s2")
	q.Update("s", "a", 1, "s2")
	if q.Episodes() != 2 {
		t.Errorf("expected 2 episodes, got %d", q.Episodes())
	}
}

func TestStateEncoder(t *testing.T) {
	enc := NewStateEncoder()
	enc.AddFeature("queue", []float64{3, 10, 50})
	enc.AddFeature("hour", []float64{6, 12, 18})

	state := enc.Encode(map[string]float64{"queue": 1, "hour": 14})
	if state == "" {
		t.Fatal("expected non-empty state string")
	}
	t.Logf("encoded state: %s", state)

	state2 := enc.Encode(map[string]float64{"queue": 100, "hour": 2})
	if state2 == state {
		t.Error("different inputs should produce different states")
	}
	t.Logf("encoded state2: %s", state2)
}

func TestQLearnerNoActions(t *testing.T) {
	q := NewQLearner(DefaultQLearnerConfig(nil))
	action, _ := q.SelectAction("any")
	if action != "" {
		t.Errorf("no actions should return empty string, got %s", action)
	}
}
