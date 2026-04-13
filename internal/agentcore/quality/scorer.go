// Package quality provides lightweight, model-free dialogue quality scoring.
// It evaluates response quality using statistical heuristics instead of LLM calls,
// making it suitable for high-frequency evaluation with zero API cost.
//
// References:
//   - N-gram diversity: arxiv.org/html/2403.00553v1
//   - ChrF / BLEU: heuristic text quality metrics
//   - PMIScore: arxiv.org/html/2603.13796v1 (lightweight engagement scoring)
package quality

import (
	"math"
	"strings"
	"unicode"
)

// Score represents a multi-dimensional quality assessment.
type Score struct {
	Overall          float64          `json:"overall"`           // weighted composite [0, 1]
	KeywordCoverage  float64          `json:"keyword_coverage"`  // fraction of query keywords in reply
	NGramDiversity   float64          `json:"ngram_diversity"`   // 1 - self-repetition ratio
	LengthRatio      float64          `json:"length_ratio"`      // normalized reply/query length
	InformationDensity float64        `json:"information_density"` // unique words / total words
	QuestionAlignment float64         `json:"question_alignment"` // does reply address the question
	Satisfied        bool             `json:"satisfied"`          // overall >= threshold
	Details          map[string]float64 `json:"details,omitempty"`
}

// Scorer evaluates dialogue quality without LLM calls.
type Scorer struct {
	// Score thresholds
	SatisfiedThreshold float64 // default: 0.45
	// Weights for overall score
	Weights ScorerWeights
}

// ScorerWeights controls the relative importance of each metric.
type ScorerWeights struct {
	KeywordCoverage    float64
	NGramDiversity     float64
	LengthRatio        float64
	InformationDensity float64
	QuestionAlignment  float64
}

// DefaultWeights returns balanced metric weights.
func DefaultWeights() ScorerWeights {
	return ScorerWeights{
		KeywordCoverage:    0.30,
		NGramDiversity:     0.20,
		LengthRatio:        0.15,
		InformationDensity: 0.15,
		QuestionAlignment:  0.20,
	}
}

// NewScorer creates a scorer with default settings.
func NewScorer() *Scorer {
	return &Scorer{
		SatisfiedThreshold: 0.45,
		Weights:            DefaultWeights(),
	}
}

// Evaluate scores a query-reply pair.
func (s *Scorer) Evaluate(query, reply string) Score {
	if len(reply) == 0 {
		return Score{Satisfied: false}
	}

	queryTokens := tokenize(query)
	replyTokens := tokenize(reply)

	kc := keywordCoverage(queryTokens, replyTokens)
	nd := ngramDiversity(replyTokens, 3)
	lr := lengthRatioScore(queryTokens, replyTokens)
	id := informationDensity(replyTokens)
	qa := questionAlignment(query, reply)

	w := s.Weights
	overall := kc*w.KeywordCoverage +
		nd*w.NGramDiversity +
		lr*w.LengthRatio +
		id*w.InformationDensity +
		qa*w.QuestionAlignment

	return Score{
		Overall:            math.Round(overall*1000) / 1000,
		KeywordCoverage:    math.Round(kc*1000) / 1000,
		NGramDiversity:     math.Round(nd*1000) / 1000,
		LengthRatio:        math.Round(lr*1000) / 1000,
		InformationDensity: math.Round(id*1000) / 1000,
		QuestionAlignment:  math.Round(qa*1000) / 1000,
		Satisfied:          overall >= s.SatisfiedThreshold,
	}
}

// keywordCoverage measures what fraction of query keywords appear in the reply.
// Returns 0-1 where 1 means all query keywords are present in the reply.
func keywordCoverage(queryTokens, replyTokens []string) float64 {
	if len(queryTokens) == 0 {
		return 1.0
	}

	replySet := make(map[string]bool, len(replyTokens))
	for _, t := range replyTokens {
		replySet[t] = true
	}

	covered := 0
	for _, qt := range queryTokens {
		if replySet[qt] {
			covered++
		}
	}
	return float64(covered) / float64(len(queryTokens))
}

