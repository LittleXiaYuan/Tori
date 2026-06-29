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

	"yunque-agent/pkg/safego"
)

// MemoryOrchestrator — five-layer memory integration.
// Connects: ShortTerm → MidTerm → LongTerm → Graph → Editable
// Handles unified recall, auto-promotion, importance scoring, and time-decay.

// Importance levels for promotion decisions.
type Importance int

const (
	ImportanceLow    Importance = 1
	ImportanceMedium Importance = 2
	ImportanceHigh   Importance = 3
)

type RecallItem struct {
	Content     string        `json:"content"`
	Source      string        `json:"source"` // "short", "mid", "long", "graph", "editable", "observation"
	Category    string        `json:"category,omitempty"`
	Score       float64       `json:"score"`     // final weighted score
	RawScore    float64       `json:"raw_score"` // score before weighting
	Importance  Importance    `json:"importance"`
	Age         time.Duration `json:"age"`
	AccessCount int           `json:"access_count,omitempty"`
}

// ImportanceFunc scores a piece of content. Can be LLM-backed or pure heuristic.
type ImportanceFunc func(ctx context.Context, content string) Importance

type OrchestratorConfig struct {
	// Layer weights for unified recall (0.0 - 1.0)
	ShortWeight       float64 `json:"short_weight"`       // default: 0.5
	MidWeight         float64 `json:"mid_weight"`         // default: 0.8
	LongWeight        float64 `json:"long_weight"`        // default: 1.0
	GraphWeight       float64 `json:"graph_weight"`       // default: 0.9
	EditableWeight    float64 `json:"editable_weight"`    // default: 0.95
	ObservationWeight float64 `json:"observation_weight"` // default: 0.7

	// Promotion thresholds
	ShortToMidAccessCount int           `json:"short_to_mid_access"` // promote after N accesses
	MidToLongAccessCount  int           `json:"mid_to_long_access"`  // promote after N accesses
	ShortToMidAge         time.Duration `json:"short_to_mid_age"`    // promote items older than this
	DecayHalfLife         time.Duration `json:"decay_half_life"`     // score halves after this duration
	MaxRecallResults      int           `json:"max_recall_results"`
	// MinRecallScore drops recalled memories whose final score is below this
	// value before they are injected into the prompt. Keeps irrelevant "filler"
	// memories out of the context (saves tokens, avoids pollution) when a query
	// has no good match. 0 = disabled (inject all top-K). Tune per embedder via
	// RECALL_MIN_SCORE; only applied in CompileContext, not raw Recall.
	MinRecallScore float64 `json:"min_recall_score"`
}

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

// TFIDFImportanceScorer computes information density scores for content.
// When attached, heuristicImportance uses it as a secondary signal to
// reduce false negatives (important content missed by keyword matching).
type TFIDFImportanceScorer interface {
	Score(content string) float64
	AddDocument(content string)
}

// Orchestrator ties the five memory layers together.
type Orchestrator struct {
	mu               sync.RWMutex
	config           OrchestratorConfig
	manager          *Manager        // short + mid + long
	graph            *Graph          // knowledge graph
	editable         *EditableMemory // agent-editable blocks
	importanceFn     ImportanceFunc
	conflictDetector *ConflictDetector // optional: detects memory contradictions
	tfidfScorer      TFIDFImportanceScorer // optional: TF-IDF importance signal
	promotionLog     []promotionEntry
	conflictLog      []Conflict // recent conflicts detected
	lastPromote      time.Time  // throttle auto-promotion
	ingestCount      int        // count ingests since last promotion
	onPromote        func()     // optional callback when promotion runs
}

// SetOnPromote sets a callback invoked each time memory promotion runs.
func (o *Orchestrator) SetOnPromote(fn func()) { o.onPromote = fn }

