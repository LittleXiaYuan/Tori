package planner

// SkillTrustGate owns the trust boundary around skill execution.
//
// Planner decides *when* a skill should run, but the gate owns whether the
// current skill is trusted enough to run and how execution outcomes feed back
// into the trust model. Keeping the callbacks behind this small service avoids
// adding more trust-related fields directly to Planner as the policy evolves.
type SkillTrustGate struct {
	check  func(skillName string) error
	record func(skillName string, success bool)
}

func NewSkillTrustGate() *SkillTrustGate { return &SkillTrustGate{} }

func (g *SkillTrustGate) SetCheck(fn func(skillName string) error) {
	if g == nil {
		return
	}
	g.check = fn
}

func (g *SkillTrustGate) SetRecord(fn func(skillName string, success bool)) {
	if g == nil {
		return
	}
	g.record = fn
}

func (g *SkillTrustGate) Check(skillName string) error {
	if g == nil || g.check == nil {
		return nil
	}
	return g.check(skillName)
}

func (g *SkillTrustGate) Record(skillName string, success bool) {
	if g == nil || g.record == nil {
		return
	}
	g.record(skillName, success)
}
