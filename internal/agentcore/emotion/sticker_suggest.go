package emotion

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
)

// StickerSuggestion represents a recommended sticker for a given emotion.
type StickerSuggestion struct {
	PackageID string  `json:"package_id"`
	StickerID string  `json:"sticker_id"`
	Platform  string  `json:"platform"`
	Emotion   Emotion `json:"emotion"`
	FileID    string  `json:"file_id,omitempty"`  // Telegram file_id / platform media ID
	SetName   string  `json:"set_name,omitempty"` // Telegram sticker set name
	CDNURL    string  `json:"cdnurl,omitempty"`   // WeChat CDN URL or direct image URL
	Emoji     string  `json:"emoji,omitempty"`    // Unicode emoji fallback (Discord, etc.)
}

// StickerMap stores per-platform emotion-to-sticker mappings.
type StickerMap struct {
	// platform -> emotion -> []StickerSuggestion
	mappings map[string]map[Emotion][]StickerSuggestion
}

// NewStickerMap creates an empty sticker map.
func NewStickerMap() *StickerMap {
	return &StickerMap{mappings: make(map[string]map[Emotion][]StickerSuggestion)}
}

// Register adds sticker suggestions for a platform+emotion combination.
func (sm *StickerMap) Register(platform string, emotion Emotion, stickers ...StickerSuggestion) {
	if sm.mappings[platform] == nil {
		sm.mappings[platform] = make(map[Emotion][]StickerSuggestion)
	}
	for i := range stickers {
		stickers[i].Platform = platform
		stickers[i].Emotion = emotion
	}
	sm.mappings[platform][emotion] = append(sm.mappings[platform][emotion], stickers...)
}

// Suggest returns the first matching sticker for the given emotion and platform.
// Returns nil if no mapping exists.
func (sm *StickerMap) Suggest(emotion Emotion, platform string) *StickerSuggestion {
	if em, ok := sm.mappings[platform]; ok {
		if stickers, ok := em[emotion]; ok && len(stickers) > 0 {
			s := stickers[0]
			return &s
		}
	}
	return nil
}

// SuggestAll returns all matching stickers for the given emotion and platform.
func (sm *StickerMap) SuggestAll(emotion Emotion, platform string) []StickerSuggestion {
	if em, ok := sm.mappings[platform]; ok {
		return em[emotion]
	}
	return nil
}

