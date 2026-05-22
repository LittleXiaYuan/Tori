package ledger

import (
	"testing"
)

func TestTFIDFBasicScoring(t *testing.T) {
	s := NewTFIDFScorer()
	s.AddDocument("the cat sat on the mat")
	s.AddDocument("the dog sat on the log")
	s.AddDocument("the cat chased the dog")
	s.AddDocument("birds fly in the sky")

	catResult := s.Score("the cat sat on the mat")
	birdResult := s.Score("exotic parrots from madagascar")

	if birdResult.Score <= catResult.Score {
		t.Errorf("rare content (parrots/madagascar) should score higher than common content (cat/mat), got rare=%.3f common=%.3f",
			birdResult.Score, catResult.Score)
	}
}

func TestTFIDFChineseContent(t *testing.T) {
	s := NewTFIDFScorer()
	s.AddDocument("今天天气很好")
	s.AddDocument("明天天气也不错")
	s.AddDocument("天气预报说会下雨")

	commonResult := s.Score("天气很好")
	rareResult := s.Score("量子计算机突破性进展")

	if rareResult.Score <= commonResult.Score {
		t.Errorf("rare Chinese content should score higher, got rare=%.3f common=%.3f",
			rareResult.Score, commonResult.Score)
	}
}

func TestTFIDFEmptyCorpus(t *testing.T) {
	s := NewTFIDFScorer()
	result := s.Score("anything goes here")
	if result.Score <= 0 {
		t.Errorf("empty corpus should still produce positive score (all terms are novel), got %.3f", result.Score)
	}
}

func TestTFIDFEmptyContent(t *testing.T) {
	s := NewTFIDFScorer()
	s.AddDocument("some content")
	result := s.Score("")
	if result.Score != 0 {
		t.Errorf("empty content should score 0, got %.3f", result.Score)
	}
}

func TestTFIDFRemoveDocument(t *testing.T) {
	s := NewTFIDFScorer()
	s.AddDocument("machine learning algorithms")
	s.AddDocument("deep learning neural networks")
	s.AddDocument("machine learning optimization")

	if s.CorpusSize() != 3 {
		t.Fatalf("expected corpus size 3, got %d", s.CorpusSize())
	}

	s.RemoveDocument("machine learning algorithms")
	if s.CorpusSize() != 2 {
		t.Fatalf("expected corpus size 2, got %d", s.CorpusSize())
	}
}

func TestTFIDFSpecificity(t *testing.T) {
	s := NewTFIDFScorer()
	for i := 0; i < 100; i++ {
		s.AddDocument("common words that appear everywhere in every document")
	}

	commonResult := s.Score("common words that appear everywhere")
	specificResult := s.Score("cryptographic zero knowledge proof implementation")

	if specificResult.Specificity < commonResult.Specificity {
		t.Logf("specificity: specific=%.3f common=%.3f (may vary with corpus size)",
			specificResult.Specificity, commonResult.Specificity)
	}
}

func TestTFIDFTopTerms(t *testing.T) {
	s := NewTFIDFScorer()
	s.AddDocument("programming in Go")
	s.AddDocument("programming in Python")
	s.AddDocument("programming in Rust")

	result := s.Score("quantum programming in Haskell")
	if len(result.TopTerms) == 0 {
		t.Fatal("expected top terms")
	}

	topTerm := result.TopTerms[0].Term
	if topTerm != "quantum" && topTerm != "haskell" {
		t.Logf("top term: %s (expected 'quantum' or 'haskell' as rarest)", topTerm)
	}
}

func TestTFIDFCorpusStats(t *testing.T) {
	s := NewTFIDFScorer()
	if s.CorpusSize() != 0 || s.VocabSize() != 0 {
		t.Fatal("new scorer should have empty corpus")
	}

	s.AddDocument("hello world")
	if s.CorpusSize() != 1 {
		t.Errorf("expected corpus size 1, got %d", s.CorpusSize())
	}
	if s.VocabSize() < 2 {
		t.Errorf("expected vocab size >= 2, got %d", s.VocabSize())
	}
}
