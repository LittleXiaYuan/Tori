package gateway

import (
	"context"
	"log/slog"
	"math/rand/v2"

	"yunque-agent/internal/execution/channel"
)

// PreAckReact sends a random emoji reaction to a message as acknowledgment.
// This is called before processing, so the user sees immediate feedback.
// Similar to AstrBot's preprocess_stage pre_ack_emoji.
func (g *Gateway) PreAckReact(ctx context.Context, channelType, target, messageID string) {
	if len(g.preAckEmojis) == 0 || g.channelReg == nil {
		return
	}

	ch, ok := g.channelReg.Get(channelType)
	if !ok {
		return
	}

	reactor, ok := ch.(channel.Reactor)
	if !ok {
		return
	}

	emoji := g.preAckEmojis[rand.IntN(len(g.preAckEmojis))]
	if err := reactor.React(ctx, target, messageID, emoji); err != nil {
		slog.Debug("pre-ack react failed", "channel", channelType, "err", err)
	}
}
