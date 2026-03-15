package knowledge

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ── Mock Reranker ──

type mockReranker struct {
	name string
}

func (m *mockReranker) Name() string { return m.name }

func (m *mockReranker) Rerank(_ context.Context, query string, documents []string, topK int) ([]RerankResult, error) {
	// Simple mock: reverse order and assign descending scores
	results := make([]RerankResult, 0, topK)
	for i := len(documents) - 1; i >= 0 && len(results) < topK; i-- {
		results = append(results, RerankResult{
			Index:    i,
			Score:    float64(len(documents)-i) / float64(len(documents)),
			Document: documents[i],
		})
	}
	return results, nil
}

// ── Reranker Interface Tests ──

func TestMockRerankerName(t *testing.T) {
	m := &mockReranker{name: "test"}
	if m.Name() != "test" {
		t.Errorf("name should be test, got %q", m.Name())
	}
}

func TestMockRerankerRerank(t *testing.T) {
	m := &mockReranker{name: "mock"}
	docs := []string{"first", "second", "third"}
	results, err := m.Rerank(context.Background(), "query", docs, 2)
	if err != nil {
		t.Fatalf("rerank failed: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	// Mock reverses, so first result should be last doc
	if results[0].Document != "third" {
		t.Errorf("first result should be 'third', got %q", results[0].Document)
	}
}

func TestRerankEmptyDocuments(t *testing.T) {
	m := &mockReranker{name: "mock"}
	results, err := m.Rerank(context.Background(), "query", nil, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("empty documents should return 0 results, got %d", len(results))
	}
}

// ── Jina Reranker Tests ──

func TestJinaRerankerName(t *testing.T) {
	j := NewJinaReranker(JinaRerankerConfig{APIKey: "test"})
	if j.Name() != "jina" {
		t.Errorf("name should be jina, got %q", j.Name())
	}
}

func TestJinaRerankerDefaults(t *testing.T) {
	j := NewJinaReranker(JinaRerankerConfig{APIKey: "key"})
	if j.model != "jina-reranker-v2-base-multilingual" {
		t.Errorf("default model mismatch: %s", j.model)
	}
	if j.baseURL != "https://api.jina.ai/v1" {
		t.Errorf("default baseURL mismatch: %s", j.baseURL)
	}
}

func TestJinaRerankerCustomConfig(t *testing.T) {
	j := NewJinaReranker(JinaRerankerConfig{
		APIKey:  "key",
		Model:   "custom-model",
		BaseURL: "https://custom.api",
	})
	if j.model != "custom-model" {
		t.Errorf("model should be custom-model, got %s", j.model)
	}
	if j.baseURL != "https://custom.api" {
		t.Errorf("baseURL should be https://custom.api, got %s", j.baseURL)
	}
}

func TestJinaRerankerWithMock(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rerank" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-key" {
			t.Errorf("auth should be 'Bearer test-key', got %q", auth)
		}

		var req jinaRerankRequest
		json.NewDecoder(r.Body).Decode(&req)

		if req.Model != "jina-reranker-v2-base-multilingual" {
			t.Errorf("model mismatch: %s", req.Model)
		}
		if req.Query != "AI" {
			t.Errorf("query should be 'AI', got %q", req.Query)
		}
		if len(req.Documents) != 3 {
			t.Errorf("expected 3 documents, got %d", len(req.Documents))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jinaRerankResponse{
			Results: []struct {
				Index          int     `json:"index"`
				RelevanceScore float64 `json:"relevance_score"`
			}{
				{Index: 1, RelevanceScore: 0.95},
				{Index: 0, RelevanceScore: 0.72},
			},
		})
	}))
	defer mock.Close()

	j := NewJinaReranker(JinaRerankerConfig{APIKey: "test-key", BaseURL: mock.URL})
	results, err := j.Rerank(context.Background(), "AI", []string{"doc1", "doc2", "doc3"}, 2)
	if err != nil {
		t.Fatalf("rerank failed: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Index != 1 {
		t.Errorf("first result index should be 1, got %d", results[0].Index)
	}
	if results[0].Score != 0.95 {
		t.Errorf("first result score should be 0.95, got %f", results[0].Score)
	}
	if results[0].Document != "doc2" {
		t.Errorf("first result document should be doc2, got %q", results[0].Document)
	}
}

