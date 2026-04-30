package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"

	"yunque-agent/internal/agentcore/costtrack"
	"yunque-agent/internal/apperror"
)

func (g *Gateway) handleCostSummary(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.costTracker == nil {
		json.NewEncoder(w).Encode(map[string]string{"status": "not configured"})
		return
	}
	summary := g.costTracker.GetSummary()
	today := g.costTracker.TodayCost()
	json.NewEncoder(w).Encode(map[string]any{
		"summary":    summary,
		"today_cost": today,
		"month_cost": g.costTracker.MonthCost(),
	})
}

func (g *Gateway) handleCostBudget(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.costTracker == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "not configured"})
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
	g.costTracker.SetBudget(budget)
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (g *Gateway) handleCostByTask(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.costTracker == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "not configured"})
		return
	}
	taskID := r.URL.Query().Get("id")
	if taskID == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "id is required")
		return
	}
	json.NewEncoder(w).Encode(g.costTracker.GetTaskCost(taskID))
}

func (g *Gateway) handleCostBreakdown(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.costTracker == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "not configured"})
		return
	}
	json.NewEncoder(w).Encode(map[string]any{
		"by_channel":     g.costTracker.GetCostByChannel(),
		"by_tier":        g.costTracker.GetCostByTier(),
		"by_runner_type": g.costTracker.GetCostByRunnerType(),
		"by_provider":    g.costTracker.GetCostByProvider(),
	})
}

func (g *Gateway) handleCostTaskTimeline(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.costTracker == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "not configured"})
		return
	}
	taskID := r.URL.Query().Get("id")
	if taskID == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "id is required")
		return
	}
	json.NewEncoder(w).Encode(g.costTracker.GetTaskTimeline(taskID))
}

func (g *Gateway) handleCostHistory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.costTracker == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "not configured"})
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
	json.NewEncoder(w).Encode(g.costTracker.GetUsageHistory(f))
}

func (g *Gateway) handleCostAlerts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.costTracker == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "not configured"})
		return
	}
	json.NewEncoder(w).Encode(map[string]any{
		"alerts":     g.costTracker.GetAlerts(),
		"today_cost": g.costTracker.TodayCost(),
		"month_cost": g.costTracker.MonthCost(),
	})
}
