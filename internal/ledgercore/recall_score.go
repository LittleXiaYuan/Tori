package ledger

import (
	"math"
	"strings"
	"time"
)

// ScoreWeights defines the weights for each recall scoring factor.
type ScoreWeights struct {
	KeywordRelevance float64
	GoalAlignment    float64
	KindBoost        float64
	Recency          float64
	Confidence       float64
	AccessFrequency  float64
	SourceTrust      float64
}

// DefaultWeights returns the default scoring weights.
func DefaultWeights() ScoreWeights {
	return ScoreWeights{
		KeywordRelevance: 0.30,
		GoalAlignment:    0.20,
		KindBoost:        0.10,
		Recency:          0.15,
		Confidence:       0.10,
		AccessFrequency:  0.05,
		SourceTrust:      0.10,
	}
}

// scoreEntry computes a composite relevance score for a memory entry
// in the context of a recall query.
func scoreEntry(m *MemoryEntry, q *RecallQuery, w ScoreWeights) (float64, string) {
	var reasons []string

	// 1. Keyword relevance: how well the content matches the query
	kr := keywordRelevance(m.Content, m.Key, q.Query)
	if kr > 0.5 {
		reasons = append(reasons, "keyword match")
	}

	// 2. Goal alignment: query-to-task-goal keyword overlap
	ga := keywordRelevance(m.Content, m.Key, q.TaskGoal)
	if ga > 0.3 {
		reasons = append(reasons, "goal aligned")
	}

	// 3. Kind boost: certain memory kinds are more relevant for certain task types
	kb := kindBoost(m.Kind, q.TaskType)
	if kb > 0.5 {
		reasons = append(reasons, "kind match")
	}

	// 4. Recency: newer memories score higher (exponential decay)
	rec := recencyScore(m.UpdatedAt)

	// 5. Confidence: the memory's own confidence score
	conf := m.Confidence

	// 6. Access frequency: frequently accessed memories might be more important
	af := accessFreqScore(m.AccessCount)

	// 7. Source trust: user > extraction > tool
	st := sourceTrust(m.Source)

	score := w.KeywordRelevance*kr +
		w.GoalAlignment*ga +
		w.KindBoost*kb +
		w.Recency*rec +
		w.Confidence*conf +
		w.AccessFrequency*af +
		w.SourceTrust*st

	// Clamp to [0, 1]
	if score > 1.0 {
		score = 1.0
	}
	if score < 0.0 {
		score = 0.0
	}

	reason := strings.Join(reasons, ", ")
	if reason == "" {
		reason = "low relevance"
	}
	return score, reason
}

// keywordRelevance computes a keyword overlap score between content and query.
// Uses CJK-aware tokenization (from bm25.go) so Chinese queries produce
// meaningful unigram tokens instead of a single giant string.
// Returns 0.0-1.0.
func keywordRelevance(content, key, query string) float64 {
	if query == "" {
		return 0
	}

	queryTokens := tokenize(query)
	if len(queryTokens) == 0 {
		return 0
	}

	target := strings.ToLower(content + " " + key)
	matches := 0
	for _, tok := range queryTokens {
		if strings.Contains(target, tok) {
			matches++
		}
	}
	return float64(matches) / float64(len(queryTokens))
}

// recencyScore applies exponential decay based on age.
// Half-life of 7 days: a 7-day-old memory scores 0.5.
func recencyScore(updatedAt time.Time) float64 {
	age := time.Since(updatedAt).Hours()
	halfLife := 7.0 * 24.0 // 7 days in hours
	return math.Exp(-0.693 * age / halfLife)
}

// kindBoost returns a boost factor for memory kind vs task type compatibility.
func kindBoost(memKind MemoryKind, taskType TaskType) float64 {
	boosts := map[MemoryKind]map[TaskType]float64{
		MemoryExperience: {TaskTypeGoal: 0.9, TaskTypeWorkflow: 0.8},
		MemoryRule:       {TaskTypeGoal: 0.8, TaskTypeChat: 0.7, TaskTypeWorkflow: 0.9},
		MemoryFact:       {TaskTypeChat: 0.9, TaskTypeGoal: 0.6},
		MemoryPreference: {TaskTypeChat: 0.9, TaskTypeGoal: 0.5},
		MemorySummary:    {TaskTypeGoal: 0.7, TaskTypeChat: 0.5},
	}
	if byTask, ok := boosts[memKind]; ok {
		if v, ok := byTask[taskType]; ok {
			return v
		}
	}
	return 0.3 // default low boost
}

// accessFreqScore normalizes access count to [0, 1] using log scale.
func accessFreqScore(count int) float64 {
	if count <= 0 {
		return 0
	}
	// log(1+count) / log(1+100) ???saturates around 100 accesses
	return math.Log1p(float64(count)) / math.Log1p(100)
}

// sourceTrust returns a trust score based on the memory source.
func sourceTrust(source string) float64 {
	switch source {
	case "user":
		return 1.0
	case "extraction":
		return 0.7
	case "tool":
		return 0.6
	case "llm":
		return 0.5
	default:
		return 0.3
	}
}

// tokenize splits text into words, filtering short tokens.
// tokenizeSimple is the legacy whitespace tokenizer used by keyword scoring.
// For BM25 search, use tokenize() in bm25.go which handles CJK and stop words.
func tokenizeSimple(text string) []string {
	words := strings.Fields(text)
	var result []string
	for _, w := range words {
		w = strings.Trim(w, ".,;:!?()[]{}\"'")
		if len(w) >= 2 {
			result = append(result, w)
		}
	}
	return result
}
