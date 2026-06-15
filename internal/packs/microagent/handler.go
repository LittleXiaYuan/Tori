// Package microagentpack exposes read-only views into the agent's microagent
// registry (domain-specific prompt enhancements) and ReAct reasoning traces
// recorded by the cognicore/react runner.
package microagentpack

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"sync/atomic"
	"time"

	"yunque-agent/internal/cognicore/microagent"
	"yunque-agent/internal/ledgercore"
	"yunque-agent/pkg/packruntime"
)

const PackID = "yunque.pack.micro-agent"

// Config wires runtime dependencies.
type Config struct {
	Registry *microagent.Registry
	Ledger   *ledger.Ledger
	TenantID string
}

// Handler serves the Micro-Agent pack endpoints.
type Handler struct {
	registry *microagent.Registry
	ledger   *ledger.Ledger
	tenantID string
	host     packruntime.Host
	started  atomic.Bool
}

// New creates a Micro-Agent pack handler.
func New(cfg Config) *Handler {
	tenantID := cfg.TenantID
	if tenantID == "" {
		tenantID = "default"
	}
	return &Handler{registry: cfg.Registry, ledger: cfg.Ledger, tenantID: tenantID}
}

// PackID returns the manifest id.
func (h *Handler) PackID() string { return PackID }

// compile-time assertion: Micro-Agent is a v2 capability Module (Tier 0 microkernel).
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
		h.host.Logger().Info("micro-agent pack started", "pack", PackID)
	}
	return nil
}

// Stop marks the pack stopped on disable.
func (h *Handler) Stop(ctx context.Context) error {
	h.started.Store(false)
	return nil
}

// Routes exposes the read-only micro-agent endpoints.
func (h *Handler) Routes() []packruntime.BackendRoute {
	return []packruntime.BackendRoute{
		{Method: http.MethodGet, Path: "/v1/micro-agent/agents", Handler: h.Agents},
		{Method: http.MethodGet, Path: "/v1/micro-agent/resolve", Handler: h.Resolve},
		{Method: http.MethodGet, Path: "/v1/micro-agent/react/trace", Handler: h.Trace},
	}
}

// AgentEntry is a serialized microagent (Content trimmed for list views).
type AgentEntry struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Scope       string            `json:"scope"`
	Trigger     string            `json:"trigger,omitempty"`
	Content     string            `json:"content"`
	Enabled     bool              `json:"enabled"`
	Priority    int               `json:"priority"`
	Tags        []string          `json:"tags,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// AgentsResponse lists all registered microagents.
type AgentsResponse struct {
	Agents []AgentEntry `json:"agents"`
	Total  int          `json:"total"`
}

// ResolveResponse lists microagents activated for a given message.
type ResolveResponse struct {
	Message string       `json:"message"`
	Matched []AgentEntry `json:"matched"`
}

// TraceEntry is a single reasoning event normalized for UI consumption.
type TraceEntry struct {
	ID        string                 `json:"id"`
	Kind      string                 `json:"kind"`
	Actor     string                 `json:"actor"`
	CreatedAt time.Time              `json:"created_at"`
	Payload   map[string]interface{} `json:"payload,omitempty"`
}

// TraceResponse is the ReAct reasoning trace for a task.
type TraceResponse struct {
	TaskID  string       `json:"task_id"`
	Entries []TraceEntry `json:"entries"`
}

// Agents lists every registered microagent, ordered by priority desc then name.
func (h *Handler) Agents(w http.ResponseWriter, r *http.Request) {
	out := []AgentEntry{}
	if h.registry != nil {
		all := h.registry.All()
		out = make([]AgentEntry, 0, len(all))
		for _, ma := range all {
			out = append(out, fromMicroAgent(ma))
		}
		scope := r.URL.Query().Get("scope")
		if scope != "" {
			filtered := make([]AgentEntry, 0, len(out))
			for _, a := range out {
				if a.Scope == scope {
					filtered = append(filtered, a)
				}
			}
			out = filtered
		}
		sort.Slice(out, func(i, j int) bool {
			if out[i].Priority != out[j].Priority {
				return out[i].Priority > out[j].Priority
			}
			return out[i].Name < out[j].Name
		})
	}
	writeJSON(w, AgentsResponse{Agents: out, Total: len(out)})
}

// Resolve returns the microagents that would activate for a given message.
func (h *Handler) Resolve(w http.ResponseWriter, r *http.Request) {
	message := r.URL.Query().Get("message")
	resp := ResolveResponse{Message: message, Matched: []AgentEntry{}}
	if h.registry == nil || message == "" {
		writeJSON(w, resp)
		return
	}
	matched := h.registry.Resolve(message)
	for _, ma := range matched {
		resp.Matched = append(resp.Matched, fromMicroAgent(ma))
	}
	writeJSON(w, resp)
}

// Trace returns the ReAct reasoning trace for a task ordered by time.
func (h *Handler) Trace(w http.ResponseWriter, r *http.Request) {
	taskID := r.URL.Query().Get("task_id")
	limit := parseLimit(r, 200)
	resp := TraceResponse{TaskID: taskID, Entries: []TraceEntry{}}
	if taskID == "" || h.ledger == nil || h.ledger.Events == nil {
		writeJSON(w, resp)
		return
	}
	events, err := h.ledger.Events.Query(r.Context(), ledger.EventQuery{
		TaskID: taskID,
		Kinds: []ledger.EventKind{
			ledger.EventReasoningThought,
			ledger.EventReasoningHypothesis,
			ledger.EventReasoningDecision,
			ledger.EventReasoningBacktrack,
			ledger.EventReasoningObserve,
			ledger.EventReasoningPlan,
			ledger.EventReasoningReflect,
			ledger.EventReasoningConfUpdate,
		},
		Limit: limit,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for _, e := range events {
		if e == nil {
			continue
		}
		resp.Entries = append(resp.Entries, TraceEntry{
			ID:        e.ID,
			Kind:      string(e.Kind),
			Actor:     e.Actor,
			CreatedAt: e.CreatedAt,
			Payload:   decodePayload(e.Payload),
		})
	}
	sort.Slice(resp.Entries, func(i, j int) bool {
		return resp.Entries[i].CreatedAt.Before(resp.Entries[j].CreatedAt)
	})
	writeJSON(w, resp)
}

func fromMicroAgent(ma *microagent.MicroAgent) AgentEntry {
	if ma == nil {
		return AgentEntry{}
	}
	return AgentEntry{
		Name:        ma.Name,
		Description: ma.Description,
		Scope:       string(ma.Scope),
		Trigger:     ma.Trigger,
		Content:     ma.Content,
		Enabled:     ma.Enabled,
		Priority:    ma.Priority,
		Tags:        ma.Tags,
		Metadata:    ma.Metadata,
	}
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
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
			return n
		}
	}
	return def
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
