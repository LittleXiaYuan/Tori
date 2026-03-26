package channel

import (
	"fmt"
	"strings"
)

// StickerCommands provides a CommandHandler for sticker-related slash commands.
// It delegates to callback functions so it can be wired to the emotion/sticker
// subsystem without importing it (avoiding circular dependency).
type StickerCommands struct {
	// StartCollect begins a single-sticker collection session.
	// Returns the prompt message to send.
	StartCollect func(channelType, userID, emotion string) string

	// StartBulkAdd begins a bulk sticker-add session.
	StartBulkAdd func(channelType, userID string) string

	// ListStickers returns a formatted list of stickers for a platform.
	ListStickers func(platform string) string

	// DeleteStickers clears stickers for a platform+emotion combo.
	// Returns a confirmation message.
	DeleteStickers func(platform, emotion string) string

	// CancelSession cancels an active collection session.
	CancelSession func(channelType, userID string) bool

	// FetchAndLearnSet fetches a sticker set by name and learns all stickers.
	// Returns (message, error).
	FetchAndLearnSet func(channelType, setName string) (string, error)
}

// Handler returns a CommandHandler for sticker commands.
// Recognized commands: /add, /add-all, /sticker, /sticker-del, /cancel
func (sc *StickerCommands) Handler() CommandHandler {
	return func(msg Message, command, args string) (Reply, bool) {
		switch command {
		case "/add":
			return sc.handleAdd(msg, args), true

		case "/add-all":
			return sc.handleAddAll(msg, args), true

		case "/sticker":
			return sc.handleSticker(msg, args), true

		case "/sticker-del":
			return sc.handleStickerDel(msg, args), true

		case "/cancel":
			return sc.handleCancel(msg), true
		}
		return Reply{}, false
	}
}

func (sc *StickerCommands) handleAdd(msg Message, args string) Reply {
	if sc.StartCollect == nil {
		return Reply{Content: "贴图收集功能未启用", Format: "text"}
	}

	emotion := "happy"
	if args != "" {
		emotion = args
	}

	text := sc.StartCollect(msg.ChannelType, msg.UserID, emotion)
	return Reply{Content: text, Format: "text"}
}

func (sc *StickerCommands) handleAddAll(msg Message, args string) Reply {
	// /add-all <set_name> — fetch and learn an entire sticker set
	if args != "" && sc.FetchAndLearnSet != nil {
		result, err := sc.FetchAndLearnSet(msg.ChannelType, args)
		if err != nil {
			return Reply{Content: fmt.Sprintf("获取贴纸包失败: %s", err), Format: "text"}
		}
		return Reply{Content: result, Format: "text"}
	}

	// No set name → start bulk add session
	if sc.StartBulkAdd == nil {
		return Reply{Content: "批量贴图收集功能未启用", Format: "text"}
	}

	text := sc.StartBulkAdd(msg.ChannelType, msg.UserID)
	return Reply{Content: text, Format: "text"}
}

func (sc *StickerCommands) handleSticker(msg Message, args string) Reply {
	if sc.ListStickers == nil {
		return Reply{Content: "贴图库功能未启用", Format: "text"}
	}

	platform := msg.ChannelType
	if args != "" {
		platform = strings.ToLower(args)
	}
	text := sc.ListStickers(platform)
	return Reply{Content: text, Format: "markdown"}
}

func (sc *StickerCommands) handleStickerDel(msg Message, args string) Reply {
	if sc.DeleteStickers == nil {
		return Reply{Content: "贴图删除功能未启用", Format: "text"}
	}

	parts := strings.Fields(args)
	if len(parts) < 1 {
		return Reply{
			Content: "用法: /sticker-del <情绪>\n" +
				"示例: /sticker-del happy\n" +
				"情绪: happy, sad, angry, surprised, fearful, disgusted, neutral",
			Format: "text",
		}
	}

	emotion := parts[0]
	text := sc.DeleteStickers(msg.ChannelType, emotion)
	return Reply{Content: text, Format: "text"}
}

func (sc *StickerCommands) handleCancel(msg Message) Reply {
	if sc.CancelSession != nil && sc.CancelSession(msg.ChannelType, msg.UserID) {
		return Reply{Content: "✅ 贴图收集已取消", Format: "text"}
	}
	return Reply{Content: "当前没有活跃的收集会话", Format: "text"}
}
