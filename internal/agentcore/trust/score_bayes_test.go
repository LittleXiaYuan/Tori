package trust

import (
	"math"
	"testing"
)

func TestBayesianScore_ZeroAlphaBetaParamFallsBackToLegacy(t *testing.T) {
	e := Entry{Score: 70, Alpha: 0, BetaParam: 0}
	perm := e.Allowed()
	if perm != PermNetwork {
		t.Errorf("expected PermNetwork for legacy score=70, got %s", perm)
	}
}

func TestBayesianScore_Calculation(t *testing.T) {
	e := Entry{Alpha: 80, BetaParam: 20}
	score := e.BayesianScore()
	if math.Abs(score-80.0) > 0.1 {
		t.Errorf("expected ~80, got %.2f", score)
	}
}

func TestBayesianConfidence_GrowsWithObservations(t *testing.T) {
	few := Entry{Alpha: 3, BetaParam: 1}
	many := Entry{Alpha: 90, BetaParam: 10}

	confFew := few.BayesianConfidence()
	confMany := many.BayesianConfidence()

	if confMany <= confFew {
		t.Errorf("more observations should yield higher confidence: few=%.3f many=%.3f", confFew, confMany)
	}
}

func TestRecordSuccess_UpdatesAlpha(t *testing.T) {
	tracker := NewTracker("")
	tracker.Seed("test-skill", 50)
	before := tracker.Get("test-skill")
	alphaBefore := before.Alpha

	tracker.RecordSuccess("test-skill")
	after := tracker.Get("test-skill")

	if after.Alpha != alphaBefore+1 {
		t.Errorf("expected Alpha to increase by 1: before=%.1f after=%.1f", alphaBefore, after.Alpha)
	}
}

func TestRecordFailure_UpdatesBeta(t *testing.T) {
	tracker := NewTracker("")
	tracker.Seed("test-skill", 50)
	before := tracker.Get("test-skill")
	betaBefore := before.BetaParam

	tracker.RecordFailure("test-skill", 5)
	after := tracker.Get("test-skill")

	if after.BetaParam != betaBefore+5 {
		t.Errorf("expected Beta to increase by 5: before=%.1f after=%.1f", betaBefore, after.BetaParam)
	}
}

func TestAllowed_LowConfidence_DemotesOneLevel(t *testing.T) {
	e := Entry{Alpha: 4, BetaParam: 1, Score: 80}
	if e.BayesianConfidence() >= 0.6 {
		t.Skip("confidence unexpectedly high for few observations")
	}
	perm := e.Allowed()
	if perm >= PermShell {
		t.Errorf("low confidence should demote from Shell to Network, got %s", perm)
	}
}

func TestSeed_InitializesBetaParams(t *testing.T) {
	tracker := NewTracker("")
	tracker.Seed("s1", 80)
	e := tracker.Get("s1")

	if e.Alpha < 1 || e.BetaParam < 1 {
		t.Errorf("Seed should initialize Beta params: α=%.1f β=%.1f", e.Alpha, e.BetaParam)
	}
	score := e.BayesianScore()
	if math.Abs(score-80.0) > 5.0 {
		t.Errorf("Seeded BayesianScore should approximate target 80: got %.1f", score)
	}
}
