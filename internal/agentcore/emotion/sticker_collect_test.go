package emotion

import (
	"context"
	"strings"
	"testing"
	"yunque-agent/internal/execution/channel"
)

func TestParseStickerCommand(t *testing.T) {
	tests := []struct {
		input  string
		wantEm Emotion
		wantOK bool
	}{
		{"/sticker", EmotionHappy, true}, // default
		{"/sticker happy", EmotionHappy, true},
		{"/sticker sad", EmotionSad, true},
		{"/sticker 开心", EmotionHappy, true},
		{"/sticker 悲伤", EmotionSad, true},
		{"/sticker 愤怒", EmotionAngry, true},
		{"/sticker 害怕", EmotionFearful, true},
		{"/sticker angry", EmotionAngry, true},
		{"/sticker neutral", EmotionNeutral, true},
		{"/sticker surprised", EmotionSurprised, true},
		{"/sticker disgusted", EmotionDisgusted, true},
		{"/sticker 中性", EmotionNeutral, true},
		{"/sticker nonsense", Emotion(""), false}, // unknown
	}
	for _, tt := range tests {
		em, ok := ParseStickerCommand(tt.input)
		if ok != tt.wantOK {
			t.Errorf("ParseStickerCommand(%q) ok=%v, want %v", tt.input, ok, tt.wantOK)
			continue
		}
		if ok && em != tt.wantEm {
			t.Errorf("ParseStickerCommand(%q) = %v, want %v", tt.input, em, tt.wantEm)
		}
	}
}

func TestStickerCollectorSessionLifecycle(t *testing.T) {
	sm := DefaultStickerMap()
	sc := NewStickerCollector(sm, "")

	// No active session initially
	if sc.HasActiveSession("telegram", "user1") {
		t.Fatal("expected no active session")
	}

	// Start session
	prompt := sc.StartSession("telegram", "user1", EmotionHappy)
	if prompt == "" {
		t.Fatal("expected non-empty prompt")
	}
	if !sc.HasActiveSession("telegram", "user1") {
		t.Fatal("expected active session after start")
	}

	// Cancel session
	if !sc.CancelSession("telegram", "user1") {
		t.Fatal("expected cancel to return true")
	}
	if sc.HasActiveSession("telegram", "user1") {
		t.Fatal("expected no session after cancel")
	}

	// Cancel non-existing session
	if sc.CancelSession("telegram", "user1") {
		t.Fatal("expected cancel to return false for non-existing session")
	}
}

func TestStickerCollectorTryCollect(t *testing.T) {
	sm := DefaultStickerMap()
	sc := NewStickerCollector(sm, "")

	// Start session
	sc.StartSession("telegram", "user1", EmotionSad)

	// Non-sticker message should not be collected
	ok, _ := sc.TryCollect(channel.Message{
		ChannelType: "telegram",
		UserID:      "user1",
		Content:     "hello",
	})
	if ok {
		t.Fatal("expected non-sticker message to not be collected")
	}

	// Sticker message should be collected
	rich := &channel.RichMessage{}
	rich.Add(&channel.StickerComponent{
		PackageID: "pkg1",
		StickerID: "stk1",
		FileID:    "file_abc123",
		Emoji:     "😢",
		SetName:   "SadPack",
	})
	ok, reply := sc.TryCollect(channel.Message{
		ChannelType: "telegram",
		UserID:      "user1",
		Content:     "",
		Rich:        rich,
	})
	if !ok {
		t.Fatal("expected sticker to be collected")
	}
	if reply == "" {
		t.Fatal("expected non-empty reply")
	}

	// Session should be consumed (one-shot)
	if sc.HasActiveSession("telegram", "user1") {
		t.Fatal("expected session consumed after collection")
	}

	// Verify sticker was registered in the map
	suggestions := sm.SuggestAll(EmotionSad, "telegram")
	found := false
	for _, s := range suggestions {
		if s.FileID == "file_abc123" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected collected sticker to appear in StickerMap suggestions")
	}
}

