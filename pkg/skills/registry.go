package skills

import (
	"context"
	"sort"
	"strings"
	"sync"
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

// ReadyChecker is an optional interface skills can implement to report
// whether they have the required configuration/dependencies to function.
type ReadyChecker interface {
	Ready() (bool, string) // (is_ready, reason_if_not)
}

// IsReady checks if a skill is ready. Skills that don't implement ReadyChecker
// are assumed to be always ready.
func IsReady(s Skill) (bool, string) {
	if rc, ok := s.(ReadyChecker); ok {
		return rc.Ready()
	}
	return true, ""
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
	// WorkspacePaths are extra host directories the current conversation has
	// opened (Cursor-style "current workspace"). Read-only skills like
	// file_search merge these into their allowed read roots for this call,
	// without mutating the global HOST_READ_PATHS configuration.
	WorkspacePaths []string
}

// SkillCategory groups skills under a named category for hierarchical invocation.
type SkillCategory struct {
	ID          string
	Name        string
	Description string
	SkillNames  []string
}

// Registry holds all registered skills.
//
// All public methods are safe for concurrent use. Read paths acquire an
// RWMutex in read mode; mutations (Register/Remove/Clear/ReplaceAll/
// DefineCategory/AssignCategory) take the write lock. This prevents the
// fatal "concurrent map read/write" panic that would otherwise happen when
// plugin hot-reload (Clear+Register) races with request handlers iterating
// All()/Get()/HierarchicalDefs().
type Registry struct {
	mu         sync.RWMutex
	skills     map[string]Skill
	pinned     map[string]Skill // skills added via Register; survive ReplaceAll (plugin hot-reload)
	categories map[string]*SkillCategory
	skillToCat map[string]string
	version    int // monotonically increasing counter, incremented on any mutation
}

// NewRegistry creates an empty skill registry.
func NewRegistry() *Registry {
	return &Registry{
		skills:     make(map[string]Skill),
		pinned:     make(map[string]Skill),
		categories: make(map[string]*SkillCategory),
		skillToCat: make(map[string]string),
	}
}

// Clear removes all skills and categories from the registry.
func (r *Registry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.skills = make(map[string]Skill)
	r.pinned = make(map[string]Skill)
	r.categories = make(map[string]*SkillCategory)
	r.skillToCat = make(map[string]string)
	r.version++
}

// Register adds a skill to the registry. Skills added this way are "pinned":
// they survive a subsequent ReplaceAll (which swaps the plugin baseline on
// hot-reload), so MCP-as-skill, file/marketplace/dynamic skills, generate_skill,
// and post-init registrations are not silently wiped by a plugin toggle/reload.
func (r *Registry) Register(s Skill) {
	if s == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.skills[s.Name()] = s
	r.pinned[s.Name()] = s
	r.version++
}

// Remove deletes a skill from the registry (and unpins it).
func (r *Registry) Remove(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.skills, name)
	delete(r.pinned, name)
	r.version++
}

// ReplaceAll atomically swaps the plugin BASELINE skill set. It preserves
// existing category definitions so that intent-based filtering keeps working
// across a plugin hot-reload, and — critically — it re-applies skills that were
// added via Register ("pinned": MCP-as-skill, file/marketplace/dynamic skills,
// generate_skill, post-init registrations). Without that overlay a plugin
// toggle/reload or python-engine swap would wipe every non-plugin skill until
// the next restart. Used by gateway rebuild paths to avoid the brief empty
// window between Clear() and a series of Register() calls.
func (r *Registry) ReplaceAll(skills []Skill) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.skills = make(map[string]Skill, len(skills)+len(r.pinned))
	for _, s := range skills {
		if s == nil {
			continue
		}
		r.skills[s.Name()] = s
	}
	// Overlay pinned (Register-added) skills so they survive the baseline swap.
	// Pinned wins on name collision, matching boot order where extras register
	// after the plugin baseline.
	for name, s := range r.pinned {
		if s == nil {
			continue
		}
		r.skills[name] = s
	}
	// Drop stale skill→category mappings that no longer point to a live skill.
	for name := range r.skillToCat {
		if _, ok := r.skills[name]; !ok {
			delete(r.skillToCat, name)
		}
	}
	for _, cat := range r.categories {
		filtered := cat.SkillNames[:0]
		for _, n := range cat.SkillNames {
			if _, ok := r.skills[n]; ok {
				filtered = append(filtered, n)
			}
		}
		cat.SkillNames = filtered
	}
	r.version++
}

