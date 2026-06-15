// Package worldmodelpack exposes read-only views into the Agent's world model
// (tracked external state) and causal-reasoning engine (failure root causes,
// failure patterns, annotated timelines).
package worldmodelpack

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"sync/atomic"
	"time"

	"yunque-agent/internal/cognicore/causal"
	"yunque-agent/internal/cognicore/world"
	"yunque-agent/pkg/packruntime"
)

const PackID = "yunque.pack.world-model"

// Config wires runtime dependencies.
type Config struct {
	WorldModel *world.Model
	Causal     *causal.CausalEngine
	TenantID   string
}

// Handler serves the World Model pack endpoints.
type Handler struct {
	worldModel *world.Model
	causal     *causal.CausalEngine
	tenantID   string
	host       packruntime.Host
	started    atomic.Bool
}

// New creates a World Model pack handler.
func New(cfg Config) *Handler {
	tenantID := cfg.TenantID
	if tenantID == "" {
		tenantID = "default"
	}
	return &Handler{worldModel: cfg.WorldModel, causal: cfg.Causal, tenantID: tenantID}
}

// PackID returns the manifest id.
func (h *Handler) PackID() string { return PackID }

// compile-time assertion: World Model is a v2 capability Module (Tier 0 microkernel).
var _ packruntime.Module = (*Handler)(nil)

// Init wires the pack against the kernel Host. Dependencies arrive via Config.
func (h *Handler) Init(host packruntime.Host) error {
	h.host = host
	return nil
}

// Start marks the pack live on enable (no background workers — read-only views).
func (h *Handler) Start(ctx context.Context) error {
	h.started.Store(true)
	if h.host != nil {
		h.host.Logger().Info("world-model pack started", "pack", PackID)
	}
	return nil
}

// Stop marks the pack stopped on disable.
func (h *Handler) Stop(ctx context.Context) error {
	h.started.Store(false)
	return nil
}

// Routes exposes the read-only world-model endpoints.
func (h *Handler) Routes() []packruntime.BackendRoute {
	return []packruntime.BackendRoute{
		{Method: http.MethodGet, Path: "/v1/world/state", Handler: h.State},
		{Method: http.MethodGet, Path: "/v1/world/state/stale", Handler: h.Stale},
		{Method: http.MethodGet, Path: "/v1/world/causal/timeline", Handler: h.Timeline},
		{Method: http.MethodGet, Path: "/v1/world/causal/root-cause", Handler: h.RootCause},
		{Method: http.MethodGet, Path: "/v1/world/causal/failure-patterns", Handler: h.FailurePatterns},
	}
}

// StateResponse lists tracked world-state entries.
type StateResponse struct {
	Entries []world.State `json:"entries"`
	Total   int           `json:"total"`
}

// StaleResponse lists state entries past a freshness threshold.
type StaleResponse struct {
	Keys   []string      `json:"keys"`
	MaxAge time.Duration `json:"max_age"`
}

// TimelineResponse is the annotated event timeline for a task.
type TimelineResponse struct {
	TaskID  string                 `json:"task_id"`
	Entries []causal.TimelineEntry `json:"entries"`
}

// RootCauseResponse contains the root-cause chain for a failed task.
type RootCauseResponse struct {
	TaskID string              `json:"task_id"`
	Chain  *causal.CausalChain `json:"chain,omitempty"`
}

// FailurePatternsResponse aggregates recurring failure patterns.
type FailurePatternsResponse struct {
	Patterns []causal.FailurePattern `json:"patterns"`
}

// State returns a snapshot of the world model.
func (h *Handler) State(w http.ResponseWriter, r *http.Request) {
	out := []world.State{}
	if h.worldModel != nil {
		snap := h.worldModel.Snapshot()
		kindFilter := r.URL.Query().Get("kind")
		out = make([]world.State, 0, len(snap))
		for _, s := range snap {
			if kindFilter != "" && string(s.Kind) != kindFilter {
				continue
			}
			out = append(out, *s)
		}
		sort.Slice(out, func(i, j int) bool {
			return out[i].LastVerified.After(out[j].LastVerified)
		})
	}
	writeJSON(w, StateResponse{Entries: out, Total: len(out)})
}

// Stale returns world-state keys not verified within ?max_age (default 24h).
func (h *Handler) Stale(w http.ResponseWriter, r *http.Request) {
	maxAge := 24 * time.Hour
	if v := r.URL.Query().Get("max_age"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			maxAge = d
		}
	}
	keys := []string{}
	if h.worldModel != nil {
		keys = h.worldModel.StaleKeys(maxAge)
		if keys == nil {
			keys = []string{}
		}
		sort.Strings(keys)
	}
	writeJSON(w, StaleResponse{Keys: keys, MaxAge: maxAge})
}

// Timeline returns the annotated event timeline for a task.
func (h *Handler) Timeline(w http.ResponseWriter, r *http.Request) {
	taskID := r.URL.Query().Get("task_id")
	resp := TimelineResponse{TaskID: taskID, Entries: []causal.TimelineEntry{}}
	if taskID == "" || h.causal == nil {
		writeJSON(w, resp)
		return
	}
	entries, err := h.causal.BuildTimeline(r.Context(), taskID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if entries != nil {
		resp.Entries = entries
	}
	writeJSON(w, resp)
}

// RootCause returns the root-cause chain for a failed task.
func (h *Handler) RootCause(w http.ResponseWriter, r *http.Request) {
	taskID := r.URL.Query().Get("task_id")
	resp := RootCauseResponse{TaskID: taskID}
	if taskID == "" || h.causal == nil {
		writeJSON(w, resp)
		return
	}
	chain, err := h.causal.FindRootCause(r.Context(), taskID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	resp.Chain = chain
	writeJSON(w, resp)
}

// FailurePatterns returns recurring failure patterns across tenant tasks.
func (h *Handler) FailurePatterns(w http.ResponseWriter, r *http.Request) {
	limit := parseLimit(r, 50)
	out := []causal.FailurePattern{}
	if h.causal != nil {
		patterns, err := h.causal.AnalyzeFailurePatterns(r.Context(), h.tenantID, limit)
		if err == nil && patterns != nil {
			out = patterns
			sort.Slice(out, func(i, j int) bool {
				return out[i].Occurrences > out[j].Occurrences
			})
		}
	}
	writeJSON(w, FailurePatternsResponse{Patterns: out})
}

func parseLimit(r *http.Request, def int) int {
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			return n
		}
	}
	return def
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
