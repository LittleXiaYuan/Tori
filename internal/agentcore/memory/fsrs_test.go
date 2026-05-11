package memory

import (
	"math"
	"testing"
	"time"
)

func TestFSRS_ZeroAccess_PureDecay(t *testing.T) {
	o := &Orchestrator{config: DefaultOrchestratorConfig()}

	oneWeek := 7 * 24 * time.Hour
	r := o.decayFactorWithAccess(oneWeek, 0)

	if r >= 1.0 || r <= 0.0 {
		t.Errorf("retention should be in (0, 1): got %.4f", r)
	}
}

func TestFSRS_MoreAccess_HigherRetention(t *testing.T) {
	o := &Orchestrator{config: DefaultOrchestratorConfig()}
	age := 14 * 24 * time.Hour

	r0 := o.decayFactorWithAccess(age, 0)
	r3 := o.decayFactorWithAccess(age, 3)
	r10 := o.decayFactorWithAccess(age, 10)

	if r3 <= r0 {
		t.Errorf("3 accesses should yield higher retention than 0: r0=%.4f r3=%.4f", r0, r3)
	}
	if r10 <= r3 {
		t.Errorf("10 accesses should yield higher retention than 3: r3=%.4f r10=%.4f", r3, r10)
	}
}

func TestFSRS_ZeroHalfLife_ReturnsOne(t *testing.T) {
	o := &Orchestrator{config: OrchestratorConfig{DecayHalfLife: 0}}
	r := o.decayFactorWithAccess(time.Hour, 5)
	if r != 1.0 {
		t.Errorf("zero half-life should return 1.0, got %.4f", r)
	}
}

func TestFSRS_StabilityGrowsCapped(t *testing.T) {
	o := &Orchestrator{config: DefaultOrchestratorConfig()}
	age := 30 * 24 * time.Hour

	r100 := o.decayFactorWithAccess(age, 100)
	if math.IsInf(r100, 0) || math.IsNaN(r100) {
		t.Errorf("extreme access count should not produce Inf/NaN: got %v", r100)
	}
	if r100 < 0 || r100 > 1.0 {
		t.Errorf("retention should be in [0, 1]: got %.4f", r100)
	}
}

func TestFSRS_DecayFactorBackwardCompat(t *testing.T) {
	o := &Orchestrator{config: DefaultOrchestratorConfig()}
	age := 7 * 24 * time.Hour

	rCompat := o.decayFactor(age)
	rExplicit := o.decayFactorWithAccess(age, 0)

	if math.Abs(rCompat-rExplicit) > 1e-10 {
		t.Errorf("decayFactor and decayFactorWithAccess(0) should be equal: %.6f vs %.6f", rCompat, rExplicit)
	}
}
