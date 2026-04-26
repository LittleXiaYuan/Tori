package memory

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// FactTransformFunc is a hook for plugins to enrich/filter/tag facts
// after extraction but before the decision phase. Return nil to suppress all.
type FactTransformFunc func(ctx context.Context, facts []string) []string

// Pipeline orchestrates the memory lifecycle:
// conversation → extract facts → transform (plugins) → decide mutations → apply → compact.
type Pipeline struct {
	extractor      *Extractor
	decider        *Decider
	compactor      *Compactor
	manager        *Manager
	graph          *Graph
	dailyDir       string            // directory for daily markdown memory files
	factTransform  FactTransformFunc // optional: plugin fact transformation hook
}

func NewPipeline(chatFn ChatFunc, manager *Manager) *Pipeline {
	return &Pipeline{
		extractor: NewExtractor(chatFn),
		decider:   NewDecider(chatFn),
		compactor: NewCompactor(chatFn),
		manager:   manager,
		graph:     NewGraph(),
	}
}

type ProcessResult struct {
	ExtractedFacts []string `json:"extracted_facts"`
	Added          int      `json:"added"`
	Updated        int      `json:"updated"`
	Deleted        int      `json:"deleted"`
	Skipped        int      `json:"skipped"`
	Entities       int      `json:"entities"`
	Relations      int      `json:"relations"`
}

func (p *Pipeline) Graph() *Graph { return p.graph }

func (p *Pipeline) SetGraph(g *Graph) { p.graph = g }

func (p *Pipeline) SetDailyDir(dir string) { p.dailyDir = dir }

func (p *Pipeline) SetFactTransform(fn FactTransformFunc) { p.factTransform = fn }

func (p *Pipeline) Process(ctx context.Context, tenantID string, messages []ChatMessage) (*ProcessResult, error) {
	result := &ProcessResult{}

	// Phase 1: Extract facts from conversation
	extracted, err := p.extractor.Extract(ctx, messages)
	if err != nil {
		slog.Warn("memory pipeline: extraction failed", "err", err)
		return result, fmt.Errorf("memory extraction: %w", err)
	}
	if len(extracted.Facts) == 0 {
		return result, nil
	}

	// Phase 1.5: Plugin fact transformation hook
	// CognitivePlugins and HookManager can enrich, filter, or tag facts
	// before they enter the decision phase.
	facts := extracted.Facts
	if p.factTransform != nil {
		facts = p.factTransform(ctx, facts)
		if len(facts) == 0 {
			slog.Info("memory pipeline: all facts suppressed by transform hook")
			return result, nil
		}
		extracted.Facts = facts
	}
	result.ExtractedFacts = extracted.Facts

	// Phase 2: Gather existing relevant memories as candidates
	candidates, err := p.gatherCandidates(ctx, tenantID, extracted.Facts)
	if err != nil {
		slog.Warn("memory pipeline: candidate search failed", "err", err)
	}

	// Phase 3: Decide what to do with each fact
	decided, err := p.decider.Decide(ctx, extracted.Facts, candidates)
	if err != nil {
		// Fallback: add all facts directly
		slog.Warn("memory pipeline: decision failed, adding all facts", "err", err)
		for _, fact := range extracted.Facts {
			_ = p.manager.AddMid(ctx, tenantID, Item{
				Key:    uuid.New().String(),
				Value:  fact,
				Source: "pipeline",
			})
			result.Added++
		}
		return result, nil
	}

	// Phase 4: Apply actions
	for _, action := range decided.Actions {
		switch action.Op {
		case "ADD":
			err := p.manager.AddMid(ctx, tenantID, Item{
				Key:      uuid.New().String(),
				Value:    action.Text,
				Source:   "pipeline",
				Category: "fact",
			})
			if err != nil {
				slog.Warn("memory pipeline: add failed", "err", err, "text", truncateLog(action.Text))
			} else {
				result.Added++
			}

		case "UPDATE":
			if action.ID != "" {
				_ = p.manager.Mid.Delete(ctx, tenantID, action.ID)
			}
			err := p.manager.AddMid(ctx, tenantID, Item{
				Key:      action.ID,
				Value:    action.Text,
				Source:   "pipeline",
				Category: "fact",
			})
			if err != nil {
				slog.Warn("memory pipeline: update failed", "err", err)
			} else {
				result.Updated++
			}

		case "DELETE":
			if action.ID != "" {
				_ = p.manager.Mid.Delete(ctx, tenantID, action.ID)
				result.Deleted++
			}

		default:
			result.Skipped++
		}
	}

	// Phase 5: Extract entities and relations into knowledge graph
	if p.graph != nil {
		result.Entities, result.Relations = p.extractToGraphLLM(ctx, extracted.Facts)
	}

	// Phase 6: Persist extracted facts to daily markdown file
	if p.dailyDir != "" && len(extracted.Facts) > 0 {
		if err := appendDailyFile(p.dailyDir, tenantID, extracted.Facts); err != nil {
			slog.Warn("daily memory file write failed", "err", err)
		}
	}

	slog.Info("memory pipeline complete",
		"tenant", tenantID,
		"facts", len(extracted.Facts),
		"added", result.Added,
		"updated", result.Updated,
		"deleted", result.Deleted,
		"entities", result.Entities,
		"relations", result.Relations,
	)
	return result, nil
}

