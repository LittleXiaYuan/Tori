package audit

import (
	"os"
	"path/filepath"
	"testing"
)

func TestChainAppendAndVerify(t *testing.T) {
	c, err := NewChain(ChainConfig{})
	if err != nil {
		t.Fatal(err)
	}
	r1 := c.Append(EventChat, "user1", "send_message", "hello")
	r2 := c.Append(EventChat, "user1", "send_message", "world")

	if r1.Seq != 1 || r2.Seq != 2 {
		t.Errorf("expected seq 1,2 got %d,%d", r1.Seq, r2.Seq)
	}
	if r2.PrevHash != r1.Hash {
		t.Error("chain linkage broken: r2.PrevHash != r1.Hash")
	}
	if r1.Hash == r2.Hash {
		t.Error("different records should have different hashes")
	}
	if idx := c.Verify(); idx != -1 {
		t.Errorf("expected valid chain, got invalid at %d", idx)
	}
}

func TestChainTamperDetection(t *testing.T) {
	c, err := NewChain(ChainConfig{})
	if err != nil {
		t.Fatal(err)
	}
	c.Append(EventChat, "user1", "msg1", "a")
	c.Append(EventChat, "user1", "msg2", "b")
	c.Append(EventChat, "user1", "msg3", "c")

	// Tamper with second record
	c.mu.Lock()
	c.records[1].Detail = "tampered"
	c.mu.Unlock()

	idx := c.Verify()
	if idx != 1 {
		t.Errorf("expected tamper detected at index 1, got %d", idx)
	}
}

func TestChainLinkageTamper(t *testing.T) {
	c, err := NewChain(ChainConfig{})
	if err != nil {
		t.Fatal(err)
	}
	c.Append(EventAuth, "admin", "login", "ok")
	c.Append(EventAuth, "admin", "action", "delete")
	c.Append(EventAuth, "admin", "logout", "ok")

	// Tamper with chain linkage
	c.mu.Lock()
	c.records[2].PrevHash = "fake_hash"
	c.mu.Unlock()

	idx := c.Verify()
	if idx != 2 {
		t.Errorf("expected tamper at index 2, got %d", idx)
	}
}

func TestChainTail(t *testing.T) {
	c, err := NewChain(ChainConfig{})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		c.Append(EventSystem, "sys", "tick", "")
	}
	tail := c.Tail(3)
	if len(tail) != 3 {
		t.Fatalf("expected 3 records, got %d", len(tail))
	}
	if tail[0].Seq != 5 || tail[2].Seq != 3 {
		t.Error("tail should be newest first")
	}
}

func TestChainTailOverflow(t *testing.T) {
	c, err := NewChain(ChainConfig{})
	if err != nil {
		t.Fatal(err)
	}
	c.Append(EventChat, "u", "a", "")
	tail := c.Tail(100)
	if len(tail) != 1 {
		t.Errorf("expected 1, got %d", len(tail))
	}
}

func TestChainSearch(t *testing.T) {
	c, err := NewChain(ChainConfig{})
	if err != nil {
		t.Fatal(err)
	}
	c.Append(EventChat, "user1", "msg", "a")
	c.Append(EventAuth, "admin", "login", "b")
	c.Append(EventChat, "user2", "msg", "c")
	c.Append(EventChat, "user1", "msg", "d")

	// Search by type
	results := c.Search(EventChat, "", 10)
	if len(results) != 3 {
		t.Errorf("expected 3 chat events, got %d", len(results))
	}

	// Search by actor
	results = c.Search("", "user1", 10)
	if len(results) != 2 {
		t.Errorf("expected 2 user1 events, got %d", len(results))
	}

	// Search by both
	results = c.Search(EventAuth, "admin", 10)
	if len(results) != 1 {
		t.Errorf("expected 1, got %d", len(results))
	}
}

func TestChainEviction(t *testing.T) {
	c, err := NewChain(ChainConfig{MaxSize: 5})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		c.Append(EventSystem, "sys", "tick", "")
	}
	if c.Len() != 5 {
		t.Errorf("expected 5 after eviction, got %d", c.Len())
	}
	last := c.Last()
	if last.Seq != 10 {
		t.Errorf("expected last seq 10, got %d", last.Seq)
	}
}

func TestChainStats(t *testing.T) {
	c, err := NewChain(ChainConfig{})
	if err != nil {
		t.Fatal(err)
	}
	c.Append(EventChat, "u1", "a", "")
	c.Append(EventAuth, "u2", "b", "")
	c.Append(EventChat, "u1", "c", "")

	stats := c.Stats()
	if stats["total_seq"].(uint64) != 3 {
		t.Errorf("expected total_seq 3, got %v", stats["total_seq"])
	}
	counts := stats["type_counts"].(map[EventType]int)
	if counts[EventChat] != 2 {
		t.Errorf("expected 2 chat events, got %d", counts[EventChat])
	}
}

func TestChainPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")

	// Write records
	c1, err := NewChain(ChainConfig{FilePath: path})
	if err != nil {
		t.Fatal(err)
	}
	c1.Append(EventChat, "u1", "msg1", "hello")
	c1.Append(EventAuth, "admin", "login", "ok")
	c1.Append(EventChat, "u1", "msg2", "world")
	lastHash := c1.Last().Hash
	c1.Close()

	// Verify file exists
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal("JSONL file not created")
	}
	if info.Size() == 0 {
		t.Fatal("JSONL file is empty")
	}

	// Reload
	c2, err := NewChain(ChainConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if err := c2.LoadFromFile(path); err != nil {
		t.Fatal(err)
	}
	if c2.Len() != 3 {
		t.Errorf("expected 3 records after reload, got %d", c2.Len())
	}
	if c2.Last().Hash != lastHash {
		t.Error("last hash mismatch after reload")
	}
	if idx := c2.Verify(); idx != -1 {
		t.Errorf("chain invalid after reload at %d", idx)
	}
}

func TestChainLoadNonexistent(t *testing.T) {
	c, _ := NewChain(ChainConfig{})
	err := c.LoadFromFile("/nonexistent/path/audit.jsonl")
	if err != nil {
		t.Error("expected nil error for nonexistent file")
	}
}

func TestChainDetailTruncation(t *testing.T) {
	c, _ := NewChain(ChainConfig{})
	longDetail := make([]byte, 5000)
	for i := range longDetail {
		longDetail[i] = 'x'
	}
	rec := c.Append(EventChat, "u", "msg", string(longDetail))
	if len(rec.Detail) > 4200 {
		t.Error("detail should be truncated")
	}
}

func TestChainLast(t *testing.T) {
	c, _ := NewChain(ChainConfig{})
	if c.Last() != nil {
		t.Error("expected nil for empty chain")
	}
	c.Append(EventSystem, "sys", "boot", "")
	if c.Last() == nil {
		t.Error("expected non-nil after append")
	}
}

func TestChainEmptyVerify(t *testing.T) {
	c, _ := NewChain(ChainConfig{})
	if idx := c.Verify(); idx != -1 {
		t.Errorf("empty chain should be valid, got %d", idx)
	}
}
