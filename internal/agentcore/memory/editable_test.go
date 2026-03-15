package memory

import (
	"testing"
)

func TestAddAndGetBlock(t *testing.T) {
	em := NewEditableMemory()
	b := em.AddBlock("persona", "I am a helpful assistant", 0)
	if b.Label != "persona" || b.Version != 1 {
		t.Fatal("wrong block")
	}
	got, ok := em.GetBlock("persona")
	if !ok || got.Content != "I am a helpful assistant" {
		t.Fatal("get failed")
	}
}

func TestAllBlocks(t *testing.T) {
	em := NewEditableMemory()
	em.AddBlock("a", "x", 0)
	em.AddBlock("b", "y", 0)
	if len(em.AllBlocks()) != 2 {
		t.Fatal("expected 2")
	}
}

func TestRemoveBlock(t *testing.T) {
	em := NewEditableMemory()
	em.AddBlock("temp", "data", 0)
	if !em.RemoveBlock("temp") {
		t.Fatal("should remove")
	}
	if em.RemoveBlock("temp") {
		t.Fatal("already removed")
	}
}

func TestRenameBlock(t *testing.T) {
	em := NewEditableMemory()
	em.AddBlock("old", "content", 0)
	if err := em.RenameBlock("old", "new"); err != nil {
		t.Fatal(err)
	}
	if _, ok := em.GetBlock("old"); ok {
		t.Fatal("old should not exist")
	}
	if _, ok := em.GetBlock("new"); !ok {
		t.Fatal("new should exist")
	}
}

func TestRenameBlockConflict(t *testing.T) {
	em := NewEditableMemory()
	em.AddBlock("a", "x", 0)
	em.AddBlock("b", "y", 0)
	if err := em.RenameBlock("a", "b"); err == nil {
		t.Fatal("should conflict")
	}
}

func TestEditReplace(t *testing.T) {
	em := NewEditableMemory()
	em.AddBlock("notes", "I like cats and dogs", 0)
	r := em.Edit(EditRequest{BlockLabel: "notes", Op: OpReplace, OldText: "cats", NewText: "birds"})
	if !r.Success {
		t.Fatalf("failed: %s", r.Error)
	}
	b, _ := em.GetBlock("notes")
	if b.Content != "I like birds and dogs" {
		t.Fatalf("wrong content: %s", b.Content)
	}
	if b.Version != 2 {
		t.Fatal("version should be 2")
	}
}

func TestEditReplaceNotFound(t *testing.T) {
	em := NewEditableMemory()
	em.AddBlock("notes", "hello", 0)
	r := em.Edit(EditRequest{BlockLabel: "notes", Op: OpReplace, OldText: "xyz", NewText: "abc"})
	if r.Success {
		t.Fatal("should fail")
	}
}

func TestEditInsert(t *testing.T) {
	em := NewEditableMemory()
	em.AddBlock("list", "line1\nline2\nline3", 0)
	r := em.Edit(EditRequest{BlockLabel: "list", Op: OpInsert, LineNumber: 2, NewText: "inserted"})
	if !r.Success {
		t.Fatalf("failed: %s", r.Error)
	}
	b, _ := em.GetBlock("list")
	lines := b.Lines()
	if len(lines) != 4 || lines[1] != "inserted" {
		t.Fatalf("wrong insert: %v", lines)
	}
}

func TestEditInsertAtEnd(t *testing.T) {
	em := NewEditableMemory()
	em.AddBlock("list", "a\nb", 0)
	r := em.Edit(EditRequest{BlockLabel: "list", Op: OpInsert, LineNumber: 99, NewText: "c"})
	if !r.Success {
		t.Fatalf("failed: %s", r.Error)
	}
	b, _ := em.GetBlock("list")
	if b.LineCount() != 3 {
		t.Fatal("should have 3 lines")
	}
}

