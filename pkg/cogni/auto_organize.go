package cogni

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
)

var jsonUnmarshal = json.Unmarshal

// SkillInfo is a lightweight representation of a registered skill.
type SkillInfo struct {
	Name        string
	Description string
	Category    string
}

// SkillCatalogFunc returns all currently registered skills with their metadata.
type SkillCatalogFunc func() []SkillInfo

// AutoOrganizer watches the skill registry and automatically creates/updates
// Cogni declarations that incorporate discovered skills. It bridges the gap
// between "installed skills" and "intelligent cogni agents" by auto-generating
// declarations with activation rules, tool surfaces, and workflows.
type AutoOrganizer struct {
	mu       sync.Mutex
	registry *Registry
	catalog  SkillCatalogFunc
	llm      LLMFunc
	managed  map[string]bool // IDs of auto-generated cognis
}

func NewAutoOrganizer(registry *Registry, catalog SkillCatalogFunc) *AutoOrganizer {
	return &AutoOrganizer{
		registry: registry,
		catalog:  catalog,
		managed:  make(map[string]bool),
	}
}

// SetLLM attaches the LLM for intelligent grouping and rule generation.
// Without LLM, falls back to simple category-based grouping.
func (ao *AutoOrganizer) SetLLM(fn LLMFunc) {
	ao.mu.Lock()
	ao.llm = fn
	ao.mu.Unlock()
}

// Sync scans the skill catalog and ensures every skill category has a
// corresponding Cogni declaration. New declarations are created for
// categories that don't have one; existing declarations are updated
// when skills are added/removed.
func (ao *AutoOrganizer) Sync(ctx context.Context) SyncResult {
	if ao.catalog == nil {
		return SyncResult{}
	}

	ao.mu.Lock()
	defer ao.mu.Unlock()

	skills := ao.catalog()
	if len(skills) == 0 {
		return SyncResult{}
	}

	groups := groupByCategory(skills)
	result := SyncResult{}

	for category, skillGroup := range groups {
		if category == "" {
			category = "general"
		}
		cogniID := "auto:" + category

		existing, exists := ao.registry.Get(cogniID)
		if exists {
			updated := ao.updateDeclaration(existing, skillGroup)
			if updated {
				result.Updated++
			}
			continue
		}

		decl := ao.buildDeclaration(ctx, cogniID, category, skillGroup)
		if err := ao.registry.Add(decl, "auto-organizer"); err != nil {
			slog.Warn("auto-organizer: failed to register", "id", cogniID, "err", err)
			result.Errors++
			continue
		}
		ao.managed[cogniID] = true
		result.Created++
		slog.Info("auto-organizer: created cogni for skill category",
			"id", cogniID, "skills", len(skillGroup))
	}

	// Remove auto-cognis for categories that no longer exist
	for id := range ao.managed {
		cat := strings.TrimPrefix(id, "auto:")
		if _, ok := groups[cat]; !ok {
			ao.registry.Remove(id)
			delete(ao.managed, id)
			result.Removed++
		}
	}

	return result
}

// EnhanceWithLLM uses the LLM to generate smarter activation rules and
// context prompts for auto-organized cognis.
func (ao *AutoOrganizer) EnhanceWithLLM(ctx context.Context) int {
	ao.mu.Lock()
	llm := ao.llm
	ids := make([]string, 0, len(ao.managed))
	for id := range ao.managed {
		ids = append(ids, id)
	}
	ao.mu.Unlock()

	if llm == nil {
		return 0
	}

	enhanced := 0
	for _, id := range ids {
		decl, ok := ao.registry.Get(id)
		if !ok {
			continue
		}

		skillNames := decl.Surface.Include
		if len(skillNames) == 0 {
			continue
		}

		prompt := fmt.Sprintf(
			"分析以下 Skill 列表，为其生成更好的激活关键词和角色描述。\n\n"+
				"Skill 列表:\n%s\n\n"+
				"当前角色描述: %s\n\n"+
				"请输出 JSON: {\"keywords\": [...], \"context\": \"新的角色描述\"}",
			strings.Join(skillNames, "\n"),
			decl.Context.Static,
		)

		raw, err := llm(ctx, "你是一个 AI 系统配置优化器。只输出 JSON。", prompt)
		if err != nil {
			continue
		}

		jsonStr := extractJSON(raw)
		if jsonStr == "" {
			continue
		}

		// Parse keywords and context from LLM response
		type enhancement struct {
			Keywords []string `json:"keywords"`
			Context  string   `json:"context"`
		}

		var enh enhancement
		if err := parseJSON(jsonStr, &enh); err != nil {
			continue
		}

		if len(enh.Keywords) > 0 {
			decl.Activation.Keywords = enh.Keywords
		}
		if enh.Context != "" {
			decl.Context.Static = enh.Context
		}
		ao.registry.Add(decl, "auto-organizer")
		enhanced++
	}
	return enhanced
}

