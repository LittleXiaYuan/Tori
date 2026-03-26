package memory

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

// ──────────────────────────────────────────────
// MemoryOrchestrator — five-layer memory integration
// Connects: ShortTerm → MidTerm → LongTerm → Graph → Editable → Observation
// Provides: unified recall, auto-promotion, importance scoring, decay
// ──────────────────────────────────────────────

// Importance levels for memory promotion decisions.
type Importance int

const (
	ImportanceLow    Importance = 1
	ImportanceMedium Importance = 2
	ImportanceHigh   Importance = 3
)

// RecallItem is a unified search result from any memory layer.
type RecallItem struct {
	Content    string    `json:"content"`
	Source     string    `json:"source"`     // "short", "mid", "long", "graph", "editable", "observation"
	Category   string    `json:"category,omitempty"`
	Score      float64   `json:"score"`      // final weighted score
	RawScore   float64   `json:"raw_score"`  // score before weighting
	Importance Importance `json:"importance"`
	Age        time.Duration `json:"age"`
	AccessCount int       `json:"access_count,omitempty"`
}

// ImportanceFunc evaluates the importance of a piece of content.
// Returns ImportanceLow/Medium/High. Can use LLM or heuristics.
type ImportanceFunc func(ctx context.Context, content string) Importance

// OrchestratorConfig configures the memory orchestrator.
type OrchestratorConfig struct {
	// Layer weights for unified recall (0.0 - 1.0)
	ShortWeight       float64 `json:"short_weight"`       // default: 0.5
	MidWeight         float64 `json:"mid_weight"`         // default: 0.8
	LongWeight        float64 `json:"long_weight"`        // default: 1.0
	GraphWeight       float64 `json:"graph_weight"`       // default: 0.9
	EditableWeight    float64 `json:"editable_weight"`    // default: 0.95
	ObservationWeight float64 `json:"observation_weight"` // default: 0.7

	// Promotion thresholds
	ShortToMidAccessCount int           `json:"short_to_mid_access"`  // promote after N accesses
	MidToLongAccessCount  int           `json:"mid_to_long_access"`   // promote after N accesses
	ShortToMidAge         time.Duration `json:"short_to_mid_age"`     // promote items older than this
	DecayHalfLife         time.Duration `json:"decay_half_life"`      // score halves after this duration
	MaxRecallResults      int           `json:"max_recall_results"`
}

// DefaultOrchestratorConfig returns sensible defaults.
func DefaultOrchestratorConfig() OrchestratorConfig {
	return OrchestratorConfig{
		ShortWeight:           0.5,
		MidWeight:             0.8,
		LongWeight:            1.0,
		GraphWeight:           0.9,
		EditableWeight:        0.95,
		ObservationWeight:     0.7,
		ShortToMidAccessCount: 3,
		MidToLongAccessCount:  10,
		ShortToMidAge:         15 * time.Minute,
		DecayHalfLife:         7 * 24 * time.Hour, // 1 week
		MaxRecallResults:      20,
	}
}

// Orchestrator coordinates five memory layers into one unified system.
type Orchestrator struct {
	mu               sync.RWMutex
	config           OrchestratorConfig
	manager          *Manager           // short + mid + long
	graph            *Graph             // knowledge graph
	editable         *EditableMemory    // agent-editable blocks
	importanceFn     ImportanceFunc
	conflictDetector *ConflictDetector  // optional: detects memory contradictions
	promotionLog     []promotionEntry
	conflictLog      []Conflict         // recent conflicts detected
}

