package taskdistill

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/LittleXiaYuan/ledger"
)

// AnalyzeFunc is the LLM-powered analysis function.
type AnalyzeFunc func(ctx context.Context, summary TaskEventSummary) (*Result, error)

// TaskEventSummary is the input to the distillation analyzer.
type TaskEventSummary struct {
	TaskID     string            `json:"task_id"`
	Goal       string            `json:"goal"`
	TaskType   ledger.TaskType   `json:"task_type"`
	Status     ledger.TaskStatus `json:"status"`
	TenantID   string            `json:"tenant_id"`
	Duration   time.Duration     `json:"duration"`
	StepCount  int               `json:"step_count"`
	ToolsUsed  []string          `json:"tools_used"`
	Backtracks int               `json:"backtracks"`
	Errors     []string          `json:"errors"`
	Thoughts   []string          `json:"thoughts"`
	Decisions  []string          `json:"decisions"`
	FinalScore float64           `json:"final_score"`
}

// Result is the output of LLM-powered event analysis.
type Result struct {
	Patterns     []Pattern     `json:"patterns"`
	Rules        []Rule        `json:"rules"`
	ToolInsights []ToolInsight `json:"tool_insights"`
}

// Pattern is a reusable strategy extracted from task execution.
type Pattern struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Trigger     string  `json:"trigger"`
	Confidence  float64 `json:"confidence"`
}

// Rule is a behavioral rule derived from experience.
type Rule struct {
	Condition  string  `json:"condition"`
	Action     string  `json:"action"`
	Rationale  string  `json:"rationale"`
	Confidence float64 `json:"confidence"`
}

// ToolInsight captures learned knowledge about tool/skill effectiveness.
type ToolInsight struct {
	ToolName    string  `json:"tool_name"`
	Context     string  `json:"context"`
	Observation string  `json:"observation"`
	Score       float64 `json:"score"`
}

// Distiller extracts reusable patterns and rules from completed tasks.
type Distiller struct {
	ldg       *ledger.Ledger
	analyzeFn AnalyzeFunc
}

// New creates an experience distiller.
func New(ldg *ledger.Ledger) *Distiller {
	return &Distiller{ldg: ldg}
}

// SetAnalyzeFunc sets the LLM-powered analysis function.
func (d *Distiller) SetAnalyzeFunc(fn AnalyzeFunc) { d.analyzeFn = fn }

// BuildTaskSummary extracts a summary from a task's event stream.
func (d *Distiller) BuildTaskSummary(ctx context.Context, taskID string) (*TaskEventSummary, error) {
	task, err := d.ldg.Backend().GetTask(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("get task: %w", err)
	}

	events, err := d.ldg.Events.ListAll(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}

	summary := &TaskEventSummary{
		TaskID:   task.ID,
		Goal:     task.Goal,
		TaskType: task.Type,
		Status:   task.Status,
		TenantID: task.TenantID,
	}

	if task.StartedAt != nil {
		endTime := task.UpdatedAt
		if task.FinishedAt != nil {
			endTime = *task.FinishedAt
		}
		summary.Duration = endTime.Sub(*task.StartedAt)
	}

	toolSet := make(map[string]bool)
	for _, e := range events {
		var p struct {
			Thought   string `json:"thought,omitempty"`
			Decision  string `json:"decision,omitempty"`
			Action    string `json:"action,omitempty"`
			SkillName string `json:"skill_name,omitempty"`
			Error     string `json:"error,omitempty"`
		}
		json.Unmarshal(e.Payload, &p)

		switch {
		case e.Kind == ledger.EventStepStarted || e.Kind == ledger.EventStepCompleted:
			summary.StepCount++
			if p.SkillName != "" {
				toolSet[p.SkillName] = true
			}
		case e.Kind == ledger.EventToolInvoked:
			if p.Action != "" {
				toolSet[p.Action] = true
			}
		case e.Kind == ledger.EventReasoningBacktrack:
			summary.Backtracks++
		case e.Kind == ledger.EventReasoningThought:
			if p.Thought != "" && len(summary.Thoughts) < 10 {
				summary.Thoughts = append(summary.Thoughts, ledger.TruncateStr(p.Thought, 200))
			}
		case e.Kind == ledger.EventReasoningDecision:
			if p.Decision != "" && len(summary.Decisions) < 10 {
				summary.Decisions = append(summary.Decisions, p.Decision)
			}
		case e.Kind == ledger.EventTaskFailed || e.Kind == ledger.EventStepFailed:
			if p.Error != "" && len(summary.Errors) < 5 {
				summary.Errors = append(summary.Errors, ledger.TruncateStr(p.Error, 200))
			}
		}
	}

	for tool := range toolSet {
		summary.ToolsUsed = append(summary.ToolsUsed, tool)
	}

	return summary, nil
}

