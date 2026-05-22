package ledger

import (
	"math"
	"sort"
	"strings"
	"sync"
	"unicode"
)

// BM25Index provides Okapi BM25 full-text search over memory entries.
// It maintains an inverted index of terms -> document IDs and computes
// BM25 relevance scores for keyword queries without any LLM calls.
type BM25Index struct {
	mu sync.RWMutex

	k1 float64 // term frequency saturation (default: 1.5)
	b  float64 // length normalization (default: 0.75)

	docs      map[string]*bm25Doc // docID -> document
	postings  map[string][]string // term -> list of docIDs
	docCount  int
	avgDocLen float64
	totalLen  int64
}

type bm25Doc struct {
	id      string
	terms   map[string]int // term -> frequency
	length  int            // total number of terms
	content string
}

// BM25Result holds a search result with its BM25 score.
type BM25Result struct {
	DocID   string
	Content string
	Score   float64
}

// NewBM25Index creates an empty BM25 index with standard parameters.
func NewBM25Index() *BM25Index {
	return &BM25Index{
		k1:       1.5,
		b:        0.75,
		docs:     make(map[string]*bm25Doc),
		postings: make(map[string][]string),
	}
}

// Add indexes a document. Safe for concurrent use.
func (idx *BM25Index) Add(id, content string) {
	terms := tokenize(content)
	freq := termFrequency(terms)

	idx.mu.Lock()
	defer idx.mu.Unlock()

	if old, exists := idx.docs[id]; exists {
		idx.totalLen -= int64(old.length)
		idx.removePostings(id, old.terms)
		idx.docCount--
	}

	doc := &bm25Doc{id: id, terms: freq, length: len(terms), content: content}
	idx.docs[id] = doc
	idx.docCount++
	idx.totalLen += int64(len(terms))
	idx.avgDocLen = float64(idx.totalLen) / math.Max(float64(idx.docCount), 1)

	for term := range freq {
		idx.postings[term] = append(idx.postings[term], id)
	}
}

// Remove deletes a document from the index.
func (idx *BM25Index) Remove(id string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	doc, exists := idx.docs[id]
	if !exists {
		return
	}
	idx.removePostings(id, doc.terms)
	idx.totalLen -= int64(doc.length)
	delete(idx.docs, id)
	idx.docCount--
	if idx.docCount > 0 {
		idx.avgDocLen = float64(idx.totalLen) / float64(idx.docCount)
	} else {
		idx.avgDocLen = 0
	}
}

func (idx *BM25Index) removePostings(id string, terms map[string]int) {
	for term := range terms {
		postings := idx.postings[term]
		for i, pid := range postings {
			if pid == id {
				idx.postings[term] = append(postings[:i], postings[i+1:]...)
				break
			}
		}
		if len(idx.postings[term]) == 0 {
			delete(idx.postings, term)
		}
	}
}

// Search returns the top-K documents ranked by BM25 score.
func (idx *BM25Index) Search(query string, limit int) []BM25Result {
	queryTerms := tokenize(query)
	if len(queryTerms) == 0 {
		return nil
	}

	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if idx.docCount == 0 {
		return nil
	}

	scores := make(map[string]float64)

	for _, qt := range queryTerms {
		postings := idx.postings[qt]
		if len(postings) == 0 {
			continue
		}

		df := float64(len(postings))
		idf := math.Log(1 + (float64(idx.docCount)-df+0.5)/(df+0.5))

		for _, docID := range postings {
			doc := idx.docs[docID]
			tf := float64(doc.terms[qt])
			docLen := float64(doc.length)

			numerator := tf * (idx.k1 + 1)
			denominator := tf + idx.k1*(1-idx.b+idx.b*docLen/idx.avgDocLen)

			scores[docID] += idf * numerator / denominator
		}
	}

	results := make([]BM25Result, 0, len(scores))
	for docID, score := range scores {
		doc := idx.docs[docID]
		results = append(results, BM25Result{
			DocID:   docID,
			Content: doc.content,
			Score:   score,
		})
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Score > results[j].Score })

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results
}

// Size returns the number of indexed documents.
func (idx *BM25Index) Size() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.docCount
}

