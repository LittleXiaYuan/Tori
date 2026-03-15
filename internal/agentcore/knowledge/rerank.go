package knowledge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"time"
)

// ──────────────────────────────────────────────
// Reranker — optional second-stage ranking for RAG retrieval.
// After initial BM25 + vector + RRF fusion, a reranker can
// further refine relevance ordering using cross-encoder models.
// ──────────────────────────────────────────────

// Reranker reranks a list of documents given a query.
type Reranker interface {
	// Name returns the reranker provider identifier.
	Name() string
	// Rerank reorders documents by relevance to the query.
	// Returns reranked results with updated scores.
	Rerank(ctx context.Context, query string, documents []string, topK int) ([]RerankResult, error)
}

// RerankResult holds a reranked item with its new score.
type RerankResult struct {
	Index    int     `json:"index"`    // original document index
	Score    float64 `json:"score"`    // relevance score (0-1)
	Document string  `json:"document"` // original text
}

// ──────────────────────────────────────────────
// Jina Reranker — Jina AI Reranker API
// Models: jina-reranker-v2-base-multilingual, etc.
// ──────────────────────────────────────────────

// JinaReranker uses the Jina AI reranker API.
type JinaReranker struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

// JinaRerankerConfig configures the Jina reranker.
type JinaRerankerConfig struct {
	APIKey  string
	Model   string // default: "jina-reranker-v2-base-multilingual"
	BaseURL string // default: "https://api.jina.ai/v1"
}

// NewJinaReranker creates a Jina AI reranker.
func NewJinaReranker(cfg JinaRerankerConfig) *JinaReranker {
	model := cfg.Model
	if model == "" {
		model = "jina-reranker-v2-base-multilingual"
	}
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.jina.ai/v1"
	}
	return &JinaReranker{
		apiKey:  cfg.APIKey,
		model:   model,
		baseURL: baseURL,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (j *JinaReranker) Name() string { return "jina" }

func (j *JinaReranker) Rerank(ctx context.Context, query string, documents []string, topK int) ([]RerankResult, error) {
	if len(documents) == 0 {
		return nil, nil
	}
	if topK <= 0 || topK > len(documents) {
		topK = len(documents)
	}

	payload := jinaRerankRequest{
		Model:     j.model,
		Query:     query,
		Documents: documents,
		TopN:      topK,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("jina rerank marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, j.baseURL+"/rerank", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("jina rerank request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+j.apiKey)

	resp, err := j.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jina rerank: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("jina rerank: status %d: %s", resp.StatusCode, string(body))
	}

	var result jinaRerankResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("jina rerank parse: %w", err)
	}

	results := make([]RerankResult, len(result.Results))
	for i, r := range result.Results {
		doc := ""
		if r.Index < len(documents) {
			doc = documents[r.Index]
		}
		results[i] = RerankResult{
			Index:    r.Index,
			Score:    r.RelevanceScore,
			Document: doc,
		}
	}
	return results, nil
}

type jinaRerankRequest struct {
	Model     string   `json:"model"`
	Query     string   `json:"query"`
	Documents []string `json:"documents"`
	TopN      int      `json:"top_n"`
}

type jinaRerankResponse struct {
	Results []struct {
		Index          int     `json:"index"`
		RelevanceScore float64 `json:"relevance_score"`
	} `json:"results"`
}

// ──────────────────────────────────────────────
// Cohere Reranker — Cohere Rerank API
// Models: rerank-v3.5, rerank-multilingual-v3.0, etc.
// ──────────────────────────────────────────────

// CohereReranker uses the Cohere rerank API.
type CohereReranker struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

// CohereRerankerConfig configures the Cohere reranker.
type CohereRerankerConfig struct {
	APIKey  string
	Model   string // default: "rerank-v3.5"
	BaseURL string // default: "https://api.cohere.ai/v1"
}

// NewCohereReranker creates a Cohere reranker.
func NewCohereReranker(cfg CohereRerankerConfig) *CohereReranker {
	model := cfg.Model
	if model == "" {
		model = "rerank-v3.5"
	}
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.cohere.ai/v1"
	}
	return &CohereReranker{
		apiKey:  cfg.APIKey,
		model:   model,
		baseURL: baseURL,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *CohereReranker) Name() string { return "cohere" }

