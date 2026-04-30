package costapi

import (
	"encoding/json"
	"fmt"
	"net/http"

	"yunque-agent/internal/agentcore/costtrack"
	"yunque-agent/internal/apperror"
	"yunque-agent/internal/controlplane/gateway/gwshared"
)

// Handler serves cost tracking HTTP endpoints.
type Handler struct {
	Tracker *costtrack.Tracker
}

// RegisterRoutes mounts all /v1/cost/* endpoints.
func (h *Handler) RegisterRoutes(mux *http.ServeMux, auth gwshared.AuthFunc) {
	mux.HandleFunc("/v1/cost/summary", auth(h.handleSummary))
	mux.HandleFunc("/v1/cost/budget", auth(h.handleBudget))
	mux.HandleFunc("/v1/cost/task", auth(h.handleByTask))
	mux.HandleFunc("/v1/cost/task/timeline", auth(h.handleTaskTimeline))
	mux.HandleFunc("/v1/cost/breakdown", auth(h.handleBreakdown))
	mux.HandleFunc("/v1/cost/history", auth(h.handleHistory))
	mux.HandleFunc("/v1/cost/alerts", auth(h.handleAlerts))
}

func (h *Handler) notConfigured(w http.ResponseWriter) bool {
	if h.Tracker == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "not configured"})
		return true
	}
	return false
}

func (h *Handler) handleSummary(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if h.Tracker == nil {
		json.NewEncoder(w).Encode(map[string]string{"status": "not configured"})
		return
	}
	json.NewEncoder(w).Encode(map[string]any{
		"summary":    h.Tracker.GetSummary(),
		"today_cost": h.Tracker.TodayCost(),
		"month_cost": h.Tracker.MonthCost(),
	})
}

func (h *Handler) handleBudget(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if h.notConfigured(w) {
		return
	}
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST required")
		return
	}
	var budget costtrack.Budget
	if err := json.NewDecoder(r.Body).Decode(&budget); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid budget")
		return
	}
	h.Tracker.SetBudget(budget)
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (h *Handler) handleByTask(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if h.notConfigured(w) {
		return
	}
	taskID := r.URL.Query().Get("id")
	if taskID == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "id is required")
		return
	}
	json.NewEncoder(w).Encode(h.Tracker.GetTaskCost(taskID))
}

func (h *Handler) handleBreakdown(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if h.notConfigured(w) {
		return
	}
	json.NewEncoder(w).Encode(map[string]any{
		"by_channel":     h.Tracker.GetCostByChannel(),
		"by_tier":        h.Tracker.GetCostByTier(),
		"by_runner_type": h.Tracker.GetCostByRunnerType(),
		"by_provider":    h.Tracker.GetCostByProvider(),
	})
}

func (h *Handler) handleTaskTimeline(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if h.notConfigured(w) {
		return
	}
	taskID := r.URL.Query().Get("id")
	if taskID == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "id is required")
		return
	}
	json.NewEncoder(w).Encode(h.Tracker.GetTaskTimeline(taskID))
}

func (h *Handler) handleHistory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if h.notConfigured(w) {
		return
	}
	q := r.URL.Query()
	page := 1
	if v := q.Get("page"); v != "" {
		fmt.Sscanf(v, "%d", &page)
	}
	limit := 50
	if v := q.Get("limit"); v != "" {
		fmt.Sscanf(v, "%d", &limit)
	}
	if limit > 200 {
		limit = 200
	}
	f := costtrack.UsageFilter{
		TaskID:     q.Get("task_id"),
		Model:      q.Get("model"),
		Channel:    q.Get("channel"),
		RunnerType: q.Get("runner_type"),
		ProviderID: q.Get("provider_id"),
		Page:       page,
		Limit:      limit,
	}
	json.NewEncoder(w).Encode(h.Tracker.GetUsageHistory(f))
}

func (h *Handler) handleAlerts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if h.notConfigured(w) {
		return
	}
	json.NewEncoder(w).Encode(map[string]any{
		"alerts":     h.Tracker.GetAlerts(),
		"today_cost": h.Tracker.TodayCost(),
		"month_cost": h.Tracker.MonthCost(),
	})
}
