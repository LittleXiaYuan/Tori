package knowledge

import (
	"math"
	"sort"
	"strings"
	"unicode"
)

// ──────────────────────────────────────────────
// BM25 稀疏检索 (纯Go实现，支持中文)
// ──────────────────────────────────────────────

// BM25Index implements the Okapi BM25 ranking algorithm.
type BM25Index struct {
	k1       float64        // term frequency saturation (default 1.5)
	b        float64        // length normalization (default 0.75)
	corpus   [][]string     // tokenized documents
	docFreq  map[string]int // document frequency per term
	docLen   []int          // document lengths
	avgDL    float64        // average document length
	docCount int            // total document count
	chunkIDs []string       // parallel to corpus: chunk IDs
}

// BM25Result holds a scored search result.
type BM25Result struct {
	ChunkIndex int
	ChunkID    string
	Score      float64
}

// NewBM25Index builds a BM25 index from knowledge chunks.
func NewBM25Index(chunks []Chunk) *BM25Index {
	idx := &BM25Index{
		k1:       1.5,
		b:        0.75,
		corpus:   make([][]string, len(chunks)),
		chunkIDs: make([]string, len(chunks)),
		docFreq:  make(map[string]int),
		docLen:   make([]int, len(chunks)),
		docCount: len(chunks),
	}

	totalLen := 0
	for i, c := range chunks {
		tokens := tokenize(c.Content)
		idx.corpus[i] = tokens
		idx.chunkIDs[i] = c.ID
		idx.docLen[i] = len(tokens)
		totalLen += len(tokens)

		// Count document frequency (unique terms per doc)
		seen := make(map[string]bool)
		for _, t := range tokens {
			if !seen[t] {
				idx.docFreq[t]++
				seen[t] = true
			}
		}
	}

	if idx.docCount > 0 {
		idx.avgDL = float64(totalLen) / float64(idx.docCount)
	}

	return idx
}

// Search returns top-K results for the given query using BM25 scoring.
func (idx *BM25Index) Search(query string, topK int) []BM25Result {
	if idx.docCount == 0 || topK <= 0 {
		return nil
	}

	queryTokens := tokenize(query)
	if len(queryTokens) == 0 {
		return nil
	}

	scores := make([]float64, idx.docCount)
	for _, qt := range queryTokens {
		df, ok := idx.docFreq[qt]
		if !ok {
			continue
		}
		// IDF: log((N - df + 0.5) / (df + 0.5) + 1)
		idf := math.Log((float64(idx.docCount)-float64(df)+0.5)/(float64(df)+0.5) + 1.0)

		for i, doc := range idx.corpus {
			tf := termFrequency(doc, qt)
			if tf == 0 {
				continue
			}
			// BM25: IDF * (tf * (k1 + 1)) / (tf + k1 * (1 - b + b * dl / avgdl))
			dl := float64(idx.docLen[i])
			numerator := float64(tf) * (idx.k1 + 1)
			denominator := float64(tf) + idx.k1*(1-idx.b+idx.b*dl/idx.avgDL)
			scores[i] += idf * numerator / denominator
		}
	}

	// Sort by score descending
	type scored struct {
		index int
		score float64
	}
	var ranked []scored
	for i, s := range scores {
		if s > 0 {
			ranked = append(ranked, scored{i, s})
		}
	}
	sort.Slice(ranked, func(i, j int) bool { return ranked[i].score > ranked[j].score })

	if topK > len(ranked) {
		topK = len(ranked)
	}

	results := make([]BM25Result, topK)
	for i := 0; i < topK; i++ {
		results[i] = BM25Result{
			ChunkIndex: ranked[i].index,
			ChunkID:    idx.chunkIDs[ranked[i].index],
			Score:      ranked[i].score,
		}
	}
	return results
}

// termFrequency counts occurrences of a term in a document.
func termFrequency(doc []string, term string) int {
	count := 0
	for _, t := range doc {
		if t == term {
			count++
		}
	}
	return count
}

// ──────────────────────────────────────────────
// Chinese-aware tokenizer (unigram + bigram)
// ──────────────────────────────────────────────

