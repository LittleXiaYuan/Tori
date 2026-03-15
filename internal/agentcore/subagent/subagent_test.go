package subagent

import (
	"testing"
)

func TestSpawnAndGet(t *testing.T) {
	m := NewManager()
	sa, err := m.Spawn("bot-1", "researcher", "does research", []string{"search"})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	if sa.ID == "" || sa.Name != "researcher" || sa.ParentID != "bot-1" {
		t.Fatalf("unexpected: %+v", sa)
	}
	got, ok := m.Get(sa.ID)
	if !ok || got.Name != "researcher" {
		t.Fatal("get failed")
	}
}

func TestSpawnEmptyName(t *testing.T) {
	m := NewManager()
	_, err := m.Spawn("p", "", "", nil)
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestListByParent(t *testing.T) {
	m := NewManager()
	m.Spawn("bot-1", "a", "", nil)
	m.Spawn("bot-1", "b", "", nil)
	m.Spawn("bot-2", "c", "", nil)

	list := m.List("bot-1")
	if len(list) != 2 {
		t.Fatalf("expected 2, got %d", len(list))
	}
	all := m.List("")
	if len(all) != 3 {
		t.Fatalf("expected 3, got %d", len(all))
	}
}

func TestAppendMessages(t *testing.T) {
	m := NewManager()
	sa, _ := m.Spawn("p", "agent", "", nil)
	err := m.AppendMessages(sa.ID, []map[string]any{
		{"role": "user", "content": "hello"},
	})
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	got, _ := m.Get(sa.ID)
	if len(got.Messages) != 1 {
		t.Fatalf("messages: %d", len(got.Messages))
	}
}

func TestSetSkills(t *testing.T) {
	m := NewManager()
	sa, _ := m.Spawn("p", "agent", "", []string{"a"})
	m.SetSkills(sa.ID, []string{"x", "y"})
	got, _ := m.Get(sa.ID)
	if len(got.Skills) != 2 || got.Skills[0] != "x" {
		t.Fatalf("skills: %v", got.Skills)
	}
}

func TestDestroy(t *testing.T) {
	m := NewManager()
	sa, _ := m.Spawn("p", "agent", "", nil)
	if !m.Destroy(sa.ID) {
		t.Fatal("destroy returned false")
	}
	if m.Count() != 0 {
		t.Fatal("should be empty")
	}
	if m.Destroy("nonexistent") {
		t.Fatal("destroy nonexistent should return false")
	}
}

func TestCount(t *testing.T) {
	m := NewManager()
	m.Spawn("p", "a", "", nil)
	m.Spawn("p", "b", "", nil)
	if m.Count() != 2 {
		t.Fatalf("count: %d", m.Count())
	}
}
