package session

import (
	"fmt"
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

func TestAddFilesAndFiles(t *testing.T) {
	s := NewStore(50)
	s.GetOrCreate("s1", "t1")
	s.AddFiles("s1",
		SessionFile{Path: "data/uploads/default/cat.png", Name: "cat.png", Kind: "uploaded"},
		SessionFile{Path: "data/output/report.docx", Name: "report.docx", Kind: "generated", Skill: "docx_create", Size: 1234},
	)
	files := s.Files("s1")
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d: %+v", len(files), files)
	}
	if files[1].Skill != "docx_create" || files[1].Size != 1234 {
		t.Fatalf("unexpected generated file entry: %+v", files[1])
	}
}

func TestAddFilesDedupesByPathRefreshingEntry(t *testing.T) {
	s := NewStore(50)
	s.GetOrCreate("s1", "t1")
	s.AddFiles("s1", SessionFile{Path: "data/output/report.docx", Name: "report.docx", Kind: "generated", Size: 100})
	s.AddFiles("s1", SessionFile{Path: "data/output/report.docx", Name: "report.docx", Kind: "generated", Size: 200})

	files := s.Files("s1")
	if len(files) != 1 {
		t.Fatalf("expected re-adding the same path to update in place, got %d files: %+v", len(files), files)
	}
	if files[0].Size != 200 {
		t.Fatalf("expected refreshed size 200, got %+v", files[0])
	}
}

func TestAddFilesNoopForUnknownSession(t *testing.T) {
	s := NewStore(50)
	s.AddFiles("missing", SessionFile{Path: "data/output/x.png", Name: "x.png"})
	if files := s.Files("missing"); files != nil {
		t.Fatalf("expected no files for unknown session, got %+v", files)
	}
}

func TestAddFilesEvictsOldestBeyondCap(t *testing.T) {
	s := NewStore(50)
	s.GetOrCreate("s1", "t1")
	for i := 0; i < maxSessionFiles+5; i++ {
		s.AddFiles("s1", SessionFile{Path: fmt.Sprintf("data/output/f%d.png", i), Name: fmt.Sprintf("f%d.png", i)})
	}
	files := s.Files("s1")
	if len(files) != maxSessionFiles {
		t.Fatalf("expected files capped at %d, got %d", maxSessionFiles, len(files))
	}
	if files[0].Name != "f5.png" {
		t.Fatalf("expected oldest files evicted, got first entry %+v", files[0])
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