func TestStickerCollectorNoSessionIgnoresSticker(t *testing.T) {
	sm := DefaultStickerMap()
	sc := NewStickerCollector(sm, "")

	// Send sticker without active session
	rich := &channel.RichMessage{}
	rich.Add(&channel.StickerComponent{StickerID: "stk1"})
	ok, _ := sc.TryCollect(channel.Message{
		ChannelType: "telegram",
		UserID:      "user2",
		Rich:        rich,
	})
	if ok {
		t.Fatal("expected sticker to be ignored without active session")
	}
}

func TestStickerCollectorDifferentUsers(t *testing.T) {
	sm := DefaultStickerMap()
	sc := NewStickerCollector(sm, "")

	// Start session for user1
	sc.StartSession("telegram", "user1", EmotionAngry)

	// User2's sticker should not be collected
	rich := &channel.RichMessage{}
	rich.Add(&channel.StickerComponent{StickerID: "stk1"})
	ok, _ := sc.TryCollect(channel.Message{
		ChannelType: "telegram",
		UserID:      "user2",
		Rich:        rich,
	})
	if ok {
		t.Fatal("expected user2's sticker to not be collected by user1's session")
	}

	// User1's session should still be active
	if !sc.HasActiveSession("telegram", "user1") {
		t.Fatal("expected user1 session still active")
	}
}

func TestAutoLearnWithMockAnalyzer(t *testing.T) {
	sm := DefaultStickerMap()
	sc := NewStickerCollector(sm, "")

	// Create an analyzer with a mock LLM that always returns "happy" with high confidence
	analyzer := NewAnalyzer()
	analyzer.SetLLMCall(func(ctx context.Context, systemPrompt, userMsg string) (string, error) {
		return `{"emotion":"happy","confidence":0.85}`, nil
	})
	sc.SetAnalyzer(analyzer)

	// Build a message with a sticker
	rich := &channel.RichMessage{}
	rich.Add(&channel.StickerComponent{
		PackageID: "auto_pkg",
		StickerID: "auto_stk",
		FileID:    "auto_file_001",
		Emoji:     "😄",
	})
	msg := channel.Message{
		ChannelType: "telegram",
		UserID:      "user_auto",
		Rich:        rich,
	}

	// AutoLearn with recent conversation context
	sc.AutoLearn(context.Background(), msg, "我今天好开心啊！太棒了！")

	// Verify sticker was registered for "happy" emotion
	suggestions := sm.SuggestAll(EmotionHappy, "telegram")
	found := false
	for _, s := range suggestions {
		if s.FileID == "auto_file_001" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected auto-learned sticker to appear in StickerMap for happy emotion")
	}
}

func TestAutoLearnDedup(t *testing.T) {
	sm := DefaultStickerMap()
	sc := NewStickerCollector(sm, "")

	callCount := 0
	analyzer := NewAnalyzer()
	analyzer.SetLLMCall(func(ctx context.Context, systemPrompt, userMsg string) (string, error) {
		callCount++
		return `{"emotion":"sad","confidence":0.9}`, nil
	})
	sc.SetAnalyzer(analyzer)

	rich := &channel.RichMessage{}
	rich.Add(&channel.StickerComponent{FileID: "dedup_file_001"})
	msg := channel.Message{
		ChannelType: "telegram",
		UserID:      "user_dedup",
		Rich:        rich,
	}

	// First call — should analyze and register
	sc.AutoLearn(context.Background(), msg, "好难过啊")
	if callCount != 1 {
		t.Fatalf("expected 1 LLM call, got %d", callCount)
	}

	// Second call with same sticker — should skip (dedup)
	sc.AutoLearn(context.Background(), msg, "真的好伤心")
	if callCount != 1 {
		t.Fatalf("expected still 1 LLM call after dedup, got %d", callCount)
	}
}

func TestAutoLearnLowConfidenceSkips(t *testing.T) {
	sm := DefaultStickerMap()
	sc := NewStickerCollector(sm, "")

	analyzer := NewAnalyzer()
	analyzer.SetLLMCall(func(ctx context.Context, systemPrompt, userMsg string) (string, error) {
		return `{"emotion":"angry","confidence":0.3}`, nil // below threshold
	})
	sc.SetAnalyzer(analyzer)

	rich := &channel.RichMessage{}
	rich.Add(&channel.StickerComponent{FileID: "low_conf_001"})
	msg := channel.Message{
		ChannelType: "telegram",
		UserID:      "user_low",
		Rich:        rich,
	}

	sc.AutoLearn(context.Background(), msg, "嗯，还行吧")

	// Should NOT be registered because confidence < 0.6
	suggestions := sm.SuggestAll(EmotionAngry, "telegram")
	for _, s := range suggestions {
		if s.FileID == "low_conf_001" {
			t.Fatal("expected low-confidence sticker to NOT be registered")
		}
	}
}

