// Package nightschoolpack exposes read-only views into the Agent's nightly
// learning: dreaming sessions, distilled task experience and learned user
// traits. It pulls from the ledger (dreaming events + distilled memory
// entries) and the trait store.
package nightschoolpack

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"yunque-agent/internal/cognicore/trait"
	"yunque-agent/internal/ledgercore"
	"yunque-agent/pkg/packruntime"
)

const PackID = "yunque.pack.night-school"

// Config wires runtime dependencies into the Night School pack.
type Config struct {
	Ledger     *ledger.Ledger
	TraitStore *trait.Store
	TenantID   string
}

// Handler serves the Night School pack API surface.
type Handler struct {
	ledger     *ledger.Ledger
	traitStore *trait.Store
	tenantID   string
}

// New creates a Night School pack handler.
func New(cfg Config) *Handler {
	tenantID := cfg.TenantID
	if tenantID == "" {
		tenantID = "default"
	}
	return &Handler{ledger: cfg.Ledger, traitStore: cfg.TraitStore, tenantID: tenantID}
}

// PackID returns the stable manifest id.
func (h *Handler) PackID() string { return PackID }

// Routes exposes the read-only night-school endpoints.
func (h *Handler) Routes() []packruntime.BackendRoute {
	return []packruntime.BackendRoute{
		{Method: http.MethodGet, Path: "/v1/night-school/dreams", Handler: h.Dreams},
		{Method: http.MethodGet, Path: "/v1/night-school/distill", Handler: h.Distill},
		{Method: http.MethodGet, Path: "/v1/night-school/traits", Handler: h.Traits},
	}
}

// DreamsResponse is the payload of GET /v1/night-school/dreams.
type DreamsResponse struct {
	Recent []DreamEntry `json:"recent"`
}

// DreamEntry is a single dreaming.completed session normalized for UI.
type DreamEntry struct {
	ID                string    `json:"id"`
	CreatedAt         time.Time `json:"created_at"`
	TenantID          string    `json:"tenant_id,omitempty"`
	ThoughtsGenerated int       `json:"thoughts_generated"`
	ExplorationsRun   int       `json:"explorations_run"`
	FactsDiscovered   int       `json:"facts_discovered"`
	SkillsSuggested   int       `json:"skills_suggested"`
}

// DistillResponse is the payload of GET /v1/night-school/distill.
type DistillResponse struct {
	Rules        []DistillEntry `json:"rules"`
	Patterns     []DistillEntry `json:"patterns"`
	ToolInsights []DistillEntry `json:"tool_insights"`
}

// DistillEntry is a distilled memory entry normalized for UI.
type DistillEntry struct {
	ID         string    `json:"id"`
	Key        string    `json:"key"`
	Content    string    `json:"content"`
	Source     string    `json:"source,omitempty"`
	Confidence float64   `json:"confidence"`
	CreatedAt  time.Time `json:"created_at"`
	TaskID     string    `json:"task_id,omitempty"`
}

// TraitsResponse is the payload of GET /v1/night-school/traits.
type TraitsResponse struct {
	Traits []trait.Trait `json:"traits"`
}

// Dreams returns recent dreaming.completed timeline events with full payload.
func (h *Handler) Dreams(w http.ResponseWriter, r *http.Request) {
	limit := parseLimit(r, 30)
	out := []DreamEntry{}
	if h.ledger != nil && h.ledger.Events != nil {
		events, err := h.ledger.Events.Query(r.Context(), ledger.EventQuery{
			Kinds: []ledger.EventKind{ledger.EventDreamingCompleted},
			Limit: limit,
		})
		if err == nil {
			for _, e := range events {
				if e == nil {
					continue
				}
				payload := decodePayload(e.Payload)
				out = append(out, DreamEntry{
					ID:                e.ID,
					CreatedAt:         e.CreatedAt,
					TenantID:          asString(payload["tenant_id"]),
					ThoughtsGenerated: asInt(payload["thoughts_generated"]),
					ExplorationsRun:   asInt(payload["explorations_run"]),
					FactsDiscovered:   asInt(payload["facts_discovered"]),
					SkillsSuggested:   asInt(payload["skills_suggested"]),
				})
			}
		}
	}
	writeJSON(w, DreamsResponse{Recent: out})
}

// Distill returns distilled rules, patterns and tool insights from the
// memory store, written by the task distiller after every completed task.
func (h *Handler) Distill(w http.ResponseWriter, r *http.Request) {
	limit := parseLimit(r, 50)
	resp := DistillResponse{
		Rules:        []DistillEntry{},
		Patterns:     []DistillEntry{},
		ToolInsights: []DistillEntry{},
	}
	if h.ledger == nil || h.ledger.Memory == nil {
		writeJSON(w, resp)
		return
	}
	ctx := r.Context()

	rules := h.searchMemory(ctx, ledger.MemoryRule, "rule.", limit)
	patterns := h.searchMemory(ctx, ledger.MemoryRule, "pattern.", limit)
	insights := h.searchMemory(ctx, ledger.MemoryExperience, "tool.", limit)

	resp.Rules = rules
	resp.Patterns = patterns
	resp.ToolInsights = insights
	writeJSON(w, resp)
}

// Traits returns learned user traits ordered by confidence desc.
func (h *Handler) Traits(w http.ResponseWriter, r *http.Request) {
	out := []trait.Trait{}
	if h.traitStore != nil {
		limit := parseLimit(r, 50)
		out = h.traitStore.TopTraits(limit)
	}
	writeJSON(w, TraitsResponse{Traits: out})
}

func (h *Handler) searchMemory(ctx context.Context, kind ledger.MemoryKind, keyPrefix string, limit int) []DistillEntry {
	entries, err := h.ledger.Memory.Search(ctx, ledger.MemoryQuery{
		TenantID: h.tenantID,
		Kinds:    []ledger.MemoryKind{kind},
		Source:   "distillation",
		Limit:    limit * 4,
	})
	if err != nil {
		return []DistillEntry{}
	}
	out := make([]DistillEntry, 0, len(entries))
	for _, e := range entries {
		if e == nil || !strings.HasPrefix(e.Key, keyPrefix) {
			continue
		}
		taskID := ""
		if e.TaskID != nil {
			taskID = *e.TaskID
		}
		out = append(out, DistillEntry{
			ID:         e.ID,
			Key:        e.Key,
			Content:    e.Content,
			Source:     e.Source,
			Confidence: e.Confidence,
			CreatedAt:  e.CreatedAt,
			TaskID:     taskID,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func decodePayload(raw ledger.JSON) map[string]interface{} {
	if len(raw) == 0 {
		return nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil
	}
	return m
}

func asString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func asInt(v interface{}) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	}
	return 0
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