func TestJinaRerankerAPIError(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"detail":"invalid api key"}`))
	}))
	defer mock.Close()

	j := NewJinaReranker(JinaRerankerConfig{APIKey: "bad", BaseURL: mock.URL})
	_, err := j.Rerank(context.Background(), "query", []string{"doc"}, 1)
	if err == nil {
		t.Fatal("expected error for 401")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error should contain 401: %v", err)
	}
}

func TestJinaRerankerEmptyDocs(t *testing.T) {
	j := NewJinaReranker(JinaRerankerConfig{APIKey: "key"})
	results, err := j.Rerank(context.Background(), "query", nil, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results != nil {
		t.Errorf("empty docs should return nil, got %v", results)
	}
}

// ── Cohere Reranker Tests ──

func TestCohereRerankerName(t *testing.T) {
	c := NewCohereReranker(CohereRerankerConfig{APIKey: "test"})
	if c.Name() != "cohere" {
		t.Errorf("name should be cohere, got %q", c.Name())
	}
}

func TestCohereRerankerDefaults(t *testing.T) {
	c := NewCohereReranker(CohereRerankerConfig{APIKey: "key"})
	if c.model != "rerank-v3.5" {
		t.Errorf("default model should be rerank-v3.5, got %s", c.model)
	}
	if c.baseURL != "https://api.cohere.ai/v1" {
		t.Errorf("default baseURL mismatch: %s", c.baseURL)
	}
}

func TestCohereRerankerWithMock(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rerank" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		var req cohereRerankRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.Model != "rerank-v3.5" {
			t.Errorf("model mismatch: %s", req.Model)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cohereRerankResponse{
			Results: []struct {
				Index          int     `json:"index"`
				RelevanceScore float64 `json:"relevance_score"`
			}{
				{Index: 2, RelevanceScore: 0.88},
				{Index: 0, RelevanceScore: 0.65},
				{Index: 1, RelevanceScore: 0.42},
			},
		})
	}))
	defer mock.Close()

	c := NewCohereReranker(CohereRerankerConfig{APIKey: "key", BaseURL: mock.URL})
	results, err := c.Rerank(context.Background(), "search", []string{"a", "b", "c"}, 3)
	if err != nil {
		t.Fatalf("rerank failed: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if results[0].Score != 0.88 {
		t.Errorf("first result score should be 0.88, got %f", results[0].Score)
	}
}

func TestCohereRerankerAPIError(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"message":"bad request"}`))
	}))
	defer mock.Close()

	c := NewCohereReranker(CohereRerankerConfig{APIKey: "key", BaseURL: mock.URL})
	_, err := c.Rerank(context.Background(), "q", []string{"doc"}, 1)
	if err == nil {
		t.Fatal("expected error")
	}
}

// ── LLM Reranker Tests ──

func TestLLMRerankerName(t *testing.T) {
	l := NewLLMReranker(nil)
	if l.Name() != "llm" {
		t.Errorf("name should be llm, got %q", l.Name())
	}
}

func TestLLMRerankerNilCall(t *testing.T) {
	l := NewLLMReranker(nil)
	results, err := l.Rerank(context.Background(), "query", []string{"doc"}, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results != nil {
		t.Errorf("nil llmCall should return nil results")
	}
}

func TestLLMRerankerWithMock(t *testing.T) {
	callCount := 0
	mockLLM := func(ctx context.Context, system, user string) (string, error) {
		callCount++
		// Return decreasing scores for document order
		scores := []string{"0.9", "0.3", "0.7"}
		if callCount <= len(scores) {
			return scores[callCount-1], nil
		}
		return "0.5", nil
	}

	l := NewLLMReranker(mockLLM)
	results, err := l.Rerank(context.Background(), "AI", []string{"AI doc", "unrelated", "semi-related"}, 2)
	if err != nil {
		t.Fatalf("rerank failed: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	// Should be sorted by score: 0.9 (index 0), 0.7 (index 2)
	if results[0].Score != 0.9 {
		t.Errorf("first result score should be 0.9, got %f", results[0].Score)
	}
	if results[1].Score != 0.7 {
		t.Errorf("second result score should be 0.7, got %f", results[1].Score)
	}
}

func TestLLMRerankerErrorHandling(t *testing.T) {
	mockLLM := func(ctx context.Context, system, user string) (string, error) {
		return "", fmt.Errorf("LLM error")
	}

	l := NewLLMReranker(mockLLM)
	results, err := l.Rerank(context.Background(), "query", []string{"doc1", "doc2"}, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should still return results with 0 scores
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for _, r := range results {
		if r.Score != 0 {
			t.Errorf("failed LLM should give score 0, got %f", r.Score)
		}
	}
}

// ── Store Integration Tests ──

func TestStoreSetReranker(t *testing.T) {
	store := NewStore(100)
	if store.reranker != nil {
		t.Error("reranker should be nil initially")
	}

	m := &mockReranker{name: "test"}
	store.SetReranker(m)
	if store.reranker == nil {
		t.Fatal("reranker should be set")
	}
	if store.reranker.Name() != "test" {
		t.Errorf("reranker name should be test, got %q", store.reranker.Name())
	}
}

func TestStoreHybridSearchRerankedWithoutReranker(t *testing.T) {
	store := NewStore(100)
	store.IngestText("test-src", "AI is great. Machine learning rocks. Deep learning is powerful.")

	// Without reranker, should behave like HybridSearch
	results := store.HybridSearchReranked(context.Background(), "AI", 3)
	// Should return some results from BM25
	if len(results) == 0 {
		t.Error("expected some results")
	}
}

func TestStoreHybridSearchRerankedWithReranker(t *testing.T) {
	store := NewStore(100)
	store.IngestText("src1", "AI and machine learning are transforming technology.")
	store.IngestText("src2", "Cooking recipes for beginners.")
	store.IngestText("src3", "Deep learning neural networks.")

	// Set mock reranker that reverses order
	store.SetReranker(&mockReranker{name: "mock"})

	results := store.HybridSearchReranked(context.Background(), "AI", 3)
	if len(results) == 0 {
		t.Error("expected reranked results")
	}
	// Results should have scores assigned by mock reranker
	for _, r := range results {
		if r.Score <= 0 {
			t.Errorf("reranked result should have positive score, got %f", r.Score)
		}
	}
}
