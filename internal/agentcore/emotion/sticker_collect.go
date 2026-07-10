package emotion

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"yunque-agent/internal/execution/channel"
)

// StickerCollector manages interactive sticker collection sessions.
// When a user sends /sticker (or equivalent), a session is created.
// The next sticker message from that user is captured and added to the StickerMap.
type StickerCollector struct {
	stickerMap *StickerMap
	sessions   map[string]*collectSession // key: "channelType:userID"
	seen       map[string]time.Time       // dedup key: "platform:fileID_or_stickerID" → last seen
	mu         sync.Mutex
	saveFile   string // path to persist sticker data
	analyzer   *Analyzer
}

type collectSession struct {
	Emotion   Emotion   // target emotion category for the sticker
	Platform  string    // source platform
	CreatedAt time.Time // session creation time (expires after 2 minutes)
}

// NewStickerCollector creates a collector linked to a StickerMap.
func NewStickerCollector(sm *StickerMap, saveFile string) *StickerCollector {
	return &StickerCollector{
		stickerMap: sm,
		sessions:   make(map[string]*collectSession),
		seen:       make(map[string]time.Time),
		saveFile:   saveFile,
	}
}

// SetAnalyzer sets the emotion analyzer for auto-learning.
func (sc *StickerCollector) SetAnalyzer(a *Analyzer) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.analyzer = a
}

// sessionKey builds a session lookup key.
func sessionKey(channelType, userID string) string {
	return channelType + ":" + userID
}

// StartSession begins a sticker collection session for a user.
// emotion is the target emotion category (e.g., "happy", "sad").
// Returns the prompt message to show the user.
func (sc *StickerCollector) StartSession(channelType, userID string, emotion Emotion) string {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	key := sessionKey(channelType, userID)
	sc.sessions[key] = &collectSession{
		Emotion:   emotion,
		Platform:  channelType,
		CreatedAt: time.Now(),
	}

	emotionLabel := string(emotion)
	switch emotion {
	case EmotionHappy:
		emotionLabel = "开心 😊"
	case EmotionSad:
		emotionLabel = "悲伤 😢"
	case EmotionAngry:
		emotionLabel = "愤怒 😠"
	case EmotionFearful:
		emotionLabel = "害怕 😰"
	case EmotionDisgusted:
		emotionLabel = "反感 😒"
	case EmotionSurprised:
		emotionLabel = "惊讶 😮"
	case EmotionNeutral:
		emotionLabel = "中性 😐"
	}

	return fmt.Sprintf("🎨 贴图收集模式已开启！\n\n"+
		"请发送一个表情包/贴图，我会把它记录为「%s」情绪的推荐贴图。\n\n"+
		"⏰ 2 分钟内有效，发送 /cancel 可取消。", emotionLabel)
}

// CancelSession cancels an active collection session.
func (sc *StickerCollector) CancelSession(channelType, userID string) bool {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	key := sessionKey(channelType, userID)
	if _, ok := sc.sessions[key]; ok {
		delete(sc.sessions, key)
		return true
	}
	return false
}

// HasActiveSession checks if a user has an active collection session.
func (sc *StickerCollector) HasActiveSession(channelType, userID string) bool {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	key := sessionKey(channelType, userID)
	sess, ok := sc.sessions[key]
	if !ok {
		return false
	}
	// Check expiry (2 minutes)
	if time.Since(sess.CreatedAt) > 2*time.Minute {
		delete(sc.sessions, key)
		return false
	}
	return true
}

// TryCollect attempts to collect a sticker from an incoming message.
// Returns (success, response message). If true, the sticker was collected and
// the caller should send the response instead of forwarding to the planner.
func (sc *StickerCollector) TryCollect(msg channel.Message) (bool, string) {
	if msg.Rich == nil || !msg.Rich.HasType(channel.ComponentSticker) {
		return false, ""
	}

	sc.mu.Lock()
	key := sessionKey(msg.ChannelType, msg.UserID)
	sess, ok := sc.sessions[key]
	if !ok {
		sc.mu.Unlock()
		return false, ""
	}
	// Check expiry
	if time.Since(sess.CreatedAt) > 2*time.Minute {
		delete(sc.sessions, key)
		sc.mu.Unlock()
		return false, ""
	}
	// Remove session (one-shot)
	delete(sc.sessions, key)
	sc.mu.Unlock()

	// Extract sticker component
	comp := msg.Rich.GetFirst(channel.ComponentSticker)
	sticker, ok := comp.(*channel.StickerComponent)
	if !ok || sticker == nil {
		return false, ""
	}

	// Register to sticker map
	suggestion := StickerSuggestion{
		PackageID: sticker.PackageID,
		StickerID: sticker.StickerID,
		Platform:  sess.Platform,
		Emotion:   sess.Emotion,
		FileID:    sticker.FileID,
		SetName:   sticker.SetName,
		Emoji:     sticker.Emoji,
	}
	if sticker.URL != "" {
		suggestion.CDNURL = sticker.URL
	} else if url := sticker.StickerURL(); url != "" {
		suggestion.CDNURL = url
	}

	sc.stickerMap.Register(sess.Platform, sess.Emotion, suggestion)

	// Persist to file
	if sc.saveFile != "" {
		sc.persistToFile()
	}

	slog.Info("sticker collected",
		"platform", sess.Platform,
		"emotion", sess.Emotion,
		"sticker_id", sticker.StickerID,
		"file_id", sticker.FileID,
	)

	return true, fmt.Sprintf("✅ 贴图已收集！\n\n"+
		"📦 贴图: %s\n"+
		"🎭 情绪: %s\n"+
		"📱 平台: %s\n\n"+
		"下次检测到该情绪时，可能会推荐这个贴图。",
		stickerLabel(sticker), string(sess.Emotion), sess.Platform)
}

