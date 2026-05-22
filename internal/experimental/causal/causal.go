package causal

import (
	"context"
	"fmt"
	"time"

	"yunque-agent/internal/ledgercore"
)

// CausalEngine extends the ContextGraph with causal reasoning capabilities.
type CausalEngine struct {
	ldg *ledger.Ledger
}

// CausalLink represents a directed causal relationship: Cause → Effect.
type CausalLink struct {
	CauseEventID  string           `json:"cause_event_id"`
	EffectEventID string           `json:"effect_event_id"`
	CauseKind     ledger.EventKind `json:"cause_kind"`
	EffectKind    ledger.EventKind `json:"effect_kind"`
	Strength      float64          `json:"strength"`
	Mechanism     string           `json:"mechanism"`
}

// CausalChain is a sequence of causal links forming a chain.
type CausalChain struct {
	Links       []CausalLink `json:"links"`
	RootCause   string       `json:"root_cause"`
	FinalEffect string       `json:"final_effect"`
}

// FailurePattern represents a recurring causal pattern in failed tasks.
type FailurePattern struct {
	CauseKind   ledger.EventKind `json:"cause_kind"`
	EffectKind  ledger.EventKind `json:"effect_kind"`
	Mechanism   string           `json:"mechanism"`
	Occurrences int              `json:"occurrences"`
	TaskIDs     []string         `json:"task_ids"`
}

// TimelineEntry is an event with annotated time gap.
type TimelineEntry struct {
	Event     *ledger.Event `json:"event"`
	GapBefore time.Duration `json:"gap_before"`
}

const EdgeCausedBy ledger.GraphEdgeKind = "caused_by"

// New creates a causal reasoning engine.
func New(ldg *ledger.Ledger) *CausalEngine {
	return &CausalEngine{ldg: ldg}
}

// InferCausality analyzes a task's event stream to find causal relationships.
func (ce *CausalEngine) InferCausality(ctx context.Context, taskID string) ([]CausalLink, error) {
	events, err := ce.ldg.Events.ListAll(ctx, taskID)
	if err != nil {
		return nil, err
	}

	var links []CausalLink

	for i := 1; i < len(events); i++ {
		prev := events[i-1]
		curr := events[i]

		if prev.Kind == ledger.EventReasoningDecision && (curr.Kind == ledger.EventToolInvoked || curr.Kind == ledger.EventStepStarted) {
			links = append(links, CausalLink{
				CauseEventID: prev.ID, EffectEventID: curr.ID,
				CauseKind: prev.Kind, EffectKind: curr.Kind,
				Strength: 0.9, Mechanism: "Decision directly triggered action execution",
			})
		}

		if (prev.Kind == ledger.EventToolFailed || prev.Kind == ledger.EventStepFailed) && curr.Kind == ledger.EventReasoningBacktrack {
			links = append(links, CausalLink{
				CauseEventID: prev.ID, EffectEventID: curr.ID,
				CauseKind: prev.Kind, EffectKind: curr.Kind,
				Strength: 0.95, Mechanism: "Tool failure caused strategy backtrack",
			})
		}

		if prev.Kind == ledger.EventReasoningObserve && curr.Kind == ledger.EventReasoningThought {
			links = append(links, CausalLink{
				CauseEventID: prev.ID, EffectEventID: curr.ID,
				CauseKind: prev.Kind, EffectKind: curr.Kind,
				Strength: 0.7, Mechanism: "Observation informed reasoning",
			})
		}

		if prev.Kind == ledger.EventReasoningBacktrack && curr.Kind == ledger.EventReasoningDecision {
			links = append(links, CausalLink{
				CauseEventID: prev.ID, EffectEventID: curr.ID,
				CauseKind: prev.Kind, EffectKind: curr.Kind,
				Strength: 0.85, Mechanism: "Backtrack led to alternative decision",
			})
		}

		if curr.Kind == ledger.EventTaskFailed {
			for j := i - 1; j >= 0 && j >= i-5; j-- {
				if events[j].Kind == ledger.EventToolFailed || events[j].Kind == ledger.EventStepFailed {
					links = append(links, CausalLink{
						CauseEventID: events[j].ID, EffectEventID: curr.ID,
						CauseKind: events[j].Kind, EffectKind: curr.Kind,
						Strength: 0.8, Mechanism: "Step/tool failure led to task failure",
					})
					break
				}
			}
		}
	}

	for _, link := range links {
		ce.ldg.Graph.Link(ctx,
			&ledger.GraphNode{Kind: ledger.NodeTask, Label: "event:" + link.CauseEventID, RefID: link.CauseEventID, TenantID: taskID},
			&ledger.GraphNode{Kind: ledger.NodeTask, Label: "event:" + link.EffectEventID, RefID: link.EffectEventID, TenantID: taskID},
			EdgeCausedBy, link.Strength)
	}

	return links, nil
}

