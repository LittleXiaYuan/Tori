package emotion

import (
	"testing"
	"time"
)

func TestHistory_RecordAndQuery(t *testing.T) {
	h := NewHistory(100)
	h.Record("s1", EmotionHappy, 0.9, "text")
	h.Record("s1", EmotionSad, 0.7, "text")
	h.Record("s2", EmotionAngry, 0.8, "audio")

	all := h.Query("", time.Time{}, time.Time{}, 0)
	if len(all) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(all))
	}
	// Chronological order.
	if all[0].Emotion != EmotionHappy || all[2].Emotion != EmotionAngry {
		t.Error("unexpected order")
	}

	s1 := h.Query("s1", time.Time{}, time.Time{}, 0)
	if len(s1) != 2 {
		t.Fatalf("expected 2 entries for s1, got %d", len(s1))
	}

	limited := h.Query("", time.Time{}, time.Time{}, 1)
	if len(limited) != 1 {
		t.Fatalf("expected 1 entry with limit, got %d", len(limited))
	}
}

func TestHistory_Cap(t *testing.T) {
	h := NewHistory(3)
	for i := 0; i < 5; i++ {
		h.Record("s", EmotionNeutral, 0.5, "text")
	}
	all := h.Query("", time.Time{}, time.Time{}, 0)
	if len(all) != 3 {
		t.Fatalf("expected cap of 3, got %d", len(all))
	}
}

func TestHistory_TimeFilter(t *testing.T) {
	h := NewHistory(100)
	h.Record("s", EmotionHappy, 0.9, "text")
	time.Sleep(10 * time.Millisecond)
	mid := time.Now()
	time.Sleep(10 * time.Millisecond)
	h.Record("s", EmotionSad, 0.7, "text")

	after := h.Query("", mid, time.Time{}, 0)
	if len(after) != 1 || after[0].Emotion != EmotionSad {
		t.Fatalf("expected 1 entry after mid, got %d", len(after))
	}
}

func TestSummary(t *testing.T) {
	entries := []HistoryEntry{
		{Emotion: EmotionHappy},
		{Emotion: EmotionHappy},
		{Emotion: EmotionSad},
	}
	s := Summary(entries)
	if s[EmotionHappy] != 2 || s[EmotionSad] != 1 {
		t.Fatalf("unexpected summary: %v", s)
	}
}
