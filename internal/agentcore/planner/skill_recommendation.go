package planner

import "yunque-agent/pkg/skills"

func (p *Planner) rankSkillsByRecommendation(userMessage string, candidates []skills.Skill) []skills.Skill {
	if p == nil {
		return candidates
	}
	skillRuntime := p.ensureSkillRuntime()
	return skillRuntime.RankByRecommendation(userMessage, candidates)
}

func (p *Planner) recordSkillRecommendationOutcome(skillName string, success bool) {
	if p == nil {
		return
	}
	skillRuntime := p.ensureSkillRuntime()
	skillRuntime.RecordRecommendationOutcome(skillName, success)
}