type promotionEntry struct {
	From      string    `json:"from"`
	To        string    `json:"to"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// NewOrchestrator creates a memory orchestrator.
func NewOrchestrator(cfg OrchestratorConfig, mgr *Manager, g *Graph, em *EditableMemory) *Orchestrator {
	return &Orchestrator{
		config:   cfg,
		manager:  mgr,
		graph:    g,
		editable: em,
	}
}

// SetImportanceFunc sets the importance evaluator.
func (o *Orchestrator) SetImportanceFunc(fn ImportanceFunc) {
	o.importanceFn = fn
}

// SetConflictDetector enables memory conflict detection.
func (o *Orchestrator) SetConflictDetector(cd *ConflictDetector) {
	o.conflictDetector = cd
}

// Conflicts returns the recent conflict log.
func (o *Orchestrator) Conflicts() []Conflict {
	o.mu.RLock()
	defer o.mu.RUnlock()
	out := make([]Conflict, len(o.conflictLog))
	copy(out, o.conflictLog)
	return out
}

// ──────────────────────────────────────────────
// Unified Recall — search all layers at once
// ──────────────────────────────────────────────

// Recall performs a unified search across all memory layers.
func (o *Orchestrator) Recall(ctx context.Context, tenantID, query string, limit int) []RecallItem {
	if limit <= 0 {
		limit = o.config.MaxRecallResults
	}
	perLayer := limit * 2

	var allResults []RecallItem
	var wg sync.WaitGroup
	var mu sync.Mutex

	// 1. Short/Mid/Long via Manager
	wg.Add(1)
	go func() {
		defer wg.Done()
		items, err := o.manager.SearchAll(ctx, tenantID, query, perLayer)
		if err != nil {
			return
		}
		mu.Lock()
		for _, item := range items {
			layer := "short"
			if strings.HasPrefix(item.Source, "mid:") {
				layer = "mid"
			} else if strings.HasPrefix(item.Source, "long:") {
				layer = "long"
			}
			ri := RecallItem{
				Content:     item.Value,
				Source:      layer,
				Category:    item.Category,
				RawScore:    item.Score,
				Score:       item.Score * o.layerWeight(layer),
				AccessCount: item.AccessCnt,
				Age:         time.Since(item.CreatedAt),
			}
			ri.Score *= o.decayFactor(ri.Age)
			allResults = append(allResults, ri)
		}
		mu.Unlock()
	}()

	// 2. Knowledge Graph
	if o.graph != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			entities := o.graph.SearchEntities(query, perLayer)
			mu.Lock()
			for _, e := range entities {
				context := o.graph.ContextFor(e.ID)
				ri := RecallItem{
					Content:  context,
					Source:   "graph",
					Category: e.Type,
					RawScore: float64(e.Mentions) * 0.1,
					Age:      time.Since(e.CreatedAt),
				}
				ri.Score = ri.RawScore * o.config.GraphWeight * o.decayFactor(ri.Age)
				allResults = append(allResults, ri)
			}
			mu.Unlock()
		}()
	}

	// 3. Editable Memory blocks
	if o.editable != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			blocks := o.editable.AllBlocks()
			queryLower := strings.ToLower(query)
			mu.Lock()
			for _, b := range blocks {
				if query == "" || strings.Contains(strings.ToLower(b.Content), queryLower) ||
					strings.Contains(strings.ToLower(b.Label), queryLower) {
					ri := RecallItem{
						Content:  fmt.Sprintf("[%s] %s", b.Label, b.Content),
						Source:   "editable",
						Category: b.Label,
						RawScore: 1.0, // editable blocks are high priority
						Age:      time.Since(b.UpdatedAt),
					}
					ri.Score = ri.RawScore * o.config.EditableWeight
					allResults = append(allResults, ri)
				}
			}
			mu.Unlock()
		}()
	}

	wg.Wait()

	// Sort by final score
	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].Score > allResults[j].Score
	})

	if len(allResults) > limit {
		allResults = allResults[:limit]
	}

	slog.Debug("orchestrator: recall", "query", query, "results", len(allResults))
	return allResults
}

// ──────────────────────────────────────────────
// Ingest — store with automatic layer routing
// ──────────────────────────────────────────────

// Ingest stores content in the appropriate layer based on importance.
func (o *Orchestrator) Ingest(ctx context.Context, tenantID, content, category, source string) error {
	importance := o.evaluateImportance(ctx, content)

	item := Item{
		Value:    content,
		Category: category,
		Source:   source,
	}

	// Async conflict detection: check new content against existing memories
	if o.conflictDetector != nil && importance >= ImportanceMedium {
		go o.detectAndResolveConflicts(ctx, tenantID, content)
	}

	switch importance {
	case ImportanceLow:
		return o.manager.Short.Put(ctx, tenantID, item)
	case ImportanceMedium:
		return o.manager.AddMid(ctx, tenantID, item)
	case ImportanceHigh:
		if err := o.manager.AddMid(ctx, tenantID, item); err != nil {
			return err
		}
		return o.manager.AddLong(ctx, tenantID, item)
	}
	return o.manager.Short.Put(ctx, tenantID, item)
}

// detectAndResolveConflicts runs asynchronously after Ingest to find and handle
// contradictions between new content and existing memories.
func (o *Orchestrator) detectAndResolveConflicts(ctx context.Context, tenantID, newContent string) {
	// Recall existing memories related to the new content
	existing := o.Recall(ctx, tenantID, newContent, 10)
	if len(existing) == 0 {
		return
	}

	conflicts := o.conflictDetector.DetectConflicts(ctx, newContent, existing)
	if len(conflicts) == 0 {
		return
	}

	o.mu.Lock()
	o.conflictLog = append(o.conflictLog, conflicts...)
	// Keep only last 100 conflicts
	if len(o.conflictLog) > 100 {
		o.conflictLog = o.conflictLog[len(o.conflictLog)-100:]
	}
	o.mu.Unlock()

	for _, c := range conflicts {
		switch c.Resolution {
		case ResOverwrite:
			if c.Confidence >= 0.7 {
				slog.Info("conflict: auto-overwrite",
					"subject", c.Subject,
					"old", truncate(c.OldFact, 60),
					"new", truncate(c.NewFact, 60),
					"confidence", c.Confidence,
				)
				// Mark old fact as superseded in editable memory
				if o.editable != nil {
					o.editable.AddBlock("_superseded:"+c.Subject, fmt.Sprintf(
						"[已过时] %s → 新事实: %s",
						truncate(c.OldFact, 80), truncate(c.NewFact, 80),
					), 500)
				}
			}
		case ResMerge:
			slog.Info("conflict: merge (both facts kept with annotation)",
				"subject", c.Subject,
				"confidence", c.Confidence,
			)
		case ResKeepBoth:
			slog.Debug("conflict: ambiguous, flagged for review",
				"subject", c.Subject,
			)
		}
	}
}

// truncate shortens a string for log display.
func truncate(s string, maxRunes int) string {
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes]) + "…"
}

// ──────────────────────────────────────────────
// Auto-Promotion — move memories between layers
// ──────────────────────────────────────────────

// Promote scans short-term memory and promotes qualifying items to mid-term.
// Also promotes frequently accessed mid-term items to long-term.
func (o *Orchestrator) Promote(ctx context.Context, tenantID string) (promoted int) {
	// Short → Mid: items accessed multiple times or aged
	shortItems, _ := o.manager.Short.List(ctx, tenantID, "", 100)
	for _, item := range shortItems {
		shouldPromote := false
		if item.AccessCnt >= o.config.ShortToMidAccessCount {
			shouldPromote = true
		}
		if !item.CreatedAt.IsZero() && time.Since(item.CreatedAt) > o.config.ShortToMidAge && item.AccessCnt > 0 {
			shouldPromote = true
		}

		if shouldPromote {
			item.Source = "promoted:short→mid"
			if err := o.manager.AddMid(ctx, tenantID, item); err == nil {
				promoted++
				o.logPromotion("short", "mid", item.Value)
			}
		}
	}

	// Mid → Long: items accessed many times
	midItems, _ := o.manager.Mid.List(ctx, tenantID, "", 200)
	for _, item := range midItems {
		if item.AccessCnt >= o.config.MidToLongAccessCount {
			item.Source = "promoted:mid→long"
			if err := o.manager.AddLong(ctx, tenantID, item); err == nil {
				promoted++
				o.logPromotion("mid", "long", item.Value)
			}
		}
	}

	if promoted > 0 {
		slog.Info("orchestrator: promoted", "count", promoted, "tenant", tenantID)
	}
	return promoted
}

// ──────────────────────────────────────────────
// Graph Bridge — connect entities with memory items
// ──────────────────────────────────────────────

// LinkEntityToMemory creates a relation between a graph entity and a memory item.
func (o *Orchestrator) LinkEntityToMemory(entityID, memoryKey, relationType string) {
	if o.graph == nil {
		return
	}
	o.graph.PutRelation(Relation{
		ID:     fmt.Sprintf("mem_%s_%s", entityID, memoryKey),
		FromID: entityID,
		ToID:   "mem:" + memoryKey,
		Type:   relationType,
		Weight: 0.7,
	})
}

// RecallForEntity retrieves all memory context relevant to a specific entity.
func (o *Orchestrator) RecallForEntity(ctx context.Context, tenantID, entityName string, limit int) []RecallItem {
	if o.graph == nil {
		return nil
	}
	entity, ok := o.graph.FindByName(entityName)
	if !ok {
		// Fall back to query-based recall
		return o.Recall(ctx, tenantID, entityName, limit)
	}

	// Get entity context + neighbors
	var results []RecallItem
	context := o.graph.ContextFor(entity.ID)
	if context != "" {
		results = append(results, RecallItem{
			Content:  context,
			Source:   "graph",
			Category: entity.Type,
			Score:    1.0,
		})
	}

	// Also search memories mentioning this entity
	memResults := o.Recall(ctx, tenantID, entityName, limit)
	results = append(results, memResults...)

	if len(results) > limit {
		results = results[:limit]
	}
	return results
}

// ──────────────────────────────────────────────
// Compile — build system prompt memory context
// ──────────────────────────────────────────────

// CompileContext builds a memory-enriched context string for the system prompt.
func (o *Orchestrator) CompileContext(ctx context.Context, tenantID, currentQuery string) string {
	var sb strings.Builder

	// 1. Editable memory blocks (always included)
	if o.editable != nil {
		compiled := o.editable.Compile()
		if compiled != "" {
			sb.WriteString(compiled)
		}
	}

	// 2. Relevant recalled memories
	if currentQuery != "" {
		items := o.Recall(ctx, tenantID, currentQuery, 10)
		if len(items) > 0 {
			sb.WriteString("<recalled_memories>\n")
			for _, item := range items {
				sb.WriteString(fmt.Sprintf("- [%s] %s\n", item.Source, item.Content))
			}
			sb.WriteString("</recalled_memories>\n\n")
		}
	}

	return sb.String()
}

// ──────────────────────────────────────────────
// Decay — time-based score reduction
// ──────────────────────────────────────────────

func (o *Orchestrator) decayFactor(age time.Duration) float64 {
	if o.config.DecayHalfLife <= 0 {
		return 1.0
	}
	// Exponential decay: score * 0.5^(age/halfLife)
	halfLives := float64(age) / float64(o.config.DecayHalfLife)
	return math.Pow(0.5, halfLives)
}

func (o *Orchestrator) layerWeight(layer string) float64 {
	switch layer {
	case "short":
		return o.config.ShortWeight
	case "mid":
		return o.config.MidWeight
	case "long":
		return o.config.LongWeight
	case "graph":
		return o.config.GraphWeight
	case "editable":
		return o.config.EditableWeight
	case "observation":
		return o.config.ObservationWeight
	}
	return 0.5
}

func (o *Orchestrator) evaluateImportance(ctx context.Context, content string) Importance {
	if o.importanceFn != nil {
		return o.importanceFn(ctx, content)
	}
	// Heuristic fallback
	return heuristicImportance(content)
}

// heuristicImportance uses simple rules to estimate importance.
func heuristicImportance(content string) Importance {
	lower := strings.ToLower(content)
	length := len(content)

	// High importance indicators
	highKeywords := []string{"important", "remember", "always", "never", "password", "key", "secret",
		"重要", "记住", "永远", "密码", "关键", "核心"}
	for _, kw := range highKeywords {
		if strings.Contains(lower, kw) {
			return ImportanceHigh
		}
	}

	// Medium importance: longer content, facts, preferences
	if length > 100 {
		return ImportanceMedium
	}
	medKeywords := []string{"prefer", "like", "dislike", "name is", "work at",
		"喜欢", "不喜欢", "叫做", "工作"}
	for _, kw := range medKeywords {
		if strings.Contains(lower, kw) {
			return ImportanceMedium
		}
	}

	return ImportanceLow
}

func (o *Orchestrator) logPromotion(from, to, content string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	entry := promotionEntry{
		From:      from,
		To:        to,
		Content:   truncateStr(content, 100),
		Timestamp: time.Now(),
	}
	o.promotionLog = append(o.promotionLog, entry)
	if len(o.promotionLog) > 200 {
		o.promotionLog = o.promotionLog[len(o.promotionLog)-200:]
	}
}

// PromotionLog returns recent promotions.
func (o *Orchestrator) PromotionLog(limit int) []promotionEntry {
	o.mu.RLock()
	defer o.mu.RUnlock()
	if limit <= 0 || limit > len(o.promotionLog) {
		limit = len(o.promotionLog)
	}
	start := len(o.promotionLog) - limit
	out := make([]promotionEntry, limit)
	copy(out, o.promotionLog[start:])
	return out
}

// Stats returns orchestrator statistics.
type OrchestratorStats struct {
	ShortCount    int `json:"short_count"`
	MidCount      int `json:"mid_count"`
	LongCount     int `json:"long_count"`
	GraphEntities int `json:"graph_entities"`
	GraphRelations int `json:"graph_relations"`
	EditableBlocks int `json:"editable_blocks"`
	Promotions    int `json:"promotions"`
}

func (o *Orchestrator) Stats(tenantID string) OrchestratorStats {
	s := OrchestratorStats{
		ShortCount: o.manager.Short.Count(tenantID),
		MidCount:   o.manager.Mid.Count(tenantID),
		LongCount:  o.manager.Long.Count(tenantID),
		Promotions: len(o.promotionLog),
	}
	if o.graph != nil {
		gs := o.graph.Stats()
		s.GraphEntities = gs["entities"]
		s.GraphRelations = gs["relations"]
	}
	if o.editable != nil {
		s.EditableBlocks = len(o.editable.AllBlocks())
	}
	return s
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