func (p *Pipeline) gatherCandidates(ctx context.Context, tenantID string, facts []string) ([]Candidate, error) {
	query := strings.Join(facts, " ")
	if len(query) > 500 {
		query = query[:500]
	}

	items, err := p.manager.Mid.Search(ctx, tenantID, query, 20)
	if err != nil {
		return nil, err
	}

	candidates := make([]Candidate, 0, len(items))
	for _, item := range items {
		candidates = append(candidates, Candidate{
			ID:        item.Key,
			Content:   item.Value,
			CreatedAt: item.CreatedAt.Format(time.RFC3339),
		})
	}
	return candidates, nil
}

func (p *Pipeline) Compact(ctx context.Context, tenantID string, targetCount, decayDays int) (*CompactOutput, error) {
	existing, err := p.manager.Mid.List(ctx, tenantID, "", 0)
	if err != nil {
		return nil, fmt.Errorf("list memories for compact: %w", err)
	}
	if len(existing) <= 1 {
		return &CompactOutput{BeforeCount: len(existing), AfterCount: len(existing)}, nil
	}

	candidates := make([]Candidate, 0, len(existing))
	for _, item := range existing {
		candidates = append(candidates, Candidate{
			ID:        item.Key,
			Content:   item.Value,
			CreatedAt: item.CreatedAt.Format(time.RFC3339),
		})
	}

	output, err := p.compactor.Compact(ctx, CompactInput{
		Memories:    candidates,
		TargetCount: targetCount,
		DecayDays:   decayDays,
	})
	if err != nil {
		return nil, err
	}

	// Replace all mid-term memories with compacted set
	for _, item := range existing {
		_ = p.manager.Mid.Delete(ctx, tenantID, item.Key)
	}
	for _, fact := range output.Facts {
		_ = p.manager.AddMid(ctx, tenantID, Item{
			Key:      uuid.New().String(),
			Value:    fact,
			Source:   "compacted",
			Category: "fact",
		})
	}

	slog.Info("memory compact complete",
		"tenant", tenantID,
		"before", output.BeforeCount,
		"after", output.AfterCount,
	)
	return output, nil
}

// extractToGraphLLM tries the structured-LLM path first
// (Extractor.ExtractGraph) and falls back to the keyword extractor when the
// LLM fails or returns nothing useful. This addresses the historical
// keyword-only HACK while keeping a no-dependency fallback for offline /
// degraded deployments.
func (p *Pipeline) extractToGraphLLM(ctx context.Context, facts []string) (entities, relations int) {
	if p.extractor != nil {
		if res, err := p.extractor.ExtractGraph(ctx, facts); err == nil && res != nil && len(res.Items) > 0 {
			return p.applyGraphItems(res.Items)
		} else if err != nil {
			slog.Warn("memory pipeline: LLM graph extract failed, falling back to keyword extractor", "err", err)
		}
	}
	return p.extractToGraph(facts)
}

// applyGraphItems writes structured triples returned by ExtractGraph into the
// knowledge graph. Subject and object become entities; predicate becomes the
// relation type. Falls through silently for malformed items.
func (p *Pipeline) applyGraphItems(items []GraphFact) (entities, relations int) {
	for _, item := range items {
		if item.Object == "" || item.Predicate == "" {
			continue
		}
		objKind := item.ObjectKind
		if objKind == "" {
			objKind = "concept"
		}
		objEnt := p.graph.PutEntity(Entity{
			ID:         entityID(item.Object),
			Name:       truncateName(item.Object),
			Type:       objKind,
			Properties: map[string]string{"source_fact": item.Fact},
		})
		entities++
		subj := item.Subject
		if subj == "" {
			subj = "用户"
		}
		subjEnt := p.graph.PutEntity(Entity{
			ID:   entityID("__user__" + subj),
			Name: subj,
			Type: "person",
		})
		p.graph.PutRelation(Relation{
			ID:      fmt.Sprintf("r_%s_%s_%s", subjEnt.ID, item.Predicate, objEnt.ID),
			FromID:  subjEnt.ID,
			ToID:    objEnt.ID,
			Type:    item.Predicate,
			Context: item.Fact,
		})
		relations++
	}
	return entities, relations
}