// DefaultStickerMap returns a StickerMap pre-loaded with default sticker sets for multiple platforms.
// LINE: Brown & Friends (package 11537) — publicly available free stickers.
// Telegram: emoji-based stickers (no file_id; channels can use emoji or upload custom sticker sets).
// Discord: unicode emoji fallback.
// Users can override or extend these with a custom sticker JSON file.
func DefaultStickerMap() *StickerMap {
	sm := NewStickerMap()

	// ── LINE: Brown & Friends (package 11537) ──
	sm.Register("line", EmotionHappy,
		StickerSuggestion{PackageID: "11537", StickerID: "52002734"},
		StickerSuggestion{PackageID: "11537", StickerID: "52002735"},
	)
	sm.Register("line", EmotionSad,
		StickerSuggestion{PackageID: "11537", StickerID: "52002739"},
		StickerSuggestion{PackageID: "11537", StickerID: "52002743"},
	)
	sm.Register("line", EmotionAngry,
		StickerSuggestion{PackageID: "11537", StickerID: "52002740"},
	)
	sm.Register("line", EmotionSurprised,
		StickerSuggestion{PackageID: "11537", StickerID: "52002737"},
	)
	sm.Register("line", EmotionFearful,
		StickerSuggestion{PackageID: "11537", StickerID: "52002742"},
	)

	// ── Telegram: emoji-based stickers (use Emoji field; file_id can be added per-bot) ──
	sm.Register("telegram", EmotionHappy, StickerSuggestion{Emoji: "😊"})
	sm.Register("telegram", EmotionSad, StickerSuggestion{Emoji: "😢"})
	sm.Register("telegram", EmotionAngry, StickerSuggestion{Emoji: "😠"})
	sm.Register("telegram", EmotionSurprised, StickerSuggestion{Emoji: "😮"})
	sm.Register("telegram", EmotionFearful, StickerSuggestion{Emoji: "😰"})
	sm.Register("telegram", EmotionDisgusted, StickerSuggestion{Emoji: "😒"})

	// ── Discord: unicode emoji (sent as message reaction or text) ──
	sm.Register("discord", EmotionHappy, StickerSuggestion{Emoji: "😊"})
	sm.Register("discord", EmotionSad, StickerSuggestion{Emoji: "😢"})
	sm.Register("discord", EmotionAngry, StickerSuggestion{Emoji: "😠"})
	sm.Register("discord", EmotionSurprised, StickerSuggestion{Emoji: "😮"})
	sm.Register("discord", EmotionFearful, StickerSuggestion{Emoji: "😰"})
	sm.Register("discord", EmotionDisgusted, StickerSuggestion{Emoji: "😒"})

	// ── Feishu: unicode emoji (飞书支持emoji文本) ──
	sm.Register("feishu", EmotionHappy, StickerSuggestion{Emoji: "😊"})
	sm.Register("feishu", EmotionSad, StickerSuggestion{Emoji: "😢"})
	sm.Register("feishu", EmotionAngry, StickerSuggestion{Emoji: "😠"})
	sm.Register("feishu", EmotionSurprised, StickerSuggestion{Emoji: "😮"})
	sm.Register("feishu", EmotionFearful, StickerSuggestion{Emoji: "😰"})
	sm.Register("feishu", EmotionDisgusted, StickerSuggestion{Emoji: "😒"})

	// ── WhatsApp: emoji text (sticker media_id requires upload; use emoji as fallback) ──
	sm.Register("whatsapp", EmotionHappy, StickerSuggestion{Emoji: "😊"})
	sm.Register("whatsapp", EmotionSad, StickerSuggestion{Emoji: "😢"})
	sm.Register("whatsapp", EmotionAngry, StickerSuggestion{Emoji: "😠"})
	sm.Register("whatsapp", EmotionSurprised, StickerSuggestion{Emoji: "😮"})
	sm.Register("whatsapp", EmotionFearful, StickerSuggestion{Emoji: "😰"})
	sm.Register("whatsapp", EmotionDisgusted, StickerSuggestion{Emoji: "😒"})

	// ── Slack: emoji shortcode (Slack uses :name: format) ──
	sm.Register("slack", EmotionHappy, StickerSuggestion{Emoji: "😊", StickerID: "blush"})
	sm.Register("slack", EmotionSad, StickerSuggestion{Emoji: "😢", StickerID: "cry"})
	sm.Register("slack", EmotionAngry, StickerSuggestion{Emoji: "😠", StickerID: "angry"})
	sm.Register("slack", EmotionSurprised, StickerSuggestion{Emoji: "😮", StickerID: "open_mouth"})
	sm.Register("slack", EmotionFearful, StickerSuggestion{Emoji: "😰", StickerID: "cold_sweat"})
	sm.Register("slack", EmotionDisgusted, StickerSuggestion{Emoji: "😒", StickerID: "unamused"})

	// ── Signal: emoji text (Signal stickers require pack upload; use emoji fallback) ──
	sm.Register("signal", EmotionHappy, StickerSuggestion{Emoji: "😊"})
	sm.Register("signal", EmotionSad, StickerSuggestion{Emoji: "😢"})
	sm.Register("signal", EmotionAngry, StickerSuggestion{Emoji: "😠"})
	sm.Register("signal", EmotionSurprised, StickerSuggestion{Emoji: "😮"})
	sm.Register("signal", EmotionFearful, StickerSuggestion{Emoji: "😰"})
	sm.Register("signal", EmotionDisgusted, StickerSuggestion{Emoji: "😒"})

	// ── WeCom: emoji in text/markdown (企业微信在消息内容中使用emoji) ──
	sm.Register("wecom", EmotionHappy, StickerSuggestion{Emoji: "😊"})
	sm.Register("wecom", EmotionSad, StickerSuggestion{Emoji: "😢"})
	sm.Register("wecom", EmotionAngry, StickerSuggestion{Emoji: "😠"})
	sm.Register("wecom", EmotionSurprised, StickerSuggestion{Emoji: "😮"})
	sm.Register("wecom", EmotionFearful, StickerSuggestion{Emoji: "😰"})
	sm.Register("wecom", EmotionDisgusted, StickerSuggestion{Emoji: "😒"})

	// ── DingTalk: emoji in text/markdown (钉钉在消息中使用emoji) ──
	sm.Register("dingtalk", EmotionHappy, StickerSuggestion{Emoji: "😊"})
	sm.Register("dingtalk", EmotionSad, StickerSuggestion{Emoji: "😢"})
	sm.Register("dingtalk", EmotionAngry, StickerSuggestion{Emoji: "😠"})
	sm.Register("dingtalk", EmotionSurprised, StickerSuggestion{Emoji: "😮"})
	sm.Register("dingtalk", EmotionFearful, StickerSuggestion{Emoji: "😰"})
	sm.Register("dingtalk", EmotionDisgusted, StickerSuggestion{Emoji: "😒"})

	// ── Kook: emoji shortcode and unicode (KOOK支持emoji) ──
	sm.Register("kook", EmotionHappy, StickerSuggestion{Emoji: "😊"})
	sm.Register("kook", EmotionSad, StickerSuggestion{Emoji: "😢"})
	sm.Register("kook", EmotionAngry, StickerSuggestion{Emoji: "😠"})
	sm.Register("kook", EmotionSurprised, StickerSuggestion{Emoji: "😮"})
	sm.Register("kook", EmotionFearful, StickerSuggestion{Emoji: "😰"})
	sm.Register("kook", EmotionDisgusted, StickerSuggestion{Emoji: "😒"})

	// ── WeChat Official: emoji in text (微信公众号文本中嵌入emoji) ──
	sm.Register("wechat_official", EmotionHappy, StickerSuggestion{Emoji: "😊"})
	sm.Register("wechat_official", EmotionSad, StickerSuggestion{Emoji: "😢"})
	sm.Register("wechat_official", EmotionAngry, StickerSuggestion{Emoji: "😠"})
	sm.Register("wechat_official", EmotionSurprised, StickerSuggestion{Emoji: "😮"})
	sm.Register("wechat_official", EmotionFearful, StickerSuggestion{Emoji: "😰"})
	sm.Register("wechat_official", EmotionDisgusted, StickerSuggestion{Emoji: "😒"})

	// ── Email: unicode emoji (HTML邮件中使用unicode emoji) ──
	sm.Register("email", EmotionHappy, StickerSuggestion{Emoji: "😊"})
	sm.Register("email", EmotionSad, StickerSuggestion{Emoji: "😢"})
	sm.Register("email", EmotionAngry, StickerSuggestion{Emoji: "😠"})
	sm.Register("email", EmotionSurprised, StickerSuggestion{Emoji: "😮"})
	sm.Register("email", EmotionFearful, StickerSuggestion{Emoji: "😰"})
	sm.Register("email", EmotionDisgusted, StickerSuggestion{Emoji: "😒"})

	// ── Satori: unicode emoji (Satori协议使用emoji) ──
	sm.Register("satori", EmotionHappy, StickerSuggestion{Emoji: "😊"})
	sm.Register("satori", EmotionSad, StickerSuggestion{Emoji: "😢"})
	sm.Register("satori", EmotionAngry, StickerSuggestion{Emoji: "😠"})
	sm.Register("satori", EmotionSurprised, StickerSuggestion{Emoji: "😮"})
	sm.Register("satori", EmotionFearful, StickerSuggestion{Emoji: "😰"})
	sm.Register("satori", EmotionDisgusted, StickerSuggestion{Emoji: "😒"})

	return sm
}

