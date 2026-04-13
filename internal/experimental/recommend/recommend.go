// Package recommend provides content-based recommendation for agent responses.
// It learns user preferences from interaction history and recommends response
// styles, skills, and content types that align with the user's profile.
//
// Uses collaborative filtering concepts adapted for single-user agent contexts:
//   - Item-based similarity (skills/topics) using cosine similarity
//   - User profile vector updated incrementally from feedback signals
//   - Thompson Sampling for exploration vs exploitation in recommendations
//
// References:
//   - Linden, Smith, York, "Amazon.com Recommendations", IEEE Internet Computing 2003
//   - Chapelle, Li, "An Empirical Evaluation of Thompson Sampling", NeurIPS 2011
package recommend

import (
	"math"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"
)

// Engine provides personalized recommendations based on user interaction history.
type Engine struct {
	mu       sync.RWMutex
	items    map[string]*ItemProfile
	userPref UserPreference
	rng      *rand.Rand
}

// ItemProfile represents a skill, topic, or response style with usage statistics.
type ItemProfile struct {
	ID          string   `json:"id"`
	Category    string   `json:"category"`
	Tags        []string `json:"tags"`
	Uses        int64    `json:"uses"`
	Successes   int64    `json:"successes"`
	Failures    int64    `json:"failures"`
	AvgRating   float64  `json:"avg_rating"`
	LastUsed    time.Time `json:"last_used"`
	Features    []float64 `json:"features"` // feature vector for similarity
}

// UserPreference tracks accumulated user preference signals.
type UserPreference struct {
	PreferredCategories map[string]float64 `json:"preferred_categories"`
	PreferredTags       map[string]float64 `json:"preferred_tags"`
	AvoidCategories     map[string]float64 `json:"avoid_categories"`
	InteractionCount    int64              `json:"interaction_count"`
}

// Recommendation is a scored suggestion.
type Recommendation struct {
	ItemID     string  `json:"item_id"`
	Score      float64 `json:"score"`
	Reason     string  `json:"reason"`
	Confidence float64 `json:"confidence"`
}