// tokenize splits text into tokens with CJK character awareness.
// For CJK text: unigram + bigram (poor man's segmentation).
// For Latin text: whitespace + punctuation split, lowercased.
func tokenize(text string) []string {
	text = strings.ToLower(text)
	var tokens []string
	var current strings.Builder
	runes := []rune(text)

	flushCurrent := func() {
		if current.Len() > 0 {
			word := current.String()
			if !isStopWord(word) {
				tokens = append(tokens, word)
			}
			current.Reset()
		}
	}

	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if isCJKRune(r) {
			flushCurrent()
			// Unigram
			tokens = append(tokens, string(r))
			// Bigram (sliding window)
			if i+1 < len(runes) && isCJKRune(runes[i+1]) {
				tokens = append(tokens, string(runes[i:i+2]))
			}
		} else if unicode.IsLetter(r) || unicode.IsDigit(r) {
			current.WriteRune(r)
		} else {
			flushCurrent()
		}
	}
	flushCurrent()
	return tokens
}

func isCJKRune(r rune) bool {
	return (r >= 0x4E00 && r <= 0x9FFF) || // CJK Unified
		(r >= 0x3400 && r <= 0x4DBF) || // CJK Extension A
		(r >= 0xF900 && r <= 0xFAFF) || // CJK Compat
		(r >= 0x3040 && r <= 0x30FF) || // Hiragana + Katakana
		(r >= 0xAC00 && r <= 0xD7AF) // Hangul
}

// isStopWord checks common stop words (Chinese + English).
func isStopWord(word string) bool {
	_, ok := stopWords[word]
	return ok
}

var stopWords = map[string]struct{}{
	// English
	"a": {}, "an": {}, "the": {}, "is": {}, "are": {}, "was": {}, "were": {},
	"be": {}, "been": {}, "being": {}, "have": {}, "has": {}, "had": {},
	"do": {}, "does": {}, "did": {}, "will": {}, "would": {}, "could": {},
	"should": {}, "may": {}, "might": {}, "shall": {}, "can": {},
	"of": {}, "in": {}, "to": {}, "for": {}, "on": {}, "at": {}, "by": {},
	"with": {}, "from": {}, "and": {}, "or": {}, "but": {}, "not": {},
	"it": {}, "this": {}, "that": {}, "i": {}, "me": {}, "my": {},
	"we": {}, "you": {}, "he": {}, "she": {}, "they": {},
	// Chinese
	"的": {}, "了": {}, "在": {}, "是": {}, "我": {}, "有": {}, "和": {},
	"就": {}, "不": {}, "人": {}, "都": {}, "一": {}, "个": {}, "上": {},
	"也": {}, "很": {}, "到": {}, "说": {}, "要": {}, "去": {}, "你": {},
	"会": {}, "着": {}, "没有": {}, "看": {}, "好": {}, "自己": {},
	"这": {}, "他": {}, "她": {}, "它": {}, "们": {},
	"那": {}, "里": {}, "把": {}, "让": {}, "还": {},
	"吗": {}, "吧": {}, "呢": {}, "啊": {}, "哦": {},
}

// ──────────────────────────────────────────────
// RRF (Reciprocal Rank Fusion)
// ──────────────────────────────────────────────

// ScoredChunk holds a retrieval result with its fusion score.
type ScoredChunk struct {
	Chunk Chunk
	Score float64
}

// FuseRRF combines dense (vector) and sparse (BM25) results using
// Reciprocal Rank Fusion: score(doc) = sum(1/(k+rank_i))
// k is the smoothing parameter (default 60).
func FuseRRF(denseChunks, sparseChunks []Chunk, k int, topK int) []ScoredChunk {
	if k <= 0 {
		k = 60
	}
	if topK <= 0 {
		topK = 10
	}

	// Build rank maps
	type rankEntry struct {
		chunk Chunk
		rrf   float64
	}
	entries := make(map[string]*rankEntry)

	for rank, c := range denseChunks {
		e, ok := entries[c.ID]
		if !ok {
			e = &rankEntry{chunk: c}
			entries[c.ID] = e
		}
		e.rrf += 1.0 / float64(k+rank+1) // rank is 0-indexed, so +1
	}

	for rank, c := range sparseChunks {
		e, ok := entries[c.ID]
		if !ok {
			e = &rankEntry{chunk: c}
			entries[c.ID] = e
		}
		e.rrf += 1.0 / float64(k+rank+1)
	}

	// Sort by RRF score
	result := make([]ScoredChunk, 0, len(entries))
	for _, e := range entries {
		result = append(result, ScoredChunk{Chunk: e.chunk, Score: e.rrf})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Score > result[j].Score })

	if topK > len(result) {
		topK = len(result)
	}
	return result[:topK]
}
