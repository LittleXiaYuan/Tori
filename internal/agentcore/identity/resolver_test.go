package identity

import "testing"

func TestResolveNewUser(t *testing.T) {
	r := NewResolver()
	p := r.Resolve("telegram", "123", "Alice")
	if p.UnifiedID == "" {
		t.Fatal("expected unified ID")
	}
	if p.DisplayName != "Alice" {
		t.Fatalf("expected Alice, got %s", p.DisplayName)
	}
	if p.Channels["telegram"] != "123" {
		t.Fatal("expected telegram channel binding")
	}
	if p.MessageCount != 1 {
		t.Fatalf("expected 1 message, got %d", p.MessageCount)
	}
}

func TestResolveSameUser(t *testing.T) {
	r := NewResolver()
	r.Resolve("telegram", "123", "Alice")
	p := r.Resolve("telegram", "123", "")
	if p.MessageCount != 2 {
		t.Fatalf("expected 2 messages, got %d", p.MessageCount)
	}
	if p.DisplayName != "Alice" {
		t.Fatalf("display name should persist, got %s", p.DisplayName)
	}
}

func TestLinkCrossChannel(t *testing.T) {
	r := NewResolver()
	p := r.Resolve("telegram", "t123", "Alice")

	ok := r.Link(p.UnifiedID, "feishu", "f456")
	if !ok {
		t.Fatal("link should succeed")
	}

	// Lookup from feishu should return same profile
	p2, found := r.Lookup("feishu", "f456")
	if !found {
		t.Fatal("should find profile via feishu")
	}
	if p2.UnifiedID != p.UnifiedID {
		t.Fatalf("expected same unified ID, got %s vs %s", p.UnifiedID, p2.UnifiedID)
	}
}

func TestMergeProfiles(t *testing.T) {
	r := NewResolver()
	p1 := r.Resolve("telegram", "t1", "Alice")
	p2 := r.Resolve("feishu", "f1", "Alice F")

	ok := r.Merge(p1.UnifiedID, p2.UnifiedID)
	if !ok {
		t.Fatal("merge should succeed")
	}

	// Now feishu lookup should resolve to p1's ID
	found, ok := r.Lookup("feishu", "f1")
	if !ok {
		t.Fatal("should find merged profile via feishu")
	}
	if found.UnifiedID != p1.UnifiedID {
		t.Fatalf("merged profile should have keepID, got %s", found.UnifiedID)
	}
	if found.Channels["feishu"] != "f1" {
		t.Fatal("should have feishu channel after merge")
	}

	if r.Count() != 1 {
		t.Fatalf("expected 1 profile after merge, got %d", r.Count())
	}
}

func TestSetMeta(t *testing.T) {
	r := NewResolver()
	p := r.Resolve("telegram", "123", "Alice")
	r.SetMeta(p.UnifiedID, "lang", "zh")

	got, _ := r.Get(p.UnifiedID)
	if got.Metadata["lang"] != "zh" {
		t.Fatalf("expected zh, got %s", got.Metadata["lang"])
	}
}

func TestLinkConflict(t *testing.T) {
	r := NewResolver()
	p1 := r.Resolve("telegram", "t1", "Alice")
	r.Resolve("telegram", "t2", "Bob")

	// Try to link t2 (Bob's) to Alice - should fail
	ok := r.Link(p1.UnifiedID, "telegram", "t2")
	if ok {
		t.Fatal("should not link already-bound identity to different profile")
	}
}