func TestEditPatch(t *testing.T) {
	em := NewEditableMemory()
	em.AddBlock("doc", "The quick brown fox", 0)
	r := em.Edit(EditRequest{BlockLabel: "doc", Op: OpPatch, OldText: "quick brown", NewText: "slow red"})
	if !r.Success {
		t.Fatalf("failed: %s", r.Error)
	}
	b, _ := em.GetBlock("doc")
	if b.Content != "The slow red fox" {
		t.Fatal("wrong patch")
	}
}

func TestEditRethink(t *testing.T) {
	em := NewEditableMemory()
	em.AddBlock("persona", "old persona text", 0)
	r := em.Edit(EditRequest{BlockLabel: "persona", Op: OpRethink, NewText: "completely new persona"})
	if !r.Success {
		t.Fatalf("failed: %s", r.Error)
	}
	b, _ := em.GetBlock("persona")
	if b.Content != "completely new persona" {
		t.Fatal("rethink failed")
	}
}

func TestEditDelete(t *testing.T) {
	em := NewEditableMemory()
	em.AddBlock("list", "a\nb\nc\nd", 0)
	r := em.Edit(EditRequest{BlockLabel: "list", Op: OpDelete, LineNumber: 2, LineCount: 2})
	if !r.Success {
		t.Fatalf("failed: %s", r.Error)
	}
	b, _ := em.GetBlock("list")
	if b.Content != "a\nd" {
		t.Fatalf("wrong delete: %q", b.Content)
	}
}

func TestEditDeleteOutOfRange(t *testing.T) {
	em := NewEditableMemory()
	em.AddBlock("x", "one", 0)
	r := em.Edit(EditRequest{BlockLabel: "x", Op: OpDelete, LineNumber: 5})
	if r.Success {
		t.Fatal("should fail out of range")
	}
}

func TestEditReadOnly(t *testing.T) {
	em := NewEditableMemory()
	b := em.AddBlock("system", "do not change", 0)
	b.ReadOnly = true
	r := em.Edit(EditRequest{BlockLabel: "system", Op: OpRethink, NewText: "changed"})
	if r.Success {
		t.Fatal("should not edit read-only")
	}
}

func TestEditMaxChars(t *testing.T) {
	em := NewEditableMemory()
	em.AddBlock("small", "hi", 10)
	r := em.Edit(EditRequest{BlockLabel: "small", Op: OpRethink, NewText: "this is way too long for the limit"})
	if r.Success {
		t.Fatal("should fail max chars")
	}
}

func TestEditBlockNotFound(t *testing.T) {
	em := NewEditableMemory()
	r := em.Edit(EditRequest{BlockLabel: "nope", Op: OpRethink, NewText: "x"})
	if r.Success {
		t.Fatal("should fail")
	}
}

func TestCompile(t *testing.T) {
	em := NewEditableMemory()
	em.AddBlock("persona", "I am helpful", 0)
	em.AddBlock("human", "User likes Go", 0)
	compiled := em.Compile()
	if compiled == "" {
		t.Fatal("empty compile")
	}
	if !containsStr(compiled, "<persona>") || !containsStr(compiled, "I am helpful") {
		t.Fatal("missing persona in compile")
	}
}

func TestEditHistory(t *testing.T) {
	em := NewEditableMemory()
	em.AddBlock("n", "text", 0)
	em.Edit(EditRequest{BlockLabel: "n", Op: OpRethink, NewText: "v2"})
	em.Edit(EditRequest{BlockLabel: "n", Op: OpRethink, NewText: "v3"})
	h := em.History(10)
	if len(h) != 2 {
		t.Fatalf("expected 2 history, got %d", len(h))
	}
}

func TestBlockLines(t *testing.T) {
	b := &Block{Content: "a\nb\nc"}
	if b.LineCount() != 3 {
		t.Fatal("expected 3")
	}
	b2 := &Block{Content: ""}
	if b2.LineCount() != 0 {
		t.Fatal("expected 0")
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && stringContains(s, sub))
}

func stringContains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
