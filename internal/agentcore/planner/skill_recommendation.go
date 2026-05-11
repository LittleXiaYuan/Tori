package planner

import (
	"sort"
	"strings"

	"yunque-agent/internal/experimental/recommend"
	"yunque-agent/pkg/skills"
)

func (p *Planner) syncSkillRecommendationItemsLocked() {
	if p.skillRecommender == nil || p.registry == nil {
		return
	}
	version := p.registry.Version()
	if p.skillRecommendVersion == version {
		return
	}
	for _, s := range p.registry.All() {
		p.skillRecommender.RegisterItem(p.recommendationProfileForSkill(s))
	}
	p.skillRecommendVersion = version
}

func (p *Planner) recommendationProfileForSkill(s skills.Skill) recommend.ItemProfile {
	name := s.Name()
	category := p.registry.CategoryOf(name)
	if category == "" {
		category = "uncategorized"
	}
	tags := skillRecommendationTags(name, s.Description(), category)
	return recommend.ItemProfile{
		ID:       name,
		Category: category,
		Tags:     tags,
	}
}

func skillRecommendationTags(name, desc, category string) []string {
	seen := map[string]bool{}
	add := func(raw string) {
		raw = strings.Trim(strings.ToLower(raw), " \t\r\n.,;:!?()[]{}<>\"'`")
		if raw == "" || seen[raw] {
			return
		}
		seen[raw] = true
	}
	add(category)
	for _, part := range strings.FieldsFunc(name, func(r rune) bool {
		return r == '_' || r == '-' || r == '.' || r == '/' || r == '\\'
	}) {
		add(part)
	}
	for _, part := range strings.Fields(desc) {
		add(part)
		if len(seen) >= 24 {
			break
		}
	}
	out := make([]string, 0, len(seen))
	for tag := range seen {
		out = append(out, tag)
	}
	sort.Strings(out)
	return out
}

func (p *Planner) rankSkillsByRecommendation(userMessage string, candidates []skills.Skill) []skills.Skill {
	if p.skillRecommender == nil || len(candidates) < 2 {
		return candidates
	}
	p.skillRecommendMu.Lock()
	p.syncSkillRecommendationItemsLocked()
	engine := p.skillRecommender
	p.skillRecommendMu.Unlock()
	if engine == nil {
		return candidates
	}

	candidateNames := make([]string, 0, len(candidates))
	byName := make(map[string]skills.Skill, len(candidates))
	for _, s := range candidates {
		name := s.Name()
		candidateNames = append(candidateNames, name)
		byName[name] = s
	}
	recs := engine.RecommendCandidates(len(candidates), userMessage, candidateNames)
	if len(recs) == 0 {
		return candidates
	}

	ranked := make([]skills.Skill, 0, len(candidates))
	used := make(map[string]bool, len(recs))
	for _, rec := range recs {
		if s, ok := byName[rec.ItemID]; ok {
			ranked = append(ranked, s)
			used[rec.ItemID] = true
		}
	}
	for _, s := range candidates {
		if !used[s.Name()] {
			ranked = append(ranked, s)
		}
	}
	return ranked
}

func (p *Planner) recordSkillRecommendationOutcome(skillName string, success bool) {
	if p.skillRecommender == nil || strings.TrimSpace(skillName) == "" {
		return
	}
	p.skillRecommendMu.Lock()
	p.syncSkillRecommendationItemsLocked()
	engine := p.skillRecommender
	p.skillRecommendMu.Unlock()
	if engine == nil {
		return
	}
	rating := 0.2
	if success {
		rating = 0.9
	}
	engine.RecordOutcome(skillName, rating, success)
}