// ngramDiversity measures how diverse the reply is using n-gram self-repetition.
// Returns 0-1 where 1 = maximally diverse (no repeated n-grams).
func ngramDiversity(tokens []string, n int) float64 {
	if len(tokens) <= n {
		return 1.0
	}

	ngrams := make(map[string]bool)
	for i := 0; i <= len(tokens)-n; i++ {
		ngram := strings.Join(tokens[i:i+n], " ")
		ngrams[ngram] = true
	}

	total := len(tokens) - n + 1
	unique := len(ngrams)
	return float64(unique) / float64(total)
}

// lengthRatioScore evaluates whether the reply length is appropriate.
// Sweet spot: reply is 1-5x the query length. Too short or too long gets penalized.
func lengthRatioScore(queryTokens, replyTokens []string) float64 {
	qLen := float64(len(queryTokens))
	rLen := float64(len(replyTokens))

	if qLen == 0 {
		qLen = 1
	}

	ratio := rLen / qLen

	switch {
	case ratio < 0.3:
		return 0.2 // too short
	case ratio < 1.0:
		return 0.5 + 0.3*(ratio-0.3)/0.7
	case ratio <= 5.0:
		return 0.8 + 0.2*(1-(ratio-1)/4) // sweet spot, slight decrease as ratio grows
	case ratio <= 10.0:
		return 0.6 - 0.3*(ratio-5)/5 // getting wordy
	default:
		return 0.2 // way too long
	}
}

// informationDensity measures the ratio of unique words to total words.
// High density = diverse vocabulary = more informative.
func informationDensity(tokens []string) float64 {
	if len(tokens) == 0 {
		return 0
	}

	unique := make(map[string]bool, len(tokens))
	for _, t := range tokens {
		unique[t] = true
	}

	ratio := float64(len(unique)) / float64(len(tokens))
	// Short texts naturally have high density; normalize by log(len)
	lenFactor := math.Min(1.0, math.Log(float64(len(tokens)+1))/math.Log(50))
	return ratio * lenFactor
}

// questionAlignment checks whether the reply addresses the question type.
// Looks for question markers in query and appropriate response patterns.
func questionAlignment(query, reply string) float64 {
	lowerQ := strings.ToLower(query)
	lowerR := strings.ToLower(reply)
	score := 0.5 // neutral baseline

	isQuestion := strings.ContainsAny(query, "?？") ||
		containsAny(lowerQ, []string{"what", "how", "why", "when", "where", "who", "which",
			"什么", "怎么", "为什么", "哪", "几", "多少", "是否", "能不能", "可以"})

	if isQuestion {
		hasExplanation := len(reply) > 30 ||
			containsAny(lowerR, []string{"because", "since", "therefore", "so",
				"因为", "所以", "由于", "因此", "首先", "其次", "总结"})
		if hasExplanation {
			score += 0.3
		}

		isRefusal := containsAny(lowerR, []string{"i can't", "i cannot", "i'm not able",
			"sorry, i", "i don't know",
			"我不能", "我无法", "抱歉", "对不起我不"})
		if isRefusal {
			score -= 0.2
		}
	}

	if strings.Contains(lowerQ, "code") || strings.Contains(lowerQ, "代码") {
		if strings.Contains(reply, "```") || strings.Contains(reply, "func ") ||
			strings.Contains(reply, "def ") || strings.Contains(reply, "class ") {
			score += 0.2
		}
	}

	return math.Max(0, math.Min(1, score))
}

func containsAny(s string, substrs []string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// tokenize splits text into lowercase tokens (CJK-aware).
func tokenize(text string) []string {
	text = strings.ToLower(text)
	var tokens []string
	var word strings.Builder

	for _, r := range text {
		if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Katakana, r) || unicode.Is(unicode.Hiragana, r) {
			if word.Len() > 0 {
				tokens = append(tokens, word.String())
				word.Reset()
			}
			tokens = append(tokens, string(r))
		} else if unicode.IsLetter(r) || unicode.IsDigit(r) {
			word.WriteRune(r)
		} else {
			if word.Len() > 0 {
				tokens = append(tokens, word.String())
				word.Reset()
			}
		}
	}
	if word.Len() > 0 {
		tokens = append(tokens, word.String())
	}

	return tokens
}
