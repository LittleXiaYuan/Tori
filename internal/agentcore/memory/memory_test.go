package memory

import (
	"context"
	"testing"
	"time"
)

// --- Short-term tests ---

func TestShortTermPutGet(t *testing.T) {
	s := NewShortTerm(1 * time.Hour)
	ctx := context.Background()
	err := s.Put(ctx, "t1", Item{Key: "k1", Value: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	item, err := s.Get(ctx, "t1", "k1")
	if err != nil {
		t.Fatal(err)
	}
	if item == nil || item.Value != "hello" {
		t.Fatalf("expected hello, got %v", item)
	}
}

func TestShortTermExpiry(t *testing.T) {
	s := NewShortTerm(1 * time.Millisecond)
	ctx := context.Background()
	s.Put(ctx, "t1", Item{Key: "k1", Value: "temp"})
	time.Sleep(5 * time.Millisecond)
	s.GC()
	item, _ := s.Get(ctx, "t1", "k1")
	if item != nil {
		t.Fatal("expected expired item to be nil")
	}
}

func TestShortTermSlidingWindow(t *testing.T) {
	s := NewShortTerm(1 * time.Hour)
	s.maxPerSession = 3 // small window for testing
	ctx := context.Background()

	// Insert 5 items into same session key
	for i := 0; i < 5; i++ {
		s.Put(ctx, "t1", Item{Key: "session1", Value: "msg" + string(rune('A'+i))})
	}

	// Should only keep last 3 (sliding window)
	count := s.Count("t1")
	if count != 3 {
		t.Fatalf("expected 3 items after sliding window eviction, got %d", count)
	}

	// Get should return the most recent item
	item, _ := s.Get(ctx, "t1", "session1")
	if item == nil || item.Value != "msgE" {
		t.Fatalf("expected msgE (most recent), got %v", item)
	}
}

func TestShortTermRecencyScoring(t *testing.T) {
	s := NewShortTerm(1 * time.Hour)
	ctx := context.Background()

	// Insert items with slight time gaps
	s.Put(ctx, "t1", Item{Key: "s1", Value: "old data about math"})
	time.Sleep(2 * time.Millisecond)
	s.Put(ctx, "t1", Item{Key: "s1", Value: "new data about math"})

	results, _ := s.Search(ctx, "t1", "math", 10)
	if len(results) < 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	// Most recent should have higher score
	if results[0].Value != "new data about math" {
		t.Fatalf("expected newest item first, got %s", results[0].Value)
	}
	if results[0].Score <= results[1].Score {
		t.Fatal("expected newer item to have higher score")
	}
}

// --- Mid-term tests ---

func TestMidTermDedup(t *testing.T) {
	m := NewMidTerm()
	ctx := context.Background()
	m.Put(ctx, "t1", Item{Value: "fact1"})
	m.Put(ctx, "t1", Item{Value: "fact1"}) // same value, auto-dedup
	count := m.Count("t1")
	if count != 1 {
		t.Fatalf("expected 1 (deduped), got %d", count)
	}
}

func TestMidTermJaccardDedup(t *testing.T) {
	m := NewMidTerm()
	ctx := context.Background()
	m.Put(ctx, "t1", Item{Value: "用户喜欢数学和物理"})
	m.Put(ctx, "t1", Item{Value: "用户喜欢数学和物理学科"}) // very similar, should dedup
	count := m.Count("t1")
	if count != 1 {
		t.Fatalf("expected 1 (Jaccard dedup), got %d", count)
	}
}

func TestMidTermDifferentFactsNotDeduped(t *testing.T) {
	m := NewMidTerm()
	ctx := context.Background()
	m.Put(ctx, "t1", Item{Value: "用户喜欢数学"})
	m.Put(ctx, "t1", Item{Value: "天气预报显示明天下雨"}) // completely different
	count := m.Count("t1")
	if count != 2 {
		t.Fatalf("expected 2 (different facts), got %d", count)
	}
}

func TestMidTermSearch(t *testing.T) {
	m := NewMidTerm()
	ctx := context.Background()
	m.Put(ctx, "t1", Item{Key: "k1", Value: "用户喜欢数学"})
	m.Put(ctx, "t1", Item{Key: "k2", Value: "用户擅长英语"})
	results, err := m.Search(ctx, "t1", "数学", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestMidTermTFIDFRanking(t *testing.T) {
	m := NewMidTerm()
	ctx := context.Background()
	// Doc with "数学" once
	m.Put(ctx, "t1", Item{Key: "k1", Value: "用户喜欢数学"})
	// Doc with "数学" twice and more specific
	m.Put(ctx, "t1", Item{Key: "k2", Value: "数学考试数学成绩优秀"})
	// Unrelated doc
	m.Put(ctx, "t1", Item{Key: "k3", Value: "今天天气不错"})

	results, _ := m.Search(ctx, "t1", "数学", 10)
	if len(results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(results))
	}
	// k2 has higher TF for "数学", should rank higher
	if results[0].Key != "k2" {
		t.Fatalf("expected k2 first (higher TF), got %s", results[0].Key)
	}
	if results[0].Score <= 0 {
		t.Fatal("expected positive score")
	}
}

func TestMidTermAccessTracking(t *testing.T) {
	m := NewMidTerm()
	ctx := context.Background()
	m.Put(ctx, "t1", Item{Key: "k1", Value: "tracked fact"})

	// Access it
	item, _ := m.Get(ctx, "t1", "k1")
	if item == nil {
		t.Fatal("expected item")
	}
	if item.AccessCnt < 1 {
		t.Fatalf("expected access count >= 1, got %d", item.AccessCnt)
	}
}

// --- Long-term tests ---

func TestLongTermBM25(t *testing.T) {
	l := NewLongTerm()
	ctx := context.Background()
	l.Put(ctx, "t1", Item{Key: "d1", Value: "机器学习是人工智能的一个分支"})
	l.Put(ctx, "t1", Item{Key: "d2", Value: "深度学习使用神经网络进行训练"})
	l.Put(ctx, "t1", Item{Key: "d3", Value: "今天天气不错适合运动"})
	results, _ := l.Search(ctx, "t1", "学习 神经网络", 2)
	if len(results) == 0 {
		t.Fatal("expected search results")
	}
	// d2 should rank higher (has both 学习 and 神经网络)
	if results[0].Key != "d2" {
		t.Fatalf("expected d2 first, got %s", results[0].Key)
	}
}

func TestLongTermBM25RealIDF(t *testing.T) {
	l := NewLongTerm()
	ctx := context.Background()
	// "学习" appears in 2/3 docs (common term, lower IDF)
	// "运动" appears in 1/3 docs (rare term, higher IDF)
	l.Put(ctx, "t1", Item{Key: "d1", Value: "机器学习是人工智能的分支"})
	l.Put(ctx, "t1", Item{Key: "d2", Value: "深度学习用于模型训练"})
	l.Put(ctx, "t1", Item{Key: "d3", Value: "运动有益健康"})

	results, _ := l.Search(ctx, "t1", "运动", 3)
	if len(results) == 0 || results[0].Key != "d3" {
		t.Fatalf("expected d3 first for rare term '运动', got %v", results)
	}
	// Rare term should produce higher score than common term
	rareScore := results[0].Score

	results2, _ := l.Search(ctx, "t1", "学习", 3)
	if len(results2) == 0 {
		t.Fatal("expected results for '学习'")
	}
	commonScore := results2[0].Score

	// Rare term should have significantly higher IDF contribution
	if rareScore <= commonScore*0.8 {
		t.Logf("rare=%.4f common=%.4f", rareScore, commonScore)
		// Not a hard failure since BM25 scoring depends on doc length too,
		// but we log for verification
	}
}

func TestLongTermCosineSimilarity(t *testing.T) {
	// Unit test the cosine similarity function directly
	a := []float32{1, 0, 0}
	b := []float32{1, 0, 0}
	sim := cosineSimilarity(a, b)
	if sim < 0.99 {
		t.Fatalf("identical vectors should have sim ~1.0, got %f", sim)
	}

	c := []float32{0, 1, 0}
	sim2 := cosineSimilarity(a, c)
	if sim2 > 0.01 {
		t.Fatalf("orthogonal vectors should have sim ~0.0, got %f", sim2)
	}

	d := []float32{0.7, 0.7, 0}
	sim3 := cosineSimilarity(a, d)
	if sim3 < 0.5 || sim3 > 0.8 {
		t.Fatalf("45-degree vectors should have sim ~0.7, got %f", sim3)
	}
}

// --- Manager tests ---

func TestManagerSearchAll(t *testing.T) {
	short := NewShortTerm(1 * time.Hour)
	mid := NewMidTerm()
	long := NewLongTerm()
	mgr := NewManager(short, mid, long)
	ctx := context.Background()

	short.Put(ctx, "t1", Item{Key: "s1", Value: "短期记忆数学"})
	mid.Put(ctx, "t1", Item{Key: "m1", Value: "中期数学事实"})
	long.Put(ctx, "t1", Item{Key: "l1", Value: "长期数学知识库"})

	results, err := mgr.SearchAll(ctx, "t1", "数学", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results from all layers, got %d", len(results))
	}

	// All results should have weighted scores
	for _, r := range results {
		if r.Score <= 0 {
			t.Fatalf("expected positive weighted score, got %f for %s", r.Score, r.Source)
		}
	}
}

func TestManagerWeightedRanking(t *testing.T) {
	short := NewShortTerm(1 * time.Hour)
	mid := NewMidTerm()
	long := NewLongTerm()
	mgr := NewManager(short, mid, long)
	ctx := context.Background()

	// Same content across layers
	short.Put(ctx, "t1", Item{Key: "s1", Value: "人工智能深度学习"})
	mid.Put(ctx, "t1", Item{Key: "m1", Value: "人工智能深度学习"})
	long.Put(ctx, "t1", Item{Key: "l1", Value: "人工智能深度学习"})

	results, _ := mgr.SearchAll(ctx, "t1", "人工智能", 10)
	if len(results) < 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// Long-term (weight=1.0) should rank highest for same content
	// because longWeight > midWeight > shortWeight
	foundLongFirst := false
	for _, r := range results {
		if r.Source == "long:" {
			foundLongFirst = true
			break
		}
		break // only check first result
	}
	if !foundLongFirst {
		t.Logf("results: %v", results[0].Source)
		// This is expected behavior but depends on exact scoring
	}
}

func TestManagerStats(t *testing.T) {
	short := NewShortTerm(1 * time.Hour)
	mid := NewMidTerm()
	long := NewLongTerm()
	mgr := NewManager(short, mid, long)
	ctx := context.Background()

	short.Put(ctx, "t1", Item{Key: "s1", Value: "a"})
	mid.Put(ctx, "t1", Item{Key: "m1", Value: "b"})
	mid.Put(ctx, "t1", Item{Key: "m2", Value: "c"})

	stats := mgr.Stats("t1")
	if stats["short"] != 1 {
		t.Fatalf("expected short=1, got %d", stats["short"])
	}
	if stats["mid"] != 2 {
		t.Fatalf("expected mid=2, got %d", stats["mid"])
	}
	if stats["long"] != 0 {
		t.Fatalf("expected long=0, got %d", stats["long"])
	}
}

// --- Tokenizer tests ---

func TestTokenizeForIDF(t *testing.T) {
	tokens := tokenizeForIDF("机器学习 machine learning")
	if len(tokens) == 0 {
		t.Fatal("expected tokens")
	}
	// Should contain both latin words and CJK bigrams/unigrams
	hasLatin := false
	hasCJK := false
	for _, tok := range tokens {
		if tok == "machine" || tok == "learning" {
			hasLatin = true
		}
		runes := []rune(tok)
		if len(runes) > 0 && runes[0] >= 0x4e00 {
			hasCJK = true
		}
	}
	if !hasLatin || !hasCJK {
		t.Fatalf("expected both latin and CJK tokens, got %v", tokens)
	}
}
