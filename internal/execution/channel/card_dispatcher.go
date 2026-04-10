package channel

import (
	"context"
	"fmt"
	"log/slog"
)

// ──────────────────────────────────────────────
// Channel Card Dispatcher
//
// Provides a unified way to dispatch interactive cards
// or rich notifications to any registered channel.
// Each channel gets the best format it supports:
//   feishu    → interactive card JSON
//   dingtalk  → sampleMarkdown (no native card via bot API)
//   wecom     → markdown message
//   telegram  → Markdown text
//   kook      → kmarkdown
//   others    → plain text fallback
// ──────────────────────────────────────────────

// CardDispatcher routes structured card content to channels,
// adapting format to each channel's capabilities.
type CardDispatcher struct {
	channels map[string]Channel // channelType → Channel
}

// NewCardDispatcher creates a dispatcher from registered channels.
func NewCardDispatcher(channels map[string]Channel) *CardDispatcher {
	return &CardDispatcher{channels: channels}
}

// SendCard dispatches a card to a specific channel target.
// cardJSON is the Feishu-format interactive card JSON.
// markdown is a Markdown fallback for channels without card support.
// plainText is the final fallback for text-only channels.
func (d *CardDispatcher) SendCard(ctx context.Context, channelType, target, cardJSON, markdown, plainText string) error {
	ch, ok := d.channels[channelType]
	if !ok {
		return fmt.Errorf("channel %q not registered", channelType)
	}

	switch channelType {
	case "feishu":
		// Native interactive card
		return ch.Send(ctx, target, Reply{Content: cardJSON, Format: "card"})

	case "dingtalk":
		// Markdown via sampleMarkdown message type
		return ch.Send(ctx, target, Reply{Content: markdown, Format: "markdown"})

	case "wecom":
		// WeChat Work supports markdown
		return ch.Send(ctx, target, Reply{Content: markdown, Format: "markdown"})

	case "telegram":
		// Telegram: Markdown text (use SendRichCard for InlineKeyboard buttons)
		return ch.Send(ctx, target, Reply{Content: markdown, Format: "markdown"})

	case "kook":
		// Kook kmarkdown
		return ch.Send(ctx, target, Reply{Content: markdown, Format: "markdown"})

	case "slack":
		// Slack supports Block Kit but we use text for now
		return ch.Send(ctx, target, Reply{Content: markdown})

	case "discord":
		// Discord supports embeds but we use text for now
		return ch.Send(ctx, target, Reply{Content: markdown})

	default:
		// All others: plain text
		return ch.Send(ctx, target, Reply{Content: plainText})
	}
}

// SendCardToAll dispatches a card to all registered channels
// (useful for broadcast notifications).
func (d *CardDispatcher) SendCardToAll(ctx context.Context, targets map[string]string, cardJSON, markdown, plainText string) {
	for chType, target := range targets {
		if err := d.SendCard(ctx, chType, target, cardJSON, markdown, plainText); err != nil {
			slog.Warn("card dispatch failed",
				"channel", chType,
				"target", target,
				"error", err)
		}
	}
}

// SendRichCard dispatches a Reply directly to a channel target.
// Use this for channels like Telegram where you need InlineKeyboard buttons.
// The Reply should contain Content + Rich (with ButtonComponents).
func (d *CardDispatcher) SendRichCard(ctx context.Context, channelType, target string, reply Reply) error {
	ch, ok := d.channels[channelType]
	if !ok {
		return fmt.Errorf("channel %q not registered", channelType)
	}
	return ch.Send(ctx, target, reply)
}