func (ao *AutoOrganizer) buildDeclaration(ctx context.Context, id, category string, skills []SkillInfo) *Declaration {
	var skillNames []string
	var descParts []string
	for _, s := range skills {
		skillNames = append(skillNames, s.Name)
		if s.Description != "" {
			descParts = append(descParts, s.Name+": "+s.Description)
		}
	}

	keywords := generateKeywords(category, skills)

	d := &Declaration{
		ID:          id,
		DisplayName: category + " 智体",
		Description: fmt.Sprintf("自动组织的 %s 类技能（%d 个）", category, len(skills)),
		Priority:    150,
		Activation: ActivationRules{
			Keywords:      keywords,
			KeywordWeight: 0.3,
			MinScore:      0.3,
		},
		Context: ContextInjection{
			Static: fmt.Sprintf("你擅长使用以下工具：\n%s", strings.Join(descParts, "\n")),
		},
		Surface: ToolSurface{
			Include: skillNames,
		},
		Memory: MemoryPolicy{
			Namespace: id,
		},
		Experience: ExperienceConfig{
			Enabled:       true,
			AutoRecord:    true,
			RequireReview: false,
			HalfLifeDays:  90,
		},
	}

	llm := ao.llm
	if llm != nil {
		enhanced := ao.tryLLMEnhance(ctx, llm, d, skills)
		if enhanced != nil {
			return enhanced
		}
	}

	return d
}

func (ao *AutoOrganizer) tryLLMEnhance(ctx context.Context, llm LLMFunc, base *Declaration, skills []SkillInfo) *Declaration {
	var descs []string
	for _, s := range skills {
		descs = append(descs, fmt.Sprintf("- %s: %s", s.Name, s.Description))
	}

	prompt := fmt.Sprintf(
		"为以下工具集生成一个智能 Agent 配置。\n\n工具:\n%s\n\n"+
			"输出 JSON: {\"display_name\": \"中文名\", \"keywords\": [\"关键词\"], \"context\": \"角色描述\"}",
		strings.Join(descs, "\n"),
	)

	raw, err := llm(ctx, "你是一个 AI 配置生成器。只输出 JSON。", prompt)
	if err != nil {
		return nil
	}

	jsonStr := extractJSON(raw)
	if jsonStr == "" {
		return nil
	}

	type result struct {
		DisplayName string   `json:"display_name"`
		Keywords    []string `json:"keywords"`
		Context     string   `json:"context"`
	}
	var r result
	if err := parseJSON(jsonStr, &r); err != nil {
		return nil
	}

	if r.DisplayName != "" {
		base.DisplayName = r.DisplayName
	}
	if len(r.Keywords) > 0 {
		base.Activation.Keywords = r.Keywords
	}
	if r.Context != "" {
		base.Context.Static = r.Context
	}
	return base
}

func (ao *AutoOrganizer) updateDeclaration(d *Declaration, skills []SkillInfo) bool {
	var newNames []string
	for _, s := range skills {
		newNames = append(newNames, s.Name)
	}

	current := d.Surface.Include
	if sameStringSet(current, newNames) {
		return false
	}

	d.Surface.Include = newNames
	d.Description = fmt.Sprintf("自动组织的技能（%d 个）", len(skills))
	ao.registry.Add(d, "auto-organizer")
	return true
}

// SyncResult describes what changed during a Sync call.
type SyncResult struct {
	Created int `json:"created"`
	Updated int `json:"updated"`
	Removed int `json:"removed"`
	Errors  int `json:"errors"`
}

func groupByCategory(skills []SkillInfo) map[string][]SkillInfo {
	groups := make(map[string][]SkillInfo)
	for _, s := range skills {
		cat := s.Category
		if cat == "" {
			cat = "general"
		}
		groups[cat] = append(groups[cat], s)
	}
	return groups
}

func generateKeywords(category string, skills []SkillInfo) []string {
	seen := make(map[string]bool)
	var kw []string

	add := func(word string) {
		w := strings.ToLower(strings.TrimSpace(word))
		if w != "" && !seen[w] && len(w) > 1 {
			seen[w] = true
			kw = append(kw, w)
		}
	}

	add(category)
	for _, s := range skills {
		// Split the skill name into meaningful tokens (docx_create → docx,
		// create) so the generated keywords actually match natural language.
		// Previously underscores were stripped into one mashed token
		// ("docxcreate"), which never matched user text, leaving auto-organized
		// cognis effectively un-activatable at runtime.
		for _, part := range strings.FieldsFunc(s.Name, func(r rune) bool {
			return r == '_' || r == '-' || r == ' '
		}) {
			add(part)
		}
	}

	if len(kw) > 10 {
		kw = kw[:10]
	}
	return kw
}

func sameStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	set := make(map[string]bool, len(a))
	for _, s := range a {
		set[s] = true
	}
	for _, s := range b {
		if !set[s] {
			return false
		}
	}
	return true
}

func parseJSON(s string, v any) error {
	cleaned := extractJSON(s)
	if cleaned == "" {
		cleaned = s
	}
	return jsonUnmarshal([]byte(cleaned), v)
}
