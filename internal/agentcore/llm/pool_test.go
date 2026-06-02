package llm

import (
	"testing"
)

func TestPool_RegisterAndGet(t *testing.T) {
	pool := NewPool()
	c := NewClient("http://a", "k", "model-a")
	pool.Register("fast", c)

	got := pool.Get("fast")
	if got != c {
		t.Fatal("expected registered client")
	}
	if pool.Get("nonexist") != nil {
		t.Fatal("expected nil for unregistered key")
	}
}

func TestPool_FirstRegisteredIsPrimary(t *testing.T) {
	pool := NewPool()
	c1 := NewClient("http://a", "k", "model-a")
	c2 := NewClient("http://b", "k", "model-b")
	pool.Register("fast", c1)
	pool.Register("smart", c2)

	if pool.Primary() != c1 {
		t.Fatal("first registered should be primary")
	}
}

func TestPool_SetPrimary(t *testing.T) {
	pool := NewPool()
	c1 := NewClient("http://a", "k", "a")
	c2 := NewClient("http://b", "k", "b")
	pool.Register("fast", c1)
	pool.Register("smart", c2)
	pool.SetPrimary("smart")

	if pool.Primary() != c2 {
		t.Fatal("expected smart as primary after SetPrimary")
	}
}

func TestPool_GetOrFallback(t *testing.T) {
	pool := NewPool()
	c1 := NewClient("http://a", "k", "a")
	pool.Register("fast", c1)

	// Exact match
	if pool.GetOrFallback("fast") != c1 {
		t.Fatal("should return exact match")
	}
	// Fallback to primary
	if pool.GetOrFallback("nonexist") != c1 {
		t.Fatal("should fallback to primary")
	}
}

func TestPool_GetFallbackChain_Full(t *testing.T) {
	pool := NewPool()
	expert := NewClient("http://e", "k", "expert-model")
	smart := NewClient("http://s", "k", "smart-model")
	fast := NewClient("http://f", "k", "fast-model")
	local := NewClient("http://l", "k", "local-model")
	pool.Register("expert", expert)
	pool.Register("smart", smart)
	pool.Register("fast", fast)
	pool.Register("local", local)
	pool.SetPrimary("smart")

	chain := pool.GetFallbackChain("expert")
	// Expected order: expert → smart → fast (primary=smart already seen).
	// Local desktop models must not be implicit fallbacks; use their provider
	// explicitly when local execution is desired.
	if len(chain) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(chain))
	}
	if chain[0] != expert {
		t.Fatal("first should be expert (requested)")
	}
	if chain[1] != smart {
		t.Fatal("second should be smart")
	}
	if chain[2] != fast {
		t.Fatal("third should be fast")
	}
	for _, c := range chain {
		if c == local {
			t.Fatal("local should not be included as an implicit fallback")
		}
	}
}

func TestPool_GetFallbackChain_UnknownKey(t *testing.T) {
	pool := NewPool()
	fast := NewClient("http://f", "k", "fast")
	pool.Register("fast", fast)
	pool.SetPrimary("fast")

	chain := pool.GetFallbackChain("nonexist")
	// nonexist → skip because not registered, expert→skip, smart→skip, fast→✓, primary=fast already seen
	if len(chain) != 1 {
		t.Fatalf("expected 1 entry (fast), got %d", len(chain))
	}
	if chain[0] != fast {
		t.Fatal("should fallback to fast")
	}
}

func TestPool_GetFallbackChain_Dedup(t *testing.T) {
	pool := NewPool()
	smart := NewClient("http://s", "k", "smart")
	pool.Register("smart", smart)
	pool.SetPrimary("smart")

	// Request smart → smart appears only once despite being both requested and primary
	chain := pool.GetFallbackChain("smart")
	if len(chain) != 1 {
		t.Fatalf("expected 1 (no duplicates), got %d", len(chain))
	}
}

func TestPool_GetFallbackChain_PrimaryAppended(t *testing.T) {
	pool := NewPool()
	c1 := NewClient("http://a", "k", "custom-a")
	c2 := NewClient("http://b", "k", "custom-b")
	pool.Register("custom", c1)
	pool.Register("backup", c2)
	pool.SetPrimary("backup")

	// Request "custom" → chain: custom, then backup as final failsafe
	chain := pool.GetFallbackChain("custom")
	if len(chain) != 2 {
		t.Fatalf("expected 2, got %d", len(chain))
	}
	if chain[0] != c1 || chain[1] != c2 {
		t.Fatal("expected custom then backup")
	}
}

func TestPool_Empty(t *testing.T) {
	pool := NewPool()
	if pool.Primary() != nil {
		t.Fatal("empty pool should return nil primary")
	}
	if pool.Size() != 0 {
		t.Fatal("expected size 0")
	}
	chain := pool.GetFallbackChain("anything")
	if len(chain) != 0 {
		t.Fatalf("expected empty chain, got %d", len(chain))
	}
}

func TestPool_Has(t *testing.T) {
	pool := NewPool()
	pool.Register("fast", NewClient("http://f", "k", "f"))
	if !pool.Has("fast") {
		t.Fatal("should have fast")
	}
	if pool.Has("slow") {
		t.Fatal("should not have slow")
	}
}

func TestPool_Keys(t *testing.T) {
	pool := NewPool()
	pool.Register("a", NewClient("http://a", "k", "a"))
	pool.Register("b", NewClient("http://b", "k", "b"))
	keys := pool.Keys()
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(keys))
	}
}

func TestPool_String(t *testing.T) {
	pool := NewPool()
	pool.Register("fast", NewClient("http://f", "k", "f"))
	s := pool.String()
	if s == "" {
		t.Fatal("expected non-empty string")
	}
}
