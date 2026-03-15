package bots

import (
	"testing"
)

func TestCreateAndGet(t *testing.T) {
	m := NewManager()
	bot, err := m.Create("test-bot", "a test bot", BotConfig{})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if bot.Name != "test-bot" || bot.ID == "" {
		t.Fatalf("unexpected bot: %+v", bot)
	}
	if bot.Config.MaxSteps != 8 {
		t.Fatalf("default max_steps: got %d", bot.Config.MaxSteps)
	}

	got, ok := m.Get(bot.ID)
	if !ok || got.Name != "test-bot" {
		t.Fatalf("get: %+v, %v", got, ok)
	}
}

func TestCreateDuplicateName(t *testing.T) {
	m := NewManager()
	m.Create("bot1", "", BotConfig{})
	_, err := m.Create("bot1", "", BotConfig{})
	if err == nil {
		t.Fatal("expected error for duplicate name")
	}
}

func TestCreateEmptyName(t *testing.T) {
	m := NewManager()
	_, err := m.Create("", "", BotConfig{})
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestGetByName(t *testing.T) {
	m := NewManager()
	m.Create("alpha", "", BotConfig{})
	bot, ok := m.GetByName("alpha")
	if !ok || bot.Name != "alpha" {
		t.Fatalf("getByName: %+v", bot)
	}
	_, ok = m.GetByName("nonexistent")
	if ok {
		t.Fatal("should not find nonexistent")
	}
}

func TestList(t *testing.T) {
	m := NewManager()
	m.Create("a", "", BotConfig{})
	m.Create("b", "", BotConfig{})
	list := m.List()
	if len(list) != 2 {
		t.Fatalf("list: got %d", len(list))
	}
}

func TestUpdate(t *testing.T) {
	m := NewManager()
	bot, _ := m.Create("original", "desc", BotConfig{Model: "gpt-4"})
	newName := "updated"
	newDesc := "new desc"
	newCfg := &BotConfig{Model: "claude-3"}
	updated, err := m.Update(bot.ID, &newName, &newDesc, newCfg)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Name != "updated" || updated.Config.Model != "claude-3" {
		t.Fatalf("unexpected: %+v", updated)
	}
}

func TestUpdateDuplicateName(t *testing.T) {
	m := NewManager()
	m.Create("a", "", BotConfig{})
	bot2, _ := m.Create("b", "", BotConfig{})
	dup := "a"
	_, err := m.Update(bot2.ID, &dup, nil, nil)
	if err == nil {
		t.Fatal("expected error for duplicate name")
	}
}

func TestSetActive(t *testing.T) {
	m := NewManager()
	bot, _ := m.Create("bot", "", BotConfig{})
	m.SetActive(bot.ID, false)
	got, _ := m.Get(bot.ID)
	if got.IsActive || got.Status != StatusStopped {
		t.Fatalf("expected stopped: %+v", got)
	}
	m.SetActive(bot.ID, true)
	got, _ = m.Get(bot.ID)
	if !got.IsActive || got.Status != StatusReady {
		t.Fatalf("expected ready: %+v", got)
	}
}

func TestDelete(t *testing.T) {
	m := NewManager()
	bot, _ := m.Create("bot", "", BotConfig{})
	if err := m.Delete(bot.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if m.Count() != 0 {
		t.Fatal("should be empty")
	}
	if err := m.Delete("nonexistent"); err == nil {
		t.Fatal("expected error for nonexistent")
	}
}

func TestCounters(t *testing.T) {
	m := NewManager()
	m.Create("a", "", BotConfig{})
	bot2, _ := m.Create("b", "", BotConfig{})
	m.SetActive(bot2.ID, false)

	if m.Count() != 2 {
		t.Fatalf("count: got %d", m.Count())
	}
	if m.ActiveCount() != 1 {
		t.Fatalf("active: got %d", m.ActiveCount())
	}
}
