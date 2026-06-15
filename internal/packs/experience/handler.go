// Package experiencepack exposes read-only views into the Agent's accumulated
// experience: skill / response recommendations, the learned user preference
// vector, and task self-evaluation results.
package experiencepack

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"sync/atomic"
	"time"

	"yunque-agent/internal/cognicore/eval"
	"yunque-agent/internal/cognicore/recommend"
	"yunque-agent/internal/ledgercore"
	"yunque-agent/pkg/packruntime"
)

const PackID = "yunque.pack.experience"

// Config wires runtime dependencies.
type Config struct {
	Ledger    *ledger.Ledger
	Recommend *recommend.Engine
	Evaluator *eval.Evaluator
	TenantID  string
}

// Handler serves the Experience pack endpoints.
type Handler struct {
	ledger    *ledger.Ledger
	recEngine *recommend.Engine
	evaluator *eval.Evaluator
	tenantID  string
	host      packruntime.Host
	started   atomic.Bool
}

// New creates an Experience pack handler.
func New(cfg Config) *Handler {
	tenantID := cfg.TenantID
	if tenantID == "" {
		tenantID = "default"
	}
	return &Handler{
		ledger:    cfg.Ledger,
		recEngine: cfg.Recommend,
		evaluator: cfg.Evaluator,
		tenantID:  tenantID,
	}
}

// PackID returns the manifest id.
func (h *Handler) PackID() string { return PackID }

// compile-time assertion: Experience is a v2 capability Module (Tier 0 microkernel).
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
		h.host.Logger().Info("experience pack started", "pack", PackID)
	}
	return nil
}

// Stop marks the pack stopped on disable.
func (h *Handler) Stop(ctx context.Context) error {
	h.started.Store(false)
	return nil
}

// Routes exposes the read-only experience endpoints.
func (h *Handler) Routes() []packruntime.BackendRoute {
	return []packruntime.BackendRoute{
		{Method: http.MethodGet, Path: "/v1/experience/recommendations", Handler: h.Recommendations},
		{Method: http.MethodGet, Path: "/v1/experience/items", Handler: h.Items},
		{Method: http.MethodGet, Path: "/v1/experience/preferences", Handler: h.Preferences},
		{Method: http.MethodGet, Path: "/v1/experience/evaluations", Handler: h.Evaluations},
	}
}

// RecommendationsResponse is the payload for top-K recommendations.
type RecommendationsResponse struct {
	Recommendations []recommend.Recommendation `json:"recommendations"`
	Context         string                     `json:"context,omitempty"`
}

// ItemEntry mirrors recommend.ItemProfile minus the feature vector (large blob).
type ItemEntry struct {
	ID        string    `json:"id"`
	Category  string    `json:"category"`
	Tags      []string  `json:"tags"`
	Uses      int64     `json:"uses"`
	Successes int64     `json:"successes"`
	Failures  int64     `json:"failures"`
	AvgRating float64   `json:"avg_rating"`
	LastUsed  time.Time `json:"last_used"`
}

// ItemsResponse lists registered items.
type ItemsResponse struct {
	Items []ItemEntry `json:"items"`
}

// PreferencesResponse exposes accumulated user preference signals.
type PreferencesResponse struct {
	PreferredCategories []ScoredLabel `json:"preferred_categories"`
	PreferredTags       []ScoredLabel `json:"preferred_tags"`
	AvoidCategories     []ScoredLabel `json:"avoid_categories"`
	InteractionCount    int64         `json:"interaction_count"`
}

// ScoredLabel is a (label, score) pair sorted by score desc.
type ScoredLabel struct {
	Label string  `json:"label"`
	Score float64 `json:"score"`
}

// EvaluationsResponse lists recent self-evaluations.
type EvaluationsResponse struct {
	Recent []EvaluationEntry `json:"recent"`
}

// EvaluationEntry is one eval.completed event normalized for UI consumption.
type EvaluationEntry struct {
	ID            string    `json:"id"`
	TaskID        string    `json:"task_id"`
	CreatedAt     time.Time `json:"created_at"`
	QualityScore  float64   `json:"quality_score"`
	GoalAchieved  float64   `json:"goal_achieved"`
	Efficiency    float64   `json:"efficiency"`
	Reasoning     string    `json:"reasoning,omitempty"`
	Suggestions   []string  `json:"suggestions,omitempty"`
	SideEffects   []string  `json:"side_effects,omitempty"`
	ShouldDistill bool      `json:"should_distill"`
}

