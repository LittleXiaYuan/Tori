package channel

import (
	"math/rand/v2"
)

// StickerEnricher is a middleware that appends a sticker to outgoing replies
// based on the detected emotion of the incoming message. This enables
// automatic sticker sending across all IM channels when emotion confidence
// exceeds the threshold.
type StickerEnricher struct {
	// AnalyzeEmotion returns the detected emotion and confidence for a message.
	// Returns ("", 0) if analysis is unavailable or fails.
	AnalyzeEmotion func(text string) (emotion string, confidence float64)

	// SuggestSticker returns a sticker for the given emotion and platform.
	// Returns nil if no matching sticker exists.
	SuggestSticker func(emotion, platform string) *StickerComponent

	// ShouldSend returns whether a sticker should actually be sent,
	// based on persona feature flags and frequency settings.
	// Called with the emotion string; returns probability 0-1.
	SendProbability func() float64

	// MinConfidence is the minimum emotion confidence to trigger sticker sending.
	MinConfidence float64
}

// Wrap returns a handler middleware that enriches replies with sticker suggestions.
func (se *StickerEnricher) Wrap(next func(Message) Reply) func(Message) Reply {
	if se.AnalyzeEmotion == nil || se.SuggestSticker == nil {
		return next
	}

	return func(msg Message) Reply {
		reply := next(msg)

		if msg.Content == "" {
			return reply
		}

		emo, conf := se.AnalyzeEmotion(msg.Content)
		if emo == "" || emo == "neutral" || emo == "unknown" {
			return reply
		}
		if conf < se.MinConfidence {
			return reply
		}

		prob := 0.5
		if se.SendProbability != nil {
			prob = se.SendProbability()
		}
		if rand.Float64() >= prob {
			return reply
		}

		sticker := se.SuggestSticker(emo, msg.ChannelType)
		if sticker == nil {
			return reply
		}

		if reply.Rich == nil {
			reply.Rich = NewRichMessage()
		}
		reply.Rich.Add(sticker)
		return reply
	}
}
