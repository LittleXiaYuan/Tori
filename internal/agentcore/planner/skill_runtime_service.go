package planner

import (
	"context"
	"sort"
	"strings"
	"sync"

	"yunque-agent/internal/cognicore/recommend"
	"yunque-agent/pkg/skills"
)

// SkillRuntimeService groups skill routing, scoring, recommendation,
// optimization hints, and growth adapters that used to live as separate
// Planner fields. Planner still executes skills; this service owns the
// evolving skill surface around that execution.
type SkillRuntimeService struct {
	registry *skills.Registry

	growth    *SkillGrowth
	optimizer *SkillOptimizer
	scorer    *skills.SkillScorer

	recommender      *recommend.Engine
	recommendVersion int
	recommendMu      sync.Mutex

	recent   []string
	recentMu sync.Mutex
}

func NewSkillRuntimeService(registry *skills.Registry) *SkillRuntimeService {
	return &SkillRuntimeService{registry: registry}
}

func (s *SkillRuntimeService) SetRegistry(registry *skills.Registry) { s.registry = registry }
func (s *SkillRuntimeService) SetGrowth(growth *SkillGrowth)         { s.growth = growth }
func (s *SkillRuntimeService) Growth() *SkillGrowth {
	if s == nil {
		return nil
	}
	return s.growth
}

func (s *SkillRuntimeService) SetOptimizer(opt *SkillOptimizer) { s.optimizer = opt }
func (s *SkillRuntimeService) OptimizationHints() string {
	if s == nil || s.optimizer == nil {
		return ""
	}
	return s.optimizer.OptimizationHints()
}

func (s *SkillRuntimeService) SetScorer(scorer *skills.SkillScorer) { s.scorer = scorer }

func (s *SkillRuntimeService) SetRecommendationEngine(engine *recommend.Engine) {
	if s == nil {
		return
	}
	s.recommendMu.Lock()
	defer s.recommendMu.Unlock()
	s.recommender = engine
	s.recommendVersion = -1
	if engine != nil {
		s.syncRecommendationItemsLocked()
	}
}

func (s *SkillRuntimeService) RecordRecent(skillNames []string) {
	if s == nil || len(skillNames) == 0 {
		return
	}
	s.recentMu.Lock()
	defer s.recentMu.Unlock()
	s.recent = append(skillNames, s.recent...)
	if len(s.recent) > 20 {
		s.recent = s.recent[:20]
	}
}

// ScorerWithRecent returns the intent scorer enriched with the recently-used
// skills tracked via RecordRecent. It returns a fresh scorer (never the shared
// base) so callers can't mutate service state, and — critically — it activates
// the recency signal even when no base scorer was set via SetScorer. Previously
// it returned nil whenever the base scorer was unset (the production reality),
// which silently discarded the tracked recency too, so ScoreCategories never got
// its recency bonus. Returns nil only when there is genuinely no signal.
func (s *SkillRuntimeService) ScorerWithRecent() *skills.SkillScorer {
	if s == nil {
		return nil
	}
	s.recentMu.Lock()
	recent := append([]string(nil), s.recent...)
	s.recentMu.Unlock()

	if s.scorer == nil && len(recent) == 0 {
		return nil
	}
	out := &skills.SkillScorer{RecentSkills: recent}
	if s.scorer != nil {
		out.SuccessRates = s.scorer.SuccessRates
	}
	return out
}

func (s *SkillRuntimeService) RankByRecommendation(userMessage string, candidates []skills.Skill) []skills.Skill {
	if s == nil || s.recommender == nil || len(candidates) < 2 {
		return candidates
	}
	s.recommendMu.Lock()
	s.syncRecommendationItemsLocked()
	engine := s.recommender
	s.recommendMu.Unlock()
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

func (s *SkillRuntimeService) RecordRecommendationOutcome(skillName string, success bool) {
	if s == nil || s.recommender == nil || strings.TrimSpace(skillName) == "" {
		return
	}
	s.recommendMu.Lock()
	s.syncRecommendationItemsLocked()
	engine := s.recommender
	s.recommendMu.Unlock()
	if engine == nil {
		return
	}
	rating := 0.2
	if success {
		rating = 0.9
	}
	engine.RecordOutcome(skillName, rating, success)
}

func (s *SkillRuntimeService) syncRecommendationItemsLocked() {
	if s.recommender == nil || s.registry == nil {
		return
	}
	version := s.registry.Version()
	if s.recommendVersion == version {
		return
	}
	for _, skill := range s.registry.All() {
		s.recommender.RegisterItem(s.recommendationProfileForSkill(skill))
	}
	s.recommendVersion = version
}

func (s *SkillRuntimeService) recommendationProfileForSkill(skill skills.Skill) recommend.ItemProfile {
	name := skill.Name()
	category := s.registry.CategoryOf(name)
	if category == "" {
		category = "uncategorized"
	}
	return recommend.ItemProfile{
		ID:       name,
		Category: category,
		Tags:     skillRecommendationTags(name, skill.Description(), category),
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

func (s *SkillRuntimeService) TryAcquire(ctx context.Context, skillName, failureContext string) (string, string) {
	if s == nil || s.growth == nil {
		return "", "skill growth disabled"
	}
	return s.growth.TryAcquire(ctx, skillName, failureContext)
}
