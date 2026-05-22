package ledger

import (
	"context"
	"encoding/json"
	"time"

	"yunque-agent/internal/ledgercore/internal/ulid"
)

// ReasoningTracer provides structured methods for recording an agent's
// thought process. Every method appends an immutable event to the Event Store,
// enabling later replay, reflection, experience distillation, and meta-cognition.
//
// Typical usage within a ReAct loop:
//
//	tracer := ldg.Reasoning(taskID)
//	tracer.Observe(ctx, "user wants a Go tutorial", nil)
//	tracer.Think(ctx, "I should check their skill level first", nil)
//	tracer.Decide(ctx, "ask_skill_level", "need to tailor content", 0.8, nil)
//	// ... execute action ...
//	tracer.Observe(ctx, "user is intermediate", nil)
//	tracer.Think(ctx, "focus on concurrency patterns", nil)
type ReasoningTracer struct {
	events *EventStore
	taskID string
	actor  string
	depth  int
}

// Reasoning creates a tracer bound to a specific task.
// The actor identifies who is reasoning (e.g. "planner", "react-loop", "reflect").
func (es *EventStore) Reasoning(taskID, actor string) *ReasoningTracer {
	return &ReasoningTracer{events: es, taskID: taskID, actor: actor}
}

// Reasoning creates a tracer from the top-level Ledger (convenience).
func (l *Ledger) Reasoning(taskID, actor string) *ReasoningTracer {
	return l.Events.Reasoning(taskID, actor)
}

// SetDepth sets the reasoning depth for nested thought trees (e.g. Tree-of-Thought).
func (rt *ReasoningTracer) SetDepth(d int) { rt.depth = d }

// Think records an intermediate thought.
func (rt *ReasoningTracer) Think(ctx context.Context, thought string, meta map[string]interface{}) error {
	p := map[string]interface{}{"thought": thought}
	if rt.depth > 0 {
		p["depth"] = rt.depth
	}
	mergeMeta(p, meta)
	return rt.emit(ctx, EventReasoningThought, p)
}

// Observe records an observation of the environment or tool output.
func (rt *ReasoningTracer) Observe(ctx context.Context, observation string, meta map[string]interface{}) error {
	p := map[string]interface{}{"observation": observation}
	mergeMeta(p, meta)
	return rt.emit(ctx, EventReasoningObserve, p)
}

// Hypothesize records a hypothesis being considered.
func (rt *ReasoningTracer) Hypothesize(ctx context.Context, hypothesis string, confidence float64, meta map[string]interface{}) error {
	p := map[string]interface{}{
		"thought":    hypothesis,
		"confidence": confidence,
	}
	mergeMeta(p, meta)
	return rt.emit(ctx, EventReasoningHypothesis, p)
}

// Decide records a decision (choosing an action or path).
func (rt *ReasoningTracer) Decide(ctx context.Context, decision, reason string, confidence float64, meta map[string]interface{}) error {
	p := map[string]interface{}{
		"decision":   decision,
		"reason":     reason,
		"confidence": confidence,
	}
	mergeMeta(p, meta)
	return rt.emit(ctx, EventReasoningDecision, p)
}

// Backtrack records abandoning a path and switching to an alternative.
func (rt *ReasoningTracer) Backtrack(ctx context.Context, reason, alternative string, meta map[string]interface{}) error {
	p := map[string]interface{}{
		"reason":      reason,
		"alternative": alternative,
	}
	mergeMeta(p, meta)
	return rt.emit(ctx, EventReasoningBacktrack, p)
}

// Plan records a multi-step plan.
func (rt *ReasoningTracer) Plan(ctx context.Context, steps []string, meta map[string]interface{}) error {
	p := map[string]interface{}{"plan_steps": steps}
	mergeMeta(p, meta)
	return rt.emit(ctx, EventReasoningPlan, p)
}

