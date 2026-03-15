package emotion

import (
	"context"
	"testing"
)

func TestAllEmotionsAreValid(t *testing.T) {
	for _, e := range AllEmotions {
		if e == EmotionUnknown {
			t.Error("AllEmotions should not contain EmotionUnknown")
		}
	}
	if len(AllEmotions) != 7 {
		t.Errorf("expected 7 emotions, got %d", len(AllEmotions))
	}
}

func TestResult_IsPositiveNegative(t *testing.T) {
	tests := []struct {
		e   Emotion
		pos bool
		neg bool
	}{
		{EmotionHappy, true, false},
		{EmotionSad, false, true},
		{EmotionAngry, false, true},
		{EmotionNeutral, false, false},
		{EmotionFearful, false, true},
		{EmotionDisgusted, false, true},
		{EmotionSurprised, true, false},
		{EmotionUnknown, false, false},
	}
	for _, tt := range tests {
		r := Result{Emotion: tt.e}
		if r.IsPositive() != tt.pos {
			t.Errorf("%s IsPositive=%v want %v", tt.e, r.IsPositive(), tt.pos)
		}
		if r.IsNegative() != tt.neg {
			t.Errorf("%s IsNegative=%v want %v", tt.e, r.IsNegative(), tt.neg)
		}
	}
}

func TestResult_ContextSnippet(t *testing.T) {
	// Neutral → empty
	r := &Result{Emotion: EmotionNeutral, Confidence: 0.9}
	if s := r.ContextSnippet(); s != "" {
		t.Errorf("neutral should have empty snippet, got %q", s)
	}

	// Unknown → empty
	r = &Result{Emotion: EmotionUnknown}
	if s := r.ContextSnippet(); s != "" {
		t.Errorf("unknown should have empty snippet, got %q", s)
	}

	// Nil → empty
	var nilR *Result
	if s := nilR.ContextSnippet(); s != "" {
		t.Errorf("nil should have empty snippet, got %q", s)
	}

	// Happy → non-empty
	r = &Result{Emotion: EmotionHappy, Confidence: 0.85}
	s := r.ContextSnippet()
	if s == "" {
		t.Error("happy should have non-empty snippet")
	}
	if !contains(s, "开心") {
		t.Errorf("expected 开心 in snippet, got %q", s)
	}
}

func TestParseEmotionResponse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		emotion Emotion
		wantErr bool
	}{
		{"plain json", `{"emotion":"happy","confidence":0.9}`, EmotionHappy, false},
		{"uppercase", `{"emotion":"ANGRY","confidence":0.7}`, EmotionAngry, false},
		{"code block", "```json\n{\"emotion\":\"sad\",\"confidence\":0.8}\n```", EmotionSad, false},
		{"invalid emotion", `{"emotion":"rage","confidence":0.5}`, EmotionUnknown, false},
		{"bad json", `not json`, EmotionUnknown, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := parseEmotionResponse(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if r.Emotion != tt.emotion {
				t.Errorf("emotion = %q, want %q", r.Emotion, tt.emotion)
			}
		})
	}
}

func TestAnalyzer_Disabled(t *testing.T) {
	a := NewAnalyzer()
	a.SetEnabled(false)
	r, err := a.AnalyzeText(context.Background(), "I'm so happy!")
	if err != nil {
		t.Fatal(err)
	}
	if r.Emotion != EmotionNeutral {
		t.Errorf("disabled analyzer should return neutral, got %q", r.Emotion)
	}
	if r.Source != "default" {
		t.Errorf("source = %q, want default", r.Source)
	}
}

func TestAnalyzer_NoLLM(t *testing.T) {
	a := NewAnalyzer()
	// enabled but no llmCall set
	r, err := a.AnalyzeText(context.Background(), "I'm so happy!")
	if err != nil {
		t.Fatal(err)
	}
	if r.Emotion != EmotionNeutral {
		t.Errorf("no-llm analyzer should return neutral, got %q", r.Emotion)
	}
}

