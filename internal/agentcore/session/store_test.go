package session

import (
	"testing"

	"yunque-agent/internal/agentcore/llm"
)

func TestGetOrCreate(t *testing.T) {
	s := NewStore(50)
	sess := s.GetOrCreate("s1", "t1")
	if sess.ID != "s1" || sess.TenantID != "t1" {
		t.Fatalf("unexpected session: %+v", sess)
	}
	// Second call returns same session
	sess2 := s.GetOrCreate("s1", "t1")
	if sess2.CreatedAt != sess.CreatedAt {
		t.Fatal("expected same session")
	}
}

func TestAppendAndGet(t *testing.T) {
	s := NewStore(50)
	s.GetOrCreate("s1", "t1")
	s.Append("s1", llm.Message{Role: "user", Content: "hello"})
	s.Append("s1", llm.Message{Role: "assistant", Content: "hi"})
	msgs := s.Get("s1")
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
}

func TestTrimming(t *testing.T) {
	s := NewStore(4)
	s.GetOrCreate("s1", "t1")
	for i := 0; i < 10; i++ {
		s.Append("s1", llm.Message{Role: "user", Content: "msg"})
	}
	msgs := s.Get("s1")
	if len(msgs) > 4 {
		t.Fatalf("expected max 4 messages after trim, got %d", len(msgs))
	}
}

func TestDelete(t *testing.T) {
	s := NewStore(50)
	s.GetOrCreate("s1", "t1")
	s.Delete("s1")
	if s.Get("s1") != nil {
		t.Fatal("expected nil after delete")
	}
}

func TestListByTenant(t *testing.T) {
	s := NewStore(50)
	s.GetOrCreate("s1", "t1")
	s.GetOrCreate("s2", "t1")
	s.GetOrCreate("s3", "t2")
	list := s.ListByTenant("t1")
	if len(list) != 2 {
		t.Fatalf("expected 2 sessions for t1, got %d", len(list))
	}
}
