package gateway

import (
	"encoding/json"
	"net/http"

	"yunque-agent/internal/agentcore/emotion"
)

// handleStickers manages sticker mappings: GET lists all, PUT adds/updates, DELETE removes.
func (g *Gateway) handleStickers(w http.ResponseWriter, r *http.Request) {
	if g.stickerMap == nil {
		http.Error(w, `{"error":"sticker map not configured"}`, http.StatusNotFound)
		return
	}

	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(g.stickerMap.Export())

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
			g.stickerMap.Register(req.Platform, emotion.Emotion(req.Emotion), emotion.StickerSuggestion{
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
		g.stickerMap.Clear(req.Platform, emotion.Emotion(req.Emotion))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
