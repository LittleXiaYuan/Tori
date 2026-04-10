package gateway

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"yunque-agent/internal/agentcore/emotion"
	"yunque-agent/internal/agentcore/modes"
	"yunque-agent/internal/agentcore/planner"
	"yunque-agent/internal/apperror"
)

//  from handlers_emotion.go 
// handleEmotionHistory returns emotion history entries (GET) with optional query params.
func (g *Gateway) handleEmotionHistory(w http.ResponseWriter, r *http.Request) {
	if g.emotionHistory == nil {
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

	entries := g.emotionHistory.Query(sessionID, from, to, limit)
	summary := emotion.Summary(entries)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"entries": entries,
		"summary": summary,
		"total":   len(entries),
	})
}

//  from handlers_stickers.go 
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

//  from handlers_modes.go 
// handleListModes returns all available persona modes (GET).
func (g *Gateway) handleListModes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if g.modeManager == nil {
		http.Error(w, `{"error":"mode system not configured"}`, http.StatusNotFound)
		return
	}

	tenantID := r.URL.Query().Get("tenant_id")
	sessionID := r.URL.Query().Get("session_id")

	modeList := g.modeManager.ListModes(r.Context(), tenantID, sessionID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"modes": modeList,
		"total": len(modeList),
	})
}

// handleSetMode switches the persona mode for a tenant (POST).
func (g *Gateway) handleSetMode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if g.modeManager == nil {
		http.Error(w, `{"error":"mode system not configured"}`, http.StatusNotFound)
		return
	}

	var req struct {
		TenantID  string          `json:"tenant_id"`
		Mode      modes.PersonaMode `json:"mode"`
		SessionID string          `json:"session_id,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if !req.Mode.Valid() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{
			"error":       "invalid mode",
			"valid_modes": modes.AllModes,
		})
		return
	}

	if err := g.modeManager.SetMode(r.Context(), req.TenantID, req.Mode, req.SessionID); err != nil {
		apperror.Write(w, apperror.Wrap(apperror.CodeInternal, "set mode failed", err))
		return
	}

	// Return the updated mode list with the new active mode
	modeList := g.modeManager.ListModes(r.Context(), req.TenantID, req.SessionID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"success":      true,
		"current_mode": req.Mode,
		"modes":        modeList,
	})
}

// handleCurrentMode returns the current active mode for a tenant (GET).
func (g *Gateway) handleCurrentMode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if g.modeManager == nil {
		http.Error(w, `{"error":"mode system not configured"}`, http.StatusNotFound)
		return
	}

	tenantID := r.URL.Query().Get("tenant_id")
	sessionID := r.URL.Query().Get("session_id")

	current := g.modeManager.CurrentMode(r.Context(), tenantID, sessionID)
	preset := modes.ModePresets[current]

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"mode":        current,
		"name":        preset.Name,
		"name_en":     preset.NameEN,
		"description": preset.Description,
		"features":    preset.Features,
	})
}

//  from handlers_reverie.go 
// 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€
// Reverie API 鈥?visualization and operations endpoints
//
// GET  /v1/reverie/journal   鈥?list thoughts (filter, paginate)
// GET  /v1/reverie/stats     鈥?summary statistics
// GET  /v1/reverie/config    鈥?current configuration
// PUT  /v1/reverie/config    鈥?update configuration
// POST /v1/reverie/think     鈥?manually trigger a think cycle
// DELETE /v1/reverie/thought 鈥?delete a specific thought by ID
// 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€

// handleReverieJournal returns the thought journal with optional filters.
// Query params: category, min_significance, limit, offset, delivered (true/false)
func (g *Gateway) handleReverieJournal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET only")
		return
	}
	if g.reverie == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "reverie not initialized")
		return
	}

	journal := g.reverie.Journal()

	// Filters
	category := r.URL.Query().Get("category")
	minSigStr := r.URL.Query().Get("min_significance")
	deliveredStr := r.URL.Query().Get("delivered")

	var minSig float64
	if minSigStr != "" {
		if v, err := strconv.ParseFloat(minSigStr, 64); err == nil {
			minSig = v
		}
	}

	filtered := make([]planner.Thought, 0, len(journal))
	for _, t := range journal {
		if category != "" && t.Category != category {
			continue
		}
		if minSig > 0 && t.Significance < minSig {
			continue
		}
		if deliveredStr == "true" && !t.Delivered {
			continue
		}
		if deliveredStr == "false" && t.Delivered {
			continue
		}
		filtered = append(filtered, t)
	}

	// Pagination
	limit := 50
	offset := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	total := len(filtered)

	// Reverse chronological order (newest first)
	for i, j := 0, len(filtered)-1; i < j; i, j = i+1, j-1 {
		filtered[i], filtered[j] = filtered[j], filtered[i]
	}

	// Apply pagination
	if offset >= len(filtered) {
		filtered = nil
	} else {
		end := offset + limit
		if end > len(filtered) {
			end = len(filtered)
		}
		filtered = filtered[offset:end]
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"thoughts": filtered,
		"total":    total,
		"limit":    limit,
		"offset":   offset,
	})
}

// handleReverieStats returns summary statistics.
func (g *Gateway) handleReverieStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET only")
		return
	}
	if g.reverie == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "reverie not initialized")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(g.reverie.Stats())
}

// handleReverieConfig returns or updates the Reverie configuration.
func (g *Gateway) handleReverieConfig(w http.ResponseWriter, r *http.Request) {
	if g.reverie == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "reverie not initialized")
		return
	}

	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		cfg := g.reverie.Config()
		json.NewEncoder(w).Encode(map[string]any{
			"config":  cfg,
			"running": g.reverie.Running(),
		})

	case http.MethodPut:
		var req struct {
			Enabled         *bool    `json:"enabled"`
			IntervalMinutes *float64 `json:"interval_minutes"`
			MinSignificance *float64 `json:"min_significance"`
			QuietStart      *int     `json:"quiet_start"`
			QuietEnd        *int     `json:"quiet_end"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON: "+err.Error())
			return
		}

		var interval time.Duration
		if req.IntervalMinutes != nil && *req.IntervalMinutes > 0 {
			interval = time.Duration(*req.IntervalMinutes * float64(time.Minute))
		}
		var minSig float64
		if req.MinSignificance != nil {
			minSig = *req.MinSignificance
		}
		quietStart := -1
		if req.QuietStart != nil {
			quietStart = *req.QuietStart
		}
		quietEnd := -1
		if req.QuietEnd != nil {
			quietEnd = *req.QuietEnd
		}

		updated := g.reverie.UpdateConfig(interval, minSig, quietStart, quietEnd, req.Enabled)

		slog.Info("reverie config updated via API",
			"enabled", updated.Enabled,
			"interval", updated.Interval,
			"min_significance", updated.MinSignificance,
			"quiet_start", updated.QuietStart,
			"quiet_end", updated.QuietEnd,
		)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"config":  updated,
			"running": g.reverie.Running(),
		})

	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET or PUT only")
	}
}

