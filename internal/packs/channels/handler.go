package channelspack

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync/atomic"

	"yunque-agent/internal/apperror"
	"yunque-agent/internal/execution/channel"
	"yunque-agent/pkg/packruntime"
)

const PackID = "yunque.pack.channels"

type Gateway interface {
	ChannelRegistry() *channel.Registry
}

// Handler exposes user-facing channel actions as a native capability pack:
// reactions, sticker sends and group discovery.
type Handler struct {
	registryOf func() *channel.Registry
	host       packruntime.Host
	started    atomic.Bool
}

func New(gateway Gateway) *Handler {
	if gateway == nil {
		return NewProvider(nil)
	}
	return NewProvider(gateway.ChannelRegistry)
}

func NewProvider(registry func() *channel.Registry) *Handler {
	return &Handler{registryOf: registry}
}

func (h *Handler) PackID() string { return PackID }

var _ packruntime.Module = (*Handler)(nil)

func (h *Handler) Init(host packruntime.Host) error {
	h.host = host
	return nil
}

func (h *Handler) Start(ctx context.Context) error {
	h.started.Store(true)
	if h.host != nil {
		h.host.Logger().Info("channels pack started", "pack", PackID)
	}
	return nil
}

func (h *Handler) Stop(ctx context.Context) error {
	h.started.Store(false)
	return nil
}

func (h *Handler) Routes() []packruntime.BackendRoute {
	return []packruntime.BackendRoute{
		{Method: http.MethodPost, Path: "/v1/react", Handler: h.React},
		{Method: http.MethodPost, Path: "/v1/sticker/send", Handler: h.SendSticker},
		{Method: http.MethodGet, Path: "/v1/channels/groups", Handler: h.Groups},
	}
}

func RouteSpecs() []packruntime.BackendRouteSpec {
	return []packruntime.BackendRouteSpec{
		{Method: http.MethodPost, Path: "/v1/react", Description: "Add or clear an emoji reaction on a channel message."},
		{Method: http.MethodPost, Path: "/v1/sticker/send", Description: "Send a native sticker through a supported channel."},
		{Method: http.MethodGet, Path: "/v1/channels/groups", Description: "List groups/guilds/rooms visible to configured channels."},
	}
}

func Paths() []string {
	return []string{"/v1/react", "/v1/sticker/send", "/v1/channels/groups"}
}

func (h *Handler) registry() *channel.Registry {
	if h.registryOf == nil {
		return nil
	}
	return h.registryOf()
}

func (h *Handler) React(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	reg := h.registry()
	if reg == nil {
		http.Error(w, `{"error":"channel registry not configured"}`, http.StatusServiceUnavailable)
		return
	}

	var req struct {
		ChannelType string `json:"channel_type"`
		Target      string `json:"target"`
		MessageID   string `json:"message_id"`
		Emoji       string `json:"emoji"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ChannelType == "" || req.Target == "" || req.MessageID == "" {
		http.Error(w, `{"error":"channel_type, target, and message_id required"}`, http.StatusBadRequest)
		return
	}

	ch, ok := reg.Get(req.ChannelType)
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
		apperror.Write(w, apperror.Wrap(apperror.CodeInternal, "reaction failed", err))
		return
	}
	writeJSON(w, map[string]string{"status": "ok"})
}

func (h *Handler) SendSticker(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	reg := h.registry()
	if reg == nil {
		http.Error(w, `{"error":"channel registry not configured"}`, http.StatusServiceUnavailable)
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

	ch, ok := reg.Get(req.ChannelType)
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
		apperror.Write(w, apperror.Wrap(apperror.CodeInternal, "sticker send failed", err))
		return
	}
	writeJSON(w, map[string]string{"status": "ok"})
}

func (h *Handler) Groups(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	reg := h.registry()
	if reg == nil {
		http.Error(w, `{"error":"channel registry not configured"}`, http.StatusServiceUnavailable)
		return
	}

	groups, err := reg.ListGroups(r.Context(), r.URL.Query().Get("type"))
	if err != nil {
		apperror.Write(w, apperror.Wrap(apperror.CodeInternal, "list groups failed", err))
		return
	}
	if groups == nil {
		groups = make([]channel.GroupInfo, 0)
	}
	writeJSON(w, map[string]any{"groups": groups, "count": len(groups)})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
