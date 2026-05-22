package ledger

import (
	"context"
	"sort"
	"strings"
	"time"
)

// RecallEngine provides multi-stage task-aware memory retrieval.
// Unlike generic RAG (query -> embedding -> top-K), Ledger recall uses
// a 4-stage pipeline:
//
//	Stage 1: Metadata filter (kind, tenant, recency, expiry)
//	Stage 2: Graph traversal (find connected entities via ContextGraph)
//	Stage 3: Semantic retrieval (vector ANN search)
//	Stage 4: Multi-signal rerank (keyword + semantic + graph + goal + confidence + recency)
type RecallEngine struct {
	backend  Backend
	weights  ScoreWeights
	vector   *VectorIndex
	graph    *ContextGraph
	graphRAG *GraphRAG
	bm25     *BM25Index
}

// SetGraph attaches the context graph for Stage 2 traversal.
func (re *RecallEngine) SetGraph(g *ContextGraph) { re.graph = g }

// SetGraphRAG attaches GraphRAG community search for community-level recall.
func (re *RecallEngine) SetGraphRAG(gr *GraphRAG) { re.graphRAG = gr }

// GraphRAG returns the attached GraphRAG engine (nil if not set).
func (re *RecallEngine) GraphRAG() *GraphRAG { return re.graphRAG }

// SetBM25 attaches a BM25 index for hybrid keyword+semantic retrieval.
func (re *RecallEngine) SetBM25(idx *BM25Index) { re.bm25 = idx }

// BM25 returns the attached BM25 index (nil if not set).
func (re *RecallEngine) BM25() *BM25Index { return re.bm25 }

