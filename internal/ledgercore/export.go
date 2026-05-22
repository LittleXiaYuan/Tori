package ledger

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"time"
)

// TrainingFormat specifies the output format for exported training data.
type TrainingFormat string

const (
	FormatOpenAIChatML TrainingFormat = "openai_chatml"
	FormatAlpaca       TrainingFormat = "alpaca"
	FormatShareGPT     TrainingFormat = "sharegpt"
)

type ExportConfig struct {
	TenantID    string
	Format      TrainingFormat
	MinScore    float64       // minimum reflection score to consider "successful" (default 0.6)
	MinSteps    int           // skip trivial single-step tasks (default 2)
	After       *time.Time    // only export tasks finished after this time
	Before      *time.Time    // only export tasks finished before this time
	TaskTypes   []TaskType    // filter by task type (nil = all)
	IncludeFail bool          // include failed tasks as negative examples
	MaxSamples  int           // cap total samples (0 = unlimited)
}

func (c *ExportConfig) defaults() {
	if c.MinScore <= 0 {
		c.MinScore = 0.6
	}
	if c.MinSteps <= 0 {
		c.MinSteps = 2
	}
	if c.Format == "" {
		c.Format = FormatOpenAIChatML
	}
}

// TrainingSample is one fine-tuning example.
type TrainingSample struct {
	// OpenAI ChatML format
	Messages []TrainingMessage `json:"messages,omitempty"`

	// Alpaca format
	Instruction string `json:"instruction,omitempty"`
	Input       string `json:"input,omitempty"`
	Output      string `json:"output,omitempty"`

	// Metadata (not sent to trainer, but useful for filtering)
	Meta *SampleMeta `json:"meta,omitempty"`
}

type TrainingMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type SampleMeta struct {
	TaskID   string  `json:"task_id"`
	TaskType string  `json:"task_type"`
	Score    float64 `json:"score"`
	Source   string  `json:"source"` // "reasoning_trace" | "conversation" | "reflection"
}

// ExportResult summarizes the export operation.
type ExportResult struct {
	TotalTasks    int `json:"total_tasks"`
	QualifiedTasks int `json:"qualified_tasks"`
	SamplesWritten int `json:"samples_written"`
	SkippedLowScore int `json:"skipped_low_score"`
	SkippedTrivial  int `json:"skipped_trivial"`
}

// ExportTrainingData extracts successful task traces and writes them as
// fine-tuning samples in JSONL format.
//
// The pipeline:
//  1. List completed tasks matching the filter
//  2. For each task, extract reasoning trace + events
//  3. Convert traces to training samples (system→user→assistant turns)
//  4. Write as JSONL to the provided writer
func (l *Ledger) ExportTrainingData(ctx context.Context, w io.Writer, cfg ExportConfig) (*ExportResult, error) {
	cfg.defaults()
	result := &ExportResult{}
	enc := json.NewEncoder(w)

	statuses := []TaskStatus{TaskCompleted}
	if cfg.IncludeFail {
		statuses = append(statuses, TaskFailed)
	}

	tasks, err := l.backend.ListTasks(ctx, TaskFilter{
		TenantID: cfg.TenantID,
		Status:   statuses,
		Limit:    5000,
	})
	if err != nil {
		return nil, fmt.Errorf("export: list tasks: %w", err)
	}

	for _, t := range tasks {
		if cfg.MaxSamples > 0 && result.SamplesWritten >= cfg.MaxSamples {
			break
		}

		if !l.taskMatchesFilter(t, &cfg) {
			continue
		}
		result.TotalTasks++

		samples, skip := l.extractSamples(ctx, t, &cfg)
		switch skip {
		case skipLowScore:
			result.SkippedLowScore++
			continue
		case skipTrivial:
			result.SkippedTrivial++
			continue
		}
		if len(samples) == 0 {
			continue
		}

		result.QualifiedTasks++
		for _, s := range samples {
			if cfg.MaxSamples > 0 && result.SamplesWritten >= cfg.MaxSamples {
				break
			}
			if err := enc.Encode(s); err != nil {
				return result, fmt.Errorf("export: write sample: %w", err)
			}
			result.SamplesWritten++
		}
	}

	slog.Info("export: complete",
		"total_tasks", result.TotalTasks,
		"qualified", result.QualifiedTasks,
		"samples", result.SamplesWritten,
		"skipped_score", result.SkippedLowScore,
		"skipped_trivial", result.SkippedTrivial,
	)
	return result, nil
}

type skipReason int

const (
	skipNone     skipReason = iota
	skipLowScore
	skipTrivial
)