// NewEngine creates a recommendation engine.
func NewEngine() *Engine {
	return &Engine{
		items: make(map[string]*ItemProfile),
		userPref: UserPreference{
			PreferredCategories: make(map[string]float64),
			PreferredTags:       make(map[string]float64),
			AvoidCategories:     make(map[string]float64),
		},
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// RegisterItem adds or updates an item that can be recommended.
func (e *Engine) RegisterItem(item ItemProfile) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.items[item.ID] = &item
}

// RecordOutcome updates an item's statistics and user preferences based on feedback.
// rating is in [0, 1] where 1 = perfect outcome.
func (e *Engine) RecordOutcome(itemID string, rating float64, positive bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	item, exists := e.items[itemID]
	if !exists {
		return
	}

	item.Uses++
	item.LastUsed = time.Now()
	if positive {
		item.Successes++
	} else {
		item.Failures++
	}

	// Exponential moving average for rating
	if item.AvgRating == 0 {
		item.AvgRating = rating
	} else {
		item.AvgRating = item.AvgRating*0.8 + rating*0.2
	}

	e.userPref.InteractionCount++

	// Update preference signals
	weight := rating
	if !positive {
		weight = -0.5
	}

	e.userPref.PreferredCategories[item.Category] += weight
	for _, tag := range item.Tags {
		e.userPref.PreferredTags[tag] += weight
	}
	if !positive {
		e.userPref.AvoidCategories[item.Category] += 0.3
	}
}

// Recommend returns the top-K items ranked by predicted preference.
// Uses a hybrid scoring approach:
//   - Category/tag alignment with user preferences (40%)
//   - Item success rate via Thompson Sampling (30%)
//   - Recency decay (15%)
//   - Novelty bonus for under-explored items (15%)
func (e *Engine) Recommend(k int, context string) []Recommendation {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if len(e.items) == 0 || k <= 0 {
		return nil
	}

	contextTerms := strings.Fields(strings.ToLower(context))

	type scored struct {
		item  *ItemProfile
		score float64
		reason string
	}
	var results []scored

	for _, item := range e.items {
		prefScore := e.preferenceScore(item)
		thompsonScore := e.thompsonScore(item)
		recencyScore := e.recencyScore(item)
		noveltyScore := e.noveltyScore(item)
		contextScore := e.contextMatch(item, contextTerms)

		total := prefScore*0.30 + thompsonScore*0.25 + recencyScore*0.10 +
			noveltyScore*0.15 + contextScore*0.20

		reason := "preference"
		maxPartial := prefScore * 0.30
		if thompsonScore*0.25 > maxPartial {
			reason = "success_rate"
			maxPartial = thompsonScore * 0.25
		}
		if contextScore*0.20 > maxPartial {
			reason = "context_match"
			maxPartial = contextScore * 0.20
		}
		if noveltyScore*0.15 > maxPartial {
			reason = "novelty"
		}

		results = append(results, scored{item, total, reason})
	}

	sort.Slice(results, func(i, j int) bool { return results[i].score > results[j].score })

	if k > len(results) {
		k = len(results)
	}

	recs := make([]Recommendation, k)
	for i := 0; i < k; i++ {
		confidence := 0.5
		if results[i].item.Uses > 10 {
			confidence = 0.8
		} else if results[i].item.Uses > 3 {
			confidence = 0.6
		}
		recs[i] = Recommendation{
			ItemID:     results[i].item.ID,
			Score:      math.Round(results[i].score*1000) / 1000,
			Reason:     results[i].reason,
			Confidence: confidence,
		}
	}
	return recs
}

// SimilarItems finds items most similar to a given item using cosine similarity on features.
func (e *Engine) SimilarItems(itemID string, k int) []Recommendation {
	e.mu.RLock()
	defer e.mu.RUnlock()

	target, exists := e.items[itemID]
	if !exists || len(target.Features) == 0 {
		return nil
	}

	type scored struct {
		id   string
		sim  float64
	}
	var results []scored

	for id, item := range e.items {
		if id == itemID || len(item.Features) == 0 {
			continue
		}
		sim := cosineSimilarity64(target.Features, item.Features)
		results = append(results, scored{id, sim})
	}

	sort.Slice(results, func(i, j int) bool { return results[i].sim > results[j].sim })
	if k > len(results) {
		k = len(results)
	}

	recs := make([]Recommendation, k)
	for i := 0; i < k; i++ {
		recs[i] = Recommendation{
			ItemID:     results[i].id,
			Score:      math.Round(results[i].sim*1000) / 1000,
			Reason:     "similar_to_" + itemID,
			Confidence: 0.7,
		}
	}
	return recs
}

// Preferences returns the current user preference state.
func (e *Engine) Preferences() UserPreference {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.userPref
}

func (e *Engine) preferenceScore(item *ItemProfile) float64 {
	score := 0.0
	if catPref, ok := e.userPref.PreferredCategories[item.Category]; ok {
		score += math.Tanh(catPref / 5.0)
	}
	for _, tag := range item.Tags {
		if tagPref, ok := e.userPref.PreferredTags[tag]; ok {
			score += math.Tanh(tagPref/5.0) * 0.3
		}
	}
	if avoid, ok := e.userPref.AvoidCategories[item.Category]; ok {
		score -= math.Tanh(avoid / 3.0) * 0.5
	}
	return math.Max(0, math.Min(1, (score+1)/2))
}

func (e *Engine) thompsonScore(item *ItemProfile) float64 {
	alpha := float64(item.Successes + 1)
	beta := float64(item.Failures + 1)
	return e.betaSample(alpha, beta)
}

func (e *Engine) recencyScore(item *ItemProfile) float64 {
	if item.LastUsed.IsZero() {
		return 0.3
	}
	hoursSince := time.Since(item.LastUsed).Hours()
	return math.Exp(-hoursSince / (24 * 7)) // 1-week half-life
}

func (e *Engine) noveltyScore(item *ItemProfile) float64 {
	if item.Uses == 0 {
		return 1.0
	}
	return 1.0 / (1.0 + math.Log(float64(item.Uses+1)))
}

func (e *Engine) contextMatch(item *ItemProfile, contextTerms []string) float64 {
	if len(contextTerms) == 0 {
		return 0.5
	}
	matches := 0
	itemText := strings.ToLower(item.ID + " " + item.Category + " " + strings.Join(item.Tags, " "))
	for _, term := range contextTerms {
		if strings.Contains(itemText, term) {
			matches++
		}
	}
	return float64(matches) / float64(len(contextTerms))
}

func (e *Engine) betaSample(alpha, beta float64) float64 {
	x := e.gammaSample(alpha)
	y := e.gammaSample(beta)
	if x+y == 0 {
		return 0.5
	}
	return x / (x + y)
}

func (e *Engine) gammaSample(shape float64) float64 {
	if shape < 1 {
		u := e.rng.Float64()
		return e.gammaSample(shape+1) * math.Pow(u, 1.0/shape)
	}
	d := shape - 1.0/3.0
	c := 1.0 / math.Sqrt(9.0*d)
	for {
		var x, v float64
		for {
			x = e.rng.NormFloat64()
			v = 1.0 + c*x
			if v > 0 {
				break
			}
		}
		v = v * v * v
		u := e.rng.Float64()
		if u < 1.0-0.0331*(x*x)*(x*x) {
			return d * v
		}
		if math.Log(u) < 0.5*x*x+d*(1.0-v+math.Log(v)) {
			return d * v
		}
	}
}

func cosineSimilarity64(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
