package knowledge

import (
	"context"
	"testing"
)

func TestTokenize_English(t *testing.T) {
	tokens := tokenize("Hello World, this is a test!")
	if len(tokens) == 0 {
		t.Fatal("expected tokens")
	}
	// "hello" and "world" should be present; "this", "is", "a" are stop words
	found := map[string]bool{}
	for _, tok := range tokens {
		found[tok] = true
	}
	if !found["hello"] {
		t.Error("missing 'hello'")
	}
	if !found["world"] {
		t.Error("missing 'world'")
	}
	if !found["test"] {
		t.Error("missing 'test'")
	}
}

func TestTokenize_Chinese(t *testing.T) {
	tokens := tokenize("知识库检索功能")
	if len(tokens) == 0 {
		t.Fatal("expected tokens")
	}
	// Should contain unigrams and bigrams
	found := map[string]bool{}
	for _, tok := range tokens {
		found[tok] = true
	}
	if !found["知"] {
		t.Error("missing unigram '知'")
	}
	if !found["知识"] {
		t.Error("missing bigram '知识'")
	}
	if !found["检索"] {
		t.Error("missing bigram '检索'")
	}
}

func TestBM25Index_Basic(t *testing.T) {
	chunks := []Chunk{
		{ID: "1", Content: "机器学习是人工智能的一个分支"},
		{ID: "2", Content: "深度学习是机器学习的子集"},
		{ID: "3", Content: "自然语言处理用于文本分析"},
		{ID: "4", Content: "Go语言是一种编程语言"},
		{ID: "5", Content: "数据库用于存储数据"},
	}

	idx := NewBM25Index(chunks)
	results := idx.Search("机器学习", 3)

	if len(results) == 0 {
		t.Fatal("expected results for '机器学习'")
	}
	// Top result should be one of ID "1" or "2"
	if results[0].ChunkID != "1" && results[0].ChunkID != "2" {
		t.Errorf("expected chunk 1 or 2 to rank highest, got %s", results[0].ChunkID)
	}
	if results[0].Score <= 0 {
		t.Error("score should be positive")
	}
}

func TestBM25Index_English(t *testing.T) {
	chunks := []Chunk{
		{ID: "1", Content: "Go programming language is efficient and concurrent"},
		{ID: "2", Content: "Python is popular for machine learning"},
		{ID: "3", Content: "JavaScript runs in the browser"},
	}

	idx := NewBM25Index(chunks)
	results := idx.Search("Go programming", 2)

	if len(results) == 0 {
		t.Fatal("expected results")
	}
	if results[0].ChunkID != "1" {
		t.Errorf("expected chunk 1, got %s", results[0].ChunkID)
	}
}

func TestBM25Index_NoMatch(t *testing.T) {
	chunks := []Chunk{
		{ID: "1", Content: "hello world"},
	}
	idx := NewBM25Index(chunks)
	results := idx.Search("quantum physics", 5)
	if len(results) != 0 {
		t.Errorf("expected no results, got %d", len(results))
	}
}

func TestBM25Index_Empty(t *testing.T) {
	idx := NewBM25Index(nil)
	results := idx.Search("test", 5)
	if len(results) != 0 {
		t.Error("expected no results from empty index")
	}
}

func TestFuseRRF_Basic(t *testing.T) {
	dense := []Chunk{
		{ID: "a", Content: "doc A"},
		{ID: "b", Content: "doc B"},
		{ID: "c", Content: "doc C"},
	}
	sparse := []Chunk{
		{ID: "b", Content: "doc B"},
		{ID: "d", Content: "doc D"},
		{ID: "a", Content: "doc A"},
	}

	results := FuseRRF(dense, sparse, 60, 5)
	if len(results) == 0 {
		t.Fatal("expected fused results")
	}

	// "b" and "a" appear in both lists, should rank higher
	topIDs := make(map[string]bool)
	for _, r := range results[:2] {
		topIDs[r.Chunk.ID] = true
	}
	if !topIDs["a"] && !topIDs["b"] {
		t.Error("expected 'a' or 'b' to be in top 2 (appear in both lists)")
	}

	// All results should have positive scores
	for _, r := range results {
		if r.Score <= 0 {
			t.Errorf("score should be positive, got %f for %s", r.Score, r.Chunk.ID)
		}
	}
}

func TestFuseRRF_DenseOnly(t *testing.T) {
	dense := []Chunk{
		{ID: "a", Content: "doc A"},
		{ID: "b", Content: "doc B"},
	}
	results := FuseRRF(dense, nil, 60, 5)
	if len(results) != 2 {
		t.Errorf("expected 2, got %d", len(results))
	}
}

func TestFuseRRF_SparseOnly(t *testing.T) {
	sparse := []Chunk{
		{ID: "x", Content: "doc X"},
	}
	results := FuseRRF(nil, sparse, 60, 5)
	if len(results) != 1 {
		t.Errorf("expected 1, got %d", len(results))
	}
}

func TestHybridSearch_Integration(t *testing.T) {
	store := NewStore(500)
	store.IngestText("ml", "机器学习是人工智能的一个重要分支，包括深度学习和强化学习。")
	store.IngestText("nlp", "自然语言处理是AI领域的核心技术，用于理解和生成人类语言。")
	store.IngestText("go", "Go语言是谷歌开发的静态类型编程语言，以并发性著称。")

	// Without embedder, HybridSearch should still work (BM25-only + keyword fallback)
	results := store.HybridSearch(context.Background(), "机器学习", 3)
	if len(results) == 0 {
		t.Fatal("expected results from hybrid search (BM25 path)")
	}

	// The ML-related chunk should rank high
	foundML := false
	for _, r := range results {
		if r.Chunk.SourceID != "" && (strContains(r.Chunk.Content, "机器学习") || strContains(r.Chunk.Content, "深度学习")) {
			foundML = true
			break
		}
	}
	if !foundML {
		t.Error("expected ML chunk in results")
	}
}

func strContains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && strContainsImpl(s, sub))
}

func strContainsImpl(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestStopWords(t *testing.T) {
	if !isStopWord("the") {
		t.Error("'the' should be a stop word")
	}
	if !isStopWord("的") {
		t.Error("'的' should be a stop word")
	}
	if isStopWord("knowledge") {
		t.Error("'knowledge' should not be a stop word")
	}
}
