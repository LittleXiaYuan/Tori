package cogni

import "testing"

func act(id string, score float64, prio int) Activation {
	return Activation{
		Declaration: &Declaration{ID: id, Priority: prio},
		Activated:   true,
		Score:       score,
	}
}

func arbIDs(acts []Activation) []string {
	out := make([]string, len(acts))
	for i, a := range acts {
		out[i] = a.Declaration.ID
	}
	return out
}

func TestArbitrate_ZeroConfigIsIdentity(t *testing.T) {
	in := []Activation{act("a", 0.6, 100), act("b", 0.9, 100)}
	out := Arbitrate(in, ArbitrationConfig{})
	if !equal(arbIDs(out), []string{"a", "b"}) {
		t.Fatalf("zero config must preserve input order, got %v", arbIDs(out))
	}
}

func TestArbitrate_TopKByScore(t *testing.T) {
	in := []Activation{act("low", 0.6, 100), act("high", 0.9, 100), act("mid", 0.7, 100)}
	out := Arbitrate(in, ArbitrationConfig{MaxActive: 2})
	if !equal(arbIDs(out), []string{"high", "mid"}) {
		t.Fatalf("top-2 by score wrong, got %v", arbIDs(out))
	}
}

func TestArbitrate_MinConfidenceFloor(t *testing.T) {
	in := []Activation{act("weak", 0.4, 100), act("strong", 0.9, 100)}
	out := Arbitrate(in, ArbitrationConfig{MinConfidence: 0.5})
	if !equal(arbIDs(out), []string{"strong"}) {
		t.Fatalf("floor must drop weak bids, got %v", arbIDs(out))
	}
}

func TestArbitrate_DeterministicTieBreak(t *testing.T) {
	// Equal scores → priority asc (lower wins) → ID asc.
	in := []Activation{
		act("z", 0.8, 10),
		act("a", 0.8, 10),
		act("p", 0.8, 5),
	}
	out := Arbitrate(in, ArbitrationConfig{MaxActive: 2})
	// p has best (lowest) priority; then a before z by ID.
	if !equal(arbIDs(out), []string{"p", "a"}) {
		t.Fatalf("deterministic tiebreak wrong, got %v", arbIDs(out))
	}
}
