package ledger

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

// RecallFeedback records whether a recalled memory was actually useful.
type RecallFeedback struct {
	MemoryID string  `json:"memory_id"`
	TaskID   string  `json:"task_id"`
	Helpful  bool    `json:"helpful"`
	Score    float64 `json:"score"` // 0-1 how helpful
}

// AdaptiveRecall wraps the RecallEngine with feedback-driven weight adaptation.
type AdaptiveRecall struct {
	engine  *RecallEngine
	backend Backend
	memory  *MemoryStore

	mu       sync.Mutex
	feedback []recallFeedbackEntry
}

type recallFeedbackEntry struct {
	feedback RecallFeedback
	query    RecallQuery
	weights  ScoreWeights
	at       time.Time
}

// NewAdaptiveRecall creates an adaptive recall wrapper.
func NewAdaptiveRecall(engine *RecallEngine, backend Backend, memory *MemoryStore) *AdaptiveRecall {
	return &AdaptiveRecall{
		engine:  engine,
		backend: backend,
		memory:  memory,
	}
}

// RecordFeedback records whether recalled memories were helpful.
func (ar *AdaptiveRecall) RecordFeedback(ctx context.Context, fb RecallFeedback, query RecallQuery) {
	ar.mu.Lock()
	ar.feedback = append(ar.feedback, recallFeedbackEntry{
		feedback: fb,
		query:    query,
		weights:  ar.engine.weights,
		at:       time.Now(),
	})
	ar.mu.Unlock()

	// Emit as memory event for persistence
	if fb.TaskID != "" {
		payload := MakePayload(map[string]interface{}{
			"memory_id": fb.MemoryID,
			"helpful":   fb.Helpful,
			"score":     fb.Score,
		})
		ar.engine.backend.AppendEvent(ctx, &Event{
			TaskID:    fb.TaskID,
			Kind:      EventMemoryRecalled,
			Actor:     "recall_feedback",
			Payload:   payload,
			CreatedAt: time.Now(),
		})
	}
}

// AdaptWeights adjusts recall scoring weights based on accumulated feedback.
// Uses a simple gradient-like approach: if memories with high keyword scores
// tend to be helpful, increase keyword weight; if not, decrease it.
//
// Returns the new weights and the number of feedback entries used.
func (ar *AdaptiveRecall) AdaptWeights(ctx context.Context, minSamples int) (ScoreWeights, int) {
	ar.mu.Lock()
	entries := make([]recallFeedbackEntry, len(ar.feedback))
	copy(entries, ar.feedback)
	ar.mu.Unlock()

	if len(entries) < minSamples {
		return ar.engine.weights, 0
	}

	// Analyze which weight dimensions correlate with helpfulness
	w := ar.engine.weights
	lr := 0.05 // learning rate

	var helpfulScores, unhelpfulScores [7]float64
	var helpfulCount, unhelpfulCount float64

	for _, e := range entries {
		mem, err := ar.backend.GetMemory(ctx, e.feedback.MemoryID)
		if err != nil || mem == nil {
			continue
		}

		// Compute individual factor scores for this memory against the query
		kr := keywordRelevance(mem.Content, mem.Key, e.query.Query)
		ga := keywordRelevance(mem.Content, mem.Key, e.query.TaskGoal)
		kb := kindBoost(mem.Kind, e.query.TaskType)
		rec := recencyScore(mem.UpdatedAt)
		conf := mem.Confidence
		af := accessFreqScore(mem.AccessCount)
		st := sourceTrust(mem.Source)

		scores := [7]float64{kr, ga, kb, rec, conf, af, st}

		if e.feedback.Helpful {
			for i := range scores {
				helpfulScores[i] += scores[i]
			}
			helpfulCount++
		} else {
			for i := range scores {
				unhelpfulScores[i] += scores[i]
			}
			unhelpfulCount++
		}
	}

	if helpfulCount == 0 || unhelpfulCount == 0 {
		return w, len(entries)
	}

	// Average scores for helpful vs unhelpful
	for i := range helpfulScores {
		helpfulScores[i] /= helpfulCount
		unhelpfulScores[i] /= unhelpfulCount
	}

	// Adjust weights: increase dimensions where helpful > unhelpful
	adjustments := [7]float64{}
	for i := range adjustments {
		diff := helpfulScores[i] - unhelpfulScores[i]
		adjustments[i] = lr * diff
	}

	w.KeywordRelevance = clamp(w.KeywordRelevance+adjustments[0], 0.05, 0.5)
	w.GoalAlignment = clamp(w.GoalAlignment+adjustments[1], 0.05, 0.4)
	w.KindBoost = clamp(w.KindBoost+adjustments[2], 0.02, 0.3)
	w.Recency = clamp(w.Recency+adjustments[3], 0.02, 0.3)
	w.Confidence = clamp(w.Confidence+adjustments[4], 0.02, 0.3)
	w.AccessFrequency = clamp(w.AccessFrequency+adjustments[5], 0.01, 0.2)
	w.SourceTrust = clamp(w.SourceTrust+adjustments[6], 0.02, 0.3)

	// Normalize to sum ???1.0
	total := w.KeywordRelevance + w.GoalAlignment + w.KindBoost +
		w.Recency + w.Confidence + w.AccessFrequency + w.SourceTrust
	if total > 0 {
		w.KeywordRelevance /= total
		w.GoalAlignment /= total
		w.KindBoost /= total
		w.Recency /= total
		w.Confidence /= total
		w.AccessFrequency /= total
		w.SourceTrust /= total
	}

	// Apply new weights
	ar.engine.weights = w

	return w, len(entries)
}

// PersistWeights saves the current weights to Memory for cross-session persistence.
func (ar *AdaptiveRecall) PersistWeights(ctx context.Context, tenantID string) error {
	data, _ := json.Marshal(ar.engine.weights)
	return ar.memory.Put(ctx, &MemoryEntry{
		TenantID:   tenantID,
		Kind:       MemoryRule,
		Key:        "recall.weights",
		Content:    string(data),
		Source:     "adaptation",
		Confidence: 0.9,
	})
}

// LoadWeights restores persisted weights from Memory.
func (ar *AdaptiveRecall) LoadWeights(ctx context.Context, tenantID string) error {
	results, err := ar.memory.Search(ctx, MemoryQuery{
		TenantID: tenantID,
		Kinds:    []MemoryKind{MemoryRule},
		Limit:    100,
	})
	if err != nil {
		return err
	}

	for _, m := range results {
		if m.Key == "recall.weights" {
			var w ScoreWeights
			if json.Unmarshal([]byte(m.Content), &w) == nil {
				ar.engine.weights = w
				return nil
			}
		}
	}
	return nil
}

// CurrentWeights returns the current scoring weights.
func (ar *AdaptiveRecall) CurrentWeights() ScoreWeights {
	return ar.engine.weights
}

// FeedbackCount returns the number of accumulated feedback entries.
func (ar *AdaptiveRecall) FeedbackCount() int {
	ar.mu.Lock()
	defer ar.mu.Unlock()
	return len(ar.feedback)
}

// ClearFeedback removes all accumulated feedback.
func (ar *AdaptiveRecall) ClearFeedback() {
	ar.mu.Lock()
	ar.feedback = nil
	ar.mu.Unlock()
}
