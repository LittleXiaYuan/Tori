package skills

import (
	"context"
	"strings"
)

// Skill is an atomic capability unit that the planner can invoke.
type Skill interface {
	// Name returns the unique skill identifier.
	Name() string
	// Description returns a human-readable description for the planner.
	Description() string
	// Parameters returns JSON schema of expected input.
	Parameters() map[string]any
	// Execute runs the skill with given arguments and context.
	Execute(ctx context.Context, args map[string]any, env *Environment) (string, error)
}

// LLMCallFunc calls the LLM with a system prompt and user prompt, returning the response.
type LLMCallFunc func(ctx context.Context, system, user string) (string, error)

// MemorySearchFunc searches memory for relevant context.
type MemorySearchFunc func(ctx context.Context, tenantID, query string, topK int) (string, error)

// Environment provides shared resources to skills.
type Environment struct {
	ClassID      string
	TeacherID    string
	StudentID    string
	TenantID     string
	LLMCall      LLMCallFunc
	MemorySearch MemorySearchFunc
}

// SkillCategory groups skills under a named category for hierarchical invocation.
type SkillCategory struct {
	ID          string
	Name        string
	Description string
	SkillNames  []string
}

// Registry holds all registered skills.
type Registry struct {
	skills     map[string]Skill
	categories map[string]*SkillCategory
	skillToCat map[string]string
	version    int // monotonically increasing counter, incremented on Register/Clear
}

// NewRegistry creates an empty skill registry.
func NewRegistry() *Registry {
	return &Registry{
		skills:     make(map[string]Skill),
		categories: make(map[string]*SkillCategory),
		skillToCat: make(map[string]string),
	}
}

// Clear removes all skills from the registry.
func (r *Registry) Clear() {
	r.skills = make(map[string]Skill)
	r.categories = make(map[string]*SkillCategory)
	r.skillToCat = make(map[string]string)
	r.version++
}

// Register adds a skill to the registry.
func (r *Registry) Register(s Skill) {
	r.skills[s.Name()] = s
	r.version++
}

// Remove deletes a skill from the registry.
func (r *Registry) Remove(name string) {
	delete(r.skills, name)
	r.version++
}

// Version returns a monotonically increasing counter that changes whenever skills are added or removed.
func (r *Registry) Version() int {
	return r.version
}

// Get returns a skill by name.
func (r *Registry) Get(name string) (Skill, bool) {
	s, ok := r.skills[name]
	return s, ok
}

// All returns all registered skills.
func (r *Registry) All() []Skill {
	out := make([]Skill, 0, len(r.skills))
	for _, s := range r.skills {
		out = append(out, s)
	}
	return out
}

// Definitions returns tool definitions for the planner prompt.
func (r *Registry) Definitions() []map[string]any {
	var defs []map[string]any
	for _, s := range r.skills {
		defs = append(defs, map[string]any{
			"name":        s.Name(),
			"description": s.Description(),
			"parameters":  s.Parameters(),
		})
	}
	return defs
}

// DefineCategory registers a skill category for hierarchical calling.
func (r *Registry) DefineCategory(cat SkillCategory) {
	r.categories[cat.ID] = &cat
	for _, sn := range cat.SkillNames {
		r.skillToCat[sn] = cat.ID
	}
}

// AssignCategory puts an existing skill into a category.
func (r *Registry) AssignCategory(skillName, catID string) {
	cat, ok := r.categories[catID]
	if !ok {
		return
	}
	for _, n := range cat.SkillNames {
		if n == skillName {
			return
		}
	}
	cat.SkillNames = append(cat.SkillNames, skillName)
	r.skillToCat[skillName] = catID
}

// Categories returns all defined categories.
func (r *Registry) Categories() []*SkillCategory {
	out := make([]*SkillCategory, 0, len(r.categories))
	for _, c := range r.categories {
		out = append(out, c)
	}
	return out
}

// CategorySkills returns all skills belonging to a category.
func (r *Registry) CategorySkills(catID string) []Skill {
	cat, ok := r.categories[catID]
	if !ok {
		return nil
	}
	var out []Skill
	for _, name := range cat.SkillNames {
		if s, ok := r.skills[name]; ok {
			out = append(out, s)
		}
	}
	return out
}

// UncategorizedSkills returns skills not assigned to any category.
func (r *Registry) UncategorizedSkills() []Skill {
	var out []Skill
	for name, s := range r.skills {
		if _, ok := r.skillToCat[name]; !ok {
			out = append(out, s)
		}
	}
	return out
}

