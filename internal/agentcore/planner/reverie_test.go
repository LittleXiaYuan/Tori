package planner

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestParseThought_Valid(t *testing.T) {
	raw := `{"content":"用户喜欢简洁的回答","category":"insight","significance":0.8,"trigger":"recent_conversations"}`
	th, err := parseThought(raw)
	if err != nil {
		t.Fatal(err)
	}
	if th.Content != "用户喜欢简洁的回答" {
		t.Errorf("content = %q", th.Content)
	}
	if th.Category != "insight" {
		t.Errorf("category = %q", th.Category)
	}
	if th.Significance != 0.8 {
		t.Errorf("significance = %f", th.Significance)
	}
}

func TestParseThought_CodeBlock(t *testing.T) {
	raw := "```json\n{\"content\":\"test\",\"category\":\"idea\",\"significance\":0.5,\"trigger\":\"x\"}\n```"
	th, err := parseThought(raw)
	if err != nil {
		t.Fatal(err)
	}
	if th.Content != "test" || th.Category != "idea" {
		t.Errorf("got %+v", th)
	}
}

func TestParseThought_InvalidCategory(t *testing.T) {
	raw := `{"content":"hello","category":"unknown","significance":0.3,"trigger":"test"}`
	th, err := parseThought(raw)
	if err != nil {
		t.Fatal(err)
	}
	if th.Category != "observation" {
		t.Errorf("expected fallback to observation, got %q", th.Category)
	}
}

func TestParseThought_ClampSignificance(t *testing.T) {
	raw := `{"content":"hello","category":"idea","significance":1.5,"trigger":"test"}`
	th, err := parseThought(raw)
	if err != nil {
		t.Fatal(err)
	}
	if th.Significance != 1.0 {
		t.Errorf("expected 1.0, got %f", th.Significance)
	}
}