// DistillTask analyzes a single completed task and writes learnings to Memory.
func (d *Distiller) DistillTask(ctx context.Context, taskID, tenantID string) (*Result, error) {
	summary, err := d.BuildTaskSummary(ctx, taskID)
	if err != nil {
		return nil, err
	}

	if !summary.Status.IsTerminal() {
		return nil, fmt.Errorf("task %s is not terminal (status: %s)", taskID, summary.Status)
	}

	var result *Result

	if d.analyzeFn != nil {
		result, err = d.analyzeFn(ctx, *summary)
		if err != nil {
			return nil, fmt.Errorf("analyze: %w", err)
		}
	} else {
		result = d.heuristicDistill(summary)
	}

	for _, p := range result.Patterns {
		d.ldg.Memory.Put(ctx, &ledger.MemoryEntry{
			TenantID: tenantID, TaskID: &taskID, Kind: ledger.MemoryRule,
			Key: "pattern." + slugify(p.Name), Source: "distillation", Confidence: p.Confidence,
			Content: fmt.Sprintf("[Pattern: %s] %s\nTrigger: %s", p.Name, p.Description, p.Trigger),
		})
	}

	for _, r := range result.Rules {
		d.ldg.Memory.Put(ctx, &ledger.MemoryEntry{
			TenantID: tenantID, TaskID: &taskID, Kind: ledger.MemoryRule,
			Key: "rule." + slugify(r.Condition), Source: "distillation", Confidence: r.Confidence,
			Content: fmt.Sprintf("When %s → %s (because %s)", r.Condition, r.Action, r.Rationale),
		})
	}

	for _, ti := range result.ToolInsights {
		d.ldg.Memory.Put(ctx, &ledger.MemoryEntry{
			TenantID: tenantID, TaskID: &taskID, Kind: ledger.MemoryExperience,
			Key: "tool." + ti.ToolName, Source: "distillation", Confidence: ti.Score,
			Content: fmt.Sprintf("[Tool: %s] %s — %s", ti.ToolName, ti.Context, ti.Observation),
		})
	}

	return result, nil
}

// DistillBatch processes multiple completed tasks.
func (d *Distiller) DistillBatch(ctx context.Context, tenantID string, limit int) (int, error) {
	tasks, err := d.ldg.Backend().ListTasks(ctx, ledger.TaskFilter{
		TenantID: tenantID,
		Status:   []ledger.TaskStatus{ledger.TaskCompleted, ledger.TaskFailed},
		Limit:    limit,
	})
	if err != nil {
		return 0, err
	}

	distilled := 0
	for _, task := range tasks {
		existing, _ := d.ldg.Memory.Search(ctx, ledger.MemoryQuery{
			TenantID: tenantID, TaskID: &task.ID,
			Kinds: []ledger.MemoryKind{ledger.MemoryRule}, Limit: 1,
		})
		if len(existing) > 0 {
			continue
		}
		if _, err := d.DistillTask(ctx, task.ID, tenantID); err == nil {
			distilled++
		}
	}

	return distilled, nil
}

