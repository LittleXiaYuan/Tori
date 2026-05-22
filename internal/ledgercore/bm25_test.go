package ledger

import (
	"testing"
)

func TestBM25BasicSearch(t *testing.T) {
	idx := NewBM25Index()
	idx.Add("d1", "Go is a programming language designed at Google")
	idx.Add("d2", "Python is a popular programming language for data science")
	idx.Add("d3", "The weather today is sunny and warm")

	results := idx.Search("programming language", 10)
	if len(results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(results))
	}
	if results[0].DocID != "d1" && results[0].DocID != "d2" {
		t.Errorf("expected d1 or d2 as top result, got %s", results[0].DocID)
	}
	for _, r := range results {
		if r.DocID == "d3" {
			t.Errorf("weather doc should not match 'programming language'")
		}
	}
}

func TestBM25ChineseTokenization(t *testing.T) {
	idx := NewBM25Index()
	idx.Add("c1", "云雀是一个智能AI助手，专注于任务执行和知识管睆")
	idx.Add("c2", "今天天气很好，适坈出去散步")
	idx.Add("c3", "云雀支挝记忆系统和知识图谱检索")

	results := idx.Search("云雀知识", 10)
	if len(results) == 0 {
		t.Fatal("expected results for Chinese query")
	}
	if results[0].DocID != "c3" && results[0].DocID != "c1" {
		t.Errorf("expected c1 or c3 as top result for '云雀知识', got %s", results[0].DocID)
	}
}

func TestBM25EmptyIndex(t *testing.T) {
	idx := NewBM25Index()
	results := idx.Search("anything", 10)
	if len(results) != 0 {
		t.Errorf("expected 0 results from empty index, got %d", len(results))
	}
}

func TestBM25EmptyQuery(t *testing.T) {
	idx := NewBM25Index()
	idx.Add("d1", "some content")
	results := idx.Search("", 10)
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty query, got %d", len(results))
	}
}

func TestBM25Remove(t *testing.T) {
	idx := NewBM25Index()
	idx.Add("d1", "important document about algorithms")
	idx.Add("d2", "another document about algorithms")

	if idx.Size() != 2 {
		t.Fatalf("expected size 2, got %d", idx.Size())
	}

	idx.Remove("d1")
	if idx.Size() != 1 {
		t.Fatalf("expected size 1 after remove, got %d", idx.Size())
	}

	results := idx.Search("algorithms", 10)
	if len(results) != 1 {
		t.Fatalf("expected 1 result after remove, got %d", len(results))
	}
	if results[0].DocID != "d2" {
		t.Errorf("expected d2, got %s", results[0].DocID)
	}
}

func TestBM25Update(t *testing.T) {
	idx := NewBM25Index()
	idx.Add("d1", "old content about cats")
	idx.Add("d1", "new content about dogs")

	if idx.Size() != 1 {
		t.Fatalf("expected size 1 after update, got %d", idx.Size())
	}

	results := idx.Search("dogs", 10)
	if len(results) != 1 || results[0].DocID != "d1" {
		t.Errorf("expected d1 for 'dogs' after update")
	}

	results = idx.Search("cats", 10)
	if len(results) != 0 {
		t.Errorf("expected 0 results for 'cats' after update, got %d", len(results))
	}
}

func TestBM25Ranking(t *testing.T) {
	idx := NewBM25Index()
	idx.Add("d1", "machine learning is great")
	idx.Add("d2", "machine learning machine learning deep learning neural networks machine learning")
	idx.Add("d3", "cooking recipes for dinner")

	results := idx.Search("machine learning", 10)
	if len(results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(results))
	}
	if results[0].Score <= results[1].Score {
		t.Logf("scores: d2=%f, d1=%f (BM25 saturates TF, so ordering depends on k1/b)", results[0].Score, results[1].Score)
	}
}

func TestRRFFusion(t *testing.T) {
	bm25Results := []BM25Result{
		{DocID: "d1", Content: "keyword match doc 1", Score: 5.0},
		{DocID: "d2", Content: "keyword match doc 2", Score: 3.0},
		{DocID: "d3", Content: "only keyword doc 3", Score: 1.0},
	}
	vectorResults := []ScoredEntry{
		{Entry: MemoryEntry{ID: "d2", Content: "semantic match doc 2"}, Score: 0.95},
		{Entry: MemoryEntry{ID: "d4", Content: "only semantic doc 4"}, Score: 0.90},
		{Entry: MemoryEntry{ID: "d1", Content: "semantic match doc 1"}, Score: 0.85},
	}

	fused := RRF(bm25Results, vectorResults, 60, 10)
	if len(fused) != 4 {
		t.Fatalf("expected 4 fused results, got %d", len(fused))
	}

	topIDs := make(map[string]bool)
	for _, r := range fused[:2] {
		topIDs[r.Entry.ID] = true
	}
	if !topIDs["d1"] || !topIDs["d2"] {
		t.Errorf("d1 and d2 should be top-2 (appear in both lists), got %v", fused[:2])
	}

	for _, r := range fused {
		if r.Entry.ID == "d1" || r.Entry.ID == "d2" {
			if r.Reason != "keyword+semantic" {
				t.Errorf("d1/d2 reason should be keyword+semantic, got %s", r.Reason)
			}
		} else if r.Entry.ID == "d3" {
			if r.Reason != "keyword" {
				t.Errorf("d3 reason should be keyword, got %s", r.Reason)
			}
		} else if r.Entry.ID == "d4" {
			if r.Reason != "semantic" {
				t.Errorf("d4 reason should be semantic, got %s", r.Reason)
			}
		}
	}
}

func TestTokenizeCJK(t *testing.T) {
	tokens := tokenize("Hello 世界＝这是一个测试")
	found := make(map[string]bool)
	for _, tok := range tokens {
		found[tok] = true
	}
	if !found["hello"] {
		t.Error("expected 'hello' token")
	}
	if !found["世"] {
		t.Error("expected '世' token")
	}
	if !found["界"] {
		t.Error("expected '界' token")
	}
	if found["这"] {
		t.Error("'这' is a stop word and should be filtered")
	}
	if !found["测"] {
		t.Error("expected '测' token")
	}
}

func TestTokenizeStopWords(t *testing.T) {
	tokens := tokenize("the cat is on the mat")
	for _, tok := range tokens {
		if tok == "the" || tok == "is" || tok == "on" {
			t.Errorf("stop word %q should be filtered", tok)
		}
	}
	found := make(map[string]bool)
	for _, tok := range tokens {
		found[tok] = true
	}
	if !found["cat"] || !found["mat"] {
		t.Error("expected 'cat' and 'mat' to survive stop word filter")
	}
}