// FindRootCause traces back from a failure event to find the root cause.
func (ce *CausalEngine) FindRootCause(ctx context.Context, taskID string) (*CausalChain, error) {
	links, err := ce.InferCausality(ctx, taskID)
	if err != nil {
		return nil, err
	}

	events, err := ce.ldg.Events.ListAll(ctx, taskID)
	if err != nil {
		return nil, err
	}

	var failureEvent *ledger.Event
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].Kind == ledger.EventTaskFailed {
			failureEvent = events[i]
			break
		}
	}

	if failureEvent == nil {
		return nil, fmt.Errorf("no failure event found for task %s", taskID)
	}

	chain := &CausalChain{FinalEffect: failureEvent.ID}
	effectID := failureEvent.ID
	visited := make(map[string]bool)

	for {
		if visited[effectID] {
			break
		}
		visited[effectID] = true

		found := false
		for _, link := range links {
			if link.EffectEventID == effectID {
				chain.Links = append([]CausalLink{link}, chain.Links...)
				chain.RootCause = link.CauseEventID
				effectID = link.CauseEventID
				found = true
				break
			}
		}
		if !found {
			break
		}
	}

	return chain, nil
}

// AnalyzeFailurePatterns finds common causal patterns across multiple failed tasks.
func (ce *CausalEngine) AnalyzeFailurePatterns(ctx context.Context, tenantID string, limit int) ([]FailurePattern, error) {
	tasks, err := ce.ldg.Backend().ListTasks(ctx, ledger.TaskFilter{
		TenantID: tenantID,
		Status:   []ledger.TaskStatus{ledger.TaskFailed},
		Limit:    limit,
	})
	if err != nil {
		return nil, err
	}

	patternCounts := make(map[string]*FailurePattern)

	for _, task := range tasks {
		chain, err := ce.FindRootCause(ctx, task.ID)
		if err != nil || chain == nil || len(chain.Links) == 0 {
			continue
		}

		rootLink := chain.Links[0]
		key := string(rootLink.CauseKind) + " → " + string(rootLink.EffectKind)

		if p, ok := patternCounts[key]; ok {
			p.Occurrences++
			p.TaskIDs = append(p.TaskIDs, task.ID)
		} else {
			patternCounts[key] = &FailurePattern{
				CauseKind: rootLink.CauseKind, EffectKind: rootLink.EffectKind,
				Mechanism: rootLink.Mechanism, Occurrences: 1, TaskIDs: []string{task.ID},
			}
		}
	}

	var patterns []FailurePattern
	for _, p := range patternCounts {
		patterns = append(patterns, *p)
	}
	return patterns, nil
}

// BuildTimeline creates an annotated timeline from task events.
func (ce *CausalEngine) BuildTimeline(ctx context.Context, taskID string) ([]TimelineEntry, error) {
	events, err := ce.ldg.Events.ListAll(ctx, taskID)
	if err != nil {
		return nil, err
	}

	var timeline []TimelineEntry
	for i, e := range events {
		entry := TimelineEntry{Event: e}
		if i > 0 {
			entry.GapBefore = e.CreatedAt.Sub(events[i-1].CreatedAt)
		}
		timeline = append(timeline, entry)
	}

	return timeline, nil
}