// HierarchicalDefs returns a reduced set of tool definitions:
// - One "meta tool" per category (use_browser, use_connector, etc.)
// - All uncategorized skills directly
// This reduces the total tools sent to the LLM.
func (r *Registry) HierarchicalDefs() []map[string]any {
	var defs []map[string]any

	for _, cat := range r.categories {
		if len(cat.SkillNames) == 0 {
			continue
		}
		defs = append(defs, map[string]any{
			"name":        "use_" + cat.ID,
			"description": cat.Description,
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"action": map[string]any{
						"type":        "string",
						"description": "The specific action to perform. Available: " + joinNames(cat.SkillNames),
					},
					"args": map[string]any{
						"type":        "object",
						"description": "Arguments for the chosen action",
					},
				},
				"required": []string{"action"},
			},
		})
	}

	for name, s := range r.skills {
		if _, ok := r.skillToCat[name]; !ok {
			defs = append(defs, map[string]any{
				"name":        s.Name(),
				"description": s.Description(),
				"parameters":  s.Parameters(),
			})
		}
	}
	return defs
}

func joinNames(names []string) string {
	out := ""
	for i, n := range names {
		if i > 0 {
			out += ", "
		}
		out += n
	}
	return out
}

// intentKeywords maps intent keywords to category IDs for keyword scoring.
var intentKeywords = map[string][]string{
	"browser": {"浏览", "网页", "网站", "打开", "搜索", "截图", "点击", "输入", "滚动",
		"查看", "访问", "前往", "跳转", "去看", "帮我看", "登录", "注册",
		"youtube", "google", "baidu", "bilibili", "twitter", "facebook", "instagram",
		"browse", "navigate", "web", "click", "screenshot", "tab", "mark", "element",
		"url", "http", "open"},
	"connector": {"github", "仓库", "repo", "issue", "pr", "邮件", "gmail", "email",
		"日历", "calendar", "notion", "slack", "linear", "jira", "outlook"},
	"research": {"研究", "调研", "报告", "research", "report", "分析", "综述"},
	"file": {"文件", "保存", "生成", "写入", "导出", "file", "save", "export", "write",
		"markdown", "html", "csv", "json", "pdf", "docx", "pptx", "xlsx"},
	"image":    {"图片", "图像", "画", "生成图", "image", "picture", "draw", "illustration"},
	"workflow": {"工作流", "workflow", "自动化", "流程", "automation"},
}

// SkillScorer provides scoring-based skill routing. Categories are scored by
// keyword overlap, Ledger success rates, and recency — higher score means more
// likely relevant to the user's intent.
type SkillScorer struct {
	SuccessRates map[string]float64 // skill_name → [0,1] success rate from Ledger
	RecentSkills []string           // last N skill names used (most recent first)
}

// ScoreCategories returns a score for each registered category based on the
// user message. Score = keyword_hits + success_rate_bonus + recency_bonus.
func (r *Registry) ScoreCategories(message string, scorer *SkillScorer) map[string]float64 {
	msg := strings.ToLower(message)
	scores := make(map[string]float64)

	for catID, keywords := range intentKeywords {
		if _, exists := r.categories[catID]; !exists {
			continue
		}
		hits := 0
		for _, kw := range keywords {
			if strings.Contains(msg, kw) {
				hits++
			}
		}
		if hits == 0 {
			continue
		}
		score := float64(hits) * 1.0

		if scorer != nil {
			cat := r.categories[catID]
			var avgSuccess float64
			var successCount int
			for _, sn := range cat.SkillNames {
				if rate, ok := scorer.SuccessRates[sn]; ok {
					avgSuccess += rate
					successCount++
				}
			}
			if successCount > 0 {
				score += (avgSuccess / float64(successCount)) * 2.0
			}

			for i, recent := range scorer.RecentSkills {
				if cat := r.skillToCat[recent]; cat == catID {
					score += 1.0 / float64(i+1)
					break
				}
			}
		}
		scores[catID] = score
	}
	return scores
}

// FilterByIntent returns skills relevant to the given user message using
// multi-signal scoring: keyword matching + success rate + recency.
// Always returns uncategorized skills + skills from matched categories.
func (r *Registry) FilterByIntent(message string) []Skill {
	return r.FilterByIntentScored(message, nil)
}

// FilterByIntentScored is like FilterByIntent but accepts a scorer
// for Ledger-driven success rate and recency data.
func (r *Registry) FilterByIntentScored(message string, scorer *SkillScorer) []Skill {
	scores := r.ScoreCategories(message, scorer)

	matchedCats := make(map[string]bool)
	for catID, score := range scores {
		if score > 0.5 {
			matchedCats[catID] = true
		}
	}

	var out []Skill
	for name, s := range r.skills {
		catID, inCat := r.skillToCat[name]
		if !inCat {
			out = append(out, s)
		} else if matchedCats[catID] {
			out = append(out, s)
		}
	}

	if len(matchedCats) == 0 {
		return r.All()
	}
	return out
}