// Version returns a monotonically increasing counter that changes whenever skills are added or removed.
func (r *Registry) Version() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.version
}

// Get returns a skill by name.
func (r *Registry) Get(name string) (Skill, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.skills[name]
	return s, ok
}

// All returns all registered skills sorted alphabetically by name.
func (r *Registry) All() []Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Skill, 0, len(r.skills))
	for _, s := range r.skills {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out
}

// CategoryOf returns the category ID for a skill, or empty if uncategorized.
func (r *Registry) CategoryOf(skillName string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.skillToCat[skillName]
}

// Definitions returns tool definitions for the planner prompt.
func (r *Registry) Definitions() []map[string]any {
	r.mu.RLock()
	defer r.mu.RUnlock()
	defs := make([]map[string]any, 0, len(r.skills))
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
	r.mu.Lock()
	defer r.mu.Unlock()
	// Clear stale skill→category mappings from a previous definition of this
	// category. Without this, redefining a category with a smaller/changed
	// SkillNames set leaves orphan mappings, so FilterByIntentScored would keep
	// exposing skills that are no longer in the category.
	for sn, cid := range r.skillToCat {
		if cid == cat.ID {
			delete(r.skillToCat, sn)
		}
	}
	c := cat
	r.categories[c.ID] = &c
	for _, sn := range c.SkillNames {
		r.skillToCat[sn] = c.ID
	}
	r.version++
}

// AssignCategory puts an existing skill into a category.
func (r *Registry) AssignCategory(skillName, catID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
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
	r.version++
}

// Categories returns all defined categories sorted by ID.
func (r *Registry) Categories() []*SkillCategory {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*SkillCategory, 0, len(r.categories))
	for _, c := range r.categories {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// UncategorizedSkills returns skills not assigned to any category.
func (r *Registry) UncategorizedSkills() []Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()
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
	r.mu.RLock()
	defer r.mu.RUnlock()
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
	r.mu.RLock()
	defer r.mu.RUnlock()
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

// UnbackedIntentBuckets returns intent keyword categories that have NO matching
// DefineCategory in this registry. Their keywords are therefore inert in
// ScoreCategories (it skips categories that aren't defined), so intent-based
// narrowing never fires for them. Surfaced at startup to make intent-router
// drift visible instead of silently dead — e.g. "file"/"image"/"research"/
// "workflow" keywords do nothing unless those categories are also defined.
func (r *Registry) UnbackedIntentBuckets() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []string
	for catID := range intentKeywords {
		if _, ok := r.categories[catID]; !ok {
			out = append(out, catID)
		}
	}
	sort.Strings(out)
	return out
}

// FilterByIntentScored returns skills relevant to the given user message using
// multi-signal scoring: keyword matching + success rate + recency. Pass a nil
// scorer for keyword-only matching. Accepts a scorer
// for Ledger-driven success rate and recency data.
func (r *Registry) FilterByIntentScored(message string, scorer *SkillScorer) []Skill {
	// ScoreCategories takes its own RLock; call it before we take ours below to
	// avoid nested lock acquisition patterns.
	scores := r.ScoreCategories(message, scorer)

	matchedCats := make(map[string]bool)
	for catID, score := range scores {
		if score > 0.5 {
			matchedCats[catID] = true
		}
	}

	if len(matchedCats) == 0 {
		return r.All()
	}

	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []Skill
	for name, s := range r.skills {
		catID, inCat := r.skillToCat[name]
		if !inCat || matchedCats[catID] {
			out = append(out, s)
		}
	}
	return out
}