func (c *CohereReranker) Rerank(ctx context.Context, query string, documents []string, topK int) ([]RerankResult, error) {
	if len(documents) == 0 {
		return nil, nil
	}
	if topK <= 0 || topK > len(documents) {
		topK = len(documents)
	}

	payload := cohereRerankRequest{
		Model:           c.model,
		Query:           query,
		Documents:       documents,
		TopN:            topK,
		ReturnDocuments: false,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("cohere rerank marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/rerank", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("cohere rerank request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cohere rerank: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("cohere rerank: status %d: %s", resp.StatusCode, string(body))
	}

	var result cohereRerankResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("cohere rerank parse: %w", err)
	}

	results := make([]RerankResult, len(result.Results))
	for i, r := range result.Results {
		doc := ""
		if r.Index < len(documents) {
			doc = documents[r.Index]
		}
		results[i] = RerankResult{
			Index:    r.Index,
			Score:    r.RelevanceScore,
			Document: doc,
		}
	}
	return results, nil
}

type cohereRerankRequest struct {
	Model           string   `json:"model"`
	Query           string   `json:"query"`
	Documents       []string `json:"documents"`
	TopN            int      `json:"top_n"`
	ReturnDocuments bool     `json:"return_documents"`
}

type cohereRerankResponse struct {
	Results []struct {
		Index          int     `json:"index"`
		RelevanceScore float64 `json:"relevance_score"`
	} `json:"results"`
}

// ──────────────────────────────────────────────
// LLM Reranker — Uses LLM to score relevance (fallback/universal)
// No external rerank model needed — works with any LLM provider.
// ──────────────────────────────────────────────

// LLMReranker uses an LLM to evaluate document relevance.
type LLMReranker struct {
	llmCall func(ctx context.Context, system, user string) (string, error)
}

// NewLLMReranker creates a reranker that uses an LLM for relevance scoring.
func NewLLMReranker(llmCall func(ctx context.Context, system, user string) (string, error)) *LLMReranker {
	return &LLMReranker{llmCall: llmCall}
}

func (l *LLMReranker) Name() string { return "llm" }

func (l *LLMReranker) Rerank(ctx context.Context, query string, documents []string, topK int) ([]RerankResult, error) {
	if len(documents) == 0 || l.llmCall == nil {
		return nil, nil
	}
	if topK <= 0 || topK > len(documents) {
		topK = len(documents)
	}

	// For efficiency, batch documents in groups and ask LLM to rank them
	// Use a simple scoring prompt
	type scored struct {
		index int
		score float64
		doc   string
	}
	var all []scored

	for i, doc := range documents {
		// Truncate very long documents for LLM evaluation
		evalDoc := doc
		if len([]rune(evalDoc)) > 500 {
			evalDoc = string([]rune(evalDoc)[:500]) + "..."
		}

		system := "你是一个文档相关性评分器。请评估给定文档与查询的相关性，返回0到1之间的分数（只返回数字）。"
		user := fmt.Sprintf("查询: %s\n\n文档: %s\n\n相关性分数（0-1）:", query, evalDoc)

		resp, err := l.llmCall(ctx, system, user)
		if err != nil {
			slog.Debug("llm rerank: scoring failed", "index", i, "err", err)
			all = append(all, scored{index: i, score: 0, doc: doc})
			continue
		}

		var score float64
		if _, err := fmt.Sscanf(resp, "%f", &score); err != nil {
			// Try to extract number from response
			for _, ch := range resp {
				if (ch >= '0' && ch <= '9') || ch == '.' {
					fmt.Sscanf(string(ch), "%f", &score)
					break
				}
			}
		}
		if score < 0 {
			score = 0
		}
		if score > 1 {
			score = 1
		}
		all = append(all, scored{index: i, score: score, doc: doc})
	}

	// Sort by score descending
	sort.Slice(all, func(i, j int) bool {
		return all[i].score > all[j].score
	})

	if len(all) > topK {
		all = all[:topK]
	}

	results := make([]RerankResult, len(all))
	for i, s := range all {
		results[i] = RerankResult{
			Index:    s.index,
			Score:    s.score,
			Document: s.doc,
		}
	}
	return results, nil
}

// ──────────────────────────────────────────────
// Integration with Store — SetReranker + HybridSearchWithRerank
// ──────────────────────────────────────────────

// SetReranker attaches an optional reranker to the knowledge store.
func (s *Store) SetReranker(r Reranker) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reranker = r
}

// HybridSearchReranked performs HybridSearch then optionally reranks results.
func (s *Store) HybridSearchReranked(ctx context.Context, query string, limit int) []ScoredChunk {
	// First pass: get more candidates for reranking
	candidateLimit := limit * 3
	if candidateLimit < 15 {
		candidateLimit = 15
	}
	candidates := s.HybridSearch(ctx, query, candidateLimit)

	s.mu.RLock()
	reranker := s.reranker
	onRerank := s.onRerank
	s.mu.RUnlock()

	// If no reranker, return original results
	if reranker == nil || len(candidates) == 0 {
		if len(candidates) > limit {
			candidates = candidates[:limit]
		}
		return candidates
	}

	// Prepare documents for reranking
	docs := make([]string, len(candidates))
	for i, c := range candidates {
		docs[i] = c.Chunk.Content
	}

	// Rerank
	rerankStart := time.Now()
	reranked, err := reranker.Rerank(ctx, query, docs, limit)
	if onRerank != nil {
		onRerank(reranker.Name(), time.Since(rerankStart), err)
	}
	if err != nil {
		slog.Warn("rerank failed, using original order", "provider", reranker.Name(), "err", err)
		if len(candidates) > limit {
			candidates = candidates[:limit]
		}
		return candidates
	}

	// Map reranked results back to ScoredChunks
	results := make([]ScoredChunk, 0, len(reranked))
	for _, r := range reranked {
		if r.Index < len(candidates) {
			results = append(results, ScoredChunk{
				Chunk: candidates[r.Index].Chunk,
				Score: r.Score,
			})
		}
	}
	return results
}
