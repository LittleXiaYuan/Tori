package costapi

import (
	"encoding/json"
	"fmt"
	"net/http"

	"yunque-agent/internal/agentcore/costtrack"
	"yunque-agent/internal/apperror"
	"yunque-agent/internal/controlplane/gateway/gwshared"
)

// Route declares one cost-tracking HTTP route.
type Route struct {
	Method      string
	Path        string
	Description string
	Handler     http.HandlerFunc
}

// Handler serves cost tracking HTTP endpoints.
type Handler struct {
	Tracker     *costtrack.Tracker
	TrackerFunc func() *costtrack.Tracker
}

// RouteSpecs returns the cost-tracking surface without mounting it. Pack
// Runtime uses this to own route registration while preserving the existing
// handler implementation.
func (h *Handler) RouteSpecs() []Route {
	return []Route{
		{Method: http.MethodGet, Path: "/v1/cost/summary", Description: "Read aggregate model/task cost summary.", Handler: h.handleSummary},
		{Method: http.MethodPost, Path: "/v1/cost/budget", Description: "Set daily/monthly cost budgets.", Handler: h.handleBudget},
		{Method: http.MethodGet, Path: "/v1/cost/task", Description: "Read cost for one task.", Handler: h.handleByTask},
		{Method: http.MethodGet, Path: "/v1/cost/task/timeline", Description: "Read task cost timeline.", Handler: h.handleTaskTimeline},
		{Method: http.MethodGet, Path: "/v1/cost/breakdown", Description: "Read cost breakdown by channel, tier, runner or provider.", Handler: h.handleBreakdown},
		{Method: http.MethodGet, Path: "/v1/cost/history", Description: "Read paginated usage history.", Handler: h.handleHistory},
		{Method: http.MethodGet, Path: "/v1/cost/alerts", Description: "Read budget alerts and current spend.", Handler: h.handleAlerts},
	}
}

// RegisterRoutes mounts all /v1/cost/* endpoints.
func (h *Handler) RegisterRoutes(mux *http.ServeMux, auth gwshared.AuthFunc) {
	for _, route := range h.RouteSpecs() {
		mux.HandleFunc(route.Path, auth(route.Handler))
	}
}

func (h *Handler) notConfigured(w http.ResponseWriter) bool {
	if h.tracker() == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "not configured"})
		return true
	}
	return false
}

func (h *Handler) tracker() *costtrack.Tracker {
	if h.TrackerFunc != nil {
		return h.TrackerFunc()
	}
	return h.Tracker
}

func (h *Handler) handleSummary(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	tracker := h.tracker()
	if tracker == nil {
		json.NewEncoder(w).Encode(map[string]string{"status": "not configured"})
		return
	}
	json.NewEncoder(w).Encode(map[string]any{
		"summary":    tracker.GetSummary(),
		"today_cost": tracker.TodayCost(),
		"month_cost": tracker.MonthCost(),
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
	h.tracker().SetBudget(budget)
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
	json.NewEncoder(w).Encode(h.tracker().GetTaskCost(taskID))
}

func (h *Handler) handleBreakdown(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if h.notConfigured(w) {
		return
	}
	tracker := h.tracker()
	json.NewEncoder(w).Encode(map[string]any{
		"by_channel":     tracker.GetCostByChannel(),
		"by_tier":        tracker.GetCostByTier(),
		"by_runner_type": tracker.GetCostByRunnerType(),
		"by_provider":    tracker.GetCostByProvider(),
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
	json.NewEncoder(w).Encode(h.tracker().GetTaskTimeline(taskID))
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
	json.NewEncoder(w).Encode(h.tracker().GetUsageHistory(f))
}

func (h *Handler) handleAlerts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if h.notConfigured(w) {
		return
	}
	tracker := h.tracker()
	json.NewEncoder(w).Encode(map[string]any{
		"alerts":     tracker.GetAlerts(),
		"today_cost": tracker.TodayCost(),
		"month_cost": tracker.MonthCost(),
	})
}