func TestAnalyzer_WithMockLLM(t *testing.T) {
	a := NewAnalyzer()
	a.SetLLMCall(func(ctx context.Context, sys, user string) (string, error) {
		return `{"emotion":"angry","confidence":0.85}`, nil
	})

	r, err := a.AnalyzeText(context.Background(), "这太让人生气了！")
	if err != nil {
		t.Fatal(err)
	}
	if r.Emotion != EmotionAngry {
		t.Errorf("emotion = %q, want angry", r.Emotion)
	}
	if r.Confidence != 0.85 {
		t.Errorf("confidence = %v, want 0.85", r.Confidence)
	}
	if r.Source != "text" {
		t.Errorf("source = %q, want text", r.Source)
	}
}

func TestAnalyzer_EmptyText(t *testing.T) {
	a := NewAnalyzer()
	a.SetLLMCall(func(ctx context.Context, sys, user string) (string, error) {
		t.Error("LLM should not be called for empty text")
		return "", nil
	})

	r, err := a.AnalyzeText(context.Background(), "  ")
	if err != nil {
		t.Fatal(err)
	}
	if r.Emotion != EmotionNeutral {
		t.Errorf("empty text should return neutral, got %q", r.Emotion)
	}
}

// ── Sticker suggestion tests ──

func TestStickerMap_Suggest(t *testing.T) {
	sm := DefaultStickerMap()

	// LINE happy sticker exists
	s := sm.Suggest(EmotionHappy, "line")
	if s == nil {
		t.Fatal("expected LINE happy sticker")
	}
	if s.Platform != "line" || s.Emotion != EmotionHappy {
		t.Errorf("unexpected sticker: %+v", s)
	}

	// Unknown platform
	if s := sm.Suggest(EmotionHappy, "wechat"); s != nil {
		t.Errorf("expected nil for unknown platform, got %+v", s)
	}

	// Neutral emotion (no mapping)
	if s := sm.Suggest(EmotionNeutral, "line"); s != nil {
		t.Errorf("expected nil for neutral emotion, got %+v", s)
	}
}

func TestStickerMap_Register(t *testing.T) {
	sm := NewStickerMap()
	sm.Register("telegram", EmotionHappy, StickerSuggestion{StickerID: "tg_happy_1"})

	s := sm.Suggest(EmotionHappy, "telegram")
	if s == nil {
		t.Fatal("expected telegram happy sticker")
	}
	if s.StickerID != "tg_happy_1" {
		t.Errorf("sticker_id = %q, want tg_happy_1", s.StickerID)
	}
	if s.Platform != "telegram" {
		t.Errorf("platform should be set to telegram, got %q", s.Platform)
	}
}

func TestStickerMap_DefaultPlatforms(t *testing.T) {
	sm := DefaultStickerMap()

	// All 13 platforms should be registered
	platforms := sm.Platforms()
	expected := []string{
		"line", "telegram", "discord", "feishu", "whatsapp", "slack",
		"signal", "wecom", "dingtalk", "kook", "wechat_official", "email", "satori",
	}
	if len(platforms) < len(expected) {
		t.Errorf("expected at least %d platforms, got %d: %v", len(expected), len(platforms), platforms)
	}

	for _, p := range expected {
		s := sm.Suggest(EmotionHappy, p)
		if s == nil {
			t.Errorf("expected happy sticker for %q, got nil", p)
			continue
		}
		if s.Platform != p {
			t.Errorf("platform mismatch for %q: got %q", p, s.Platform)
		}
	}
}

func TestStickerMap_MultiPlatformEmoji(t *testing.T) {
	sm := DefaultStickerMap()

	// Non-LINE platforms should have emoji set
	emojiPlatforms := []string{"telegram", "discord", "slack", "whatsapp", "signal", "wecom", "dingtalk", "kook", "feishu", "wechat_official", "email", "satori"}
	for _, p := range emojiPlatforms {
		s := sm.Suggest(EmotionHappy, p)
		if s == nil {
			t.Fatalf("no happy sticker for %q", p)
		}
		if s.Emoji == "" {
			t.Errorf("%q happy sticker should have emoji fallback", p)
		}
	}

	// LINE should have packageID/stickerID instead of emoji
	lineS := sm.Suggest(EmotionHappy, "line")
	if lineS == nil {
		t.Fatal("no LINE happy sticker")
	}
	if lineS.PackageID == "" || lineS.StickerID == "" {
		t.Error("LINE sticker should have PackageID and StickerID")
	}
}

