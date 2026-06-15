// Package reveriepack mounts the Reverie (inner-monologue) HTTP surface
// (/v1/reverie/{journal,stats,config,think,thought,actions,targets}) as a v2
// capability pack (Tier 0 microkernel). Native pack: handler logic lives here
// and talks to the Reverie engine through a narrow accessor; the gateway no
// longer hosts these routes (the admin-only /v1/cognitive-layer and
// /v1/reverie/dream/status stay on the gateway for now).
package reveriepack

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"yunque-agent/internal/agentcore/planner"
	"yunque-agent/internal/apperror"
	"yunque-agent/pkg/packruntime"
)

// PackID is the stable manifest id.
const PackID = "yunque.pack.reverie"

// Gateway is the narrow host surface the reverie pack needs.
type Gateway interface {
	Reverie() *planner.Reverie
	// ReverieChannelTypes lists registered channel types, for the targets view.
	ReverieChannelTypes() []string
}

// Handler is the reverie pack backend module.
type Handler struct {
	gw      Gateway
	host    packruntime.Host
	started atomic.Bool
}

// New builds the reverie pack backed by the host accessors.
func New(gw Gateway) *Handler { return &Handler{gw: gw} }

var _ packruntime.Module = (*Handler)(nil)

func (h *Handler) PackID() string { return PackID }

func (h *Handler) Init(host packruntime.Host) error {
	h.host = host
	return nil
}

func (h *Handler) Start(ctx context.Context) error {
	h.started.Store(true)
	if h.host != nil {
		h.host.Logger().Info("reverie pack started", "pack", PackID)
	}
	return nil
}

func (h *Handler) Stop(ctx context.Context) error {
	h.started.Store(false)
	return nil
}

func (h *Handler) rev() *planner.Reverie {
	if h.gw == nil {
		return nil
	}
	return h.gw.Reverie()
}

// Routes mounts the reverie surface natively.
func (h *Handler) Routes() []packruntime.BackendRoute {
	m := []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete}
	mk := func(path string, fn http.HandlerFunc) packruntime.BackendRoute {
		return packruntime.BackendRoute{Methods: m, Path: path, Handler: fn}
	}
	return []packruntime.BackendRoute{
		mk("/v1/reverie/journal", h.handleJournal),
		mk("/v1/reverie/stats", h.handleStats),
		mk("/v1/reverie/config", h.handleConfig),
		mk("/v1/reverie/think", h.handleThink),
		mk("/v1/reverie/thought", h.handleDeleteThought),
		mk("/v1/reverie/targets", h.handleTargets),
		mk("/v1/reverie/actions", h.handleActions),
	}
}

// handleJournal returns the thought journal with optional filters.
func (h *Handler) handleJournal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET only")
		return
	}
	rev := h.rev()
	if rev == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "reverie not initialized")
		return
	}

	journal := rev.Journal()
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
	for i, j := 0, len(filtered)-1; i < j; i, j = i+1, j-1 {
		filtered[i], filtered[j] = filtered[j], filtered[i]
	}
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
	_ = json.NewEncoder(w).Encode(map[string]any{
		"thoughts": filtered,
		"total":    total,
		"limit":    limit,
		"offset":   offset,
	})
}

// handleStats returns summary statistics.
func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET only")
		return
	}
	rev := h.rev()
	if rev == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "reverie not initialized")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(rev.Stats())
}

// handleConfig returns or updates the Reverie configuration.
func (h *Handler) handleConfig(w http.ResponseWriter, r *http.Request) {
	rev := h.rev()
	if rev == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "reverie not initialized")
		return
	}

	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		cfg := rev.Config()
		_ = json.NewEncoder(w).Encode(map[string]any{
			"config":  cfg,
			"running": rev.Running(),
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

		updated := rev.UpdateConfig(interval, minSig, quietStart, quietEnd, req.Enabled)
		slog.Info("reverie config updated via API",
			"enabled", updated.Enabled,
			"interval", updated.Interval,
			"min_significance", updated.MinSignificance,
			"quiet_start", updated.QuietStart,
			"quiet_end", updated.QuietEnd,
		)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"config":  updated,
			"running": rev.Running(),
		})

	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET or PUT only")
	}
}

// handleThink manually triggers a single think cycle.
func (h *Handler) handleThink(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	rev := h.rev()
	if rev == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "reverie not initialized")
		return
	}

	var req struct {
		EventType string `json:"event_type"`
		Trigger   string `json:"trigger"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	var thought *planner.Thought
	var err error
	if req.EventType != "" {
		ev := planner.ReverieEvent{
			Type:    planner.ReverieEventType(req.EventType),
			Trigger: req.Trigger,
		}
		thought, err = rev.ThinkWithEvent(ctx, ev)
	} else {
		thought, err = rev.Think(ctx)
	}
	if err != nil {
		apperror.WriteCode(w, apperror.CodeInternal, "think failed: "+err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"thought": thought})
}

// handleDeleteThought deletes a thought from the journal.
func (h *Handler) handleDeleteThought(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "DELETE only")
		return
	}
	rev := h.rev()
	if rev == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "reverie not initialized")
		return
	}
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "id parameter required")
		return
	}
	if rev.DeleteThought(id) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"deleted": true, "id": id})
	} else {
		apperror.WriteCode(w, apperror.CodeNotFound, "thought not found: "+id)
	}
}

// handleActions returns the action execution log for P4 active Reverie.
func (h *Handler) handleActions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET only")
		return
	}
	rev := h.rev()
	if rev == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "reverie not initialized")
		return
	}
	log := rev.ActionLog()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"actions": log, "total": len(log)})
}

// handleTargets returns the configured push targets for Reverie delivery.
func (h *Handler) handleTargets(w http.ResponseWriter, r *http.Request) {
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
	scan := func(ch string) {
		envKey := "REVERIE_TARGET_" + strings.ToUpper(ch)
		raw := os.Getenv(envKey)
		if raw == "" {
			return
		}
		for _, existing := range result {
			if existing.Channel == ch {
				return
			}
		}
		parts := strings.Split(raw, ",")
		cleaned := make([]string, 0, len(parts))
		for _, t := range parts {
			if s := strings.TrimSpace(t); s != "" {
				cleaned = append(cleaned, s)
			}
		}
		if len(cleaned) > 0 {
			result = append(result, targetInfo{Channel: ch, Targets: cleaned, EnvVar: envKey})
		}
	}

	for _, ch := range []string{"telegram", "feishu", "discord", "whatsapp", "slack", "signal", "email", "wecom", "dingtalk", "line", "kook", "satori"} {
		scan(ch)
	}
	if h.gw != nil {
		for _, ch := range h.gw.ReverieChannelTypes() {
			scan(ch)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"targets":    result,
		"count":      len(result),
		"env_prefix": "REVERIE_TARGET_",
	})
}
