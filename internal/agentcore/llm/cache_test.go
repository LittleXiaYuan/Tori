package llm

import (
	"testing"
	"time"
)

func TestResponseCache_PutGet(t *testing.T) {
	c := NewResponseCache(5*time.Second, 10)
	msgs := []Message{{Role: "user", Content: "hello"}}

	// Miss
	_, ok := c.Get(msgs, 0.7)
	if ok {
		t.Fatal("expected cache miss")
	}

	// Put + Hit
	c.Put(msgs, 0.7, "world")
	reply, ok := c.Get(msgs, 0.7)
	if !ok || reply != "world" {
		t.Fatalf("expected cache hit with 'world', got ok=%v reply=%q", ok, reply)
	}

	// Different temperature = miss
	_, ok = c.Get(msgs, 0.5)
	if ok {
		t.Fatal("expected miss for different temperature")
	}
}

func TestResponseCache_TTLExpiry(t *testing.T) {
	c := NewResponseCache(50*time.Millisecond, 10)
	msgs := []Message{{Role: "user", Content: "test"}}
	c.Put(msgs, 0.7, "response")

	// Should hit immediately
	_, ok := c.Get(msgs, 0.7)
	if !ok {
		t.Fatal("expected hit before TTL")
	}

	time.Sleep(100 * time.Millisecond)

	// Should miss after TTL
	_, ok = c.Get(msgs, 0.7)
	if ok {
		t.Fatal("expected miss after TTL")
	}
}

func TestResponseCache_MaxSize(t *testing.T) {
	c := NewResponseCache(10*time.Second, 3)

	for i := 0; i < 5; i++ {
		msgs := []Message{{Role: "user", Content: string(rune('a' + i))}}
		c.Put(msgs, 0.7, "reply")
	}

	stats := c.Stats()
	size := stats["size"].(int)
	if size > 3 {
		t.Fatalf("expected max 3 entries, got %d", size)
	}
}

func TestResponseCache_Stats(t *testing.T) {
	c := NewResponseCache(10*time.Second, 100)
	msgs := []Message{{Role: "user", Content: "stats test"}}
	c.Put(msgs, 0.7, "reply")
	c.Get(msgs, 0.7)
	c.Get(msgs, 0.7)

	stats := c.Stats()
	if stats["size"].(int) != 1 {
		t.Fatalf("expected size 1, got %d", stats["size"])
	}
	if stats["total_hits"].(int) != 2 {
		t.Fatalf("expected 2 hits, got %d", stats["total_hits"])
	}
}

func TestCacheKeyPrefix(t *testing.T) {
	msgs := []Message{{Role: "user", Content: "short"}}
	prefix := CacheKeyPrefix(msgs)
	if prefix != "short" {
		t.Fatalf("expected 'short', got %q", prefix)
	}

	long := []Message{{Role: "user", Content: "这是一段很长的中文消息用来测试截断功能是否正确工作在四十个字符以上的情况下"}}
	prefix = CacheKeyPrefix(long)
	if len([]rune(prefix)) > 44 { // 40 + "..."
		t.Fatalf("expected truncated prefix, got %q", prefix)
	}
}
