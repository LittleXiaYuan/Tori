// Package emotionpack mounts the emotion surface (/v1/emotion/stickers,
// /v1/emotion/history) as a v2 capability pack (Tier 0 microkernel). It is a
// native pack: the sticker-map CRUD and emotion-history query logic live here
// and talk to the emotion subsystems through narrow host accessors — the
// gateway no longer hosts these routes.
package emotionpack

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"yunque-agent/internal/agentcore/emotion"
	"yunque-agent/pkg/packruntime"
)

// PackID is the stable manifest id.
const PackID = "yunque.pack.emotion"

// Gateway is the narrow host surface the emotion pack needs: handles to the
// emotion history store and the sticker suggestion map, resolved per request so
// registration order does not matter.
type Gateway interface {
	EmotionHistory() *emotion.History
	StickerMap() *emotion.StickerMap
}

// Handler is the emotion pack backend module.
type Handler struct {
	gw      Gateway
	host    packruntime.Host
	started atomic.Bool
}

// New builds the emotion pack backed by the host's emotion accessors.
func New(gw Gateway) *Handler { return &Handler{gw: gw} }

// compile-time assertion: this is a valid v2 Module.
var _ packruntime.Module = (*Handler)(nil)

func (h *Handler) PackID() string { return PackID }

// Init wires the pack against the kernel Host.
func (h *Handler) Init(host packruntime.Host) error {
	h.host = host
	return nil
}

// Start marks the pack live on enable.
func (h *Handler) Start(ctx context.Context) error {
	h.started.Store(true)
	if h.host != nil {
		h.host.Logger().Info("emotion pack started", "pack", PackID)
	}
	return nil
}

// Stop marks the pack stopped on disable.
func (h *Handler) Stop(ctx context.Context) error {
	h.started.Store(false)
	return nil
}

// Routes mounts the emotion surface natively.
func (h *Handler) Routes() []packruntime.BackendRoute {
	return []packruntime.BackendRoute{
		{
			Methods: []string{http.MethodGet, http.MethodPut, http.MethodDelete},
			Path:    "/v1/emotion/stickers",
			Handler: h.handleStickers,
		},
		{
			Methods: []string{http.MethodGet},
			Path:    "/v1/emotion/history",
			Handler: h.handleHistory,
		},
	}
}

func (h *Handler) history() *emotion.History {
	if h.gw == nil {
		return nil
	}
	return h.gw.EmotionHistory()
}

func (h *Handler) stickers() *emotion.StickerMap {
	if h.gw == nil {
		return nil
	}
	return h.gw.StickerMap()
}

// handleHistory returns emotion history entries (GET) with optional query params.
func (h *Handler) handleHistory(w http.ResponseWriter, r *http.Request) {
	hist := h.history()
	if hist == nil {
		http.Error(w, `{"error":"emotion history not configured"}`, http.StatusNotFound)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID := r.URL.Query().Get("session_id")
	limitStr := r.URL.Query().Get("limit")
	limit := 100
	if limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil && n > 0 {
			limit = n
		}
	}

	var from, to time.Time
	if v := r.URL.Query().Get("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			from = t
		}
	}
	if v := r.URL.Query().Get("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			to = t
		}
	}

	entries := hist.Query(sessionID, from, to, limit)
	summary := emotion.Summary(entries)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"entries": entries,
		"summary": summary,
		"total":   len(entries),
	})
}

// handleStickers manages sticker mappings: GET lists all, PUT adds/updates, DELETE removes.
func (h *Handler) handleStickers(w http.ResponseWriter, r *http.Request) {
	sm := h.stickers()
	if sm == nil {
		http.Error(w, `{"error":"sticker map not configured"}`, http.StatusNotFound)
		return
	}

	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sm.Export())

	case http.MethodPut:
		var req struct {
			Platform string `json:"platform"`
			Emotion  string `json:"emotion"`
			Stickers []struct {
				PackageID string `json:"package_id"`
				StickerID string `json:"sticker_id"`
			} `json:"stickers"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Platform == "" || req.Emotion == "" {
			http.Error(w, `{"error":"platform, emotion, and stickers required"}`, http.StatusBadRequest)
			return
		}
		for _, s := range req.Stickers {
			sm.Register(req.Platform, emotion.Emotion(req.Emotion), emotion.StickerSuggestion{
				PackageID: s.PackageID,
				StickerID: s.StickerID,
				Platform:  req.Platform,
				Emotion:   emotion.Emotion(req.Emotion),
			})
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

	case http.MethodDelete:
		var req struct {
			Platform string `json:"platform"`
			Emotion  string `json:"emotion"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Platform == "" || req.Emotion == "" {
			http.Error(w, `{"error":"platform and emotion required"}`, http.StatusBadRequest)
			return
		}
		sm.Clear(req.Platform, emotion.Emotion(req.Emotion))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
