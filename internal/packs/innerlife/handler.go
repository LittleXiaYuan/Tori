// Package innerlifepack exposes read-only views into the Agent's inner life:
// curiosity exploration, post-conversation reflection, and nighttime dreaming.
// It surfaces ledger events emitted by cognikernel.ReflectiveLoop and
// DreamingLoop plus live curiosity-module questions.
package innerlifepack

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"yunque-agent/internal/cognicore/curiosity"
	"yunque-agent/internal/ledgercore"
	"yunque-agent/pkg/packruntime"
)

const PackID = "yunque.pack.inner-life"

// Config wires runtime dependencies into the Inner Life pack.
type Config struct {
	Ledger    *ledger.Ledger
	Curiosity *curiosity.Module
	TenantID  string
}

// Handler serves the Inner Life pack API surface.
type Handler struct {
	ledger    *ledger.Ledger
	curiosity *curiosity.Module
	tenantID  string
	host      packruntime.Host
	started   atomic.Bool
}

// New creates an Inner Life pack handler.
func New(cfg Config) *Handler {
	tenantID := cfg.TenantID
	if tenantID == "" {
		tenantID = "default"
	}
	return &Handler{ledger: cfg.Ledger, curiosity: cfg.Curiosity, tenantID: tenantID}
}

// PackID returns the stable manifest id.
func (h *Handler) PackID() string { return PackID }

// compile-time assertion: Inner Life is a v2 capability Module (Tier 0 microkernel).
var _ packruntime.Module = (*Handler)(nil)

// Init wires the pack against the kernel Host. Inner Life takes its data
// dependencies via Config, so Host is retained only for kernel services
// (logging today); no concrete gateway type is referenced.
func (h *Handler) Init(host packruntime.Host) error {
	h.host = host
	return nil
}

// Start marks the pack live on enable. Inner Life has no background workers, so
// this only flips the running flag and logs via the kernel logger when present.
func (h *Handler) Start(ctx context.Context) error {
	h.started.Store(true)
	if h.host != nil {
		h.host.Logger().Info("inner-life pack started", "pack", PackID)
	}
	return nil
}

// Stop marks the pack stopped on disable.
func (h *Handler) Stop(ctx context.Context) error {
	h.started.Store(false)
	return nil
}

// Routes exposes the read-only inner-life endpoints.
func (h *Handler) Routes() []packruntime.BackendRoute {
	return []packruntime.BackendRoute{
		{Method: http.MethodGet, Path: "/v1/inner-life/curiosity", Handler: h.Curiosity},
		{Method: http.MethodGet, Path: "/v1/inner-life/reflection", Handler: h.Reflection},
		{Method: http.MethodGet, Path: "/v1/inner-life/dreaming", Handler: h.Dreaming},
	}
}

// CuriosityResponse is the payload of GET /v1/inner-life/curiosity.
type CuriosityResponse struct {
	Pending []curiosity.Question `json:"pending"`
	Recent  []TimelineEntry      `json:"recent"`
}

// ReflectionResponse is the payload of GET /v1/inner-life/reflection.
type ReflectionResponse struct {
	Recent []TimelineEntry `json:"recent"`
}

// DreamingResponse is the payload of GET /v1/inner-life/dreaming.
type DreamingResponse struct {
	Recent []TimelineEntry `json:"recent"`
}

// TimelineEntry is one ledger event normalized for UI consumption.
type TimelineEntry struct {
	ID        string                 `json:"id"`
	Kind      string                 `json:"kind"`
	Actor     string                 `json:"actor"`
	CreatedAt time.Time              `json:"created_at"`
	Payload   map[string]interface{} `json:"payload,omitempty"`
}

// Curiosity returns pending curiosity questions plus a slice of past curiosity
// exploration ledger events. Pending questions are derived from the live
// curiosity module; events are not stored as a dedicated ledger kind yet, so
// the recent slice is empty until the curiosity persistence path is added.
func (h *Handler) Curiosity(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	resp := CuriosityResponse{Pending: []curiosity.Question{}, Recent: []TimelineEntry{}}
	if h.curiosity != nil {
		limit := parseLimit(r, 8)
		if questions, err := h.curiosity.GenerateQuestions(ctx, h.tenantID, limit); err == nil {
			resp.Pending = questions
		}
	}
	writeJSON(w, resp)
}

// Reflection returns recent reflection.completed timeline events.
func (h *Handler) Reflection(w http.ResponseWriter, r *http.Request) {
	entries := h.queryEvents(r.Context(), ledger.EventReflectionCompleted, parseLimit(r, 30))
	writeJSON(w, ReflectionResponse{Recent: entries})
}

// Dreaming returns recent dreaming.completed timeline events.
func (h *Handler) Dreaming(w http.ResponseWriter, r *http.Request) {
	entries := h.queryEvents(r.Context(), ledger.EventDreamingCompleted, parseLimit(r, 30))
	writeJSON(w, DreamingResponse{Recent: entries})
}

func (h *Handler) queryEvents(ctx context.Context, kind ledger.EventKind, limit int) []TimelineEntry {
	if h.ledger == nil || h.ledger.Events == nil {
		return []TimelineEntry{}
	}
	events, err := h.ledger.Events.Query(ctx, ledger.EventQuery{
		Kinds: []ledger.EventKind{kind},
		Limit: limit,
	})
	if err != nil {
		return []TimelineEntry{}
	}
	out := make([]TimelineEntry, 0, len(events))
	for _, e := range events {
		if e == nil {
			continue
		}
		out = append(out, TimelineEntry{
			ID:        e.ID,
			Kind:      string(e.Kind),
			Actor:     e.Actor,
			CreatedAt: e.CreatedAt,
			Payload:   decodePayload(e.Payload),
		})
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
