package education

import "yunque-agent/pkg/skills"

// EducationPlugin bundles all education-domain skills.
type EducationPlugin struct{}

func New() *EducationPlugin { return &EducationPlugin{} }

func (p *EducationPlugin) Name() string        { return "education" }
func (p *EducationPlugin) Description() string { return "教育领域插件：备课、出题、批改" }

func (p *EducationPlugin) Skills() []skills.Skill {
	return []skills.Skill{
		NewLessonPlanSkill(),
		NewQuizGenSkill(),
		NewGradingSkill(),
	}
}

func (p *EducationPlugin) SystemPrompt() string {
	return `你具备教育领域专业能力：
- 根据课标和教材生成教案
- 自动出题（选择/填空/解答）
- 智能批改作业并给出反馈
- 学情分析和个性化辅导建议`
}