func TestAutoLearnNeutralSkips(t *testing.T) {
	sm := DefaultStickerMap()
	sc := NewStickerCollector(sm, "")

	analyzer := NewAnalyzer()
	analyzer.SetLLMCall(func(ctx context.Context, systemPrompt, userMsg string) (string, error) {
		return `{"emotion":"neutral","confidence":0.9}`, nil
	})
	sc.SetAnalyzer(analyzer)

	rich := &channel.RichMessage{}
	rich.Add(&channel.StickerComponent{FileID: "neutral_001"})
	msg := channel.Message{
		ChannelType: "telegram",
		UserID:      "user_neutral",
		Rich:        rich,
	}

	sc.AutoLearn(context.Background(), msg, "今天天气不错")

	// Should NOT be registered for neutral emotion
	suggestions := sm.SuggestAll(EmotionNeutral, "telegram")
	for _, s := range suggestions {
		if s.FileID == "neutral_001" {
			t.Fatal("expected neutral emotion sticker to NOT be registered")
		}
	}
}

func TestAutoLearnNoAnalyzerSkips(t *testing.T) {
	sm := DefaultStickerMap()
	sc := NewStickerCollector(sm, "")
	// No analyzer set — AutoLearn should be a no-op

	rich := &channel.RichMessage{}
	rich.Add(&channel.StickerComponent{FileID: "no_analyzer_001"})
	msg := channel.Message{
		ChannelType: "telegram",
		UserID:      "user_noa",
		Rich:        rich,
	}

	sc.AutoLearn(context.Background(), msg, "hello")
	// Should not panic or register anything
}

func TestCleanupSeen(t *testing.T) {
	sm := DefaultStickerMap()
	sc := NewStickerCollector(sm, "")

	// Manually insert an old entry
	sc.mu.Lock()
	sc.seen["telegram:old_file"] = sc.seen["telegram:old_file"] // zero time
	sc.mu.Unlock()

	sc.CleanupSeen()

	sc.mu.Lock()
	_, exists := sc.seen["telegram:old_file"]
	sc.mu.Unlock()
	if exists {
		t.Fatal("expected stale entry to be cleaned up")
	}
}

func TestListStickersEmpty(t *testing.T) {
	sm := NewStickerMap() // empty map, not default
	sc := NewStickerCollector(sm, "")

	result := sc.ListStickers("")
	if !strings.Contains(result, "贴图库为空") {
		t.Fatalf("expected empty message, got: %s", result)
	}
}

func TestListStickersWithData(t *testing.T) {
	sm := DefaultStickerMap()
	sc := NewStickerCollector(sm, "")

	result := sc.ListStickers("")
	if !strings.Contains(result, "贴图库") {
		t.Fatalf("expected header, got: %s", result)
	}
	// DefaultStickerMap has telegram, line, discord entries
	if !strings.Contains(result, "telegram") && !strings.Contains(result, "line") {
		t.Fatal("expected platform names in listing")
	}
}

func TestListStickersPlatformFilter(t *testing.T) {
	sm := DefaultStickerMap()
	sc := NewStickerCollector(sm, "")

	result := sc.ListStickers("telegram")
	if !strings.Contains(result, "telegram") {
		t.Fatal("expected telegram in filtered listing")
	}
	// Should not contain other platforms
	if strings.Contains(result, "📱 line") {
		t.Fatal("expected line NOT in telegram-filtered listing")
	}
}

func TestListStickersUnknownPlatform(t *testing.T) {
	sm := DefaultStickerMap()
	sc := NewStickerCollector(sm, "")

	result := sc.ListStickers("nonexistent")
	if !strings.Contains(result, "暂无贴图") {
		t.Fatalf("expected 'no stickers' message, got: %s", result)
	}
}
