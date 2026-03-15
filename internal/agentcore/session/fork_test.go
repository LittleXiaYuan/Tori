package session

import (
	"testing"
)

func TestForkCreate(t *testing.T) {
	ft := NewForkTree()
	msgs := []ForkMessage{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi there"},
	}
	f := ft.Create("s1", msgs)
	if f.ID == "" {
		t.Fatal("expected fork ID")
	}
	if len(f.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(f.Messages))
	}
	if f.Label != "main" {
		t.Fatalf("expected 'main' label, got %s", f.Label)
	}

	// Creating again returns same root
	f2 := ft.Create("s1", nil)
	if f2.ID != f.ID {
		t.Fatalf("expected same root fork ID, got %s vs %s", f.ID, f2.ID)
	}
}

func TestForkBranch(t *testing.T) {
	ft := NewForkTree()
	msgs := []ForkMessage{
		{Role: "user", Content: "what is Go?"},
		{Role: "assistant", Content: "Go is a programming language"},
		{Role: "user", Content: "tell me more about concurrency"},
		{Role: "assistant", Content: "Go uses goroutines..."},
	}
	root := ft.Create("s1", msgs)

	// Branch after message index 1 (after "Go is a programming language")
	branch, err := ft.Branch(root.ID, 1, "explore-rust")
	if err != nil {
		t.Fatal(err)
	}
	if len(branch.Messages) != 2 {
		t.Fatalf("branch should have 2 messages (up to index 1), got %d", len(branch.Messages))
	}
	if branch.ParentID != root.ID {
		t.Fatalf("branch parent should be root, got %s", branch.ParentID)
	}
	if branch.Label != "explore-rust" {
		t.Fatalf("expected label 'explore-rust', got %s", branch.Label)
	}

	// Original root should still have 4 messages
	rootAgain, _ := ft.Get(root.ID)
	if len(rootAgain.Messages) != 4 {
		t.Fatalf("root should still have 4 messages, got %d", len(rootAgain.Messages))
	}
}

func TestForkAppend(t *testing.T) {
	ft := NewForkTree()
	root := ft.Create("s1", nil)

	ft.Append(root.ID, ForkMessage{Role: "user", Content: "hi"})
	ft.Append(root.ID, ForkMessage{Role: "assistant", Content: "hello"})

	f, _ := ft.Get(root.ID)
	if len(f.Messages) != 2 {
		t.Fatalf("expected 2 messages after append, got %d", len(f.Messages))
	}
}

func TestForkListBranches(t *testing.T) {
	ft := NewForkTree()
	root := ft.Create("s1", []ForkMessage{{Role: "user", Content: "start"}})
	ft.Branch(root.ID, 0, "b1")
	ft.Branch(root.ID, 0, "b2")

	branches := ft.ListBranches("s1")
	if len(branches) != 3 { // root + 2 branches
		t.Fatalf("expected 3 forks, got %d", len(branches))
	}
}

func TestForkAncestry(t *testing.T) {
	ft := NewForkTree()
	root := ft.Create("s1", []ForkMessage{{Role: "user", Content: "root"}})
	child, _ := ft.Branch(root.ID, 0, "child")
	grandchild, _ := ft.Branch(child.ID, 0, "grandchild")

	chain := ft.Ancestry(grandchild.ID)
	if len(chain) != 3 {
		t.Fatalf("expected 3 in ancestry chain, got %d", len(chain))
	}
	if chain[0].ID != root.ID {
		t.Fatal("first in chain should be root")
	}
	if chain[2].ID != grandchild.ID {
		t.Fatal("last in chain should be grandchild")
	}
}

func TestForkDelete(t *testing.T) {
	ft := NewForkTree()
	root := ft.Create("s1", []ForkMessage{{Role: "user", Content: "root"}})
	child, _ := ft.Branch(root.ID, 0, "child")
	ft.Branch(child.ID, 0, "grandchild")

	// Delete child — should also delete grandchild
	ft.Delete(child.ID)

	branches := ft.ListBranches("s1")
	if len(branches) != 1 { // only root remains
		t.Fatalf("expected 1 fork after delete, got %d", len(branches))
	}

	// Root should no longer list child
	rootAgain, _ := ft.Get(root.ID)
	if len(rootAgain.Children) != 0 {
		t.Fatalf("root should have 0 children after delete, got %d", len(rootAgain.Children))
	}
}

func TestForkGetRoot(t *testing.T) {
	ft := NewForkTree()
	root := ft.Create("s1", nil)

	got, ok := ft.GetRoot("s1")
	if !ok {
		t.Fatal("should find root")
	}
	if got.ID != root.ID {
		t.Fatalf("expected root ID %s, got %s", root.ID, got.ID)
	}

	_, ok = ft.GetRoot("nonexistent")
	if ok {
		t.Fatal("should not find root for nonexistent session")
	}
}

func TestForkIsolation(t *testing.T) {
	ft := NewForkTree()
	msgs := []ForkMessage{{Role: "user", Content: "shared"}}
	root := ft.Create("s1", msgs)
	branch, _ := ft.Branch(root.ID, 0, "diverge")

	// Append different messages to root vs branch
	ft.Append(root.ID, ForkMessage{Role: "user", Content: "root path"})
	ft.Append(branch.ID, ForkMessage{Role: "user", Content: "branch path"})

	rootFork, _ := ft.Get(root.ID)
	branchFork, _ := ft.Get(branch.ID)

	if len(rootFork.Messages) != 2 || rootFork.Messages[1].Content != "root path" {
		t.Fatal("root should have its own message path")
	}
	if len(branchFork.Messages) != 2 || branchFork.Messages[1].Content != "branch path" {
		t.Fatal("branch should have its own message path")
	}
}