func TestStickerMap_SuggestMulti(t *testing.T) {
	sm := DefaultStickerMap()

	multi := sm.SuggestMulti(EmotionHappy)
	if len(multi) == 0 {
		t.Fatal("SuggestMulti returned empty map")
	}

	// Line and Telegram should both be present
	if _, ok := multi["line"]; !ok {
		t.Error("SuggestMulti missing line")
	}
	if _, ok := multi["telegram"]; !ok {
		t.Error("SuggestMulti missing telegram")
	}

	// Neutral emotion should return empty/nil for most platforms
	neutral := sm.SuggestMulti(EmotionNeutral)
	if len(neutral) != 0 {
		t.Errorf("SuggestMulti for neutral should be empty, got %d entries", len(neutral))
	}
}

func TestStickerMap_AllEmotions(t *testing.T) {
	sm := DefaultStickerMap()

	// Every non-neutral/unknown emotion should have at least one sticker for each platform
	emotions := []Emotion{EmotionHappy, EmotionSad, EmotionAngry, EmotionSurprised, EmotionFearful, EmotionDisgusted}
	platforms := []string{"telegram", "discord", "slack", "feishu"}

	for _, p := range platforms {
		for _, e := range emotions {
			s := sm.Suggest(e, p)
			if s == nil {
				t.Errorf("no sticker for %s/%s", p, e)
			}
		}
	}
}

func TestStickerMap_SlackShortcode(t *testing.T) {
	sm := DefaultStickerMap()

	s := sm.Suggest(EmotionHappy, "slack")
	if s == nil {
		t.Fatal("no Slack happy sticker")
	}
	if s.Emoji == "" {
		t.Error("Slack sticker should have emoji")
	}
	if s.StickerID == "" {
		t.Error("Slack sticker should have shortcode in StickerID")
	}
}

func TestStickerMap_NewFields(t *testing.T) {
	sm := NewStickerMap()

	// Register with FileID (Telegram-style)
	sm.Register("telegram", EmotionHappy, StickerSuggestion{
		FileID:  "CAACAgIAAxkBAAIBL...",
		SetName: "HappyAnimals",
		Emoji:   "😊",
	})
	s := sm.Suggest(EmotionHappy, "telegram")
	if s == nil {
		t.Fatal("expected telegram sticker")
	}
	if s.FileID != "CAACAgIAAxkBAAIBL..." {
		t.Errorf("FileID mismatch: %q", s.FileID)
	}
	if s.SetName != "HappyAnimals" {
		t.Errorf("SetName mismatch: %q", s.SetName)
	}

	// Register with CDNURL (WeChat-style)
	sm.Register("wechat", EmotionSad, StickerSuggestion{
		CDNURL: "https://wx.cdn/emoji/sad.gif",
		Emoji:  "😢",
	})
	ws := sm.Suggest(EmotionSad, "wechat")
	if ws == nil {
		t.Fatal("expected wechat sticker")
	}
	if ws.CDNURL != "https://wx.cdn/emoji/sad.gif" {
		t.Errorf("CDNURL mismatch: %q", ws.CDNURL)
	}
}

func TestStickerMap_Clear(t *testing.T) {
	sm := DefaultStickerMap()

	// Verify exists
	if sm.Suggest(EmotionHappy, "telegram") == nil {
		t.Fatal("telegram happy should exist before clear")
	}

	sm.Clear("telegram", EmotionHappy)

	if sm.Suggest(EmotionHappy, "telegram") != nil {
		t.Error("telegram happy should be nil after clear")
	}

	// Other platforms unaffected
	if sm.Suggest(EmotionHappy, "discord") == nil {
		t.Error("discord happy should still exist after clearing telegram")
	}
}

