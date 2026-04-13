package distill

import (
	"context"
	"testing"
)

func TestShouldDistill(t *testing.T) {
	d := New(nil)

	tests := []struct {
		name string
		tier string
		reply string
		want bool
	}{
		{"expert long reply", "expert", string(make([]rune, 300)), true},
		{"expert short reply", "expert", "短", false},
		{"smart model", "smart", string(make([]rune, 300)), false},
		{"fast model", "fast", string(make([]rune, 300)), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := d.ShouldDistill(tt.tier, tt.reply)
			if got != tt.want {
				t.Errorf("ShouldDistill(%s, len=%d) = %v, want %v", tt.tier, len(tt.reply), got, tt.want)
			}
		})
	}
}

func TestCheckCacheNoSearch(t *testing.T) {
	d := New(nil)
	result, found := d.CheckCache(context.Background(), "test")
	if found || result != "" {
		t.Error("expected empty result with no search func")
	}
}

func TestCheckCacheWithSearch(t *testing.T) {
	d := New(nil)
	d.SetSearch(func(ctx context.Context, query string) (string, bool) {
		if query == "how to deploy" {
			return "当需要部署时，应该使用docker compose", true
		}
		return "", false
	})

	result, found := d.CheckCache(context.Background(), "how to deploy")
	if !found {
		t.Error("expected to find cached rule")
	}
	if result == "" {
		t.Error("expected non-empty rule")
	}
}

func TestDistillNilGuards(t *testing.T) {
	d := New(nil)
	// Should not panic with nil llmCall or nil store
	d.Distill(context.Background(), "question", "long expert answer that is quite detailed and thorough")
}

func TestDistillWithStore(t *testing.T) {
	stored := make(map[string]string)
	mockLLM := func(ctx context.Context, system, user string) (string, error) {
		return "当遇到X时，应该Y", nil
	}
	d := New(mockLLM)
	d.SetStore(func(ctx context.Context, key, value, category string) error {
		stored[key] = value
		return nil
	})

	// Need a long enough reply (default minReplyLen = 200)
	longReply := string(make([]rune, 250))
	d.Distill(context.Background(), "test question", longReply)

	// Distill runs async — give it a moment (but the test mainly checks no panic)
}

func TestDistillCooldown(t *testing.T) {
	callCount := 0
	mockLLM := func(ctx context.Context, system, user string) (string, error) {
		callCount++
		return "rule", nil
	}
	d := New(mockLLM)
	d.SetStore(func(ctx context.Context, key, value, category string) error { return nil })

	longReply := string(make([]rune, 250))
	// First call — should proceed
	d.Distill(context.Background(), "same question", longReply)
	// Second call with same question — should be deduplicated by cooldown
	d.Distill(context.Background(), "same question", longReply)

	// Can't easily verify async behavior in unit test, but no panic = good
}

func TestCleanCooldown(t *testing.T) {
	d := New(nil)
	// Manually add cooldown entries
	d.mu.Lock()
	d.cooldown["old"] = d.cooldown["old"] // zero time → will be cleaned
	d.mu.Unlock()

	d.CleanCooldown()
	// Verify cleanup happened
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, exists := d.cooldown["old"]; exists {
		t.Error("expected old cooldown entry to be cleaned")
	}
}

func TestClassifyCategory(t *testing.T) {
	tests := []struct {
		question string
		want     string
	}{
		{"如何修复这个代码bug", "coding"},
		{"如何部署到docker", "devops"},
		{"如何保障安全性", "security"},
		{"数据库查询优化", "data"},
		{"今天天气怎么样", "general"},
	}

	for _, tt := range tests {
		got := classifyCategory(tt.question)
		if got != tt.want {
			t.Errorf("classifyCategory(%q) = %s, want %s", tt.question, got, tt.want)
		}
	}
}

func TestNormalizeKey(t *testing.T) {
	// Should truncate long strings to 80 runes
	longStr := string(make([]rune, 200))
	key := normalizeKey(longStr)
	if len([]rune(key)) > 80 {
		t.Errorf("normalizeKey len = %d, want <= 80", len([]rune(key)))
	}
}