func (l *Ledger) taskMatchesFilter(t *Task, cfg *ExportConfig) bool {
	if cfg.After != nil && (t.FinishedAt == nil || t.FinishedAt.Before(*cfg.After)) {
		return false
	}
	if cfg.Before != nil && (t.FinishedAt == nil || t.FinishedAt.After(*cfg.Before)) {
		return false
	}
	if len(cfg.TaskTypes) > 0 {
		found := false
		for _, tt := range cfg.TaskTypes {
			if t.Type == tt {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func (l *Ledger) extractSamples(ctx context.Context, t *Task, cfg *ExportConfig) ([]TrainingSample, skipReason) {
	trace, err := l.Events.GetReasoningTrace(ctx, t.ID)
	if err != nil || trace == nil {
		return nil, skipNone
	}

	if trace.Summary.TotalSteps < cfg.MinSteps {
		return nil, skipTrivial
	}
	if trace.Summary.AvgConfidence > 0 && trace.Summary.AvgConfidence < cfg.MinScore {
		return nil, skipLowScore
	}

	var samples []TrainingSample

	sample := l.traceToSample(t, trace, cfg)
	if sample != nil {
		samples = append(samples, *sample)
	}

	reflectionSamples := l.extractReflectionSamples(t, trace, cfg)
	samples = append(samples, reflectionSamples...)

	return samples, skipNone
}

// traceToSample converts a full reasoning trace into a multi-turn training sample.
func (l *Ledger) traceToSample(t *Task, trace *ReasoningTrace, cfg *ExportConfig) *TrainingSample {
	if len(trace.Events) == 0 {
		return nil
	}

	meta := &SampleMeta{
		TaskID:   t.ID,
		TaskType: string(t.Type),
		Score:    trace.Summary.AvgConfidence,
		Source:   "reasoning_trace",
	}

	switch cfg.Format {
	case FormatAlpaca:
		return l.traceToAlpaca(t, trace, meta)
	case FormatShareGPT:
		return l.traceToShareGPT(t, trace, meta)
	default:
		return l.traceToChatML(t, trace, meta)
	}
}

func (l *Ledger) traceToChatML(t *Task, trace *ReasoningTrace, meta *SampleMeta) *TrainingSample {
	msgs := []TrainingMessage{
		{Role: "system", Content: "You are a task execution agent. Think step by step, observe results, and make decisions."},
		{Role: "user", Content: t.Goal},
	}

	var assistantContent string
	for _, e := range trace.Events {
		var p eventPayload
		if err := json.Unmarshal(e.Payload, &p); err != nil {
			continue
		}
		switch e.Kind {
		case EventReasoningThought:
			assistantContent += fmt.Sprintf("[Thought] %s\n", p.Thought)
		case EventReasoningObserve:
			assistantContent += fmt.Sprintf("[Observe] %s\n", p.Observation)
		case EventReasoningDecision:
			assistantContent += fmt.Sprintf("[Decision] %s\n", p.Decision)
		case EventReasoningPlan:
			if len(p.PlanSteps) > 0 {
				assistantContent += "[Plan]\n"
				for i, step := range p.PlanSteps {
					assistantContent += fmt.Sprintf("  %d. %s\n", i+1, step)
				}
			}
		case EventReasoningReflect:
			assistantContent += fmt.Sprintf("[Reflect] %s\n", p.Thought)
		}
	}

	if assistantContent == "" {
		return nil
	}

	output := extractOutput(t)
	if output != "" {
		assistantContent += fmt.Sprintf("\n[Answer] %s\n", output)
	}

	msgs = append(msgs, TrainingMessage{Role: "assistant", Content: assistantContent})

	return &TrainingSample{Messages: msgs, Meta: meta}
}

func (l *Ledger) traceToAlpaca(t *Task, trace *ReasoningTrace, meta *SampleMeta) *TrainingSample {
	var reasoning string
	for _, e := range trace.Events {
		var p eventPayload
		if err := json.Unmarshal(e.Payload, &p); err != nil {
			continue
		}
		if p.Thought != "" {
			reasoning += p.Thought + "\n"
		}
	}
	if reasoning == "" {
		return nil
	}

	return &TrainingSample{
		Instruction: t.Goal,
		Input:       string(t.Input),
		Output:      reasoning + extractOutput(t),
		Meta:        meta,
	}
}

func (l *Ledger) traceToShareGPT(t *Task, trace *ReasoningTrace, meta *SampleMeta) *TrainingSample {
	msgs := []TrainingMessage{
		{Role: "human", Content: t.Goal},
	}

	var assistantContent string
	for _, e := range trace.Events {
		var p eventPayload
		if err := json.Unmarshal(e.Payload, &p); err != nil {
			continue
		}
		if p.Thought != "" {
			assistantContent += p.Thought + "\n"
		}
	}
	if assistantContent == "" {
		return nil
	}

	output := extractOutput(t)
	if output != "" {
		assistantContent += "\n" + output
	}

	msgs = append(msgs, TrainingMessage{Role: "gpt", Content: assistantContent})
	return &TrainingSample{Messages: msgs, Meta: meta}
}

func (l *Ledger) extractReflectionSamples(t *Task, trace *ReasoningTrace, cfg *ExportConfig) []TrainingSample {
	var samples []TrainingSample

	for _, e := range trace.Events {
		if e.Kind != EventReasoningReflect {
			continue
		}
		var p eventPayload
		if err := json.Unmarshal(e.Payload, &p); err != nil {
			continue
		}
		if p.Confidence != nil && *p.Confidence < cfg.MinScore {
			continue
		}
		if p.Thought == "" {
			continue
		}

		meta := &SampleMeta{
			TaskID:   t.ID,
			TaskType: string(t.Type),
			Source:   "reflection",
		}
		if p.Confidence != nil {
			meta.Score = *p.Confidence
		}

		switch cfg.Format {
		case FormatAlpaca:
			samples = append(samples, TrainingSample{
				Instruction: fmt.Sprintf("Reflect on this task: %s", t.Goal),
				Output:      p.Thought,
				Meta:        meta,
			})
		default:
			samples = append(samples, TrainingSample{
				Messages: []TrainingMessage{
					{Role: "system", Content: "You are a reflective agent. Analyze task execution and extract learnings."},
					{Role: "user", Content: fmt.Sprintf("Reflect on this completed task: %s", t.Goal)},
					{Role: "assistant", Content: p.Thought},
				},
				Meta: meta,
			})
		}
	}
	return samples
}

func extractOutput(t *Task) string {
	if len(t.Output) == 0 || string(t.Output) == "null" {
		return ""
	}
	var s string
	if err := json.Unmarshal(t.Output, &s); err == nil {
		return s
	}
	return string(t.Output)
}

// eventPayload is declared in event_apply.go (shared union struct).