// RRF performs Reciprocal Rank Fusion between two ranked lists.
// It combines BM25 keyword results with vector semantic results
// into a single ranking. The constant k controls how much weight
// is given to lower-ranked items (default: 60).
func RRF(bm25Results []BM25Result, vectorResults []ScoredEntry, k float64, limit int) []ScoredEntry {
	if k <= 0 {
		k = 60
	}

	fusedScores := make(map[string]float64)
	entryMap := make(map[string]MemoryEntry)

	for rank, r := range bm25Results {
		fusedScores[r.DocID] += 1.0 / (k + float64(rank+1))
	}
	for rank, r := range vectorResults {
		fusedScores[r.Entry.ID] += 1.0 / (k + float64(rank+1))
		entryMap[r.Entry.ID] = r.Entry
	}

	for _, r := range bm25Results {
		if _, exists := entryMap[r.DocID]; !exists {
			entryMap[r.DocID] = MemoryEntry{ID: r.DocID, Content: r.Content}
		}
	}

	results := make([]ScoredEntry, 0, len(fusedScores))
	for id, score := range fusedScores {
		entry, ok := entryMap[id]
		if !ok {
			continue
		}
		reason := "hybrid"
		if _, inBM25 := findBM25(bm25Results, id); inBM25 {
			if _, inVec := findVec(vectorResults, id); inVec {
				reason = "keyword+semantic"
			} else {
				reason = "keyword"
			}
		} else {
			reason = "semantic"
		}
		results = append(results, ScoredEntry{
			Entry:  entry,
			Score:  score,
			Reason: reason,
		})
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Score > results[j].Score })

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results
}

func findBM25(results []BM25Result, id string) (int, bool) {
	for i, r := range results {
		if r.DocID == id {
			return i, true
		}
	}
	return -1, false
}

func findVec(results []ScoredEntry, id string) (int, bool) {
	for i, r := range results {
		if r.Entry.ID == id {
			return i, true
		}
	}
	return -1, false
}

// tokenize splits text into lowercase terms, handling CJK characters
// with unigram + bigram (sliding window) and Latin text as
// whitespace-delimited words. Aligned with knowledge/bm25.go so both
// modules produce identical tokens for the same input.
func tokenize(text string) []string {
	text = strings.ToLower(text)
	var tokens []string
	var word strings.Builder
	runes := []rune(text)

	flushWord := func() {
		if word.Len() > 0 {
			tokens = append(tokens, word.String())
			word.Reset()
		}
	}

	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if isCJKRune(r) {
			flushWord()
			tokens = append(tokens, string(r))
			if i+1 < len(runes) && isCJKRune(runes[i+1]) {
				tokens = append(tokens, string(runes[i:i+2]))
			}
		} else if unicode.IsLetter(r) || unicode.IsDigit(r) {
			word.WriteRune(r)
		} else {
			flushWord()
		}
	}
	flushWord()

	return filterStopWords(tokens)
}

func isCJKRune(r rune) bool {
	return (r >= 0x4E00 && r <= 0x9FFF) || // CJK Unified
		(r >= 0x3400 && r <= 0x4DBF) || // CJK Extension A
		(r >= 0xF900 && r <= 0xFAFF) || // CJK Compat
		(r >= 0x3040 && r <= 0x30FF) || // Hiragana + Katakana
		(r >= 0xAC00 && r <= 0xD7AF) // Hangul
}

func termFrequency(terms []string) map[string]int {
	freq := make(map[string]int, len(terms))
	for _, t := range terms {
		freq[t]++
	}
	return freq
}

var defaultStopWords = map[string]bool{
	"the": true, "a": true, "an": true, "is": true, "are": true,
	"was": true, "were": true, "be": true, "been": true, "being": true,
	"have": true, "has": true, "had": true, "do": true, "does": true,
	"did": true, "will": true, "would": true, "could": true, "should": true,
	"may": true, "might": true, "shall": true, "can": true,
	"of": true, "in": true, "to": true, "for": true, "with": true,
	"on": true, "at": true, "by": true, "from": true, "as": true,
	"it": true, "its": true, "this": true, "that": true, "and": true,
	"or": true, "but": true, "not": true, "no": true, "if": true,
	"的": true, "了": true, "在": true, "是": true, "我": true,
	"有": true, "和": true, "就": true, "不": true, "人": true,
	"都": true, "着": true, "一": true, "他": true, "这": true,
	"中": true, "大": true, "为": true, "上": true, "也": true,
}

func filterStopWords(tokens []string) []string {
	result := make([]string, 0, len(tokens))
	for _, t := range tokens {
		if len(t) < 2 && !isCJK(t) {
			continue
		}
		if defaultStopWords[t] {
			continue
		}
		result = append(result, t)
	}
	return result
}

func isCJK(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Katakana, r) || unicode.Is(unicode.Hiragana, r) {
			return true
		}
	}
	return false
}
