package task

import (
	"encoding/json"
	"testing"
)

// ── TaskConstraints JSON 序列化 ─────────────────────────────

func TestConstraints_JSONRoundTrip(t *testing.T) {
	c := &TaskConstraints{
		MaxSteps:        15,
		TimeoutSec:      600,
		MaxCostUSD:      1.5,
		SuccessCriteria: "所有测试通过",
		TestCommand:     "go test ./...",
		Priority:        "high",
		AutoApprove:     true,
		Tags:            []string{"backend", "urgent"},
		Extra:           map[string]any{"reviewer": "alice"},
	}

	data, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got TaskConstraints
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.MaxSteps != 15 {
		t.Fatalf("max_steps: want 15, got %d", got.MaxSteps)
	}
	if got.TimeoutSec != 600 {
		t.Fatalf("timeout_sec: want 600, got %d", got.TimeoutSec)
	}
	if got.MaxCostUSD != 1.5 {
		t.Fatalf("max_cost_usd: want 1.5, got %f", got.MaxCostUSD)
	}
	if got.SuccessCriteria != "所有测试通过" {
		t.Fatalf("success_criteria mismatch")
	}
	if got.TestCommand != "go test ./..." {
		t.Fatalf("test_command mismatch")
	}
	if got.Priority != "high" {
		t.Fatalf("priority mismatch")
	}
	if !got.AutoApprove {
		t.Fatal("auto_approve should be true")
	}
	if len(got.Tags) != 2 || got.Tags[0] != "backend" {
		t.Fatalf("tags mismatch: %v", got.Tags)
	}
}

func TestConstraints_OmitEmpty(t *testing.T) {
	c := &TaskConstraints{} // all zero-values
	data, _ := json.Marshal(c)
	str := string(data)
	if str != "{}" {
		t.Fatalf("zero-value should marshal to {}, got %s", str)
	}
}

func TestConstraints_ParseFromJSON(t *testing.T) {
	raw := `{"max_steps":10,"timeout_sec":300,"priority":"medium","test_command":"pytest"}`
	var c TaskConstraints
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if c.MaxSteps != 10 {
		t.Fatalf("expected max_steps=10, got %d", c.MaxSteps)
	}
	if c.TimeoutSec != 300 {
		t.Fatalf("expected timeout_sec=300, got %d", c.TimeoutSec)
	}
	if c.Priority != "medium" {
		t.Fatalf("expected priority=medium, got %s", c.Priority)
	}
}

// ── Task with Constraints 持久化 ────────────────────────────

func TestCreateTaskWithConstraints(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	constraints := &TaskConstraints{
		MaxSteps:        12,
		TimeoutSec:      180,
		SuccessCriteria: "编译通过且无 lint 警告",
		TestCommand:     "make test",
		Priority:        "high",
	}

	task, err := s.Create(CreateRequest{
		Description: "重构认证模块",
		Constraints: constraints,
	})
	if err != nil {
		t.Fatal(err)
	}

	if task.Constraints == nil {
		t.Fatal("constraints should be set")
	}
	if task.Constraints.MaxSteps != 12 {
		t.Fatalf("max_steps: want 12, got %d", task.Constraints.MaxSteps)
	}
	if task.Constraints.TimeoutSec != 180 {
		t.Fatalf("timeout_sec: want 180, got %d", task.Constraints.TimeoutSec)
	}
	if task.Constraints.SuccessCriteria != "编译通过且无 lint 警告" {
		t.Fatal("success_criteria mismatch")
	}
	if task.Constraints.TestCommand != "make test" {
		t.Fatal("test_command mismatch")
	}

	// Verify persisted (Get returns stored copy)
	got, ok := s.Get(task.ID)
	if !ok {
		t.Fatal("task not found")
	}
	if got.Constraints == nil {
		t.Fatal("persisted constraints should not be nil")
	}
	if got.Constraints.Priority != "high" {
		t.Fatalf("persisted priority: want high, got %s", got.Constraints.Priority)
	}
}

func TestCreateTaskWithoutConstraints(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	task, err := s.Create(CreateRequest{Description: "简单任务"})
	if err != nil {
		t.Fatal(err)
	}
	if task.Constraints != nil {
		t.Fatal("constraints should be nil when not provided")
	}
}

// ── Task JSON 含 Constraints ────────────────────────────────

func TestTaskJSON_WithConstraints(t *testing.T) {
	task := &Task{
		ID:          "tsk-1",
		Title:       "test",
		Description: "desc",
		Constraints: &TaskConstraints{
			MaxSteps:   5,
			TimeoutSec: 60,
			Tags:       []string{"ci"},
		},
	}

	data, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got Task
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Constraints == nil {
		t.Fatal("constraints should survive roundtrip")
	}
	if got.Constraints.MaxSteps != 5 {
		t.Fatalf("max_steps roundtrip: want 5, got %d", got.Constraints.MaxSteps)
	}
	if len(got.Constraints.Tags) != 1 || got.Constraints.Tags[0] != "ci" {
		t.Fatalf("tags roundtrip mismatch")
	}
}

func TestTaskJSON_WithoutConstraints(t *testing.T) {
	task := &Task{
		ID: "tsk-2",
	}

	data, _ := json.Marshal(task)
	str := string(data)

	// constraints 字段应被 omitempty 省略
	if json.Valid(data) {
		var m map[string]any
		json.Unmarshal(data, &m)
		if _, exists := m["constraints"]; exists {
			t.Fatalf("constraints should be omitted when nil, got %s", str)
		}
	}
}
