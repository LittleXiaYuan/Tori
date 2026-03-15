package trigger

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// ──────────────────────────────────────────────
// ConditionEvaluator Tests
// ──────────────────────────────────────────────

func TestConditionEvaluator_TaskStatus(t *testing.T) {
	ds := &DataSource{
		GetTaskStatus: func(taskID string) (string, error) {
			if taskID == "task-1" {
				return "completed", nil
			}
			return "", fmt.Errorf("not found")
		},
	}
	eval := NewConditionEvaluator(ds)

	// Test: task-1 status eq completed → true
	met, err := eval(context.Background(), &ConditionConfig{
		CheckType: "task_status",
		TargetID:  "task-1",
		Operator:  "eq",
		Value:     "completed",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !met {
		t.Error("expected condition to be met")
	}

	// Test: task-1 status eq running → false
	met, err = eval(context.Background(), &ConditionConfig{
		CheckType: "task_status",
		TargetID:  "task-1",
		Operator:  "eq",
		Value:     "running",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if met {
		t.Error("expected condition to NOT be met")
	}

	// Test: task-1 status neq running → true
	met, err = eval(context.Background(), &ConditionConfig{
		CheckType: "task_status",
		TargetID:  "task-1",
		Operator:  "neq",
		Value:     "running",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !met {
		t.Error("expected condition to be met")
	}

	// Test: unknown task → error
	_, err = eval(context.Background(), &ConditionConfig{
		CheckType: "task_status",
		TargetID:  "task-999",
		Operator:  "eq",
		Value:     "completed",
	})
	if err == nil {
		t.Error("expected error for unknown task")
	}
}

func TestConditionEvaluator_CostThreshold(t *testing.T) {
	ds := &DataSource{
		GetTodayCost: func() float64 { return 5.5 },
		GetMonthCost: func() float64 { return 42.0 },
	}
	eval := NewConditionEvaluator(ds)

	// Test: today's cost > 3.0 → true
	met, err := eval(context.Background(), &ConditionConfig{
		CheckType: "cost_threshold",
		TargetID:  "day",
		Operator:  "gt",
		Value:     "3.0",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !met {
		t.Error("expected cost > 3.0 to be met")
	}

	// Test: today's cost > 10.0 → false
	met, err = eval(context.Background(), &ConditionConfig{
		CheckType: "cost_threshold",
		TargetID:  "day",
		Operator:  "gt",
		Value:     "10.0",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if met {
		t.Error("expected cost > 10.0 to NOT be met")
	}

	// Test: month cost >= 42 → true
	met, err = eval(context.Background(), &ConditionConfig{
		CheckType: "cost_threshold",
		TargetID:  "month",
		Operator:  "gte",
		Value:     "42",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !met {
		t.Error("expected month cost >= 42 to be met")
	}
}

func TestConditionEvaluator_MemoryCount(t *testing.T) {
	ds := &DataSource{
		GetMemoryCount: func(tenantID string) int {
			if tenantID == "tenant-1" {
				return 150
			}
			return 0
		},
	}
	eval := NewConditionEvaluator(ds)

	// Test: memory count > 100 → true
	met, err := eval(context.Background(), &ConditionConfig{
		CheckType: "memory_count",
		TargetID:  "tenant-1",
		Operator:  "gt",
		Value:     "100",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !met {
		t.Error("expected memory count > 100 to be met")
	}

	// Test: memory count > 200 → false
	met, err = eval(context.Background(), &ConditionConfig{
		CheckType: "memory_count",
		TargetID:  "tenant-1",
		Operator:  "gt",
		Value:     "200",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if met {
		t.Error("expected memory count > 200 to NOT be met")
	}
}

func TestConditionEvaluator_Custom(t *testing.T) {
	ds := &DataSource{
		GetCustomValue: func(key string) (string, error) {
			if key == "active_users" {
				return "25", nil
			}
			return "", fmt.Errorf("key not found: %s", key)
		},
	}
	eval := NewConditionEvaluator(ds)

	// Test: active_users > 20 → true
	met, err := eval(context.Background(), &ConditionConfig{
		CheckType: "custom",
		TargetID:  "active_users",
		Operator:  "gt",
		Value:     "20",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !met {
		t.Error("expected custom > 20 to be met")
	}

	// Test: unknown key → error
	_, err = eval(context.Background(), &ConditionConfig{
		CheckType: "custom",
		TargetID:  "unknown_key",
		Operator:  "eq",
		Value:     "1",
	})
	if err == nil {
		t.Error("expected error for unknown key")
	}
}

func TestCompare(t *testing.T) {
	tests := []struct {
		actual, op, expected string
		want                 bool
	}{
		{"hello", "eq", "hello", true},
		{"hello", "neq", "world", true},
		{"hello world", "contains", "world", true},
		{"5.5", "gt", "3.0", true},
		{"5.5", "lt", "3.0", false},
		{"5.5", "gte", "5.5", true},
		{"5.5", "lte", "5.5", true},
		{"10", "gt", "9", true},
		{"abc", "gt", "aaa", true},
	}

	for _, tc := range tests {
		got, err := compare(tc.actual, tc.op, tc.expected)
		if err != nil {
			t.Errorf("compare(%q, %q, %q): unexpected error: %v", tc.actual, tc.op, tc.expected, err)
			continue
		}
		if got != tc.want {
			t.Errorf("compare(%q, %q, %q) = %v, want %v", tc.actual, tc.op, tc.expected, got, tc.want)
		}
	}
}

// ──────────────────────────────────────────────
// Manager Integration Tests
// ──────────────────────────────────────────────

func TestManagerEventEmit(t *testing.T) {
	store := NewStore(t.TempDir())
	executor := NewExecutor(store)

	var createdTaskTitle string
	executor.SetCreateTask(func(ctx context.Context, tenantID, title, desc string) (string, error) {
		createdTaskTitle = title
		return "new-task-1", nil
	})

	// Create cron manager stub (nil-safe because we won't use time triggers)
	mgr := NewManager(store, executor, nil)

	// Create event trigger
	err := store.Create(&TriggerDef{
		Name:     "on-task-fail",
		Type:     TriggerTypeEvent,
		Status:   TriggerStatusActive,
		TenantID: "t1",
		EventConfig: &EventConfig{
			EventType: "task_failed",
		},
		Actions: []TriggerAction{
			{
				Type:            ActionCreateTask,
				TaskTitle:       "Retry failed task",
				TaskDescription: "Auto-retry",
			},
		},
	})
	if err != nil {
		t.Fatalf("create trigger: %v", err)
	}

	// Emit event
	mgr.Emit(context.Background(), EventPayload{
		Event:     EventTaskFailed,
		TenantID:  "t1",
		TaskID:    "failing-task",
		Text:      "task failed",
		Timestamp: time.Now(),
	})

	// Give goroutine time to execute
	time.Sleep(100 * time.Millisecond)

	if createdTaskTitle != "Retry failed task" {
		t.Errorf("expected task created with title 'Retry failed task', got %q", createdTaskTitle)
	}
}

func TestManagerCognitiveEmit(t *testing.T) {
	store := NewStore(t.TempDir())
	executor := NewExecutor(store)

	var sentMessage string
	executor.SetSendMessage(func(ctx context.Context, channelID, threadID, message string) (string, error) {
		sentMessage = message
		return "msg-1", nil
	})

	mgr := NewManager(store, executor, nil)

	// Create cognitive trigger
	err := store.Create(&TriggerDef{
		Name:      "on-high-insight",
		Type:      TriggerTypeCognitive,
		Status:    TriggerStatusActive,
		TenantID:  "t1",
		ChannelID: "telegram",
		CognitiveConfig: &CognitiveConfig{
			SourceType:      "reverie_insight",
			MinSignificance: 0.5,
		},
		Actions: []TriggerAction{
			{
				Type:    ActionSendMessage,
				Message: "High insight detected!",
			},
		},
	})
	if err != nil {
		t.Fatalf("create trigger: %v", err)
	}

	// Emit cognitive event
	mgr.EmitCognitive(context.Background(), "reverie_insight", map[string]any{
		"significance": 0.8,
		"category":     "insight",
	})

	time.Sleep(100 * time.Millisecond)

	if sentMessage != "High insight detected!" {
		t.Errorf("expected message 'High insight detected!', got %q", sentMessage)
	}
}

func TestManagerConditionCheck(t *testing.T) {
	store := NewStore(t.TempDir())
	executor := NewExecutor(store)

	var memoryWritten string
	executor.SetWriteMemory(func(ctx context.Context, tenantID, content string) error {
		memoryWritten = content
		return nil
	})

	mgr := NewManager(store, executor, nil)

	// Set condition evaluator
	mgr.SetConditionEvaluator(NewConditionEvaluator(&DataSource{
		GetTodayCost: func() float64 { return 15.0 },
		GetMonthCost: func() float64 { return 100.0 },
	}))

	// Create condition trigger
	err := store.Create(&TriggerDef{
		Name:     "cost-alert",
		Type:     TriggerTypeCondition,
		Status:   TriggerStatusActive,
		TenantID: "t1",
		ConditionConfig: &ConditionConfig{
			CheckType: "cost_threshold",
			TargetID:  "day",
			Operator:  "gt",
			Value:     "10.0",
		},
		Actions: []TriggerAction{
			{
				Type:          ActionWriteMemory,
				MemoryContent: "Daily cost exceeded $10",
			},
		},
	})
	if err != nil {
		t.Fatalf("create trigger: %v", err)
	}

	// Manually run checkConditions
	mgr.SetConditionEvaluator(NewConditionEvaluator(&DataSource{
		GetTodayCost: func() float64 { return 15.0 },
		GetMonthCost: func() float64 { return 100.0 },
	}))
	mgr.checkConditions()

	time.Sleep(100 * time.Millisecond)

	if memoryWritten != "Daily cost exceeded $10" {
		t.Errorf("expected memory written 'Daily cost exceeded $10', got %q", memoryWritten)
	}
}

func TestManagerBudgetBlock(t *testing.T) {
	store := NewStore(t.TempDir())
	executor := NewExecutor(store)

	callCount := 0
	executor.SetCreateTask(func(ctx context.Context, tenantID, title, desc string) (string, error) {
		callCount++
		return "t-1", nil
	})

	mgr := NewManager(store, executor, nil)

	// Create trigger with exceeded budget
	err := store.Create(&TriggerDef{
		Name:     "budget-blocked",
		Type:     TriggerTypeEvent,
		Status:   TriggerStatusActive,
		TenantID: "t1",
		EventConfig: &EventConfig{
			EventType: "task_completed",
		},
		Actions: []TriggerAction{
			{Type: ActionCreateTask, TaskTitle: "follow-up", TaskDescription: "auto"},
		},
		Budget: &BudgetConfig{
			MaxTotalCost:   1.0,
			CurrentDayCost: 2.0, // exceeded!
		},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	mgr.Emit(context.Background(), EventPayload{
		Event:     EventTaskCompleted,
		TenantID:  "t1",
		Timestamp: time.Now(),
	})

	time.Sleep(100 * time.Millisecond)

	if callCount != 0 {
		t.Errorf("expected 0 task creations (budget blocked), got %d", callCount)
	}

	// Verify a skipped run was recorded
	runs := store.ListRuns("", 10)
	found := false
	for _, r := range runs {
		if r.Status == RunStatusSkipped {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected a skipped run record")
	}
}

func TestExecutorAllActions(t *testing.T) {
	store := NewStore(t.TempDir())
	executor := NewExecutor(store)

	var log []string

	executor.SetCreateTask(func(ctx context.Context, tenantID, title, desc string) (string, error) {
		log = append(log, "create:"+title)
		return "t-1", nil
	})
	executor.SetContinueTask(func(ctx context.Context, taskID, message string) error {
		log = append(log, "continue:"+taskID)
		return nil
	})
	executor.SetSendMessage(func(ctx context.Context, channelID, threadID, message string) (string, error) {
		log = append(log, "send:"+message)
		return "msg-1", nil
	})
	executor.SetCallSkill(func(ctx context.Context, skillName string, args map[string]any) (string, float64, error) {
		log = append(log, "skill:"+skillName)
		return "done", 0.1, nil
	})
	executor.SetWriteMemory(func(ctx context.Context, tenantID, content string) error {
		log = append(log, "memory:"+content)
		return nil
	})
	executor.SetUpdateProfile(func(ctx context.Context, tenantID, key, value string) error {
		log = append(log, "profile:"+key+"="+value)
		return nil
	})

	trigger := &TriggerDef{
		ID:        "multi",
		Name:      "multi action",
		Type:      TriggerTypeEvent,
		Status:    TriggerStatusActive,
		TenantID:  "t1",
		ChannelID: "telegram",
		ThreadID:  "thread-1",
		Actions: []TriggerAction{
			{Type: ActionCreateTask, TaskTitle: "new task", TaskDescription: "auto"},
			{Type: ActionContinueTask, TaskID: "task-99", Message: "go"},
			{Type: ActionSendMessage, Message: "hello"},
			{Type: ActionCallSkill, SkillName: "web_search", SkillArgs: map[string]any{"query": "test"}},
			{Type: ActionWriteMemory, MemoryContent: "important fact"},
			{Type: ActionWriteMemory, ProfileKey: "pref", ProfileValue: "dark mode"},
		},
	}
	store.Create(trigger)

	run, err := executor.Execute(context.Background(), trigger, nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	if run.ActionsExecuted != 6 {
		t.Errorf("expected 6 actions, got %d", run.ActionsExecuted)
	}
	if run.ActionsSucceeded != 6 {
		t.Errorf("expected 6 succeeded, got %d", run.ActionsSucceeded)
	}
	if len(log) != 6 {
		t.Fatalf("expected 6 log entries, got %d: %v", len(log), log)
	}

	expected := []string{
		"create:new task",
		"continue:task-99",
		"send:hello",
		"skill:web_search",
		"memory:important fact",
		"profile:pref=dark mode",
	}
	for i, want := range expected {
		if log[i] != want {
			t.Errorf("log[%d] = %q, want %q", i, log[i], want)
		}
	}
}

func TestStoreRunsAndEvents(t *testing.T) {
	store := NewStore(t.TempDir())

	// Create a trigger
	store.Create(&TriggerDef{
		Name:        "test",
		Type:        TriggerTypeEvent,
		Status:      TriggerStatusActive,
		TenantID:    "t1",
		EventConfig: &EventConfig{EventType: "test"},
		Actions:     []TriggerAction{{Type: ActionLog}},
	})

	// List events (should have "created" event)
	events := store.ListEvents("", 10)
	if len(events) == 0 {
		t.Error("expected at least one event after create")
	}

	// Create runs
	for i := 0; i < 5; i++ {
		store.CreateRun(&TriggerRun{
			TriggerID:   "trg-1",
			TenantID:    "t1",
			Status:      RunStatusCompleted,
			StartedAt:   time.Now().Add(time.Duration(i) * time.Minute),
			TriggerType: TriggerTypeEvent,
		})
	}

	// List runs with limit
	runs := store.ListRuns("trg-1", 3)
	if len(runs) != 3 {
		t.Errorf("expected 3 runs, got %d", len(runs))
	}

	// Verify newest first
	if len(runs) >= 2 && runs[0].StartedAt.Before(runs[1].StartedAt) {
		t.Error("runs should be sorted newest first")
	}
}
