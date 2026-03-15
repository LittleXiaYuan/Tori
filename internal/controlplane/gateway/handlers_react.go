package gateway

import (
	"context"
	"encoding/json"
	"log/slog"
	"math/rand/v2"
	"net/http"

	"yunque-agent/internal/execution/channel"
)

// handleReact handles POST /v1/react to add emoji reactions to messages.
func (g *Gateway) handleReact(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ChannelType string `json:"channel_type"` // "telegram", "discord", etc.
		Target      string `json:"target"`       // chat ID
		MessageID   string `json:"message_id"`   // message to react to
		Emoji       string `json:"emoji"`        // unicode emoji or custom emoji ID; empty to clear
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ChannelType == "" || req.Target == "" || req.MessageID == "" {
		http.Error(w, `{"error":"channel_type, target, and message_id required"}`, http.StatusBadRequest)
		return
	}

	if g.channelReg == nil {
		http.Error(w, `{"error":"channel registry not configured"}`, http.StatusServiceUnavailable)
		return
	}

	ch, ok := g.channelReg.Get(req.ChannelType)
	if !ok {
		http.Error(w, `{"error":"channel not found"}`, http.StatusNotFound)
		return
	}

	reactor, ok := ch.(channel.Reactor)
	if !ok {
		http.Error(w, `{"error":"channel does not support reactions"}`, http.StatusBadRequest)
		return
	}

	if err := reactor.React(r.Context(), req.Target, req.MessageID, req.Emoji); err != nil {
		slog.Error("react failed", "channel", req.ChannelType, "err", err)
		http.Error(w, `{"error":"reaction failed: `+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleSendSticker handles POST /v1/sticker/send to send stickers via channels.
func (g *Gateway) handleSendSticker(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ChannelType string `json:"channel_type"`
		Target      string `json:"target"`
		PackageID   string `json:"package_id"`
		StickerID   string `json:"sticker_id"`
		FileID      string `json:"file_id,omitempty"`
		Emoji       string `json:"emoji,omitempty"`
		Platform    string `json:"platform,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ChannelType == "" || req.Target == "" {
		http.Error(w, `{"error":"channel_type and target required"}`, http.StatusBadRequest)
		return
	}

	if g.channelReg == nil {
		http.Error(w, `{"error":"channel registry not configured"}`, http.StatusServiceUnavailable)
		return
	}

	ch, ok := g.channelReg.Get(req.ChannelType)
	if !ok {
		http.Error(w, `{"error":"channel not found"}`, http.StatusNotFound)
		return
	}

	sender, ok := ch.(channel.StickerSender)
	if !ok {
		http.Error(w, `{"error":"channel does not support sticker sending"}`, http.StatusBadRequest)
		return
	}

	sticker := channel.NewSticker(req.PackageID, req.StickerID)
	sticker.FileID = req.FileID
	sticker.Emoji = req.Emoji
	sticker.Platform = req.Platform
	if sticker.Platform == "" {
		sticker.Platform = req.ChannelType
	}

	if err := sender.SendSticker(r.Context(), req.Target, sticker); err != nil {
		slog.Error("sendSticker failed", "channel", req.ChannelType, "err", err)
		http.Error(w, `{"error":"sticker send failed: `+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

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