// Reflect records a post-action or post-task reflection.
func (rt *ReasoningTracer) Reflect(ctx context.Context, reflection string, confidence float64, meta map[string]interface{}) error {
	p := map[string]interface{}{
		"thought":    reflection,
		"confidence": confidence,
	}
	mergeMeta(p, meta)
	return rt.emit(ctx, EventReasoningReflect, p)
}

// ConfidenceUpdate records a change in confidence level.
func (rt *ReasoningTracer) ConfidenceUpdate(ctx context.Context, newConf float64, reason string) error {
	return rt.emit(ctx, EventReasoningConfUpdate, map[string]interface{}{
		"confidence": newConf,
		"reason":     reason,
	})
}

// ── Query methods ──

// ReasoningTrace holds the extracted thought process for a task.
type ReasoningTrace struct {
	TaskID   string          `json:"task_id"`
	Events   []*Event        `json:"events"`
	Summary  *TraceSummary   `json:"summary"`
}

// TraceSummary provides aggregate statistics over a reasoning trace.
type TraceSummary struct {
	TotalSteps    int     `json:"total_steps"`
	Thoughts      int     `json:"thoughts"`
	Observations  int     `json:"observations"`
	Decisions     int     `json:"decisions"`
	Backtracks    int     `json:"backtracks"`
	Reflections   int     `json:"reflections"`
	AvgConfidence float64 `json:"avg_confidence"`
	MaxDepth      int     `json:"max_depth"`
}

// reasoningKinds lists all reasoning event kinds for filtering.
var reasoningKinds = map[EventKind]bool{
	EventReasoningThought:    true,
	EventReasoningHypothesis: true,
	EventReasoningDecision:   true,
	EventReasoningBacktrack:  true,
	EventReasoningObserve:    true,
	EventReasoningPlan:       true,
	EventReasoningReflect:    true,
	EventReasoningConfUpdate: true,
}

// IsReasoningEvent returns true if the event kind is a reasoning trace event.
func IsReasoningEvent(kind EventKind) bool {
	return reasoningKinds[kind]
}

// GetReasoningTrace extracts the full reasoning trace for a task.
func (es *EventStore) GetReasoningTrace(ctx context.Context, taskID string) (*ReasoningTrace, error) {
	all, err := es.ListAll(ctx, taskID)
	if err != nil {
		return nil, err
	}

	trace := &ReasoningTrace{TaskID: taskID}
	var confSum float64
	var confCount int

	for _, e := range all {
		if !reasoningKinds[e.Kind] {
			continue
		}
		trace.Events = append(trace.Events, e)
	}

	summary := &TraceSummary{}
	for _, e := range trace.Events {
		summary.TotalSteps++

		switch e.Kind {
		case EventReasoningThought:
			summary.Thoughts++
		case EventReasoningObserve:
			summary.Observations++
		case EventReasoningDecision:
			summary.Decisions++
		case EventReasoningBacktrack:
			summary.Backtracks++
		case EventReasoningReflect:
			summary.Reflections++
		}

		var p eventPayload
		if json.Unmarshal(e.Payload, &p) == nil {
			if p.Confidence != nil {
				confSum += *p.Confidence
				confCount++
			}
			if p.Depth != nil && *p.Depth > summary.MaxDepth {
				summary.MaxDepth = *p.Depth
			}
		}
	}

	if confCount > 0 {
		summary.AvgConfidence = confSum / float64(confCount)
	}
	trace.Summary = summary
	return trace, nil
}

// ── internal ──

func (rt *ReasoningTracer) emit(ctx context.Context, kind EventKind, payload map[string]interface{}) error {
	raw, _ := json.Marshal(payload)
	return rt.events.Append(ctx, &Event{
		ID:        ulid.New(),
		TaskID:    rt.taskID,
		Kind:      kind,
		Actor:     rt.actor,
		Payload:   raw,
		CreatedAt: time.Now(),
	})
}

func mergeMeta(dst map[string]interface{}, meta map[string]interface{}) {
	for k, v := range meta {
		dst[k] = v
	}
}