type promotionEntry struct {
	From      string    `json:"from"`
	To        string    `json:"to"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

func NewOrchestrator(cfg OrchestratorConfig, mgr *Manager, g *Graph, em *EditableMemory) *Orchestrator {
	return &Orchestrator{
		config:   cfg,
		manager:  mgr,
		graph:    g,
		editable: em,
	}
}

// Editable returns the editable memory instances (Persona, etc).
func (o *Orchestrator) Editable() *EditableMemory {
	return o.editable
}

func (o *Orchestrator) SetImportanceFunc(fn ImportanceFunc) {
	o.importanceFn = fn
}

func (o *Orchestrator) SetConflictDetector(cd *ConflictDetector) {
	o.conflictDetector = cd
}

// SetTFIDFScorer attaches a TF-IDF scorer for information density-aware
// importance evaluation. When set, heuristicImportance uses TF-IDF scores
// as a secondary signal to promote content with rare/specific terms to
// ImportanceMedium even when no keyword match is found.
func (o *Orchestrator) SetTFIDFScorer(scorer TFIDFImportanceScorer) {
	o.tfidfScorer = scorer
}

func (o *Orchestrator) Conflicts() []Conflict {
	o.mu.RLock()
	defer o.mu.RUnlock()
	out := make([]Conflict, len(o.conflictLog))
	copy(out, o.conflictLog)
	return out
}

// ---- unified recall ----

// Recall searches all memory layers in parallel and returns merged results,
// ranked by (layerWeight * rawScore * timeDecay).
//
// Uses channel-based aggregation instead of a shared Mutex to eliminate lock
// contention between layer goroutines. Each goroutine builds its local result
// slice independently and sends it through a channel; the caller collects and
// merges without any shared mutable state.
func (o *Orchestrator) Recall(ctx context.Context, tenantID, query string, limit int) []RecallItem {
	if limit <= 0 {
		limit = o.config.MaxRecallResults
	}
	perLayer := limit * 2

	layerCount := 1 // Manager (short/mid/long) always runs
	if o.graph != nil {
		layerCount++
	}
	if o.editable != nil {
		layerCount++
	}

	resultCh := make(chan []RecallItem, layerCount)

	// 1. Short/Mid/Long via Manager
	safego.Go("memory-recall-layers", func() {
		items, err := o.manager.SearchAll(ctx, tenantID, query, perLayer)
		if err != nil {
			slog.Warn("memory recall: layer search failed", "err", err)
			resultCh <- nil
			return
		}
		local := make([]RecallItem, 0, len(items))
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
			ri.Score *= o.decayFactorForCategory(ri.Age, item.AccessCnt, item.Category)
			local = append(local, ri)
		}
		resultCh <- local
	})

	// 2. Knowledge Graph — builds results outside any shared lock
	if o.graph != nil {
		safego.Go("memory-recall-graph", func() {
			entities := o.graph.SearchEntitiesForTenant(tenantID, query, perLayer)
			local := make([]RecallItem, 0, len(entities))
			for _, e := range entities {
				ctx := o.graph.ContextFor(e.ID)
				ri := RecallItem{
					Content:  ctx,
					Source:   "graph",
					Category: e.Type,
					RawScore: float64(e.Mentions) * 0.1,
					Age:      time.Since(e.CreatedAt),
				}
				ri.Score = ri.RawScore * o.config.GraphWeight * o.decayFactor(ri.Age)
				local = append(local, ri)
			}
			resultCh <- local
		})
	}

	// 3. Editable Memory blocks
	if o.editable != nil {
		safego.Go("memory-recall-editable", func() {
			blocks := o.editable.BlocksForTenant(tenantID)
			queryLower := strings.ToLower(query)
			var local []RecallItem
			for _, b := range blocks {
				if query == "" || strings.Contains(strings.ToLower(b.Content), queryLower) ||
					strings.Contains(strings.ToLower(b.Label), queryLower) {
					ri := RecallItem{
						Content:  fmt.Sprintf("[%s] %s", b.Label, b.Content),
						Source:   "editable",
						Category: b.Label,
						RawScore: 1.0,
						Age:      time.Since(b.UpdatedAt),
					}
					ri.Score = ri.RawScore * o.config.EditableWeight
					local = append(local, ri)
				}
			}
			resultCh <- local
		})
	}

	// Collect results from all layers — no shared mutable state
	var allResults []RecallItem
	for i := 0; i < layerCount; i++ {
		if batch := <-resultCh; len(batch) > 0 {
			allResults = append(allResults, batch...)
		}
	}

	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].Score > allResults[j].Score
	})

	if len(allResults) > limit {
		allResults = allResults[:limit]
	}

	slog.Debug("orchestrator: recall", "query", query, "results", len(allResults))
	return allResults
}

// ---- ingest ----

// Ingest routes content to the right layer based on importance.
// High-importance facts get written to both mid and long.
//
// Conflict detection roadmap (TECH-DEBT-2026-04-18.md item #11):
//
//   1. [DONE] `conflict_embedding.go` implements the embedding + cosine
//      similarity gate. `ConflictDetector.SetEmbeddingGate(embed, cfg)`
//      pre-filters the `existing` recall set by cosine ≥ 0.82 before the
//      LLM / heuristic arbiter runs, killing the false positives the
//      keyword path produces on shared Chinese negation words.
//   2. [DONE] Graceful degradation — transient embedder failures fall
//      through to the keyword / LLM path; per-item embed errors drop
//      only that item, never the whole call.
//   3. [DONE] Wiring lives in cmd/agent/init_tasks.go right after
//      `gw.SetEmbeddings(embedRes)`: when an embedder is configured the
//      gate is enabled automatically at the default threshold; when
//      EMBED_BASE_URL is unset we fall back to the LLM + keyword path,
//      which is still strictly better than the pre-wiring "nil detector"
//      state where the orchestrator never even ran arbitration.
//   4. [DONE] Per-tenant top-K cache: `conflict_embedding.go` now caches
//      stored-memory embeddings per tenant (`tenantVecCache`) with
//      configurable TTL and max-items cap, avoiding O(existing) embed
//      calls on every Ingest. `DetectConflictsForTenant` and
//      `filterByEmbeddingForTenant` propagate tenantID through the gate.
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

	var err error
	switch importance {
	case ImportanceLow:
		err = o.manager.Short.Put(ctx, tenantID, item)
	case ImportanceMedium:
		err = o.manager.AddMid(ctx, tenantID, item)
	case ImportanceHigh:
		if err = o.manager.AddMid(ctx, tenantID, item); err != nil {
			return err
		}
		err = o.manager.AddLong(ctx, tenantID, item)
	default:
		err = o.manager.Short.Put(ctx, tenantID, item)
	}
	if err != nil {
		return err
	}

	// Auto-promote: every 20 ingests or every 5 minutes
	o.mu.Lock()
	o.ingestCount++
	shouldPromote := o.ingestCount >= 20 || time.Since(o.lastPromote) > 5*time.Minute
	if shouldPromote {
		o.ingestCount = 0
		o.lastPromote = time.Now()
	}
	o.mu.Unlock()
	if shouldPromote {
		if o.onPromote != nil {
			o.onPromote()
		}
		safego.Go("memory-promote-async", func() {
			promoteCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			n := o.Promote(promoteCtx, tenantID)
			if n > 0 {
				slog.Info("memory auto-promote completed", "promoted", n, "tenant", tenantID)
			}
		})
	}

	return nil
}

// detectAndResolveConflicts runs async after Ingest — finds contradictions
// between new content and existing memories, then handles resolution.
func (o *Orchestrator) detectAndResolveConflicts(ctx context.Context, tenantID, newContent string) {
	// Recall existing memories related to the new content
	existing := o.Recall(ctx, tenantID, newContent, 10)
	if len(existing) == 0 {
		return
	}

	conflicts := o.conflictDetector.DetectConflictsForTenant(ctx, tenantID, newContent, existing)
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

// ---- auto-promotion ----

// Promote moves qualifying items up the memory hierarchy.
// Short→Mid after N accesses or age, Mid→Long after many accesses.
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

// ---- graph bridge ----

// LinkEntityToMemory wires a graph entity to a memory item.
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

// RecallForEntity gets everything we know about a named entity — graph context + memory matches.
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

// ---- context compilation ----

// CompileContext builds the memory block for the system prompt.
// Includes editable memory + recalled items relevant to currentQuery.
func (o *Orchestrator) CompileContext(ctx context.Context, tenantID, currentQuery string) string {
	var sb strings.Builder

	// 1. Editable memory blocks — global blocks (empty TenantID, e.g. persona)
	// plus this tenant's own blocks. Per-tenant scoping happens in CompileForTenant.
	if o.editable != nil {
		compiled := o.editable.CompileForTenant(tenantID)
		if compiled != "" {
			sb.WriteString(compiled)
		}
	}

	// 2. Relevant recalled memories — union the active tenant with the shared
	// "system" namespace so background/global writes (e.g. reverie insights)
	// surface regardless of which tenant is asking. Graph + editable recall
	// items are already tenant-scoped inside Recall.
	if currentQuery != "" {
		items := o.Recall(ctx, tenantID, currentQuery, 10)
		if tenantID != "" && tenantID != "system" {
			items = mergeRecallItems(items, o.Recall(ctx, "system", currentQuery, 10), 10)
		}
		// Drop low-relevance "filler" memories before injection: when a query has
		// no good match, top-K still returns the least-irrelevant items; injecting
		// them wastes tokens and pollutes context. Skip in raw Recall so debug/eval
		// still see the full ranking.
		if min := o.config.MinRecallScore; min > 0 {
			kept := items[:0]
			for _, item := range items {
				if item.Score >= min {
					kept = append(kept, item)
				}
			}
			items = kept
		}
		if len(items) > 0 {
			sb.WriteString("<recalled_memories>\n")
			for _, item := range items {
			sb.WriteString(fmt.Sprintf("- [%s][%s] %s\n", recallTierLabel(item.Importance), item.Source, item.Content))
			}
			sb.WriteString("</recalled_memories>\n\n")
		}
	}

	return sb.String()
}

// recallTierLabel maps Importance to a human-readable tier prefix so the LLM
// can prioritize recalled memories by significance in the prompt.
func recallTierLabel(imp Importance) string {
	switch imp {
	case ImportanceHigh:
		return "核心"
	case ImportanceMedium:
		return "项目"
	default:
		return "情境"
	}
}

// mergeRecallItems combines two recall slices, de-duplicating by content,
// re-sorting by score, and capping to limit. Used to union a tenant's memories
// with the shared "system" namespace so background/global memories surface.
func mergeRecallItems(a, b []RecallItem, limit int) []RecallItem {
	if len(b) == 0 {
		return a
	}
	all := make([]RecallItem, 0, len(a)+len(b))
	all = append(all, a...)
	all = append(all, b...)

	seen := make(map[string]bool, len(all))
	merged := make([]RecallItem, 0, len(all))
	for _, it := range all {
		if it.Content != "" && seen[it.Content] {
			continue
		}
		seen[it.Content] = true
		merged = append(merged, it)
	}
	sort.Slice(merged, func(i, j int) bool { return merged[i].Score > merged[j].Score })
	if limit > 0 && len(merged) > limit {
		merged = merged[:limit]
	}
	return merged
}

// ---- FSRS (Free Spaced Repetition Scheduler) memory decay ----
//
// FSRS is empirically validated on millions of Anki review logs and
// outperforms SM-2 by ~30% in retention accuracy (Ye, Su & Cao, 2022).
//
// Core model: R(t) = e^(-t/S)
// Stability update after successful recall:
//   S' = S × (1 + e^w1 × D^(-w2) × (e^(w3×(1-R)) - 1))
//
// where D is difficulty (1.0-10.0), R is retrievability at recall time,
// and w1/w2/w3 are learned parameters from the FSRS research.
//
// Reference: https://github.com/open-spaced-repetition/fsrs4anki

// fsrsParams are learned from the FSRS research on Anki review logs.
var fsrsParams = struct {
	W1 float64 // controls base stability growth rate
	W2 float64 // difficulty sensitivity exponent
	W3 float64 // retrievability bonus for "hard" recalls (low R at review time)
}{
	W1: 0.9,
	W2: 0.5,
	W3: 2.0,
}

func (o *Orchestrator) decayFactor(age time.Duration) float64 {
	return o.decayFactorForCategory(age, 0, "")
}

func (o *Orchestrator) decayFactorWithAccess(age time.Duration, accessCount int) float64 {
	return o.decayFactorForCategory(age, accessCount, "")
}

// categoryDifficulty maps a memory category to an FSRS difficulty parameter.
// Lower difficulty = slower decay (memory persists longer).
//
//   identity  → 1.0  (names, IDs — near-permanent)
//   preference→ 3.0  (likes, dislikes — slow decay)
//   knowledge → 5.0  (general facts — default rate)
//   fact      → 5.0  (extracted facts — default rate)
//   chat      → 8.0  (casual conversation — fast decay)
//
// Unknown or empty categories fall back to 5.0 for backward compatibility.
func categoryDifficulty(category string) float64 {
	switch strings.ToLower(category) {
	case "identity", "name", "persona":
		return 1.0
	case "preference", "preference_like", "preference_dislike":
		return 3.0
	case "knowledge", "fact", "experience":
		return 5.0
	case "chat", "casual", "greeting":
		return 8.0
	default:
		return 5.0
	}
}

// decayFactorForCategory computes retention using FSRS stability scheduling
// with category-adaptive difficulty. Each access simulates a successful review,
// growing stability via the FSRS update formula:
//
//	S' = S × (1 + e^w1 × D^(-w2) × (e^(w3×(1-R)) - 1))
//
// Identity memories (D=1.0) decay ~8x slower than chat (D=8.0).
func (o *Orchestrator) decayFactorForCategory(age time.Duration, accessCount int, category string) float64 {
	if o.config.DecayHalfLife <= 0 {
		return 1.0
	}

	stability := float64(o.config.DecayHalfLife)
	difficulty := categoryDifficulty(category)

	for i := 0; i < accessCount; i++ {
		elapsedFrac := float64(i+1) / float64(accessCount+1)
		t := float64(age) * elapsedFrac
		r := math.Exp(-t / stability)
		if r < 0.01 {
			r = 0.01
		}

		growth := 1.0 + math.Exp(fsrsParams.W1)*
			math.Pow(difficulty, -fsrsParams.W2)*
			(math.Exp(fsrsParams.W3*(1.0-r)) - 1.0)

		if growth < 1.0 {
			growth = 1.0
		}
		if growth > 10.0 {
			growth = 10.0
		}
		stability *= growth
	}

	return math.Exp(-float64(age) / stability)
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

	result := heuristicImportance(content)

	// TF-IDF uplift: if keyword heuristic says Low but TF-IDF shows
	// high information density (rare/specific terms), promote to Medium.
	if result == ImportanceLow && o.tfidfScorer != nil {
		tfidfScore := o.tfidfScorer.Score(content)
		if tfidfScore >= 0.6 {
			result = ImportanceMedium
		}
	}

	// Feed content into TF-IDF corpus for future scoring
	if o.tfidfScorer != nil {
		o.tfidfScorer.AddDocument(content)
	}

	return result
}

// heuristicImportance: keyword-based fallback when no LLM evaluator is set.
// NB: these keyword lists are intentionally broad — false positives are ok,
// missed important facts are not.
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

type OrchestratorStats struct {
	ShortCount     int `json:"short_count"`
	MidCount       int `json:"mid_count"`
	LongCount      int `json:"long_count"`
	GraphEntities  int `json:"graph_entities"`
	GraphRelations int `json:"graph_relations"`
	EditableBlocks int `json:"editable_blocks"`
	Promotions     int `json:"promotions"`
}

func (o *Orchestrator) Stats(tenantID string) OrchestratorStats {
	o.mu.RLock()
	promoCount := len(o.promotionLog)
	o.mu.RUnlock()
	s := OrchestratorStats{
		ShortCount: o.manager.Short.Count(tenantID),
		MidCount:   o.manager.Mid.Count(tenantID),
		LongCount:  o.manager.Long.Count(tenantID),
		Promotions: promoCount,
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
