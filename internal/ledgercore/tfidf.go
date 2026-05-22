package ledger

import (
	"math"
	"sort"
	"sync"
)

// TFIDFScorer computes TF-IDF importance scores for text content.
// It maintains a global document frequency table built from all indexed memories
// and scores new content by how "informative" it is relative to the corpus.
// High TF-IDF = rare/specific terms = more important to remember.
type TFIDFScorer struct {
	mu       sync.RWMutex
	docFreq  map[string]int // term ???number of documents containing it
	docCount int            // total documents in corpus
}

// TFIDFResult holds per-term and aggregate scores.
type TFIDFResult struct {
	Score      float64            // aggregate importance score (0-1, normalized)
	RawScore   float64            // sum of TF-IDF weights
	TopTerms   []TermScore        // highest-scoring terms
	TermCount  int                // number of unique terms after tokenization
	Specificity float64           // what fraction of terms are rare (IDF > median)
}

// TermScore pairs a term with its TF-IDF weight.
type TermScore struct {
	Term  string  `json:"term"`
	Score float64 `json:"score"`
}

// NewTFIDFScorer creates a scorer with an empty corpus.
func NewTFIDFScorer() *TFIDFScorer {
	return &TFIDFScorer{
		docFreq: make(map[string]int),
	}
}

// AddDocument updates the global IDF table with a new document.
// Call this whenever a memory is ingested to keep the corpus statistics current.
func (s *TFIDFScorer) AddDocument(content string) {
	terms := tokenize(content)
	seen := make(map[string]bool, len(terms))

	s.mu.Lock()
	defer s.mu.Unlock()

	s.docCount++
	for _, t := range terms {
		if !seen[t] {
			s.docFreq[t]++
			seen[t] = true
		}
	}
}

// RemoveDocument decrements DF counts. Best-effort; skips unknown terms.
func (s *TFIDFScorer) RemoveDocument(content string) {
	terms := tokenize(content)
	seen := make(map[string]bool, len(terms))

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.docCount > 0 {
		s.docCount--
	}
	for _, t := range terms {
		if !seen[t] {
			if s.docFreq[t] > 0 {
				s.docFreq[t]--
			}
			if s.docFreq[t] == 0 {
				delete(s.docFreq, t)
			}
			seen[t] = true
		}
	}
}

// Score computes the TF-IDF importance of a piece of content.
// Returns a normalized score in [0, 1] plus detailed per-term breakdown.
func (s *TFIDFScorer) Score(content string) TFIDFResult {
	terms := tokenize(content)
	if len(terms) == 0 {
		return TFIDFResult{}
	}

	freq := make(map[string]int, len(terms))
	for _, t := range terms {
		freq[t]++
	}

	s.mu.RLock()
	docCount := s.docCount
	dfCopy := make(map[string]int, len(freq))
	for t := range freq {
		dfCopy[t] = s.docFreq[t]
	}
	s.mu.RUnlock()

	if docCount == 0 {
		docCount = 1
	}

	var totalScore float64
	termScores := make([]TermScore, 0, len(freq))

	for term, tf := range freq {
		df := dfCopy[term]
		if df == 0 {
			df = 1 // unseen term gets max IDF boost
		}

		tfNorm := 1 + math.Log(float64(tf))
		idf := math.Log(float64(docCount+1) / float64(df+1))

		score := tfNorm * idf
		totalScore += score
		termScores = append(termScores, TermScore{Term: term, Score: score})
	}

	sort.Slice(termScores, func(i, j int) bool { return termScores[i].Score > termScores[j].Score })

	topN := 5
	if len(termScores) < topN {
		topN = len(termScores)
	}

	// Specificity: fraction of terms with IDF above median
	var specificity float64
	if len(termScores) > 0 {
		medianIDF := termScores[len(termScores)/2].Score
		rare := 0
		for _, ts := range termScores {
			if ts.Score > medianIDF {
				rare++
			}
		}
		specificity = float64(rare) / float64(len(termScores))
	}

	// Normalize to [0, 1] using sigmoid-like function
	// avgScore ranges from 0 to ~10; sigmoid centers around 2.0
	avgScore := totalScore / float64(len(freq))
	normalized := 1.0 / (1.0 + math.Exp(-1.5*(avgScore-2.0)))

	return TFIDFResult{
		Score:       normalized,
		RawScore:    totalScore,
		TopTerms:    termScores[:topN],
		TermCount:   len(freq),
		Specificity: specificity,
	}
}

// CorpusSize returns the number of documents in the IDF table.
func (s *TFIDFScorer) CorpusSize() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.docCount
}

// VocabSize returns the number of unique terms in the corpus.
func (s *TFIDFScorer) VocabSize() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.docFreq)
}
