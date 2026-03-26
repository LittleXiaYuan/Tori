package channel

import (
	"context"
	"fmt"
)

// StickerSetFetcher is an optional interface for channels that can fetch
// all stickers in a sticker set/pack (e.g., Telegram getStickerSet).
type StickerSetFetcher interface {
	FetchStickerSet(setName string) ([]StickerComponent, error)
	GetStickerSet(ctx context.Context, setName string) ([]StickerComponent, error)
}

// FormatUserMessageYAML formats a user message with channel metadata as YAML
// for structured injection into prompts.
func FormatUserMessageYAML(userName, channelType, chatType, channelID, content string) string {
	return fmt.Sprintf("user: %s\nchannel: %s\nchat_type: %s\nchannel_id: %s\nmessage: |\n  %s\n", userName, channelType, chatType, channelID, content)
}
