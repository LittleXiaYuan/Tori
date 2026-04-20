package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/pkg/jsonutil"
)

// MissionParseResult is the structured intent returned from NL mission parsing.
type MissionParseResult struct {
	Type        string         `json:"type"`        // "task" | "workflow" | "cron" | "trigger"
	Name        string         `json:"name"`        // suggested mission name
	Description string         `json:"description"` // cleaned description
	Config      map[string]any `json:"config"`      // type-specific config (cron_expr, event_type, steps, etc.)
	Confidence  float64        `json:"confidence"`  // 0-1 how confident the parse is
	Explanation string         `json:"explanation"` // why this type was chosen
}

const missionParsePrompt = `You are a mission intent classifier. Given a user's natural language description,
determine what type of automated mission to create and extract structured parameters.

Respond ONLY with a JSON object (no markdown, no explanation outside JSON):
{
  "type": "task" | "workflow" | "cron" | "trigger",
  "name": "concise mission name in the user's language",
  "description": "cleaned one-line description",
  "config": {
    // For "cron": include "cron_expr" (standard 5-field cron), "message" (what to send to agent)
    // For "trigger": include "event_type", "condition", "action_type" 
    // For "workflow": include "steps" (array of step descriptions)
    // For "task": include "goal" (task goal)
  },
  "confidence": 0.0-1.0,
  "explanation": "one sentence explaining the classification"
}

Rules:
- If user mentions time/schedule/daily/weekly/hourly → "cron"
- If user mentions "when X happens" / event-driven / condition → "trigger"
- If user mentions multi-step / pipeline / flow / DAG → "workflow"
- Otherwise → "task" (one-off agent task)
- For cron, always generate a valid 5-field cron expression
- Confidence < 0.5 means you're unsure — the user should verify`

func (g *Gateway) handleMissionParse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Description == "" {
		http.Error(w, "description is required", http.StatusBadRequest)
		return
	}

	client := g.planner.LLMClientFor("fast")

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	result, err := parseMissionIntent(ctx, client, req.Description)
	if err != nil {
		slog.Error("mission parse: failed", "err", err)
		http.Error(w, "failed to parse mission intent", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// parseMissionIntent calls the LLM to classify a natural language description
// into a structured MissionParseResult. Reusable from both HTTP handler and IM commands.
func parseMissionIntent(ctx context.Context, client *llm.Client, description string) (MissionParseResult, error) {
	messages := []llm.Message{
		{Role: "system", Content: missionParsePrompt},
		{Role: "user", Content: description},
	}

	reply, err := client.Chat(ctx, messages, 0.3)
	if err != nil {
		return MissionParseResult{}, fmt.Errorf("LLM call failed: %w", err)
	}

	var result MissionParseResult
	if err := json.Unmarshal([]byte(reply), &result); err != nil {
		cleaned := jsonutil.Extract(reply)
		if err2 := json.Unmarshal([]byte(cleaned), &result); err2 != nil {
			slog.Warn("mission parse: failed to parse LLM JSON", "raw", reply, "err", err2)
			result = MissionParseResult{
				Type:        "task",
				Name:        description,
				Description: description,
				Config:      map[string]any{"goal": description},
				Confidence:  0.3,
				Explanation: "Could not reliably classify — defaulting to one-off task.",
			}
		}
	}
	return result, nil
}

