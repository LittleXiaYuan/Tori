package inbox

import (
	"testing"
)

func TestPushAndList(t *testing.T) {
	s := NewStore(100)
	item, err := s.Push("email", "hello world", ActionNotify, nil)
	if err != nil {
		t.Fatalf("push: %v", err)
	}
	if item.ID == "" || item.Source != "email" {
		t.Fatalf("unexpected item: %+v", item)
	}

	items := s.List(false, 10)
	if len(items) != 1 {
		t.Fatalf("list: got %d, want 1", len(items))
	}

	c := s.Count()
	if c.Total != 1 || c.Unread != 1 {
		t.Fatalf("count: %+v", c)
	}
}

func TestPushEmptyContent(t *testing.T) {
	s := NewStore(100)
	_, err := s.Push("x", "", ActionNotify, nil)
	if err == nil {
		t.Fatal("expected error for empty content")
	}
}

func TestMarkRead(t *testing.T) {
	s := NewStore(100)
	item1, _ := s.Push("a", "msg1", ActionNotify, nil)
	s.Push("b", "msg2", ActionNotify, nil)

	n := s.MarkRead([]string{item1.ID})
	if n != 1 {
		t.Fatalf("marked: got %d, want 1", n)
	}

	c := s.Count()
	if c.Unread != 1 {
		t.Fatalf("unread: got %d, want 1", c.Unread)
	}

	unread := s.Unread(10)
	if len(unread) != 1 {
		t.Fatalf("unread list: got %d", len(unread))
	}
}

func TestMarkAllRead(t *testing.T) {
	s := NewStore(100)
	s.Push("a", "msg1", ActionNotify, nil)
	s.Push("b", "msg2", ActionNotify, nil)

	n := s.MarkAllRead()
	if n != 2 {
		t.Fatalf("marked: got %d", n)
	}
	if s.Count().Unread != 0 {
		t.Fatal("should have 0 unread")
	}
}

func TestDelete(t *testing.T) {
	s := NewStore(100)
	item, _ := s.Push("x", "content", ActionNotify, nil)
	if !s.Delete(item.ID) {
		t.Fatal("delete returned false")
	}
	if s.Count().Total != 0 {
		t.Fatal("should be empty after delete")
	}
}

func TestEviction(t *testing.T) {
	s := NewStore(3)
	s.Push("a", "1", ActionNotify, nil)
	s.Push("b", "2", ActionNotify, nil)
	s.Push("c", "3", ActionNotify, nil)
	s.Push("d", "4", ActionNotify, nil)

	if s.Count().Total != 3 {
		t.Fatalf("expected eviction to cap at 3, got %d", s.Count().Total)
	}
}

func TestPendingTriggers(t *testing.T) {
	s := NewStore(100)
	s.Push("a", "notify msg", ActionNotify, nil)
	s.Push("b", "trigger msg", ActionTrigger, nil)

	triggers := s.PendingTriggers(10)
	if len(triggers) != 1 || triggers[0].Action != ActionTrigger {
		t.Fatalf("triggers: %+v", triggers)
	}
}

func TestSummary(t *testing.T) {
	s := NewStore(100)
	if s.Summary(5) != "" {
		t.Fatal("empty store should return empty summary")
	}
	s.Push("email", "important message", ActionNotify, nil)
	summary := s.Summary(5)
	if summary == "" {
		t.Fatal("summary should not be empty")
	}
}

func TestGet(t *testing.T) {
	s := NewStore(100)
	item, _ := s.Push("x", "content", ActionNotify, nil)
	got, ok := s.Get(item.ID)
	if !ok || got.Content != "content" {
		t.Fatalf("get: %+v, %v", got, ok)
	}
	_, ok = s.Get("nonexistent")
	if ok {
		t.Fatal("get nonexistent should return false")
	}
}

func TestDefaultAction(t *testing.T) {
	s := NewStore(100)
	item, _ := s.Push("x", "content", "invalid", nil)
	if item.Action != ActionNotify {
		t.Fatalf("expected default action notify, got %s", item.Action)
	}
}