// Recall performs a multi-stage memory search.
func (re *RecallEngine) Recall(ctx context.Context, q RecallQuery) (*RecallResult, error) {
	start := time.Now()

	if q.Limit <= 0 {
		q.Limit = 10
	}
	if q.MinScore <= 0 {
		q.MinScore = 0.1
	}

	// ── Stage 0.5: Query expansion ──
	// Merge unique tokens from TaskGoal into the query. Used only for
	// BM25 keyword retrieval and embedding, NOT for the backend LIKE
	// search which needs the original compact query.
	expandedQuery := expandQuery(q.Query, q.TaskGoal)

	// ── Stage 1: Metadata filter + keyword candidates ──
	backendQuery := MemoryQuery{
		TenantID: q.TenantID,
		Query:    q.Query, // original query for backend LIKE search
		Kinds:    q.MemoryKinds,
		Limit:    q.Limit * 5,
	}
	if q.TaskID != "" {
		backendQuery.TaskID = &q.TaskID
	}
	candidates, err := re.backend.SearchMemories(ctx, backendQuery)
	if err != nil {
		return nil, err
	}

	if q.Recency != nil {
		cutoff := time.Now().Add(-*q.Recency)
		filtered := candidates[:0]
		for _, m := range candidates {
			if m.UpdatedAt.After(cutoff) {
				filtered = append(filtered, m)
			}
		}
		candidates = filtered
	}

	now := time.Now()
	active := candidates[:0]
	for _, m := range candidates {
		if m.ExpiresAt == nil || m.ExpiresAt.After(now) {
			active = append(active, m)
		}
	}
	candidates = active

	// Build candidate map for dedup
	candidateMap := make(map[string]*MemoryEntry, len(candidates))
	for _, m := range candidates {
		candidateMap[m.ID] = m
	}

	// ── Pre-compute query embedding once (shared by Stage 3 & 4) ──
	// Uses the expanded query for richer semantic coverage.
	var queryEmbed []float32
	if re.vector != nil && re.vector.Enabled() && expandedQuery != "" {
		queryEmbed, _ = re.vector.Embed(ctx, expandedQuery)
	}

	// ── Stage 2: Graph traversal (optional) ──
	if re.graph != nil && q.TaskID != "" {
		relatedIDs, _ := re.graph.FindRelatedMemoriesForTenant(ctx, q.TenantID, NodeTask, q.TaskID, q.Limit*2)
		for _, rid := range relatedIDs {
			if _, exists := candidateMap[rid]; !exists {
				m, err := re.backend.GetMemory(ctx, rid)
				if err == nil && m != nil {
					candidateMap[m.ID] = m
				}
			}
		}
	}

	// ── Stage 2.5: BM25 keyword retrieval (optional) ──
	// Uses expandedQuery to capture goal-related keywords.
	var bm25Hits []BM25Result
	if re.bm25 != nil && expandedQuery != "" {
		bm25Hits = re.bm25.Search(expandedQuery, q.Limit*3)
		for _, hit := range bm25Hits {
			if _, exists := candidateMap[hit.DocID]; !exists {
				m, err := re.backend.GetMemory(ctx, hit.DocID)
				if err == nil && m != nil {
					candidateMap[m.ID] = m
				}
			}
		}
	}

	// ── Stage 2.75: GraphRAG community retrieval (optional) ──
	// Community search bridges from query terms to memory nodes through topic /
	// entity communities, so recall can surface memories that do not directly
	// contain the query text but belong to a relevant graph cluster.
	communityHitMap := make(map[string]bool)
	if re.graphRAG != nil && expandedQuery != "" {
		communities := re.graphRAG.SearchByCommunity(expandedQuery, q.Limit*2)
		for _, comm := range communities {
			for _, node := range comm.Nodes {
				if node.Kind != NodeMemory {
					continue
				}
				memID := node.RefID
				if memID == "" {
					memID = node.ID
				}
				if memID == "" {
					continue
				}
				if q.TenantID != "" && node.TenantID != "" && node.TenantID != q.TenantID {
					continue
				}
				m, err := re.backend.GetMemory(ctx, memID)
				if err != nil || m == nil || !recallMemoryAllowed(m, &q, now) {
					continue
				}
				communityHitMap[m.ID] = true
				if _, exists := candidateMap[m.ID]; !exists {
					candidateMap[m.ID] = m
				}
			}
		}
	}

	// ── Stage 3: Semantic retrieval (optional) ──
	if re.vector != nil && re.vector.Enabled() && len(queryEmbed) > 0 {
		vecResults, err := re.vector.Search(ctx, VectorQuery{
			TenantID:  q.TenantID,
			Embedding: queryEmbed,
			Kinds:     q.MemoryKinds,
			Limit:     q.Limit * 3,
			MinScore:  0.3,
		})
		if err == nil {
			for _, sr := range vecResults {
				if _, exists := candidateMap[sr.Entry.ID]; !exists {
					entryCopy := sr.Entry
					entryCopy.Embedding = nil // don't carry embeddings in results
					candidateMap[sr.Entry.ID] = &entryCopy
				}
			}
		}
	}

	// ── Stage 4: Multi-signal rerank ──
	//
	// Independent signals are fused additively (not serially) so each
	// channel retains its full dynamic range regardless of which combination
	// of retrieval backends is active:
	//
	//   signal          | weight when all 3 active | solo
	//   ────────────────┼──────────────────────────┼──────
	//   scoreEntry (7D) | 0.40                     | 1.00
	//   semantic cosine | 0.35                     | 0.40
	//   BM25 keyword    | 0.25                     | 0.30
	//
	// This replaces the previous serial-override scheme that compressed
	// the 7-dimensional weight system to ~42% of its intended contribution.

	bm25ScoreMap := make(map[string]float64, len(bm25Hits))
	for _, hit := range bm25Hits {
		bm25ScoreMap[hit.DocID] = hit.Score
	}

	var scored []ScoredEntry
	for _, m := range candidateMap {
		baseScore, reason := scoreEntry(m, &q, re.weights)

		hasSemantic := len(queryEmbed) > 0 && len(m.Embedding) > 0
		_, hasBM25 := bm25ScoreMap[m.ID]
		_, hasCommunity := communityHitMap[m.ID]

		var semanticSim float64
		if hasSemantic {
			semanticSim = CosineSimilarity(queryEmbed, m.Embedding)
		}

		var normalizedBM25 float64
		if hasBM25 {
			raw := bm25ScoreMap[m.ID]
			normalizedBM25 = raw / (raw + 5.0) // saturate at ~0.5
		}

		// Additive fusion with dynamic channel weights
		var score float64
		switch {
		case hasSemantic && hasBM25:
			score = 0.40*baseScore + 0.35*semanticSim + 0.25*normalizedBM25
		case hasSemantic:
			score = 0.60*baseScore + 0.40*semanticSim
		case hasBM25:
			score = 0.70*baseScore + 0.30*normalizedBM25
		default:
			score = baseScore
		}

		if hasSemantic && semanticSim > 0.7 {
			if reason != "" {
				reason += ", "
			}
			reason += "semantic match"
		}
		if hasBM25 {
			if reason != "" {
				reason += ", "
			}
			reason += "keyword match"
		}
		if hasCommunity {
			score += 0.12
			if score > 1.0 {
				score = 1.0
			}
			if reason != "" {
				reason += ", "
			}
			reason += "community match"
		}

		if score >= q.MinScore {
			scored = append(scored, ScoredEntry{
				Entry:  *m,
				Score:  score,
				Reason: reason,
			})
		}
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})

	if len(scored) > q.Limit {
		scored = scored[:q.Limit]
	}

	// Clear embeddings from results to save bandwidth
	for i := range scored {
		scored[i].Entry.Embedding = nil
	}

	// Load related artifacts
	var artifacts []Artifact
	taskIDs := make(map[string]bool)
	for _, s := range scored {
		if s.Entry.TaskID != nil {
			taskIDs[*s.Entry.TaskID] = true
		}
	}
	for tid := range taskIDs {
		arts, err := re.backend.ListArtifacts(ctx, tid)
		if err == nil {
			for _, a := range arts {
				artifacts = append(artifacts, *a)
			}
		}
	}

	return &RecallResult{
		Entries:     scored,
		Artifacts:   artifacts,
		TotalFound:  len(scored),
		QueryTimeMs: time.Since(start).Milliseconds(),
	}, nil
}