// ParseStickerCommand parses a /sticker command and returns the emotion.
// Supports: /sticker happy, /sticker 开心, /sticker sad, etc.
// Returns (emotion, ok). If ok is false, the command format was invalid.
func ParseStickerCommand(text string) (Emotion, bool) {
	// Extract the argument after "/sticker"
	if len(text) <= 8 { // "/sticker" is 8 chars
		return EmotionHappy, true // default to happy if no arg
	}
	arg := text[8:]
	if len(arg) > 0 && arg[0] == ' ' {
		arg = arg[1:]
	}
	if arg == "" {
		return EmotionHappy, true
	}

	// Map Chinese/English labels to emotions
	emotionMap := map[string]Emotion{
		"happy": EmotionHappy, "开心": EmotionHappy, "高兴": EmotionHappy, "快乐": EmotionHappy,
		"sad": EmotionSad, "悲伤": EmotionSad, "难过": EmotionSad, "伤心": EmotionSad,
		"angry": EmotionAngry, "愤怒": EmotionAngry, "生气": EmotionAngry,
		"fearful": EmotionFearful, "害怕": EmotionFearful, "恐惧": EmotionFearful,
		"disgusted": EmotionDisgusted, "反感": EmotionDisgusted, "厌恶": EmotionDisgusted,
		"surprised": EmotionSurprised, "惊讶": EmotionSurprised, "吃惊": EmotionSurprised,
		"neutral": EmotionNeutral, "中性": EmotionNeutral, "平静": EmotionNeutral,
	}

	if em, ok := emotionMap[arg]; ok {
		return em, true
	}
	return "", false
}

func stickerLabel(s *channel.StickerComponent) string {
	if s.Emoji != "" {
		return s.Emoji
	}
	if s.SetName != "" {
		return s.SetName
	}
	if s.FileID != "" {
		id := s.FileID
		if len(id) > 16 {
			id = id[:16] + "..."
		}
		return id
	}
	return fmt.Sprintf("pkg=%s stk=%s", s.PackageID, s.StickerID)
}

func (sc *StickerCollector) persistToFile() {
	if sc.saveFile == "" {
		return
	}
	data := sc.stickerMap.Export()
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		slog.Warn("sticker persist marshal failed", "err", err)
		return
	}
	if err := os.WriteFile(sc.saveFile, b, 0644); err != nil {
		slog.Warn("sticker persist write failed", "err", err)
	}
}

// ────────────────────────────────────────
// Auto-learn: implicit sticker discovery
// ────────────────────────────────────────

const (
	// autoLearnCooldown prevents re-learning the same sticker within this window.
	autoLearnCooldown = 24 * time.Hour
	// autoLearnMinConfidence is the minimum emotion confidence to auto-register.
	autoLearnMinConfidence = 0.6
)

// stickerKey builds a deduplication key for a sticker.
func stickerKey(platform string, s *channel.StickerComponent) string {
	if s.FileID != "" {
		return platform + ":" + s.FileID
	}
	return fmt.Sprintf("%s:%s:%s", platform, s.PackageID, s.StickerID)
}

