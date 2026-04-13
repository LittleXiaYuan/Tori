package curiosity

import (
	"context"
	"fmt"
	"time"

	"github.com/LittleXiaYuan/ledger"
)

// Module drives autonomous exploration when the agent is idle.
type Module struct {
	ldg         *ledger.Ledger
	exploreFn   ExploreFunc
	cooldown    time.Duration
	lastExplore time.Time
}

// ExploreFunc is called to execute an exploration task.
type ExploreFunc func(ctx context.Context, question Question) (*Result, error)

// Question is a question the curiosity module wants to explore.
type Question struct {
	Question  string   `json:"question"`
	Category  Category `json:"category"`
	Priority  float64  `json:"priority"`
	Context   string   `json:"context"`
	RelatedTo []string `json:"related_to"`
}

// Category classifies the type of exploration.
type Category string

const (
	KnowledgeGap  Category = "knowledge_gap"
	FailureReview Category = "failure_review"
	SkillGap      Category = "skill_gap"
	UserInterest  Category = "user_interest"
	WeakMemory    Category = "weak_memory"
)

// Result is the output of an exploration task.
type Result struct {
	Question   string   `json:"question"`
	Findings   []string `json:"findings"`
	NewFacts   []string `json:"new_facts"`
	Confidence float64  `json:"confidence"`
	Useful     bool     `json:"useful"`
}

// New creates a curiosity module.
func New(ldg *ledger.Ledger) *Module {
	return &Module{ldg: ldg, cooldown: 1 * time.Hour}
}

// SetExploreFn sets the exploration execution function.
func (cm *Module) SetExploreFn(fn ExploreFunc) { cm.exploreFn = fn }

// SetCooldown sets the minimum time between exploration runs.
func (cm *Module) SetCooldown(d time.Duration) { cm.cooldown = d }

// ShouldExplore returns true if conditions are met for exploration.
func (cm *Module) ShouldExplore(ctx context.Context, tenantID string) bool {
	if time.Since(cm.lastExplore) < cm.cooldown {
		return false
	}
	running, _ := cm.ldg.Backend().ListTasks(ctx, ledger.TaskFilter{
		TenantID: tenantID,
		Status:   []ledger.TaskStatus{ledger.TaskRunning},
		Limit:    1,
	})
	return len(running) == 0
}

// GenerateQuestions identifies what to explore based on current knowledge state.
func (cm *Module) GenerateQuestions(ctx context.Context, tenantID string, limit int) ([]Question, error) {
	var questions []Question

	gapQuestions, _ := cm.findKnowledgeGaps(ctx, tenantID, limit/3)
	questions = append(questions, gapQuestions...)

	failQuestions, _ := cm.findFailuresToReview(ctx, tenantID, limit/3)
	questions = append(questions, failQuestions...)

	weakQuestions, _ := cm.findWeakMemories(ctx, tenantID, limit/3)
	questions = append(questions, weakQuestions...)

	sortByPriority(questions)
	if len(questions) > limit {
		questions = questions[:limit]
	}

	return questions, nil
}

// Explore runs a single exploration cycle.
func (cm *Module) Explore(ctx context.Context, tenantID string) ([]Result, error) {
	if cm.exploreFn == nil {
		return nil, nil
	}

	questions, err := cm.GenerateQuestions(ctx, tenantID, 3)
	if err != nil {
		return nil, err
	}

	var results []Result
	for _, q := range questions {
		result, err := cm.exploreFn(ctx, q)
		if err != nil {
			continue
		}

		if result.Useful {
			for _, fact := range result.NewFacts {
				cm.ldg.Memory.Put(ctx, &ledger.MemoryEntry{
					TenantID:   tenantID,
					Kind:       ledger.MemoryFact,
					Key:        "curiosity." + slugify(q.Question),
					Content:    fact,
					Source:     "curiosity",
					Confidence: result.Confidence,
				})
			}
		}

		results = append(results, *result)
	}

	cm.lastExplore = time.Now()
	return results, nil
}

func (cm *Module) findKnowledgeGaps(ctx context.Context, tenantID string, limit int) ([]Question, error) {
	memories, err := cm.ldg.Memory.Search(ctx, ledger.MemoryQuery{
		TenantID: tenantID,
		Kinds:    []ledger.MemoryKind{ledger.MemoryFact},
		Limit:    50,
	})
	if err != nil {
		return nil, err
	}

	var questions []Question
	for _, m := range memories {
		if m.AccessCount <= 1 && m.Confidence < 0.7 {
			questions = append(questions, Question{
				Question:  "What more can we learn about: " + ledger.TruncateStr(m.Content, 100),
				Category:  KnowledgeGap,
				Priority:  0.5 + (1.0-m.Confidence)*0.3,
				Context:   "Low-confidence fact with few accesses",
				RelatedTo: []string{m.ID},
			})
			if len(questions) >= limit {
				break
			}
		}
	}
	return questions, nil
}

func (cm *Module) findFailuresToReview(ctx context.Context, tenantID string, limit int) ([]Question, error) {
	failed, err := cm.ldg.Backend().ListTasks(ctx, ledger.TaskFilter{
		TenantID: tenantID,
		Status:   []ledger.TaskStatus{ledger.TaskFailed},
		Limit:    10,
	})
	if err != nil {
		return nil, err
	}

	var questions []Question
	for _, task := range failed {
		existing, _ := cm.ldg.Memory.Search(ctx, ledger.MemoryQuery{
			TenantID: tenantID,
			TaskID:   &task.ID,
			Kinds:    []ledger.MemoryKind{ledger.MemoryExperience},
			Limit:    1,
		})
		if len(existing) > 0 {
			continue
		}

		questions = append(questions, Question{
			Question:  "Why did this task fail: " + ledger.TruncateStr(task.Goal, 100),
			Category:  FailureReview,
			Priority:  0.8,
			Context:   "Failed task hasn't been analyzed",
			RelatedTo: []string{task.ID},
		})
		if len(questions) >= limit {
			break
		}
	}
	return questions, nil
}

func (cm *Module) findWeakMemories(ctx context.Context, tenantID string, limit int) ([]Question, error) {
	memories, err := cm.ldg.Memory.Search(ctx, ledger.MemoryQuery{
		TenantID: tenantID,
		Limit:    100,
	})
	if err != nil {
		return nil, err
	}

	var questions []Question
	for _, m := range memories {
		if m.Confidence < 0.4 && m.Kind != ledger.MemoryPreference {
			questions = append(questions, Question{
				Question:  "Verify: " + ledger.TruncateStr(m.Content, 100),
				Category:  WeakMemory,
				Priority:  0.6,
				Context:   "Low confidence (" + fmt.Sprintf("%.2f", m.Confidence) + "), needs verification",
				RelatedTo: []string{m.ID},
			})
			if len(questions) >= limit {
				break
			}
		}
	}
	return questions, nil
}

func sortByPriority(questions []Question) {
	for i := 0; i < len(questions); i++ {
		for j := i + 1; j < len(questions); j++ {
			if questions[j].Priority > questions[i].Priority {
				questions[i], questions[j] = questions[j], questions[i]
			}
		}
	}
}

func slugify(s string) string {
	r := []rune(s)
	if len(r) > 50 {
		s = string(r[:50])
	}
	return s
}
