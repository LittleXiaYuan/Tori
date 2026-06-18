package gateway

import (
	"context"
	"encoding/json"
	"log/slog"
	"math/rand/v2"
	"net/http"

	"yunque-agent/internal/agentcore/state"
	"yunque-agent/internal/apperror"
	"yunque-agent/internal/execution/channel"
)

//	from handlers_state.go
//
// handleStateSnapshot GET /v1/state 鈥?杩斿洖瀹屾暣鐘舵€佸揩鐓?
func (g *Gateway) handleStateSnapshot(w http.ResponseWriter, r *http.Request) {
	if g.stateKernel == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "state kernel not initialized")
		return
	}
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeBadRequest, "method not allowed")
		return
	}
	snap := g.stateKernel.TakeSnapshot()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(snap)
}

// handleStateGoals GET/POST/DELETE /v1/state/goals
func (g *Gateway) handleStateGoals(w http.ResponseWriter, r *http.Request) {
	if g.stateKernel == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "state kernel not initialized")
		return
	}

	switch r.Method {
	case http.MethodGet:
		goals := g.stateKernel.Goals()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(goals)

	case http.MethodPost:
		var req struct {
			ID          string   `json:"id"`
			Title       string   `json:"title"`
			Description string   `json:"description"`
			Priority    int      `json:"priority"`
			Status      string   `json:"status"`
			Progress    float64  `json:"progress"`
			ParentGoal  string   `json:"parent_goal"`
			TaskIDs     []string `json:"task_ids"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON")
			return
		}
		if req.Title == "" {
			apperror.WriteCode(w, apperror.CodeBadRequest, "title required")
			return
		}

		// If ID is provided, try to update
		if req.ID != "" {
			if g.stateKernel.UpdateGoal(req.ID, func(g *state.Goal) {
				if req.Title != "" {
					g.Title = req.Title
				}
				if req.Description != "" {
					g.Description = req.Description
				}
				if req.Priority > 0 {
					g.Priority = req.Priority
				}
				if req.Status != "" {
					g.Status = req.Status
				}
				if req.Progress > 0 {
					g.Progress = req.Progress
				}
			}) {
				g.stateKernel.Save()
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"id": req.ID, "status": "updated"})
				return
			}
		}

		// Create new goal
		id := g.stateKernel.AddGoal(state.Goal{
			ID:          req.ID,
			Title:       req.Title,
			Description: req.Description,
			Priority:    req.Priority,
			ParentGoal:  req.ParentGoal,
			TaskIDs:     req.TaskIDs,
		})
		g.stateKernel.Save()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"id": id, "status": "created"})

	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			apperror.WriteCode(w, apperror.CodeBadRequest, "id required")
			return
		}
		if !g.stateKernel.RemoveGoal(id) {
			apperror.WriteCode(w, apperror.CodeNotFound, "goal not found")
			return
		}
		g.stateKernel.Save()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})

	default:
		apperror.WriteCode(w, apperror.CodeBadRequest, "method not allowed")
	}
}

// handleStateFocus GET/POST /v1/state/focus
func (g *Gateway) handleStateFocus(w http.ResponseWriter, r *http.Request) {
	if g.stateKernel == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "state kernel not initialized")
		return
	}

	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"focus": g.stateKernel.Focus()})

	case http.MethodPost:
		var req struct {
			Focus  string   `json:"focus"`
			Topics []string `json:"topics"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON")
			return
		}
		if req.Focus != "" {
			g.stateKernel.SetFocus(req.Focus)
		}
		for _, t := range req.Topics {
			g.stateKernel.AddTopic(t)
		}
		g.stateKernel.Save()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

	default:
		apperror.WriteCode(w, apperror.CodeBadRequest, "method not allowed")
	}
}

// handleStateResources GET/POST/DELETE /v1/state/resources
func (g *Gateway) handleStateResources(w http.ResponseWriter, r *http.Request) {
	if g.stateKernel == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "state kernel not initialized")
		return
	}

	switch r.Method {
	case http.MethodGet:
		res := g.stateKernel.Resources()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(res)

	case http.MethodPost:
		var req state.Resource
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON")
			return
		}
		if req.Path == "" {
			apperror.WriteCode(w, apperror.CodeBadRequest, "path required")
			return
		}
		g.stateKernel.TrackResource(req)
		g.stateKernel.Save()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "tracked"})

	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			apperror.WriteCode(w, apperror.CodeBadRequest, "id required")
			return
		}
		if !g.stateKernel.ReleaseResource(id) {
			apperror.WriteCode(w, apperror.CodeNotFound, "resource not found")
			return
		}
		g.stateKernel.Save()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "released"})

	default:
		apperror.WriteCode(w, apperror.CodeBadRequest, "method not allowed")
	}
}

// PreAckReact sends a random emoji reaction to a message as acknowledgment.
// This is called before processing, so the user sees immediate feedback.
// Similar to AstrBot's preprocess_stage pre_ack_emoji.
func (g *Gateway) PreAckReact(ctx context.Context, channelType, target, messageID string) {
	if len(g.preAckEmojis) == 0 || g.channelReg == nil {
		return
	}

	ch, ok := g.channelReg.Get(channelType)
	if !ok {
		return
	}

	reactor, ok := ch.(channel.Reactor)
	if !ok {
		return
	}

	emoji := g.preAckEmojis[rand.IntN(len(g.preAckEmojis))]
	if err := reactor.React(ctx, target, messageID, emoji); err != nil {
		slog.Debug("pre-ack react failed", "channel", channelType, "err", err)
	}
}