// AutoLearn observes a sticker message and, if the recent conversation context
// indicates a clear emotion, automatically registers the sticker.
// This runs asynchronously (caller should use `go sc.AutoLearn(...)`).
//
// Parameters:
//   - ctx: context for the LLM call
//   - msg: the incoming message containing a StickerComponent
//   - recentText: concatenated recent conversation text (last 3-5 messages)
//     used to infer the current emotional context
func (sc *StickerCollector) AutoLearn(ctx context.Context, msg channel.Message, recentText string) {
	if sc.analyzer == nil || !sc.analyzer.Enabled() {
		return
	}

	// Extract sticker
	if msg.Rich == nil || !msg.Rich.HasType(channel.ComponentSticker) {
		return
	}
	comp := msg.Rich.GetFirst(channel.ComponentSticker)
	sticker, ok := comp.(*channel.StickerComponent)
	if !ok || sticker == nil {
		return
	}

	// Dedup check: skip if recently seen
	key := stickerKey(msg.ChannelType, sticker)
	sc.mu.Lock()
	if lastSeen, exists := sc.seen[key]; exists && time.Since(lastSeen) < autoLearnCooldown {
		sc.mu.Unlock()
		return
	}
	// Mark as seen now (even if analysis fails, to avoid repeated LLM calls)
	sc.seen[key] = time.Now()
	sc.mu.Unlock()

	// Skip if no conversation context to analyze
	if len(recentText) < 5 {
		return
	}

	// Analyze emotion from recent conversation context
	result, err := sc.analyzer.AnalyzeText(ctx, recentText)
	if err != nil || result == nil {
		return
	}

	// Only auto-learn with sufficient confidence and non-neutral emotions
	if result.Confidence < autoLearnMinConfidence {
		return
	}
	if result.Emotion == EmotionNeutral || result.Emotion == EmotionUnknown {
		return
	}

	// Register the sticker
	suggestion := StickerSuggestion{
		PackageID: sticker.PackageID,
		StickerID: sticker.StickerID,
		Platform:  msg.ChannelType,
		Emotion:   result.Emotion,
		FileID:    sticker.FileID,
		SetName:   sticker.SetName,
		Emoji:     sticker.Emoji,
	}
	if sticker.URL != "" {
		suggestion.CDNURL = sticker.URL
	} else if url := sticker.StickerURL(); url != "" {
		suggestion.CDNURL = url
	}

	sc.stickerMap.Register(msg.ChannelType, result.Emotion, suggestion)

	if sc.saveFile != "" {
		sc.persistToFile()
	}

	slog.Info("sticker auto-learned",
		"platform", msg.ChannelType,
		"emotion", result.Emotion,
		"confidence", result.Confidence,
		"sticker_id", sticker.StickerID,
		"file_id", sticker.FileID,
	)
}

// CleanupSeen purges expired dedup entries. Safe to call periodically.
func (sc *StickerCollector) CleanupSeen() {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	now := time.Now()
	for k, t := range sc.seen {
		if now.Sub(t) > autoLearnCooldown {
			delete(sc.seen, k)
		}
	}
}

// ────────────────────────────────────────
// Sticker library listing
// ────────────────────────────────────────

// emotionEmoji maps emotions to display emoji.
var emotionEmoji = map[Emotion]string{
	EmotionHappy:     "😊",
	EmotionSad:       "😢",
	EmotionAngry:     "😠",
	EmotionFearful:   "😰",
	EmotionDisgusted: "😒",
	EmotionSurprised: "😮",
	EmotionNeutral:   "😐",
}

// ListStickers returns a formatted text summary of all stickers for a platform.
// If platform is empty, shows all platforms.
func (sc *StickerCollector) ListStickers(platform string) string {
	data := sc.stickerMap.Export()
	if len(data) == 0 {
		return "📦 贴图库为空，还没有收集任何贴图。\n\n使用 /sticker [情绪] 开始收集，或者直接发送贴图让我自动学习！"
	}

	var b strings.Builder
	b.WriteString("📦 **贴图库**\n\n")

	totalStickers := 0
	platforms := sortedKeys(data)
	for _, plat := range platforms {
		if platform != "" && plat != platform {
			continue
		}
		emotions := data[plat]
		platCount := 0
		for _, stickers := range emotions {
			platCount += len(stickers)
		}
		totalStickers += platCount

		b.WriteString(fmt.Sprintf("**📱 %s** (%d 个贴图)\n", plat, platCount))

		for _, em := range AllEmotions {
			stickers, ok := emotions[em]
			if !ok || len(stickers) == 0 {
				continue
			}
			emoji := emotionEmoji[em]
			if emoji == "" {
				emoji = "❓"
			}
			b.WriteString(fmt.Sprintf("  %s %s (%d):", emoji, string(em), len(stickers)))
			for i, s := range stickers {
				if i >= 5 { // show max 5 per emotion
					b.WriteString(fmt.Sprintf(" +%d more", len(stickers)-5))
					break
				}
				label := stickerLabel(&channel.StickerComponent{
					Emoji:     s.Emoji,
					SetName:   s.SetName,
					FileID:    s.FileID,
					PackageID: s.PackageID,
					StickerID: s.StickerID,
				})
				b.WriteString(" " + label)
				if i < len(stickers)-1 && i < 4 {
					b.WriteString(",")
				}
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	if platform != "" && totalStickers == 0 {
		return fmt.Sprintf("📦 平台 %s 暂无贴图。\n\n使用 /sticker [情绪] 开始收集！", platform)
	}

	b.WriteString(fmt.Sprintf("📊 共 %d 个贴图，覆盖 %d 个平台", totalStickers, len(platforms)))
	return b.String()
}

// sortedKeys returns map keys in sorted order.
func sortedKeys(m map[string]map[Emotion][]StickerSuggestion) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