func (d *Distiller) heuristicDistill(s *TaskEventSummary) *Result {
	result := &Result{}

	if s.Backtracks > 0 && s.Status == ledger.TaskCompleted {
		result.Patterns = append(result.Patterns, Pattern{
			Name: "backtrack_recovery", Confidence: 0.6,
			Description: fmt.Sprintf("Task succeeded after %d backtrack(s)", s.Backtracks),
			Trigger:     "When initial approach fails for " + string(s.TaskType) + " tasks",
		})
	}

	if s.Backtracks == 0 && s.StepCount <= 3 && s.Status == ledger.TaskCompleted {
		result.Patterns = append(result.Patterns, Pattern{
			Name: "efficient_execution", Confidence: 0.7,
			Description: "Task completed efficiently in ≤3 steps without backtracking",
			Trigger:     "Similar " + string(s.TaskType) + " tasks with goal: " + ledger.TruncateStr(s.Goal, 100),
		})
	}

	if len(s.Errors) > 0 && s.Status == ledger.TaskCompleted {
		for _, errMsg := range s.Errors {
			result.Rules = append(result.Rules, Rule{
				Condition: "Encountering error: " + ledger.TruncateStr(errMsg, 100),
				Action:    "Try alternative approach (backtrack recovered in this task)",
				Rationale: "Error was overcome during task " + s.TaskID, Confidence: 0.5,
			})
		}
	}

	if s.Status == ledger.TaskFailed {
		result.Rules = append(result.Rules, Rule{
			Condition: "Attempting similar goal: " + ledger.TruncateStr(s.Goal, 100),
			Action:    "Consider a different approach — previous attempt failed",
			Rationale: fmt.Sprintf("Task %s failed after %d steps with %d backtracks", s.TaskID, s.StepCount, s.Backtracks),
			Confidence: 0.6,
		})
	}

	for _, tool := range s.ToolsUsed {
		score := 0.5
		toolCtx := "used during " + string(s.TaskType) + " task"
		obs := "tool was part of the execution"
		if s.Status == ledger.TaskCompleted {
			score = 0.7
			obs = "contributed to successful completion"
		} else {
			score = 0.3
			obs = "was part of a failed execution"
		}
		result.ToolInsights = append(result.ToolInsights, ToolInsight{
			ToolName: tool, Context: toolCtx, Observation: obs, Score: score,
		})
	}

	return result
}

// ToPrompt renders the summary as a prompt for LLM analysis.
func (s *TaskEventSummary) ToPrompt() string {
	var b strings.Builder
	b.WriteString("## Task Analysis\n")
	b.WriteString(fmt.Sprintf("- **Goal**: %s\n", s.Goal))
	b.WriteString(fmt.Sprintf("- **Type**: %s\n", s.TaskType))
	b.WriteString(fmt.Sprintf("- **Outcome**: %s\n", s.Status))
	b.WriteString(fmt.Sprintf("- **Duration**: %s\n", s.Duration.Round(time.Second)))
	b.WriteString(fmt.Sprintf("- **Steps**: %d\n", s.StepCount))
	b.WriteString(fmt.Sprintf("- **Backtracks**: %d\n", s.Backtracks))

	if len(s.ToolsUsed) > 0 {
		b.WriteString(fmt.Sprintf("- **Tools used**: %s\n", strings.Join(s.ToolsUsed, ", ")))
	}
	if len(s.Errors) > 0 {
		b.WriteString("\n### Errors:\n")
		for _, e := range s.Errors {
			b.WriteString(fmt.Sprintf("- %s\n", e))
		}
	}
	if len(s.Thoughts) > 0 {
		b.WriteString("\n### Key thoughts:\n")
		for _, t := range s.Thoughts {
			b.WriteString(fmt.Sprintf("- %s\n", t))
		}
	}
	if len(s.Decisions) > 0 {
		b.WriteString("\n### Key decisions:\n")
		for _, d := range s.Decisions {
			b.WriteString(fmt.Sprintf("- %s\n", d))
		}
	}

	b.WriteString("\n### Instructions\n")
	b.WriteString("Extract reusable patterns, behavioral rules, and tool insights.\n")
	b.WriteString("Return JSON with fields: patterns[], rules[], tool_insights[]\n")

	return b.String()
}

func slugify(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "_")
	r := []rune(s)
	if len(r) > 50 {
		return string(r[:50])
	}
	return s
}