// Recommendations returns top-K recommendations from the engine.
func (h *Handler) Recommendations(w http.ResponseWriter, r *http.Request) {
	limit := parseLimit(r, 10)
	context := r.URL.Query().Get("context")
	out := []recommend.Recommendation{}
	if h.recEngine != nil {
		out = h.recEngine.Recommend(limit, context)
		if out == nil {
			out = []recommend.Recommendation{}
		}
	}
	writeJSON(w, RecommendationsResponse{Recommendations: out, Context: context})
}

// Items returns all registered items with usage statistics.
func (h *Handler) Items(w http.ResponseWriter, r *http.Request) {
	out := []ItemEntry{}
	if h.recEngine != nil {
		profiles := h.recEngine.Items()
		out = make([]ItemEntry, 0, len(profiles))
		for _, p := range profiles {
			out = append(out, ItemEntry{
				ID:        p.ID,
				Category:  p.Category,
				Tags:      p.Tags,
				Uses:      p.Uses,
				Successes: p.Successes,
				Failures:  p.Failures,
				AvgRating: p.AvgRating,
				LastUsed:  p.LastUsed,
			})
		}
		sort.Slice(out, func(i, j int) bool {
			if out[i].Uses != out[j].Uses {
				return out[i].Uses > out[j].Uses
			}
			return out[i].AvgRating > out[j].AvgRating
		})
	}
	writeJSON(w, ItemsResponse{Items: out})
}

// Preferences returns the accumulated user-preference signals, sorted by score.
func (h *Handler) Preferences(w http.ResponseWriter, r *http.Request) {
	resp := PreferencesResponse{
		PreferredCategories: []ScoredLabel{},
		PreferredTags:       []ScoredLabel{},
		AvoidCategories:     []ScoredLabel{},
	}
	if h.recEngine != nil {
		prefs := h.recEngine.Preferences()
		resp.InteractionCount = prefs.InteractionCount
		resp.PreferredCategories = sortMap(prefs.PreferredCategories)
		resp.PreferredTags = sortMap(prefs.PreferredTags)
		resp.AvoidCategories = sortMap(prefs.AvoidCategories)
	}
	writeJSON(w, resp)
}

// Evaluations returns recent eval.completed events.
func (h *Handler) Evaluations(w http.ResponseWriter, r *http.Request) {
	limit := parseLimit(r, 30)
	out := []EvaluationEntry{}
	if h.ledger != nil && h.ledger.Events != nil {
		events, err := h.ledger.Events.Query(r.Context(), ledger.EventQuery{
			Kinds: []ledger.EventKind{eval.EventEvalCompleted},
			Limit: limit,
		})
		if err == nil {
			for _, e := range events {
				if e == nil {
					continue
				}
				out = append(out, decodeEval(e))
			}
		}
	}
	writeJSON(w, EvaluationsResponse{Recent: out})
}

func decodeEval(e *ledger.Event) EvaluationEntry {
	entry := EvaluationEntry{
		ID:        e.ID,
		TaskID:    e.TaskID,
		CreatedAt: e.CreatedAt,
	}
	if len(e.Payload) == 0 {
		return entry
	}
	var er eval.EvalResult
	if err := json.Unmarshal(e.Payload, &er); err != nil {
		return entry
	}
	entry.QualityScore = er.QualityScore
	entry.GoalAchieved = er.GoalAchieved
	entry.Efficiency = er.Efficiency
	entry.Reasoning = er.Reasoning
	entry.Suggestions = er.Suggestions
	entry.SideEffects = er.SideEffects
	entry.ShouldDistill = er.ShouldDistill
	if er.TaskID != "" {
		entry.TaskID = er.TaskID
	}
	return entry
}

func sortMap(m map[string]float64) []ScoredLabel {
	out := make([]ScoredLabel, 0, len(m))
	for k, v := range m {
		out = append(out, ScoredLabel{Label: k, Score: v})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	return out
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
