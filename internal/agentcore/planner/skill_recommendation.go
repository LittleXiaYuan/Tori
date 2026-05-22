package planner

import "yunque-agent/pkg/skills"

func (p *Planner) rankSkillsByRecommendation(userMessage string, candidates []skills.Skill) []skills.Skill {
	if p == nil || p.skillRuntime == nil {
		return candidates
	}
	return p.skillRuntime.RankByRecommendation(userMessage, candidates)
}

func (p *Planner) recordSkillRecommendationOutcome(skillName string, success bool) {
	if p == nil || p.skillRuntime == nil {
		return
	}
	p.skillRuntime.RecordRecommendationOutcome(skillName, success)
}
