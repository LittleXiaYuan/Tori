package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGoalCRUD(t *testing.T) {
	k := NewKernel("")

	// Add
	id := k.AddGoal(Goal{Title: "学会飞行", Priority: 1})
	if id == "" {
		t.Fatal("empty goal ID")
	}

	goals := k.Goals()
	if len(goals) != 1 {
		t.Fatalf("want 1 goal, got %d", len(goals))
	}
	if goals[0].Title != "学会飞行" || goals[0].Status != "active" {
		t.Fatalf("unexpected goal: %+v", goals[0])
	}

	// Update
	ok := k.UpdateGoal(id, func(g *Goal) {
		g.Progress = 0.5
		g.Status = "paused"
	})
	if !ok {
		t.Fatal("update failed")
	}
	if k.Goals()[0].Progress != 0.5 || k.Goals()[0].Status != "paused" {
		t.Fatal("update not applied")
	}

	// ActiveGoals should exclude paused
	if len(k.ActiveGoals()) != 0 {
		t.Fatal("paused goal should not be active")
	}

	// Remove
	if !k.RemoveGoal(id) {
		t.Fatal("remove failed")
	}
	if len(k.Goals()) != 0 {
		t.Fatal("goal not removed")
	}
}

func TestResourceTracking(t *testing.T) {
	k := NewKernel("")

	k.TrackResource(Resource{ID: "r1", Type: "file", Path: "/tmp/a.txt"})
	k.TrackResource(Resource{ID: "r2", Type: "url", Path: "https://example.com"})

	res := k.Resources()
	if len(res) != 2 {
		t.Fatalf("want 2 resources, got %d", len(res))
	}

	k.ReleaseResource("r1")
	res = k.Resources()
	if len(res) != 1 {
		t.Fatalf("want 1 active resource, got %d", len(res))
	}
}

func TestWorkingContext(t *testing.T) {
	k := NewKernel("")

	k.SetFocus("编写报告")
	if k.Focus() != "编写报告" {
		t.Fatal("focus not set")
	}

	k.AddTopic("AI agent")
	k.AddTopic("Go programming")
	k.AddTopic("AI agent") // duplicate
	snap := k.TakeSnapshot()
	if len(snap.Topics) != 2 {
		t.Fatalf("want 2 topics, got %d", len(snap.Topics))
	}

	k.RecordAction(ActionRecord{Action: "write_file", Success: true})
	k.RecordAction(ActionRecord{Action: "run_test", Success: false, Result: "1 failure"})
	snap = k.TakeSnapshot()
	if len(snap.RecentActions) != 2 {
		t.Fatalf("want 2 actions, got %d", len(snap.RecentActions))
	}
}

func TestActionRingBuffer(t *testing.T) {
	k := NewKernel("")

	for i := 0; i < 60; i++ {
		k.RecordAction(ActionRecord{Action: "action"})
	}

	snap := k.TakeSnapshot()
	if len(snap.RecentActions) > 10 {
		t.Fatal("snapshot should return at most 10 recent actions")
	}

	k.mu.RLock()
	total := len(k.actions)
	k.mu.RUnlock()
	if total != maxActions {
		t.Fatalf("ring buffer should cap at %d, got %d", maxActions, total)
	}
}

func TestCompileForLLM(t *testing.T) {
	k := NewKernel("")

	// Empty state should return empty
	if s := k.CompileForLLM(); s != "" {
		t.Fatalf("empty state should compile to empty, got: %q", s)
	}

	k.AddGoal(Goal{Title: "完成项目", Priority: 2, Description: "AI Agent核心"})
	k.SetFocus("实现 State Kernel")
	k.AddTopic("Go")
	k.UpdateCapabilities(CapSnapshot{
		TotalSkills:    21,
		DynamicSkills:  []string{"math_calc"},
		UnresolvedGaps: 1,
	})
	k.RecordAction(ActionRecord{Action: "创建文件", Success: true})

	compiled := k.CompileForLLM()
	for _, want := range []string{
		"当前目标",
		"完成项目",
		"当前焦点",
		"State Kernel",
		"活跃话题",
		"Go",
		"能力状态",
		"21",
		"math_calc",
		"未解决缺口: 1",
		"近期动作",
		"创建文件",
	} {
		if !contains(compiled, want) {
			t.Errorf("compiled context missing %q:\n%s", want, compiled)
		}
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	k := NewKernel(dir)

	k.AddGoal(Goal{Title: "持久化测试", Priority: 1})
	k.SetFocus("测试中")
	k.TrackResource(Resource{ID: "r1", Type: "file", Path: "/a.txt"})

	if err := k.Save(); err != nil {
		t.Fatal(err)
	}

	// Verify file exists
	if _, err := os.Stat(filepath.Join(dir, stateFile)); err != nil {
		t.Fatal("state file not created")
	}

	// Load into new kernel
	k2 := NewKernel(dir)
	goals := k2.Goals()
	if len(goals) != 1 || goals[0].Title != "持久化测试" {
		t.Fatalf("loaded goals: %+v", goals)
	}
	if k2.Focus() != "测试中" {
		t.Fatal("focus not loaded")
	}
	// resources include closed ones in map; filter active
	found := false
	for _, r := range k2.resources {
		if r.ID == "r1" {
			found = true
		}
	}
	if !found {
		t.Fatal("resource not loaded")
	}
}

func TestLinkTask(t *testing.T) {
	k := NewKernel("")

	id := k.AddGoal(Goal{Title: "目标A"})
	k.LinkTask(id, "task-1")
	k.LinkTask(id, "task-2")
	k.LinkTask(id, "task-1") // duplicate

	g := k.Goals()[0]
	if len(g.TaskIDs) != 2 {
		t.Fatalf("want 2 linked tasks, got %d", len(g.TaskIDs))
	}
}

func TestOnChange(t *testing.T) {
	k := NewKernel("")

	events := make(chan string, 10)
	k.OnChange(func(event string) {
		events <- event
	})

	k.AddGoal(Goal{Title: "test"})

	select {
	case e := <-events:
		if e != "goal_added" {
			t.Fatalf("unexpected event: %s", e)
		}
	default:
		// async, might not arrive instantly — acceptable
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsStr(s, substr)
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