// RecallForTask is a convenience method that fills in task context automatically.
func (re *RecallEngine) RecallForTask(ctx context.Context, task *Task, query string, limit int) (*RecallResult, error) {
	return re.Recall(ctx, RecallQuery{
		TenantID: task.TenantID,
		TaskID:   task.ID,
		Query:    query,
		TaskGoal: task.Goal,
		TaskType: task.Type,
		Limit:    limit,
	})
}

// expandQuery merges unique tokens from goalText into query to
// broaden candidate retrieval without LLM involvement. Keeps the
// expanded query compact (≤ 200 chars) to avoid diluting precision.
func expandQuery(query, goalText string) string {
	if goalText == "" || query == "" {
		if query == "" {
			return goalText
		}
		return query
	}

	queryLower := strings.ToLower(query)
	goalTokens := tokenize(goalText)

	var extras []string
	for _, tok := range goalTokens {
		if len(tok) < 2 {
			continue
		}
		if !strings.Contains(queryLower, tok) {
			extras = append(extras, tok)
		}
	}

	if len(extras) == 0 {
		return query
	}
	// Cap expansion to avoid precision dilution
	if len(extras) > 5 {
		extras = extras[:5]
	}
	expanded := query + " " + strings.Join(extras, " ")
	if len(expanded) > 200 {
		expanded = expanded[:200]
	}
	return expanded
}

func recallMemoryAllowed(m *MemoryEntry, q *RecallQuery, now time.Time) bool {
	if m == nil {
		return false
	}
	if q.TenantID != "" && m.TenantID != q.TenantID {
		return false
	}
	if len(q.MemoryKinds) > 0 {
		matched := false
		for _, kind := range q.MemoryKinds {
			if m.Kind == kind {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if q.Recency != nil && !m.UpdatedAt.After(now.Add(-*q.Recency)) {
		return false
	}
	if m.ExpiresAt != nil && !m.ExpiresAt.After(now) {
		return false
	}
	return true
}
