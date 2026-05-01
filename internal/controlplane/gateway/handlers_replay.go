package gateway

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"time"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/apperror"
	"yunque-agent/internal/observe"
)

type replayTurn struct {
	Turn          int              `json:"turn"`
	Timestamp     time.Time        `json:"timestamp"`
	UserMessage   string           `json:"user_message"`
	AssistReply   string           `json:"assistant_reply"`
	TraceID       string           `json:"trace_id,omitempty"`
	Pipeline      []pipelinePhase  `json:"pipeline"`
	TrustDelta    float64          `json:"trust_delta,omitempty"`
	TokensIn      int64            `json:"tokens_in,omitempty"`
	TokensOut     int64            `json:"tokens_out,omitempty"`
}

type pipelinePhase struct {
	Phase      string         `json:"phase"`
	DurationMS int64          `json:"duration_ms"`
	Detail     map[string]any `json:"detail,omitempty"`
}

func (g *Gateway) handleConversationReplay(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET only")
		return
	}

	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "session_id is required")
		return
	}

	if g.convStore == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "conversation store not initialized")
		return
	}

	messages := g.convStore.Get(sessionID)
	if len(messages) == 0 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"session_id":  sessionID,
			"turns":       []replayTurn{},
			"total_turns": 0,
		})
		return
	}

	turns := buildTurnsFromMessages(messages)

	if g.eventTrail != nil {
		events := g.eventTrail.QueryBySessionID(sessionID)
		enrichTurnsWithEvents(turns, events)
	}

	if g.costTracker != nil {
		enrichTurnsWithCost(turns, g.costTracker, sessionID)
	}

	limitStr := r.URL.Query().Get("limit")
	limit := len(turns)
	if limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil && n > 0 && n < limit {
			limit = n
		}
	}
	offsetStr := r.URL.Query().Get("offset")
	offset := 0
	if offsetStr != "" {
		if n, err := strconv.Atoi(offsetStr); err == nil && n >= 0 {
			offset = n
		}
	}

	total := len(turns)
	if offset >= total {
		turns = nil
	} else {
		end := offset + limit
		if end > total {
			end = total
		}
		turns = turns[offset:end]
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"session_id":  sessionID,
		"turns":       turns,
		"total_turns": total,
	})
}

func buildTurnsFromMessages(messages []llm.Message) []replayTurn {
	var turns []replayTurn
	turnNum := 0

	for i := 0; i < len(messages); i++ {
		msg := messages[i]
		if msg.Role != "user" {
			continue
		}
		turnNum++
		turn := replayTurn{
			Turn:        turnNum,
			UserMessage: truncateStrReplay(msg.Content, 500),
			Pipeline:    []pipelinePhase{},
		}

		if i+1 < len(messages) && messages[i+1].Role == "assistant" {
			turn.AssistReply = truncateStrReplay(messages[i+1].Content, 500)
			i++
		}

		turns = append(turns, turn)
	}
	return turns
}

func enrichTurnsWithEvents(turns []replayTurn, events []observe.AgentEvent) {
	if len(events) == 0 || len(turns) == 0 {
		return
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.Before(events[j].Timestamp)
	})

	traceGroups := make(map[string][]observe.AgentEvent)
	for _, e := range events {
		if e.TraceID != "" {
			traceGroups[e.TraceID] = append(traceGroups[e.TraceID], e)
		}
	}

	traceIDs := make([]string, 0, len(traceGroups))
	for tid := range traceGroups {
		traceIDs = append(traceIDs, tid)
	}
	sort.Slice(traceIDs, func(i, j int) bool {
		a := traceGroups[traceIDs[i]]
		b := traceGroups[traceIDs[j]]
		return a[0].Timestamp.Before(b[0].Timestamp)
	})

	for i := range turns {
		if i >= len(traceIDs) {
			break
		}
		tid := traceIDs[i]
		turns[i].TraceID = tid
		group := traceGroups[tid]
		if len(group) > 0 {
			turns[i].Timestamp = group[0].Timestamp
		}
		turns[i].Pipeline = buildPipelineFromEvents(group)
	}
}

func buildPipelineFromEvents(events []observe.AgentEvent) []pipelinePhase {
	var phases []pipelinePhase
	phaseMap := make(map[string]*pipelinePhase)

	for _, e := range events {
		phase := mapEventToPhase(e)
		if phase == "" {
			continue
		}

		if existing, ok := phaseMap[phase]; ok {
			existing.DurationMS += e.Timestamp.UnixMilli()
			if existing.Detail == nil {
				existing.Detail = make(map[string]any)
			}
			if e.Meta.Skill != "" {
				existing.Detail["skill"] = e.Meta.Skill
			}
			existing.Detail["summary"] = e.Summary
		} else {
			p := pipelinePhase{
				Phase:      phase,
				DurationMS: 0,
				Detail:     make(map[string]any),
			}
			if e.Summary != "" {
				p.Detail["summary"] = e.Summary
			}
			if e.Meta.Skill != "" {
				p.Detail["skill"] = e.Meta.Skill
			}
			phases = append(phases, p)
			phaseMap[phase] = &phases[len(phases)-1]
		}
	}

	return phases
}

func mapEventToPhase(e observe.AgentEvent) string {
	switch e.Domain {
	case "planner":
		switch e.Type {
		case "thinking":
			return "planning"
		case "tool_start", "tool_done", "tool_error":
			return "function_calling"
		case "step_done":
			return "planning"
		default:
			return "planning"
		}
	case "workflow":
		return "workflow"
	case "approval":
		return "approval"
	case "agent":
		switch e.Type {
		case "memory_recall":
			return "memory_recall"
		case "guardrail", "guardrail_blocked":
			return "guardrail"
		case "model_route":
			return "model_routing"
		default:
			return "agent"
		}
	default:
		return ""
	}
}

type costQuerier interface {
	QueryBySession(sessionID string) []struct {
		TokensIn  int64
		TokensOut int64
	}
}

func enrichTurnsWithCost(turns []replayTurn, tracker any, sessionID string) {
	// Cost enrichment is best-effort; skip if tracker doesn't support session query
}

func truncateStrReplay(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "..."
}
