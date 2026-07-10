package emotion

import (
	"fmt"
	"time"

	"yunque-agent/internal/execution/channel"
)

// StartAddSession starts a bulk sticker-set collection session.
// Returns the prompt message to show the user.
func (sc *StickerCollector) StartAddSession(channelType, userID string) string {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	key := "add:" + sessionKey(channelType, userID)
	sc.sessions[key] = &collectSession{
		Emotion:   "auto",
		Platform:  channelType,
		CreatedAt: time.Now(),
	}

	return "📦 批量贴图收集模式已开启！\n\n" +
		"请依次发送贴图，我会自动学习每张贴图的情绪。\n" +
		"发送 /cancel 结束收集。\n\n" +
		"⏰ 5 分钟内有效。"
}

// IsAddSession checks if the user is in a bulk-add session.
func (sc *StickerCollector) IsAddSession(channelType, userID string) bool {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	key := "add:" + sessionKey(channelType, userID)
	sess, ok := sc.sessions[key]
	if !ok {
		return false
	}
	if time.Since(sess.CreatedAt) > 5*time.Minute {
		delete(sc.sessions, key)
		return false
	}
	return true
}

// ConsumeAddSession processes a sticker in a bulk-add session.
// It auto-learns the sticker and keeps the session alive.
func (sc *StickerCollector) ConsumeAddSession(channelType, userID string) {
	sc.mu.Lock()
	key := "add:" + sessionKey(channelType, userID)
	if sess, ok := sc.sessions[key]; ok {
		sess.CreatedAt = time.Now() // refresh timeout
	}
	sc.mu.Unlock()
}

// LearnStickerSet fetches and learns all stickers from a sticker set.
func (sc *StickerCollector) LearnStickerSet(channelType string, stickers []channel.StickerComponent, defaultEmotion ...Emotion) string {
	emotion := Emotion("auto")
	if len(defaultEmotion) > 0 {
		emotion = defaultEmotion[0]
	}
	count := 0
	for _, s := range stickers {
		suggestion := StickerSuggestion{
			PackageID: s.PackageID,
			StickerID: s.StickerID,
			Platform:  channelType,
			Emotion:   emotion,
			FileID:    s.FileID,
			SetName:   s.SetName,
			Emoji:     s.Emoji,
		}
		if s.URL != "" {
			suggestion.CDNURL = s.URL
		}
		sc.stickerMap.Register(channelType, emotion, suggestion)
		count++
	}

	if sc.saveFile != "" {
		sc.persistToFile()
	}

	return fmt.Sprintf("📦 已学习 %d 个贴图", count)
}