// extractToGraph: keyword-only pattern-match fallback used when no structured
// extractor is configured or the LLM call fails. Retained as a deterministic,
// dependency-free path for offline/personal deployments.
func (p *Pipeline) extractToGraph(facts []string) (entities, relations int) {
	// Entity type detection patterns (Chinese + English)
	personPatterns := []string{"用户", "他", "她", "我", "Alice", "Bob", "老师", "同事", "朋友", "user", "person"}
	placePatterns := []string{"在", "位于", "住在", "located", "lives in", "city", "country", "公司", "学校"}
	projectPatterns := []string{"项目", "project", "开发", "develop", "系统", "system", "应用", "app", "平台"}
	skillPatterns := []string{"会", "擅长", "使用", "学习", "knows", "uses", "skilled", "programming", "语言", "框架"}
	prefPatterns := []string{"喜欢", "偏好", "prefer", "like", "love", "favorite", "讨厌", "不喜欢", "dislike"}

	// Relation extraction patterns
	type relPattern struct {
		keywords []string
		relType  string
	}
	relPatterns := []relPattern{
		{[]string{"使用", "用", "uses", "using"}, "uses"},
		{[]string{"喜欢", "偏好", "prefer", "like", "love"}, "prefers"},
		{[]string{"工作", "就职", "works", "employed"}, "works_at"},
		{[]string{"开发", "维护", "develop", "maintain", "负责"}, "works_on"},
		{[]string{"学习", "学", "learning", "studying"}, "learning"},
		{[]string{"住在", "位于", "lives", "located"}, "located_in"},
		{[]string{"认识", "知道", "knows", "know"}, "knows"},
		{[]string{"属于", "部分", "part of", "belongs"}, "part_of"},
	}

	for _, fact := range facts {
		lower := strings.ToLower(fact)

		// Determine entity type from fact content
		entityType := "concept"
		if matchAny(lower, personPatterns) {
			entityType = "person"
		} else if matchAny(lower, placePatterns) {
			entityType = "place"
		} else if matchAny(lower, projectPatterns) {
			entityType = "project"
		} else if matchAny(lower, skillPatterns) {
			entityType = "skill"
		} else if matchAny(lower, prefPatterns) {
			entityType = "preference"
		}

		// Create entity from the fact
		eid := entityID(fact)
		p.graph.PutEntity(Entity{
			ID:         eid,
			Name:       truncateName(fact),
			Type:       entityType,
			Properties: map[string]string{"source_fact": fact},
		})
		entities++

		// Try to extract relations
		for _, rp := range relPatterns {
			if matchAny(lower, rp.keywords) {
				// Create a relation from a generic "user" entity to this fact-entity
				userEnt := p.graph.PutEntity(Entity{
					ID:   "user_self",
					Name: "用户",
					Type: "person",
				})
				rid := fmt.Sprintf("r_%s_%s", userEnt.ID, eid)
				p.graph.PutRelation(Relation{
					ID:      rid,
					FromID:  userEnt.ID,
					ToID:    eid,
					Type:    rp.relType,
					Context: fact,
				})
				relations++
				break // one relation per fact
			}
		}
	}

	return entities, relations
}

func matchAny(s string, patterns []string) bool {
	for _, p := range patterns {
		if strings.Contains(s, strings.ToLower(p)) {
			return true
		}
	}
	return false
}

// Simple deterministic ID (FNV-1a) from fact text.
func entityID(fact string) string {
	// Simple deterministic ID from fact content
	var h uint64 = 14695981039346656037
	for i := 0; i < len(fact); i++ {
		h ^= uint64(fact[i])
		h *= 1099511628211
	}
	return fmt.Sprintf("e_%x", h)
}

func truncateName(s string) string {
	r := []rune(s)
	if len(r) > 50 {
		return string(r[:50]) + "..."
	}
	return s
}

func truncateLog(s string) string {
	if len(s) > 80 {
		return s[:80] + "..."
	}
	return s
}

// ---- daily markdown persistence (similar to OpenClaw's MEMORY.md) ----
func appendDailyFile(dir, tenantID string, facts []string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	filename := filepath.Join(dir, time.Now().Format("2006-01-02")+".md")
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	timestamp := time.Now().Format("15:04:05")
	for _, fact := range facts {
		line := fmt.Sprintf("- [%s][%s] %s\n", timestamp, tenantID, fact)
		if _, err := f.WriteString(line); err != nil {
			return err
		}
	}
	return nil
}

// LoadDailyFiles rehydrates mid-term memory from persisted daily markdown files.
func LoadDailyFiles(dir string, mgr *Manager) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	loaded := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "- [") {
				continue
			}
			// Parse: "- [HH:MM:SS][tenantID] fact text"
			// Find the fact text after the second ] bracket
			idx := strings.Index(line, "] ")
			if idx < 0 {
				continue
			}
			// Find tenant bracket
			rest := line[idx+2:]
			tenantEnd := strings.Index(line[3:], "][")
			tenantID := "default"
			fact := rest
			if tenantEnd >= 0 {
				// Extract tenant from "- [time][tenant] fact"
				bracketStart := 3 + tenantEnd + 2 // skip "]["
				bracketEnd := strings.Index(line[bracketStart:], "]")
				if bracketEnd >= 0 {
					tenantID = line[bracketStart : bracketStart+bracketEnd]
					fact = strings.TrimSpace(line[bracketStart+bracketEnd+1:])
				}
			}
			if fact == "" {
				continue
			}
			_ = mgr.AddMid(context.Background(), tenantID, Item{
				Key:      uuid.New().String(),
				Value:    fact,
				Source:   "daily_file",
				Category: "fact",
			})
			loaded++
		}
	}
	return loaded, nil
}