func TestStickerMap_Export(t *testing.T) {
	sm := DefaultStickerMap()
	exported := sm.Export()

	if len(exported) == 0 {
		t.Fatal("Export returned empty map")
	}

	// Verify LINE data in export
	lineEmotions, ok := exported["line"]
	if !ok {
		t.Fatal("Export missing line platform")
	}
	if _, ok := lineEmotions[EmotionHappy]; !ok {
		t.Error("Export missing line/happy")
	}

	// Verify telegram data in export
	tgEmotions, ok := exported["telegram"]
	if !ok {
		t.Fatal("Export missing telegram platform")
	}
	if _, ok := tgEmotions[EmotionHappy]; !ok {
		t.Error("Export missing telegram/happy")
	}
}

func TestAnalyzer_Locale(t *testing.T) {
	a := NewAnalyzer()
	if a.Locale() != "zh" {
		t.Errorf("default locale = %q, want zh", a.Locale())
	}
	a.SetLocale("en")
	if a.Locale() != "en" {
		t.Errorf("after set locale = %q, want en", a.Locale())
	}
	a.SetLocale("")
	if a.Locale() != "zh" {
		t.Errorf("empty locale should fallback to zh, got %q", a.Locale())
	}
}

func TestAnalyzer_LocalePrompt(t *testing.T) {
	a := NewAnalyzer()
	called := ""
	a.SetLLMCall(func(ctx context.Context, sys, user string) (string, error) {
		called = sys
		return `{"emotion":"happy","confidence":0.9}`, nil
	})

	// Default zh
	a.AnalyzeText(context.Background(), "hello")
	if !containsStr(called, "情绪分析专家") {
		t.Error("zh prompt should contain Chinese text")
	}

	// Switch to en
	a.SetLocale("en")
	a.AnalyzeText(context.Background(), "hello")
	if !containsStr(called, "emotion analysis expert") {
		t.Error("en prompt should contain English text")
	}

	// Switch to ja
	a.SetLocale("ja")
	a.AnalyzeText(context.Background(), "hello")
	if !containsStr(called, "感情分析の専門家") {
		t.Error("ja prompt should contain Japanese text")
	}
}

func TestContextSnippetLocale(t *testing.T) {
	r := &Result{Emotion: EmotionHappy, Confidence: 0.85}

	zh := r.ContextSnippetLocale("zh")
	if !containsStr(zh, "开心") || !containsStr(zh, "用户情绪") {
		t.Errorf("zh snippet unexpected: %q", zh)
	}

	en := r.ContextSnippetLocale("en")
	if !containsStr(en, "happy") || !containsStr(en, "User emotion") {
		t.Errorf("en snippet unexpected: %q", en)
	}

	ja := r.ContextSnippetLocale("ja")
	if !containsStr(ja, "嬉しい") || !containsStr(ja, "ユーザーの感情") {
		t.Errorf("ja snippet unexpected: %q", ja)
	}

	// Neutral → empty for all locales
	nr := &Result{Emotion: EmotionNeutral}
	for _, loc := range []string{"zh", "en", "ja"} {
		if s := nr.ContextSnippetLocale(loc); s != "" {
			t.Errorf("neutral %s should be empty, got %q", loc, s)
		}
	}
}

func TestEmotionLabelLocale(t *testing.T) {
	tests := []struct {
		e      Emotion
		locale string
		want   string
	}{
		{EmotionHappy, "zh", "开心"},
		{EmotionHappy, "en", "happy"},
		{EmotionHappy, "ja", "嬉しい"},
		{EmotionAngry, "en", "angry"},
		{EmotionSad, "ja", "悲しい"},
	}
	for _, tt := range tests {
		label := emotionLabelLocale(tt.e, tt.locale)
		if !containsStr(label, tt.want) {
			t.Errorf("emotionLabelLocale(%s, %s) = %q, want contains %q", tt.e, tt.locale, label, tt.want)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