// handleReverieThink manually triggers a single think cycle.
func (g *Gateway) handleReverieThink(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	if g.reverie == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "reverie not initialized")
		return
	}

	// Optional: accept an event type to simulate event-driven thinking
	var req struct {
		EventType string `json:"event_type"`
		Trigger   string `json:"trigger"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req) // best-effort, empty body is fine

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	var thought *planner.Thought
	var err error

	if req.EventType != "" {
		ev := planner.ReverieEvent{
			Type:    planner.ReverieEventType(req.EventType),
			Trigger: req.Trigger,
		}
		thought, err = g.reverie.ThinkWithEvent(ctx, ev)
	} else {
		thought, err = g.reverie.Think(ctx)
	}

	if err != nil {
		apperror.WriteCode(w, apperror.CodeInternal, "think failed: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"thought": thought})
}

// handleReverieDeleteThought deletes a thought from the journal.
// Query param: id (required)
func (g *Gateway) handleReverieDeleteThought(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "DELETE only")
		return
	}
	if g.reverie == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "reverie not initialized")
		return
	}

	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "id parameter required")
		return
	}

	if g.reverie.DeleteThought(id) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"deleted": true, "id": id})
	} else {
		apperror.WriteCode(w, apperror.CodeNotFound, "thought not found: "+id)
	}
}

// handleReverieActions returns the action execution log for P4 active Reverie.
func (g *Gateway) handleReverieActions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET only")
		return
	}
	if g.reverie == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "reverie not initialized")
		return
	}

	log := g.reverie.ActionLog()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"actions": log, "total": len(log)})
}

// handleReverieTargets returns the configured push targets for Reverie delivery.
func (g *Gateway) handleReverieTargets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET only")
		return
	}

	type targetInfo struct {
		Channel string   `json:"channel"`
		Targets []string `json:"targets"`
		EnvVar  string   `json:"env_var"`
	}

	var result []targetInfo

	// Scan known channel types for configured targets
	knownChannels := []string{"telegram", "feishu", "discord", "whatsapp", "slack", "signal", "email", "wecom", "dingtalk", "line", "kook", "satori"}
	for _, ch := range knownChannels {
		envKey := "REVERIE_TARGET_" + strings.ToUpper(ch)
		raw := os.Getenv(envKey)
		if raw == "" {
			continue
		}
		targets := strings.Split(raw, ",")
		cleaned := make([]string, 0, len(targets))
		for _, t := range targets {
			if s := strings.TrimSpace(t); s != "" {
				cleaned = append(cleaned, s)
			}
		}
		if len(cleaned) > 0 {
			result = append(result, targetInfo{Channel: ch, Targets: cleaned, EnvVar: envKey})
		}
	}

	// Also check if any registered channels have targets
	if g.channelReg != nil {
		for _, ch := range g.channelReg.All() {
			envKey := "REVERIE_TARGET_" + strings.ToUpper(ch.Type())
			raw := os.Getenv(envKey)
			if raw == "" {
				continue
			}
			// Avoid duplicates
			found := false
			for _, existing := range result {
				if existing.Channel == ch.Type() {
					found = true
					break
				}
			}
			if !found {
				targets := strings.Split(raw, ",")
				cleaned := make([]string, 0, len(targets))
				for _, t := range targets {
					if s := strings.TrimSpace(t); s != "" {
						cleaned = append(cleaned, s)
					}
				}
				if len(cleaned) > 0 {
					result = append(result, targetInfo{Channel: ch.Type(), Targets: cleaned, EnvVar: envKey})
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"targets":    result,
		"count":      len(result),
		"env_prefix": "REVERIE_TARGET_",
	})
}

