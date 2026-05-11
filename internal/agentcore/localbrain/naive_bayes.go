package localbrain

import (
	"math"
	"strings"
	"sync"
	"unicode"
)

// NaiveBayesClassifier is a multinomial Naive Bayes intent classifier.
// Trained incrementally from RecordFeedback data; provides sub-millisecond
// classification as a fast-path before the LLM-based classifyWithLocal.
//
// P(category|words) ∝ P(category) × Π P(word|category)
// with Laplace smoothing: P(w|c) = (count(w,c) + α) / (totalWords(c) + α×|V|)
type NaiveBayesClassifier struct {
	mu sync.RWMutex

	// wordCounts[category][word] = occurrence count
	wordCounts map[string]map[string]int
	// classCounts[category] = total documents in this class
	classCounts map[string]int
	// classWordTotals[category] = total word count in this class
	classWordTotals map[string]int
	// vocabSize is |V| — number of unique words across all classes
	vocabSize int
	vocab     map[string]bool

	smoothing float64 // Laplace smoothing α (default 1.0)
	totalDocs int
}

// NewNaiveBayesClassifier creates a new classifier with Laplace smoothing.
func NewNaiveBayesClassifier() *NaiveBayesClassifier {
	return &NaiveBayesClassifier{
		wordCounts:      make(map[string]map[string]int),
		classCounts:     make(map[string]int),
		classWordTotals: make(map[string]int),
		vocab:           make(map[string]bool),
		smoothing:       1.0,
	}
}

// Train adds a single training example (query → category).
func (nb *NaiveBayesClassifier) Train(query string, category string) {
	nb.mu.Lock()
	defer nb.mu.Unlock()

	tokens := nbTokenize(query)
	if len(tokens) == 0 {
		return
	}

	nb.classCounts[category]++
	nb.totalDocs++

	if nb.wordCounts[category] == nil {
		nb.wordCounts[category] = make(map[string]int)
	}
	for _, token := range tokens {
		nb.wordCounts[category][token]++
		nb.classWordTotals[category]++
		if !nb.vocab[token] {
			nb.vocab[token] = true
			nb.vocabSize++
		}
	}
}

// NBPrediction holds the classification result.
type NBPrediction struct {
	Category   string
	Confidence float64
	Scores     map[string]float64
}

// Predict returns the most likely category and a confidence score [0, 1].
// Confidence is derived from the gap between the top-1 and top-2 log-probs.
func (nb *NaiveBayesClassifier) Predict(query string) (*Intent, float64) {
	nb.mu.RLock()
	defer nb.mu.RUnlock()

	if nb.totalDocs == 0 {
		return nil, 0
	}

	tokens := nbTokenize(query)
	if len(tokens) == 0 {
		return nil, 0
	}

	logProbs := make(map[string]float64)
	for category := range nb.classCounts {
		logProbs[category] = nb.logPosterior(tokens, category)
	}

	bestCat, bestScore := "", math.Inf(-1)
	secondScore := math.Inf(-1)
	for cat, score := range logProbs {
		if score > bestScore {
			secondScore = bestScore
			bestScore = score
			bestCat = cat
		} else if score > secondScore {
			secondScore = score
		}
	}

	if bestCat == "" {
		return nil, 0
	}

	// Confidence: softmax-normalized probability of the best class
	confidence := nb.softmaxConfidence(logProbs, bestCat)

	complexity := "medium"
	needTools := false
	switch bestCat {
	case "chat":
		complexity = "simple"
	case "complex":
		complexity = "hard"
		needTools = true
	case "tool":
		needTools = true
	case "code":
		complexity = "medium"
	}

	intent := &Intent{
		Category:   bestCat,
		Complexity: complexity,
		Confidence: confidence,
		NeedTools:  needTools,
	}
	return intent, confidence
}

// logPosterior computes log P(category|words) ∝ log P(category) + Σ log P(word|category)
func (nb *NaiveBayesClassifier) logPosterior(tokens []string, category string) float64 {
	logPrior := math.Log(float64(nb.classCounts[category]) / float64(nb.totalDocs))

	totalWords := nb.classWordTotals[category]
	smoothedDenom := float64(totalWords) + nb.smoothing*float64(nb.vocabSize)

	logLikelihood := 0.0
	catWords := nb.wordCounts[category]
	for _, token := range tokens {
		count := 0
		if catWords != nil {
			count = catWords[token]
		}
		logLikelihood += math.Log((float64(count) + nb.smoothing) / smoothedDenom)
	}

	return logPrior + logLikelihood
}

// softmaxConfidence converts log-probabilities to a normalized confidence via softmax.
func (nb *NaiveBayesClassifier) softmaxConfidence(logProbs map[string]float64, bestCat string) float64 {
	maxLogProb := math.Inf(-1)
	for _, lp := range logProbs {
		if lp > maxLogProb {
			maxLogProb = lp
		}
	}

	sumExp := 0.0
	for _, lp := range logProbs {
		sumExp += math.Exp(lp - maxLogProb)
	}

	bestExp := math.Exp(logProbs[bestCat] - maxLogProb)
	if sumExp == 0 {
		return 0
	}
	return bestExp / sumExp
}

// Trained returns whether the classifier has enough training data to be useful.
func (nb *NaiveBayesClassifier) Trained() bool {
	nb.mu.RLock()
	defer nb.mu.RUnlock()
	return nb.totalDocs >= 10 && len(nb.classCounts) >= 2
}

// Stats returns training statistics.
func (nb *NaiveBayesClassifier) Stats() (docs int, classes int, vocabSize int) {
	nb.mu.RLock()
	defer nb.mu.RUnlock()
	return nb.totalDocs, len(nb.classCounts), nb.vocabSize
}

// nbTokenize splits text into tokens for Naive Bayes.
// Handles CJK (bigram) and Latin (whitespace split, lowercased).
func nbTokenize(text string) []string {
	text = strings.ToLower(text)
	var tokens []string
	var current strings.Builder
	runes := []rune(text)

	flush := func() {
		if current.Len() > 0 {
			w := current.String()
			if len(w) > 1 && !nbStopWord(w) {
				tokens = append(tokens, w)
			}
			current.Reset()
		}
	}

	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if isCJK(r) {
			flush()
			tokens = append(tokens, string(r))
			if i+1 < len(runes) && isCJK(runes[i+1]) {
				tokens = append(tokens, string(runes[i:i+2]))
			}
		} else if unicode.IsLetter(r) || unicode.IsDigit(r) {
			current.WriteRune(r)
		} else {
			flush()
		}
	}
	flush()
	return tokens
}

func isCJK(r rune) bool {
	return (r >= 0x4E00 && r <= 0x9FFF) ||
		(r >= 0x3400 && r <= 0x4DBF) ||
		(r >= 0xF900 && r <= 0xFAFF)
}

func nbStopWord(w string) bool {
	switch w {
	case "the", "is", "are", "was", "and", "or", "of", "to", "in", "for",
		"a", "an", "it", "on", "at", "by", "be", "do", "as",
		"的", "了", "在", "是", "我", "有", "和", "就", "不", "人",
		"都", "一", "个", "上", "也", "很", "到", "说", "要", "你":
		return true
	}
	return false
}