// LoadFromFile loads additional sticker mappings from a JSON file.
// File format: {"line": {"happy": [{"package_id":"X","sticker_id":"Y"}, ...], ...}, ...}
// Merges with existing mappings (does not replace).
func (sm *StickerMap) LoadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("sticker map: read %s: %w", path, err)
	}

	var raw map[string]map[string][]StickerSuggestion
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("sticker map: parse %s: %w", path, err)
	}

	for platform, emotionMap := range raw {
		for emotionStr, stickers := range emotionMap {
			em := Emotion(emotionStr)
			sm.Register(platform, em, stickers...)
		}
	}
	slog.Info("sticker map loaded from file", "path", path, "platforms", len(raw))
	return nil
}

// Export returns the full mapping data for serialization.
func (sm *StickerMap) Export() map[string]map[Emotion][]StickerSuggestion {
	out := make(map[string]map[Emotion][]StickerSuggestion)
	for platform, emotions := range sm.mappings {
		out[platform] = make(map[Emotion][]StickerSuggestion)
		for e, stickers := range emotions {
			cp := make([]StickerSuggestion, len(stickers))
			copy(cp, stickers)
			out[platform][e] = cp
		}
	}
	return out
}

// SuggestMulti returns the first matching sticker for each registered platform.
// Useful for API responses that need to serve multiple client types.
func (sm *StickerMap) SuggestMulti(emotion Emotion) map[string]*StickerSuggestion {
	out := make(map[string]*StickerSuggestion)
	for platform := range sm.mappings {
		if s := sm.Suggest(emotion, platform); s != nil {
			out[platform] = s
		}
	}
	return out
}

// Platforms returns the list of registered platform names.
func (sm *StickerMap) Platforms() []string {
	platforms := make([]string, 0, len(sm.mappings))
	for p := range sm.mappings {
		platforms = append(platforms, p)
	}
	return platforms
}

// Clear removes all stickers for a platform+emotion combination.
func (sm *StickerMap) Clear(platform string, emotion Emotion) {
	if em, ok := sm.mappings[platform]; ok {
		delete(em, emotion)
	}
}
