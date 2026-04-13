package curiosity

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"ledger"
)

// InterestTracker tracks user interests and skill gaps to generate better exploration questions.
type InterestTracker struct {
	ldg      *ledger.Ledger
	mu       sync.Mutex
	topics   map[string]*TopicInterest // tenantID:topic → interest
	skillGaps map[string][]string      // tenantID → missing skill names
}

// TopicInterest records how interested a user is in a topic.
type TopicInterest struct {
	Topic      string    `json:"topic"`
	TenantID   string    `json:"tenant_id"`
	QueryCount int       `json:"query_count"`
	LastAsked  time.Time `json:"last_asked"`
	Depth      int       `json:"depth"` // 0=surface, 1=explored, 2=deep
}

// NewInterestTracker creates an interest tracker.
func NewInterestTracker(ldg *ledger.Ledger) *InterestTracker {
	return &InterestTracker{
		ldg:       ldg,
		topics:    make(map[string]*TopicInterest),
		skillGaps: make(map[string][]string),
	}
}

// RecordInterest notes that a user asked about a topic.
func (it *InterestTracker) RecordInterest(tenantID, topic string) {
	it.mu.Lock()
	defer it.mu.Unlock()

	key := tenantID + ":" + topic
	if t, ok := it.topics[key]; ok {
		t.QueryCount++
		t.LastAsked = time.Now()
	} else {
		it.topics[key] = &TopicInterest{
			Topic:      topic,
			TenantID:   tenantID,
			QueryCount: 1,
			LastAsked:  time.Now(),
		}
	}
}

// RecordSkillGap notes that a skill was needed but not available.
func (it *InterestTracker) RecordSkillGap(tenantID, skillName string) {
	it.mu.Lock()
	defer it.mu.Unlock()

	gaps := it.skillGaps[tenantID]
	for _, g := range gaps {
		if g == skillName {
			return
		}
	}
	it.skillGaps[tenantID] = append(gaps, skillName)
}

// TopInterests returns the most active user interests.
func (it *InterestTracker) TopInterests(tenantID string, limit int) []TopicInterest {
	it.mu.Lock()
	defer it.mu.Unlock()

	var interests []TopicInterest
	prefix := tenantID + ":"
	for key, topic := range it.topics {
		if strings.HasPrefix(key, prefix) {
			interests = append(interests, *topic)
		}
	}

	// Sort by query count (bubble sort, small N)
	for i := 0; i < len(interests); i++ {
		for j := i + 1; j < len(interests); j++ {
			if interests[j].QueryCount > interests[i].QueryCount {
				interests[i], interests[j] = interests[j], interests[i]
			}
		}
	}

	if len(interests) > limit {
		interests = interests[:limit]
	}
	return interests
}

// SkillGaps returns known skill gaps for a tenant.
func (it *InterestTracker) SkillGaps(tenantID string) []string {
	it.mu.Lock()
	defer it.mu.Unlock()
	return it.skillGaps[tenantID]
}

// GenerateUserInterestQuestions creates exploration questions based on user interests.
func (it *InterestTracker) GenerateUserInterestQuestions(tenantID string, limit int) []Question {
	interests := it.TopInterests(tenantID, limit*2)
	var questions []Question

	for _, interest := range interests {
		if interest.Depth >= 2 {
			continue // already deeply explored
		}
		q := Question{
			Question:  fmt.Sprintf("What new developments or techniques exist for: %s", interest.Topic),
			Category:  UserInterest,
			Priority:  float64(interest.QueryCount) / 20.0,
			Context:   fmt.Sprintf("User asked %d times, last %s ago", interest.QueryCount, time.Since(interest.LastAsked).Round(time.Hour)),
			RelatedTo: []string{interest.TenantID},
		}
		if q.Priority > 1.0 {
			q.Priority = 1.0
		}
		questions = append(questions, q)
		if len(questions) >= limit {
			break
		}
	}
	return questions
}

// GenerateSkillGapQuestions creates exploration questions for missing capabilities.
func (it *InterestTracker) GenerateSkillGapQuestions(tenantID string, limit int) []Question {
	gaps := it.SkillGaps(tenantID)
	var questions []Question

	for _, gap := range gaps {
		questions = append(questions, Question{
			Question:  fmt.Sprintf("How to implement a '%s' skill for the agent?", gap),
			Category:  SkillGap,
			Priority:  0.9, // skill gaps are high priority
			Context:   "User requested a capability the agent doesn't have",
			RelatedTo: []string{gap},
		})
		if len(questions) >= limit {
			break
		}
	}
	return questions
}

// LLMExplorer uses LLM to actually explore and answer curiosity questions.
type LLMExplorer struct {
	llmCall func(ctx context.Context, system, user string) (string, error)
}

// NewLLMExplorer creates an LLM-powered explorer.
func NewLLMExplorer(llmCall func(ctx context.Context, system, user string) (string, error)) *LLMExplorer {
	return &LLMExplorer{llmCall: llmCall}
}

// ExploreQuestion uses LLM to research a curiosity question and extract facts.
func (e *LLMExplorer) ExploreQuestion(ctx context.Context, q Question) (*Result, error) {
	if e.llmCall == nil {
		return nil, fmt.Errorf("no LLM configured")
	}

	system := `You are a knowledge exploration agent. Research the question and extract facts.
Output JSON:
{"findings":["fact1","fact2"],"new_facts":["concise reusable fact"],"confidence":0.0-1.0,"useful":true|false}`

	user := fmt.Sprintf("Question: %s\nCategory: %s\nContext: %s", q.Question, q.Category, q.Context)

	reply, err := e.llmCall(ctx, system, user)
	if err != nil {
		return nil, err
	}

	result := &Result{Question: q.Question}
	if err := json.Unmarshal([]byte(extractJSON(reply)), result); err != nil {
		slog.Warn("curiosity: parse exploration result failed", "err", err)
		result.Findings = []string{reply}
		result.Confidence = 0.3
	}

	return result, nil
}

func extractJSON(s string) string {
	start := -1
	for i, c := range s {
		if c == '{' {
			start = i
			break
		}
	}
	if start < 0 {
		return s
	}
	depth := 0
	for i := start; i < len(s); i++ {
		if s[i] == '{' {
			depth++
		} else if s[i] == '}' {
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return s[start:]
}