func TestThink_GeneratesThought(t *testing.T) {
	cfg := DefaultReverieConfig()
	cfg.SaveFile = "" // no persistence
	cfg.QuietStart = 0
	cfg.QuietEnd = 0 // disable quiet hours for test
	r := NewReverie(cfg)

	r.SetLLMCall(func(ctx context.Context, system, user string) (string, error) {
		return `{"content":"用户最近在学Go","category":"observation","significance":0.7,"trigger":"periodic"}`, nil
	})
	r.SetRecall(func(query string) string {
		return "用户在讨论Go并发模式"
	})

	delivered := make(chan Thought, 1)
	r.SetDeliver(func(th Thought) {
		delivered <- th
	})

	th, err := r.Think(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if th.Content != "用户最近在学Go" {
		t.Errorf("content = %q", th.Content)
	}

	// Should be delivered (significance 0.7 >= 0.6)
	select {
	case d := <-delivered:
		if d.Content != th.Content {
			t.Errorf("delivered content mismatch")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("expected delivery but got none")
	}

	// Journal should have the thought
	journal := r.Journal()
	if len(journal) != 1 {
		t.Errorf("journal len = %d", len(journal))
	}
}

func TestThink_LowSignificance_NoDelivery(t *testing.T) {
	cfg := DefaultReverieConfig()
	cfg.SaveFile = ""
	r := NewReverie(cfg)

	r.SetLLMCall(func(ctx context.Context, system, user string) (string, error) {
		return `{"content":"没什么特别的","category":"observation","significance":0.2,"trigger":"periodic"}`, nil
	})

	delivered := false
	r.SetDeliver(func(th Thought) {
		delivered = true
	})

	th, err := r.Think(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if th.Significance != 0.2 {
		t.Errorf("significance = %f", th.Significance)
	}
	if delivered {
		t.Error("should NOT deliver low-significance thought")
	}
}

func TestJournalContext(t *testing.T) {
	cfg := DefaultReverieConfig()
	cfg.SaveFile = ""
	r := NewReverie(cfg)

	// Empty journal
	if ctx := r.JournalContext(5, ""); ctx != "" {
		t.Errorf("expected empty, got %q", ctx)
	}

	// Add thoughts manually with high significance (no-query mode requires ≥0.8)
	r.mu.Lock()
	r.journal = append(r.journal, Thought{Content: "想法1", Category: "insight", Significance: 0.9, CreatedAt: time.Now()})
	r.journal = append(r.journal, Thought{Content: "想法2", Category: "idea", Significance: 0.85, CreatedAt: time.Now()})
	r.mu.Unlock()

	ctx := r.JournalContext(5, "")
	if ctx == "" {
		t.Error("expected non-empty context")
	}
	if !contains(ctx, "insight") || !contains(ctx, "idea") {
		t.Errorf("context missing categories: %q", ctx)
	}
}

func TestJournalContext_Limit(t *testing.T) {
	cfg := DefaultReverieConfig()
	cfg.SaveFile = ""
	r := NewReverie(cfg)

	r.mu.Lock()
	for i := 0; i < 10; i++ {
		// Use insight category with high significance to pass no-query filter (≥0.8)
		r.journal = append(r.journal, Thought{Content: "thought", Category: "insight", Significance: 0.9, CreatedAt: time.Now()})
	}
	r.mu.Unlock()

	ctx := r.JournalContext(3, "")
	// Should only contain 3 thoughts (capped by maxThoughts — but 2 is the new default limit)
	count := 0
	for _, line := range splitLines(ctx) {
		if len(line) > 0 && line[0] == '-' {
			count++
		}
	}
	if count != 3 {
		t.Errorf("expected 3 thought lines, got %d", count)
	}
}

func TestJournalContext_QueryAware(t *testing.T) {
	cfg := DefaultReverieConfig()
	cfg.SaveFile = ""
	r := NewReverie(cfg)

	r.mu.Lock()
	r.journal = append(r.journal, Thought{Content: "Go语言并发编程技巧", Category: "insight", Significance: 0.7})
	r.journal = append(r.journal, Thought{Content: "用户喜欢简洁的回复风格", Category: "insight", Significance: 0.75})
	r.journal = append(r.journal, Thought{Content: "Python数据分析常见错误", Category: "observation", Significance: 0.7})
	r.mu.Unlock()

	// Query about Go should surface Go-related thought first
	ctx := r.JournalContext(1, "Go语言协程怎么用")
	if !contains(ctx, "Go") {
		t.Errorf("query-aware context should prioritize Go-related thought, got: %q", ctx)
	}
}

func TestReverie_Persistence(t *testing.T) {
	tmp := t.TempDir() + "/reverie.json"
	cfg := DefaultReverieConfig()
	cfg.SaveFile = tmp

	r := NewReverie(cfg)
	r.SetLLMCall(func(ctx context.Context, system, user string) (string, error) {
		return `{"content":"persist me","category":"idea","significance":0.3,"trigger":"test"}`, nil
	})

	if _, err := r.Think(context.Background()); err != nil {
		t.Fatal(err)
	}

	// Verify file exists
	if _, err := os.Stat(tmp); err != nil {
		t.Fatalf("save file not created: %v", err)
	}

	// Load into new instance
	r2 := NewReverie(cfg)
	journal := r2.Journal()
	if len(journal) != 1 || journal[0].Content != "persist me" {
		t.Errorf("persistence failed: %+v", journal)
	}
}

func TestIsQuietHours(t *testing.T) {
	cfg := DefaultReverieConfig()
	cfg.SaveFile = ""
	cfg.QuietStart = 22
	cfg.QuietEnd = 7
	r := NewReverie(cfg)

	// We can't control time.Now() easily, so just verify the method doesn't panic
	_ = r.isQuietHours()
}

func TestTruncateStr(t *testing.T) {
	if got := truncateStr("hello", 10); got != "hello" {
		t.Errorf("got %q", got)
	}
	if got := truncateStr("这是一段很长的中文文本", 5); got != "这是一段很..." {
		t.Errorf("got %q", got)
	}
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// ─── P4: Action Tests ──────────────────────────────────────

func TestParseThought_WithActions(t *testing.T) {
	raw := `{"content":"用户经常提到Go并发","category":"insight","significance":0.9,"trigger":"periodic","actions":[{"type":"write_memory","key":"用户对Go并发很感兴趣"},{"type":"create_task","key":"整理Go并发学习路径","value":"为用户创建关于goroutine/channel/select的学习路径任务"}]}`
	th, err := parseThought(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(th.Actions) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(th.Actions))
	}
	if th.Actions[0].Type != "write_memory" {
		t.Errorf("action[0].Type = %q", th.Actions[0].Type)
	}
	if th.Actions[1].Type != "create_task" || th.Actions[1].Key != "整理Go并发学习路径" {
		t.Errorf("action[1] = %+v", th.Actions[1])
	}
}

func TestParseThought_InvalidActionsFiltered(t *testing.T) {
	raw := `{"content":"test","category":"idea","significance":0.5,"trigger":"x","actions":[{"type":"delete_all","key":"bad"},{"type":"write_memory","key":""},{"type":"write_memory","key":"valid fact"}]}`
	th, err := parseThought(raw)
	if err != nil {
		t.Fatal(err)
	}
	// Only the last action is valid (valid type + non-empty key)
	if len(th.Actions) != 1 {
		t.Fatalf("expected 1 valid action, got %d", len(th.Actions))
	}
	if th.Actions[0].Key != "valid fact" {
		t.Errorf("action[0].Key = %q", th.Actions[0].Key)
	}
}

func TestExecuteActions_WriteMemory(t *testing.T) {
	cfg := DefaultReverieConfig()
	cfg.SaveFile = ""
	r := NewReverie(cfg)

	var written string
	r.SetWriteMemory(func(ctx context.Context, fact string) error {
		written = fact
		return nil
	})

	thought := &Thought{
		ID: "t_test_1",
		Actions: []ReverieAction{
			{Type: "write_memory", Key: "用户是程序员"},
		},
	}
	r.executeActions(context.Background(), thought)

	if written != "用户是程序员" {
		t.Errorf("expected write_memory callback with fact, got %q", written)
	}
	log := r.ActionLog()
	if len(log) != 1 || !log[0].Success {
		t.Errorf("action log unexpected: %+v", log)
	}
}

func TestExecuteActions_CreateTask(t *testing.T) {
	cfg := DefaultReverieConfig()
	cfg.SaveFile = ""
	r := NewReverie(cfg)

	var taskTitle, taskDesc string
	r.SetCreateTask(func(ctx context.Context, title, desc string) error {
		taskTitle = title
		taskDesc = desc
		return nil
	})

	thought := &Thought{
		ID: "t_test_2",
		Actions: []ReverieAction{
			{Type: "create_task", Key: "学习并发", Value: "创建Go并发教程"},
		},
	}
	r.executeActions(context.Background(), thought)

	if taskTitle != "学习并发" || taskDesc != "创建Go并发教程" {
		t.Errorf("create_task: title=%q desc=%q", taskTitle, taskDesc)
	}
}

func TestExecuteActions_UpdateProfile(t *testing.T) {
	cfg := DefaultReverieConfig()
	cfg.SaveFile = ""
	r := NewReverie(cfg)

	var profileKey, profileValue string
	r.SetUpdateProfile(func(ctx context.Context, key, value string) error {
		profileKey = key
		profileValue = value
		return nil
	})

	thought := &Thought{
		ID: "t_test_3",
		Actions: []ReverieAction{
			{Type: "update_profile", Key: "编程偏好", Value: "偏好函数式风格"},
		},
	}
	r.executeActions(context.Background(), thought)

	if profileKey != "编程偏好" || profileValue != "偏好函数式风格" {
		t.Errorf("update_profile: key=%q value=%q", profileKey, profileValue)
	}
}

func TestExecuteActions_NoCallback(t *testing.T) {
	cfg := DefaultReverieConfig()
	cfg.SaveFile = ""
	r := NewReverie(cfg)
	// No callbacks set

	thought := &Thought{
		ID: "t_test_4",
		Actions: []ReverieAction{
			{Type: "write_memory", Key: "test"},
		},
	}
	r.executeActions(context.Background(), thought)

	log := r.ActionLog()
	if len(log) != 1 || log[0].Success {
		t.Errorf("expected failed action, got %+v", log)
	}
}

func TestThink_WithActions(t *testing.T) {
	cfg := DefaultReverieConfig()
	cfg.SaveFile = ""
	cfg.QuietStart = 0
	cfg.QuietEnd = 0
	r := NewReverie(cfg)

	r.SetLLMCall(func(ctx context.Context, system, user string) (string, error) {
		return `{"content":"用户热爱编程","category":"insight","significance":0.8,"trigger":"periodic","actions":[{"type":"write_memory","key":"用户热爱编程"}]}`, nil
	})
	r.SetRecall(func(query string) string { return "" })

	var memWritten string
	r.SetWriteMemory(func(ctx context.Context, fact string) error {
		memWritten = fact
		return nil
	})

	th, err := r.Think(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(th.Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(th.Actions))
	}
	if memWritten != "用户热爱编程" {
		t.Errorf("write_memory not called, got %q", memWritten)
	}
	if len(r.ActionLog()) != 1 {
		t.Errorf("action log len = %d", len(r.ActionLog()))
	}
}
